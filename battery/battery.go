// Copyright ©2025 Dan Kortschak. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Package battery implements reading of the standard 180f Bluetooth
// battery service characteristic.
package battery

import (
	"fmt"

	"tinygo.org/x/bluetooth"

	"github.com/kortschak/polar/internal/forkbeard"
)

const (
	ServiceID             = "180f"
	LevelCharacteristicID = "2a19"
)

var (
	batteryService             = must(bluetooth.ParseUUID(ServiceID))
	batteryLevelCharacteristic = must(bluetooth.ParseUUID(LevelCharacteristicID))
)

func must[T any](v T, err error) T {
	if err != nil {
		panic(err)
	}
	return v
}

// Level returns the battery level for the provided Bluetooth device.
func Level(dev *bluetooth.Device) (int, error) {
	// https://www.bluetooth.com/specifications/specs/battery-service/

	batteryDevice, err := forkbeard.DeviceCharacteristic(dev, batteryService, batteryLevelCharacteristic)
	if err != nil {
		return 0, fmt.Errorf("failed to get battery device characteristic: %w", err)
	}
	resp, err := forkbeard.ReadCharacteristic(batteryDevice)
	if err != nil {
		return 0, fmt.Errorf("failed read battery characteristic: %w", err)
	}
	return int(resp[0]), nil
}
