package lex

import (
	"bytes"
	"io"
	"testing"
)

// bytesScanner wraps a bytes.Reader to satisfy io.ByteScanner.
// bytes.Reader already implements ReadByte/UnreadByte so this is just an alias.
type bytesScanner struct {
	*bytes.Reader
}

func newFragger(src string) *Fragger {
	return NewFragger(&bytesScanner{bytes.NewReader([]byte(src))})
}

func TestFraggerSimpleKeyword(t *testing.T) {
	f := newFragger("if")
	defer f.Close()

	frag := f.Next()
	if frag == nil {
		t.Fatal("expected fragment, got nil")
	}
	if frag.String() != "if" {
		t.Errorf("expected 'if', got %q", frag.String())
	}
	if f.Next() != nil {
		t.Errorf("expected nil after EOF")
	}
}

func TestFraggerWhitespaceSeparated(t *testing.T) {
	f := newFragger("var x = 1")
	defer f.Close()

	expected := []string{"var", "x", "=", "1"}
	for i, want := range expected {
		frag := f.Next()
		if frag == nil {
			t.Fatalf("fragment[%d]: got nil, want %q", i, want)
		}
		if frag.String() != want {
			t.Errorf("fragment[%d]: got %q, want %q", i, frag.String(), want)
		}
	}
	if f.Next() != nil {
		t.Errorf("expected nil after last fragment")
	}
}

func TestFraggerSingleRuneTokensInline(t *testing.T) {
	// no spaces; single-rune tokens act as delimiters
	f := newFragger("func(x,y){return}")
	defer f.Close()

	expected := []string{"func", "(", "x", ",", "y", ")", "{", "return", "}"}
	for i, want := range expected {
		frag := f.Next()
		if frag == nil {
			t.Fatalf("fragment[%d]: got nil, want %q", i, want)
		}
		if frag.String() != want {
			t.Errorf("fragment[%d]: got %q, want %q", i, frag.String(), want)
		}
	}
}

func TestFraggerMore(t *testing.T) {
	f := newFragger("x")
	defer f.Close()

	if !f.More() {
		t.Error("expected More() == true before reading")
	}
	f.Next()
	if f.More() {
		t.Error("expected More() == false after consuming all input")
	}
}

func TestFraggerEmpty(t *testing.T) {
	f := newFragger("")
	defer f.Close()

	if f.More() {
		t.Error("expected More() == false on empty input")
	}
	if f.Next() != nil {
		t.Error("expected nil from Next() on empty input")
	}
}

func TestFraggerWhitespaceOnly(t *testing.T) {
	f := newFragger("   \t\n  ")
	defer f.Close()

	if f.Next() != nil {
		t.Error("expected nil from Next() on whitespace-only input")
	}
}

func TestFraggerReadRuneAndUnread(t *testing.T) {
	f := newFragger("if")
	defer f.Close()

	r, sz, err := f.ReadRune()
	if err != nil {
		t.Fatalf("ReadRune error: %v", err)
	}
	if r != 'i' || sz != 1 {
		t.Errorf("expected 'i'/1, got %q/%d", r, sz)
	}
	if err := f.UnreadRune(); err != nil {
		t.Fatalf("UnreadRune error: %v", err)
	}
	r2, _, err := f.ReadRune()
	if err != nil {
		t.Fatalf("ReadRune after unread error: %v", err)
	}
	if r2 != 'i' {
		t.Errorf("expected 'i' after UnreadRune, got %q", r2)
	}
}

func TestFraggerUnreadAtStart(t *testing.T) {
	f := newFragger("x")
	defer f.Close()

	if err := f.UnreadRune(); err == nil {
		t.Error("expected error from UnreadRune at start, got nil")
	}
}

func TestFraggerWriteByte(t *testing.T) {
	f := newFragger("")
	defer f.Close()

	if err := f.WriteByte('x'); err != nil {
		t.Errorf("WriteByte error: %v", err)
	}
}

func TestFraggerFragmentIsToken(t *testing.T) {
	f := newFragger("while")
	defer f.Close()

	frag := f.Next()
	if frag == nil {
		t.Fatal("expected fragment, got nil")
	}
	if !frag.IsToken() {
		t.Errorf("expected 'while' to be a token")
	}
	if frag.Token() != TokenWHILE {
		t.Errorf("expected TokenWHILE, got %v", frag.Token())
	}
}

func TestFraggerFragmentNotToken(t *testing.T) {
	f := newFragger("myVar")
	defer f.Close()

	frag := f.Next()
	if frag == nil {
		t.Fatal("expected fragment, got nil")
	}
	if frag.IsToken() {
		t.Errorf("expected 'myVar' to not be a token")
	}
	if frag.Token() != TokenBAD {
		t.Errorf("expected TokenBAD for unknown identifier")
	}
}

func TestFraggerPipeline(t *testing.T) {
	// simulates: exec ls | exec grep foo
	f := newFragger("exec ls|exec grep foo")
	defer f.Close()

	expected := []string{"exec", "ls", "|", "exec", "grep", "foo"}
	for i, want := range expected {
		frag := f.Next()
		if frag == nil {
			t.Fatalf("fragment[%d]: got nil, want %q", i, want)
		}
		if frag.String() != want {
			t.Errorf("fragment[%d]: got %q, want %q", i, frag.String(), want)
		}
	}
}

func TestFraggerSatisfiesFragmenterInterface(t *testing.T) {
	f := newFragger("x")
	defer f.Close()

	var _ Fragmenter = f
	var _ io.Closer = f
}

func TestFraggerFragmentLen(t *testing.T) {
	f := newFragger("return")
	defer f.Close()

	frag := f.Next()
	if frag == nil {
		t.Fatal("expected fragment")
	}
	if frag.RuneLen() != 6 {
		t.Errorf("expected RuneLen 6, got %d", frag.RuneLen())
	}
	if frag.Len() != 6 {
		t.Errorf("expected byte Len 6, got %d", frag.Len())
	}
}

