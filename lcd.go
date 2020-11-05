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
	"flag"
	"fmt"
	"image"
	"image/color"
)

var history = flag.Int("history", 5, "Size of history cache")
var levelSize = flag.Int("level_size", 100, "Size of level map")
var savedLevels = flag.Int("level_saved", 50, "Number of levels saved")

// Default threshold and margins.
const defaultThreshold = 50
const offMargin = 5
const onMargin = 2

// Segments, as enum and bit mask.
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

// DigitScan is the result of scanning one digit in the image.
type DigitScan struct {
	Char     byte   // The decoded character
	Str      string // The decoded char as a string
	Valid    bool   // True if the decode was successful
	DP       bool   // True if the decimal point is set
	Mask     int    // The segments that are set
	Segments []int  // The summed average of the segment points.
}

// ScanResult contains the results of decoding one image.
type ScanResult struct {
	img     image.Image // Image that has been decoded
	Text    string      // Decoded string of digits
	Invalid int         // Count of invalid digits
	Digits  []DigitScan // List of scanned digits
}

type segment struct {
	bb     BBox
	points PList
}

// LcdDecoder contains all the digit data required to decode
// the digits in an image.
type LcdDecoder struct {
	Digits       []*Digit             // List of digits to decode
	templates    map[string]*Template // Templates used to create digits
	offset       Point                // Global offset used to adjust image
	Threshold    int                  // Default on/off threshold
	levelsList   []*levels            // List of saved threshold levels
	qualityTotal int                  // Sum of quality values
	curLevels    *levels              // Current threshold levels
}

// There are 128 possible values in a 7 segment digit, but only a subset
// are used to represent digits and characters.
// This table maps that subset of bit masks to the digits and characters.
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
	____ | M_TM | M_TR | M_BR | ____ | ____ | ____: '7',	// Alternate '7'
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

// Create a new LcdDecoder.
func NewLcdDecoder() *LcdDecoder {
	l := new(LcdDecoder)
	l.templates = make(map[string]*Template)
	l.Threshold = defaultThreshold
	l.curLevels = new(levels)
	return l
}

// Add a digit template.
// Each template describes the parameters of one type/size of digit.
// bb contains a list of 3 points representing the top right,
// bottom right and bottom left of the boundaries of the digit.
// These are signed offsets from the implied base of (0,0) representing
// the top left of the digit.
// dp is an optional point offset where a decimal place is located.
// width is the width of the segment in pixels.
// All point references in the template are relative to the origin of the digit.
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
	// Prepend the implied top left origin (0,0).
	bb = append([]int{0, 0}, bb...)
	t := &Template{name: name, line: width}
	for i := range t.bb {
		t.bb[i].X = bb[i*2]
		t.bb[i].Y = bb[i*2+1]
	}
	if len(dp) == 2 {
		t.dp = Point{X: dp[0], Y: dp[1]}.Block((width + 1) / 2)
	}
	// Initialise the bounding boxes representing the segments of the digit.
	// Calculate the middle points of the digit.
	t.mr = Split(t.bb[TR], t.bb[BR], 2)[0]
	t.tmr = Adjust(t.mr, t.bb[TR], width/2)
	t.bmr = Adjust(t.mr, t.bb[BR], width/2)
	t.ml = Split(t.bb[TL], t.bb[BL], 2)[0]
	t.tml = Adjust(t.ml, t.bb[TL], width/2)
	t.bml = Adjust(t.ml, t.bb[BL], width/2)
	// Build the 'off' point list using the middle blocks inside the
	// upper and lower squares of the segments.
	offbb1 := BBox{t.bb[TL], t.bb[TR], t.bmr, t.bml}.Inner(width + offMargin)
	offbb2 := BBox{t.tml, t.tmr, t.bb[BR], t.bb[BL]}.Inner(width + offMargin)
	t.off = offbb1.Points()
	t.off = append(t.off, offbb2.Points()...)
	// Create the bounding boxes of each segment in the digit.
	// The assignments must match the bit allocation in the lookup table.
	t.seg[S_TL].bb = SegmentBB(t.bb[TL], t.ml, t.bb[TR], t.mr, width, onMargin)
	t.seg[S_TM].bb = SegmentBB(t.bb[TL], t.bb[TR], t.bb[BL], t.bb[BR], width, onMargin)
	t.seg[S_TR].bb = SegmentBB(t.bb[TR], t.mr, t.bb[TL], t.ml, width, onMargin)
	t.seg[S_BR].bb = SegmentBB(t.mr, t.bb[BR], t.ml, t.bb[BL], width, onMargin)
	t.seg[S_BM].bb = SegmentBB(t.bb[BL], t.bb[BR], t.ml, t.mr, width, onMargin)
	t.seg[S_BL].bb = SegmentBB(t.ml, t.bb[BL], t.mr, t.bb[BR], width, onMargin)
	t.seg[S_MM].bb = SegmentBB(t.tml, t.tmr, t.bb[BL], t.bb[BR], width, onMargin)
	// For each segment, create a list of all the points within the segment.
	for i := range t.seg {
		t.seg[i].points = t.seg[i].bb.Points()
	}
	l.templates[name] = t
	return nil
}

