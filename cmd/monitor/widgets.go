// Copyright ©2025 Dan Kortschak. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

import (
	"image/color"
	"image/draw"
	"strconv"
	"time"

	"tinygo.org/x/tinyfont"
	"tinygo.org/x/tinyfont/freesans"

	"github.com/kortschak/polar/cmd/internal/ring"
)

type heartRate struct {
	img    draw.Image
	lastRR time.Duration
}

func newHeartRate(img draw.Image) *heartRate {
	return &heartRate{img: img}
}

func (r *heartRate) add(hr int, rrs ...time.Duration) {
	blank(r.img)

	width := r.img.Bounds().Dx()
	yOffset := -10

	hrText := strconv.Itoa(hr)
	hrFont := &freesans.Bold18pt7b
	_, hrW := tinyfont.LineWidth(hrFont, hrText)
	tinyfont.WriteLine(
		displayShim{r.img},
		hrFont,
		int16(width-int(hrW))/2, int16(int(hrFont.YAdvance)+yOffset), hrText,
		color.RGBA{A: 0xff},
	)

	if len(rrs) != 0 {
		r.lastRR = 0
		for _, v := range rrs {
			r.lastRR += v
		}
		r.lastRR /= time.Duration(len(rrs))
	}
	rrText := "-"
	if r.lastRR != 0 {
		rrText = r.lastRR.Round(time.Millisecond).String()
	}
	rrFont := &freesans.Regular9pt7b
	_, rrW := tinyfont.LineWidth(rrFont, rrText)
	tinyfont.WriteLine(
		displayShim{r.img},
		rrFont,
		int16(width-int(rrW))/2, int16(int(rrFont.YAdvance)+int(hrFont.YAdvance)+yOffset), rrText,
		color.RGBA{A: 0xff},
	)
}

type rateHistory struct {
	ring *ring.Buffer[uint16]
	wait time.Duration
	last time.Time
	img  draw.Image
	buf  []uint16
}

func newRateHistory(period time.Duration, img draw.Image) *rateHistory {
	return &rateHistory{
		ring: ring.NewBuffer[uint16](img.Bounds().Dx()),
		wait: period,
		img:  img,
	}
}

func (h *rateHistory) add(ts time.Time, rates *ring.Buffer[uint16]) {
	if h.last.IsZero() {
		h.last = ts
		return
	}
	if ts.Sub(h.last) <= h.wait {
		return
	}
	h.last = ts
	if len(h.buf) < rates.Size() {
		h.buf = make([]uint16, rates.Size())
	}
	n := rates.Read(h.buf)
	var s float64
	for _, v := range h.buf[:n] {
		s += float64(v)
	}
	h.ring.Write([]uint16{uint16(s / float64(n))})

	h.plot()
}

func (h *rateHistory) plot() {
	if len(h.buf) < h.ring.Size() {
		h.buf = make([]uint16, h.ring.Size())
	}

	blank(h.img)

	n := h.ring.CopyTo(h.buf)
	for i, v := range h.buf[:n] {
		h.buf[i] = -v
	}
	min := h.buf[0]
	max := min
	for _, v := range h.buf[1:n] {
		if v < min {
			min = v
		}
		if v > max {
			max = v
		}
	}
	const minRange = 20
	height := h.img.Bounds().Dy()
	for i, v := range h.buf[1:n] {
		line(h.img, i, scale(h.buf[i], min, max, minRange, height), i+1, scale(v, min, max, minRange, height), color.Black)
	}
}

type ecgPlot struct {
	img draw.Image
	buf []int32
}

func newECGPlot(img draw.Image) *ecgPlot {
	return &ecgPlot{
		img: img,
		buf: make([]int32, img.Bounds().Dx()),
	}
}

func (p *ecgPlot) width() int {
	return p.img.Bounds().Dx()
}

func (p *ecgPlot) add(r *ring.Buffer[int32]) {
	if r.Len() < p.width() {
		return
	}
	r.CopyTo(p.buf)
	plotECG(p.img, p.buf)
}

func plotECG(dst draw.Image, trace []int32) {
	blank(dst)

	for i, v := range trace {
		trace[i] = -v
	}
	min := trace[0]
	max := min
	for _, v := range trace[1:] {
		if v < min {
			min = v
		}
		if v > max {
			max = v
		}
	}
	const minRange = 1200
	height := dst.Bounds().Dy()
	for i, v := range trace[1:] {
		line(dst, i, scale(trace[i], min, max, minRange, height), i+1, scale(v, min, max, minRange, height), color.Black)
	}
}
