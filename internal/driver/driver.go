// -*- Mode: Go; indent-tabs-mode: t -*-
//
// Copyright (C) 2023 YIQISOFT
//
// SPDX-License-Identifier: Apache-2.0

// This package provides an example implementation of
// S7 interface.
package driver

import (
	"fmt"
	"sync"
	"time"

	"github.com/edgexfoundry/device-sdk-go/v4/pkg/interfaces"
	sdkModel "github.com/edgexfoundry/device-sdk-go/v4/pkg/models"
	"github.com/edgexfoundry/go-mod-core-contracts/v4/clients/logger"
	"github.com/edgexfoundry/go-mod-core-contracts/v4/models"

	"github.com/robinson/gos7"
	"github.com/spf13/cast"
)

const (
	// Word Length
	s7wlbit  = 0x01 // Bit (inside a word)
	s7wlbyte = 0x02 // Byte (8 bit)
)

var once sync.Once
var driver *Driver

type Driver struct {
	lc        logger.LoggingClient
	asyncCh   chan<- *sdkModel.AsyncValues
	s7Clients map[string]*S7Client
	mu        sync.Mutex
}

func NewProtocolDriver() interfaces.ProtocolDriver {
	once.Do(func() {
		driver = new(Driver)
	})
	return driver
}

// Initialize performs protocol-specific initialization for the device
// service.
func (s *Driver) Initialize(sdk interfaces.DeviceServiceSDK) error {
	s.lc = sdk.LoggingClient()
	s.asyncCh = sdk.AsyncValuesChannel()
	s.s7Clients = make(map[string]*S7Client)

	// initialize the all devices connection in the service started
	for _, device := range sdk.Devices() {
		s7Client := s.NewS7Client(device.Name, device.Protocols)
		if s7Client == nil {
			s.lc.Errorf("failed to initialize S7 client for '%s' device, skipping this device.", device.Name)
			continue
		}
		s.s7Clients[device.Name] = s7Client
		s.lc.Debugf("S7Client connected for device: %s", device.Name)
	}

	return nil
}

// withClient runs op on the device's connection; a failed op reconnects and is tried up to 3 times.
func (s *Driver) withClient(deviceName string, protocols map[string]models.ProtocolProperties, op func(c *S7Client) error) error {
	var err error
	for attempt := 0; attempt < 3; attempt++ {
		c := s.getS7Client(deviceName, protocols)
		if err = op(c); err == nil {
			return nil
		}
		s.lc.Errorf("S7 request to %s failed: %s, reconnecting...", deviceName, err)
		s.mu.Lock()
		s.s7Clients[deviceName] = nil
		s.mu.Unlock()
	}
	return err
}

// planRequests plans every request; an invalid one gets its error and is left out of the transfer.
func (s *Driver) planRequests(reqs []sdkModel.CommandRequest) ([]item, []int, []error) {
	var items []item
	var index []int // position in reqs of each planned item
	errs := make([]error, len(reqs))
	for i, req := range reqs {
		it, err := plan(cast.ToString(req.Attributes["NodeName"]), req.Type)
		if err != nil {
			s.lc.Errorf("resource %s: %s", req.DeviceResourceName, err)
			errs[i] = err
			continue
		}
		items = append(items, it)
		index = append(index, i)
	}
	return items, index, errs
}

// HandleReadCommands triggers a protocol Read operation for the specified device.
func (s *Driver) HandleReadCommands(deviceName string, protocols map[string]models.ProtocolProperties, reqs []sdkModel.CommandRequest) ([]*sdkModel.CommandValue, error) {
	s.lc.Debugf("Driver.HandleReadCommands: protocols: %v, resource: %v, attributes: %v", protocols, reqs[0].DeviceResourceName, reqs[0].Attributes)

	items, index, errs := s.planRequests(reqs)
	data := make([][]byte, len(items))
	for i, it := range items {
		data[i] = make([]byte, it.size)
	}

	batches := splitBatches(items, pduOf(s.getS7Client(deviceName, protocols)), false)
	for _, batch := range batches.multi {
		dataItems := make([]gos7.S7DataItem, len(batch))
		for k, i := range batch {
			dataItems[k] = dataItem(items[i], data[i], false)
		}
		err := s.withClient(deviceName, protocols, func(c *S7Client) error {
			return c.Client.AGReadMulti(dataItems, len(dataItems))
		})
		for k, i := range batch {
			switch {
			case err != nil:
				errs[index[i]] = err
			case dataItems[k].Error != "":
				errs[index[i]] = fmt.Errorf("%s", dataItems[k].Error)
			}
		}
	}
	for _, i := range batches.single {
		it := items[i]
		errs[index[i]] = s.withClient(deviceName, protocols, func(c *S7Client) error {
			return c.Client.AGReadDB(it.addr.DB, it.addr.Byte, it.size, data[i])
		})
	}

	var res []*sdkModel.CommandValue
	for i, it := range items {
		req := reqs[index[i]]
		if err := errs[index[i]]; err != nil {
			s.lc.Errorf("reading %s failed: %s", req.DeviceResourceName, err)
			continue
		}
		value, err := decode(it, data[i])
		if err != nil {
			s.lc.Errorf("decoding %s failed: %s", req.DeviceResourceName, err)
			continue
		}
		cv, err := sdkModel.NewCommandValue(req.DeviceResourceName, req.Type, value)
		if err != nil {
			s.lc.Errorf("reading %s: %s", req.DeviceResourceName, err)
			continue
		}
		cv.Origin = time.Now().UnixNano()
		res = append(res, cv)
	}
	if len(res) == 0 {
		return nil, fmt.Errorf("read of %d resources from %s failed", len(reqs), deviceName)
	}
	s.lc.Debugf("CommandValues: %s", res)
	return res, nil
}

