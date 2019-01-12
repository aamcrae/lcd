package lcd

import (
	"fmt"
	"image"
	"image/color"
	"math"
)

// Default threshold
const defaultThreshold = 50
const shrinkBB = false
const offMargin = 4
const onMargin = 2

// Bounding box
const (
    TL = iota
    TR = iota
    BR = iota
    BL = iota
)

// Segments.
const (
	S_TL, M_TL = iota, 1 << iota    // Top left
	S_T , M_T  = iota, 1 << iota    // Top
	S_TR, M_TR = iota, 1 << iota    // Top right
	S_BR, M_BR = iota, 1 << iota    // Bottom right
	S_B , M_B  = iota, 1 << iota    // Bottom
	S_BL, M_BL = iota, 1 << iota    // Bottom left
	S_M , M_M  = iota, 1 << iota    // Middle
    SEGMENTS = iota
)

type point struct {
	x int
	y int
}

type sample []point

type Lcd struct {
	name     string
	bb       []point
	scaled   []point
	line     int
	off      sample
	segments []sample
}

type Digit struct {
	index int
	lcd   *Lcd
	pos   point
	min   []int
	max   []int
}

type LcdDecoder struct {
	digits    []*Digit
	lcdMap    map[string]*Lcd
	threshold int
}

// There are 128 possible values in a 7 segment display,
// and this table maps selected values to a string result.
const X = 0

var resultTable = map[int]string{
	 X   |  X   |  X   |  X   |  X   |  X   |  X  : " ",
	 X   |  X   |  X   |  X   |  X   |  X   | M_M : "-",
	M_TL | M_T  | M_TR | M_BR | M_B  | M_BL |  X  : "0",
	 X   |  X   | M_TR | M_BR |  X   |  X   |  X  : "1",
	 X   | M_T  | M_TR |  X   | M_B  | M_BL | M_M : "2",
	 X   | M_T  | M_TR | M_BR | M_B  |  X   | M_M : "3",
	M_TL |  X   | M_TR | M_BR |  X   |  X   | M_M : "4",
	M_TL | M_T  |  X   | M_BR | M_B  |  X   | M_M : "5",
	M_TL | M_T  |  X   | M_BR | M_B  | M_BL | M_M : "6",
	M_TL | M_T  | M_TR | M_BR |  X   |  X   |  X  : "7",
	 X   | M_T  | M_TR | M_BR |  X   |  X   |  X  : "7",
	M_TL | M_T  | M_TR | M_BR | M_B  | M_BL | M_M : "8",
	M_TL | M_T  | M_TR | M_BR | M_B  |  X   | M_M : "9",
	M_TL | M_T  | M_TR | M_BR |  X   | M_BL | M_M : "A",
	M_TL |  X   |  X   | M_BR | M_B  | M_BL | M_M : "b",
	M_TL | M_T  |  X   |  X   | M_B  | M_BL |  X  : "C",
	 X   |  X   | M_TR | M_BR | M_B  | M_BL | M_M : "d",
	M_TL | M_T  |  X   |  X   | M_B  | M_BL | M_M : "E",
	M_TL | M_T  |  X   |  X   |  X   | M_BL | M_M : "F",
	M_TL |  X   |  X   | M_BR |  X   | M_BL | M_M : "h",
	M_TL |  X   | M_TR | M_BR |  X   | M_BL | M_M : "H",
	M_TL |  X   |  X   |  X   | M_B  | M_BL |  X  : "L",
	M_TL | M_T  | M_TR | M_BR |  X   | M_BL |  X  : "N",
	 X   |  X   |  X   | M_BR |  X   | M_BL | M_M : "n",
	 X   |  X   |  X   | M_BR | M_B  | M_BL | M_M : "o",
	M_TL | M_T  | M_TR |  X   |  X   | M_BL | M_M : "P",
	 X   |  X   |  X   |  X   |  X   | M_BL | M_M : "r",
	M_TL |  X   |  X   |  X   | M_B  | M_BL | M_M : "t",
}

func NewLcdDecoder() *LcdDecoder {
	return &LcdDecoder{[]*Digit{}, map[string]*Lcd{}, defaultThreshold}
}

