package lcd

import (
    "github.com/aamcrae/config"
    "fmt"
    "strconv"
)

func CreateLcdDecoder(conf *config.Config) (* LcdDecoder, error) {
    l := NewLcdDecoder()
    for _, e := range conf.Get("lcd") {
        if len(e.Tokens) < 1 {
            return nil, fmt.Errorf("No config for LCD at line %d", e.Lineno)
        }
        name := e.Tokens[0]
        v := readInts(e.Tokens[1:])
        if len(v) != 7 {
            return nil, fmt.Errorf("Bad config for LCD at line %d", e.Lineno)
        }
        if err := l.AddLCD(name, v[:6], v[6]); err != nil {
            return nil, fmt.Errorf("Invalid config at line %d: %v", e.Lineno, err)
        }
    }
    for _, e := range conf.Get("digit") {
        if len(e.Tokens) != 3 && len(e.Tokens) != 5 {
            return nil, fmt.Errorf("Bad digit config line %d", e.Lineno)
        }
        v := readInts(e.Tokens[1:])
        min := 0
        max := 0x10000
        if (len(v) == 4) {
            min = v[2]
            max = v[3]
        }
        if len(v) != 2 && len(v) != 4 {
            return nil, fmt.Errorf("Bad config for digit at line %d", e.Lineno)
        }
        if _, err := l.AddDigit(e.Tokens[0], v[0], v[1], min, max); err != nil {
            return nil, fmt.Errorf("Invalid digit config at line %d: %v", e.Lineno, err)
        }
    }
    t := conf.Get("threshold")
    if len(t) > 0 {
        v := readInts(t[0].Tokens)
        if len(v) == 1 {
            l.SetThreshold(v[0])
        }
    }
    return l, nil
}

func readInts(strs []string) []int {
    vals := []int{}
    for _, s := range strs {
        if v, err := strconv.ParseInt(s, 10, 32); err != nil {
            break
        } else {
            vals = append(vals, int(v))
        }
    }
    return vals
}
