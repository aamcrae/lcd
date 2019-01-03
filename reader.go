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
    "github.com/aamcrae/MeterMan/core"
)

var recalibrate = flag.Bool("recalibrate", false, "Recalibrate with new image")

const calibrateDelay = time.Minute * 10

type Reader struct {
    conf *config.Config
    decoder *LcdDecoder
    current image.Image
    key string
    value string
    m measure
    lastCalibration time.Time
}

type measure struct {
    handler func (*Reader) (string, float64, error)
    scale float64
    tag string
}

var measures map [string]measure = map[string]measure {
    "1nP1": measure{handlerNumber, 100.0, "IN-P1"},
    "1nP2": measure{handlerNumber, 100.0, "IN-P2"},
    "t1NE": measure{handlerIgnore, 1.0, "TIME"},
    "1NtL": measure{handlerNumber, 100.0, core.A_OUT_TOTAL},
    "tP  ": measure{handlerNumber, 10000.0, core.G_TP},
    "EHtL": measure{handlerNumber, 100.0, core.A_IN_TOTAL},
    "EHL1": measure{handlerNumber, 100.0, core.A_IMPORT + "/0"},
    "EHL2": measure{handlerNumber, 100.0, core.A_IMPORT + "/1"},
    "1NL1": measure{handlerNumber, 100.0, core.A_EXPORT + "/0"},
    "1NL2": measure{handlerNumber, 100.0, core.A_EXPORT + "/1"},
    "8888": measure{handlerCalibrate, 1.0, ""},
}

func NewReader(c *config.Config) (*Reader, error) {
    d, err := CreateLcdDecoder(c)
    if  err != nil {
        return nil, err
    }
    return &Reader{conf:c, decoder:d}, nil
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
    r.key = key
    r.value = value
    r.m = m
    return r.m.handler(r)
}

func handlerIgnore(r *Reader) (string, float64, error) {
    return "", 0.0, nil
}

func handlerNumber(r *Reader) (string, float64, error) {
    sv := r.value
    scale := r.m.scale
    if sv[0] == '-' {
        scale = -scale
        sv = sv[1:]
    }
    v, err := strconv.ParseFloat(strings.Trim(sv, " "), 64)
    if err != nil {
        return "", 0.0, fmt.Errorf("Bad number (%v): %s for %s\n", err, r.value, r.key)
    }
    return r.m.tag, v / scale, nil
}

func handlerCalibrate(r *Reader) (string, float64, error) {
    if r.value == "88888888" {
        SaveImage("/tmp/cal.jpg", r.current)
        if *recalibrate {
            r.Calibrate(r.current)
        }
    }
    return "", 0.0, nil
}
