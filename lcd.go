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
	"fmt"
	"image"
	"image/color"
)

// Default threshold
const defaultThreshold = 50
const offMargin = 4
const onMargin = 2

var trace = false

// Segments.
const (
	S_TL, M_TL = iota, 1 << iota // Top left
	S_T, M_T   = iota, 1 << iota // Top
	S_TR, M_TR = iota, 1 << iota // Top right
	S_BR, M_BR = iota, 1 << iota // Bottom right
	S_B, M_B   = iota, 1 << iota // Bottom
	S_BL, M_BL = iota, 1 << iota // Bottom left
	S_M, M_M   = iota, 1 << iota // Middle
	SEGMENTS   = iota
)

type sample []point

// Points are all relative to TL position.
type Template struct {
	name     string
	line     int
	bb       bbox
	off      sample
	segbb	 []bbox
	segments []sample
	mr		 point
	ml		 point
	tmr		 point
	tml		 point
	bmr		 point
	bml		 point
}

// Scale holds the calibrated on/off values for each segment.
type Scale struct {
	min []int
	max []int
}

// All points are absolute.
type Digit struct {
	index		int
	pos			point
	min			[]int
	max			[]int
	bb			bbox
	tmr			point
	tml			point
	bmr			point
	bml			point
	off			sample
	segbb	 []bbox
	segments	[]sample
}

type LcdDecoder struct {
	digits    []*Digit
	templates    map[string]*Template
	threshold int
}

// There are 128 possible values in a 7 segment display,
// and this table maps a subset of the values to a string.
const X = 0

var resultTable = map[int]string{
	X | X | X | X | X | X | X:                   " ",
	X | X | X | X | X | X | M_M:                 "-",
	M_TL | M_T | M_TR | M_BR | M_B | M_BL | X:   "0",
	X | X | M_TR | M_BR | X | X | X:             "1",
	X | M_T | M_TR | X | M_B | M_BL | M_M:       "2",
	X | M_T | M_TR | M_BR | M_B | X | M_M:       "3",
	M_TL | X | M_TR | M_BR | X | X | M_M:        "4",
	M_TL | M_T | X | M_BR | M_B | X | M_M:       "5",
	M_TL | M_T | X | M_BR | M_B | M_BL | M_M:    "6",
	M_TL | M_T | M_TR | M_BR | X | X | X:        "7",
	X | M_T | M_TR | M_BR | X | X | X:           "7",
	M_TL | M_T | M_TR | M_BR | M_B | M_BL | M_M: "8",
	M_TL | M_T | M_TR | M_BR | M_B | X | M_M:    "9",
	M_TL | M_T | M_TR | M_BR | X | M_BL | M_M:   "A",
	M_TL | X | X | M_BR | M_B | M_BL | M_M:      "b",
	M_TL | M_T | X | X | M_B | M_BL | X:         "C",
	X | X | M_TR | M_BR | M_B | M_BL | M_M:      "d",
	M_TL | M_T | X | X | M_B | M_BL | M_M:       "E",
	M_TL | M_T | X | X | X | M_BL | M_M:         "F",
	M_TL | X | X | M_BR | X | M_BL | M_M:        "h",
	M_TL | X | M_TR | M_BR | X | M_BL | M_M:     "H",
	M_TL | X | X | X | M_B | M_BL | X:           "L",
	M_TL | M_T | M_TR | M_BR | X | M_BL | X:     "N",
	X | X | X | M_BR | X | M_BL | M_M:           "n",
	X | X | X | M_BR | M_B | M_BL | M_M:         "o",
	M_TL | M_T | M_TR | X | X | M_BL | M_M:      "P",
	X | X | X | X | X | M_BL | M_M:              "r",
	M_TL | X | X | X | M_B | M_BL | M_M:         "t",
}

func NewLcdDecoder() *LcdDecoder {
	return &LcdDecoder{[]*Digit{}, map[string]*Template{}, defaultThreshold}
}

// Add a template.
func (l *LcdDecoder) AddTemplate(name string, points []int, width int) error {
	if _, ok := l.templates[name]; ok {
		return fmt.Errorf("Duplicate template entry: %s", name)
	}
	points = append([]int{0, 0}, points...)
	t := &Template{name: name, line: width}
	for i := range t.bb {
		t.bb[i].x = points[i * 2]
		t.bb[i].y = points[i * 2 + 1]
	}
	// Initialise the sample lists
	// Middle points.
	t.mr = split(t.bb[TR], t.bb[BR], 2)[0]
	t.tmr = adjust(t.mr, t.bb[TR], width/2)
	t.bmr = adjust(t.mr, t.bb[BR], width/2)
	t.ml = split(t.bb[TL], t.bb[BL], 2)[0]
	t.tml = adjust(t.ml, t.bb[TL], width/2)
	t.bml = adjust(t.ml, t.bb[BL], width/2)
	// Build the 'off' sample using the middle blocks.
	offbb1 := innerBB(bbox{t.bb[TL], t.bb[TR], t.bmr, t.bml}, width + offMargin)
	offbb2 := innerBB(bbox{t.tml, t.tmr, t.bb[BR], t.bb[BL]}, width + offMargin)
	t.off = fillBB(offbb1)
	t.off = append(t.off, fillBB(offbb2)...)
	t.segbb = make([]bbox, SEGMENTS, SEGMENTS)
	t.segments = make([]sample, SEGMENTS, SEGMENTS)
	// The assignments must match the bit allocation in
	// the lookup table.
	t.segbb[S_TL] = segmentBB(t.bb[TL], t.ml, t.bb[TR], t.mr, width)
	t.segbb[S_T] = segmentBB(t.bb[TL], t.bb[TR], t.bb[BL], t.bb[BR], width)
	t.segbb[S_TR] = segmentBB(t.bb[TR], t.mr, t.bb[TL], t.ml, width)
	t.segbb[S_BR] = segmentBB(t.mr, t.bb[BR], t.ml, t.bb[BL], width)
	t.segbb[S_B] = segmentBB(t.bb[BL], t.bb[BR], t.ml, t.mr, width)
	t.segbb[S_BL] = segmentBB(t.ml, t.bb[BL], t.mr, t.bb[BR], width)
	t.segbb[S_M] = segmentBB(t.tml, t.tmr, t.bb[BL], t.bb[BR], width)
	for i := range t.segbb {
		t.segments[i] = fillBB(t.segbb[i])
	}
	l.templates[name] = t
	return nil
}

