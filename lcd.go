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

var trace = false

// Segments.
const (
	S_TL, M_TL = iota, 1 << iota // Top left
	S_TM, M_TM = iota, 1 << iota // Top middle
	S_TR, M_TR = iota, 1 << iota // Top right
	S_BR, M_BR = iota, 1 << iota // Bottom right
	S_BM, M_BM = iota, 1 << iota // Bottom middle
	S_BL, M_BL = iota, 1 << iota // Bottom left
	S_MM, M_MM = iota, 1 << iota // Middle
	SEGMENTS   = iota
)

type sample []point

type segment struct {
	bb     bbox
	points sample
	max    int
}

// Points are all relative to TL position.
type Template struct {
	name string
	line int
	bb   bbox
	off  sample
	dp   sample
	min  int
	mr   point
	ml   point
	tmr  point
	tml  point
	bmr  point
	bml  point
	seg  [SEGMENTS]segment
}

// All points are absolute.
type Digit struct {
	index  int
	pos    point
	bb     bbox
	dp     sample
	min    int
	avgMax int
	tmr    point
	tml    point
	bmr    point
	bml    point
	off    sample
	seg    [SEGMENTS]segment
}

type LcdDecoder struct {
	digits    []*Digit
	templates map[string]*Template
	threshold int
}

// There are 128 possible values in a 7 segment display,
// and this table maps a subset of the values to a string.
const ____ = 0

var resultTable = map[int]byte{
	____ | ____ | ____ | ____ | ____ | ____ | ____: ' ',
	____ | ____ | ____ | ____ | ____ | ____ | M_MM: '-',
	M_TL | M_TM | M_TR | M_BR | M_BM | M_BL | ____: '0',
	____ | ____ | M_TR | M_BR | ____ | ____ | ____: '1',
	____ | M_TM | M_TR | ____ | M_BM | M_BL | M_MM: '2',
	____ | M_TM | M_TR | M_BR | M_BM | ____ | M_MM: '3',
	M_TL | ____ | M_TR | M_BR | ____ | ____ | M_MM: '4',
	M_TL | M_TM | ____ | M_BR | M_BM | ____ | M_MM: '5',
	M_TL | M_TM | ____ | M_BR | M_BM | M_BL | M_MM: '6',
	M_TL | M_TM | M_TR | M_BR | ____ | ____ | ____: '7',
	____ | M_TM | M_TR | M_BR | ____ | ____ | ____: '7',
	M_TL | M_TM | M_TR | M_BR | M_BM | M_BL | M_MM: '8',
	M_TL | M_TM | M_TR | M_BR | M_BM | ____ | M_MM: '9',
	M_TL | M_TM | M_TR | M_BR | ____ | M_BL | M_MM: 'A',
	M_TL | ____ | ____ | M_BR | M_BM | M_BL | M_MM: 'b',
	M_TL | M_TM | ____ | ____ | M_BM | M_BL | ____: 'C',
	____ | ____ | M_TR | M_BR | M_BM | M_BL | M_MM: 'd',
	M_TL | M_TM | ____ | ____ | M_BM | M_BL | M_MM: 'E',
	M_TL | M_TM | ____ | ____ | ____ | M_BL | M_MM: 'F',
	M_TL | ____ | ____ | M_BR | ____ | M_BL | M_MM: 'h',
	M_TL | ____ | M_TR | M_BR | ____ | M_BL | M_MM: 'H',
	M_TL | ____ | ____ | ____ | M_BM | M_BL | ____: 'L',
	M_TL | M_TM | M_TR | M_BR | ____ | M_BL | ____: 'N',
	____ | ____ | ____ | M_BR | ____ | M_BL | M_MM: 'n',
	____ | ____ | ____ | M_BR | M_BM | M_BL | M_MM: 'o',
	M_TL | M_TM | M_TR | ____ | ____ | M_BL | M_MM: 'P',
	____ | ____ | ____ | ____ | ____ | M_BL | M_MM: 'r',
	M_TL | ____ | ____ | ____ | M_BM | M_BL | M_MM: 't',
}

