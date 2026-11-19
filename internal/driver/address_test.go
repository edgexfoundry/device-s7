package driver

import (
	"reflect"
	"testing"

	sdkModel "github.com/edgexfoundry/device-sdk-go/v4/pkg/models"
	"github.com/edgexfoundry/go-mod-core-contracts/v4/common"
)

func TestParseAddress(t *testing.T) {
	valid := map[string]Address{
		"DB1.DBX100.0":     {DB: 1, Kind: "X", Byte: 100},
		"db700.dbx4.2":     {DB: 700, Kind: "X", Byte: 4, Bit: 2},
		"DB1.DBX4.3.10":    {DB: 1, Kind: "X", Byte: 4, Bit: 3, Count: 10},
		"DB700.DBB0":       {DB: 700, Kind: "B"},
		"DB700.DBB44.20":   {DB: 700, Kind: "B", Byte: 44, Count: 20},
		"DB2.DBW 204":      {DB: 2, Kind: "W", Byte: 204},
		"DB2.DBD826":       {DB: 2, Kind: "D", Byte: 826},
		"DB700.DBC44.20":   {DB: 700, Kind: "C", Byte: 44, Len: 20},
		"DB700.DBC44.20.8": {DB: 700, Kind: "C", Byte: 44, Len: 20, Count: 8},
		"DB9.DBS10.254":    {DB: 9, Kind: "S", Byte: 10, Len: 254},
	}
	for in, want := range valid {
		got, err := parseAddress(in)
		if err != nil || got != want {
			t.Errorf("parseAddress(%q) = %+v, %v; want %+v", in, got, err, want)
		}
	}
	for _, in := range []string{"", "DBX100", "DB1.DBX100", "DB1.DBX86.8", "DB1.DBX86.2.1.1", "DB0.DBB1",
		"DB1.DBQ1", "DB1.DBC4", "DB1.DBS4.255", "DB1.DBB4.0", "MB10", "T1", "C1", "DB1.DB"} {
		if _, err := parseAddress(in); err == nil {
			t.Errorf("parseAddress(%q) accepted", in)
		}
	}
}

func TestPlanSizes(t *testing.T) {
	cases := []struct {
		node, typ string
		size      int
		bit       bool
	}{
		{"DB1.DBX4.2", common.ValueTypeBool, 1, true},
		{"DB1.DBX4.6.4", common.ValueTypeBoolArray, 2, false},
		{"DB1.DBB0", common.ValueTypeUint8, 1, false},
		{"DB1.DBW2", common.ValueTypeInt16, 2, false},
		{"DB1.DBD2", common.ValueTypeFloat32, 4, false},
		{"DB1.DBD2", common.ValueTypeFloat64, 8, false},
		{"DB1.DBD2", common.ValueTypeInt64, 8, false},
		{"DB1.DBW10.5", common.ValueTypeInt16Array, 10, false},
		{"DB1.DBD10.3", common.ValueTypeFloat64Array, 24, false},
		{"DB1.DBB44.20", common.ValueTypeString, 20, false},
		{"DB1.DBB44.20", common.ValueTypeBinary, 20, false},
		{"DB1.DBB44.20", common.ValueTypeUint8Array, 20, false},
		{"DB1.DBC44.20", common.ValueTypeString, 20, false},
		{"DB1.DBC44.20.8", common.ValueTypeStringArray, 160, false},
		{"DB1.DBS44.20", common.ValueTypeString, 22, false},
		{"DB1.DBS44.20.2", common.ValueTypeStringArray, 44, false},
	}
	for _, c := range cases {
		it, err := plan(c.node, c.typ)
		if err != nil || it.size != c.size || it.bit != c.bit {
			t.Errorf("plan(%s, %s) = size %d bit %v, %v; want %d %v", c.node, c.typ, it.size, it.bit, err, c.size, c.bit)
		}
	}
	for _, c := range [][2]string{
		{"DB1.DBX4.2", common.ValueTypeUint8}, {"DB1.DBX4.2.3", common.ValueTypeBool}, {"DB1.DBX4.2", common.ValueTypeBoolArray},
		{"DB1.DBB4", common.ValueTypeString}, {"DB1.DBB4", common.ValueTypeBinary}, {"DB1.DBW4.2", common.ValueTypeInt16},
		{"DB1.DBW4", common.ValueTypeInt16Array}, {"DB1.DBB4", common.ValueTypeBool}, {"DB1.DBC4.2", common.ValueTypeInt16},
		{"DB1.DBS4.2.2", common.ValueTypeString}, {"DB1.DBB4", common.ValueTypeObject},
	} {
		if _, err := plan(c[0], c[1]); err == nil {
			t.Errorf("plan(%s, %s) accepted", c[0], c[1])
		}
	}
}

