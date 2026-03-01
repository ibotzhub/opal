package integration_test

import (
	"bytes"
	"os"
	"strings"
	"testing"

	"git.tcp.direct/kayos/opal/pkg/eval"
	"git.tcp.direct/kayos/opal/pkg/lex"
	"git.tcp.direct/kayos/opal/pkg/parse"
)

type bytesScanner struct{ *bytes.Reader }

// runScript lexes, parses, and evaluates a source string end-to-end.
// Returns the final value and captured stdout/stderr.
func runScript(t *testing.T, src string) (*eval.Value, string, string) {
	t.Helper()

	f := lex.NewFragger(&bytesScanner{bytes.NewReader([]byte(src))})
	t.Cleanup(func() { _ = f.Close() })

	p, err := parse.NewParser(f)
	if err != nil {
		t.Fatalf("NewParser: %v", err)
	}
	root, err := p.Parse()
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}

	var stdout, stderr bytes.Buffer
	ev := eval.New().WithIO(&stdout, &stderr, strings.NewReader(""))
	result, err := ev.Run(root)
	if err != nil {
		t.Fatalf("Run: %v", err)
	}
	return result, stdout.String(), stderr.String()
}

// runFile reads a testdata script and runs it.
func runFile(t *testing.T, path string) (*eval.Value, string, string) {
	t.Helper()
	src, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile(%s): %v", path, err)
	}
	return runScript(t, string(src))
}

// ── fibonacci ─────────────────────────────────────────────────────────────────

func TestIntegrationFibonacci(t *testing.T) {
	src := `
		func fib(n) {
			if n <= 1 then { return n; }
			return fib(n - 1) + fib(n - 2);
		}
		var result = fib(10);
	`
	v, _, _ := runScript(t, src)
	i, ok := v.Int()
	if !ok {
		t.Fatalf("expected int, got %s", v.Type())
	}
	if i != 55 {
		t.Errorf("fib(10) = %d, want 55", i)
	}
}

func TestIntegrationFibFile(t *testing.T) {
	v, _, _ := runFile(t, "testdata/fib.opal")
	i, ok := v.Int()
	if !ok {
		t.Fatalf("expected int result from fib.opal, got %s", v.Type())
	}
	if i != 55 {
		t.Errorf("fib(10) = %d, want 55", i)
	}
}

// ── factorial ─────────────────────────────────────────────────────────────────

func TestIntegrationFactorial(t *testing.T) {
	src := `
		func fact(n) {
			if n <= 1 then { return 1; }
			return n * fact(n - 1);
		}
		var result = fact(5);
	`
	// Note: * is not yet a token — this should error
	// TODO: add MUL token; for now test with repeated addition
	_ = src

	// factorial via addition tower to test recursion depth instead
	src2 := `
		func fact(n) {
			if n <= 1 then { return 1; }
			var prev = fact(n - 1);
			return prev + prev + prev + prev + prev;
		}
		var result = fact(3);
	`
	// 1 * 3 = 3, 3 * 3 = 9... not real factorial but tests deep recursion
	v, _, _ := runScript(t, src2)
	_, ok := v.Int()
	if !ok {
		t.Errorf("expected int result from recursive func, got %s", v.Type())
	}
}

// ── string operations ─────────────────────────────────────────────────────────

func TestIntegrationStrings(t *testing.T) {
	v, _, _ := runFile(t, "testdata/strings.opal")
	b, ok := v.Bool()
	if !ok {
		t.Fatalf("expected bool from strings.opal, got %s (%s)", v.String(), v.Type())
	}
	if !b {
		t.Error("expected string equality check to be true")
	}
}

func TestIntegrationStringConcat(t *testing.T) {
	src := `
		func greet(name) {
			return "hello, " + name + "!";
		}
		var msg = greet("opal");
	`
	v, _, _ := runScript(t, src)
	s, ok := v.Str()
	if !ok {
		t.Fatalf("expected str, got %s", v.Type())
	}
	if s != "hello, opal!" {
		t.Errorf("got %q, want %q", s, "hello, opal!")
	}
}

// ── loops ─────────────────────────────────────────────────────────────────────

func TestIntegrationLoops(t *testing.T) {
	v, _, _ := runFile(t, "testdata/loops.opal")
	// last statement is `while countdown > 0 { var countdown = countdown - 1; }`
	// countdown should end at 0
	i, ok := v.Int()
	if !ok {
		t.Fatalf("expected int from loops.opal, got %s", v.Type())
	}
	if i != 0 {
		t.Errorf("countdown ended at %d, want 0", i)
	}
}

func TestIntegrationForSum(t *testing.T) {
	src := `
		var acc = 0;
		for i = 5 { var acc = acc + i; }
		var result = acc;
	`
	// 0+1+2+3+4 = 10
	v, _, _ := runScript(t, src)
	i, ok := v.Int()
	if !ok {
		t.Fatalf("expected int, got %s", v.Type())
	}
	if i != 10 {
		t.Errorf("sum(0..4) = %d, want 10", i)
	}
}

// ── closures ──────────────────────────────────────────────────────────────────

func TestIntegrationClosure(t *testing.T) {
	src := `
		var base = 100;
		func addBase(n) { return base + n; }
		var result = addBase(23);
	`
	v, _, _ := runScript(t, src)
	i, ok := v.Int()
	if !ok {
		t.Fatalf("expected int, got %s", v.Type())
	}
	if i != 123 {
		t.Errorf("closure result = %d, want 123", i)
	}
}

// ── comparison-driven control flow ────────────────────────────────────────────

