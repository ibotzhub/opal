// opal: a scripting language runtime by kayos
// nothing to see here yet :^)
package main

import (
	"bufio"
	"bytes"
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/gookit/color"
	"github.com/l0nax/go-spew/spew"

	"git.tcp.direct/kayos/opal/pkg/eval"
	"git.tcp.direct/kayos/opal/pkg/lex"
	"git.tcp.direct/kayos/opal/pkg/parse"
)

const (
	prompt     = "opal> "
	contPrompt = "    > " // continuation prompt for multi-line input
)

var (
	errColor = color.New(color.FgRed, color.OpBold)
	dimColor = color.New(color.FgGray)
)

func main() {
	switch len(os.Args) {
	case 1:
		// no args — drop into REPL
		repl()
	case 2:
		// one arg — run as script file
		runFile(os.Args[1])
	default:
		fmt.Fprintf(os.Stderr, "usage: opal [script]\n")
		os.Exit(1)
	}
}

// runFile reads and executes a script file.
// Errors are fatal — scripts are not interactive.
func runFile(path string) {
	src, err := os.ReadFile(path)
	if err != nil {
		fatal("could not read %s: %v", path, err)
	}

	root, err := parseSource(src)
	if err != nil {
		fatal("parse error in %s: %v", path, err)
	}

	if _, err = eval.New().Run(root); err != nil {
		fatal("runtime error in %s: %v", path, err)
	}
}

// repl runs an interactive read-eval-print loop.
//
// Variables and functions persist across entries via a shared scope.
// Multi-line input is accumulated until the parser stops returning ErrUnexpectedEOF.
// Errors print with spew detail and keep going — we don't die on bad input.
func repl() {
	sc := bufio.NewScanner(os.Stdin)

	// ev persists for the lifetime of the REPL session.
	// Each entry runs in the same scope so vars and funcs accumulate.
	ev := eval.New()

	fmt.Printf("opal — tcp.direct/kayos\n")
	dimColor.Printf("type 'exit;' to quit\n\n")

	var acc strings.Builder // accumulates multi-line input

	fmt.Print(prompt)
	for sc.Scan() {
		line := sc.Text()
		acc.WriteString(line)
		acc.WriteByte('\n')

		src := acc.String()

		root, err := parseSource([]byte(src))
		switch {
		case err == nil:
			// complete — evaluate against the persistent scope
			acc.Reset()
			result, runErr := ev.Run(root)
			if runErr != nil {
				replError("runtime", runErr, nil)
			} else if result != nil && result != eval.Nil {
				dimColor.Printf("= %s\n", result.String())
			}
			fmt.Print(prompt)

		case errors.Is(err, parse.ErrUnexpectedEOF):
			// user isn't done typing yet
			fmt.Print(contPrompt)

		default:
			// genuine parse error — dump it and reset accumulator
			replError("parse", err, nil)
			acc.Reset()
			fmt.Print(prompt)
		}
	}

	if err := sc.Err(); err != nil {
		fatal("scanner: %v", err)
	}

	fmt.Println()
}

// parseSource lexes and parses a complete source buffer.
func parseSource(src []byte) (*parse.Node, error) {
	f := lex.NewFragger(&bytesScanner{bytes.NewReader(src)})
	defer func() { _ = f.Close() }()

	p, err := parse.NewParser(f)
	if err != nil {
		return nil, err
	}
	return p.Parse()
}

// replError prints a formatted error with optional spew node dump, Josh-style.
// Pass node=nil when you don't have an AST node to show.
func replError(stage string, err error, node interface{}) {
	errColor.Printf("[%s error] %v\n", stage, err)
	if node != nil {
		dimColor.Println("--- node dump ---")
		spew.Fdump(os.Stderr, node)
		dimColor.Println("---")
	}
}

// fatal prints to stderr and exits. non-interactive failures only.
func fatal(format string, args ...any) {
	errColor.Fprintf(os.Stderr, "opal: "+format+"\n", args...)
	os.Exit(1)
}

// bytesScanner wraps bytes.Reader to satisfy io.ByteScanner.
// bytes.Reader already implements ReadByte and UnreadByte, so this is just
// a named wrapper to make the type system happy. :^)
type bytesScanner struct {
	*bytes.Reader
}
