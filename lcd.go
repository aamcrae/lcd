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

// Corners.
const (
    TL = 0
    TR = 1
    BR = 2
    BL = 3
)

type point struct {
    x int
    y int
}

type sample []point

type Lcd struct {
    name string
    bb []point
    scaled []point
    line int
    off sample
    decimal sample
    segments []sample
}

type Digit struct {
    index int
    lcd *Lcd
    pos point
    min int
    max int
}

type LcdDecoder struct {
    digits []*Digit
    lcdMap map[string]*Lcd
    threshold int
}

// There are 128 possible values in a 7 segment display,
// and this table maps known values to a string result.
// The segments are used as a bit in 7 bit value, and mapped as:
// top left     bit 0
// top          bit 1
// top right    bit 2
// bottom right bit 3
// bottom       bit 4
// bottom left  bit 5
// middle       bit 6
const (
  X = 0
  s_tl = 1
  s_t = 2
  s_tr = 4
  s_br = 8
  s_b = 0x10
  s_bl = 0x20
  s_m = 0x40
)

var resultTable = map[int]string {
     X   |  X   |  X   |  X   |  X   |  X   |  X   : " ",
     X   |  X   |  X   |  X   |  X   |  X   | s_m  : "-",
    s_tl | s_t  | s_tr | s_br | s_b  | s_bl |  X   : "0",
     X   |  X   | s_tr | s_br |  X   |  X   |  X   : "1",
     X   | s_t  | s_tr |  X   | s_b  | s_bl | s_m  : "2",
     X   | s_t  | s_tr | s_br | s_b  |  X   | s_m  : "3",
    s_tl |  X   | s_tr | s_br |  X   |  X   | s_m  : "4",
    s_tl | s_t  |  X   | s_br | s_b  |  X   | s_m  : "5",
    s_tl | s_t  |  X   | s_br | s_b  | s_bl | s_m  : "6",
    s_tl | s_t  | s_tr | s_br |  X   |  X   |  X   : "7",
     X   | s_t  | s_tr | s_br |  X   |  X   |  X   : "7",
    s_tl | s_t  | s_tr | s_br | s_b  | s_bl | s_m  : "8",
    s_tl | s_t  | s_tr | s_br | s_b  |  X   | s_m  : "9",
    s_tl | s_t  | s_tr | s_br |  X   | s_bl | s_m  : "A",
    s_tl |  X   |  X   | s_br | s_b  | s_bl | s_m  : "b",
    s_tl | s_t  |  X   |  X   | s_b  | s_bl |  X   : "C",
     X   |  X   | s_tr | s_br | s_b  | s_bl | s_m  : "d",
    s_tl | s_t  |  X   |  X   | s_b  | s_bl | s_m  : "E",
    s_tl | s_t  |  X   |  X   |  X   | s_bl | s_m  : "F",
    s_tl |  X   |  X   | s_br |  X   | s_bl | s_m  : "h",
    s_tl |  X   | s_tr | s_br |  X   | s_bl | s_m  : "H",
    s_tl |  X   |  X   |  X   | s_b  | s_bl |  X   : "L",
    s_tl | s_t  | s_tr | s_br |  X   | s_bl |  X   : "N",
     X   |  X   |  X   | s_br |  X   | s_bl | s_m  : "n",
     X   |  X   |  X   | s_br | s_b  | s_bl | s_m  : "o",
    s_tl | s_t  | s_tr |  X   |  X   | s_bl | s_m  : "P",
     X   |  X   |  X   |  X   |  X   | s_bl | s_m  : "r",
    s_tl |  X   |  X   |  X   | s_b  | s_bl | s_m  : "t",
}

func NewLcdDecoder() *LcdDecoder {
   return &LcdDecoder{[]*Digit{}, map[string]*Lcd{}, defaultThreshold}
}

func (l *LcdDecoder) AddLCD(name string, bb []int, width int, decimal []int) error {
    if _, ok := l.lcdMap[name]; ok {
        return fmt.Errorf("Duplicate LCD entry: %s", name)
    }
    lcd := &Lcd{name:name, bb:[]point{point{0,0}, point{bb[0],bb[1]}, point{bb[2],bb[3]}, point{bb[4],bb[5]}}, line:width}
    // Initialise the sample lists
    if len(decimal) == 2 {
        lcd.decimal = []point{{lcd.bb[BR].x + decimal[0], lcd.bb[BR].y + decimal[1]}}
    }
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
    lcd.segments = make([]sample, 7)
    // The assignments must match the bit allocation in
    // the lookup table.
    // Top left
    lcd.segments[0] = vertical(tl, bml, width)
    // Top
    lcd.segments[1] = horiz(tl, tr, width)
    // Top right
    lcd.segments[2] = vertical(tr, bmr, -width)
    // Bottom right
    lcd.segments[3] = vertical(tmr, br, -width)
    // Bottom
    lcd.segments[4] = horiz(bl, br, -width)
    // Bottom left
    lcd.segments[5] = vertical(tml, bl, width)
    // Middle
    lcd.segments[6] = horiz(tml, tmr, width)
    l.lcdMap[name] = lcd
    return nil
}