func mustPlan(t *testing.T, node, typ string) item {
	t.Helper()
	it, err := plan(node, typ)
	if err != nil {
		t.Fatal(err)
	}
	return it
}

func TestDecode(t *testing.T) {
	cases := []struct {
		node, typ string
		data      []byte
		want      any
	}{
		{"DB1.DBX0.3", common.ValueTypeBool, []byte{1}, true},
		{"DB1.DBX0.3", common.ValueTypeBool, []byte{0}, false},
		{"DB1.DBX0.6.4", common.ValueTypeBoolArray, []byte{0b0100_0000, 0b0000_0010}, []bool{true, false, false, true}},
		{"DB1.DBB0", common.ValueTypeUint8, []byte{0xFE}, uint8(0xFE)},
		{"DB1.DBB0", common.ValueTypeInt8, []byte{0xFE}, int8(-2)},
		{"DB1.DBW0", common.ValueTypeInt16, []byte{0xFF, 0x9C}, int16(-100)},
		{"DB1.DBW0", common.ValueTypeUint16, []byte{0x01, 0x02}, uint16(0x0102)},
		{"DB1.DBD0", common.ValueTypeFloat32, []byte{0x3F, 0xC0, 0, 0}, float32(1.5)},
		{"DB1.DBD0", common.ValueTypeInt32, []byte{0xFF, 0xFF, 0xFF, 0xFE}, int32(-2)},
		{"DB1.DBD0", common.ValueTypeFloat64, []byte{0x3F, 0xF8, 0, 0, 0, 0, 0, 0}, float64(1.5)},
		{"DB1.DBD0", common.ValueTypeUint64, []byte{0, 0, 0, 1, 0, 0, 0, 2}, uint64(1<<32 + 2)},
		{"DB1.DBW0.3", common.ValueTypeInt16Array, []byte{0, 1, 0xFF, 0xFF, 0x7F, 0xFF}, []int16{1, -1, 32767}},
		{"DB1.DBB0.3", common.ValueTypeUint8Array, []byte{1, 2, 3}, []uint8{1, 2, 3}},
		{"DB1.DBD0.2", common.ValueTypeFloat32Array, []byte{0x3F, 0xC0, 0, 0, 0xC0, 0, 0, 0}, []float32{1.5, -2}},
		{"DB1.DBB0.4", common.ValueTypeBinary, []byte{0, 1, 2, 0xFF}, []byte{0, 1, 2, 0xFF}},
		{"DB1.DBB0.8", common.ValueTypeString, []byte("PAL01   "), "PAL01"},
		{"DB1.DBC0.8", common.ValueTypeString, []byte{'A', 'B', 0, 'x', 0, 0, 0, 0}, "AB"},
		{"DB1.DBC0.3", common.ValueTypeString, []byte("..."), "..."},
		{"DB1.DBC0.2", common.ValueTypeString, []byte{0xE9, 'a'}, "éa"},
		{"DB1.DBC0.3.2", common.ValueTypeStringArray, []byte("AB CD "), []string{"AB", "CD"}},
		{"DB1.DBS0.6", common.ValueTypeString, []byte{6, 3, 'a', 'b', 'c', 'z', 'z', 'z'}, "abc"},
		{"DB1.DBS0.2", common.ValueTypeString, []byte{2, 9, 'a', 'b'}, "ab"},
		{"DB1.DBS0.2.2", common.ValueTypeStringArray, []byte{2, 1, 'a', 0, 2, 2, 'b', 'c'}, []string{"a", "bc"}},
	}
	for _, c := range cases {
		got, err := decode(mustPlan(t, c.node, c.typ), c.data)
		if err != nil || !reflect.DeepEqual(got, c.want) {
			t.Errorf("decode(%s, %s, %v) = %#v, %v; want %#v", c.node, c.typ, c.data, got, err, c.want)
			continue
		}
		// Every decoded value must be accepted by the SDK as its type.
		if _, err := sdkModel.NewCommandValue("r", c.typ, got); err != nil {
			t.Errorf("NewCommandValue(%s, %#v): %v", c.typ, got, err)
		}
	}
}

