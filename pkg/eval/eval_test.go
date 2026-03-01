package eval

import (
	"bytes"
	"strings"
	"testing"

	"git.tcp.direct/kayos/opal/pkg/lex"
	"git.tcp.direct/kayos/opal/pkg/parse"
)

// ── test helpers ─────────────────────────────────────────────────────────────

type bytesScanner struct{ *bytes.Reader }

func run(t *testing.T, src string) (*Value, string, string) {
	t.Helper()
	f := lex.NewFragger(&bytesScanner{bytes.NewReader([]byte(src))})
	t.Cleanup(func() { _ = f.Close() })

	p, err := parse.NewParser(f)
	if err != nil {
		t.Fatalf("NewParser: %v", err)
	}
	root, err := p.Parse()
	if err != nil {
		t.Fatalf("Parse(%q): %v", src, err)
	}

	var stdout, stderr bytes.Buffer
	ev := New().WithIO(&stdout, &stderr, strings.NewReader(""))
	result, err := ev.Run(root)
	if err != nil {
		t.Fatalf("Run(%q): %v", src, err)
	}
	return result, stdout.String(), stderr.String()
}

func runExpectErr(t *testing.T, src string) error {
	t.Helper()
	f := lex.NewFragger(&bytesScanner{bytes.NewReader([]byte(src))})
	t.Cleanup(func() { _ = f.Close() })

	p, err := parse.NewParser(f)
	if err != nil {
		return err
	}
	root, err := p.Parse()
	if err != nil {
		return err
	}
	var stdout, stderr bytes.Buffer
	ev := New().WithIO(&stdout, &stderr, strings.NewReader(""))
	_, err = ev.Run(root)
	return err
}

func assertInt(t *testing.T, v *Value, want int) {
	t.Helper()
	i, ok := v.Int()
	if !ok {
		t.Errorf("expected int value, got %s (%s)", v.String(), v.Type())
		return
	}
	if i != want {
		t.Errorf("int value: got %d, want %d", i, want)
	}
}

func assertStr(t *testing.T, v *Value, want string) {
	t.Helper()
	s, ok := v.Str()
	if !ok {
		t.Errorf("expected str value, got %s (%s)", v.String(), v.Type())
		return
	}
	if s != want {
		t.Errorf("str value: got %q, want %q", s, want)
	}
}

func assertBool(t *testing.T, v *Value, want bool) {
	t.Helper()
	b, ok := v.Bool()
	if !ok {
		t.Errorf("expected bool value, got %s (%s)", v.String(), v.Type())
		return
	}
	if b != want {
		t.Errorf("bool value: got %v, want %v", b, want)
	}
}

// ── value ─────────────────────────────────────────────────────────────────────

func TestIntVal(t *testing.T) {
	v := IntVal(42)
	assertInt(t, v, 42)
	if !v.Truthy() {
		t.Error("42 should be truthy")
	}
}

func TestIntZeroFalsy(t *testing.T) {
	v := IntVal(0)
	if v.Truthy() {
		t.Error("0 should be falsy")
	}
}

func TestStrVal(t *testing.T) {
	v := StrVal("hello")
	assertStr(t, v, "hello")
	if !v.Truthy() {
		t.Error("non-empty string should be truthy")
	}
}

func TestStrEmptyFalsy(t *testing.T) {
	v := StrVal("")
	if v.Truthy() {
		t.Error("empty string should be falsy")
	}
}

func TestBoolVal(t *testing.T) {
	assertBool(t, BoolVal(true), true)
	assertBool(t, BoolVal(false), false)
	if !BoolVal(true).Truthy() {
		t.Error("true should be truthy")
	}
	if BoolVal(false).Truthy() {
		t.Error("false should be falsy")
	}
}

func TestNilFalsy(t *testing.T) {
	if Nil.Truthy() {
		t.Error("nil should be falsy")
	}
}

