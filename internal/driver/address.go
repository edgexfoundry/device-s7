// -*- Mode: Go; indent-tabs-mode: t -*-
//
// SPDX-License-Identifier: Apache-2.0

package driver

import (
	"encoding/binary"
	"fmt"
	"math"
	"strconv"
	"strings"

	sdkModel "github.com/edgexfoundry/device-sdk-go/v4/pkg/models"
	"github.com/edgexfoundry/go-mod-core-contracts/v4/common"
)

// Address is a parsed NodeName of the data block area.
//
//	DB<n>.DBX<byte>.<bit>   Bool
//	DB<n>.DBB<byte>         8-bit values
//	DB<n>.DBW<byte>         16-bit values (the value type decides the size, the letter is informative)
//	DB<n>.DBD<byte>         32-bit and 64-bit values
type Address struct {
	DB   int
	Kind string // X, B, W, D
	Byte int
	Bit  int
}

// item is an address planned for one value type: what to transfer and how to decode it.
type item struct {
	addr      Address
	valueType string
	bit       bool // a single bit transfer
	size      int  // bytes to transfer (1 for a bit)
}

func parseAddress(nodeName string) (Address, error) {
	v := strings.ToUpper(strings.ReplaceAll(nodeName, " ", ""))
	parts := strings.Split(v, ".")
	if len(parts) < 2 || !strings.HasPrefix(parts[0], "DB") || len(parts[1]) < 4 || !strings.HasPrefix(parts[1], "DB") {
		return Address{}, fmt.Errorf("NodeName %q is not a data block address like DB1.DBW2", nodeName)
	}
	var a Address
	var err error
	if a.DB, err = number(parts[0][2:], 1, math.MaxUint16); err != nil {
		return a, fmt.Errorf("NodeName %q: data block number: %w", nodeName, err)
	}
	a.Kind = parts[1][2:3]
	if a.Byte, err = number(parts[1][3:], 0, math.MaxUint16); err != nil {
		return a, fmt.Errorf("NodeName %q: byte offset: %w", nodeName, err)
	}
	rest := parts[2:]
	switch a.Kind {
	case "X":
		if len(rest) != 1 {
			return a, fmt.Errorf("NodeName %q: a bit address is DB<n>.DBX<byte>.<bit>", nodeName)
		}
		if a.Bit, err = number(rest[0], 0, 7); err != nil {
			return a, fmt.Errorf("NodeName %q: bit: %w", nodeName, err)
		}
	case "B", "W", "D":
		if len(rest) != 0 {
			return a, fmt.Errorf("NodeName %q has too many parts", nodeName)
		}
	default:
		return a, fmt.Errorf("NodeName %q: unknown address type DB%s (use DBX, DBB, DBW or DBD)", nodeName, a.Kind)
	}
	return a, nil
}

func number(s string, min, max int) (int, error) {
	n, err := strconv.Atoi(s)
	if err != nil {
		return 0, fmt.Errorf("%q is not a number", s)
	}
	if n < min || n > max {
		return 0, fmt.Errorf("%d is outside %d..%d", n, min, max)
	}
	return n, nil
}

// scalarSize is the size in bytes of one numeric element.
var scalarSize = map[string]int{
	common.ValueTypeUint8: 1, common.ValueTypeInt8: 1,
	common.ValueTypeUint16: 2, common.ValueTypeInt16: 2,
	common.ValueTypeUint32: 4, common.ValueTypeInt32: 4, common.ValueTypeFloat32: 4,
	common.ValueTypeUint64: 8, common.ValueTypeInt64: 8, common.ValueTypeFloat64: 8,
}

// plan checks that the value type fits the address and computes the transfer size.
func plan(nodeName, valueType string) (item, error) {
	a, err := parseAddress(nodeName)
	if err != nil {
		return item{}, err
	}
	it := item{addr: a, valueType: valueType}
	switch {
	case a.Kind == "X" && valueType == common.ValueTypeBool:
		it.bit, it.size = true, 1
	case a.Kind != "X" && scalarSize[valueType] > 0:
		it.size = scalarSize[valueType]
	case a.Kind == "X":
		return item{}, fmt.Errorf("NodeName %q does not fit value type %s: DBX holds a Bool", nodeName, valueType)
	default:
		return item{}, fmt.Errorf("NodeName %q does not fit value type %s: unsupported value type", nodeName, valueType)
	}
	return it, nil
}

