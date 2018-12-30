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
var sampleTime = flag.Int("sample", 2, "Sample time (seconds)")


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
    r, err := NewReader(conf)
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
    go runReader(r, source, angle, wr)
    return nil
}

func runReader(r *Reader, source string, angle float64, wr chan<- core.Input) {
    delay := time.Duration(*sampleTime) * time.Second
    lastTime := time.Now()
    for {
        img, err := GetSource(source)
        if err != nil {
            log.Printf("Failed to retrieve source image from %s: %v", source, err)
            continue
        }
        img = RotateImage(img, angle)
        tag, val, err := r.Read(img)
        if err != nil {
            log.Printf("Read error: %v", err)
            if *saveBad {
                SaveImage(*badFile, img)
            }
        } else if len(tag) > 0 {
            wr <- core.Input{tag, val}
        }
        time.Sleep(delay - time.Now().Sub(lastTime))
        lastTime = time.Now()
    }
}