func TestCoerce(t *testing.T) {
	type tc struct {
		in  string
		typ Type
	}
	cases := []tc{
		{"42", TypeInt},
		{"0", TypeInt},
		{"true", TypeBool},
		{"false", TypeBool},
		{"hello", TypeStr},
		{"3.14", TypeStr}, // floats stay as str for now
	}
	for _, c := range cases {
		v := coerce(c.in)
		if v.typ != c.typ {
			t.Errorf("coerce(%q): got %s, want %s", c.in, v.typ, c.typ)
		}
	}
}

func TestAddIntInt(t *testing.T) {
	v, err := add(IntVal(3), IntVal(4))
	if err != nil {
		t.Fatal(err)
	}
	assertInt(t, v, 7)
}

func TestAddStrStr(t *testing.T) {
	v, err := add(StrVal("hello"), StrVal(" world"))
	if err != nil {
		t.Fatal(err)
	}
	assertStr(t, v, "hello world")
}

func TestSubIntInt(t *testing.T) {
	v, err := sub(IntVal(10), IntVal(3))
	if err != nil {
		t.Fatal(err)
	}
	assertInt(t, v, 7)
}

func TestSubTypeMismatch(t *testing.T) {
	_, err := sub(StrVal("a"), IntVal(1))
	if err == nil {
		t.Error("expected type mismatch error")
	}
}

// ── scope ─────────────────────────────────────────────────────────────────────

func TestScopeSetGet(t *testing.T) {
	s := NewScope()
	s.Declare("x", IntVal(1))
	v, ok := s.Get("x")
	if !ok {
		t.Fatal("expected to find x")
	}
	assertInt(t, v, 1)
}

func TestScopeChain(t *testing.T) {
	parent := NewScope()
	parent.Declare("x", IntVal(1))
	child := parent.child()

	v, ok := child.Get("x")
	if !ok {
		t.Fatal("child should see parent x")
	}
	assertInt(t, v, 1)
}

func TestScopeShadow(t *testing.T) {
	parent := NewScope()
	parent.Declare("x", IntVal(1))
	child := parent.child()
	child.Declare("x", IntVal(99))

	v, _ := child.Get("x")
	assertInt(t, v, 99)

	// parent is unaffected
	pv, _ := parent.Get("x")
	assertInt(t, pv, 1)
}

func TestScopeMustGetMissing(t *testing.T) {
	s := NewScope()
	defer func() {
		if r := recover(); r == nil {
			t.Error("expected panic for missing var")
		}
	}()
	s.MustGet("missing")
}

// ── full program evaluation ────────────────────────────────────────────────────

func TestEvalVarDecl(t *testing.T) {
	v, _, _ := run(t, "var x = 42;")
	assertInt(t, v, 42)
}

func TestEvalVarDeclStr(t *testing.T) {
	v, _, _ := run(t, "var name = hello;")
	assertStr(t, v, "hello")
}

func TestEvalBinaryAdd(t *testing.T) {
	v, _, _ := run(t, "var z = 3 + 4;")
	assertInt(t, v, 7)
}

func TestEvalBinarySub(t *testing.T) {
	v, _, _ := run(t, "var z = 10 - 3;")
	assertInt(t, v, 7)
}

func TestEvalBinaryChain(t *testing.T) {
	v, _, _ := run(t, "var z = 1 + 2 + 3;")
	assertInt(t, v, 6)
}

func TestEvalStrConcat(t *testing.T) {
	v, _, _ := run(t, "var s = hello + world;")
	assertStr(t, v, "helloworld")
}

func TestEvalFuncDeclAndCall(t *testing.T) {
	src := `
		func add(a, b) { return a + b; }
		var result = add(3, 4);
	`
	v, _, _ := run(t, src)
	assertInt(t, v, 7)
}

func TestEvalFuncNoParams(t *testing.T) {
	src := `
		func answer() { return 42; }
		var x = answer();
	`
	v, _, _ := run(t, src)
	assertInt(t, v, 42)
}

func TestEvalFuncClosure(t *testing.T) {
	src := `
		var base = 10;
		func addBase(n) { return base + n; }
		var result = addBase(5);
	`
	v, _, _ := run(t, src)
	assertInt(t, v, 15)
}

