// Copyright 2019 Google LLC
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     https://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package lcd

import (
	"image"
)

// Base template for one type/size of 7-segment digit.
// Points are all relative to the top left corner position.
// When a digit is created using this template, the points are
// offset from the point where the digit is placed.
// The idea is that different size of digits use a different
// template, and that multiple digits are created from a single template.
type Template struct {
	name string
	line int
	bb   BBox
	off  PList
	dp   PList
	min  int
	mr   Point
	ml   Point
	tmr  Point
	tml  Point
	bmr  Point
	bml  Point
	seg  [SEGMENTS]segment
}

// Digit represents one 7-segment digit.
// It is typically created from a template, by offsetting the relative
// point values with the absolute point representing the top left of the digit.
// All point values are absolute as a result.
type Digit struct {
	index int
	pos   Point
	bb    BBox
	dp    PList
	tmr   Point
	tml   Point
	bmr   Point
	bml   Point
	off   PList
	lev   *digLevels
	seg   [SEGMENTS]segment
}

// Return true if decimal place is on.
func (d *Digit) decimal(img image.Image) bool {
	return len(d.dp) != 0 && sampleRegion(img, d.dp) >= d.lev.threshold
}

// Calibrate one digit.
func (d *Digit) calibrateDigit(samp []int, off, mask, th int) {
	dl := d.lev
	var tmax, tcount int
	for i := range dl.segLevels {
		if ((1 << uint(i)) & mask) != 0 {
			dl.segLevels[i].max.Add(samp[i])
			tmax += dl.segLevels[i].max.Value
			tcount++
			dl.segLevels[i].min.Set(off)
		} else {
			dl.segLevels[i].min.Add(samp[i])
		}
	}
	// For segments that are not on, set the max to an average of the segments
	// that are on.
	if tcount > 0 {
		for i := range dl.segLevels {
			dl.segLevels[i].max.Set(tmax / tcount)
		}
	}
	var max, min int
	for i := range dl.segLevels {
		dl.segLevels[i].threshold = threshold(dl.segLevels[i].min.Value, dl.segLevels[i].max.Value, th)
		min += dl.segLevels[i].min.Value
		max += dl.segLevels[i].max.Value
	}
	dl.min = min / len(dl.segLevels)
	dl.max = max / len(dl.segLevels)
	dl.threshold = threshold(dl.min, dl.max, th)
}

// Set the min and max for the segments
func (d *Digit) SetMinMax(min, max, th int) {
	d.lev.min = min
	d.lev.max = max
	for i := range d.seg {
		d.lev.segLevels[i].min.Init(min)
		d.lev.segLevels[i].max.Init(max)
		d.lev.segLevels[i].threshold = threshold(min, max, th)
	}
}

// Get the sampled 'off' value (usually the upper and lower blank squares).
func (d *Digit) Off(img image.Image) int {
	return sampleRegion(img, d.off)
}

// Retrieve the list of minimum values for this digit.
func (d *Digit) Min() []int {
	m := make([]int, SEGMENTS, SEGMENTS)
	for i := range d.seg {
		m[i] = d.lev.segLevels[i].min.Value
	}
	return m
}

// Retrieve the list of maximum values for this digit.
func (d *Digit) Max() []int {
	m := make([]int, SEGMENTS, SEGMENTS)
	for i := range d.seg {
		m[i] = d.lev.segLevels[i].max.Value
	}
	return m
}