func (l *LcdDecoder) AddLCD(name string, bb []int, width int) error {
	if _, ok := l.lcdMap[name]; ok {
		return fmt.Errorf("Duplicate LCD entry: %s", name)
	}
	lcd := &Lcd{name: name, bb: []point{point{0, 0}, point{bb[0], bb[1]}, point{bb[2], bb[3]}, point{bb[4], bb[5]}}, line: width}
	// Initialise the sample lists
	// Shrink the bounding box by 1/2 the width
	if shrinkBB {
		lcd.scaled = shrink(lcd.bb, lcd.line/2)
	} else {
		lcd.scaled = lcd.bb
	}
	tl := lcd.scaled[TL]
	tr := lcd.scaled[TR]
	br := lcd.scaled[BR]
	bl := lcd.scaled[BL]
	// Middle points.
	mr := split(tr, br, 2)[0]
	tmr := point{mr.x, mr.y - width/2}
	bmr := point{mr.x, mr.y + width/2}
	ml := split(tl, bl, 2)[0]
	tml := point{ml.x, ml.y - width/2}
	bml := point{ml.x, ml.y + width/2}
	// Build the 'off' sample using the middle blocks.
	lcd.off = buildOff(tl, tr, bmr, bml, width)
	lcd.off = append(lcd.off, buildOff(tml, tmr, br, bl, width)...)
	lcd.segments = make([]sample, SEGMENTS)
	// The assignments must match the bit allocation in
	// the lookup table.
	lcd.segments[S_TL] = vertical(tl, bml, width)
	lcd.segments[S_T] = horiz(tl, tr, width)
	lcd.segments[S_TR] = vertical(tr, bmr, -width)
	lcd.segments[S_BR] = vertical(tmr, br, -width)
	lcd.segments[S_B] = horiz(bl, br, -width)
	lcd.segments[S_BL] = vertical(tml, bl, width)
	lcd.segments[S_M] = horiz(tml, tmr, width)
	l.lcdMap[name] = lcd
	return nil
}

func (l *LcdDecoder) AddDigit(name string, x, y, min, max int) (int, error) {
	lcd, ok := l.lcdMap[name]
	if !ok {
		return 0, fmt.Errorf("Unknown LCD %s", name)
	}
	index := len(l.digits)
	d := &Digit{index, lcd, point{x, y}, []int{}, []int{}}
	d.min = make([]int, SEGMENTS, SEGMENTS)
	d.max = make([]int, SEGMENTS, SEGMENTS)
	for i := 0; i < SEGMENTS; i++ {
		d.min[i] = min
		d.max[i] = max
	}
	l.digits = append(l.digits, d)
	return index, nil
}

func (l *LcdDecoder) SetThreshold(threshold int) {
	l.threshold = threshold
}

func (l *LcdDecoder) Decode(img image.Image) ([]string, []bool) {
	strs := []string{}
	ok := []bool{}
	for _, d := range l.digits {
		lcd := d.lcd
		// Find off point.
		// off := scaledSample(img, d, lcd.off, 0, 0x10000)
		lookup := 0
		p := make([]int, len(lcd.segments))
		on := l.threshold
		//fmt.Printf("Digit %d Max = %d, Min = %d, On = %d, off = %d\n", i, d.max, d.min, on, off)
		for seg, s := range lcd.segments {
			p[seg] = scaledSample(img, d, s, d.min[seg], d.max[seg])
			if p[seg] >= on {
				lookup |= 1 << uint(seg)
			}
		}
		result, found := resultTable[lookup]
		//if !found {
		//fmt.Printf("Element not found, on = %d, off = %d, pixels: %v\n", on, off, p)
		//}
		strs = append(strs, result)
		ok = append(ok, found)
	}
	return strs, ok
}

// Build the 'off' sample list.
func buildOff(tl, tr, br, bl point, w int) []point {
	tl.x += w
	tl.y += w
	tr.x -= w
	tr.y += w
	br.x -= w
	br.y -= w
	bl.x += w
	bl.y -= w
	return bbToSample([]point{tl, tr, br, bl}, offMargin)
}

// Given a bounding box, create a list of unique points to be sampled.
func bbToSample(c []point, m int) sample {
	c[TL].x += m
	c[TL].y += m
	c[TR].x -= m
	c[TR].y += m
	c[BR].x -= m
	c[BR].y -= m
	c[BL].x += m
	c[BL].y -= m
	// Map to hold unique samples.
	points := make(map[point]struct{})
	down := length(c[TL], c[BL])
	across := length(c[TL], c[TR])
	for x := 0; x < across; x++ {
		ap := step(c[TL], c[TR], x, across)
		xoffs := ap.x - c[TL].x
		for y := 0; y < down; y++ {
			dp := step(c[TL], c[BL], y, down)
			dp.x += xoffs
			points[dp] = struct{}{}
		}
	}
	s := sample{}
	for p, _ := range points {
		s = append(s, p)
	}
	return s
}

// Build a vertical segment.
func vertical(p1, p2 point, w int) []point {
	if w < 0 {
		p1.y -= w
		p2.y += w
		return bbToSample([]point{point{p1.x + w, p1.y}, p1,
			p2, point{p2.x + w, p2.y}}, onMargin)
	} else {
		p1.y += w
		p2.y -= w
		return bbToSample([]point{p1, point{p1.x + w, p1.y},
			point{p2.x + w, p2.y}, p2}, onMargin)
	}
}

