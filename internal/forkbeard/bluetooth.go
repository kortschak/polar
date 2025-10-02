// Copyright ©2025 Dan Kortschak. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Package forkbeard provides helper functions for interacting with
// Bluetooth devices.
package forkbeard

import (
	"fmt"
	"io"

	"tinygo.org/x/bluetooth"
)

// DeviceCharacteristic returns a specified bluetooth.DeviceCharacteristic
// from a Bluetooth service.
func DeviceCharacteristic(dev *bluetooth.Device, srvID, charID bluetooth.UUID) (bluetooth.DeviceCharacteristic, error) {
	srv, err := dev.DiscoverServices([]bluetooth.UUID{srvID})
	if err != nil {
		return bluetooth.DeviceCharacteristic{}, fmt.Errorf("failed to discover service %s: %w", srvID, err)
	}
	for _, s := range srv {
		char, err := s.DiscoverCharacteristics([]bluetooth.UUID{charID})
		if err != nil {
			return bluetooth.DeviceCharacteristic{}, fmt.Errorf("failed to discover characteristic %s: %w", charID, err)
		}
		if len(char) == 0 {
			break
		}
		return char[0], nil
	}
	return bluetooth.DeviceCharacteristic{}, fmt.Errorf("device characteristic not found")
}

// ReadCharacteristic reads data from a Bluetooth characteristic.
func ReadCharacteristic(char bluetooth.DeviceCharacteristic) ([]byte, error) {
	mtu, err := char.GetMTU()
	if err != nil {
		return nil, fmt.Errorf("failed to obtain mtu of characteristic: %w", err)
	}
	buf := make([]byte, mtu)
	n, err := char.Read(buf)
	if err != nil && err != io.EOF {
		return buf[:n], fmt.Errorf("failed to read response from characteristic: %w", err)
	}
	return buf[:n], nil
}