// HandleWriteCommands passes a slice of CommandRequest struct each representing
// a ResourceOperation for a specific device resource.
// Since the commands are actuation commands, params provide parameters for the individual
// command.
func (s *Driver) HandleWriteCommands(deviceName string, protocols map[string]models.ProtocolProperties, reqs []sdkModel.CommandRequest,
	params []*sdkModel.CommandValue) error {
	s.lc.Debugf("Driver.HandleWriteCommands: protocols: %v, resource: %v, parameters: %v", protocols, reqs[0].DeviceResourceName, params)

	planned, index, errs := s.planRequests(reqs)
	for _, err := range errs {
		if err != nil {
			return err
		}
	}
	data := make([][]byte, len(planned))
	for i, it := range planned {
		b, err := encode(it, params[index[i]])
		if err != nil {
			return fmt.Errorf("resource %s: %w", reqs[index[i]].DeviceResourceName, err)
		}
		data[i] = b
	}

	batches := splitBatches(planned, pduOf(s.getS7Client(deviceName, protocols)), true)
	for _, batch := range batches.multi {
		dataItems := make([]gos7.S7DataItem, len(batch))
		for k, i := range batch {
			dataItems[k] = dataItem(planned[i], data[i], true)
		}
		if err := s.withClient(deviceName, protocols, func(c *S7Client) error {
			return c.Client.AGWriteMulti(dataItems, len(dataItems))
		}); err != nil {
			return err
		}
		for k, i := range batch {
			if e := dataItems[k].Error; e != "" {
				return fmt.Errorf("writing %s failed: %s", reqs[index[i]].DeviceResourceName, e)
			}
		}
	}
	for _, i := range batches.single {
		it := planned[i]
		if err := s.withClient(deviceName, protocols, func(c *S7Client) error {
			return c.Client.AGWriteDB(it.addr.DB, it.addr.Byte, it.size, data[i])
		}); err != nil {
			return fmt.Errorf("writing %s failed: %w", reqs[index[i]].DeviceResourceName, err)
		}
	}
	return nil
}

// Stop the protocol-specific DS code to shutdown gracefully, or
// if the force parameter is 'true', immediately. The driver is responsible
// for closing any in-use channels, including the channel used to send async
// readings (if supported).
func (s *Driver) Stop(force bool) error {

	s.mu.Lock()
	s.s7Clients = nil
	s.mu.Unlock()

	// Then Logging Client might not be initialized
	if s.lc != nil {
		s.lc.Debugf("Driver.Stop called: force=%v", force)
	}
	return nil
}

// AddDevice is a callback function that is invoked
// when a new Device associated with this Device Service is added
func (s *Driver) AddDevice(deviceName string, protocols map[string]models.ProtocolProperties, adminState models.AdminState) error {
	s.lc.Debugf("a new Device is added: %s", deviceName)

	s.mu.Lock()
	s.s7Clients[deviceName] = nil
	s.mu.Unlock()
	s7Client := s.getS7Client(deviceName, protocols)
	if s7Client == nil {
		errt := fmt.Errorf("failed to initialize S7 client for '%s' device, skipping this device", deviceName)
		s.lc.Errorf(errt.Error())
		return errt
	}
	s.mu.Lock()
	s.s7Clients[deviceName] = s7Client
	s.mu.Unlock()
	return nil
}