func (l *LcdDecoder) AddDigit(name string, x, y, min, max int) (int, error) {
    lcd, ok := l.lcdMap[name]
    if !ok {
        return 0, fmt.Errorf("Unknown LCD %s", name)
    }
    index := len(l.digits)
    l.digits = append(l.digits, &Digit{index, lcd, point{x, y}, min, max})
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
        off := takeSample(img, d, lcd.off)
        var decimal bool = false
        lookup := 0
        p := make([]int, len(lcd.segments))
        on := l.threshold
        //fmt.Printf("Digit %d Max = %d, Min = %d, On = %d, off = %d\n", i, d.max, d.min, on, off)
        for seg, s := range lcd.segments {
            p[seg] = takeSample(img, d, s)
            if p[seg] >= on {
                lookup |= 1 << uint(seg)
            }
        }
        if len(lcd.decimal) != 0 && on >= takeSample(img, d, lcd.decimal) {
            decimal = true
        }
        result, found := resultTable[lookup]
        if !found {
            fmt.Printf("Element not found, on = %d, off = %d, pixels: %v\n", on, off, p)
        }
        if decimal {
            result = result + "."
        }
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
    for x:= 0; x < across; x++ {
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
    p.x = ((p2.x - p1.x) * inc) / length + p1.x
    p.y = ((p2.y - p1.y) * inc) / length + p1.y
    return p
}

// Return the line length.
func length(p1, p2 point) int {
    dx := p1.x - p2.x
    dy := p1.y - p2.y
    return(int(math.Sqrt(float64(dx * dx + dy * dy))))
}

// Return a slice of points that splits the line into sections.
func split(start, end point, sections int) []point {
    lx := end.x - start.x
    ly := end.y - start.y
    p := make([]point, sections - 1)
    for i := 1; i < sections; i++ {
        p[i-1] = point{start.x + lx * i / sections, start.y + ly * i / sections}
    }
    return p
}

// Shrink the bounding box by the selected size.
func shrink(bb []point, s int) ([]point) {
    if s == 0 {
        return bb
    }
    nb := make([]point, 4)
    for _, c := range [][]int{{TL,BR}, {TR,BL}} {
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
func takeSample(img image.Image, d *Digit, slist sample) int {
    //var acc int
    var gacc int
    for _, s := range slist {
        c := img.At(d.pos.x + s.x, d.pos.y + s.y)
        //r, g, b, _ := c.RGBA()
        //p := ((r + g + b) * 100) / max
        //acc += int(r) + int(g) + int(b)
        pix := color.Gray16Model.Convert(c).(color.Gray16)
        gacc += int(pix.Y)
        //fmt.Printf("rgb = %d, %d %d (%d%%), grey %d (%d%%)\n", r, g, b, p,
            //pix.Y, (int(pix.Y) * 100) / 0x10000)
    }
    //scaled := 0x10000 - acc / (len(slist) * 3)
    //if scaled < d.min {
        //scaled = d.min
    //}
    //if scaled >= d.max {
        //scaled = d.max - 1
    //}
    gscaled := 0x10000 - gacc / len(slist)
    if gscaled < d.min {
        gscaled = d.min
    }
    if gscaled >= d.max {
        gscaled = d.max - 1
    }
    //pscale := (scaled - d.min) * 100 / (d.max - d.min)
    gpscale := (gscaled - d.min) * 100 / (d.max - d.min)
    //fmt.Printf("acc = %d, len = %d, result = %d, (%d%%)\n", acc, len(slist), scaled, pscale)
    //fmt.Printf("grey = %d, len = %d, result = %d, (%d%%)\n", gacc, len(slist), gscaled, gpscale)
    return gpscale
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
        if len(lcd.decimal) != 0 {
            drawPoint(img, d, lcd.decimal, red)
        }
        for _, s := range lcd.segments {
            drawPoint(img, d, s, red)
        }
    }
}

func drawPoint(img *image.RGBA, d *Digit, s sample, c color.Color) {
    for _, p := range s {
        img.Set(d.pos.x + p.x, d.pos.y + p.y, c)
    }
}
func drawCross(img *image.RGBA, d *Digit, s sample, c color.Color) {
    for _, p := range s {
        x := d.pos.x + p.x
        y := d.pos.y + p.y
        img.Set(x, y, c)
        for i := 1; i < 3; i++ {
            img.Set(x - i, y, c)
            img.Set(x + i, y, c)
            img.Set(x, y - i, c)
            img.Set(x, y + i, c)
        }
    }
}

func printSamples(s []point) {
    for _, p := range s {
        fmt.Printf("x = %d, y = %d\n", p.x, p.y)
    }
}
