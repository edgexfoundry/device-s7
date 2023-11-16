// -*- Mode: Go; indent-tabs-mode: t -*-
//
// Copyright (C) 2023 YIQISOFT
//
// SPDX-License-Identifier: Apache-2.0

// This package provides a simple example of a S7 device service.
package main

import (
	device_s7 "github.com/edgexfoundry/device-s7"

	"github.com/edgexfoundry/device-s7/internal/driver"
	"github.com/edgexfoundry/device-sdk-go/v3/pkg/startup"
)

const (
	serviceName string = "device-s7"
)

func main() {
	sd := driver.NewProtocolDriver()
	startup.Bootstrap(serviceName, device_s7.Version, sd)
}
