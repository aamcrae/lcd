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
	"bufio"
	"flag"
	"fmt"
	"image"
	"image/color"
	"io"
	"log"
	"strconv"
	"strings"
)

var history = flag.Int("history", 5, "Size of history cache")

// Default threshold
const defaultThreshold = 50
const offMargin = 5
const onMargin = 2

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

type DigitScan struct {
	Char     byte
	Str      string
	Valid    bool
	DP       bool
	Mask     int
	Segments []int
}

type ScanResult struct {
	img     image.Image
	Text    string
	Invalid int
	Digits  []DigitScan
}

type sample []point

type segment struct {
	bb        bbox
	points    sample
	max       *Avg
	min       *Avg
	threshold int
}

// Base template for one type of digit.
// Points are all relative to top left corner position.
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
	index     int
	pos       point
	bb        bbox
	dp        sample
	tmr       point
	tml       point
	bmr       point
	bml       point
	off       sample
	min       int
	max       int
	threshold int
	seg       [SEGMENTS]segment
}

type LcdDecoder struct {
	Digits    []*Digit
	templates map[string]*Template
	Threshold int
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
// This is used in calibration to map
// a character to the segments representing that character.
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

// Add a digit template. Each template describes the parameters of a type of digit.
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
	t.seg[S_TL].bb = segmentBB(t.bb[TL], t.ml, t.bb[TR], t.mr, width, onMargin)
	t.seg[S_TM].bb = segmentBB(t.bb[TL], t.bb[TR], t.bb[BL], t.bb[BR], width, onMargin)
	t.seg[S_TR].bb = segmentBB(t.bb[TR], t.mr, t.bb[TL], t.ml, width, onMargin)
	t.seg[S_BR].bb = segmentBB(t.mr, t.bb[BR], t.ml, t.bb[BL], width, onMargin)
	t.seg[S_BM].bb = segmentBB(t.bb[BL], t.bb[BR], t.ml, t.mr, width, onMargin)
	t.seg[S_BL].bb = segmentBB(t.ml, t.bb[BL], t.mr, t.bb[BR], width, onMargin)
	t.seg[S_MM].bb = segmentBB(t.tml, t.tmr, t.bb[BL], t.bb[BR], width, onMargin)
	for i := range t.seg {
		t.seg[i].points = fillBB(t.seg[i].bb)
	}
	l.templates[name] = t
	return nil
}

