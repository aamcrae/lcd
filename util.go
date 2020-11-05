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

// Functions used by test or utility programs.

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
