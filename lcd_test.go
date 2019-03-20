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

package lcd_test

import (
	"testing"

	"fmt"
	"image"
	"image/jpeg"
	"os"
	"path/filepath"
	"strings"

	"github.com/aamcrae/MeterMan/lcd"
	"github.com/aamcrae/config"
)

func TestImg1(t *testing.T) {
	runTest(t, "test1", "12345678.")
	runTest(t, "test2", "12345678.")
	runTest(t, "test3", "12345678.")
	runTest(t, "test4", "12345678.")
	runTest(t, "lcd6", "123.456")
	runTest(t, "meter", "tot008765.4")
}

func runTest(t *testing.T, name string, result string) {
	cname := name + ".config"
	imagename := name + ".jpg"
	conf, err := config.ParseFile(filepath.Join("testdata", cname))
	if err != nil {
		t.Fatalf("Can't read config %s: %v", cname, err)
	}
	lcd, err := lcd.CreateLcdDecoder(conf.GetSection(""))
	if err != nil {
		t.Fatalf("LCD config for %s failed %v", cname, err)
	}
	ifile, err := os.Open(filepath.Join("testdata", imagename))
	if err != nil {
		t.Fatalf("%s: %v", imagename, err)
	}
	img, err := jpeg.Decode(ifile)
	if err != nil {
		t.Fatalf("Can't decode %s: %v", imagename, err)
	}
	// Convert image to gray scale.
	gi := image.NewGray(img.Bounds())
	b := img.Bounds()
	for y := b.Min.Y; y < b.Max.Y; y++ {
		for x := b.Min.X; x < b.Max.X; x++ {
			gi.Set(x, y, img.At(x, y))
		}
	}
	str, found := lcd.Decode(gi)
	got := strings.Join(str, "")
	if got != result {
		for i, f := range found {
			if !f {
				fmt.Printf("Element %d not found\n", i)
			}
		}
		t.Fatalf("For test %s, expected %s, found %s", name, result, got)
	}
}
