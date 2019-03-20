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

package lcd_test

import (
	"testing"

	"github.com/aamcrae/MeterMan/lcd"
)

func TestBbox(t *testing.T) {
	// Diamond.
	bb := lcd.BBox{lcd.Point{10, 5}, lcd.Point{15, 10}, lcd.Point{10, 15}, lcd.Point{5, 10}}
	in := []lcd.Point{lcd.Point{6, 10}, lcd.Point{10, 6}, lcd.Point{14,10}, lcd.Point{10, 9}, lcd.Point{10, 10}}
	for _, p := range in {
		if !lcd.InBB(bb, p) {
			t.Errorf("Expected true, got false for point (%d, %d)", p.X, p.Y)
		}
	}
	out := []lcd.Point{lcd.Point{0,0}, lcd.Point{15,15}, lcd.Point{5, 11}, lcd.Point{4, 8}, lcd.Point{11, 5}, lcd.Point{4, 10}, lcd.Point{16, 10}}
	for _, p := range out {
		if lcd.InBB(bb, p) {
			t.Errorf("Expected false, got true for point (%d, %d)", p.X, p.Y)
		}
	}
	// extended diamond.
	bb = lcd.BBox{lcd.Point{8, 3}, lcd.Point{15, 10}, lcd.Point{10, 15}, lcd.Point{3, 8}}
	in = []lcd.Point{lcd.Point{8, 10}, lcd.Point{10, 8}, lcd.Point{14,10}, lcd.Point{10, 9},
		lcd.Point{10, 10}, lcd.Point{8, 3}, lcd.Point{3, 8}}
	for _, p := range in {
		if !lcd.InBB(bb, p) {
			t.Errorf("Expected true, got false for point (%d, %d)", p.X, p.Y)
		}
	}
	out = []lcd.Point{lcd.Point{0,0}, lcd.Point{15,15}, lcd.Point{5, 11}, lcd.Point{2, 8}, lcd.Point{11, 5}, lcd.Point{2, 10}, lcd.Point{16, 10}}
	for _, p := range out {
		if lcd.InBB(bb, p) {
			t.Errorf("Expected false, got true for point (%d, %d)", p.X, p.Y)
		}
	}
	// Square.
	bb = lcd.BBox{lcd.Point{5, 5}, lcd.Point{10, 5}, lcd.Point{10, 10}, lcd.Point{5, 10}}
	in = []lcd.Point{lcd.Point{5,5}, lcd.Point{7,8}, lcd.Point{10,10}, lcd.Point{5, 10}, lcd.Point{10,5}, lcd.Point{8, 10}}
	for _, p := range in {
		if !lcd.InBB(bb, p) {
			t.Errorf("Expected true, got false for point (%d, %d)", p.X, p.Y)
		}
	}
	out = []lcd.Point{lcd.Point{0,0}, lcd.Point{11,11}, lcd.Point{11,10}, lcd.Point{4, 8}, lcd.Point{12,8}, lcd.Point{11, 5}}
	for _, p := range out {
		if lcd.InBB(bb, p) {
			t.Errorf("Expected false, got true for point (%d, %d)", p.X, p.Y)
		}
	}
}
