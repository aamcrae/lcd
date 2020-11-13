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
	"strconv"
	"strings"
)

// levels contains the on/off thresholds for the individual segments.
// Considerable effort is made to dynamically track these thresholds, since
// light levels (and thus the value at which a segment is considered 'on' or 'off)
// will vary over time depending on external conditions.
// The threshold tracking also handles incorrect reading due to segments being
// captured in the middle of transitioning to the opposite state.
//
// For each segment, a max, min and threshold is held.
// Max and min represent the measured maximum and minimum levels representing
// the 'on' and 'off' states respectively.
// The threshold represents the middle point either side of which a segment
// is considered 'on' or 'off'.
//
// The max and min are stored as moving averages, and are updated dynamically
// when successful decoding of segments occurs - the averaged samples for each
// segment are added either to the min or the max depending on whether the segment
// is considered 'off' or 'on'. From the updated min and max, a new threshold is
// calculated that is then used for future decodes.
// A moving average is used so that a single poor read does not skew the
// thresholds too much (such as can occur if the segments are changing at the time
// the image is captured).
// A quality value (0-100) is calculated for every set of thresholds, and this is used
// to select a new set of thresholds periodically.
//
// TODO: Since all the segment points are sampled and then averaged, it may be possible to
// define a minimum and maximum band that more accurately captures an 'on' or 'off' state,
// and a middle band to detect segments that are transitioning.
//
// The list can be saved periodically, and restored from disk at startup
// to provide an initial set of calibrated thresholds to use.

type levels struct {
	bad     int          // Count of undecodeable scans
	good    int          // Count of successful scans
	quality int          // quality metric 0-100
	digits  []*digLevels // List of levels for each digit
}

// digLevels holds the calibration levels for one digit.
type digLevels struct {
	min       int                 // Average min value for all segments
	max       int                 // Average max value for all segments
	threshold int                 // Average threshold
	segLevels [SEGMENTS]segLevels // Per-segment levels data
}

// segLevels holds the calibration levels for one segment of a digit.
type segLevels struct {
	min       *Avg // Moving average of minimum ('off') value
	max       *Avg // Moving average of maximum ('on') value
	threshold int  // Threshold middle point
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
		off := l.sampleRegion(img, l.Digits[i].off)
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
		off := l.sampleRegion(scan.img, l.Digits[i].off)
		l.Digits[i].calibrateDigit(scan.Digits[i].Segments, off, scan.Digits[i].Mask, l.Threshold)
	}
	return nil
}

// Restore the calibration data from a saved cache.
// Format is a line of CSV, either:
//   index,quality
//   index,digit,segment,min,max
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

// Save the threshold data.
// Only the highest quality level sets are saved.
func (l *LcdDecoder) SaveCalibration(w io.WriteCloser, max int) {
	written := 0
	worst, best := l.qualRange()
	for qual := best; qual >= worst; qual-- {
		for _, lev := range l.levelsMap[qual] {
			fmt.Fprintf(w, "%d,%d\n", written, lev.quality)
			for i, d := range lev.digits {
				for s := range d.segLevels {
					fmt.Fprintf(w, "%d,%d,%d,%d,%d\n", written, i, s, d.segLevels[s].min.Value, d.segLevels[s].max.Value)
				}
			}
			written++
			if written == max {
				return
			}
		}
	}
}

// Add a new calibration entry to the map
func (l *LcdDecoder) AddCalibration(lev *levels) {
	l.levelsMap[lev.quality] = append(l.levelsMap[lev.quality], lev)
	l.qualityTotal += lev.quality
	l.levelsCount++
	log.Printf("Adding entry to qual %d, number of entries: %d", lev.quality, len(l.levelsMap[lev.quality]))
}

// Get one calibration entry from the map entry specified, removing
// it from the map.
// At least one element must exist in the map entry list.
// The first list item is returned.
// TODO: Consider selecting a random element.
func (l *LcdDecoder) GetCalibration(qual int) *levels {
	blist := l.levelsMap[qual]
	l.levelsMap[qual] = blist[1:]
	if len(blist) == 1 {
		log.Printf("Removing entry from qual %d, none left, removing from map", qual)
		delete(l.levelsMap, qual)
	} else {
		log.Printf("Removing entry from qual %d, number left: %d", qual, len(l.levelsMap[qual]))
	}
	// Adjust the total quality and count of levels.
	l.qualityTotal -= qual
	l.levelsCount--
	return blist[0]
}

// Save the current levels calibration in the map, discard the worst, and pick the best.
func (l *LcdDecoder) Recalibrate() {
	// Calculate a quality metric between 0-100 inclusive from
	// the total number of good and bad scans.
	t := l.curLevels.bad + l.curLevels.good
	l.curLevels.quality = l.curLevels.good * 100 / t
	// Add the most recent threshold calibration back into the list.
	l.AddCalibration(l.curLevels)
	// If the map hasn't reached the maximum number, add a copy to
	// increase the number of calibrations available.
	if l.levelsCount < l.MaxLevels {
		l.AddCalibration(l.curLevels.Copy())
	} else {
		// The map is at maximum capacity.
		// Get the worst quality in the map, and if required
		// drop the one of the worst and add another copy of the current one.
		// If the current calibration is one of the worst, just ignore it.
		w, _ := l.qualRange()
		if w != l.curLevels.quality {
			l.AddCalibration(l.curLevels.Copy())
			l.GetCalibration(w)
		}
	}
	l.PickCalibration()
}

// Pick the best calibration from the list.
func (l *LcdDecoder) PickCalibration() {
	worst, best := l.qualRange()
	log.Printf("Recalibration: last %3d (good %2d, bad %2d), new %3d, worst %3d, count %d, avg %5.1f",
		l.curLevels.quality, l.curLevels.good, l.curLevels.bad, best, worst,
		l.levelsCount, float32(l.qualityTotal)/float32(l.levelsCount))
	// Get one entry from the list of the best.
	l.curLevels = l.GetCalibration(best)
	l.curLevels.bad = 0
	l.curLevels.good = 0
	for i := range l.curLevels.digits {
		l.Digits[i].lev = l.curLevels.digits[i]
	}
}

// Return the quality range of the calibration data as lowest to highest,
// deleting any empty list entries in the map that are found.
func (l *LcdDecoder) qualRange() (int, int) {
	var best, worst int
	// Init the best and worst from the first entry.
	for q := range l.levelsMap {
		best = q
		worst = q
		break
	}
	for q := range l.levelsMap {
		if q > best {
			best = q
		}
		if q < worst {
			worst = q
		}
	}
	return worst, best
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

// Calculate the threshold as a percentage between the min and max limits.
func threshold(min, max, perc int) int {
	return min + (max-min)*perc/100
}
