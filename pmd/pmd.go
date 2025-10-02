// Copyright ©2025 Dan Kortschak. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

// Package PMD implements interaction with Polar Measurement Data
// Bluetooth services.
//
// Technical documentation for the PMD protocols are available from the
// [Polar BLE SDK] repository.
//
// [Polar BLE SDK]: https://github.com/polarofficial/polar-ble-sdk/tree/master/technical_documentation
package pmd

import (
	"bytes"
	"context"
	"encoding/binary"
	"fmt"
	"io"
	"math"
	"strings"

	"tinygo.org/x/bluetooth"
)

// Service and characteristic identifiers.
const (
	pmdServiceID      = "fb005c80-02e7-f387-1cad-8acd2d8df0c8"
	pmdControlPointID = "fb005c81-02e7-f387-1cad-8acd2d8df0c8"
	pmdDataID         = "fb005c82-02e7-f387-1cad-8acd2d8df0c8"
)

var (
	pmdService = must(bluetooth.ParseUUID(pmdServiceID))
	pmdCP      = must(bluetooth.ParseUUID(pmdControlPointID))
	pmdData    = must(bluetooth.ParseUUID(pmdDataID))
)

func must[T any](v T, err error) T {
	if err != nil {
		panic(err)
	}
	return v
}

// Features is the a set of supported PMD features.
type Features [2]byte

func (f Features) String() string {
	if f[0] != 0xf {
		return fmt.Sprintf("%#x", f)
	}
	var s strings.Builder
	for b := 1; b < 256; b <<= 1 {
		if f[1]&byte(b) != 0 {
			if s.Len() != 0 {
				s.WriteByte('|')
			}
			s.WriteString(Support(b).String())
		}
	}
	return s.String()
}

// Support is the flag set of supported PMD features.
type Support byte

//go:generate go tool golang.org/x/tools/cmd/stringer -type Support -trimprefix Support
const (
	SupportECG          Support = 1 << 0
	SupportPPG          Support = 1 << 1
	SupportAcc          Support = 1 << 2
	SupportPPI          Support = 1 << 3
	SupportBioImpedance Support = 1 << 4
	SupportGyro         Support = 1 << 5
	SupportMag          Support = 1 << 6
)

const epoch = 946684800 // epoch 2000 January 1st 00:00:00 UTC

// Command is a PMD control point command.
type Command uint8

const (
	MeasureSettings Command = 1
	MeasureStart    Command = 2
	MeasureStop     Command = 3
)

// RecordingType is a PMD recording mode type.
type RecordingType uint8

const (
	Online  RecordingType = 0
	Offline RecordingType = 1
)

type (
	// MeasureType is a measurement stream data type.
	MeasureType uint8
	// FrameType is the sub-type for a MeasureType.
	FrameType uint8
)

// Measurement types and their frame types.
const (
	ECGType           MeasureType = 0
	ECGFrameType0     FrameType   = 0
	ECGSamplingStride             = 3

	PPGType       MeasureType = 1
	PPGFrameType0 FrameType   = 0
	PPGFrameType4 FrameType   = 4
	PPGFrameType5 FrameType   = 5
	PPGFrameType6 FrameType   = 6
	PPGFrameType7 FrameType   = 7
	PPGFrameType8 FrameType   = 8
	PPGFrameType9 FrameType   = 9

	AccType       MeasureType = 2
	AccFrameType0 FrameType   = 0
	AccFrameType1 FrameType   = 1
	AccFrameType2 FrameType   = 2

	PPIType       MeasureType = 3
	PPIFrameType0 FrameType   = 0

	GyroType       MeasureType = 5
	GyroFrameType0 FrameType   = 0
	GyroFrameType1 FrameType   = 1

	MagnetometerType       MeasureType = 6
	MagnetometerFrameType0 FrameType   = 0

	SDKModeType MeasureType = 9

	LocationType MeasureType = 10

	PressureType       MeasureType = 11
	PressureFrameType0 FrameType   = 0

	TemperatureType       MeasureType = 12
	TemperatureFrameType0 FrameType   = 0

	measurementTypes = 13
)

// Packet offsets.
const (
	sampleTypeOffset = 0
	timeStampOffset  = 1
	frameTypeOffset  = 9
	dataOffset       = 10
)

