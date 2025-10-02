// Copyright ©2025 Dan Kortschak. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package pmd

import (
	"context"
	"fmt"

	"tinygo.org/x/bluetooth"

	"github.com/kortschak/polar/internal/forkbeard"
)

// Listener implements PMD notification listening.
type Listener struct {
	dev *bluetooth.Device

	cpDevice, dataDevice bluetooth.DeviceCharacteristic

	features Features

	handlers [measurementTypes]func([]byte)
}

// NewListener returns a new Listener for the provided Bluetooth device.
func NewListener(dev *bluetooth.Device) (*Listener, error) {
	cpDevice, err := forkbeard.DeviceCharacteristic(dev, pmdService, pmdCP)
	if err != nil {
		return nil, fmt.Errorf("failed to get device pmd control point characteristic: %w", err)
	}
	// Section 5.1 Figure 1 shows 17 bytes, but this
	// is not otherwise documented and cp does not have
	// an MTU characteristic. The first two bytes are
	// the only relevant data for our use.
	var buf [32]byte
	n, err := cpDevice.Read(buf[:])
	if err != nil {
		return nil, fmt.Errorf("failed read device features: %w", err)
	}
	if n < 2 {
		return nil, fmt.Errorf("device features too short: %#x", buf[:n])
	}
	var feats Features
	copy(feats[:], buf[:2])
	dataDevice, err := forkbeard.DeviceCharacteristic(dev, pmdService, pmdData)
	if err != nil {
		return nil, fmt.Errorf("failed to get device pmd data characteristic: %w", err)
	}
	l := &Listener{
		dev:        dev,
		cpDevice:   cpDevice,
		features:   feats,
		dataDevice: dataDevice,
	}
	err = dataDevice.EnableNotifications(l.dispatch)
	if err != nil {
		return nil, err
	}
	return l, nil
}

func (l *Listener) dispatch(buf []byte) {
	if len(buf) == 0 {
		return
	}
	handle := l.handlers[buf[sampleTypeOffset]]
	if handle != nil {
		handle(buf)
	}
}

// Settings returns the available setting for the recording and measurement type
// of the sensor the Listener is connected to.
func (l *Listener) Settings(ctx context.Context, m MeasureType) ([]Setting, error) {
	return querySettings(ctx, l.cpDevice, MeasureSettings, Online, m)
}

// Set Handler sets the notification handler, command, recording type and
// settings with the results of the h.Handler call.
func (l *Listener) SetHandler(ctx context.Context, h Handler) ([]byte, error) {
	com, measureTyp, settings, handle := h.Handle()
	if int(measureTyp) >= len(l.handlers) {
		return nil, fmt.Errorf("invalid measurement type: %d", measureTyp)
	}
	l.handlers[measureTyp] = handle
	return sendCommand(ctx, l.cpDevice, com, Online, measureTyp, settings...)
}

// Close disables notifications and disconnects the device.
func (l *Listener) Close() error {
	l.dataDevice.EnableNotifications(nil)
	return l.dev.Disconnect()
}

// Features returns the set of features supported by the connected sensor.
func (l *Listener) Features() Features {
	return l.features
}

// Handler defines a PMD notification handler.
type Handler interface {
	// Handle returns the command, measurement types and
	// the settings to configure notifications with. The
	// function is called with the data for all notifications
	// of the specified measurement type.
	Handle() (Command, MeasureType, []Setting, func(buf []byte))
}
