package main

import (
	"flag"
	"log"
	"os"
	"strings"
	"time"

	"github.com/benhoyt/goawk/interp"
	"github.com/benhoyt/goawk/parser"
)

func main() {
	src := flag.String("f", "", "program source to read from")
	in := flag.String("i", "", "use input file (instead on stdin)")
	flag.Parse()

	parserConfig := &parser.ParserConfig{
		Funcs: map[string]interface{}{
			"sum": func(args ...float64) float64 {
				sum := 0.0
				for _, a := range args {
					sum += a
				}
				return sum
			},
			"repeat":  strings.Repeat,
			"isodate": func(arg int64) string { return time.UnixMilli(arg).Format(time.RFC3339) },
		},
	}

	prg, err := os.ReadFile(*src)
	if err != nil {
		log.Fatalf("cannot read program %s: %s", *src, err)
	}

	prog, err := parser.ParseProgram(prg, parserConfig)
	if err != nil {
		log.Fatalf("cannot parse program: %s", err)
	}
	interpConfig := &interp.Config{Funcs: parserConfig.Funcs}

	if *in != "" {
		fh, err := os.Open(*in)
		if err != nil {
			log.Fatalf("cannot open %s: %s", *in, err)
		}
		interpConfig.Stdin = fh
	}

	_, err = interp.ExecProgram(prog, interpConfig)
	if err != nil {
		log.Fatalf("error executing program: %s", err)
	}
}
