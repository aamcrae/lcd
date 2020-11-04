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

// Bounding box
const (
	TL = iota
	TR = iota
	BR = iota
	BL = iota
)

type BBox [4]Point

// Make a bounding box for a segment.
// Shrink the base by width as well.
func SegmentBB(s1, s2, e1, e2 Point, w, m int) BBox {
	var bb BBox
	bb[TL] = Adjust(s1, s2, w+m)
	bb[TR] = Adjust(s2, s1, w+m)
	ne1 := Adjust(e1, e2, w+m)
	ne2 := Adjust(e2, e1, w+m)
	bb[BL] = Adjust(bb[TL], ne1, w-m)
	bb[BR] = Adjust(bb[TR], ne2, w-m)
	bb[TL] = Adjust(bb[TL], ne1, m)
	bb[TR] = Adjust(bb[TR], ne2, m)
	return bb
}

// Create a new inner bounding box with the given margin.
func (bb BBox) Inner(m int) BBox {
	tl := Adjust(bb[TL], bb[TR], m)
	tr := Adjust(bb[TR], bb[TL], m)
	bl := Adjust(bb[BL], bb[BR], m)
	br := Adjust(bb[BR], bb[BL], m)
	nb := BBox{}
	nb[TL] = Adjust(tl, bl, m)
	nb[TR] = Adjust(tr, br, m)
	nb[BL] = Adjust(bl, tl, m)
	nb[BR] = Adjust(br, tr, m)
	return nb
}

// Return a list of all the points in the bounding box.
func (bb BBox) Fill() PList {
	points := PList{}
	// Find min and max X & Y that completely covers the bounding box.
	minx := bb[0].X
	maxx := bb[0].X
	miny := bb[0].Y
	maxy := bb[0].Y
	for i := 1; i < len(bb); i++ {
		if minx > bb[i].X {
			minx = bb[i].X
		}
		if maxx < bb[i].X {
			maxx = bb[i].X
		}
		if miny > bb[i].Y {
			miny = bb[i].Y
		}
		if maxy < bb[i].Y {
			maxy = bb[i].Y
		}
	}
	for y := miny; y <= maxy; y++ {
		for x := minx; x <= maxx; x++ {
			p := Point{x, y}
			if bb.In(p) {
				points = append(points, p)
			}
		}
	}
	return points
}

// In returns true if the point is in the bounding box.
func (bb BBox) In(p Point) bool {
	limit := Point{10000, p.Y}
	var count int
	for i := range bb {
		next := (i + 1) % len(bb)
		if intersect(bb[i], bb[next], p, limit) {
			if orientation(bb[i], p, bb[next]) == 0 {
				return onSegment(bb[i], p, bb[next])
			}
			// Check whether ray passes through the vertex, in which
			// case only count it once.
			if p.Y == bb[i].Y {
				if bb[next].Y <= p.Y {
					count++
				}
			} else if p.Y == bb[next].Y {
				if bb[i].Y <= p.Y {
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
func intersect(p1, q1, p2, q2 Point) bool {
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

func orientation(p, q, r Point) int {
	v := (q.Y-p.Y)*(r.X-q.X) - (q.X-p.X)*(r.Y-q.Y)
	if v == 0 {
		return 0
	}
	if v > 0 {
		return 1
	} else {
		return 2
	}
}

func onSegment(p, q, r Point) bool {
	if q.X <= max(p.X, r.X) && q.X >= min(p.X, r.X) &&
		q.Y <= max(p.Y, r.Y) && q.Y >= min(p.Y, r.Y) {
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

// Create a new bounding box offset by x and y.
func (bb BBox) Offset(x, y int) BBox {
	var nb BBox
	for i := range bb {
		nb[i].X = bb[i].X + x
		nb[i].Y = bb[i].Y + y
	}
	return nb
}
