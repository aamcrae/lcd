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
	bb := bbox{point{10, 5}, point{15, 10}, point{10, 15}, point{5, 10}}
	in := []point{point{6, 10}, point{10, 6}, point{14, 10}, point{10, 9}, point{10, 10}}
	for _, p := range in {
		if !inBB(bb, p) {
			t.Fatalf("Expected true, got false for point (%d, %d)", p.x, p.y)
		}
	}
	out := []point{point{0, 0}, point{15, 15}, point{5, 11}, point{4, 8}, point{11, 5}, point{4, 10}, point{16, 10}}
	for _, p := range out {
		if inBB(bb, p) {
			t.Errorf("Expected false, got true for point (%d, %d)", p.x, p.y)
		}
	}
	// extended diamond.
	bb = bbox{point{8, 3}, point{15, 10}, point{10, 15}, point{3, 8}}
	in = []point{point{8, 10}, point{10, 8}, point{14, 10}, point{10, 9},
		point{10, 10}, point{8, 3}, point{3, 8}}
	for _, p := range in {
		if !inBB(bb, p) {
			t.Errorf("Expected true, got false for point (%d, %d)", p.x, p.y)
		}
	}
	out = []point{point{0, 0}, point{15, 15}, point{5, 11}, point{2, 8}, point{11, 5}, point{2, 10}, point{16, 10}}
	for _, p := range out {
		if inBB(bb, p) {
			t.Errorf("Expected false, got true for point (%d, %d)", p.x, p.y)
		}
	}
	// Square.
	bb = bbox{point{5, 5}, point{10, 5}, point{10, 10}, point{5, 10}}
	in = []point{point{5, 5}, point{7, 8}, point{10, 10}, point{5, 10}, point{10, 5}, point{8, 10}}
	for _, p := range in {
		if !inBB(bb, p) {
			t.Errorf("Expected true, got false for point (%d, %d)", p.x, p.y)
		}
	}
	out = []point{point{0, 0}, point{11, 11}, point{11, 10}, point{4, 8}, point{12, 8}, point{11, 5}}
	for _, p := range out {
		if inBB(bb, p) {
			t.Errorf("Expected false, got true for point (%d, %d)", p.x, p.y)
		}
	}
}
