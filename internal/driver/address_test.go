package driver

import (
	"reflect"
	"testing"

	sdkModel "github.com/edgexfoundry/device-sdk-go/v4/pkg/models"
	"github.com/edgexfoundry/go-mod-core-contracts/v4/common"
)

func TestParseAddress(t *testing.T) {
	valid := map[string]Address{
		"DB1.DBX100.0": {DB: 1, Kind: "X", Byte: 100},
		"db700.dbx4.2": {DB: 700, Kind: "X", Byte: 4, Bit: 2},
		"DB700.DBB0":   {DB: 700, Kind: "B"},
		"DB2.DBW 204":  {DB: 2, Kind: "W", Byte: 204},
		"DB2.DBD826":   {DB: 2, Kind: "D", Byte: 826},
	}
	for in, want := range valid {
		got, err := parseAddress(in)
		if err != nil || got != want {
			t.Errorf("parseAddress(%q) = %+v, %v; want %+v", in, got, err, want)
		}
	}
	for _, in := range []string{"", "DBX100", "DB1.DBX100", "DB1.DBX86.8", "DB1.DBX86.2.1", "DB0.DBB1",
		"DB1.DBQ1", "DB1.DBW4.2", "MB10", "T1", "C1", "DB1.DB"} {
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
		{"DB1.DBB0", common.ValueTypeUint8, 1, false},
		{"DB1.DBW2", common.ValueTypeInt16, 2, false},
		{"DB1.DBD2", common.ValueTypeFloat32, 4, false},
		{"DB1.DBD2", common.ValueTypeFloat64, 8, false},
		{"DB1.DBD2", common.ValueTypeInt64, 8, false},
		{"DB1.DBD2", common.ValueTypeUint64, 8, false},
	}
	for _, c := range cases {
		it, err := plan(c.node, c.typ)
		if err != nil || it.size != c.size || it.bit != c.bit {
			t.Errorf("plan(%s, %s) = size %d bit %v, %v; want %d %v", c.node, c.typ, it.size, it.bit, err, c.size, c.bit)
		}
	}
	for _, c := range [][2]string{
		{"DB1.DBX4.2", common.ValueTypeUint8}, {"DB1.DBB4", common.ValueTypeBool}, {"DB1.DBB4", common.ValueTypeObject},
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

func TestDecodeEncode(t *testing.T) {
	cases := []struct {
		node, typ string
		data      []byte
		value     any
	}{
		{"DB1.DBX0.3", common.ValueTypeBool, []byte{1}, true},
		{"DB1.DBX0.3", common.ValueTypeBool, []byte{0}, false},
		{"DB1.DBB0", common.ValueTypeUint8, []byte{0xFE}, uint8(0xFE)},
		{"DB1.DBB0", common.ValueTypeInt8, []byte{0xFE}, int8(-2)},
		{"DB1.DBW0", common.ValueTypeInt16, []byte{0xFF, 0x9C}, int16(-100)},
		{"DB1.DBW0", common.ValueTypeUint16, []byte{0x01, 0x02}, uint16(0x0102)},
		{"DB1.DBD0", common.ValueTypeFloat32, []byte{0x3F, 0xC0, 0, 0}, float32(1.5)},
		{"DB1.DBD0", common.ValueTypeInt32, []byte{0xFF, 0xFF, 0xFF, 0xFE}, int32(-2)},
		{"DB1.DBD0", common.ValueTypeUint32, []byte{0x80, 0, 0, 1}, uint32(0x80000001)},
		{"DB1.DBD0", common.ValueTypeFloat64, []byte{0x3F, 0xF8, 0, 0, 0, 0, 0, 0}, float64(1.5)},
		{"DB1.DBD0", common.ValueTypeInt64, []byte{0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFF, 0xFD}, int64(-3)},
		{"DB1.DBD0", common.ValueTypeUint64, []byte{0, 0, 0, 1, 0, 0, 0, 2}, uint64(1<<32 + 2)},
	}
	for _, c := range cases {
		it := mustPlan(t, c.node, c.typ)
		got, err := decode(it, c.data)
		if err != nil || !reflect.DeepEqual(got, c.value) {
			t.Errorf("decode(%s, %s, %v) = %#v, %v; want %#v", c.node, c.typ, c.data, got, err, c.value)
			continue
		}
		if _, err := sdkModel.NewCommandValue("r", c.typ, got); err != nil {
			t.Errorf("NewCommandValue(%s, %#v): %v", c.typ, got, err)
		}
		cv, _ := sdkModel.NewCommandValue("r", c.typ, c.value)
		encoded, err := encode(it, cv)
		if err != nil || !reflect.DeepEqual(encoded, c.data) && c.typ != common.ValueTypeBool {
			t.Errorf("encode(%s, %s, %#v) = %v, %v; want %v", c.node, c.typ, c.value, encoded, err, c.data)
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
	d := dataItem(mustPlan(t, "DB700.DBD10", common.ValueTypeFloat64), make([]byte, 8), false)
	if d.Start != 10 || d.Amount != 8 || d.WordLen != s7wlbyte {
		t.Errorf("Float64 item %+v, want 8 bytes from byte 10", d)
	}
}

func TestSplitBatches(t *testing.T) {
	var items []item
	for i := 0; i < 40; i++ {
		items = append(items, mustPlan(t, "DB700.DBX4.2", common.ValueTypeBool))
	}
	for i := 0; i < 30; i++ {
		items = append(items, mustPlan(t, "DB700.DBD8", common.ValueTypeFloat64))
	}
	for _, pdu := range []int{240, 960} {
		for _, write := range []bool{false, true} {
			p := splitBatches(items, pdu, write)
			seen := 0
			for _, b := range p.multi {
				if len(b) > maxItems {
					t.Errorf("pdu %d write=%v: batch of %d items", pdu, write, len(b))
				}
				req, resp := 12+12*len(b), 14
				for _, i := range b {
					resp += readCost(items[i])
					if write {
						req += 4 + even(items[i].size)
					}
				}
				if req > pdu-pduSlack || (!write && resp > pdu-pduSlack) {
					t.Errorf("pdu %d write=%v: batch %v exceeds the PDU (request %d, reply %d)", pdu, write, b, req, resp)
				}
				seen += len(b)
			}
			if seen != len(items) || len(p.single) != 0 {
				t.Errorf("pdu %d write=%v: %d items batched, %d single", pdu, write, seen, len(p.single))
			}
		}
	}
	// An item too large for any multi request goes on its own.
	if p := splitBatches(items[40:41], 30, false); len(p.multi) != 0 || len(p.single) != 1 {
		t.Errorf("Float64 at PDU 30: %+v, want it single", p)
	}
}
