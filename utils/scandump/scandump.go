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

package main

import (
	"flag"
	"fmt"
	"image"
	"io/ioutil"
	"log"
	"os"

	"github.com/aamcrae/lcd"
	"gopkg.in/yaml.v3"
)

var configFile = flag.String("config", "config", "Configuration file")
var input = flag.String("input", "input.jpg", "Input file")
var calImage = flag.String("image", "", "Calibration image")
var calibration = flag.String("calibration", "", "Calibration cache file")
var digits = flag.String("digits", "888888888888", "Digits for calibration")
var rewrite = flag.String("rewrite", "", "Write calibration to file")

func init() {
	flag.Parse()
}

type config struct {
	Meter struct {
		Source        string
		Rotate        float64
		lcd.LcdConfig `yaml:",inline"`
	}
}

func main() {
	s, err := ioutil.ReadFile(*configFile)
	if err != nil {
		log.Fatalf("Can't read config %s: %v", *configFile, err)
	}
	var conf config
	err = yaml.Unmarshal([]byte(s), &conf)
	if err != nil {
		log.Fatalf("config parse fail %s: %v", *configFile, err)
	}
	var lc lcd.LcdConfig
	lc.Threshold = conf.Meter.Threshold
	lc.Offset = conf.Meter.Offset
	copy(lc.Lcd, conf.Meter.Lcd)
	copy(lc.Digit, conf.Meter.Digit)
	l, err := lcd.CreateLcdDecoder(lc)
	if len(*calibration) > 0 {
		if _, err := l.RestoreFromFile(*calibration); err != nil {
			log.Fatalf("%s: %v\n", *calibration, err)
		}
	}
	if len(*calImage) > 0 {
		img, err := lcd.ReadImage(*calImage)
		if err != nil {
			log.Fatalf("%v", err)
		}
		l.Preset(img, *digits)
	}
	inf, err := os.Open(*input)
	if err != nil {
		log.Fatalf("Failed to open %s: %v", *input, err)
	}
	defer inf.Close()
	img, _, err := image.Decode(inf)
	if err != nil {
		log.Fatalf("Failed to read %s: %v", *input, err)
	}
	if conf.Meter.Rotate != 0 {
		img = lcd.RotateImage(img, conf.Meter.Rotate)
	}
	res := l.Decode(img)
	for i := range res.Decodes {
		d := res.Decodes[i]
		fmt.Printf("segment %d = '%s', ok = %v, bits = %02x\n", i, d.Str, d.Valid, res.Scans[i].Mask)
	}
	fmt.Printf("Digit |  Off |  TL  |  TM  |  TR  |  BR  |  BM  |  BL  |  MM  |\n")
	for i, _ := range l.Digits {
		min := []int{} // d.Min()
		off := 0       // d.Off(img)
		max := []int{} // d.Max()
		fmt.Printf("  %-2d  | %-5d|", i, off)
		for _, v := range res.Scans[i].Segments {
			fmt.Printf(" %-5d|", v)
		}
		fmt.Printf("\n")
		fmt.Printf("         Min |")
		for _, v := range min {
			fmt.Printf(" %-5d|", v)
		}
		fmt.Printf("\n")
		fmt.Printf("         Max |")
		for _, v := range max {
			fmt.Printf(" %-5d|", v)
		}
		fmt.Printf("\n")
		fmt.Printf("        Perc |")
		for i, v := range res.Scans[i].Segments {
			fmt.Printf(" %-5d|", perc(max[i], min[i], v))
		}
		fmt.Printf("\n")
	}
	if len(*rewrite) > 0 {
		out, err := os.Create(*rewrite)
		if err != nil {
			log.Fatalf("Failed to create %s: %v", *rewrite, err)
		}
		defer out.Close()
		b, err := lcd.DigitsToSegments(*digits)
		if err != nil {
			log.Fatalf("DigitsToSegments: %v", err)
		}
		if len(b) != len(l.Digits) {
			log.Fatalf("Wrong digit count (%d), expected (%d)", len(b), len(l.Digits))
		}
		for i, _ := range l.Digits {
			min := []int{} // d.Min()
			max := []int{} // d.Max()
			for j, s := range res.Scans[i].Segments {
				if ((1 << uint(j)) & b[i]) != 0 {
					fmt.Fprintf(out, "%d,%d,%d,%d\n", i, j, min[j], s)
				} else {
					fmt.Fprintf(out, "%d,%d,%d,%d\n", i, j, s, max[j])
				}
			}
		}
	}
}

func perc(max, min, v int) int {
	return (v - min) * 100 / (max - min)
}