func TestEvalIfTruthy(t *testing.T) {
	src := `
		var x = 1;
		var result = 0;
		if x then { var result = 99; }
		var result = 99;
	`
	// simplification: just check the if branch runs without error
	_, _, _ = run(t, src)
}

func TestEvalIfElse(t *testing.T) {
	src := `
		func pick(cond) {
			if cond then { return 1; } else { return 2; }
		}
		var a = pick(true);
		var b = pick(false);
	`
	// a=1, b=2 — check b is the last result
	v, _, _ := run(t, src)
	assertInt(t, v, 2)
}

func TestEvalWhile(t *testing.T) {
	// while loop mutates n in the same scope — scope fix means this actually works now
	src := `
		func countdown(n) {
			while n > 0 { var n = n - 1; }
			return n;
		}
		var result = countdown(3);
	`
	v, _, _ := run(t, src)
	assertInt(t, v, 0)
}

func TestEvalWhileMutationVisible(t *testing.T) {
	// direct test that while-body mutations are visible to the condition
	src := `
		var x = 5;
		while x > 0 { var x = x - 1; }
		var done = x;
	`
	v, _, _ := run(t, src)
	assertInt(t, v, 0)
}

func TestEvalForInt(t *testing.T) {
	src := `
		func sum(n) {
			var acc = 0;
			for i = n { var acc = acc + i; }
			return acc;
		}
		var result = sum(4);
	`
	// 0+1+2+3 = 6 — acc mutation visible across iterations after scope fix
	v, _, _ := run(t, src)
	assertInt(t, v, 6)
}

func TestEvalReturn(t *testing.T) {
	src := `
		func early() { return 7; return 999; }
		var x = early();
	`
	v, _, _ := run(t, src)
	assertInt(t, v, 7)
}

func TestEvalUndefinedVar(t *testing.T) {
	err := runExpectErr(t, "var x = missing_func(1);")
	// will panic with ErrNotCallable, caught by Run's recover
	if err == nil {
		t.Error("expected error for calling undefined function")
	}
}

func TestEvalArityMismatch(t *testing.T) {
	src := `
		func add(a, b) { return a + b; }
		var x = add(1);
	`
	err := runExpectErr(t, src)
	if err == nil {
		t.Error("expected arity error")
	}
}

// ── scope: var scoping in blocks ──────────────────────────────────────────────

func TestEvalBlockScope(t *testing.T) {
	src := `
		var x = 1;
		func inner() {
			var x = 99;
			return x;
		}
		var a = inner();
		var b = x;
	`
	// b should be 1 (outer x unaffected by inner's declaration)
	v, _, _ := run(t, src)
	assertInt(t, v, 1)
}

// ── splitWords ────────────────────────────────────────────────────────────────

func TestSplitWords(t *testing.T) {
	type tc struct {
		in   string
		want []string
	}
	cases := []tc{
		{"ls -la", []string{"ls", "-la"}},
		{"  echo  hello  ", []string{"echo", "hello"}},
		{"single", []string{"single"}},
		{"", nil},
		{"   ", nil},
	}
	for _, c := range cases {
		got := splitWords(c.in)
		if len(got) != len(c.want) {
			t.Errorf("splitWords(%q): got %v, want %v", c.in, got, c.want)
			continue
		}
		for i := range got {
			if got[i] != c.want[i] {
				t.Errorf("splitWords(%q)[%d]: got %q, want %q", c.in, i, got[i], c.want[i])
			}
		}
	}
}

// ── value string representation ──────────────────────────────────────────────

func TestValueString(t *testing.T) {
	type tc struct {
		v    *Value
		want string
	}
	cases := []tc{
		{Nil, "<nil>"},
		{IntVal(42), "42"},
		{StrVal("hi"), "hi"},
		{BoolVal(true), "true"},
		{BoolVal(false), "false"},
	}
	for _, c := range cases {
		if got := c.v.String(); got != c.want {
			t.Errorf("Value.String(): got %q, want %q", got, c.want)
		}
	}
}