func querySettings(ctx context.Context, dev bluetooth.DeviceCharacteristic, com Command, rec RecordingType, measure MeasureType) ([]Setting, error) {
	msg := make([]byte, settingSize(setCommand{}))
	off := 0
	_, err := setCommand{
		Command: com,
		Record:  rec,
		Measure: measure,
	}.write(msg[off:])
	if err != nil {
		return nil, err
	}
	var settings []byte
	done := make(chan struct{})
	dev.EnableNotifications(func(buf []byte) {
		settings = bytes.Clone(buf)
		close(done)
	})
	dev.WriteWithoutResponse(msg)
	select {
	case <-ctx.Done():
		err = ctx.Err()
	case <-done:
	}
	dev.EnableNotifications(nil)
	if len(settings) < 5 {
		return nil, fmt.Errorf("short response: %#x", settings)
	}
	if settings[0] != 0xf0 || Command(settings[1]) != com {
		return nil, fmt.Errorf("invalid response: %#x", settings)
	}
	if settings[3] != 0 {
		// https://www.bluetooth.com/wp-content/uploads/Files/Specification/HTML/Core-54/out/en/host/attribute-protocol--att-.html#UUID-5a07e398-0e4d-af25-0243-2b45ebfbda5b
		return nil, fmt.Errorf("invalid response: %#x", settings[:5])
	}
	return parseSetting(settings[5:])
}

func parseSetting(data []byte) ([]Setting, error) {
	var settings []Setting
	for len(data) != 0 {
		if uint(data[0]) >= uint(len(settingTypes)) || settingTypes[data[0]].typ == 0 {
			return settings, fmt.Errorf("unknown setting type: %x", data[0])
		}
		var (
			set Setting
			err error
		)
		switch typ := settingTypes[data[0]].typ; typ {
		case uint8Kind:
			var s Uint8
			err = s.UnmarshalBinary(data)
			set = s
		case uint16Kind:
			var s Uint16
			err = s.UnmarshalBinary(data)
			set = s
		case float32Kind:
			var s Float32
			err = s.UnmarshalBinary(data)
			set = s
		default:
			return settings, fmt.Errorf("unknown setting type: %x", data[0])
		}
		if err != nil {
			return settings, err
		}
		data = data[set.Size():]
		settings = append(settings, set)
	}
	return settings, nil
}

func sendCommand(ctx context.Context, dev bluetooth.DeviceCharacteristic, com Command, rec RecordingType, measure MeasureType, settings ...Setting) ([]byte, error) {
	msg := make([]byte, settingSize(setCommand{})+settingSize(settings...))
	off := 0
	n, err := setCommand{
		Command: com,
		Record:  rec,
		Measure: measure,
	}.write(msg[off:])
	if err != nil {
		return nil, err
	}
	off += n
	for _, w := range settings {
		n, err := w.write(msg[off:])
		if err != nil {
			return nil, err
		}
		off += n
	}
	var resp []byte
	done := make(chan struct{})
	dev.EnableNotifications(func(buf []byte) {
		resp = bytes.Clone(buf)
		close(done)
	})
	dev.WriteWithoutResponse(msg)
	select {
	case <-ctx.Done():
		err = ctx.Err()
	case <-done:
	}
	dev.EnableNotifications(nil)
	return resp, err
}

// SettingType specifies PMD measurement settings.
type SettingType uint8

const (
	SampleRateSetting       SettingType = 0
	ResolutionSetting       SettingType = 1
	RangeUnitSetting        SettingType = 2
	ChannelsSetting         SettingType = 4
	ConversionFactorSetting SettingType = 5
)

const headerSize = 2

var settingTypes = [...]struct {
	typ  byte
	n    byte
	size byte
}{
	SampleRateSetting:       {typ: uint16Kind, n: 1, size: uint16Size},
	ResolutionSetting:       {typ: uint16Kind, n: 1, size: uint16Size},
	RangeUnitSetting:        {typ: uint16Kind, n: 1, size: uint16Size},
	ChannelsSetting:         {typ: uint8Kind, n: 1, size: uint8Size},
	ConversionFactorSetting: {typ: float32Kind, n: 1, size: float32Size},
}

const (
	uint8Kind = iota + 1
	uint16Kind
	float32Kind
)

// Defined explicitly to follow spec.
const (
	uint8Size   = 1
	uint16Size  = 2
	int24Size   = 3
	float32Size = 4
)

func settingSize(s ...Setting) int {
	var n int
	for _, t := range s {
		n += t.Size()
	}
	return n
}

// Setting defines the behaviour of PMD measurement settings.
type Setting interface {
	// Size returns the number of bytes the setting
	// writes to the PMD Bluetooth service control
	// point characteristic.
	Size() int

	write([]byte) (int, error)
}

// setCommand specifies the command, and recording and measurement types
// for a control point command.
type setCommand struct {
	Command Command
	Record  RecordingType
	Measure MeasureType
}

func (w setCommand) Size() int { return 2 }

func (w setCommand) write(dst []byte) (int, error) {
	const size = 2
	if len(dst) < size {
		return 0, fmt.Errorf("dst too short")
	}
	dst[0] = byte(w.Command)
	dst[1] = byte(w.Record)<<7 | byte(w.Measure)
	return size, nil
}