// Add a digit using the named template. The template points are offset
// by the absolute point location of the digit (x, y).
func (l *LcdDecoder) AddDigit(name string, x, y int) (*Digit, error) {
	t, ok := l.templates[name]
	if !ok {
		return nil, fmt.Errorf("Unknown template %s", name)
	}
	index := len(l.Digits)
	d := &Digit{}
	d.index = index
	d.bb = t.bb.Offset(x, y)
	d.off = t.off.Offset(x, y)
	d.dp = t.dp.Offset(x, y)
	// Copy over the segment data from the template, offsetting the points
	// using the digit's origin.
	d.lev = new(digLevels)
	for i := 0; i < SEGMENTS; i++ {
		d.seg[i].bb = t.seg[i].bb.Offset(x, y)
		d.seg[i].points = t.seg[i].points.Offset(x, y)
		d.lev.segLevels[i].min = NewAvg(*history)
		d.lev.segLevels[i].max = NewAvg(*history)
	}
	d.dp = t.dp.Offset(x, y)
	d.tmr = t.tmr.Offset(x, y)
	d.tml = t.tml.Offset(x, y)
	d.bmr = t.bmr.Offset(x, y)
	d.bml = t.bml.Offset(x, y)
	l.curLevels.digits = append(l.curLevels.digits, d.lev)
	l.Digits = append(l.Digits, d)
	return d, nil
}

func (l *LcdDecoder) SetThreshold(th int) {
	l.Threshold = th
}

// Decode the 7 segment digits in the image.
func (l *LcdDecoder) Decode(img image.Image) *ScanResult {
	var res ScanResult
	// Save the image
	res.img = img
	var str []byte
	for _, d := range l.Digits {
		var ds DigitScan
		ds.Segments = make([]int, SEGMENTS, SEGMENTS)
		for i := range ds.Segments {
			// Sample the segment blocks.
			ds.Segments[i] = sampleRegion(img, d.seg[i].points)
			if ds.Segments[i] >= d.lev.segLevels[i].threshold {
				// Set mask bit if segment considered 'on'.
				ds.Mask |= 1 << uint(i)
			}
		}
		ds.Char, ds.Valid = resultTable[ds.Mask]
		if ds.Valid {
			// Valid character found.
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

// Sample the points in the points list.
// Each point is converted to 16 bit grayscale and averaged across all the points in the list.
// The 16 bit result is inverted so that darker values are higher.
func sampleRegion(img image.Image, pl PList) int {
	var gacc int
	for _, s := range pl {
		c := img.At(s.X, s.Y)
		pix := color.Gray16Model.Convert(c).(color.Gray16)
		gacc += int(pix.Y)
	}
	return 0x10000 - gacc/len(pl)
}

// Map each character in s to the bit mask representing the segments for
// that character.
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

// Mark the segments on this image.
// Draw white cross markers on the corners of the segments.
// If fill true, block fill the on and off portions of the segments.
func (l *LcdDecoder) MarkSamples(img *image.RGBA, fill bool) {
	red := color.RGBA{255, 0, 0, 50}
	green := color.RGBA{0, 255, 0, 50}
	white := color.RGBA{255, 255, 255, 255}
	for _, d := range l.Digits {
		drawBB(img, d.bb, white)
		ext := PList{d.tmr, d.tml, d.bmr, d.bml}
		drawCross(img, ext, white)
		if fill {
			drawFill(img, d.off, green)
		}
		for i := range d.seg {
			if fill {
				drawFill(img, d.seg[i].points, red)
			}
		}
		drawFill(img, d.dp, red)
	}
}

func drawBB(img *image.RGBA, b BBox, c color.Color) {
	drawCross(img, b[:], c)
}

func drawFill(img *image.RGBA, pl PList, c color.Color) {
	for _, p := range pl {
		img.Set(p.X, p.Y, c)
	}
}

func drawCross(img *image.RGBA, pl PList, c color.Color) {
	for _, p := range pl {
		x := p.X
		y := p.Y
		img.Set(x, y, c)
		for i := 1; i < 3; i++ {
			img.Set(x-i, y, c)
			img.Set(x+i, y, c)
			img.Set(x, y-i, c)
			img.Set(x, y+i, c)
		}
	}
}
