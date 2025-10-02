// Copyright ©2025 Dan Kortschak. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package pmd

import (
	"encoding/binary"
	"fmt"
	"time"
)

const (
	AccSampleFreq25      AccSampleFreq = 25 // Hz
	AccSampleInterval25                = time.Second / time.Duration(AccSampleFreq25)
	AccSampleFreq50      AccSampleFreq = 50 // Hz
	AccSampleInterval50                = time.Second / time.Duration(AccSampleFreq50)
	AccSampleFreq100     AccSampleFreq = 100 // Hz
	AccSampleInterval100               = time.Second / time.Duration(AccSampleFreq100)
	AccSampleFreq200     AccSampleFreq = 200 // Hz
	AccSampleInterval200               = time.Second / time.Duration(AccSampleFreq200)

	AccRange2G AccRange = 2 // G
	AccRange4G AccRange = 4 // G
	AccRange8G AccRange = 8 // G
)

type AccSampleFreq uint16

type AccRange uint16

// AccHandler implements the Handler interface for accelerometer data.
type AccHandler struct {
	// SampleFreq is the sample frequency to use.
	SampleFreq AccSampleFreq
	// Range is the acceleration resolution to use.
	Range AccRange

	// Handler is called for each notification.
	Handler func([]byte)
}

func (h AccHandler) Handle() (Command, MeasureType, []Setting, func([]byte)) {
	if h.Handler == nil {
		return MeasureStop, AccType, nil, nil
	}
	return MeasureStart, AccType, []Setting{
		Uint16{Type: SampleRateSetting, Val: []uint16{uint16(h.SampleFreq)}}, // Hz
		Uint16{Type: ResolutionSetting, Val: []uint16{16}},                   // bits
		Uint16{Type: RangeUnitSetting, Val: []uint16{uint16(h.Range)}},       // G
	}, h.Handler
}

// Acc is an acceleration measurement.
type Acc struct {
	Timestamp time.Time
	X, Y, Z   int32
}

func (m *Acc) UnmarshalBinary(data []byte) error {
	if MeasureType(data[sampleTypeOffset]) != AccType {
		return fmt.Errorf("expected sample type acc: %v", data[sampleTypeOffset])
	}
	var x, y, z int32
	switch FrameType(data[frameTypeOffset]) {
	case AccFrameType0:
		x = int32(int8(data[dataOffset]))
		y = int32(int8(data[dataOffset+1]))
		z = int32(int8(data[dataOffset+2]))
	case AccFrameType1:
		x = int32(int16(binary.LittleEndian.Uint16(data[dataOffset:])))
		y = int32(int16(binary.LittleEndian.Uint16(data[dataOffset+uint16Size:])))
		z = int32(int16(binary.LittleEndian.Uint16(data[dataOffset+2*uint16Size:])))
	case AccFrameType2:
		x = leInt24(data[dataOffset:])
		y = leInt24(data[dataOffset+int24Size:])
		z = leInt24(data[dataOffset+2*int24Size:])
	default:
		return fmt.Errorf("expected frame type acc0/acc1/acc2: %v", data[frameTypeOffset])
	}

	timestamp := binary.LittleEndian.Uint64(data[timeStampOffset:])

	*m = Acc{
		Timestamp: time.Unix(int64(timestamp)/1e9+epoch, int64(timestamp)%1e9),

		X: x, Y: y, Z: z,
	}
	return nil
}
