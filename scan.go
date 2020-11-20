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
	"image"
	"image/color"
)

// DigitScan contains the scanned values for one digit.
type DigitScan struct {
	Segments []int // Averaged value for each segment
	DP       int   // Decimal point sample (if any)
	Mask     int   // Mask of segment bits
}

// DigitDecode is the result of decoding one digit in the image.
type DigitDecode struct {
	Char  byte   // The decoded character
	Str   string // The decoded char as a string
	Valid bool   // True if the decode was successful
	DP    bool   // True if the decimal point is set
}

// DecodeResult contains the results of scanning and decoding one image.
type DecodeResult struct {
	Img     image.Image    // Image that has been scanned
	Text    string         // Decoded string of digits
	Invalid int            // Count of invalid digits
	Scans   []*DigitScan   // Scan result
	Decodes []*DigitDecode // List of decoded digits
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
	____ | M_TM | M_TR | M_BR | ____ | ____ | ____: '7', // Alternate '7'
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

// Decode the 7 segment digits in the image, and return a summary of the decoded values.
// curLevels must be initialised either by having the levels restored from
// a file, or having been calibrated with an image via Preset.
func (l *LcdDecoder) Decode(img image.Image) *DecodeResult {
	res := new(DecodeResult)
	res.Img = img
	res.Scans = l.Scan(img)
	var str []byte
	for di, scan := range res.Scans {
		decode := new(DigitDecode)
		// Check if sampled segment value is over threshold, and
		// if so, set mask bit on.
		for si, v := range scan.Segments {
			if v >= l.curLevels.digits[di].segLevels[si].threshold {
				scan.Mask |= 1 << uint(si)
			}
		}
		decode.Char, decode.Valid = resultTable[scan.Mask]
		if decode.Valid {
			// Valid character found.
			decode.Str = string([]byte{decode.Char})
			str = append(str, decode.Char)
		} else {
			res.Invalid++
		}
		if scan.DP > l.curLevels.digits[di].threshold {
			decode.DP = true
			str = append(str, '.')
		}
		res.Decodes = append(res.Decodes, decode)
	}
	res.Text = string(str)
	return res
}

// Scan samples the regions of the image that map to the segments of the digits,
// and returns a list of the scanned digits.
func (l *LcdDecoder) Scan(img image.Image) []*DigitScan {
	var scans []*DigitScan
	for _, d := range l.Digits {
		ds := new(DigitScan)
		ds.Segments = make([]int, SEGMENTS, SEGMENTS)
		for i := range ds.Segments {
			// Sample the segment blocks.
			ds.Segments[i] = l.sampleRegion(img, d.seg[i].points)
		}
		// Check for decimal place.
		if len(d.dp) > 0 {
			ds.DP = l.sampleRegion(img, d.dp)
		}
		scans = append(scans, ds)
	}
	return scans
}

// Sample the points in the points list, and return a 16 bit value
// representing the brightness level of the region.
// Each point is converted to 16 bit grayscale and averaged across all the points in the list.
// The value is normalised so that higher values represent an 'on' state.
func (l *LcdDecoder) sampleRegion(img image.Image, pl PList) int {
	var gacc int
	for _, s := range pl {
		c := img.At(s.X, s.Y)
		pix := color.Gray16Model.Convert(c).(color.Gray16)
		gacc += int(pix.Y)
	}
	if l.Inverse {
		// Lighter values are considered 'on' e.g when a LED image is scanned.
		return gacc / len(pl)
	} else {
		// Darker values are considered 'on' e.g when an LCD image is scanned.
		return 0x10000 - gacc/len(pl)
	}
}
