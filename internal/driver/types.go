// -*- mode: Go; indent-tabs-mode: t -*-
//
// Copyright (C) 2023 YIQISOFT
//
// SPDX-License-Identifier: Apache-2.0

package driver

import (
	"github.com/robinson/gos7"
)

type S7Info struct {
	// PLC connection info
	Host string
	Port int
	Rack int
	Slot int

	// DB address, start, size
	DbAddress    int
	StartAddress int
	ReadSize     int

	ConnEstablishingRetry int
	ConnRetryWaitTime     int
}

type S7Client struct {
	DeviceName string
	Client     gos7.Client
}
