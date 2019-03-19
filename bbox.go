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

type Point struct {
	X int
	Y int
}

type BBox [4]Point

// Make a bounding box of the given width.
// Shrink the base by width as well.
func MakeBB(s1, s2, e1, e2 Point, w int) BBox {
	var bb BBox
	bb[TL] = Adjust(s1, s2, w)
	bb[BL] = Adjust(s2, s1, w)
	ne1 := Adjust(e1, e2, w)
	ne2 := Adjust(e2, e1, w)
	bb[TR] = Adjust(bb[TL], ne1, w)
	bb[BR] = Adjust(bb[BL], ne2, w)
	return bb
}

// Make a inner bounding box with the given margin.
func MakeInnerBB(b BBox, m int) BBox {
	tl := Adjust(b[TL], b[TR], m)
	tr := Adjust(b[TR], b[TL], m)
	bl := Adjust(b[BL], b[BR], m)
	br := Adjust(b[BR], b[BL], m)
	nb := BBox{}
	nb[TL] = Adjust(tl, bl, m)
	nb[TR] = Adjust(tr, br, m)
	nb[BL] = Adjust(bl, tl, m)
	nb[BR] = Adjust(br, tr, m)
	return nb
}

// Return a list of all the points in the bounding box.
func FillBB(bb BBox) []Point {
	points := []Point{}
	// Find the edges.
	minx := bb[0].X
	maxx :=	bb[0].X
	miny := bb[0].Y
	maxy :=	bb[0].Y
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
			if InBB(bb, p) {
				points = append(points, p)
			}
		}
	}
	return points
}

func InBB(bb BBox, p Point) bool {
	limit := Point{10000, p.Y}
	// Check whether ray hits a vertex.
	var vertex int
	for _, v := range bb {
		if v.Y == p.Y {
			vertex++;
		}
	}
	var count int
	for i := range bb {
		next := (i + 1) % len(bb)
		if intersect(bb[i], bb[next], p, limit) {
			if orientation(bb[i], p, bb[next]) == 0 {
				return onSegment(bb[i], p, bb[next])
			}
            count++
		}
	}
	if vertex > 0 && count == 3 {
		count--
	}
	return (count & 1) != 0
}

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
	v := (q.Y - p.Y) * (r.X - q.X) - (q.X - p.X) * (r.Y - q.Y)
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

// return an adjusted point that is closer to e by the given amount.
func Adjust(s, e Point, amt int) Point {
	x := e.X - s.X
	y := e.Y - s.Y
	length := int(math.Round(math.Sqrt(float64(x*x)+float64(y*y)) + 0.5))
	return Point{s.X + amt * x / length, s.Y + amt * y / length}
}

// Return a slice of points that splits the line into sections.
func Split(start, end Point, sections int) []Point {
	lx := end.X - start.X
	ly := end.Y - start.Y
	p := make([]Point, sections-1)
	for i := 1; i < sections; i++ {
		p[i-1] = Point{start.X + lx*i/sections, start.Y + ly*i/sections}
	}
	return p
}

// Build a new point list, adding in x and y.
func Offset(p []Point, x, y int) []Point {
	np := make([]Point, len(p), len(p))
	for i := range p {
		np[i].X = p[i].X + x
		np[i].Y = p[i].Y + y
	}
	return np
}

// Build a new bounding box offset by x and y.
func OffsetBB(bb BBox, x, y int) BBox {
	var nb BBox
	for i := range bb {
		nb[i].X = bb[i].X
		nb[i].Y = bb[i].Y
	}
	return nb
}
