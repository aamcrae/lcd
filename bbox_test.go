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
	"testing"
)

func TestBbox(t *testing.T) {
	// Diamond.
	bb := BBox{Point{10, 5}, Point{15, 10}, Point{10, 15}, Point{5, 10}}
	in := []Point{Point{10, 5}, Point{6, 10}, Point{10, 6}, Point{14, 10}, Point{10, 9}, Point{10, 10}}
	for _, p := range in {
		if !bb.In(p) {
			t.Fatalf("Expected true, got false for point (%d, %d)", p.X, p.Y)
		}
	}
	out := []Point{Point{3, 5}, Point{0, 0}, Point{3, 15}, Point{15, 15}, Point{5, 11}, Point{4, 8}, Point{11, 5}, Point{4, 10}, Point{16, 10}}
	for _, p := range out {
		if bb.In(p) {
			t.Fatalf("Expected false, got true for point (%d, %d)", p.X, p.Y)
		}
	}
	// extended diamond.
	bb = BBox{Point{8, 3}, Point{15, 10}, Point{10, 15}, Point{3, 8}}
	in = []Point{Point{8, 10}, Point{10, 8}, Point{14, 10}, Point{10, 9},
		Point{10, 10}, Point{8, 3}, Point{3, 8}}
	for _, p := range in {
		if !bb.In(p) {
			t.Errorf("Expected true, got false for point (%d, %d)", p.X, p.Y)
		}
	}
	out = []Point{Point{0, 0}, Point{15, 15}, Point{5, 11}, Point{2, 8}, Point{11, 5}, Point{2, 10}, Point{16, 10}}
	for _, p := range out {
		if bb.In(p) {
			t.Errorf("Expected false, got true for point (%d, %d)", p.X, p.Y)
		}
	}
	// Square.
	bb = BBox{Point{5, 5}, Point{10, 5}, Point{10, 10}, Point{5, 10}}
	in = []Point{Point{5, 5}, Point{7, 8}, Point{10, 10}, Point{5, 10}, Point{10, 5}, Point{8, 10}}
	for _, p := range in {
		if !bb.In(p) {
			t.Errorf("Expected true, got false for point (%d, %d)", p.X, p.Y)
		}
	}
	out = []Point{Point{0, 0}, Point{11, 11}, Point{11, 10}, Point{4, 8}, Point{12, 8}, Point{11, 5}}
	for _, p := range out {
		if bb.In(p) {
			t.Errorf("Expected false, got true for point (%d, %d)", p.X, p.Y)
		}
	}
}