// ── comparison operators ──────────────────────────────────────────────────────

func TestEvalEqEqTrue(t *testing.T) {
	v, _, _ := run(t, "var v = 1 == 1;")
	assertBool(t, v, true)
}

func TestEvalEqEqFalse(t *testing.T) {
	v, _, _ := run(t, "var v = 1 == 2;")
	assertBool(t, v, false)
}

func TestEvalNeq(t *testing.T) {
	v, _, _ := run(t, "var v = 1 != 2;")
	assertBool(t, v, true)
}

func TestEvalLt(t *testing.T) {
	v, _, _ := run(t, "var v = 3 < 5;")
	assertBool(t, v, true)
}

func TestEvalGt(t *testing.T) {
	v, _, _ := run(t, "var v = 5 > 3;")
	assertBool(t, v, true)
}

func TestEvalLte(t *testing.T) {
	v, _, _ := run(t, "var v = 3 <= 3;")
	assertBool(t, v, true)
}

func TestEvalGte(t *testing.T) {
	v, _, _ := run(t, "var v = 4 >= 5;")
	assertBool(t, v, false)
}

func TestEvalAnd(t *testing.T) {
	v, _, _ := run(t, "var v = true && false;")
	assertBool(t, v, false)
}

func TestEvalOr(t *testing.T) {
	v, _, _ := run(t, "var v = false || true;")
	assertBool(t, v, true)
}

func TestEvalCmpInIf(t *testing.T) {
	src := `
		func max(a, b) {
			if a > b then { return a; } else { return b; }
		}
		var result = max(3, 7);
	`
	v, _, _ := run(t, src)
	assertInt(t, v, 7)
}

func TestCmpOpIntInt(t *testing.T) {
	v, err := cmpOp(IntVal(3), IntVal(5), func(a, b int) bool { return a < b })
	if err != nil {
		t.Fatal(err)
	}
	assertBool(t, v, true)
}

func TestCmpOpTypeMismatch(t *testing.T) {
	_, err := cmpOp(StrVal("a"), IntVal(1), func(a, b int) bool { return a < b })
	if err == nil {
		t.Error("expected type mismatch error")
	}
}

// ── string literals ───────────────────────────────────────────────────────────

func TestEvalQuotedString(t *testing.T) {
	v, _, _ := run(t, `var s = "hello world";`)
	assertStr(t, v, "hello world")
}

func TestEvalQuotedStringEmpty(t *testing.T) {
	v, _, _ := run(t, `var s = "";`)
	assertStr(t, v, "")
}

func TestEvalQuotedStringConcat(t *testing.T) {
	v, _, _ := run(t, `var s = "hello" + " " + "world";`)
	assertStr(t, v, "hello world")
}

func TestEvalQuotedStringEqEq(t *testing.T) {
	v, _, _ := run(t, `var v = "hello" == "hello";`)
	assertBool(t, v, true)
}

func TestUnquote(t *testing.T) {
	type tc struct {
		in   string
		want string
	}
	cases := []tc{
		{`hello`, `hello`},
		{`say \"hi\"`, `say "hi"`},
		{`back\\slash`, `back\slash`},
		{``, ``},
	}
	for _, c := range cases {
		got := unquote(c.in)
		if got != c.want {
			t.Errorf("unquote(%q): got %q, want %q", c.in, got, c.want)
		}
	}
}

// ── persistent eval state (REPL simulation) ───────────────────────────────────

func TestEvalPersistentScope(t *testing.T) {
	// simulates two REPL entries sharing the same Eval
	ev := New()

	parse1 := func(src string) *parse.Node {
		f := lex.NewFragger(&bytesScanner{bytes.NewReader([]byte(src))})
		defer f.Close()
		p, _ := parse.NewParser(f)
		root, _ := p.Parse()
		return root
	}

	// first entry: declare x
	if _, err := ev.Run(parse1("var x = 42;")); err != nil {
		t.Fatalf("first Run: %v", err)
	}
	// second entry: read x — should still be 42
	v, err := ev.Run(parse1("var result = x;"))
	if err != nil {
		t.Fatalf("second Run: %v", err)
	}
	assertInt(t, v, 42)
}

