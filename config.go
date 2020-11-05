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
	"strconv"

	"github.com/aamcrae/config"
)

// Create a 7 segment decoder using the configuration data provided.
func CreateLcdDecoder(conf *config.Section) (*LcdDecoder, error) {
	var xoff, yoff int
	l := NewLcdDecoder()
	// threshold is a percentage defining the point between the
	// min and max sampled values where 'on' and 'off' are measured.
	t := conf.Get("threshold")
	if len(t) > 0 {
		v := readInts(t[0].Tokens)
		if len(v) == 1 {
			l.SetThreshold(v[0])
		}
	}
	// offset defines a bulk adjustment to all digits, as x and y offsets.
	o := conf.Get("offset")
	if len(o) > 0 {
		v := readInts(o[0].Tokens)
		if len(v) == 2 {
			xoff = v[0]
			yoff = v[1]
		}
	}
	// lcd defines one 7 segment digit template.
	// The format is a name followed by 3 pairs of x/y offsets defining the corners
	// of the digit (relative to implied top left of 0,0), followed by a value defining
	// the pixel width of the segment elements.
	for _, e := range conf.Get("lcd") {
		if len(e.Tokens) < 1 {
			return nil, fmt.Errorf("No config for template at line %d", e.Lineno)
		}
		name := e.Tokens[0]
		v := readInts(e.Tokens[1:])
		var dp []int
		if len(v) == 9 {
			dp = v[7:9]
		} else if len(v) != 7 {
			return nil, fmt.Errorf("Bad config for template at line %d", e.Lineno)
		}
		if err := l.AddTemplate(name, v[:6], dp, v[6]); err != nil {
			return nil, fmt.Errorf("Invalid config at line %d: %v", e.Lineno, err)
		}
	}
	// digit declares an instance of a digit.
	// A string references the template name, followed by the point (x,y) defining
	// the top left corner of the digit (adjusted using the global offset).
	for _, e := range conf.Get("digit") {
		if len(e.Tokens) != 3 && len(e.Tokens) != 5 {
			return nil, fmt.Errorf("Bad digit config line %d", e.Lineno)
		}
		v := readInts(e.Tokens[1:])
		if len(v) != 2 && len(v) != 4 {
			return nil, fmt.Errorf("Bad config for digit at line %d", e.Lineno)
		}
		d, err := l.AddDigit(e.Tokens[0], v[0]+xoff, v[1]+yoff)
		if err != nil {
			return nil, fmt.Errorf("Invalid digit config at line %d: %v", e.Lineno, err)
		}
		// An optional pair of min/max values may be defined that set the
		// default min/max threshold values of the digit.
		if len(v) == 4 {
			d.SetMinMax(v[2], v[3], l.Threshold)
		}
	}
	return l, nil
}

func readInts(strs []string) []int {
	vals := []int{}
	for _, s := range strs {
		if v, err := strconv.ParseInt(s, 10, 32); err != nil {
			break
		} else {
			vals = append(vals, int(v))
		}
	}
	return vals
}