// ── operator disambiguation (readOp) ─────────────────────────────────────────

func TestFraggerEqVsEqEq(t *testing.T) {
	type tc struct {
		src  string
		want []string
	}
	cases := []tc{
		{"x = 1", []string{"x", "=", "1"}},
		{"x == 1", []string{"x", "==", "1"}},
		{"x != 1", []string{"x", "!=", "1"}},
		{"x <= 1", []string{"x", "<=", "1"}},
		{"x >= 1", []string{"x", ">=", "1"}},
		{"x < 1", []string{"x", "<", "1"}},
		{"x > 1", []string{"x", ">", "1"}},
		{"a && b", []string{"a", "&&", "b"}},
		{"a || b", []string{"a", "||", "b"}},
	}
	for _, c := range cases {
		f := newFragger(c.src)
		for i, want := range c.want {
			frag := f.Next()
			if frag == nil {
				t.Errorf("%q fragment[%d]: got nil, want %q", c.src, i, want)
				break
			}
			if frag.String() != want {
				t.Errorf("%q fragment[%d]: got %q, want %q", c.src, i, frag.String(), want)
			}
		}
		f.Close()
	}
}

func TestFraggerPipeVsPipeOr(t *testing.T) {
	// | is a single pipe (exec pipeline), || is logical or
	f := newFragger("a || b | c")
	defer f.Close()
	expected := []string{"a", "||", "b", "|", "c"}
	for i, want := range expected {
		frag := f.Next()
		if frag == nil {
			t.Fatalf("fragment[%d]: got nil, want %q", i, want)
		}
		if frag.String() != want {
			t.Errorf("fragment[%d]: got %q, want %q", i, frag.String(), want)
		}
	}
}

// ── quoted string literals ────────────────────────────────────────────────────

func TestFraggerQuotedString(t *testing.T) {
	f := newFragger(`"hello world"`)
	defer f.Close()

	frag := f.Next()
	if frag == nil {
		t.Fatal("expected fragment, got nil")
	}
	if frag.String() != `"hello world"` {
		t.Errorf("got %q, want %q", frag.String(), `"hello world"`)
	}
	if f.Next() != nil {
		t.Error("expected nil after quoted string")
	}
}

func TestFraggerQuotedStringInExpr(t *testing.T) {
	f := newFragger(`var x = "hello world";`)
	defer f.Close()

	expected := []string{"var", "x", "=", `"hello world"`, ";"}
	for i, want := range expected {
		frag := f.Next()
		if frag == nil {
			t.Fatalf("fragment[%d]: got nil, want %q", i, want)
		}
		if frag.String() != want {
			t.Errorf("fragment[%d]: got %q, want %q", i, frag.String(), want)
		}
	}
}

func TestFraggerQuotedStringEscape(t *testing.T) {
	f := newFragger(`"say \"hi\""`)
	defer f.Close()

	frag := f.Next()
	if frag == nil {
		t.Fatal("expected fragment")
	}
	// should include the escape sequences intact; eval handles unquoting
	if frag.String() != `"say \"hi\""` {
		t.Errorf("got %q", frag.String())
	}
}

func TestFraggerQuotedStringEmpty(t *testing.T) {
	f := newFragger(`""`)
	defer f.Close()

	frag := f.Next()
	if frag == nil {
		t.Fatal("expected fragment")
	}
	if frag.String() != `""` {
		t.Errorf("got %q, want %q", frag.String(), `""`)
	}
}

// ── arithmetic operators ──────────────────────────────────────────────────────

func TestFraggerMulDivMod(t *testing.T) {
	type tc struct {
		src  string
		want []string
	}
	cases := []tc{
		{"a * b", []string{"a", "*", "b"}},
		{"a / b", []string{"a", "/", "b"}},
		{"a % b", []string{"a", "%", "b"}},
		{"a*b", []string{"a", "*", "b"}},
		{"10/2", []string{"10", "/", "2"}},
	}
	for _, c := range cases {
		f := newFragger(c.src)
		for i, want := range c.want {
			frag := f.Next()
			if frag == nil {
				t.Fatalf("%q fragment[%d]: nil, want %q", c.src, i, want)
			}
			if frag.String() != want {
				t.Errorf("%q fragment[%d]: got %q, want %q", c.src, i, frag.String(), want)
			}
		}
		f.Close()
	}
}

// ── line tracking ─────────────────────────────────────────────────────────────

func TestFraggerLineTracking(t *testing.T) {
	src := "var x = 1;\nvar y = 2;\nvar z = 3;"
	f := newFragger(src)
	defer f.Close()

	if f.Line() != 1 {
		t.Errorf("initial Line() = %d, want 1", f.Line())
	}
	// consume first line
	for frag := f.Next(); frag != nil && frag.String() != ";"; frag = f.Next() {}
	// consume the newline — it happens inside Next(), so Line() should now be 2
	f.Next() // "var" on line 2
	if f.Line() != 2 {
		t.Errorf("after line 1, Line() = %d, want 2", f.Line())
	}
}

// ── print keyword ─────────────────────────────────────────────────────────────

func TestFraggerPrintKeyword(t *testing.T) {
	f := newFragger(`print "hello";`)
	defer f.Close()

	expected := []string{"print", `"hello"`, ";"}
	for i, want := range expected {
		frag := f.Next()
		if frag == nil {
			t.Fatalf("fragment[%d]: nil, want %q", i, want)
		}
		if frag.String() != want {
			t.Errorf("fragment[%d]: got %q, want %q", i, frag.String(), want)
		}
	}
}