// ── mul / div / mod ───────────────────────────────────────────────────────────

func TestMulIntInt(t *testing.T) {
	v, err := mul(IntVal(6), IntVal(7))
	if err != nil { t.Fatal(err) }
	assertInt(t, v, 42)
}

func TestDivIntInt(t *testing.T) {
	v, err := div(IntVal(10), IntVal(3))
	if err != nil { t.Fatal(err) }
	assertInt(t, v, 3) // integer division
}

func TestDivByZero(t *testing.T) {
	_, err := div(IntVal(5), IntVal(0))
	if err == nil {
		t.Error("expected div by zero error")
	}
}

func TestModIntInt(t *testing.T) {
	v, err := mod(IntVal(10), IntVal(3))
	if err != nil { t.Fatal(err) }
	assertInt(t, v, 1)
}

func TestModByZero(t *testing.T) {
	_, err := mod(IntVal(5), IntVal(0))
	if err == nil {
		t.Error("expected mod by zero error")
	}
}

func TestMulTypeMismatch(t *testing.T) {
	_, err := mul(StrVal("a"), IntVal(2))
	if err == nil {
		t.Error("expected type mismatch")
	}
}

func TestEvalMul(t *testing.T) {
	v, _, _ := run(t, "var v = 6 * 7;")
	assertInt(t, v, 42)
}

func TestEvalDiv(t *testing.T) {
	v, _, _ := run(t, "var v = 10 / 2;")
	assertInt(t, v, 5)
}

func TestEvalMod(t *testing.T) {
	v, _, _ := run(t, "var v = 10 % 3;")
	assertInt(t, v, 1)
}

func TestEvalMulPrecedence(t *testing.T) {
	v, _, _ := run(t, "var v = 2 + 3 * 4;")
	assertInt(t, v, 14) // 2 + (3*4) = 14, not (2+3)*4 = 20
}

// ── unary operators ───────────────────────────────────────────────────────────

func TestEvalUnaryNeg(t *testing.T) {
	v, _, _ := run(t, "var v = -5;")
	assertInt(t, v, -5)
}

func TestEvalUnaryNegVar(t *testing.T) {
	v, _, _ := run(t, "var x = 7; var v = -x;")
	assertInt(t, v, -7)
}

func TestEvalUnaryNot(t *testing.T) {
	v, _, _ := run(t, "var v = !true;")
	assertBool(t, v, false)
}

func TestEvalUnaryNotFalsy(t *testing.T) {
	v, _, _ := run(t, "var v = !false;")
	assertBool(t, v, true)
}

func TestEvalUnaryNegInExpr(t *testing.T) {
	v, _, _ := run(t, "var v = 10 + -3;")
	assertInt(t, v, 7)
}

func TestEvalUnaryNegTypeMismatch(t *testing.T) {
	err := runExpectErr(t, `var v = -"hello";`)
	if err == nil {
		t.Error("expected type mismatch for unary - on string")
	}
}

// ── print statement ───────────────────────────────────────────────────────────

func TestEvalPrint(t *testing.T) {
	_, stdout, _ := run(t, `print "hello";`)
	if stdout != "hello\n" {
		t.Errorf("stdout = %q, want %q", stdout, "hello\n")
	}
}

func TestEvalPrintExpr(t *testing.T) {
	_, stdout, _ := run(t, "print 6 * 7;")
	if stdout != "42\n" {
		t.Errorf("stdout = %q, want %q", stdout, "42\n")
	}
}

func TestEvalPrintReturnsValue(t *testing.T) {
	// print returns the value it printed
	v, _, _ := run(t, "print 99;")
	assertInt(t, v, 99)
}

func TestEvalPrintMultiple(t *testing.T) {
	_, stdout, _ := run(t, "print 1;\nprint 2;\nprint 3;")
	if stdout != "1\n2\n3\n" {
		t.Errorf("stdout = %q, want %q", stdout, "1\n2\n3\n")
	}
}
