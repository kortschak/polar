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
	ECGSampleFreq     = 130 // Hz
	ECGSampleInterval = time.Second / ECGSampleFreq

	ECGResolution = 14 // bits
)

// ECGHandler implements the Handler interface for ECG data.
// The function is called for each notification.
type ECGHandler func([]byte)

func (h ECGHandler) Handle() (Command, MeasureType, []Setting, func([]byte)) {
	if h == nil {
		return MeasureStop, ECGType, nil, nil
	}
	return MeasureStart, ECGType, []Setting{
		Uint16{Type: SampleRateSetting, Val: []uint16{ECGSampleFreq}},
		Uint16{Type: ResolutionSetting, Val: []uint16{ECGResolution}},
	}, h
}

// ECG is an ECG measurement.
type ECG struct {
	Timestamp time.Time
	Trace     []int32 // µV
}

func (m *ECG) UnmarshalBinary(data []byte) error {
	if MeasureType(data[sampleTypeOffset]) != ECGType {
		return fmt.Errorf("expected sample type ecg: %v", data[sampleTypeOffset])
	}
	if FrameType(data[frameTypeOffset]) != ECGFrameType0 {
		return fmt.Errorf("expected frame type ecg: %v", data[frameTypeOffset])
	}

	timestamp := binary.LittleEndian.Uint64(data[timeStampOffset:])

	trace := data[dataOffset:]
	if len(trace)%ECGSamplingStride != 0 {
		return fmt.Errorf("number of samples not a factor of 3: %v", len(trace)%3)
	}

	ecgTrace := make([]int32, 0, len(trace)/3)
	for i := 0; i < len(trace); i += ECGSamplingStride {
		ecgTrace = append(ecgTrace, leInt24(trace[i:i+ECGSamplingStride]))
	}

	*m = ECG{
		Timestamp: time.Unix(int64(timestamp)/1e9+epoch, int64(timestamp)%1e9),
		Trace:     ecgTrace,
	}
	return nil
}