func TestEncode(t *testing.T) {
	cases := []struct {
		node, typ string
		value     any
		want      []byte
	}{
		{"DB1.DBX0.3", common.ValueTypeBool, true, []byte{1}},
		{"DB1.DBW0", common.ValueTypeInt16, int16(-100), []byte{0xFF, 0x9C}},
		{"DB1.DBD0", common.ValueTypeFloat64, float64(1.5), []byte{0x3F, 0xF8, 0, 0, 0, 0, 0, 0}},
		{"DB1.DBW0.2", common.ValueTypeUint16Array, []uint16{1, 0x0203}, []byte{0, 1, 2, 3}},
		{"DB1.DBB0.3", common.ValueTypeBinary, []byte{9, 8, 7}, []byte{9, 8, 7}},
		{"DB1.DBC0.5", common.ValueTypeString, "AB", []byte("AB   ")},
		{"DB1.DBB0.3", common.ValueTypeString, "é", []byte{0xE9, ' ', ' '}},
		{"DB1.DBS0.4", common.ValueTypeString, "ab", []byte{4, 2, 'a', 'b', 0, 0}},
		{"DB1.DBC0.2.2", common.ValueTypeStringArray, []string{"a", "bc"}, []byte("a bc")},
	}
	for _, c := range cases {
		it := mustPlan(t, c.node, c.typ)
		cv, err := sdkModel.NewCommandValue("r", c.typ, c.value)
		if err != nil {
			t.Fatal(err)
		}
		got, err := encode(it, cv)
		if err != nil || !reflect.DeepEqual(got, c.want) {
			t.Errorf("encode(%s, %s, %#v) = %v, %v; want %v", c.node, c.typ, c.value, got, err, c.want)
			continue
		}
		back, err := decode(it, got)
		if err != nil || !reflect.DeepEqual(back, c.value) {
			t.Errorf("round trip %s %s: %#v, %v; want %#v", c.node, c.typ, back, err, c.value)
		}
	}
	bad := []struct {
		node, typ string
		value     any
	}{
		{"DB1.DBC0.2", common.ValueTypeString, "abc"},
		{"DB1.DBS0.2", common.ValueTypeString, "abc"},
		{"DB1.DBC0.2", common.ValueTypeString, "Ω"},
		{"DB1.DBB0.3", common.ValueTypeBinary, []byte{1}},
		{"DB1.DBW0.3", common.ValueTypeInt16Array, []int16{1}},
		{"DB1.DBX0.0.3", common.ValueTypeBoolArray, []bool{true, true, true}},
		{"DB1.DBC0.2.2", common.ValueTypeStringArray, []string{"a"}},
	}
	for _, c := range bad {
		cv, err := sdkModel.NewCommandValue("r", c.typ, c.value)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := encode(mustPlan(t, c.node, c.typ), cv); err == nil {
			t.Errorf("encode(%s, %s, %#v) accepted", c.node, c.typ, c.value)
		}
	}
}

func TestDataItemBitAddressing(t *testing.T) {
	it := mustPlan(t, "DB700.DBX4.2", common.ValueTypeBool)
	r := dataItem(it, make([]byte, 1), false)
	if r.Start != 4 || r.Bit != 2 || r.WordLen != s7wlbit || r.DBNumber != 700 {
		t.Errorf("read item %+v; gos7 reads a bit at Start<<3+Bit", r)
	}
	w := dataItem(it, []byte{1}, true)
	if w.Start != 34 || w.Bit != 0 {
		t.Errorf("write item %+v; gos7 writes a bit at Start in bits", w)
	}
	s := dataItem(mustPlan(t, "DB700.DBB44.20", common.ValueTypeString), make([]byte, 20), false)
	if s.Start != 44 || s.Amount != 20 || s.WordLen != s7wlbyte {
		t.Errorf("string item %+v", s)
	}
}

func TestSplitBatches(t *testing.T) {
	var items []item
	for i := 0; i < 40; i++ {
		items = append(items, mustPlan(t, "DB700.DBX4.2", common.ValueTypeBool))
	}
	for i := 0; i < 8; i++ {
		items = append(items, mustPlan(t, "DB700.DBB44.20", common.ValueTypeString))
	}
	items = append(items, mustPlan(t, "DB700.DBB0.400", common.ValueTypeBinary))

	for _, write := range []bool{false, true} {
		p := splitBatches(items, 240, write)
		seen := map[int]bool{}
		for _, b := range p.multi {
			if len(b) > maxItems {
				t.Errorf("write=%v: batch of %d items", write, len(b))
			}
			req, resp := 12+12*len(b), 14
			for _, i := range b {
				seen[i] = true
				resp += readCost(items[i])
				if write {
					req += 4 + even(items[i].size)
				}
			}
			if req > 240-pduSlack || (!write && resp > 240-pduSlack) {
				t.Errorf("write=%v: batch %v exceeds the PDU (request %d, reply %d)", write, b, req, resp)
			}
		}
		if !reflect.DeepEqual(p.single, []int{48}) {
			t.Errorf("write=%v: single %v, want the 400-byte item", write, p.single)
		}
		if len(seen) != 48 {
			t.Errorf("write=%v: %d items in multi batches, want 48", write, len(seen))
		}
	}
	if p := splitBatches(items[:40], 960, false); len(p.multi) != 3 {
		t.Errorf("40 bits at PDU 960: %d batches, want 3 (16 items each)", len(p.multi))
	}
}
