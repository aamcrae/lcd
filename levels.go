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
	"fmt"
	"image"
	"io"
	"log"
	"sort"
	"strconv"
	"strings"
)

// levels contains the on/off thresholds for the digit segments.
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
		if v[0] < 0 || v[0] >= l.MaxLevels {
			log.Printf("RestoreCalibration: line %d, level index (%d) out of range, max %d", line, v[0], l.MaxLevels)
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
		for i := 0; i < l.MaxLevels; i += 1 {
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
func (l *LcdDecoder) SaveCalibration(w io.WriteCloser, max int) {
	start := len(l.levelsList) - max
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
	// Add the most recent threshold calibration back into the list.
	// It is added twice since the worst result is dropped if the list
	// is at the maximum size.
	l.AddCalibration(l.curLevels.Copy())
	l.AddCalibration(l.curLevels)
	if len(l.levelsList) > l.MaxLevels {
		// Drop the worst result
		l.qualityTotal -= l.levelsList[0].quality
		l.levelsList = l.levelsList[1:]
	}
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
