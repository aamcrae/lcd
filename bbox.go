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
	"math"
)

// Bounding box
const (
	TL = iota
	TR = iota
	BR = iota
	BL = iota
)

type point struct {
	x int
	y int
}

type bbox [4]point

// Make a bounding box for a segment.
// Shrink the base by width as well.
func segmentBB(s1, s2, e1, e2 point, w, m int) bbox {
	var bb bbox
	bb[TL] = adjust(s1, s2, w + m)
	bb[TR] = adjust(s2, s1, w + m)
	ne1 := adjust(e1, e2, w + m)
	ne2 := adjust(e2, e1, w + m)
	bb[BL] = adjust(bb[TL], ne1, w - m)
	bb[BR] = adjust(bb[TR], ne2, w - m)
	bb[TL] = adjust(bb[TL], ne1, m)
	bb[TR] = adjust(bb[TR], ne2, m)
	return bb
}

// Make a inner bounding box with the given margin.
func innerBB(b bbox, m int) bbox {
	tl := adjust(b[TL], b[TR], m)
	tr := adjust(b[TR], b[TL], m)
	bl := adjust(b[BL], b[BR], m)
	br := adjust(b[BR], b[BL], m)
	nb := bbox{}
	nb[TL] = adjust(tl, bl, m)
	nb[TR] = adjust(tr, br, m)
	nb[BL] = adjust(bl, tl, m)
	nb[BR] = adjust(br, tr, m)
	return nb
}

// Return a list of all the points in the bounding box.
func fillBB(bb bbox) []point {
	points := []point{}
	// Find min and max X & Y that completely covers the bounding box.
	minx := bb[0].x
	maxx := bb[0].x
	miny := bb[0].y
	maxy := bb[0].y
	for i := 1; i < len(bb); i++ {
		if minx > bb[i].x {
			minx = bb[i].x
		}
		if maxx < bb[i].x {
			maxx = bb[i].x
		}
		if miny > bb[i].y {
			miny = bb[i].y
		}
		if maxy < bb[i].y {
			maxy = bb[i].y
		}
	}
	for y := miny; y <= maxy; y++ {
		for x := minx; x <= maxx; x++ {
			p := point{x, y}
			if inBB(bb, p) {
				points = append(points, p)
			}
		}
	}
	return points
}

// inBB returns true if the point is in the bounding box.
func inBB(bb bbox, p point) bool {
	limit := point{10000, p.y}
	var count int
	for i := range bb {
		next := (i + 1) % len(bb)
		if intersect(bb[i], bb[next], p, limit) {
			if orientation(bb[i], p, bb[next]) == 0 {
				return onSegment(bb[i], p, bb[next])
			}
			// Check whether ray passes through the vertex, in which
			// case only count it once.
			if p.y == bb[i].y {
				if bb[next].y <= p.y {
					count++
				}
			} else if p.y == bb[next].y {
				if bb[i].y <= p.y {
					count++
				}
			} else {
				count++
			}
		}
	}
	return (count & 1) != 0
}

// intersect returns true if the lines (p1,q1) and (p2,q2) intersect.
func intersect(p1, q1, p2, q2 point) bool {
	o1 := orientation(p1, q1, p2)
	o2 := orientation(p1, q1, q2)
	o3 := orientation(p2, q2, p1)
	o4 := orientation(p2, q2, q1)
	if o1 != o2 && o3 != o4 {
		return true
	}
	if o1 == 0 && onSegment(p1, p2, q1) {
		return true
	}
	if o2 == 0 && onSegment(p1, q2, q1) {
		return true
	}
	if o3 == 0 && onSegment(p2, p1, q2) {
		return true
	}
	if o4 == 0 && onSegment(p2, q1, q2) {
		return true
	}
	return false
}

func orientation(p, q, r point) int {
	v := (q.y-p.y)*(r.x-q.x) - (q.x-p.x)*(r.y-q.y)
	if v == 0 {
		return 0
	}
	if v > 0 {
		return 1
	} else {
		return 2
	}
}

func onSegment(p, q, r point) bool {
	if q.x <= max(p.x, r.x) && q.x >= min(p.x, r.x) &&
		q.y <= max(p.y, r.y) && q.y >= min(p.y, r.y) {
		return true
	}
	return false
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}

// Build a square sample centered at p of width w.
func blockSample(p point, w int) sample {
	w = w / 2
	var s sample
	for x := p.x - w; x <= p.x+w; x++ {
		for y := p.y - w; y <= p.y+w; y++ {
			s = append(s, point{x, y})
		}
	}
	return s
}

// return an adjusted point that is closer to e by the given amount.
func adjust(s, e point, amt int) point {
	x := e.x - s.x
	y := e.y - s.y
	length := int(math.Round(math.Sqrt(float64(x*x)+float64(y*y)) + 0.5))
	return point{s.x + amt*x/length, s.y + amt*y/length}
}

// Return a slice of points that splits the line into sections.
func split(start, end point, sections int) []point {
	lx := end.x - start.x
	ly := end.y - start.y
	p := make([]point, sections-1)
	for i := 1; i < sections; i++ {
		p[i-1] = point{start.x + lx*i/sections, start.y + ly*i/sections}
	}
	return p
}

// Build a new point list, adding x and y to each point.
func offset(p []point, x, y int) []point {
	np := make([]point, len(p), len(p))
	for i := range p {
		np[i].x = p[i].x + x
		np[i].y = p[i].y + y
	}
	return np
}

// Build a new bounding box offset by x and y.
func offsetBB(bb bbox, x, y int) bbox {
	var nb bbox
	for i := range bb {
		nb[i].x = bb[i].x + x
		nb[i].y = bb[i].y + y
	}
	return nb
}
