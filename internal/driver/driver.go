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
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/edgexfoundry/device-sdk-go/v3/pkg/interfaces"
	sdkModel "github.com/edgexfoundry/device-sdk-go/v3/pkg/models"
	"github.com/edgexfoundry/go-mod-core-contracts/v3/clients/logger"
	"github.com/edgexfoundry/go-mod-core-contracts/v3/common"
	"github.com/edgexfoundry/go-mod-core-contracts/v3/models"
	"github.com/robinson/gos7"
	"github.com/spf13/cast"
)

const (
	// Word Length
	s7wlbit     = 0x01 // Bit (inside a word)
	s7wlbyte    = 0x02 // Byte (8 bit)
	s7wlChar    = 0x03 // Char (8 bit)
	s7wlword    = 0x04 // Word (16 bit)
	s7wlint     = 0x05 // Int (16 bit)
	s7wldword   = 0x06 // Double Word (32 bit)
	s7wldint    = 0x07 // DInt (32 bit)
	s7wlreal    = 0x08 // Real (32 bit float)
	s7wlcounter = 0x1C // Counter (16 bit)
	s7wltimer   = 0x1D // Timer (16 bit)
)

var once sync.Once
var driver *Driver

type Driver struct {
	lc         logger.LoggingClient
	asyncCh    chan<- *sdkModel.AsyncValues
	sdkService interfaces.DeviceServiceSDK
	s7Clients  map[string]*S7Client
	mu         sync.Mutex
}

func NewProtocolDriver() interfaces.ProtocolDriver {
	once.Do(func() {
		driver = new(Driver)
	})
	return driver
}

type CommandInfo struct {
	Host            string
	Port            int
	Rack            int
	Slot            int
	AddressType     string
	DbAddress       int
	StartingAddress int
	Length          int
	Pos             int
	ValueType       string
}