// Add a digit using the named template.
func (l *LcdDecoder) AddDigit(name string, x, y, min, max int) (int, error) {
	t, ok := l.templates[name]
	if !ok {
		return 0, fmt.Errorf("Unknown LCD %s", name)
	}
	index := len(l.digits)
	d := &Digit{}
	d.index = index
	d.min = make([]int, SEGMENTS, SEGMENTS)
	d.max = make([]int, SEGMENTS, SEGMENTS)
	d.bb = offsetBB(t.bb, x, y)
	d.off = offset(t.off, x, y)
	d.segments = make([]sample, SEGMENTS, SEGMENTS)
	d.segbb = make([]bbox, SEGMENTS, SEGMENTS)
	for i := 0; i < SEGMENTS; i++ {
		d.min[i] = min
		d.max[i] = max
		d.segments[i] = offset(t.segments[i], x, y)
		d.segbb[i] = offsetBB(t.segbb[i], x, y)
	}
	d.tmr.x = t.tmr.x + x
	d.tmr.y = t.tmr.y + y
	d.tml.x = t.tml.x + x
	d.tml.y = t.tml.y + y
	d.bmr.x = t.bmr.x + x
	d.bmr.y = t.bmr.y + y
	d.bml.x = t.bml.x + x
	d.bml.y = t.bml.y + y
	l.digits = append(l.digits, d)
	return index, nil
}

func (l *LcdDecoder) SetThreshold(threshold int) {
	l.threshold = threshold
}

func (l *LcdDecoder) Decode(img image.Image) ([]string, []bool) {
	strs := []string{}
	ok := []bool{}
	for _, d := range l.digits {
		// Find off point.
		// off := scaledSample(img, d.off, 0, 0x10000)
		lookup := 0
		p := make([]int, SEGMENTS)
		on := l.threshold
		//fmt.Printf("Digit %d Max = %d, Min = %d, On = %d, off = %d\n", i, d.max, d.min, on, off)
		for seg, s := range d.segments {
			p[seg] = scaledSample(img, s, d.min[seg], d.max[seg])
			if p[seg] >= on {
				lookup |= 1 << uint(seg)
			}
		}
		result, found := resultTable[lookup]
		//if !found {
		//fmt.Printf("Element not found, on = %d, off = %d, pixels: %v\n", on, off, p)
		//}
		strs = append(strs, result)
		ok = append(ok, found)
	}
	return strs, ok
}

// Return an average of the sampled points as a int
// between 0 and 100, where 0 is lightest and 100 is darkest using
// the scale provided.
func scaledSample(img image.Image, slist sample, min, max int) int {
	gscaled := rawSample(img, slist)
	if gscaled < min {
		gscaled = min
	}
	if gscaled >= max {
		gscaled = max - 1
	}
	gpscale := (gscaled - min) * 100 / (max - min)
	//fmt.Printf("grey = %d, len = %d, result = %d, (%d%%)\n", gacc, len(slist), gscaled, gpscale)
	return gpscale
}

// Take a raw sample.
func rawSample(img image.Image, slist sample) int {
	var gacc int
	for _, s := range slist {
		c := img.At(s.x, s.y)
		pix := color.Gray16Model.Convert(c).(color.Gray16)
		gacc += int(pix.Y)
	}
	return 0x10000 - gacc/len(slist)
}

// Calibrate calculates the on and off values from the image provided.
func (l *LcdDecoder) Calibrate(img image.Image) {
	for _, d := range l.digits {
		// Find off point.
		min := rawSample(img, d.off)
		for seg, s := range d.segments {
			d.min[seg] = min
			d.max[seg] = rawSample(img, s)
		}
	}
}

// Mark the samples with a red cross.
func (l *LcdDecoder) MarkSamples(img *image.RGBA) {
	red := color.RGBA{255, 0, 0, 50}
	green := color.RGBA{0, 255, 0, 50}
	white := color.RGBA{255, 255, 255, 255}
	for _, d := range l.digits {
		drawBB(img, d.bb, white)
		ext := sample{d.tmr, d.tml, d.bmr, d.bml}
		drawCross(img, ext, white)
		drawPoint(img, d.off, green)
		for i := range d.segments {
			drawPoint(img, d.segments[i], red)
			//drawBB(img, d.segbb[i], green)
		}
	}
}

func drawBB(img *image.RGBA, b bbox, c color.Color) {
	drawCross(img, b[:], c)
}

func drawPoint(img *image.RGBA, s sample, c color.Color) {
	for _, p := range s {
		img.Set(p.x, p.y, c)
	}
}
func drawCross(img *image.RGBA, s sample, c color.Color) {
	for _, p := range s {
		x := p.x
		y := p.y
		img.Set(x, y, c)
		for i := 1; i < 3; i++ {
			img.Set(x-i, y, c)
			img.Set(x+i, y, c)
			img.Set(x, y-i, c)
			img.Set(x, y+i, c)
		}
	}
}

func printSamples(s []point) {
	for _, p := range s {
		fmt.Printf("x = %d, y = %d\n", p.x, p.y)
	}
}
