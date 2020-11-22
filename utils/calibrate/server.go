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
	"fmt"
	"image"
	"image/draw"
	"image/jpeg"
	"log"
	"net/http"
	"os"

	"github.com/aamcrae/MeterMan/lcd"
)

const (
	plain   = iota
	filled  = iota
	outline = iota
)

type server struct {
	port    int
	refresh int
	l       *lcd.LcdDecoder
	img     image.Image
	str     string
}

// Initialise a http server.
func serverInit(port, refresh int) (*server, error) {
	h, err := os.Hostname()
	if err != nil {
		return nil, fmt.Errorf("hostname error: %v", err)
	}
	mux := http.NewServeMux()
	s := &server{port: port, refresh: refresh}
	mux.HandleFunc("/plain.jpg", func(w http.ResponseWriter, req *http.Request) {
		s.sendImage(w, plain)
	})
	mux.HandleFunc("/filled.jpg", func(w http.ResponseWriter, req *http.Request) {
		s.sendImage(w, filled)
	})
	mux.HandleFunc("/outline.jpg", func(w http.ResponseWriter, req *http.Request) {
		s.sendImage(w, outline)
	})
	mux.HandleFunc("/plain.html", func(w http.ResponseWriter, req *http.Request) {
		s.page(w, plain)
	})
	mux.HandleFunc("/outline.html", func(w http.ResponseWriter, req *http.Request) {
		s.page(w, outline)
	})
	mux.HandleFunc("/filled.html", func(w http.ResponseWriter, req *http.Request) {
		s.page(w, filled)
	})
	mux.HandleFunc("/", func(w http.ResponseWriter, req *http.Request) {
		if req.URL.Path != "/" {
			http.NotFound(w, req)
			return
		}
		s.page(w, filled)
	})
	go func() {
		log.Fatal(http.ListenAndServe(fmt.Sprintf(":%d", s.port), mux))
	}()
	log.Printf("Server on http:%s:%d/outline.html\n", h, s.port)
	return s, nil
}

// Update image
func (s *server) updateImage(img image.Image, str string) {
	s.img = img
	s.str = str
}

// Update decoder
func (s *server) updateDecoder(l *lcd.LcdDecoder) {
	s.l = l
}

func (s *server) page(w http.ResponseWriter, req int) {
	w.Header().Set("Content-Type", "text/html")
	fmt.Fprintf(w, "<html><head>")
	if s.refresh > 0 {
		fmt.Fprintf(w, "<meta http-equiv=\"refresh\" content=\"%d\">", s.refresh)
	}
	fmt.Fprintf(w, "</head><body>")
	if len(s.str) != 0 {
		fmt.Fprintf(w, "Decoded segments = %s<br>", s.str)
	}
	fmt.Fprintf(w, "<a href=\"plain.html\">Untouched image</a><br>")
	fmt.Fprintf(w, "<a href=\"outline.html\">Outlined image</a><br>")
	fmt.Fprintf(w, "<a href=\"filled.html\">Filled image</a><p>")
	switch req {
	case plain:
		fmt.Fprintf(w, "<img src=\"plain.jpg\">")
	case filled:
		fmt.Fprintf(w, "<img src=\"filled.jpg\">")
	case outline:
		fmt.Fprintf(w, "<img src=\"outline.jpg\">")
	}
	fmt.Fprintf(w, "</body>")
}

func (s *server) sendImage(w http.ResponseWriter, req int) {
	if s.l == nil || s.img == nil {
		http.Error(w, "No image yet", http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "image/jpeg")
	var img image.Image
	if req == plain {
		img = s.img
	} else {
		// Copy the image first.
		b := s.img.Bounds()
		dst := image.NewRGBA(b)
		draw.Draw(dst, b, s.img, b.Min, draw.Src)
		s.l.MarkSamples(dst, req == filled)
		img = dst
	}
	err := jpeg.Encode(w, img, nil)
	if err != nil {
		http.Error(w, "JPEG encode error", http.StatusInternalServerError)
	}
}
