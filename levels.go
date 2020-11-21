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
	"os"
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

// Preset calculates the on and off threshold values from the image provided,
// using a preset result to map the on/off values for each segment.
func (l *LcdDecoder) Preset(img image.Image, digits string) error {
	// Scan the image.
	scans := l.Scan(img)
	if len(digits) != len(scans) {
		return fmt.Errorf("Digit count mismatch (found: %d, expected: %d", len(digits), len(scans))
	}
	for i, ds := range scans {
		char := byte(digits[i])
		m, ok := reverseTable[char]
		if !ok {
			return fmt.Errorf("Unknown digit %d: 0x%02x", i, char)
		}
		ds.Mask = m
	}
	if l.curLevels == nil {
		l.curLevels = l.newLevels()
	}
	return l.CalibrateUsingScan(img, scans)
}

// Create a new levels structure.
func (l *LcdDecoder) newLevels() *levels {
	lev := new(levels)
	for i := 0; i < len(l.Digits); i++ {
		dl := new(digLevels)
		for s := 0; s < SEGMENTS; s++ {
			dl.segLevels[s].min = NewAvg(l.History)
			dl.segLevels[s].max = NewAvg(l.History)
		}
		lev.digits = append(lev.digits, dl)
	}
	return lev
}

// Adjust levels using scan result and segment bit masks.
func (l *LcdDecoder) CalibrateUsingScan(img image.Image, scans []*DigitScan) error {
	if len(scans) != len(l.Digits) {
		return fmt.Errorf("Digit count mismatch (digits: %d, calibration: %d", len(scans), len(l.Digits))
	}
	var default_on int
	// If any digit has all segments off, we need to calculate an average max by
	// averaging the on segments for all the (other) digits.
	for _, s := range scans {
		if s.Mask == 0 {
			var on_segments int
			for _, s2 := range scans {
				for m := 0; m < SEGMENTS; m++ {
					if (s2.Mask & (1 << uint(m))) != 0 {
						on_segments++
						default_on += s2.Segments[m]
					}
				}
			}
			if on_segments == 0 {
				return fmt.Errorf("No segments are on, unable to calibrate")
			}
			default_on = default_on / on_segments
			break
		}
	}
	for i, d := range l.Digits {
		// Calculate a default 'off' value for the digit using the off centre blocks
		// of the digit.
		default_off := l.sampleRegion(img, d.off)
		l.curLevels.digits[i].adjustLevels(scans[i], default_off, default_on, l.Threshold)
	}
	return nil
}

// Restore the calibration data from a file.
func (l *LcdDecoder) RestoreFromFile(f string) (int, error) {
	if of, err := os.Open(f); err != nil {
		return 0, err
	} else {
		defer of.Close()
		return l.Restore(of)
	}
}

// Restore the calibration data from a saved cache.
// Format is a line of CSV, either:
//   index,quality
//   index,digit,segment,min,max
func (l *LcdDecoder) Restore(r io.Reader) (int, error) {
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
				return len(calList), fmt.Errorf("line %d, bad number (%v): %s", line, err, tok)
			} else {
				v = append(v, int(val))
			}
		}
		if len(v) != 2 && len(v) != 5 {
			return len(calList), fmt.Errorf("line %d, illegal count of numbers (%d) - must be 2 or 5)", line, len(v))
		}
		if v[0] < 0 || v[0] >= l.MaxLevels {
			return len(calList), fmt.Errorf("line %d, index (%d) out of range - max %d", line, v[0], l.MaxLevels)
		}
		if v[0] != oldIndex {
			cal = l.newLevels()
			cal.quality = 100
			oldIndex = v[0]
			calList = append(calList, cal)
		}
		if len(v) == 2 {
			cal.quality = v[1]
		} else {
			if v[1] < 0 || v[1] >= len(l.Digits) {
				return len(calList), fmt.Errorf("line %d, out of range digit (%d)", line, v[1])
			}
			if v[2] < 0 || v[2] >= SEGMENTS {
				return len(calList), fmt.Errorf("line %d, out of range segment (%d)", line, v[2])
			}
			s := &cal.digits[v[1]].segLevels[v[2]]
			s.min.Init(v[3])
			s.max.Init(v[4])
			s.threshold = thresholdPercent(s.min.Value, s.max.Value, l.Threshold)
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
			d.threshold = thresholdPercent(d.min, d.max, l.Threshold)
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
	l.PickCalibration()
	return len(calList), nil
}

// Save the threshold data to a file.
func (l *LcdDecoder) SaveToFile(f string, max int) error {
	if of, err := os.Create(f); err != nil {
		return err
	} else {
		defer of.Close()
		return l.Save(of, max)
	}
}