// Add a digit using the named template. The template points are offset
// by the point location of the digit.
func (l *LcdDecoder) AddDigit(name string, x, y int) (*Digit, error) {
	t, ok := l.templates[name]
	if !ok {
		return nil, fmt.Errorf("Unknown template %s", name)
	}
	index := len(l.Digits)
	d := &Digit{}
	d.index = index
	d.bb = offsetBB(t.bb, x, y)
	d.off = offset(t.off, x, y)
	d.dp = offset(t.dp, x, y)
	// Copy over the segment data from the template, offsetting the points
	// using the digit's origin.
	for i := 0; i < SEGMENTS; i++ {
		d.seg[i].bb = offsetBB(t.seg[i].bb, x, y)
		d.seg[i].points = offset(t.seg[i].points, x, y)
		d.seg[i].min = NewAvg(*history)
		d.seg[i].max = NewAvg(*history)
		d.seg[i].threshold = threshold(d.seg[i].min.Value, d.seg[i].max.Value, l.Threshold)
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
	l.Digits = append(l.Digits, d)
	return d, nil
}

func (l *LcdDecoder) SetThreshold(th int) {
	l.Threshold = th
}

// Decode the LCD digits in the image.
func (l *LcdDecoder) Decode(img image.Image) *ScanResult {
	var res ScanResult
	res.img = img
	var str []byte
	for _, d := range l.Digits {
		var ds DigitScan
		ds.Segments = make([]int, SEGMENTS, SEGMENTS)
		for i := range ds.Segments {
			ds.Segments[i] = sampleRegion(img, d.seg[i].points)
			if ds.Segments[i] >= d.seg[i].threshold {
				ds.Mask |= 1 << uint(i)
			}
		}
		ds.Char, ds.Valid = resultTable[ds.Mask]
		if ds.Valid {
			ds.Str = string([]byte{ds.Char})
		} else {
			res.Invalid++
		}
		str = append(str, ds.Char)
		// Check for decimal place.
		if d.decimal(img) {
			ds.DP = true
			str = append(str, '.')
		}
		res.Digits = append(res.Digits, ds)
	}
	res.Text = string(str)
	return &res
}

// Calibrate calculates the on and off values from the image provided.
func (l *LcdDecoder) CalibrateImage(img image.Image, digits string) error {
	if len(digits) != len(l.Digits) {
		return fmt.Errorf("Digit count mismatch (digits: %d, calibration: %d", len(digits), len(l.Digits))
	}
	res := l.Decode(img)
	for i := range l.Digits {
		char := byte(digits[i])
		mask, ok := reverseTable[char]
		if !ok {
			return fmt.Errorf("Unknown digit: 0x%02x", char)
		}
		off := sampleRegion(img, l.Digits[i].off)
		l.Digits[i].calibrateDigit(res.Digits[i].Segments, off, mask, l.Threshold)
	}
	return nil
}

// Calibrate using a previous scan.
func (l *LcdDecoder) CalibrateScan(scan *ScanResult) error {
	if len(scan.Digits) != len(l.Digits) {
		return fmt.Errorf("Digit count mismatch (digits: %d, calibration: %d", len(scan.Digits), len(l.Digits))
	}
	for i := range l.Digits {
		off := sampleRegion(scan.img, l.Digits[i].off)
		l.Digits[i].calibrateDigit(scan.Digits[i].Segments, off, scan.Digits[i].Mask, l.Threshold)
	}
	return nil
}

// Mark the segments on this image.
func (l *LcdDecoder) MarkSamples(img *image.RGBA, fill bool) {
	red := color.RGBA{255, 0, 0, 50}
	green := color.RGBA{0, 255, 0, 50}
	white := color.RGBA{255, 255, 255, 255}
	for _, d := range l.Digits {
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

// Restore the calibration data from a saved cache.
// Format is a line of CSV:
// digit,segment,min,max
func (l *LcdDecoder) RestoreCalibration(r io.Reader) {
	scanner := bufio.NewScanner(r)
	var line int
	for scanner.Scan() {
		line++
		var v []int
		tok := strings.Split(scanner.Text(), ",")
		for _, s := range tok {
			if val, err := strconv.ParseInt(s, 10, 32); err != nil {
				log.Printf("RestoreCalibration: line %d: %v", line, err)
				break
			} else {
				v = append(v, int(val))
			}
		}
		if len(v) != 4 {
			log.Printf("RestoreCalibration: line %d, too few fields (%d)", line, len(v))
			continue
		}
		if v[0] < 0 || v[0] >= len(l.Digits) {
			log.Printf("RestoreCalibration: line %d, out of range digit (%d)", line, v[0])
			continue
		}
		if v[1] < 0 || v[1] >= SEGMENTS {
			log.Printf("RestoreCalibration: line %d, out of range segment (%d)", line, v[1])
			continue
		}
		s := &l.Digits[v[0]].seg[v[1]]
		s.min.Init(v[2])
		s.max.Init(v[3])
		s.threshold = threshold(s.min.Value, s.max.Value, l.Threshold)
	}
	for _, d := range l.Digits {
		var min, max int
		for i := range d.seg {
			min += d.seg[i].min.Value
			max += d.seg[i].max.Value
		}
		// Keep the average of the min and max.
		d.min = min / len(d.seg)
		d.max = max / len(d.seg)
		d.threshold = threshold(d.min, d.max, l.Threshold)
	}
}

// Save the calibration data.
func (l *LcdDecoder) SaveCalibration(w io.WriteCloser) {
	for i, d := range l.Digits {
		for s := range d.seg {
			fmt.Fprintf(w, "%d,%d,%d,%d\n", i, s, d.seg[s].min.Value, d.seg[s].max.Value)
		}
	}
}

// Return true if decimal place is on.
func (d *Digit) decimal(img image.Image) bool {
	return len(d.dp) != 0 && sampleRegion(img, d.dp) >= d.threshold
}

// Calibrate one digit.
func (d *Digit) calibrateDigit(samp []int, off, mask, th int) {
	var tmax, tcount int
	for i := range d.seg {
		if ((1 << uint(i)) & mask) != 0 {
			d.seg[i].max.Add(samp[i])
			tmax += d.seg[i].max.Value
			tcount++
			d.seg[i].min.Set(off)
		} else {
			d.seg[i].min.Add(samp[i])
		}
	}
	// For segments that are not on, set the max to an average of the segments
	// that are on.
	if tcount > 0 {
		for i := range d.seg {
			d.seg[i].max.Set(tmax / tcount)
		}
	}
	var max, min int
	for i := range d.seg {
		d.seg[i].threshold = threshold(d.seg[i].min.Value, d.seg[i].max.Value, th)
		min += d.seg[i].min.Value
		max += d.seg[i].max.Value
	}
	d.min = min / len(d.seg)
	d.max = max / len(d.seg)
	d.threshold = threshold(d.min, d.max, th)
}

// Set the min and max for the segments
func (d *Digit) SetMinMax(min, max, th int) {
	d.min = min
	d.max = max
	for i := range d.seg {
		d.seg[i].min.Init(min)
		d.seg[i].max.Init(max)
		d.seg[i].threshold = threshold(min, max, th)
	}
}

func (d *Digit) Min() []int {
	m := make([]int, SEGMENTS, SEGMENTS)
	for i := range d.seg {
		m[i] = d.seg[i].min.Value
	}
	return m
}

func (d *Digit) Max() []int {
	m := make([]int, SEGMENTS, SEGMENTS)
	for i := range d.seg {
		m[i] = d.seg[i].max.Value
	}
	return m
}

// Get the sampled off value.
func (d *Digit) Off(img image.Image) int {
	return sampleRegion(img, d.off)
}

// Calculate the threshold using a percentage.
func threshold(min, max, perc int) int {
	return min + (max-min)*perc/100
}

func DigitsToSegments(s string) ([]int, error) {
	var b []int
	for i, c := range s {
		mask, ok := reverseTable[byte(c)]
		if !ok {
			return nil, fmt.Errorf("Unknown character (#%d - %c)", i, c)
		}
		b = append(b, mask)
	}
	return b, nil
}

// Sample the points. Each point is converted to grayscale and averaged
// across all the points. The result is inverted so that darker values are higher.
func sampleRegion(img image.Image, slist sample) int {
	var gacc int
	for _, s := range slist {
		c := img.At(s.x, s.y)
		pix := color.Gray16Model.Convert(c).(color.Gray16)
		gacc += int(pix.Y)
	}
	return 0x10000 - gacc/len(slist)
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