// reverseTable maps a character to the segments that are on.
// Used for calibrating on/off segment values.
var reverseTable map[byte]int = make(map[byte]int)

// Initialise reverse table lookup.
func init() {
	for v, s := range resultTable {
		r, ok := reverseTable[s]
		// If an entry already exists use the one that has least segments.
		if ok {
			if v > r {
				continue
			}
		}
		reverseTable[s] = v
	}
}

func NewLcdDecoder() *LcdDecoder {
	return &LcdDecoder{[]*Digit{}, map[string]*Template{}, defaultThreshold}
}

// Add a template.
func (l *LcdDecoder) AddTemplate(name string, bb []int, dp []int, width int) error {
	if _, ok := l.templates[name]; ok {
		return fmt.Errorf("Duplicate template entry: %s", name)
	}
	if len(bb) != 6 {
		return fmt.Errorf("Invalid bounding box length (expected 6, got %d)", len(bb))
	}
	if len(dp) != 0 && len(dp) != 2 {
		return fmt.Errorf("Invalid decimal point")
	}
	// Prepend the implied TL origin.
	bb = append([]int{0, 0}, bb...)
	t := &Template{name: name, line: width}
	for i := range t.bb {
		t.bb[i].x = bb[i*2]
		t.bb[i].y = bb[i*2+1]
	}
	if len(dp) == 2 {
		t.dp = blockSample(point{dp[0], dp[1]}, (width+1)/2)
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
	offbb1 := innerBB(bbox{t.bb[TL], t.bb[TR], t.bmr, t.bml}, width+offMargin)
	offbb2 := innerBB(bbox{t.tml, t.tmr, t.bb[BR], t.bb[BL]}, width+offMargin)
	t.off = fillBB(offbb1)
	t.off = append(t.off, fillBB(offbb2)...)
	// The assignments must match the bit allocation in
	// the lookup table.
	t.seg[S_TL].bb = segmentBB(t.bb[TL], t.ml, t.bb[TR], t.mr, width)
	t.seg[S_TM].bb = segmentBB(t.bb[TL], t.bb[TR], t.bb[BL], t.bb[BR], width)
	t.seg[S_TR].bb = segmentBB(t.bb[TR], t.mr, t.bb[TL], t.ml, width)
	t.seg[S_BR].bb = segmentBB(t.mr, t.bb[BR], t.ml, t.bb[BL], width)
	t.seg[S_BM].bb = segmentBB(t.bb[BL], t.bb[BR], t.ml, t.mr, width)
	t.seg[S_BL].bb = segmentBB(t.ml, t.bb[BL], t.mr, t.bb[BR], width)
	t.seg[S_MM].bb = segmentBB(t.tml, t.tmr, t.bb[BL], t.bb[BR], width)
	for i := range t.seg {
		t.seg[i].points = fillBB(t.seg[i].bb)
	}
	l.templates[name] = t
	return nil
}

// Add a digit using the named template.
func (l *LcdDecoder) AddDigit(name string, x, y, min, max int) (int, error) {
	t, ok := l.templates[name]
	if !ok {
		return 0, fmt.Errorf("Unknown template %s", name)
	}
	index := len(l.digits)
	d := &Digit{}
	d.index = index
	d.bb = offsetBB(t.bb, x, y)
	d.off = offset(t.off, x, y)
	d.dp = offset(t.dp, x, y)
	d.min = min
	d.avgMax = max
	// Copy over the segment data from the template, offsetting the points
	// using the digit's origin.
	for i := 0; i < SEGMENTS; i++ {
		d.seg[i].bb = offsetBB(t.seg[i].bb, x, y)
		d.seg[i].points = offset(t.seg[i].points, x, y)
		d.seg[i].max = max
	}
	d.dp = offset(t.dp, x, y)
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
		char, found := d.scan(img, l.threshold)
		strs = append(strs, char)
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
	// fmt.Printf("scaled = %d, raw = %d, min = %d, max = %d\n", gscaled, gpscale, min, max)
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
func (l *LcdDecoder) Calibrate(img image.Image, digits string) error {
	if len(digits) != len(l.digits) {
		return fmt.Errorf("Digit count mismatch (digits: %d, calibration: %d", len(digits), len(l.digits))
	}
	for i := range l.digits {
		char := byte(digits[i])
		mask, ok := reverseTable[char]
		if !ok {
			return fmt.Errorf("Unknown digit: %c", char)
		}
		l.digits[i].calibrateDigit(img, mask)
	}
	return nil
}

// Calibrate using one digit, and apply the calibration to all other digits.
func (l *LcdDecoder) CalibrateUsingDigit(img image.Image, digit int, char byte) error {
	if digit < 0 || digit >= len(l.digits) {
		return fmt.Errorf("Digit out of range (max value: %d)", len(l.digits)-1)
	}
	dig := l.digits[digit]
	mask, ok := reverseTable[char]
	if !ok {
		return fmt.Errorf("Unknown digit: %c", char)
	}
	dig.calibrateDigit(img, mask)
	for _, d := range l.digits {
		d.min = dig.min
		for i := range d.seg {
			d.seg[i].max = dig.seg[i].max
		}
	}
	return nil
}

// Mark the segments on this image.
func (l *LcdDecoder) MarkSamples(img *image.RGBA, fill bool) {
	red := color.RGBA{255, 0, 0, 50}
	green := color.RGBA{0, 255, 0, 50}
	white := color.RGBA{255, 255, 255, 255}
	for _, d := range l.digits {
		drawBB(img, d.bb, white)
		ext := sample{d.tmr, d.tml, d.bmr, d.bml}
		drawCross(img, ext, white)
		if fill {
			drawFill(img, d.off, green)
		}
		for i := range d.seg {
			if fill {
				drawFill(img, d.seg[i].points, red)
			}
			//drawBB(img, d.seg[i].bb, green)
		}
		drawFill(img, d.dp, red)
	}
}

// Scan one digit and return the decoded character.
func (d *Digit) scan(img image.Image, threshold int) (string, bool) {
	lookup := 0
	p := make([]int, SEGMENTS)
	//fmt.Printf("Digit %d Max = %d, Min = %d, On = %d, off = %d\n", i, d.calib.max, d.calib.min, threshold, off)
	for i := range d.seg {
		p[i] = scaledSample(img, d.seg[i].points, d.min, d.seg[i].max)
		if p[i] >= threshold {
			lookup |= 1 << uint(i)
		}
	}
	chr, found := resultTable[lookup]
	result := string([]byte{chr})
	// Check for decimal place.
	if len(d.dp) != 0 && scaledSample(img, d.dp, d.min, d.avgMax) >= threshold {
		result = result + "."
	}
	return result, found
}

// Calibrate one digit using an image.
func (d *Digit) calibrateDigit(img image.Image, mask int) {
	if mask == 0 {
		return
	}
	var total, count int
	// Find off average.
	d.min = rawSample(img, d.off)
	for i := range d.seg {
		if ((1 << uint(i)) & mask) != 0 {
			d.seg[i].max = rawSample(img, d.seg[i].points)
			count++
			total += d.seg[i].max
		}
	}
	d.avgMax = total / count
	// For segments that are not included, use an average of the others.
	if mask == ((1 << SEGMENTS) - 1) {
		return
	}
	for i := range d.seg {
		if ((1 << uint(i)) & mask) == 0 {
			d.seg[i].max = d.avgMax
		}
	}
}

func drawBB(img *image.RGBA, b bbox, c color.Color) {
	drawCross(img, b[:], c)
}

func drawFill(img *image.RGBA, s sample, c color.Color) {
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
