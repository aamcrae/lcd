package lcd

import (
    "fmt"
    "image"
    "image/color"
)

// Default threshold
const defaultThreshold = 20
const shrinkBB = false

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
    s_tl |  X   |  X   |  X   | s_b  | s_bl |  X   : "L",
     X   |  X   |  X   | s_br | s_b  | s_bl | s_m  : "o",
    s_tl | s_t  | s_tr |  X   | s_b  | s_bl | s_m  : "P",
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
    // Shrink the bounding box by 1/2 the line width
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
    ml := split(tl, bl, 2)[0]
    // For sampling the 'off' value, sample the middle of each of the 2 halves by
    // Taking 3 samples through the axis and dropping the middle one.
    lcd.off = split(split(tl, tr, 2)[0], split(bl, br, 2)[0], 4)
    lcd.off = sample{lcd.off[0], lcd.off[2]}
    lcd.segments = make([]sample, 7)
    // The assignments must match the bit allocation in
    // the lookup table.
    // Top left
    lcd.segments[0] = split(ml, tl, 3)
    // Top
    lcd.segments[1] = split(tl, tr, 3)
    // Top right
    lcd.segments[2] = split(tr, mr, 3)
    // Bottom right
    lcd.segments[3] = split(mr, br, 3)
    // Bottom
    lcd.segments[4] = split(br, bl, 3)
    // Bottom left
    lcd.segments[5] = split(bl, ml, 3)
    // Middle
    lcd.segments[6] = split(ml, mr, 3)
    l.lcdMap[name] = lcd
    return nil
}

func (l *LcdDecoder) AddDigit(name string, x int, y int) (int, error) {
    lcd, ok := l.lcdMap[name]
    if !ok {
        return 0, fmt.Errorf("Unknown LCD %s", name)
    }
    index := len(l.digits)
    l.digits = append(l.digits, &Digit{index, lcd, point{x, y}})
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
        // Considered on if darker by a set threshold.
        on := off - ((off * l.threshold) / 100)
        var decimal bool = false
        if len(lcd.decimal) != 0 && on >= takeSample(img, d, lcd.decimal) {
            decimal = true
        }
        lookup := 0
        p := make([]int, len(lcd.segments))
        for seg, s := range lcd.segments {
            pixel := takeSample(img, d, s)
            if on >= pixel {
                lookup |= 1 << uint(seg)
            }
            p[seg] = pixel
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

// Return an average of the sampled points.
func takeSample(img image.Image, d *Digit, slist sample) int {
    var g int
    for _, s := range slist {
        pix := color.Gray16Model.Convert(img.At(d.pos.x + s.x, d.pos.y + s.y)).(color.Gray16)
        g += int(pix.Y)
    }
    return g / len(slist)
}

// Mark the samples with a red cross.
func (l *LcdDecoder) MarkSamples(img *image.RGBA) {
    red := color.RGBA{255, 0, 0, 255}
    green := color.RGBA{0, 255, 0, 255}
    blue := color.RGBA{0, 0, 255, 255}
    white := color.RGBA{255, 255, 255, 255}
    for _, d := range l.digits {
        lcd := d.lcd
        drawCross(img, d, lcd.bb, white)
        if shrinkBB {
            drawCross(img, d, lcd.scaled, blue)
        }
        drawCross(img, d, lcd.off, green)
        if len(lcd.decimal) != 0 {
            drawCross(img, d, lcd.decimal, red)
        }
        for _, s := range lcd.segments {
            drawCross(img, d, s, red)
        }
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
