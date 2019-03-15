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
	"flag"
	"fmt"
	"image"
	"log"
	"strconv"
	"strings"
	"time"

	"github.com/aamcrae/config"
)

var recalibrate = flag.Bool("recalibrate", false, "Recalibrate with new image")

const calibrateDelay = time.Minute * 10

type limit struct {
	last  time.Time
	value float64
}

type Reader struct {
	trace           bool
	decoder         *LcdDecoder
	current         image.Image
	lastCalibration time.Time
	limits          map[string]limit
}

type measure struct {
	handler func(*Reader, *measure, string, string) (string, float64, error)
	scale   float64
	min     float64 // Valid minimum
	max     float64 // Valid maximum (or hourly max increase)
}

var measures map[string]*measure = map[string]*measure{
	"1nP1": &measure{handlerIgnore, 1.0, 0, 0},
	"1nP2": &measure{handlerIgnore, 1.0, 0, 0},
	"t1NE": &measure{handlerIgnore, 1.0, 0, 0},
	"1NtL": &measure{handlerAccum, 100.0, 0, 0},     // KwH
	"tP  ": &measure{handlerNumber, 10000.0, 0, 0},  // Kw
	"EHtL": &measure{handlerAccum, 100.0, 0, 0},     // KwH
	"EHL1": &measure{handlerAccum, 100.0, 0, 0},     // KwH
	"EHL2": &measure{handlerAccum, 100.0, 0, 0},     // KwH
	"1NL1": &measure{handlerAccum, 100.0, 0, 0},     // KwH
	"1NL2": &measure{handlerAccum, 100.0, 0, 0},     // KwH
	"8888": &measure{handlerCalibrate, 1.0, 0, 0},
}

func NewReader(c *config.Section, trace bool) (*Reader, error) {
	d, err := CreateLcdDecoder(c)
	if err != nil {
		return nil, err
	}
	for _, e := range c.Get("range") {
		if len(e.Tokens) != 3 {
			return nil, fmt.Errorf("Bad 'range' parameters at %s:%d", e.Filename, e.Lineno)
		}
		m, ok := measures[e.Tokens[0]]
		if !ok {
			return nil, fmt.Errorf("Unknown measurement (%s) at %s:%d", e.Tokens[0], e.Filename, e.Lineno)
		}
		min, err := strconv.ParseFloat(e.Tokens[1], 64)
		if err != nil {
			return nil, fmt.Errorf("Ilegal min value at %s:%d", e.Tokens[0], e.Filename, e.Lineno)
		}
		max, err := strconv.ParseFloat(e.Tokens[2], 64)
		if err != nil {
			return nil, fmt.Errorf("Ilegal max value at %s:%d", e.Tokens[0], e.Filename, e.Lineno)
		}
		m.min = min
		m.max = max
		if trace {
			log.Printf("Setting range of '%s' to [%f, %f]\n", e.Tokens[0], min, max)
		}
	}
	return &Reader{trace: trace, decoder: d, limits: map[string]limit{}}, nil
}

func (r *Reader) Calibrate(img image.Image) {
	now := time.Now()
	if time.Now().Sub(r.lastCalibration) >= calibrateDelay {
		r.lastCalibration = now
		log.Printf("Recalibrating")
		r.decoder.Calibrate(img)
	}
}

func (r *Reader) Read(img image.Image) (string, float64, error) {
	r.current = img
	vals, vok := r.decoder.Decode(img)
	bad := false
	var seg int
	for s, okDigit := range vok {
		if !okDigit {
			bad = true
			seg = s
			break
		}
	}
	if bad {
		return "", 0.0, fmt.Errorf("Bad read on segment %d", seg)
	}
	key := strings.Join(vals[0:4], "")
	value := strings.Join(vals[4:], "")
	m, ok := measures[key]
	if !ok {
		return "", 0.0, fmt.Errorf("Unknown key (%s) value %s", key, value)
	}
	handler := m.handler
	return handler(r, m, key, value)
}

func handlerIgnore(r *Reader, m *measure, key, value string) (string, float64, error) {
	return "", 0.0, nil
}

func handlerNumber(r *Reader, m *measure, key, value string) (string, float64, error) {
	v, err := r.getNumber(m, value)
	if err != nil {
		return "", 0, fmt.Errorf("key %s: %v", key, err)
	}
	if v < m.min || v >= m.max {
		return "", 0, fmt.Errorf("%s Out of range (%f)", key, v)
	}
	if r.trace {
		log.Printf("Meter read: key %s value %f, min %f, max %f\n", key, v, m.min, m.max)
	}
	return key, v, nil
}

func handlerAccum(r *Reader, m *measure, key, value string) (string, float64, error) {
	v, err := r.getNumber(m, value)
	if err != nil {
		return "", 0, fmt.Errorf("key %s: %v", key, err)
	}
	lv, ok := r.limits[key]
	now := time.Now()
	if ok {
		diff := (v - lv.value) * 3600 / now.Sub(lv.last).Seconds()
		if diff < 0 {
			return "", 0.0, fmt.Errorf("%s Going backwards (old %f, new %f)", key, lv.value, v)
		}
		// Calculate and compare hourly change.
		if diff > m.max {
			return "", 0.0, fmt.Errorf("%s limit exceeded (old %f, change = %f, limit = %f)", key, lv.value, diff, m.max)
		}
		if r.trace {
			log.Printf("Meter read: key %s value %f, change %f, max %f\n", key, v, diff, m.max)
		}
	}
	r.limits[key] = limit{now, v}
	return key, v, nil
}

func handlerCalibrate(r *Reader, m *measure, key, value string) (string, float64, error) {
	if value == "88888888" {
		SaveImage("/tmp/cal.jpg", r.current)
		if *recalibrate {
			r.Calibrate(r.current)
		}
	}
	return "", 0.0, nil
}

func (*Reader) getNumber(m *measure, value string) (float64, error) {
	sv := value
	scale := m.scale
	if sv[0] == '-' {
		scale = -scale
		sv = sv[1:]
	}
	v, err := strconv.ParseFloat(strings.Trim(sv, " "), 64)
	if err != nil {
		return 0, fmt.Errorf("%s: bad number (%v)\n", value, err)
	}
	return v / scale, nil
}