// UpdateDevice is a callback function that is invoked
// when a Device associated with this Device Service is updated
func (s *Driver) UpdateDevice(deviceName string, protocols map[string]models.ProtocolProperties, adminState models.AdminState) error {
	s.lc.Debugf("Device %s is updated", deviceName)

	s7Client := s.NewS7Client(deviceName, protocols)
	if s7Client == nil {
		errt := fmt.Errorf("failed to initialize S7 client for '%s' device, skipping this device", deviceName)
		s.lc.Errorf(errt.Error())
		return errt
	}
	s.mu.Lock()
	s.s7Clients[deviceName] = s7Client
	s.mu.Unlock()

	return nil
}

// RemoveDevice is a callback function that is invoked
// when a Device associated with this Device Service is removed
func (s *Driver) RemoveDevice(deviceName string, protocols map[string]models.ProtocolProperties) error {
	s.lc.Debugf("Device %s is removed", deviceName)
	s.mu.Lock()
	delete(s.s7Clients, deviceName)
	s.mu.Unlock()
	return nil
}

func (s *Driver) ValidateDevice(device models.Device) error {
	s.lc.Debugf("Validating device: %s", device.Name)

	protocols := device.Protocols
	pp := protocols[Protocol]
	var errt error

	if pp == nil {
		errt = fmt.Errorf("%s not found in protocols for device %s", Protocol, device.Name)
		s.lc.Error(errt.Error())
		return errt
	}

	_, errt = cast.ToStringE(pp["Host"])
	if errt != nil {
		s.lc.Errorf("Host not found or not a string in Protocol, error: %s", errt)
		return errt
	}
	_, errt = cast.ToIntE(pp["Port"])
	if errt != nil {
		s.lc.Errorf("Port not found or not an integer in Protocol, error: %s", errt)
		return errt
	}
	_, errt = cast.ToIntE(pp["Rack"])
	if errt != nil {
		s.lc.Errorf("Rack not found or not an integer in Protocol, error: %s", errt)
		return errt
	}
	_, errt = cast.ToIntE(pp["Slot"])
	if errt != nil {
		s.lc.Errorf("Slot not found or not an integer in Protocol, error: %s", errt)
		return errt
	}
	_, errt = cast.ToIntE(pp["Timeout"])
	if errt != nil {
		s.lc.Errorf("Timeout not found or not an integer in Protocol, USE DEFAULT 30s error: %s", errt)
		pp["Timeout"] = 30
	}
	_, errt = cast.ToIntE(pp["IdleTimeout"])
	if errt != nil {
		s.lc.Errorf("IdleTimeout not found or not an ingeger in Protocol, USE DEFAULT 30s, error: %s", errt)
		pp["IdleTimeout"] = 30
	}

	return nil
}

// Create S7Client by 'Device' definition
func (s *Driver) NewS7Client(deviceName string, protocol map[string]models.ProtocolProperties) *S7Client {

	pp := protocol[Protocol]

	host, _ := cast.ToStringE(pp["Host"])
	port, _ := cast.ToStringE(pp["Port"])
	rack, _ := cast.ToIntE(pp["Rack"])
	slot, _ := cast.ToIntE(pp["Slot"])
	timeout, _ := cast.ToIntE(pp["Timeout"])
	idletimeout, _ := cast.ToIntE(pp["IdleTimeout"])

	// create handler: PLC tcp client
	handler := gos7.NewTCPClientHandler(host+":"+port, rack, slot)
	if handler == nil {
		s.lc.Errorf("Cant not create NewTCPClientHandler: %s", handler)
		return nil
	}
	s.lc.Debugf("New TCP Client: %s", handler)

	// handler connect timeout from 'Timeout'
	handler.Timeout = time.Duration(timeout) * time.Second

	// handler connect idle timeout from 'IdleTimeout'
	handler.IdleTimeout = time.Duration(idletimeout) * time.Second

	// connect to S7
	err := handler.Connect()
	if err != nil {
		s.lc.Errorf("Can't handler S7 Connect: %s, error: %s", deviceName, err)
		// return nil
	}

	s7client := gos7.NewClient(handler)
	client := &S7Client{
		DeviceName: deviceName,
		Client:     s7client,
		Handler:    handler,
	}
	return client

}

// Get S7Client by 'DeviceName'
func (s *Driver) getS7Client(deviceName string, protocols map[string]models.ProtocolProperties) *S7Client {
	s.mu.Lock()
	s7Client := s.s7Clients[deviceName]
	s.mu.Unlock()

	if s7Client == nil {
		s.lc.Warnf("S7CLient for device %s not found. Creating it...", deviceName)
		s7Client = s.NewS7Client(deviceName, protocols)
		s.mu.Lock()
		s.s7Clients[deviceName] = s7Client
		s.mu.Unlock()
	}

	return s7Client

}

func (d *Driver) Start() error {

	return nil
}

func (d *Driver) Discover() error {
	return fmt.Errorf("driver's Discover function isn't implemented")
}
