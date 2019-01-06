package lcd

import (
    "flag"
    "log"
    "strconv"
    "time"

    "github.com/aamcrae/config"
    "github.com/aamcrae/MeterMan/core"
)


var saveBad = flag.Bool("savebad", false, "Save each bad image")
var badFile = flag.String("bad", "/tmp/bad.jpg", "Bad images")
var sampleTime = flag.Int("sample", 3, "Sample time (seconds)")

// Maps meter label to tag.
var tagMap map[string]string = map[string]string {
    "1NtL": core.A_OUT_TOTAL,
    "tP  ": core.G_TP,
    "EHtL": core.A_IN_TOTAL,
    "EHL1": core.A_IMPORT + "/0",
    "EHL2": core.A_IMPORT + "/1",
    "1NL1": core.A_EXPORT + "/0",
    "1NL2": core.A_EXPORT + "/1",
}

func init() {
    core.RegisterReader(lcdReader)
}

func lcdReader(conf *config.Config, wr chan<- core.Input) error {
    log.Printf("Registered LCD decoder as reader\n")
    var angle float64
    a, err := conf.GetArg("rotate")
    if err != nil {
        return err
    }
    angle, err = strconv.ParseFloat(a, 64)
    if err != nil {
        return err
    }
    source, err := conf.GetArg("source")
    if err != nil {
        return err
    }
    r, err := NewReader(conf, *core.Verbose)
    if  err != nil {
        return err
    }
    s := conf.Get("calibrate")
    if len(s) == 1 && len(s[0].Tokens) == 1 {
        img, err := ReadImage(s[0].Tokens[0])
        if  err != nil {
            return err
        }
        r.Calibrate(img)
    }
    core.AddGauge(core.G_TP)
    core.AddAccum(core.A_IN_TOTAL)
    core.AddAccum(core.A_OUT_TOTAL)
    core.AddSubAccum(core.A_IMPORT)
    core.AddSubAccum(core.A_IMPORT)
    core.AddSubAccum(core.A_EXPORT)
    core.AddSubAccum(core.A_EXPORT)
    go runReader(r, source, angle, wr)
    return nil
}

func runReader(r *Reader, source string, angle float64, wr chan<- core.Input) {
    delay := time.Duration(*sampleTime) * time.Second
    lastTime := time.Now()
    for {
        time.Sleep(delay - time.Now().Sub(lastTime))
        lastTime = time.Now()
        img, err := GetSource(source)
        if err != nil {
            log.Printf("Failed to retrieve source image from %s: %v", source, err)
            continue
        }
        img = RotateImage(img, angle)
        label, val, err := r.Read(img)
        if err != nil {
            log.Printf("Read error: %v", err)
            if *saveBad {
                SaveImage(*badFile, img)
            }
        } else {
            tag, ok := tagMap[label]
            if !ok {
                log.Printf("Unknown meter label: %s\n", label)
            } else {
                wr <- core.Input{tag, val}
            }
        }
    }
}