// Build a horizontal segment.
func horiz(p1, p2 point, w int) []point {
	if w < 0 {
		p1.x -= w
		p2.x += w
		return bbToSample([]point{point{p1.x, p1.y + w}, point{p2.x, p2.y + w},
			p2, p1}, onMargin)
	} else {
		p1.x += w
		p2.x -= w
		return bbToSample([]point{p1, p2,
			point{p2.x, p2.y + w}, point{p1.x, p1.y + w}}, onMargin)
	}
}

// Return the point closest to the ratio between 2 points
func step(p1, p2 point, inc, length int) point {
	var p point
	p.x = ((p2.x-p1.x)*inc)/length + p1.x
	p.y = ((p2.y-p1.y)*inc)/length + p1.y
	return p
}

// Return the line length.
func length(p1, p2 point) int {
	dx := p1.x - p2.x
	dy := p1.y - p2.y
	return (int(math.Sqrt(float64(dx*dx + dy*dy))))
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

// Shrink the bounding box by the selected size.
func shrink(bb []point, s int) []point {
	if s == 0 {
		return bb
	}
	nb := make([]point, 4)
	for _, c := range [][]int{{TL, BR}, {TR, BL}} {
		moveCorner(nb, bb, c[0], c[1], s)
		moveCorner(nb, bb, c[1], c[0], s)
	}
	return nb
}

func moveCorner(n []point, o []point, corner int, opps int, offset int) {
	if o[corner].x > o[opps].x {
		n[corner].x = o[corner].x - offset
	} else {
		n[corner].x = o[corner].x + offset
	}
	if o[corner].y > o[opps].y {
		n[corner].y = o[corner].y - offset
	} else {
		n[corner].y = o[corner].y + offset
	}
}

// Return an average of the sampled points as a int
// between 0 and 100, where 0 is lightest and 100 is darkest using
// the scale provided.
func scaledSample(img image.Image, d *Digit, slist sample, min, max int) int {
	gscaled := rawSample(img, d, slist)
	if gscaled < min {
		gscaled = min
	}
	if gscaled >= max {
		gscaled = max - 1
	}
	gpscale := (gscaled - min) * 100 / (max - min)
	//fmt.Printf("grey = %d, len = %d, result = %d, (%d%%)\n", gacc, len(slist), gscaled, gpscale)
	return gpscale
}

// Take a raw sample.
func rawSample(img image.Image, d *Digit, slist sample) int {
	var gacc int
	for _, s := range slist {
		c := img.At(d.pos.x+s.x, d.pos.y+s.y)
		pix := color.Gray16Model.Convert(c).(color.Gray16)
		gacc += int(pix.Y)
	}
	return 0x10000 - gacc/len(slist)
}

func (l *LcdDecoder) Calibrate(img image.Image) {
	for _, d := range l.digits {
		lcd := d.lcd
		// Find off point.
		min := rawSample(img, d, lcd.off)
		for seg, s := range lcd.segments {
			d.min[seg] = min
			d.max[seg] = rawSample(img, d, s)
		}
	}
}

// Mark the samples with a red cross.
func (l *LcdDecoder) MarkSamples(img *image.RGBA) {
	red := color.RGBA{255, 0, 0, 100}
	green := color.RGBA{0, 255, 0, 100}
	blue := color.RGBA{0, 0, 255, 255}
	white := color.RGBA{255, 255, 255, 255}
	for _, d := range l.digits {
		lcd := d.lcd
		drawCross(img, d, lcd.bb, white)
		if shrinkBB {
			drawCross(img, d, lcd.scaled, blue)
		}
		drawPoint(img, d, lcd.off, green)
		for _, s := range lcd.segments {
			drawPoint(img, d, s, red)
		}
	}
}

func drawPoint(img *image.RGBA, d *Digit, s sample, c color.Color) {
	for _, p := range s {
		img.Set(d.pos.x+p.x, d.pos.y+p.y, c)
	}
}
func drawCross(img *image.RGBA, d *Digit, s sample, c color.Color) {
	for _, p := range s {
		x := d.pos.x + p.x
		y := d.pos.y + p.y
		img.Set(x, y, c)
		for i := 1; i < 3; i++ {
			img.Set(x-i, y, c)
			img.Set(x+i, y, c)
			img.Set(x, y-i, c)
			img.Set(x, y+i, c)
		}
	}
}

func printSamples(s []point) {
	for _, p := range s {
		fmt.Printf("x = %d, y = %d\n", p.x, p.y)
	}
}
