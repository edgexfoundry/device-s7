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

const readCommandsExecutedName = "ReadCommandsExecuted"

const (
	// Word Length
	s7wlbit     = 0x01 //Bit (inside a word)
	s7wlbyte    = 0x02 //Byte (8 bit)
	s7wlChar    = 0x03
	s7wlword    = 0x04 //Word (16 bit)
	s7wlint     = 0x05
	s7wldword   = 0x06 //Double Word (32 bit)
	s7wldint    = 0x07
	s7wlreal    = 0x08 //Real (32 bit float)
	s7wlcounter = 0x1C //Counter (16 bit)
	s7wltimer   = 0x1D //Timer (16 bit)
)

var once sync.Once
var driver *Driver

type Driver struct {
	lc          logger.LoggingClient
	asyncCh     chan<- *sdkModel.AsyncValues
	stringArray []string
	sdkService  interfaces.DeviceServiceSDK
	s7Clients   map[string]*S7Client
	mu          sync.Mutex
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

	for _, device := range sdk.Devices() {
		s7Client := s.NewS7Client(device.Name, device.Protocols)
		if s7Client == nil {
			s.lc.Errorf("failed to initialize S7 client for '%s' device, skipping this device.", device.Name)
			continue
		}
		s.s7Clients[device.Name] = s7Client
	}

	return nil
}

