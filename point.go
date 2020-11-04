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
	"math"
)

type Point struct {
	X int
	Y int
}

// List of points.
type PList []Point

// Return an adjusted point that is closer to e by the given amount.
func Adjust(s, e Point, adj int) Point {
	x := e.X - s.X
	y := e.Y - s.Y
	length := int(math.Round(math.Sqrt(float64(x*x)+float64(y*y)) + 0.5))
	return Point{s.X + adj*x/length, s.Y + adj*y/length}
}

// Return a list of points that splits the line (identified by start and
// end) into a number of sections e.g if 3 sections are requested, a list of 2 points
// are returned, representing the points 1/3 and 2/3 along the line.
func Split(start, end Point, sections int) PList {
	lx := end.X - start.X
	ly := end.Y - start.Y
	p := make(PList, sections-1)
	for i := 1; i < sections; i++ {
		p[i-1] = Point{start.X + lx*i/sections, start.Y + ly*i/sections}
	}
	return p
}

// Return a new point offset from this point by x, y.
func (p Point) Offset(x, y int) Point {
	return Point{p.X + x, p.Y + y}
}

// Create a new point list representing a square centered at p of width w.
func (p Point) Block(w int) PList {
	w = w / 2
	var s PList
	for x := p.X - w; x <= p.X+w; x++ {
		for y := p.Y - w; y <= p.Y+w; y++ {
			s = append(s, Point{x, y})
		}
	}
	return s
}

// Create a new point list from the point list, adding a x and y offset to each point.
func (p PList) Offset(x, y int) PList {
	np := make(PList, len(p), len(p))
	for i := range p {
		np[i].X = p[i].X + x
		np[i].Y = p[i].Y + y
	}
	return np
}

// Print the point list.
func (pl PList) Print() {
	for _, p := range pl {
		fmt.Printf("x = %d, y = %d\n", p.X, p.Y)
	}
}