// Save the threshold data.
// Only the highest quality level sets are saved.
func (l *LcdDecoder) Save(w io.WriteCloser, max int) error {
	written := 0
	worst, best := l.qualRange()
	for qual := best; qual >= worst; qual-- {
		for _, lev := range l.levelsMap[qual] {
			_, err := fmt.Fprintf(w, "%d,%d\n", written, lev.quality)
			if err != nil {
				return err
			}
			for i, d := range lev.digits {
				for s := range d.segLevels {
					_, err := fmt.Fprintf(w, "%d,%d,%d,%d,%d\n", written, i, s, d.segLevels[s].min.Value, d.segLevels[s].max.Value)
					if err != nil {
						return err
					}
				}
			}
			written++
			if written == max {
				return nil
			}
		}
	}
	return nil
}

// Add a new calibration entry to the map
func (l *LcdDecoder) AddCalibration(lev *levels) {
	l.levelsMap[lev.quality] = append(l.levelsMap[lev.quality], lev)
	l.Total += lev.quality
	l.Count++
}

// Get one calibration entry from the map entry specified, removing
// it from the map.
// At least one element must exist in the map entry list.
func (l *LcdDecoder) GetCalibration(qual int) (lev *levels) {
	blist := l.levelsMap[qual]
	if len(blist) == 1 {
		// Only 1 entry.
		lev = blist[0]
		delete(l.levelsMap, qual)
	} else {
		// Select a random entry.
		index := l.rng.Intn(len(blist))
		lev = blist[index]
		// Remove the element by copying the last element to this index and
		// shortening the list by one element.
		blist[index] = blist[len(blist)-1]
		l.levelsMap[qual] = blist[:len(blist)-1]
	}
	// Adjust the total quality and count of levels.
	l.Total -= qual
	l.Count--
	return
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
	if l.Count < l.MaxLevels {
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
	// Update quality summary.
	l.Worst, l.Best = l.qualRange()
	if l.curLevels != nil {
		l.LastQuality = l.curLevels.quality
		l.LastGood = l.curLevels.good
		l.LastBad = l.curLevels.bad
	}
	// Get one entry from the list of the best.
	if l.Count > 0 {
		l.curLevels = l.GetCalibration(l.Best)
		l.curLevels.bad = 0
		l.curLevels.good = 0
	} else {
		l.curLevels = l.newLevels()
	}
}

// Return the quality range of the calibration data as lowest to highest.
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

// Adjust and update the levels for this digit. The goal is to update
// the moving average for the max and min values representing 'on' and 'off',
// and recalculate the threshold from the updated values.
// The challenge is that the sampled data either represents the 'on' value
// or the 'off' value, so effort is made to initialise the opposite to
// a reasonable value.
//
// scan contains the sampled values, one per segment. The value represents either
// the on value or the off value, depending on the mask bit for the segment.
// off is an averaged 'off' value for the entire digit, used for the
// off level for the segment when the segment is on.
// threshold is the percentage separating on and off (e.g 50 for mid-point)
func (d *digLevels) adjustLevels(scan *DigitScan, default_off, default_on, threshold int) {
	var tmax, tcount, off_segments int
	for i := range d.segLevels {
		if ((1 << uint(i)) & scan.Mask) != 0 {
			// Mask bit is on, so the level represents
			// an 'on' segment.
			d.segLevels[i].max.Add(scan.Segments[i])
			tmax += d.segLevels[i].max.Value
			tcount++
			d.segLevels[i].min.SetDefault(default_off)
		} else {
			// Mask bit is off, so the level represents
			// an 'off' segment.
			d.segLevels[i].min.Add(scan.Segments[i])
			off_segments++
		}
	}
	// For segments that are off, set the max to an average of the segments
	// that are on. If none are on, use the default value provided.
	if off_segments > 0 {
		if tcount > 0 {
			default_on = tmax / tcount
		}
		for i := range d.segLevels {
			d.segLevels[i].max.SetDefault(default_on)
		}
	}
	var max, min int
	for i := range d.segLevels {
		d.segLevels[i].threshold = thresholdPercent(d.segLevels[i].min.Value, d.segLevels[i].max.Value, threshold)
		min += d.segLevels[i].min.Value
		max += d.segLevels[i].max.Value
	}
	d.min = min / len(d.segLevels)
	d.max = max / len(d.segLevels)
	d.threshold = thresholdPercent(d.min, d.max, threshold)
}

// Set the min and max for the segments
func (d *digLevels) InitLevels(min, max, th int) {
	d.min = min
	d.max = max
	for _, sl := range d.segLevels {
		sl.min.Init(min)
		sl.max.Init(max)
		sl.threshold = thresholdPercent(min, max, th)
	}
}

// Calculate the threshold as a percentage between the min and max limits.
func thresholdPercent(min, max, perc int) int {
	return min + (max-min)*perc/100
}
