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
	"sort"
	"strconv"
	"strings"
)

var history = flag.Int("history", 5, "Size of history cache")
var levelSize = flag.Int("level_size", 100, "Size of level map")
var savedLevels = flag.Int("level_saved", 50, "Number of levels saved")

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

// Levels contains the on/off thresholds for the digit segments.
// They are stored as moving averages, and are recalculated dynamically
// when successful translation of segments occurs.
// They can be saved periodically, and restored from disk at startup
// to provide an initial set of calibrated thresholds to use.
type levels struct {
	bad, good, quality int
	digits             []*digLevels
}

type digLevels struct {
	min       int
	max       int
	threshold int
	segLevels [SEGMENTS]segLevels
}

type segLevels struct {
	min       *Avg
	max       *Avg
	threshold int
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

// There are 128 possible values in a 7 segment digit,
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

// Calibrate calculates the on and off threshold values from the image provided,
// using digits as the value of the segments.
func (l *LcdDecoder) CalibrateImage(img image.Image, digits string) error {
	if len(digits) != len(l.Digits) {
		return fmt.Errorf("Digit count mismatch (digits: %d, calibration: %d", len(digits), len(l.Digits))
	}
	// Create a decoded result for this image.
	res := l.Decode(img)
	for i := range l.Digits {
		char := byte(digits[i])
		mask, ok := reverseTable[char]
		if !ok {
			return fmt.Errorf("Unknown digit: 0x%02x", char)
		}
		// Use the bits that are expected to be on (found via the reverse lookup) to
		// calibrate the values that are sampled from the image.
		off := sampleRegion(img, l.Digits[i].off)
		l.Digits[i].calibrateDigit(res.Digits[i].Segments, off, mask, l.Threshold)
	}
	return nil
}

// Calibrate using a previous result.
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

// Restore the calibration data from a saved cache.
// Format is a line of CSV:
// index,quality
// index,digit,segment,min,max
func (l *LcdDecoder) RestoreCalibration(r io.Reader) {
	oldIndex := -1
	scanner := bufio.NewScanner(r)
	var cal *levels
	var calList []*levels
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
		if len(v) != 2 && len(v) != 5 {
			log.Printf("RestoreCalibration: line %d, field count mismatch (%d)", line, len(v))
			continue
		}
		if v[0] < 0 || v[0] >= *levelSize {
			log.Printf("RestoreCalibration: line %d, level index out of range (%d)", line, v[0])
			continue
		}
		if v[0] != oldIndex {
			cal = l.curLevels.Copy()
			cal.quality = 100
			oldIndex = v[0]
			calList = append(calList, cal)
		}
		if len(v) == 2 {
			cal.quality = v[1]
		} else {
			if v[1] < 0 || v[1] >= len(l.Digits) {
				log.Printf("RestoreCalibration: line %d, out of range digit (%d)", line, v[1])
				continue
			}
			if v[2] < 0 || v[2] >= SEGMENTS {
				log.Printf("RestoreCalibration: line %d, out of range segment (%d)", line, v[2])
				continue
			}
			s := &cal.digits[v[1]].segLevels[v[2]]
			s.min.Init(v[3])
			s.max.Init(v[4])
			s.threshold = threshold(s.min.Value, s.max.Value, l.Threshold)
		}
	}
	for _, lv := range calList {
		for _, d := range lv.digits {
			var min, max int
			for i := range d.segLevels {
				min += d.segLevels[i].min.Value
				max += d.segLevels[i].max.Value
			}
			// Keep the average of the min and max.
			d.min = min / len(d.segLevels)
			d.max = max / len(d.segLevels)
			d.threshold = threshold(d.min, d.max, l.Threshold)
		}
	}
	// Fill entire calibration list with saved entries.
	if len(calList) > 0 {
		calIndex := 0
		for i := 0; i < *levelSize; i += 1 {
			l.AddCalibration(calList[calIndex].Copy())
			calIndex += 1
			if calIndex >= len(calList) {
				calIndex = 0
			}
		}
	}
	log.Printf("RestoreCalibration: %d entries read", len(calList))
}

// Save the threshold data. Only a selected number are saved,
// not the entire list.
func (l *LcdDecoder) SaveCalibration(w io.WriteCloser) {
	start := len(l.levelsList) - *savedLevels
	if start < 0 {
		start = 0
	}
	for li, lev := range l.levelsList[start:] {
		fmt.Fprintf(w, "%d,%d\n", li, lev.quality)
		for i, d := range lev.digits {
			for s := range d.segLevels {
				fmt.Fprintf(w, "%d,%d,%d,%d,%d\n", li, i, s, d.segLevels[s].min.Value, d.segLevels[s].max.Value)
			}
		}
	}
}

// Add a new calibration to the list of calibrations.
func (l *LcdDecoder) AddCalibration(lev *levels) {
	l.levelsList = insert(l.levelsList, lev)
	l.qualityTotal += lev.quality
}

// Save the current levels in the map, discard the worst, and pick the best.
func (l *LcdDecoder) Recalibrate() {
	t := l.curLevels.bad + l.curLevels.good
	l.curLevels.quality = l.curLevels.good * 100 / t
	l.AddCalibration(l.curLevels.Copy())
	l.AddCalibration(l.curLevels)
	l.qualityTotal -= l.levelsList[0].quality
	l.levelsList = l.levelsList[1:]
	l.PickCalibration()
}

// Pick the best calibration from the list.
func (l *LcdDecoder) PickCalibration() {
	sz := len(l.levelsList)
	best := l.levelsList[sz-1]
	worst := l.levelsList[0]
	log.Printf("Recalibration: last %3d (good %2d, bad %2d), new %3d, worst %3d, count %d, avg %5.1f",
		l.curLevels.quality, l.curLevels.good, l.curLevels.bad, best.quality, worst.quality, sz, float32(l.qualityTotal)/float32(sz))
	// Remove selected (best) entry from list.
	l.qualityTotal -= best.quality
	l.levelsList = l.levelsList[:sz-1]
	best.bad = 0
	best.good = 0
	l.curLevels = best
	for i := range l.curLevels.digits {
		l.Digits[i].lev = l.curLevels.digits[i]
	}
}

// Record a successful decode.
func (l *LcdDecoder) Good() {
	l.curLevels.good++
}

// Record an unsuccessful decode.
func (l *LcdDecoder) Bad() {
	l.curLevels.bad++
}

// Copy the calibration threshold values struct.
func (l *levels) Copy() *levels {
	nl := new(levels)
	nl.quality = l.quality
	for _, d := range l.digits {
		nd := new(digLevels)
		nd.min = d.min
		nd.max = d.max
		nd.threshold = d.threshold
		// Need to clone the moving averages.
		for i := range nd.segLevels {
			nd.segLevels[i].min = d.segLevels[i].min.Copy()
			nd.segLevels[i].max = d.segLevels[i].max.Copy()
			nd.segLevels[i].threshold = d.segLevels[i].threshold
		}
		nl.digits = append(nl.digits, nd)
	}
	return nl
}

// Insert an instance of the levels into the sorted list.
func insert(list []*levels, l *levels) []*levels {
	index := sort.Search(len(list), func(i int) bool { return list[i].quality > l.quality })
	list = append(list, nil)
	copy(list[index+1:], list[index:])
	list[index] = l
	return list
}

// Calculate the threshold as a percentage between the min and max limits.
func threshold(min, max, perc int) int {
	return min + (max-min)*perc/100
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