type DBInfo struct {
	Area       int
	DBNumber   int
	Start      int
	Amount     int
	WordLength int
	DBArray    []string
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

// HandleReadCommands triggers a protocol Read operation for the specified device.
func (s *Driver) HandleReadCommands(deviceName string, protocols map[string]models.ProtocolProperties, reqs []sdkModel.CommandRequest) (res []*sdkModel.CommandValue, err error) {
	s.lc.Debugf("Driver.HandleReadCommands: protocols: %v, resource: %v, attributes: %v", protocols, reqs[0].DeviceResourceName, reqs[0].Attributes)

	// assume the max batch size is 16, must be less than 20
	var batch_size = 16

	var reqs_len = len(reqs)
	var s7DataItems = []gos7.S7DataItem{}

	res = make([]*sdkModel.CommandValue, reqs_len)
	var s7_errors = make([]string, reqs_len)

	// two-dimensional array for handle S7DataItems
	var dataset = make([][]byte, reqs_len)
	for i := range dataset {
		dataset[i] = make([]byte, 4) // 4 bytes
	}

	// Get S7 device connection information, each Device has its own connection.
	s7Client := s.getS7Client(deviceName, protocols)

	// assemble s7DataItems, get items, fetch data

	var times = int(reqs_len / batch_size)
	var remains = reqs_len % batch_size
	var tmp_reqs []sdkModel.CommandRequest

outloop:
	for j := 0; j <= times; j++ {

		// 1. init array
		s7DataItems = s7DataItems[:0] // clear array
		tmp_reqs = tmp_reqs[:0]       // clear array
		if j >= times {
			if remains > 0 {
				tmp_reqs = reqs[times*batch_size : times*batch_size+remains]
			} else {
				break outloop // load complete, exit loop
			}
		} else {
			tmp_reqs = reqs[j*batch_size : j*batch_size+batch_size]
		}

		// 2. get resources from reqs append to items
		for i, req := range tmp_reqs {

			nodename := cast.ToString(req.Attributes["NodeName"])
			dbInfo, _ := s.getDBInfo(nodename)

			var s7DataItem = gos7.S7DataItem{
				Area:     dbInfo.Area,
				WordLen:  dbInfo.WordLength,
				DBNumber: dbInfo.DBNumber,
				Start:    dbInfo.Start,
				Amount:   dbInfo.Amount,
				Data:     dataset[j*batch_size+i],
				Error:    s7_errors[i],
			}
			s7DataItems = append(s7DataItems, s7DataItem)
		}
		s.lc.Debugf("Read from S7DataItems: ", s7DataItems)

		// 3. use AGReadMulti api to get values from S7 device, if error, try 3 times
		retrytimes := 3
		for {
			err = s7Client.Client.AGReadMulti(s7DataItems, len(s7DataItems))
			if err != nil {
				s.lc.Errorf("AGReadMulti Error: %s, reconnecting...", err)
				s.mu.Lock()
				s.s7Clients[deviceName] = nil
				s.mu.Unlock()
				s7Client = s.getS7Client(deviceName, protocols)
			} else {
				s.lc.Debugf("AGReadMulti read from 'dataset': ", dataset)
				break
			}

			retrytimes--
			if retrytimes == 0 {
				break
			}

		}

	}
	// end assemble s7DataItems

	for _, s7_error := range s7_errors {
		if s7_error != "" {
			s.lc.Errorf("S7 Client AGReadMulti error: %s", s7_error)
			return
		}
	}

	// read results from the dataset of s7DataItems
	for i, req := range reqs {

		var result = &sdkModel.CommandValue{}
		var value any

		value, err := getCommandValueType(dataset[i], req.Type)
		if err != nil {
			s.lc.Errorf("getCommandValueType error: %s", err)
			continue
		}

		result, err = getCommandValue(req, value)
		res[i] = result
	}

	s.lc.Debugf("CommandValues: %s", res)

	return
}

// HandleWriteCommands passes a slice of CommandRequest struct each representing
// a ResourceOperation for a specific device resource.
// Since the commands are actuation commands, params provide parameters for the individual
// command.
func (s *Driver) HandleWriteCommands(deviceName string, protocols map[string]models.ProtocolProperties, reqs []sdkModel.CommandRequest,
	params []*sdkModel.CommandValue) error {
	s.lc.Debugf("Driver.HandleWriteCommands: protocols: %v, resource: %v, parameters: %v", protocols, reqs[0].DeviceResourceName, params)

	var err error

	// assume the max batch size is 16, must be less than 20
	var batch_size = 16

	var reqs_len = len(reqs)
	var s7DataItems = []gos7.S7DataItem{}
	var helper gos7.Helper

	var s7_errors = make([]string, reqs_len)

	// two-dimensional array for handle S7DataItems
	var dataset = make([][]byte, reqs_len)
	for i := range dataset {
		dataset[i] = make([]byte, 4) // 4 bytes
	}

	// transfer command values to S7DataItems
	for i, req := range reqs {

		s.lc.Debugf("S7Driver.HandleWriteCommands: protocols: %v, resource: %v, parameters: %v, attributes: %v", protocols, req.DeviceResourceName, params[i], req.Attributes)

		var nodename = cast.ToString(req.Attributes["NodeName"])
		var dbInfo, _ = s.getDBInfo(nodename)

		reading, err := newCommandValue(req.Type, params[i])
		if err != nil {
			s.lc.Errorf("newCommandValue error: %s", err)
		}
		helper.SetValueAt(dataset[i], 0, reading)

		// create gos7 DataItem
		var s7DataItem = gos7.S7DataItem{
			Area:     dbInfo.Area,
			WordLen:  dbInfo.WordLength,
			DBNumber: dbInfo.DBNumber,
			Start:    dbInfo.Start,
			Amount:   dbInfo.Amount,
			Data:     dataset[i],
			Error:    s7_errors[i],
		}
		s7DataItems = append(s7DataItems, s7DataItem)

	}

	s.lc.Debugf("Write to S7DataItems: %s", s7DataItems)

	// send command requests
	times := int(reqs_len / batch_size)
	remains := reqs_len % batch_size
	var tmp_reqs []sdkModel.CommandRequest
	var tmp_s7DateItems = []gos7.S7DataItem{}

	s7Client := s.getS7Client(deviceName, protocols)

outloop:
	for j := 0; j <= times; j++ {

		tmp_s7DateItems = tmp_s7DateItems[:0] // clear array
		tmp_reqs = tmp_reqs[:0]               // clear array
		if j >= times {
			if remains > 0 {
				tmp_s7DateItems = s7DataItems[times*batch_size : times*batch_size+remains]
			} else {
				break outloop // load complete, exit loop
			}
		} else {
			tmp_s7DateItems = s7DataItems[j*batch_size : j*batch_size+batch_size]
		}

		// write data to S7 device, if error, try 3 times
		retrytimes := 3
		for {
			err = s7Client.Client.AGWriteMulti(tmp_s7DateItems, len(tmp_s7DateItems))
			if err != nil {
				s.lc.Errorf("AGWriteMulti Error: %s, reconnecting...", err)
				s.mu.Lock()
				s.s7Clients[deviceName] = nil
				s.mu.Unlock()
				s7Client = s.getS7Client(deviceName, protocols)
			} else {
				s.lc.Debugf("AGWriteMulti write from 'dataset': %s", dataset)
				break
			}

			retrytimes--
			if retrytimes == 0 {
				break
			}
		}
	}

	for _, s7_error := range s7_errors {
		if s7_error != "" {
			s.lc.Errorf("S7 Client AGWriteMulti error: %s", s7_error)
			return err
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
		errt := fmt.Errorf("Failed to initialize S7 client for '%s' device, skipping this device.", deviceName)
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
		errt := fmt.Errorf("Failed to initialize S7 client for '%s' device, skipping this device.", deviceName)
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

// transfer DBstring to DBInfo
func (s *Driver) getDBInfo(variable string) (dbInfo *DBInfo, err error) {

	// varibale sample: DB2.DBX1.0 / DB2.DBD26 / DB2.DBD826
	variable = strings.ToUpper(variable)              //upper
	variable = strings.Replace(variable, " ", "", -1) //remove spaces

	if variable == "" {
		s.lc.Errorf("input [NodeName] variable is empty, variable should be S7 syntax")
		return
	}

	var area int
	var amount int
	var wordLen int
	var dbNo int64
	var dbIndex int64
	var dbArray []string

	//var area, dbNumber, start, amount, wordLen int
	switch valueArea := variable[0:2]; valueArea {
	case "EB": //input byte
	case "EW": //input word
	case "ED": //Input double-word
	case "AB": //Output byte
	case "AW": //Output word
	case "AD": //Output double-word
	case "MB": //Memory byte
	case "MW": //Memory word
	case "MD": //Memory double-word
	case "DB": //Data Block
		// Area ID
		// s7areape = 0x81 //process inputs
		// s7areapa = 0x82 //process outputs
		// s7areamk = 0x83 //Merkers
		// s7areadb = 0x84 //DB
		// s7areact = 0x1C //counters
		// s7areatm = 0x1D //timers
		area = 0x84
		amount = 1
		dbArray = strings.Split(variable, ".")
		if len(dbArray) < 2 {
			s.lc.Errorf("Db Area read variable should not be empty")
			return
		}
		dbNo, _ = strconv.ParseInt(string(string(dbArray[0])[2:]), 10, 16)
		dbIndex, _ = strconv.ParseInt(string(string(dbArray[1])[3:]), 10, 16)
		dbType := string(dbArray[1])[0:3]

		switch dbType {
		case "DBX": //bit
			wordLen = s7wlbit
			// DBIndex = dbIndex + dbBit (DBX12.5 = 12<<3 + 5 = 96+5 = 101 = 0x65)
			dbBit, _ := strconv.ParseInt(string(string(dbArray[2])), 10, 16)
			dbIndex = dbIndex<<3 + dbBit
		case "DBB": //byte
			wordLen = s7wlbyte
		case "DBW": //word
			wordLen = s7wlword
			// amount = 2
		case "DBD": //dword
			wordLen = s7wlreal
			// amount = 4
		default:
			s.lc.Errorf("error when parsing dbtype")

		}
	default:
		switch otherArea := variable[0:1]; otherArea {
		case "E":
		case "I": //input
		case "A":
		case "0": //output
		case "M": //memory
		case "T": //timer
			return
		case "Z":
		case "C": //counter
			return
		default:
			s.lc.Errorf("error when parsing db area")
			return
		}

	}

	return &DBInfo{
		Area:       area,
		DBNumber:   int(dbNo),
		Start:      int(dbIndex),
		Amount:     amount,
		WordLength: wordLen,
		DBArray:    dbArray,
	}, nil

}

// Get command value type
func getCommandValueType(buffer []byte, valueType string) (value any, err error) {
	var helper gos7.Helper

	switch valueType {
	case common.ValueTypeBool:
		var commandValue bool
		helper.GetValueAt(buffer, 0, &commandValue)
		value = commandValue
	case common.ValueTypeString:
		var commandValue string
		helper.GetValueAt(buffer, 0, &commandValue)
		value = commandValue
	case common.ValueTypeUint8:
		var commandValue uint8
		helper.GetValueAt(buffer, 0, &commandValue)
		value = commandValue
	case common.ValueTypeUint16:
		var commandValue uint16
		helper.GetValueAt(buffer, 0, &commandValue)
		value = commandValue
	case common.ValueTypeUint32:
		var commandValue uint32
		helper.GetValueAt(buffer, 0, &commandValue)
		value = commandValue
	case common.ValueTypeUint64:
		var commandValue uint64
		helper.GetValueAt(buffer, 0, &commandValue)
		value = commandValue
	case common.ValueTypeInt8:
		var commandValue int8
		helper.GetValueAt(buffer, 0, &commandValue)
		value = commandValue
	case common.ValueTypeInt16:
		var commandValue int16
		helper.GetValueAt(buffer, 0, &commandValue)
		value = commandValue
	case common.ValueTypeInt32:
		var commandValue int32
		helper.GetValueAt(buffer, 0, &commandValue)
		value = commandValue
	case common.ValueTypeInt64:
		var commandValue int64
		helper.GetValueAt(buffer, 0, &commandValue)
		value = commandValue
	case common.ValueTypeFloat32:
		var commandValue float32
		helper.GetValueAt(buffer, 0, &commandValue)
		value = commandValue
	case common.ValueTypeFloat64:
		var commandValue float64
		helper.GetValueAt(buffer, 0, &commandValue)
		value = commandValue
	default:
		err = fmt.Errorf("fail to convert param, none supported value type: %v", valueType)
	}

	return value, err
}

// Create command value
func newCommandValue(valueType string, param *sdkModel.CommandValue) (any, error) {
	var commandValue any
	var err error
	switch valueType {
	case common.ValueTypeBool:
		commandValue, err = param.BoolValue()
	case common.ValueTypeString:
		commandValue, err = param.StringValue()
	case common.ValueTypeUint8:
		commandValue, err = param.Uint8Value()
	case common.ValueTypeUint16:
		commandValue, err = param.Uint16Value()
	case common.ValueTypeUint32:
		commandValue, err = param.Uint32Value()
	case common.ValueTypeUint64:
		commandValue, err = param.Uint64Value()
	case common.ValueTypeInt8:
		commandValue, err = param.Int8Value()
	case common.ValueTypeInt16:
		commandValue, err = param.Int16Value()
	case common.ValueTypeInt32:
		commandValue, err = param.Int32Value()
	case common.ValueTypeInt64:
		commandValue, err = param.Int64Value()
	case common.ValueTypeFloat32:
		commandValue, err = param.Float32Value()
	case common.ValueTypeFloat64:
		commandValue, err = param.Float64Value()
	default:
		err = fmt.Errorf("fail to convert param, none supported value type: %v", valueType)
	}

	return commandValue, err
}

// Get command value
func getCommandValue(req sdkModel.CommandRequest, reading any) (*sdkModel.CommandValue, error) {
	var err error
	var result = &sdkModel.CommandValue{}
	castError := "fail to parse %v reading, %v"

	if !checkValueInRange(req.Type, reading) {
		err = fmt.Errorf("parse reading fail. Reading %v is out of the value type(%v)'s range", reading, req.Type)
		driver.lc.Error(err.Error())
		return result, err
	}

	var val any
	switch req.Type {
	case common.ValueTypeBool:
		val, err = cast.ToBoolE(reading)
		if err != nil {
			return nil, fmt.Errorf(castError, req.DeviceResourceName, err)
		}
	case common.ValueTypeString:
		val, err = cast.ToStringE(reading)
		if err != nil {
			return nil, fmt.Errorf(castError, req.DeviceResourceName, err)
		}
	case common.ValueTypeUint8:
		val, err = cast.ToUint8E(reading)
		if err != nil {
			return nil, fmt.Errorf(castError, req.DeviceResourceName, err)
		}
	case common.ValueTypeUint16:
		val, err = cast.ToUint16E(reading)
		if err != nil {
			return nil, fmt.Errorf(castError, req.DeviceResourceName, err)
		}
	case common.ValueTypeUint32:
		val, err = cast.ToUint32E(reading)
		if err != nil {
			return nil, fmt.Errorf(castError, req.DeviceResourceName, err)
		}
	case common.ValueTypeUint64:
		val, err = cast.ToUint64E(reading)
		if err != nil {
			return nil, fmt.Errorf(castError, req.DeviceResourceName, err)
		}
	case common.ValueTypeInt8:
		val, err = cast.ToInt8E(reading)
		if err != nil {
			return nil, fmt.Errorf(castError, req.DeviceResourceName, err)
		}
	case common.ValueTypeInt16:
		val, err = cast.ToInt16E(reading)
		if err != nil {
			return nil, fmt.Errorf(castError, req.DeviceResourceName, err)
		}
	case common.ValueTypeInt32:
		val, err = cast.ToInt32E(reading)
		if err != nil {
			return nil, fmt.Errorf(castError, req.DeviceResourceName, err)
		}
	case common.ValueTypeInt64:
		val, err = cast.ToInt64E(reading)
		if err != nil {
			return nil, fmt.Errorf(castError, req.DeviceResourceName, err)
		}
	case common.ValueTypeFloat32:
		val, err = cast.ToFloat32E(reading)
		if err != nil {
			return nil, fmt.Errorf(castError, req.DeviceResourceName, err)
		}
	case common.ValueTypeFloat64:
		val, err = cast.ToFloat64E(reading)
		if err != nil {
			return nil, fmt.Errorf(castError, req.DeviceResourceName, err)
		}
	case common.ValueTypeObject:
		val = reading
	default:
		return nil, fmt.Errorf("return result fail, none supported value type: %v", req.Type)

	}

	result, err = sdkModel.NewCommandValue(req.DeviceResourceName, req.Type, val)
	if err != nil {
		return nil, err
	}
	result.Origin = time.Now().UnixNano()

	return result, nil
}

func (d *Driver) Start() error {

	return nil
}

func (d *Driver) Discover() error {
	return fmt.Errorf("driver's Discover function isn't implemented")
}
