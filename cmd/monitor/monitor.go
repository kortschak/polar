// Copyright ©2025 Dan Kortschak. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

import (
	"context"
	"errors"
	"fmt"
	"image"
	"log"
	"sync"
	"sync/atomic"
	"time"

	"tinygo.org/x/bluetooth"

	"github.com/kortschak/polar/cmd/internal/ring"
	"github.com/kortschak/polar/heart"
	"github.com/kortschak/polar/pmd"
)

type monitor struct {
	heart  *heart.RateListener
	pmd    *pmd.Listener
	cancel context.CancelFunc
}

func newMonitor(ctx context.Context, dev bluetooth.Device, update chan image.Image) (*monitor, error) {
	l, err := pmd.NewListener(&dev)
	if err != nil {
		return nil, fmt.Errorf("failed to create listener: %v", err)
	}
	fmt.Printf("supported feature: %s\n", l.Features())

	card := image.NewGray(image.Rectangle{Max: image.Point{X: 296, Y: 128}})
	blank(card)

	hrStats := newHeartRate(subDrawImage(card, image.Rectangle{
		Min: image.Point{X: 0, Y: 0},
		Max: image.Point{X: 64, Y: 64},
	}))
	ecg := newECGPlot(subDrawImage(card, image.Rectangle{
		Min: image.Point{X: 0, Y: 64},
		Max: image.Point{X: 296, Y: 128},
	}))
	history := newRateHistory(time.Minute, subDrawImage(card, image.Rectangle{
		Min: image.Point{X: 64, Y: 0},
		Max: image.Point{X: 296, Y: 64},
	}))

	timeout, cancelTimeout := context.WithTimeout(context.Background(), time.Second)
	settings, err := l.Settings(timeout, pmd.ECGType)
	cancelTimeout()
	if err != nil {
		l.Close()
		return nil, fmt.Errorf("error getting settings from Polar H10: %v", err)
	}
	fmt.Printf("ecg settings: %+v\n", settings)

	var ok atomic.Bool
	var muECG sync.Mutex
	traceRing := ring.NewBuffer[int32](3 * pmd.ECGSampleFreq)
	ecgTick := make(chan time.Time)
	timeout, cancelTimeout = context.WithTimeout(ctx, time.Second)
	_, err = l.SetHandler(timeout, pmd.ECGHandler(func(buf []byte) {
		if !ok.Load() {
			return
		}
		var ecg pmd.ECG
		err := ecg.UnmarshalBinary(buf)
		if err != nil {
			log.Printf("failed to get ecg measurement: %v", err)
			return
		}
		muECG.Lock()
		traceRing.Write(ecg.Trace)
		select {
		case ecgTick <- ecg.Timestamp:
		default:
		}
		muECG.Unlock()
	}))
	cancelTimeout()
	if err != nil {
		l.Close()
		return nil, fmt.Errorf("error occurred during ecg streaming from Polar H10: %v", err)
	}

	hrTick := make(chan heart.Rate, 1)
	hr, err := heart.NewRateListener(&dev, func(hr heart.Rate, err error) {
		ok.Store(hr.Contact)
		if err != nil {
			log.Printf("failed to get hr measurement: %v", err)
			return
		}
		select {
		case hrTick <- hr:
		default:
		}
	})
	if err != nil {
		l.Close()
		return nil, fmt.Errorf("failed to start streaming hr: %w", err)
	}

	ctx, cancel := context.WithCancel(ctx)
	go func() {
		hrRing := ring.NewBuffer[uint16](130)
		for {
			select {
			case <-ctx.Done():
				return
			case hr := <-hrTick:
				hrStats.add(int(hr.HR), hr.RR...)
				hrRing.Write([]uint16{hr.HR})
				history.add(time.Now(), hrRing)
				update <- card
			case <-ecgTick:
				muECG.Lock()
				if traceRing.Len() < ecg.width() {
					muECG.Unlock()
					continue
				}
				ecg.add(traceRing)
				muECG.Unlock()
			}
		}
	}()

	return &monitor{
		pmd:    l,
		heart:  hr,
		cancel: cancel,
	}, nil
}

func (m *monitor) Close() error {
	err := errors.Join(m.pmd.Close(), m.heart.Close())
	m.cancel()
	return err
}
