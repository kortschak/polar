// Copyright ©2025 Dan Kortschak. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Package heart implements handling of the standard 180d Bluetooth
// heart rate service notifications.
package heart

import (
	"encoding/binary"
	"errors"
	"fmt"
	"time"

	"tinygo.org/x/bluetooth"

	"github.com/kortschak/polar/internal/forkbeard"
)

const (
	RateServiceID     = "180d"
	RateMeasurementID = "2a37"
)

var (
	hrService     = must(bluetooth.ParseUUID(RateServiceID))
	hrMeasurement = must(bluetooth.ParseUUID(RateMeasurementID))
)

func must[T any](v T, err error) T {
	if err != nil {
		panic(err)
	}
	return v
}

// RateListener implements handling of heart rate notifications.
type RateListener struct {
	char bluetooth.DeviceCharacteristic
}

// NewRateListener returns a new RateListener for the provided Bluetooth
// device. The h function is called with received heart rate notifications.
func NewRateListener(dev *bluetooth.Device, h func(Rate, error)) (*RateListener, error) {
	char, err := forkbeard.DeviceCharacteristic(dev, hrService, hrMeasurement)
	if err != nil {
		return nil, fmt.Errorf("failed to get heart rate device characteristic: %w", err)
	}
	err = char.EnableNotifications(func(buf []byte) {
		var m Rate
		err := m.UnmarshalBinary(buf)
		h(m, err)
	})
	if err != nil {
		return nil, err
	}
	return &RateListener{char: char}, nil
}

// Close disables heart rate notifications from the connected sensor.
func (l *RateListener) Close() error { return l.char.EnableNotifications(nil) }

// Rate is a heart rate measurement.
type Rate struct {
	HR               uint16
	RR               []time.Duration
	Energy           int // kJ
	EnergyExpended   bool
	Contact          bool
	ContactSupported bool
}

func (m *Rate) UnmarshalBinary(data []byte) error {
	// https://www.bluetooth.com/specifications/specs/heart-rate-service-1-0/

	// 3.1.1.1. Flags Field
	// | 0x10 | 0x8 | 0x4  0x2 | 0x1 |
	// |  rr  | nrg | scs  cnt | fmt |
	hrFormat := int(data[0] & 0x01)
	contact := data[0]&0x6 == 0x6
	contactSupported := data[0]&0x4 != 0
	energyExpended := data[0]&0x8 != 0
	rrPresent := data[0]&0x10 != 0
	offset := 1
	if contactSupported && !contact {
		*m = Rate{
			ContactSupported: true,
		}
		return errors.New("no sensor contact")
	}

	var hrValue uint16
	if hrFormat == 1 {
		hrValue = binary.LittleEndian.Uint16(data[offset:])
	} else {
		hrValue = uint16(data[offset])
	}
	offset += 1 + hrFormat

	energy := -1
	if energyExpended {
		energy = int(binary.LittleEndian.Uint16(data[offset:]))
		offset += 2
	}

	var rr []time.Duration
	if rrPresent {
		rrData := data[offset:]
		rr = make([]time.Duration, 0, len(rrData)/2)
		for i := 0; i < len(rrData); i += 2 {
			rr = append(rr, time.Duration(binary.LittleEndian.Uint16(rrData[i:]))*time.Second/1024)
		}
	}

	*m = Rate{
		HR:               hrValue,
		RR:               rr,
		Energy:           energy,
		EnergyExpended:   energyExpended,
		Contact:          contact,
		ContactSupported: contactSupported,
	}
	return nil
}
