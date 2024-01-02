package main

import (
	"bytes"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"runtime/pprof"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/benhoyt/goawk/interp"
	"github.com/benhoyt/goawk/lexer"
	"github.com/benhoyt/goawk/parser"
)

const version = "1.0"

func main() {
	// Main AWK arguments
	var progFiles multiString
	flag.Var(&progFiles, "f", "load AWK source from `progfile` (multiple allowed)")
	fieldSep := flag.String("F", " ", "field `separator`")
	var vars multiString
	flag.Var(&vars, "v", "name=value variable `assignment` (multiple allowed)")
	showVersion := flag.Bool("version", false, "show GoAWK version and exit")

	// Debugging and profiling arguments
	debug := flag.Bool("d", false, "debug mode (print parsed AST to stderr)")
	debugTypes := flag.Bool("dt", false, "show variable types debug info")
	cpuprofile := flag.String("cpuprofile", "", "write CPU profile to `file`")
	memprofile := flag.String("memprofile", "", "write memory profile to `file`")

	flag.Parse()
	args := flag.Args()

	if *showVersion {
		fmt.Printf("TroutAWK %s\n", version)
		return
	}

	var src []byte
	if len(progFiles) > 0 {
		// Read source: the concatenation of all source files specified
		buf := &bytes.Buffer{}
		for _, progFile := range progFiles {
			if progFile == "-" {
				_, err := buf.ReadFrom(os.Stdin)
				if err != nil {
					errorExit("%s", err)
				}
			} else {
				f, err := os.Open(progFile)
				if err != nil {
					errorExit("%s", err)
				}
				_, err = buf.ReadFrom(f)
				if err != nil {
					f.Close()
					errorExit("%s", err)
				}
				f.Close()
			}
			// Append newline to file in case it doesn't end with one
			buf.WriteByte('\n')
		}
		src = buf.Bytes()
	} else {
		if len(args) < 1 {
			errorExit("usage: tawk [-F fs] [-v var=value] [-f progfile | 'prog'] [file ...]")
		}
		src = []byte(args[0])
		args = args[1:]
	}

	// Parse source code and setup interpreter
	parserConfig := &parser.ParserConfig{
		DebugTypes:  *debugTypes,
		DebugWriter: os.Stderr,
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
	prog, err := parser.ParseProgram(src, parserConfig)
	if err != nil {
		errMsg := fmt.Sprintf("%s", err)
		if err, ok := err.(*parser.ParseError); ok {
			showSourceLine(src, err.Position, len(errMsg))
		}
		errorExit("%s", errMsg)
	}
	if *debug {
		fmt.Fprintln(os.Stderr, prog)
	}
	config := &interp.Config{
		Argv0: filepath.Base(os.Args[0]),
		Args:  args,
		Vars:  []string{"FS", *fieldSep},
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
	for _, v := range vars {
		parts := strings.SplitN(v, "=", 2)
		if len(parts) != 2 {
			errorExit("-v flag must be in format name=value")
		}
		config.Vars = append(config.Vars, parts[0], parts[1])
	}

	if *cpuprofile != "" {
		f, err := os.Create(*cpuprofile)
		if err != nil {
			errorExit("could not create CPU profile: %v", err)
		}
		if err := pprof.StartCPUProfile(f); err != nil {
			errorExit("could not start CPU profile: %v", err)
		}
	}

	status, err := interp.ExecProgram(prog, config)
	if err != nil {
		errorExit("%s", err)
	}

	if *cpuprofile != "" {
		pprof.StopCPUProfile()
	}

	if *memprofile != "" {
		f, err := os.Create(*memprofile)
		if err != nil {
			errorExit("could not create memory profile: %v", err)
		}
		runtime.GC() // get up-to-date statistics
		if err := pprof.WriteHeapProfile(f); err != nil {
			errorExit("could not write memory profile: %v", err)
		}
		f.Close()
	}

	os.Exit(status)
}

// For parse errors, show source line and position of error, eg:
//
// -----------------------------------------------------
// BEGIN { x*; }
//
//	^
//
// -----------------------------------------------------
// parse error at 1:11: expected expression instead of ;
func showSourceLine(src []byte, pos lexer.Position, dividerLen int) {
	divider := strings.Repeat("-", dividerLen)
	if divider != "" {
		fmt.Fprintln(os.Stderr, divider)
	}
	lines := bytes.Split(src, []byte{'\n'})
	srcLine := string(lines[pos.Line-1])
	numTabs := strings.Count(srcLine[:pos.Column-1], "\t")
	runeColumn := utf8.RuneCountInString(srcLine[:pos.Column-1])
	fmt.Fprintln(os.Stderr, strings.Replace(srcLine, "\t", "    ", -1))
	fmt.Fprintln(os.Stderr, strings.Repeat(" ", runeColumn)+strings.Repeat("   ", numTabs)+"^")
	if divider != "" {
		fmt.Fprintln(os.Stderr, divider)
	}
}

func errorExit(format string, args ...interface{}) {
	fmt.Fprintf(os.Stderr, format+"\n", args...)
	os.Exit(1)
}

// Helper type for flag parsing to allow multiple -f and -v arguments
type multiString []string

func (m *multiString) String() string {
	return fmt.Sprintf("%v", []string(*m))
}

func (m *multiString) Set(value string) error {
	*m = append(*m, value)
	return nil
}