// decode turns the bytes read for an item into the value of its type.
func decode(it item, buf []byte) (any, error) {
	if len(buf) < it.size {
		return nil, fmt.Errorf("read %d bytes, need %d", len(buf), it.size)
	}
	if it.valueType == common.ValueTypeBool {
		return buf[0]&1 == 1, nil
	}
	if _, ok := scalarSize[it.valueType]; ok {
		return decodeScalar(it.valueType, buf[:it.size]), nil
	}
	return nil, fmt.Errorf("unsupported value type %s", it.valueType)
}

func decodeScalar(t string, b []byte) any {
	switch t {
	case common.ValueTypeUint8:
		return b[0]
	case common.ValueTypeInt8:
		return int8(b[0])
	case common.ValueTypeUint16:
		return binary.BigEndian.Uint16(b)
	case common.ValueTypeInt16:
		return signed[int16](b)
	case common.ValueTypeUint32:
		return binary.BigEndian.Uint32(b)
	case common.ValueTypeInt32:
		return signed[int32](b)
	case common.ValueTypeFloat32:
		return math.Float32frombits(binary.BigEndian.Uint32(b))
	case common.ValueTypeUint64:
		return binary.BigEndian.Uint64(b)
	case common.ValueTypeInt64:
		return signed[int64](b)
	case common.ValueTypeFloat64:
		return math.Float64frombits(binary.BigEndian.Uint64(b))
	}
	return nil
}

// signed reads a big-endian two's complement integer.
func signed[T int16 | int32 | int64](b []byte) T {
	var v T
	_, _ = binary.Decode(b, binary.BigEndian, &v)
	return v
}

// encode turns a write parameter into the bytes of its item.
func encode(it item, cv *sdkModel.CommandValue) ([]byte, error) {
	buf := make([]byte, it.size)
	if it.valueType == common.ValueTypeBool {
		v, err := cv.BoolValue()
		if err != nil {
			return nil, err
		}
		if v {
			buf[0] = 1
		}
		return buf, nil
	}
	v, err := scalarValue(it.valueType, cv)
	if err != nil {
		return nil, err
	}
	putScalar(buf, v)
	return buf, nil
}

func putScalar(b []byte, v any) {
	switch x := v.(type) {
	case uint8:
		b[0] = x
	case int8:
		b[0] = byte(x)
	case uint16:
		binary.BigEndian.PutUint16(b, x)
	case int16:
		_, _ = binary.Encode(b, binary.BigEndian, x)
	case uint32:
		binary.BigEndian.PutUint32(b, x)
	case int32:
		_, _ = binary.Encode(b, binary.BigEndian, x)
	case float32:
		binary.BigEndian.PutUint32(b, math.Float32bits(x))
	case uint64:
		binary.BigEndian.PutUint64(b, x)
	case int64:
		_, _ = binary.Encode(b, binary.BigEndian, x)
	case float64:
		binary.BigEndian.PutUint64(b, math.Float64bits(x))
	}
}

func scalarValue(t string, cv *sdkModel.CommandValue) (any, error) {
	switch t {
	case common.ValueTypeUint8:
		return cv.Uint8Value()
	case common.ValueTypeInt8:
		return cv.Int8Value()
	case common.ValueTypeUint16:
		return cv.Uint16Value()
	case common.ValueTypeInt16:
		return cv.Int16Value()
	case common.ValueTypeUint32:
		return cv.Uint32Value()
	case common.ValueTypeInt32:
		return cv.Int32Value()
	case common.ValueTypeFloat32:
		return cv.Float32Value()
	case common.ValueTypeUint64:
		return cv.Uint64Value()
	case common.ValueTypeInt64:
		return cv.Int64Value()
	case common.ValueTypeFloat64:
		return cv.Float64Value()
	}
	return nil, fmt.Errorf("unsupported value type %s", t)
}