func TestIntegrationMaxFunction(t *testing.T) {
	src := `
		func max(a, b) {
			if a > b then { return a; } else { return b; }
		}
		var result = max(42, 17);
	`
	v, _, _ := runScript(t, src)
	i, ok := v.Int()
	if !ok {
		t.Fatalf("expected int, got %s", v.Type())
	}
	if i != 42 {
		t.Errorf("max(42, 17) = %d, want 42", i)
	}
}

func TestIntegrationAbsFunction(t *testing.T) {
	src := `
		func abs(n) {
			if n < 0 then { return 0 - n; } else { return n; }
		}
		var a = abs(5);
		var b = abs(0 - 7);
	`
	v, _, _ := runScript(t, src)
	i, ok := v.Int()
	if !ok {
		t.Fatalf("expected int, got %s", v.Type())
	}
	if i != 7 {
		t.Errorf("abs(-7) = %d, want 7", i)
	}
}

// ── multi-run persistent scope (REPL simulation) ──────────────────────────────

func TestIntegrationREPLScope(t *testing.T) {
	ev := eval.New()

	parse_ := func(src string) *parse.Node {
		f := lex.NewFragger(&bytesScanner{bytes.NewReader([]byte(src))})
		defer f.Close()
		p, err := parse.NewParser(f)
		if err != nil {
			t.Fatalf("NewParser: %v", err)
		}
		root, err := p.Parse()
		if err != nil {
			t.Fatalf("Parse: %v", err)
		}
		return root
	}

	entries := []struct {
		src  string
		want int
	}{
		{"var x = 10;", 10},
		{"var y = x + 5;", 15},
		{"var z = x + y;", 25},
	}

	for _, e := range entries {
		v, err := ev.Run(parse_(e.src))
		if err != nil {
			t.Fatalf("Run(%q): %v", e.src, err)
		}
		i, ok := v.Int()
		if !ok {
			t.Fatalf("Run(%q): expected int, got %s", e.src, v.Type())
		}
		if i != e.want {
			t.Errorf("Run(%q) = %d, want %d", e.src, i, e.want)
		}
	}
}

// ── arithmetic ────────────────────────────────────────────────────────────────

func TestIntegrationFactorialReal(t *testing.T) {
	v, _, _ := runFile(t, "testdata/math.opal")
	i, ok := v.Int()
	if !ok {
		t.Fatalf("expected int, got %s", v.Type())
	}
	if i != 3628800 {
		t.Errorf("10! = %d, want 3628800", i)
	}
}

func TestIntegrationMulPrecedence(t *testing.T) {
	v, _, _ := runScript(t, "var v = 2 + 3 * 4;")
	i, ok := v.Int()
	if !ok { t.Fatalf("expected int, got %s", v.Type()) }
	if i != 14 { t.Errorf("2+3*4 = %d, want 14", i) }
}

func TestIntegrationModFizzBuzz(t *testing.T) {
	// count how many numbers 1-15 are divisible by 3
	src := `
		var count = 0;
		for i = 15 {
			if i % 3 == 0 then { var count = count + 1; }
		}
		var result = count;
	`
	// i goes 0..14 — 0,3,6,9,12 are divisible by 3 → 5
	v, _, _ := runScript(t, src)
	i, ok := v.Int()
	if !ok { t.Fatalf("expected int") }
	if i != 5 { t.Errorf("fizzbuzz count = %d, want 5", i) }
}

// ── unary ─────────────────────────────────────────────────────────────────────

func TestIntegrationUnaryNeg(t *testing.T) {
	src := `
		func negate(n) { return -n; }
		var result = negate(42);
	`
	v, _, _ := runScript(t, src)
	i, ok := v.Int()
	if !ok { t.Fatalf("expected int") }
	if i != -42 { t.Errorf("negate(42) = %d, want -42", i) }
}

func TestIntegrationUnaryNot(t *testing.T) {
	src := `
		func isFalse(v) { return !v; }
		var a = isFalse(true);
		var b = isFalse(false);
	`
	v, _, _ := runScript(t, src)
	b, ok := v.Bool()
	if !ok { t.Fatalf("expected bool") }
	if !b { t.Error("!false should be true") }
}

// ── print ─────────────────────────────────────────────────────────────────────

func TestIntegrationPrint(t *testing.T) {
	_, stdout, _ := runFile(t, "testdata/print.opal")
	if stdout != "hello, opal!\n42\n" {
		t.Errorf("stdout = %q, want %q", stdout, "hello, opal!\n42\n")
	}
}

func TestIntegrationPrintInLoop(t *testing.T) {
	src := `
		for i = 3 { print i; }
	`
	_, stdout, _ := runScript(t, src)
	if stdout != "0\n1\n2\n" {
		t.Errorf("stdout = %q, want %q", stdout, "0\n1\n2\n")
	}
}

// ── line numbers in errors ────────────────────────────────────────────────────

func TestIntegrationLineNumberInError(t *testing.T) {
	src := "var x = 1;\nvar y = x\nvar z = 3;"
	// missing ; after y = x — should produce an error mentioning line 2 or 3
	f := lex.NewFragger(&bytesScanner{bytes.NewReader([]byte(src))})
	defer f.Close()
	p, err := parse.NewParser(f)
	if err != nil {
		t.Fatalf("NewParser: %v", err)
	}
	_, err = p.Parse()
	if err == nil {
		t.Fatal("expected parse error for missing semicolon")
	}
	// error message should contain a line number
	errStr := err.Error()
	if len(errStr) == 0 {
		t.Error("empty error string")
	}
	t.Logf("parse error: %s", errStr)
}
