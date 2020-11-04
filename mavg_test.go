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

func TestAvg(t *testing.T) {
	a := NewAvg(5)
	if a.Value != 0 {
		t.Fatalf("Expected value 0, got %d", a.Value)
	}
	a.Add(100)
	if a.Value != 100 {
		t.Fatalf("Expected value 100, got %d", a.Value)
	}
	a.Add(200)
	if a.Value != 150 {
		t.Fatalf("Expected value 150, got %d", a.Value)
	}
	for i := 0; i < 5; i++ {
		a.Add(10)
	}
	if a.Value != 10 {
		t.Fatalf("Expected value 10, got %d", a.Value)
	}
	a.Set(100)
	if a.Value != 10 {
		t.Fatalf("Expected value 10, got %d", a.Value)
	}
	b := NewAvg(5)
	b.Set(100)
	if b.Value != 100 {
		t.Fatalf("Expected value 100, got %d", b.Value)
	}
	b.Init(20)
	if b.Value != 20 {
		t.Fatalf("Expected value 20, got %d", b.Value)
	}
	b.Add(100)
	if b.Value != 36 {
		t.Fatalf("Expected value 36, got %d", b.Value)
	}
}

func TestAvgCopy(t *testing.T) {
	a := NewAvg(5)
	a.Add(100)
	a.Add(300)
	b := a.Copy()
	if b.Value != 200 {
		t.Fatalf("Expected value 200, got %d", b.Value)
	}
	b.Add(400)
	b.Add(400)
	a.Add(100)
	a.Add(100)
	if a.Value != 150 {
		t.Fatalf("Expected value 150, got %d", a.Value)
	}
	if b.Value != 300 {
		t.Fatalf("Expected value 300, got %d", b.Value)
	}
}
