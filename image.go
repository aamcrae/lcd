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
	"fmt"
	"image"
	"image/gif"
	"image/jpeg"
	"image/png"
	"math"
	"net/http"
	"os"
	"strings"

	"github.com/fogleman/gg"
)

// Save the image, using the suffix to select the type of image.
func SaveImage(name string, img image.Image) error {
	of, err := os.Create(name)
	if err != nil {
		return err
	}
	defer of.Close()
	if strings.HasSuffix(name, "png") {
		return png.Encode(of, img)
	} else if strings.HasSuffix(name, "jpg") {
		return jpeg.Encode(of, img, nil)
	} else if strings.HasSuffix(name, "gif") {
		return gif.Encode(of, img, nil)
	} else {
		return fmt.Errorf("%s: unknown image format", name)
	}
}

// Rotate the image, using a max sized canvas.
func RotateImage(image image.Image, angle float64) image.Image {
	if angle == 0 {
		return image
	}
	// Create a canvas of the maximum size required.
	dx := float64(image.Bounds().Dx())
	dy := float64(image.Bounds().Dy())
	size := int(math.Sqrt(dx*dx + dy*dy))
	c := gg.NewContext(size, size)
	width := image.Bounds().Dx()
	height := image.Bounds().Dy()
	startx := (size - width) / 2
	starty := (size - height) / 2
	c.RotateAbout(gg.Radians(angle), float64(size/2), float64(size/2))
	c.DrawImage(image, startx, starty)
	return c.Image()
}

// Get an image from the source URL.
func GetImage(src string) (image.Image, error) {
	res, err := http.Get(src)
	if err != nil {
		return nil, err
	}
	img, _, err := image.Decode(res.Body)
	res.Body.Close()
	return img, err
}

// Read an image from a file.
func ReadImage(name string) (image.Image, error) {
	inf, err := os.Open(name)
	if err != nil {
		return nil, err
	}
	defer inf.Close()
	in, _, err := image.Decode(inf)
	return in, err
}
