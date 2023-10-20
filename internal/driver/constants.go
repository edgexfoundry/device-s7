// -*- Mode: Go; indent-tabs-mode: t -*-
//
// Copyright (C) 2023 YIQISOFT
//
// SPDX-License-Identifier: Apache-2.0

package driver

// Constants related to protocol properties
const (
	Protocol     = "s7"
	CommandTopic = "CommandTopic"
)

// Constants related to custom configuration
const (
	CustomConfigSectionName = "S7Info"
	WritableInfoSectionName = CustomConfigSectionName + "/Writable"
)

const (
	INT16 = "INT16"
	INT32 = "INT32"
	BOOL  = "BOOL"

	HOST             = "Host"
	PORT             = "Port"
	RACK             = "Rack"
	SLOT             = "Slot"
	ADDRESS_TYPE     = "AddressType"
	DBADDRESS        = "DBAddress"
	STARTING_ADDRESS = "StartingAddress"
	LENGTH           = "Length"
	POS              = "Pos"
)
