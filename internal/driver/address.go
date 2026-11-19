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
//	DB<n>.DBX<byte>.<bit>[.<count>]   Bool, or BoolArray of <count> bits
//	DB<n>.DBB<byte>[.<count>]         8-bit values; with <count>: arrays, String (Array of Char), Binary
//	DB<n>.DBW<byte>[.<count>]         16-bit values (the type decides the size, the letter is informative)
//	DB<n>.DBD<byte>[.<count>]         32-bit and 64-bit values
//	DB<n>.DBC<byte>.<len>[.<count>]   Array[0..len-1] of Char as String, or StringArray of <count>
//	DB<n>.DBS<byte>.<len>[.<count>]   S7 STRING[len] as String, or StringArray of <count>
type Address struct {
	DB    int
	Kind  string // X, B, W, D, C, S
	Byte  int
	Bit   int
	Count int // elements of an array (bits for X); 0 means a scalar
	Len   int // characters of one C or S string
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
		if len(rest) < 1 || len(rest) > 2 {
			return a, fmt.Errorf("NodeName %q: a bit address is DB<n>.DBX<byte>.<bit>[.<count>]", nodeName)
		}
		if a.Bit, err = number(rest[0], 0, 7); err != nil {
			return a, fmt.Errorf("NodeName %q: bit: %w", nodeName, err)
		}
		rest = rest[1:]
	case "B", "W", "D":
	case "C", "S":
		if len(rest) < 1 {
			return a, fmt.Errorf("NodeName %q: a string address is DB<n>.DB%s<byte>.<length>[.<count>]", nodeName, a.Kind)
		}
		max := math.MaxUint16
		if a.Kind == "S" {
			max = 254
		}
		if a.Len, err = number(rest[0], 1, max); err != nil {
			return a, fmt.Errorf("NodeName %q: string length: %w", nodeName, err)
		}
		rest = rest[1:]
	default:
		return a, fmt.Errorf("NodeName %q: unknown address type DB%s (use DBX, DBB, DBW, DBD, DBC or DBS)", nodeName, a.Kind)
	}
	switch len(rest) {
	case 0:
	case 1:
		if a.Count, err = number(rest[0], 1, math.MaxUint16); err != nil {
			return a, fmt.Errorf("NodeName %q: count: %w", nodeName, err)
		}
	default:
		return a, fmt.Errorf("NodeName %q has too many parts", nodeName)
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

// arrayElement maps a numeric array type to its element type.
var arrayElement = map[string]string{
	common.ValueTypeUint8Array: common.ValueTypeUint8, common.ValueTypeInt8Array: common.ValueTypeInt8,
	common.ValueTypeUint16Array: common.ValueTypeUint16, common.ValueTypeInt16Array: common.ValueTypeInt16,
	common.ValueTypeUint32Array: common.ValueTypeUint32, common.ValueTypeInt32Array: common.ValueTypeInt32,
	common.ValueTypeFloat32Array: common.ValueTypeFloat32,
	common.ValueTypeUint64Array:  common.ValueTypeUint64, common.ValueTypeInt64Array: common.ValueTypeInt64,
	common.ValueTypeFloat64Array: common.ValueTypeFloat64,
}

// strideOf is the size of one string in the PLC: S7 STRING has a 2-byte header (max, actual length).
func (a Address) strideOf() int {
	if a.Kind == "S" {
		return a.Len + 2
	}
	return a.Len
}

// plan checks that the value type fits the address and computes the transfer size.
func plan(nodeName, valueType string) (item, error) {
	a, err := parseAddress(nodeName)
	if err != nil {
		return item{}, err
	}
	it := item{addr: a, valueType: valueType}
	bad := func(hint string) (item, error) {
		return item{}, fmt.Errorf("NodeName %q does not fit value type %s: %s", nodeName, valueType, hint)
	}
	count := a.Count
	switch a.Kind {
	case "X":
		switch valueType {
		case common.ValueTypeBool:
			if count != 0 {
				return bad("a Bool is one bit, DB<n>.DBX<byte>.<bit>")
			}
			it.bit, it.size = true, 1
		case common.ValueTypeBoolArray:
			if count == 0 {
				return bad("a BoolArray needs a count, DB<n>.DBX<byte>.<bit>.<count>")
			}
			it.size = (a.Bit + count + 7) / 8
		default:
			return bad("DBX holds Bool or BoolArray")
		}
	case "C", "S":
		switch valueType {
		case common.ValueTypeString:
			if count != 0 {
				return bad("use StringArray for several strings")
			}
			it.size = a.strideOf()
		case common.ValueTypeStringArray:
			if count == 0 {
				return bad("a StringArray needs a count")
			}
			it.size = a.strideOf() * count
		default:
			return bad("DBC and DBS hold String or StringArray")
		}
	default: // B, W, D
		switch {
		case scalarSize[valueType] > 0:
			if count != 0 {
				return bad("a scalar takes no count; use the array type")
			}
			it.size = scalarSize[valueType]
		case arrayElement[valueType] != "":
			if count == 0 {
				return bad("an array needs a count, e.g. DB1.DBW10.5")
			}
			it.size = scalarSize[arrayElement[valueType]] * count
		case valueType == common.ValueTypeString:
			if count == 0 {
				return bad("an Array of Char needs its length, e.g. DB1.DBB44.20 (or DB1.DBC44.20)")
			}
			// DBB<byte>.<n> as String is the same as DBC<byte>.<n>.
			it.addr.Kind, it.addr.Len, it.addr.Count = "C", count, 0
			it.size = count
		case valueType == common.ValueTypeBinary:
			if count == 0 {
				return bad("Binary needs its length in bytes, e.g. DB1.DBB44.20")
			}
			it.size = count
		default:
			return bad("unsupported value type")
		}
	}
	return it, nil
}

// decode turns the bytes read for an item into the value of its type.
func decode(it item, buf []byte) (any, error) {
	if len(buf) < it.size {
		return nil, fmt.Errorf("read %d bytes, need %d", len(buf), it.size)
	}
	buf = buf[:it.size]
	a := it.addr
	switch it.valueType {
	case common.ValueTypeBool:
		return buf[0]&1 == 1, nil
	case common.ValueTypeBoolArray:
		out := make([]bool, a.Count)
		for i := range out {
			pos := a.Bit + i
			out[i] = buf[pos/8]&(1<<(pos%8)) != 0
		}
		return out, nil
	case common.ValueTypeString:
		return decodeString(a, buf), nil
	case common.ValueTypeStringArray:
		out := make([]string, a.Count)
		for i := range out {
			out[i] = decodeString(a, buf[i*a.strideOf():(i+1)*a.strideOf()])
		}
		return out, nil
	case common.ValueTypeBinary:
		return append([]byte(nil), buf...), nil
	}
	if t, ok := arrayElement[it.valueType]; ok {
		return decodeArray(t, buf, a.Count), nil
	}
	if _, ok := scalarSize[it.valueType]; ok {
		return decodeScalar(it.valueType, buf), nil
	}
	return nil, fmt.Errorf("unsupported value type %s", it.valueType)
}

// decodeString reads one S7 STRING (header: max length, actual length) or Array of Char.
// Characters are Latin-1; an Array of Char ends at the first NUL and loses trailing spaces.
func decodeString(a Address, b []byte) string {
	if a.Kind == "S" {
		n := int(b[1])
		if n > a.Len {
			n = a.Len
		}
		return latin1(b[2 : 2+n])
	}
	for i, c := range b {
		if c == 0 {
			b = b[:i]
			break
		}
	}
	return strings.TrimRight(latin1(b), " ")
}

func latin1(b []byte) string {
	r := make([]rune, len(b))
	for i, c := range b {
		r[i] = rune(c)
	}
	return string(r)
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

func decodeArray(t string, b []byte, n int) any {
	s := scalarSize[t]
	switch t {
	case common.ValueTypeUint8:
		return append([]uint8(nil), b[:n]...)
	case common.ValueTypeInt8:
		return collect[int8](t, b, s, n)
	case common.ValueTypeUint16:
		return collect[uint16](t, b, s, n)
	case common.ValueTypeInt16:
		return collect[int16](t, b, s, n)
	case common.ValueTypeUint32:
		return collect[uint32](t, b, s, n)
	case common.ValueTypeInt32:
		return collect[int32](t, b, s, n)
	case common.ValueTypeFloat32:
		return collect[float32](t, b, s, n)
	case common.ValueTypeUint64:
		return collect[uint64](t, b, s, n)
	case common.ValueTypeInt64:
		return collect[int64](t, b, s, n)
	case common.ValueTypeFloat64:
		return collect[float64](t, b, s, n)
	}
	return nil
}

func collect[T any](t string, b []byte, size, n int) []T {
	out := make([]T, n)
	for i := range out {
		out[i] = decodeScalar(t, b[i*size:(i+1)*size]).(T)
	}
	return out
}

// encode turns a write parameter into the bytes of its item.
func encode(it item, cv *sdkModel.CommandValue) ([]byte, error) {
	a := it.addr
	buf := make([]byte, it.size)
	switch it.valueType {
	case common.ValueTypeBool:
		v, err := cv.BoolValue()
		if err != nil {
			return nil, err
		}
		if v {
			buf[0] = 1
		}
		return buf, nil
	case common.ValueTypeBoolArray:
		return nil, fmt.Errorf("writing a BoolArray is not supported: write its bits one by one")
	case common.ValueTypeString:
		v, err := cv.StringValue()
		if err != nil {
			return nil, err
		}
		return buf, encodeString(a, buf, v)
	case common.ValueTypeStringArray:
		v, err := cv.StringArrayValue()
		if err != nil {
			return nil, err
		}
		if len(v) != a.Count {
			return nil, fmt.Errorf("StringArray has %d strings, the address holds %d", len(v), a.Count)
		}
		for i, s := range v {
			if err := encodeString(a, buf[i*a.strideOf():(i+1)*a.strideOf()], s); err != nil {
				return nil, fmt.Errorf("string %d: %w", i, err)
			}
		}
		return buf, nil
	case common.ValueTypeBinary:
		v, err := cv.BinaryValue()
		if err != nil {
			return nil, err
		}
		if len(v) != it.size {
			return nil, fmt.Errorf("the Binary value has %d bytes, the address holds %d", len(v), it.size)
		}
		copy(buf, v)
		return buf, nil
	}
	if t, ok := arrayElement[it.valueType]; ok {
		values, err := arrayValues(it.valueType, cv)
		if err != nil {
			return nil, err
		}
		if len(values) != a.Count {
			return nil, fmt.Errorf("%s has %d elements, the address holds %d", it.valueType, len(values), a.Count)
		}
		s := scalarSize[t]
		for i, v := range values {
			putScalar(buf[i*s:(i+1)*s], v)
		}
		return buf, nil
	}
	if _, ok := scalarSize[it.valueType]; ok {
		v, err := scalarValue(it.valueType, cv)
		if err != nil {
			return nil, err
		}
		putScalar(buf, v)
		return buf, nil
	}
	return nil, fmt.Errorf("unsupported value type %s", it.valueType)
}

// encodeString writes an S7 STRING, or an Array of Char padded with spaces. Characters must be Latin-1.
func encodeString(a Address, b []byte, s string) error {
	runes := []rune(s)
	if len(runes) > a.Len {
		return fmt.Errorf("%q is longer than %d characters", s, a.Len)
	}
	chars := b
	if a.Kind == "S" {
		b[0], b[1] = byte(a.Len), byte(len(runes))
		chars = b[2:]
	} else {
		for i := range chars {
			chars[i] = ' '
		}
	}
	for i, r := range runes {
		if r > 0xFF {
			return fmt.Errorf("%q has a character outside Latin-1", s)
		}
		chars[i] = byte(r)
	}
	return nil
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

func arrayValues(t string, cv *sdkModel.CommandValue) ([]any, error) {
	switch t {
	case common.ValueTypeUint8Array:
		return anys(cv.Uint8ArrayValue())
	case common.ValueTypeInt8Array:
		return anys(cv.Int8ArrayValue())
	case common.ValueTypeUint16Array:
		return anys(cv.Uint16ArrayValue())
	case common.ValueTypeInt16Array:
		return anys(cv.Int16ArrayValue())
	case common.ValueTypeUint32Array:
		return anys(cv.Uint32ArrayValue())
	case common.ValueTypeInt32Array:
		return anys(cv.Int32ArrayValue())
	case common.ValueTypeFloat32Array:
		return anys(cv.Float32ArrayValue())
	case common.ValueTypeUint64Array:
		return anys(cv.Uint64ArrayValue())
	case common.ValueTypeInt64Array:
		return anys(cv.Int64ArrayValue())
	case common.ValueTypeFloat64Array:
		return anys(cv.Float64ArrayValue())
	}
	return nil, fmt.Errorf("unsupported value type %s", t)
}

func anys[T any](v []T, err error) ([]any, error) {
	if err != nil {
		return nil, err
	}
	out := make([]any, len(v))
	for i, x := range v {
		out[i] = x
	}
	return out, nil
}
