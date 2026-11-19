// -*- Mode: Go; indent-tabs-mode: t -*-
//
// SPDX-License-Identifier: Apache-2.0

package driver

import (
	"github.com/robinson/gos7"
)

const (
	areaDB = 0x84
	// maxItems per multi request (the protocol allows 20).
	maxItems = 16
	// defaultPDU is the smallest PDU S7 CPUs negotiate, used before a connection told us more.
	defaultPDU = 240
	// pduSlack covers the ISO-on-TCP headers that gos7 counts in its own PDU checks.
	pduSlack = 8
)

func even(n int) int { return n + n%2 }

// readCost and writeCost are the bytes one item adds to the S7 PDU of a multi request or its reply.
func readCost(it item) int  { return 4 + even(it.size) }
func writeCost(it item) int { return 12 + 4 + even(it.size) }

// plan of one multi transfer: indices into the items, plus items too large for any multi request,
// which go one by one through AGReadDB/AGWriteDB (those split by PDU themselves).
type batchPlan struct {
	multi  [][]int
	single []int
}

// splitBatches groups items so that every request and reply fits the PDU.
// Read: request 12+12n, reply 14+sum(4+size). Write: request 12+sum(12+4+size), reply small.
func splitBatches(items []item, pdu int, write bool) batchPlan {
	limit := pdu - pduSlack
	var p batchPlan
	var cur []int
	used := 0
	flush := func() {
		if len(cur) > 0 {
			p.multi = append(p.multi, cur)
		}
		cur, used = nil, 0
	}
	for i, it := range items {
		var cost, base int
		if write {
			cost, base = writeCost(it), 12
		} else {
			cost, base = readCost(it), 14
		}
		if !it.bit && base+cost > limit {
			p.single = append(p.single, i)
			continue
		}
		requestFits := write || 12+12*(len(cur)+1) <= limit
		if len(cur) == maxItems || base+used+cost > limit || !requestFits {
			flush()
		}
		cur = append(cur, i)
		used += cost
	}
	flush()
	return p
}

// dataItem builds the gos7 item. gos7 addresses a bit as Start<<3+Bit when reading and as Start
// (in bits) when writing; other items are transferred as bytes.
func dataItem(it item, data []byte, write bool) gos7.S7DataItem {
	a := it.addr
	if it.bit {
		d := gos7.S7DataItem{Area: areaDB, WordLen: s7wlbit, DBNumber: a.DB, Amount: 1, Data: data}
		if write {
			d.Start = a.Byte<<3 + a.Bit
		} else {
			d.Start, d.Bit = a.Byte, a.Bit
		}
		return d
	}
	return gos7.S7DataItem{Area: areaDB, WordLen: s7wlbyte, DBNumber: a.DB, Start: a.Byte, Amount: it.size, Data: data}
}

func pduOf(c *S7Client) int {
	if c != nil && c.Handler != nil && c.Handler.PDULength > 0 {
		return c.Handler.PDULength
	}
	return defaultPDU
}
