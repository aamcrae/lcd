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
	"image"
	"log"
	"os"

	"github.com/aamcrae/MeterMan/lcd"
)

var angle = flag.Float64("angle", 215.5, "Rotation angle (degrees clockwise)")
var input = flag.String("input", "input.png", "Input image (png, jpg, gif")
var output = flag.String("output", "output.png", "Output image (png, jpg, gif")

func init() {
	flag.Parse()
}

func main() {
	ifile, err := os.Open(*input)
	if err != nil {
		log.Fatalf("%s: %v", *input, err)
	}
	defer ifile.Close()
	img, _, err := image.Decode(ifile)
	if err != nil {
		log.Fatalf("%s: %v", *input, err)
	}
	result := lcd.RotateImage(img, *angle)
	err = lcd.SaveImage(*output, result)
	if err != nil {
		log.Fatalf("%s: %v", *output, err)
	}
}
