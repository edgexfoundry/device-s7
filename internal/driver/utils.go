// -*- Mode: Go; indent-tabs-mode: t -*-
//
// Copyright (C) 2023 YIQISOFT
//
// SPDX-License-Identifier: Apache-2.0

package driver

import (
	"fmt"

	"github.com/edgexfoundry/go-mod-core-contracts/v3/errors"
	"github.com/spf13/cast"
)

func castToInt(i any, field string) (int, errors.EdgeX) {
	res, err := cast.ToIntE(i)
	if err != nil {
		return 0, errors.NewCommonEdgeX(errors.KindContractInvalid, fmt.Sprintf("%s should be castable to an integer value", field), err)
	}

	return res, nil
}

func castToString(i any) (string, errors.EdgeX) {
	res, err := cast.ToStringE(i)
	if err != nil {
		return "", errors.NewCommonEdgeX(errors.KindContractInvalid, fmt.Sprintf("should be castable to a string value"), err)
	}

	return res, nil
}