// Uint8 is an 8-bit integer setting.
type Uint8 struct {
	Type SettingType
	Val  []uint8
}

func (w Uint8) Size() int      { return w.size(len(w.Val)) }
func (w Uint8) size(n int) int { return headerSize + n*uint8Size }

func (w Uint8) write(dst []byte) (int, error) {
	const size = uint8Size
	n := len(w.Val)
	if uint(w.Type) >= uint(len(settingTypes)) || int(settingTypes[w.Type].n) != n || settingTypes[w.Type].size != size {
		return 0, fmt.Errorf("invalid setting type: %d", w.Type)
	}
	if len(dst) < w.Size() {
		return 0, fmt.Errorf("dst too short")
	}
	dst[0] = byte(w.Type)
	dst[1] = byte(n)
	copy(dst[headerSize:], w.Val)
	return w.Size(), nil
}

func (w *Uint8) UnmarshalBinary(data []byte) error {
	if len(data) < headerSize {
		return io.ErrUnexpectedEOF
	}
	n := int(data[1])
	if len(data) < w.size(n) {
		return io.ErrUnexpectedEOF
	}
	w.Type = SettingType(data[0])
	w.Val = make([]uint8, n)
	copy(w.Val, data[headerSize:])
	return nil
}

// Uint16 is a 16-bit integer setting.
type Uint16 struct {
	Type SettingType
	Val  []uint16
}

func (w Uint16) Size() int      { return w.size(len(w.Val)) }
func (w Uint16) size(n int) int { return headerSize + n*uint16Size }

func (w Uint16) write(dst []byte) (int, error) {
	const size = uint16Size
	n := len(w.Val)
	if uint(w.Type) >= uint(len(settingTypes)) || int(settingTypes[w.Type].n) != n || settingTypes[w.Type].size != size {
		return 0, fmt.Errorf("invalid setting type: %d", w.Type)
	}
	if len(dst) < w.Size() {
		return 0, fmt.Errorf("dst too short")
	}
	l := settingTypes[w.Type]
	if l.size != size {
		return 0, fmt.Errorf("invalid setting type: %d", w.Type)
	}
	dst[0] = byte(w.Type)
	dst[1] = byte(len(w.Val))
	for i, e := range w.Val {
		binary.LittleEndian.PutUint16(dst[headerSize+i*int(l.size):], e)
	}
	return w.Size(), nil
}

func (w *Uint16) UnmarshalBinary(data []byte) error {
	if len(data) < headerSize {
		return io.ErrUnexpectedEOF
	}
	n := int(data[1])
	if len(data) < w.size(n) {
		return io.ErrUnexpectedEOF
	}
	w.Type = SettingType(data[0])
	w.Val = make([]uint16, n)
	data = data[headerSize:]
	for i := range w.Val {
		w.Val[i] = binary.LittleEndian.Uint16(data)
		data = data[uint16Size:]
	}
	return nil
}

// Float32 is a 32-bit floating point setting.
type Float32 struct {
	Type SettingType
	Val  []float32
}

func (w Float32) Size() int      { return w.size(len(w.Val)) }
func (w Float32) size(n int) int { return headerSize + n*float32Size }

func (w Float32) write(dst []byte) (int, error) {
	const size = float32Size
	n := len(w.Val)
	if uint(w.Type) >= uint(len(settingTypes)) || int(settingTypes[w.Type].n) != n || settingTypes[w.Type].size != size {
		return 0, fmt.Errorf("invalid setting type: %d", w.Type)
	}
	if len(dst) < w.Size() {
		return 0, fmt.Errorf("dst too short")
	}
	l := settingTypes[w.Type]
	if l.size != size {
		return 0, fmt.Errorf("invalid setting type: %d", w.Type)
	}
	dst[0] = byte(w.Type)
	dst[1] = byte(len(w.Val))
	for i, e := range w.Val {
		binary.LittleEndian.PutUint32(dst[headerSize+i*int(l.size):], math.Float32bits(e))
	}
	return w.Size(), nil
}

func (w *Float32) UnmarshalBinary(data []byte) error {
	if len(data) < headerSize {
		return io.ErrUnexpectedEOF
	}
	n := int(data[1])
	if len(data) < w.size(n) {
		return io.ErrUnexpectedEOF
	}
	w.Type = SettingType(data[0])
	w.Val = make([]float32, n)
	data = data[headerSize:]
	for i := range w.Val {
		w.Val[i] = math.Float32frombits(binary.LittleEndian.Uint32(data))
		data = data[float32Size:]
	}
	return nil
}

func leInt24(b []byte) int32 {
	_ = b[2] // bounds check hint to compiler; see golang.org/issue/14808
	return int32(b[0]) | int32(b[1])<<8 | int32(int8(b[2]))<<16
}
