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

// Avg represents a moving average of a value.
// A list of historical values is maintained and when new values
// are added, the moving average is recalculated.
type Avg struct {
	size    int
	Value   int
	total   int
	history []int
}

// Create a new moving average structure, size indicating the
// number of historical values to be kept.
func NewAvg(size int) *Avg {
	return &Avg{size: size}
}

// Init the moving average with a default value.
func (m *Avg) Init(v int) {
	for i := 0; i < m.size; i++ {
		m.Add(v)
	}
}

// Copy the moving average.
func (m *Avg) Copy() *Avg {
	na := new(Avg)
	na.size = m.size
	na.Value = m.Value
	na.total = m.total
	na.history = make([]int, len(m.history))
	copy(na.history, m.history)
	return na
}

// If not already initialised, init using this value.
func (m *Avg) SetDefault(v int) {
	if len(m.history) == 0 {
		m.Init(v)
	}
}

// Add a new value to the history slice, drop the oldest,
// and recalculate the average.
func (m *Avg) Add(v int) {
	m.history = append(m.history, v)
	m.total += v
	if len(m.history) > m.size {
		m.total -= m.history[0]
		m.history = m.history[1:]
	}
	m.Value = m.total / len(m.history)
}
