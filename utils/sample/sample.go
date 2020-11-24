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
	"image/color"
	"log"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/aamcrae/config"
	"github.com/aamcrae/lcd"
)

var output = flag.String("output", "output.jpg", "output jpeg file")
var configFile = flag.String("config", "config", "Configuration file")
var section = flag.String("section", "", "Configuration section (if any)")
var input = flag.String("input", "", "Input file")
var decode = flag.Bool("decode", false, "Decode digits in image")
var fill = flag.Bool("fill", true, "Fill in segments")
var calibrate = flag.Bool("calibrate", false, "Calibrate using image")
var digits = flag.String("digits", "888888888888", "Digits for calibration")

func init() {
	flag.Parse()
}

func main() {
	c, err := config.ParseFile(*configFile)
	if err != nil {
		log.Fatalf("Failed to read config %s: %v", *configFile, err)
	}
	var conf config.Conf
	if len(*section) > 0 {
		conf = c.GetSection(*section)
	} else {
		conf = c
	}
	var angle float64
	a, err := conf.GetArg("rotate")
	if err == nil {
		angle, err = strconv.ParseFloat(a, 64)
		if err != nil {
			angle = 0.0
		}
	}
	l, err := lcd.CreateLcdDecoder(conf)
	if err != nil {
		log.Fatalf("LCD config failed %v", err)
	}
	var in image.Image
	if len(*input) == 0 {
		client := http.Client{
			Timeout: time.Duration(10) * time.Second,
		}
		source, _ := conf.GetArg("source")
		res, err := client.Get(source)
		if err != nil {
			log.Fatalf("Failed to retrieve source image from %s: %v", source, err)
		}
		in, _, err = image.Decode(res.Body)
		res.Body.Close()
		if err != nil {
			log.Fatalf("Failed to decode image from %s: %v", source, err)
		}
	} else {
		inf, err := os.Open(*input)
		if err != nil {
			log.Fatalf("Failed to open %s: %v", *input, err)
		}
		defer inf.Close()
		in, _, err = image.Decode(inf)
		if err != nil {
			log.Fatalf("Failed to read %s: %v", *input, err)
		}
	}
	if angle != 0 {
		in = lcd.RotateImage(in, angle)
	}
	if *calibrate && *decode {
		l.Preset(in, *digits)
	}
	// Convert image to RGBA.
	b := in.Bounds()
	img := image.NewRGBA(b)
	for y := b.Min.Y; y < b.Max.Y; y++ {
		for x := b.Min.X; x < b.Max.X; x++ {
			img.Set(x, y, color.RGBAModel.Convert(in.At(x, y)))
		}
	}
	if *decode {
		res := l.Decode(img)
		var str strings.Builder
		for _, d := range res.Decodes {
			if d.Valid {
				str.WriteString(d.Str)
			} else {
				str.WriteRune('X')
			}
		}
		fmt.Printf("Segments = <%s>\n", str.String())
	}
	l.MarkSamples(img, *fill)
	err = lcd.SaveImage(*output, img)
	if err != nil {
		log.Fatalf("%s encode error: %v", *output, err)
	}
}