// HandleReadCommands triggers a protocol Read operation for the specified device.
func (s *Driver) HandleReadCommands(deviceName string, protocols map[string]models.ProtocolProperties, reqs []sdkModel.CommandRequest) (res []*sdkModel.CommandValue, err error) {
	s.lc.Debugf("Driver.HandleReadCommands: protocols: %v, resource: %v, attributes: %v", protocols, reqs[0].DeviceResourceName, reqs[0].Attributes)

	var batch_size = 16
	var reqs_len = len(reqs)
	var items = []gos7.S7DataItem{}
	res = make([]*sdkModel.CommandValue, reqs_len)

	var errors = make([]string, reqs_len)

	// Init a two-dimensional array
	var datas = make([][]byte, reqs_len)
	for i := range datas {
		datas[i] = make([]byte, 4) // 4 bytes
	}

	// Get S7 device connection information, each Device has its own connection.
	s7Client := s.getS7Client(deviceName)
	if s7Client == nil {
		s.lc.Errorf("Can not get S7CLient from: %s", deviceName)
		return
	}

	// assembly items
	times := int(reqs_len / batch_size)
	remains := reqs_len % batch_size

	var reqs1 []sdkModel.CommandRequest
outloop:
	for j := 0; j <= times; j++ {

		// 1. init array
		items = items[:0] // clear array
		reqs1 = reqs1[:0] // clear array
		if j >= times {
			if remains > 0 {
				reqs1 = reqs[times*batch_size : times*batch_size+remains]
			} else {
				break outloop // load complete, exit loop
			}
		} else {
			reqs1 = reqs[j*batch_size : j*batch_size+batch_size]
		}

		// 2. get resources from reqs append to items
		for i, req := range reqs1 {

			variable := cast.ToString(req.Attributes["NodeName"])
			dbInfo, _ := s.getDBInfo(variable)
			// fmt.Println(dbInfo, err)

			var item = gos7.S7DataItem{
				Area:     dbInfo.Area,
				WordLen:  dbInfo.WordLength,
				DBNumber: dbInfo.DBNumber,
				Start:    dbInfo.Start,
				Amount:   dbInfo.Amount,
				Data:     datas[j*batch_size+i],
				Error:    errors[i],
			}

			items = append(items, item)
		}

		// 3. use AGReadMulti api to get values from S7
		err = s7Client.Client.AGReadMulti(items, len(items))
		if err != nil {
			s.lc.Info("AGReadMulti Error: %s, reconnecting...", err)
			s.mu.Lock()
			s.s7Clients[deviceName] = s.NewS7Client(deviceName, protocols)
			s.mu.Unlock()
			return
		} else {
			//fmt.Println(datas)
		}
	}

	for _, err1 := range errors {
		if err1 != "" {
			s.lc.Errorf("S7 Client AGReadMulti error: %s", err1)
			return
		}
	}

	// read results from items
	for i, req := range reqs {

		// var helper gos7.Helper

		var result = &sdkModel.CommandValue{}
		var value interface{}

		value, err := getCommandValueType(datas[i], req.Type)
		if err != nil {
			s.lc.Errorf("getCommandValueType error: %s", err)
			continue
		}
		result, err = getCommandValue(req, value)

		res[i] = result
	}

	// fmt.Println(res)
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
	var batch_size = 16
	var reqs_len = len(reqs)
	var items = []gos7.S7DataItem{}
	var helper gos7.Helper

	var errors = make([]string, reqs_len)

	// init array
	var datas = make([][]byte, reqs_len)
	for i := range datas {
		datas[i] = make([]byte, 4) // 4 bytes
	}

	// transfer command values to S7DataItems
	for i, req := range reqs {
		s.lc.Debugf("S7Driver.HandleWriteCommands: protocols: %v, resource: %v, parameters: %v, attributes: %v", protocols, req.DeviceResourceName, params[i], req.Attributes)

		variable := cast.ToString(req.Attributes["NodeName"])
		dbInfo, _ := s.getDBInfo(variable)

		reading, err := newCommandValue(req.Type, params[i])
		if err != nil {
			s.lc.Errorf("newCommandValue error: %s", err)
		}
		helper.SetValueAt(datas[i], 0, reading)

		// create gos7 DataItem
		var item = gos7.S7DataItem{
			Area:     dbInfo.Area,
			WordLen:  dbInfo.WordLength,
			DBNumber: dbInfo.DBNumber,
			Start:    dbInfo.Start,
			Amount:   dbInfo.Amount,
			Data:     datas[i],
			Error:    errors[i],
		}
		items = append(items, item)

	}
	fmt.Println("S7DataItems: ", items)

	// send command requests
	times := int(reqs_len / batch_size)
	remains := reqs_len % batch_size
	var reqs1 []sdkModel.CommandRequest
	var items1 = []gos7.S7DataItem{}
	s7Client := s.getS7Client(deviceName)

outloop:
	for j := 0; j <= times; j++ {
		items1 = items1[:0]
		reqs1 = reqs1[:0] // clear array
		if j >= times {
			if remains > 0 {
				items1 = items[times*batch_size : times*batch_size+remains]
			} else {
				break outloop // load complete, exit loop
			}
		} else {
			items1 = items[j*batch_size : j*batch_size+batch_size]
		}
		// fmt.Println("reqs1: ", reqs1, "items1: ", items1)

		// write data to S7
		// fmt.Println("reqs len: ", len(items))
		err = s7Client.Client.AGWriteMulti(items1, len(items1))
		if err != nil {
			s.lc.Errorf("AGWriteMulti Error: %s, reconnecting...", err)
			s.mu.Lock()
			s.s7Clients[deviceName] = s.NewS7Client(deviceName, protocols)
			s.mu.Unlock()
			return err
		} else {
			//fmt.Println(datas)
		}
	}
	for _, err1 := range errors {
		if err1 != "" {
			s.lc.Errorf("S7 Client AGWriteMulti error: %s", err1)
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

	s7Client := s.NewS7Client(deviceName, protocols)
	if s7Client == nil {
		s.lc.Errorf("failed to initialize S7 client for '%s' device, skipping this device.", deviceName)
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
	return nil
}

// RemoveDevice is a callback function that is invoked
// when a Device associated with this Device Service is removed
func (s *Driver) RemoveDevice(deviceName string, protocols map[string]models.ProtocolProperties) error {
	s.lc.Debugf("Device %s is removed", deviceName)
	s.mu.Lock()
	s.s7Clients[deviceName] = nil
	s.mu.Unlock()
	return nil
}

func (s *Driver) ValidateDevice(device models.Device) error {

	return nil
}

// Create S7Client by 'Device' definition
func (s *Driver) NewS7Client(deviceName string, protocol map[string]models.ProtocolProperties) *S7Client {

	pp := protocol[Protocol]
	var errt error
	host, errt := cast.ToStringE(pp["Host"])
	if errt != nil {
		s.lc.Errorf("Host not found or not a string in Protocol, error: %s", errt)
		return nil
	}
	port, errt := cast.ToStringE(pp["Port"])
	if errt != nil {
		s.lc.Errorf("Port not found or not an integer in Protocol, error: %s", errt)
		return nil
	}
	rack, errt := cast.ToIntE(pp["Rack"])
	if errt != nil {
		s.lc.Errorf("Rack not found or not an integer in Protocol, error: %s", errt)
		return nil
	}
	slot, errt := cast.ToIntE(pp["Slot"])
	if errt != nil {
		s.lc.Errorf("Slot not found or not an integer in Protocol, error: %s", errt)
		return nil
	}
	timeout, errt := cast.ToIntE(pp["Timeout"])
	if errt != nil {
		s.lc.Errorf("Timeout not found or not an integer in Protocol, USE DEFAULT 30s error: %s", errt)
		timeout = 30
	}
	idletimeout, errt := cast.ToIntE(pp["IdleTimeout"])
	if errt != nil {
		s.lc.Errorf("IdleTimeout not found or not an ingeger in Protocol, USE DEFAULT 30s, error: %s", errt)
		idletimeout = 30
	}

	// create handler: PLC tcp client
	handler := gos7.NewTCPClientHandler(host+":"+port, rack, slot)
	if handler == nil {
		s.lc.Errorf("Cant not create NewTCPClientHandler: %s", handler)
		return nil
	}
	s.lc.Infof("New TCP Client: %s", handler)

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
func (s *Driver) getS7Client(deviceName string) *S7Client {
	s.mu.Lock()
	s7Client := s.s7Clients[deviceName]
	s.mu.Unlock()
	if s7Client == nil {
		s.lc.Errorf("Get S7Client for device [%s] error.", deviceName)
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
func getCommandValueType(buffer []byte, valueType string) (value interface{}, err error) {
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
func newCommandValue(valueType string, param *sdkModel.CommandValue) (interface{}, error) {
	var commandValue interface{}
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
func getCommandValue(req sdkModel.CommandRequest, reading interface{}) (*sdkModel.CommandValue, error) {
	var err error
	var result = &sdkModel.CommandValue{}
	castError := "fail to parse %v reading, %v"

	if !checkValueInRange(req.Type, reading) {
		err = fmt.Errorf("parse reading fail. Reading %v is out of the value type(%v)'s range", reading, req.Type)
		driver.lc.Error(err.Error())
		return result, err
	}

	var val interface{}
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
