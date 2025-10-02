// Copyright ©2025 Dan Kortschak. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package main

import (
	"image"
	"image/color"
	"image/draw"
)

type subImager interface {
	draw.Image
	SubImage(image.Rectangle) image.Image
}

func subDrawImage(img subImager, rect image.Rectangle) draw.Image {
	return drawOffset{
		Image:  img.SubImage(rect).(draw.Image),
		offset: rect.Min,
	}
}

type drawOffset struct {
	draw.Image
	offset image.Point
}

func (i drawOffset) Set(x, y int, c color.Color) {
	i.Image.Set(x+i.offset.X, y+i.offset.Y, c)
}

func (i drawOffset) At(x, y int) color.Color {
	return i.Image.At(x-i.offset.X, y-i.offset.Y)
}

type number interface{ int32 | uint16 }

func scale[T number](v, min, max, minRange T, height int) int {
	v -= min
	spread := max - min
	var offset T = 0
	if spread < minRange {
		offset = (minRange - spread) / 2
		spread = minRange
	}
	return int(float64(v+offset) / float64(spread) * float64(height))
}

func line(img draw.Image, x0, y0, x1, y1 int, c color.Color) {
	switch {
	case x0 == x1:
		if y0 > y1 {
			y0, y1 = y1, y0
		}
		for ; y0 <= y1; y0++ {
			img.Set(x0, y0, c)
		}
	case y0 == y1:
		if x0 > x1 {
			x0, x1 = x1, x0
		}
		for ; x0 <= x1; x0++ {
			img.Set(x0, y0, c)
		}
	default:
		bresenham(img, x0, y0, x1, y1, c)
	}
}

func bresenham(img draw.Image, x0, y0, x1, y1 int, c color.Color) {
	dx, sx := absSign(x1 - x0)
	dy, sy := absSign(y1 - y0)
	dy = -dy
	err := dx + dy
	for {
		img.Set(x0, y0, c)
		e2 := 2 * err
		if e2 >= dy {
			if x0 == x1 {
				return
			}
			err += dy
			x0 += sx
		}
		if e2 <= dx {
			if y0 == y1 {
				return
			}
			err += dx
			y0 += sy
		}
	}
}

func absSign(a int) (abs, sign int) {
	if a < 0 {
		return -a, -1
	}
	return a, 1
}

func blank(img draw.Image) {
	b := img.Bounds()
	dx := b.Dx()
	dy := b.Dy()
	for x := range dx {
		for y := range dy {
			img.Set(x, y, color.White)
		}
	}
}

type displayShim struct {
	// ¯\_(ツ)_/¯
	img draw.Image
}

func (d displayShim) SetPixel(x, y int16, c color.RGBA) {
	d.img.Set(int(x), int(y), c)
}

func (d displayShim) Size() (x, y int16) {
	b := d.img.Bounds()
	return int16(b.Dx()), int16(b.Dy())
}

func (d displayShim) Display() error { return nil }
