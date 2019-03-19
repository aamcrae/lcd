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

type bbox struct {
	tl, tr, br, bl point
}

// Make a bounding box of the given width.
// Shrink the base by width as well.
func mkbb(s1, s2, e1, e2 point, w int) bbox {
	bb := bbox{}
	bb.tl = adjust(s1, s2, w)
	bb.bl = adjust(s2, s1, w)
	ne1 := adjust(e1, e2, w)
	ne2 := adjust(e2, e1, w)
	bb.tr = adjust(bb.tl, ne1, w)
	bb.br = adjust(bb.bl, ne2, w)
	return bb
}

// Make a inner bounding box with the given margin.
func mkinnerbb(b bbox, m int) bbox {
	tl := adjust(b.tl, b.tr, m)
	tr := adjust(b.tr, b.tl, m)
	bl := adjust(b.bl, b.br, m)
	br := adjust(b.br, b.bl, m)
	nb := bbox{}
	nb.tl = adjust(tl, bl, m)
	nb.tr = adjust(tr, br, m)
	nb.bl = adjust(bl, tl, m)
	nb.br = adjust(br, tr, m)
	return nb
}

// Return a list of all the points in the bounding box.
func fill(bb bbox) []point {
	points := []point{}
	// Find the edges.
	minx := bb.tl.x
	maxx :=	bb.tl.x
	miny := bb.tl.y
	maxy :=	bb.tl.y
	l := []point{bb.tr, bb.br, bb.bl}
	for _, b := range l {
		if minx > b.x {
			minx = b.x
		}
		if maxx < b.x {
			maxx = b.x
		}
		if miny > b.y {
			miny = b.y
		}
		if maxy < b.y {
			maxy = b.y
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

func inBB(bb bbox, p point) bool {
	limit := point{10000, p.y}
	l := []point{bb.tl, bb.tr, bb.br, bb.bl}
	// Check whether ray hits a vertex.
	var vertex int
	for _, v := range l {
		if v.y == p.y {
			vertex++;
		}
	}
	var count int
	for i := range l {
		next := (i + 1) % len(l)
		if intersect(l[i], l[next], p, limit) {
			if orientation(l[i], p, l[next]) == 0 {
				return onSegment(l[i], p, l[next])
			}
            count++
		}
	}
	if vertex > 0 && count == 3 {
		count--
	}
	return (count & 1) != 0
}

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
	v := (q.y - p.y) * (r.x - q.x) - (q.x - p.x) * (r.y - q.y)
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

// return an adjusted point that is closer to e by the given amount.
func adjust(s, e point, amt int) point {
	x := e.x - s.x
	y := e.y - s.y
	length := int(math.Round(math.Sqrt(float64(x*x)+float64(y*y)) + 0.5))
	return point{s.x + amt * x / length, s.y + amt * y / length}
}

// Return the point closest to the ratio between 2 points
func step(p1, p2 point, inc, length int) point {
	var p point
	p.x = ((p2.x-p1.x)*inc)/length + p1.x
	p.y = ((p2.y-p1.y)*inc)/length + p1.y
	return p
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

// Build a new point list, adding in x and y.
func offset(p []point, x, y int) []point {
	np := make([]point, len(p), len(p))
	for i := range p {
		np[i].x = p[i].x + x
		np[i].y = p[i].y + y
	}
	return np
}

// Build a new bounding box offset by x and y.
func offsetbb(bb bbox, x, y int) bbox {
	nb := bbox{}
	nb.tl.x = bb.tl.x + x
	nb.tl.y = bb.tl.y + y
	nb.tr.x = bb.tr.x + x
	nb.tr.y = bb.tr.y + y
	nb.br.x = bb.br.x + x
	nb.br.y = bb.br.y + y
	nb.bl.x = bb.bl.x + x
	nb.bl.y = bb.bl.y + y
	return nb
}
