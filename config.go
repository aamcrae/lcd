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
)

type LcdTemplate struct {
	Name  string
	Tl    [2]int // Top left (origin)
	Tr    [2]int // Top right
	Br    [2]int // Bottom right
	Bl    [2]int // Bottom left
	Width int
	Dp    []int
}

type DigitConfig struct {
	Lcd   string
	Coord [2]int
}

// Configuration block
type LcdConfig struct {
	Threshold int
	Offset    [2]int
	Lcd       []LcdTemplate
	Digit     []DigitConfig
}

// Create a 7 segment decoder using the configuration data provided.
func CreateLcdDecoder(conf LcdConfig) (*LcdDecoder, error) {
	l := NewLcdDecoder()
	// threshold is a percentage defining the point between the max and min.
	if conf.Threshold != 0 {
		l.Threshold = conf.Threshold
	}
	// lcd defines one 7 segment digit template.
	// The format is a name followed by 3 pairs of x/y offsets defining the corners
	// of the digit (relative to implied top left of 0,0), followed by a value defining
	// the pixel width of the segment elements.
	if len(conf.Lcd) == 0 {
		return nil, fmt.Errorf("No LCDs defined")
	}
	if len(conf.Digit) == 0 {
		return nil, fmt.Errorf("No digits defined")
	}
	for i, e := range conf.Lcd {
		if err := l.AddTemplate(e); err != nil {
			return nil, fmt.Errorf("Invalid LCD (index %d): %v", i, err)
		}
	}
	// digit declares an instance of a digit.
	// A string references the template name, followed by the point (x,y) defining
	// the top left corner of the digit (adjusted using the global offset).
	for i, e := range conf.Digit {
		e.Coord[0] += conf.Offset[0]
		e.Coord[1] += conf.Offset[1]
		_, err := l.AddDigit(e)
		if err != nil {
			return nil, fmt.Errorf("Invalid digit config (index %d): %v", i, err)
		}
	}
	return l, nil
}
