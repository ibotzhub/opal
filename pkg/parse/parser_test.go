package parse

import (
	"bytes"
	"testing"

	"git.tcp.direct/kayos/opal/pkg/lex"
)

// bytesScanner wraps bytes.Reader to satisfy io.ByteScanner (same helper as fragment_test).
type bytesScanner struct {
	*bytes.Reader
}

func mustParser(t *testing.T, src string) *Parser {
	t.Helper()
	f := lex.NewFragger(&bytesScanner{bytes.NewReader([]byte(src))})
	p, err := NewParser(f)
	if err != nil {
		t.Fatalf("NewParser: %v", err)
	}
	t.Cleanup(func() { _ = f.Close() })
	return p
}

func mustParse(t *testing.T, src string) *Node {
	t.Helper()
	p := mustParser(t, src)
	root, err := p.Parse()
	if err != nil {
		t.Fatalf("Parse(%q): %v", src, err)
	}
	if root == nil {
		t.Fatalf("Parse(%q): got nil root", src)
	}
	return root
}

func assertKind(t *testing.T, n *Node, want Kind) {
	t.Helper()
	if n == nil {
		t.Fatalf("node is nil, want kind %s", want)
	}
	if n.Kind() != want {
		t.Errorf("kind: got %s, want %s", n.Kind(), want)
	}
}

func assertFrag(t *testing.T, n *Node, want string) {
	t.Helper()
	if n.Frag() != want {
		t.Errorf("frag: got %q, want %q", n.Frag(), want)
	}
}

func assertChildCount(t *testing.T, n *Node, want int) {
	t.Helper()
	if len(n.Children()) != want {
		t.Errorf("children: got %d, want %d (node kind=%s)", len(n.Children()), want, n.Kind())
	}
}

// ── program ──────────────────────────────────────────────────────────────────

func TestParseEmptyProgram(t *testing.T) {
	root := mustParse(t, "")
	assertKind(t, root, KindProgram)
	assertChildCount(t, root, 0)
}

func TestParseSemicolonOnly(t *testing.T) {
	root := mustParse(t, ";;;")
	assertKind(t, root, KindProgram)
	assertChildCount(t, root, 0)
}

// ── var decl ─────────────────────────────────────────────────────────────────

func TestParseVarDecl(t *testing.T) {
	root := mustParse(t, "var x = y;")
	assertChildCount(t, root, 1)

	v := root.Children()[0]
	assertKind(t, v, KindVarDecl)
	assertChildCount(t, v, 2)

	// child 0: identifier name
	assertKind(t, v.Children()[0], KindIdent)
	assertFrag(t, v.Children()[0], "x")

	// child 1: value expression
	assertKind(t, v.Children()[1], KindIdent)
	assertFrag(t, v.Children()[1], "y")
}

func TestParseVarDeclWithArith(t *testing.T) {
	root := mustParse(t, "var z = a + b;")
	v := root.Children()[0]
	assertKind(t, v, KindVarDecl)

	val := v.Children()[1]
	assertKind(t, val, KindBinaryExpr)
	assertFrag(t, val, "+")
	assertFrag(t, val.Children()[0], "a")
	assertFrag(t, val.Children()[1], "b")
}

func TestParseVarDeclMissingSemiError(t *testing.T) {
	p := mustParser(t, "var x = y")
	_, err := p.Parse()
	if err == nil {
		t.Error("expected error for missing semicolon, got nil")
	}
}

// ── func decl ────────────────────────────────────────────────────────────────

func TestParseFuncDeclNoParams(t *testing.T) {
	root := mustParse(t, "func greet() { return msg; }")
	assertChildCount(t, root, 1)

	fn := root.Children()[0]
	assertKind(t, fn, KindFuncDecl)
	assertFrag(t, fn, "greet")

	// only child is the body block (no params)
	assertChildCount(t, fn, 1)
	body := fn.Children()[0]
	assertKind(t, body, KindProgram) // blocks are KindProgram
	assertChildCount(t, body, 1)
}

func TestParseFuncDeclWithParams(t *testing.T) {
	root := mustParse(t, "func add(x, y) { return x + y; }")
	fn := root.Children()[0]
	assertKind(t, fn, KindFuncDecl)
	assertFrag(t, fn, "add")

	// params + body = 3 children
	assertChildCount(t, fn, 3)
	assertKind(t, fn.Children()[0], KindIdent)
	assertFrag(t, fn.Children()[0], "x")
	assertKind(t, fn.Children()[1], KindIdent)
	assertFrag(t, fn.Children()[1], "y")

	body := fn.Children()[2]
	assertKind(t, body, KindProgram)
}

// ── if / then / else ─────────────────────────────────────────────────────────

func TestParseIfThen(t *testing.T) {
	root := mustParse(t, "if cond then { exit; }")
	stmt := root.Children()[0]
	assertKind(t, stmt, KindIfStmt)

	// child 0: condition, child 1: then block
	assertChildCount(t, stmt, 2)
	assertKind(t, stmt.Children()[0], KindIdent)
	assertFrag(t, stmt.Children()[0], "cond")
	assertKind(t, stmt.Children()[1], KindProgram)
}

func TestParseIfThenElse(t *testing.T) {
	root := mustParse(t, "if x then { exit; } else { return y; }")
	stmt := root.Children()[0]
	assertKind(t, stmt, KindIfStmt)
	assertChildCount(t, stmt, 3) // cond, then, else

	assertKind(t, stmt.Children()[2], KindProgram)
}

func TestParseIfMissingThenError(t *testing.T) {
	p := mustParser(t, "if cond { exit; }")
	_, err := p.Parse()
	if err == nil {
		t.Error("expected error for missing 'then', got nil")
	}
}

// ── for ──────────────────────────────────────────────────────────────────────

func TestParseForStmt(t *testing.T) {
	root := mustParse(t, "for i = items { exec ls; }")
	stmt := root.Children()[0]
	assertKind(t, stmt, KindForStmt)

	// children: iter_var, iter_expr, body
	assertChildCount(t, stmt, 3)
	assertKind(t, stmt.Children()[0], KindIdent)
	assertFrag(t, stmt.Children()[0], "i")
	assertKind(t, stmt.Children()[2], KindProgram)
}

// ── while ─────────────────────────────────────────────────────────────────────

func TestParseWhileStmt(t *testing.T) {
	root := mustParse(t, "while running { exec ping; }")
	stmt := root.Children()[0]
	assertKind(t, stmt, KindWhileStmt)
	assertChildCount(t, stmt, 2) // cond, body
}

// ── exec / pipeline ──────────────────────────────────────────────────────────

func TestParseExecSimple(t *testing.T) {
	root := mustParse(t, "exec ls;")
	stmt := root.Children()[0]
	assertKind(t, stmt, KindExecStmt)
	assertChildCount(t, stmt, 1)
}

func TestParseExecPipeline(t *testing.T) {
	root := mustParse(t, "exec ls | exec grep foo")
	stmt := root.Children()[0]
	assertKind(t, stmt, KindPipeline)
	assertChildCount(t, stmt, 2)
	assertKind(t, stmt.Children()[0], KindExecStmt)
	assertKind(t, stmt.Children()[1], KindExecStmt)
}

func TestParseExecLongPipeline(t *testing.T) {
	root := mustParse(t, "exec ls | exec grep foo | exec wc")
	stmt := root.Children()[0]
	assertKind(t, stmt, KindPipeline)
	assertChildCount(t, stmt, 3)
}

// ── bg ───────────────────────────────────────────────────────────────────────

func TestParseBgStmt(t *testing.T) {
	root := mustParse(t, "bg serve;")
	stmt := root.Children()[0]
	assertKind(t, stmt, KindBgStmt)
	assertChildCount(t, stmt, 1)
}

// ── exit ─────────────────────────────────────────────────────────────────────

func TestParseExitStmt(t *testing.T) {
	root := mustParse(t, "exit;")
	stmt := root.Children()[0]
	assertKind(t, stmt, KindExitStmt)
	assertChildCount(t, stmt, 0)
}

// ── return ───────────────────────────────────────────────────────────────────

func TestParseReturnStmt(t *testing.T) {
	root := mustParse(t, "return result;")
	stmt := root.Children()[0]
	assertKind(t, stmt, KindReturnStmt)
	assertChildCount(t, stmt, 1)
	assertFrag(t, stmt.Children()[0], "result")
}

// ── expressions ──────────────────────────────────────────────────────────────

func TestParseBinaryExprChain(t *testing.T) {
	root := mustParse(t, "var v = a + b + c;")
	v := root.Children()[0]
	// a + b + c is left-associative: ((a + b) + c)
	top := v.Children()[1]
	assertKind(t, top, KindBinaryExpr)
	assertFrag(t, top, "+")

	left := top.Children()[0]
	assertKind(t, left, KindBinaryExpr)
	assertFrag(t, left.Children()[0], "a")
	assertFrag(t, left.Children()[1], "b")

	assertFrag(t, top.Children()[1], "c")
}

func TestParseGroupedExpr(t *testing.T) {
	root := mustParse(t, "var v = (a + b);")
	v := root.Children()[0]
	inner := v.Children()[1]
	assertKind(t, inner, KindBinaryExpr)
}

func TestParseCallExpr(t *testing.T) {
	root := mustParse(t, "var v = add(x, y);")
	v := root.Children()[0]
	call := v.Children()[1]
	assertKind(t, call, KindCallExpr)
	assertFrag(t, call, "add")
	assertChildCount(t, call, 2)
}

func TestParseCallExprNoArgs(t *testing.T) {
	root := mustParse(t, "var v = now();")
	v := root.Children()[0]
	call := v.Children()[1]
	assertKind(t, call, KindCallExpr)
	assertFrag(t, call, "now")
	assertChildCount(t, call, 0)
}

// ── multi-statement program ───────────────────────────────────────────────────

func TestParseMultiStatement(t *testing.T) {
	src := `
		var x = 1;
		var y = 2;
		func add(a, b) { return a + b; }
		if x then { exit; } else { return y; }
	`
	root := mustParse(t, src)
	assertChildCount(t, root, 4)
	assertKind(t, root.Children()[0], KindVarDecl)
	assertKind(t, root.Children()[1], KindVarDecl)
	assertKind(t, root.Children()[2], KindFuncDecl)
	assertKind(t, root.Children()[3], KindIfStmt)
}

// ── node API ──────────────────────────────────────────────────────────────────

func TestNodeValid(t *testing.T) {
	n := newNode(KindIdent).withFrag("x")
	if !n.Valid() {
		t.Error("expected valid node")
	}
}

func TestNodeBadInvalid(t *testing.T) {
	n := newNode(kindBad)
	if n.Valid() {
		t.Error("expected invalid node for kindBad")
	}
}

func TestNodeIsLeaf(t *testing.T) {
	n := newNode(KindIdent).withFrag("x")
	if !n.IsLeaf() {
		t.Error("expected leaf")
	}
	parent := newNode(KindExprStmt)
	parent.appendChild(n)
	if parent.IsLeaf() {
		t.Error("expected non-leaf after appendChild")
	}
}

func TestNodeParentTracking(t *testing.T) {
	parent := newNode(KindVarDecl)
	child := newNode(KindIdent).withFrag("x")
	parent.appendChild(child)
	if child.Parent() != parent {
		t.Error("expected child.Parent() == parent")
	}
}

func TestKindString(t *testing.T) {
	if KindVarDecl.String() != "var_decl" {
		t.Errorf("KindVarDecl.String() = %q, want %q", KindVarDecl.String(), "var_decl")
	}
	if kindBad.String() != "bad" {
		t.Errorf("kindBad.String() = %q, want %q", kindBad.String(), "bad")
	}
}

// ── comparison expressions ────────────────────────────────────────────────────

func TestParseEqEq(t *testing.T) {
	root := mustParse(t, "var v = a == b;")
	v := root.Children()[0]
	cmp := v.Children()[1]
	assertKind(t, cmp, KindBinaryExpr)
	assertFrag(t, cmp, "==")
	assertFrag(t, cmp.Children()[0], "a")
	assertFrag(t, cmp.Children()[1], "b")
}

func TestParseNeq(t *testing.T) {
	root := mustParse(t, "var v = a != b;")
	cmp := root.Children()[0].Children()[1]
	assertKind(t, cmp, KindBinaryExpr)
	assertFrag(t, cmp, "!=")
}

func TestParseLtGt(t *testing.T) {
	for _, op := range []string{"<", ">", "<=", ">="} {
		src := "var v = a " + op + " b;"
		root := mustParse(t, src)
		cmp := root.Children()[0].Children()[1]
		assertKind(t, cmp, KindBinaryExpr)
		assertFrag(t, cmp, op)
	}
}

func TestParseLogicalAnd(t *testing.T) {
	root := mustParse(t, "var v = a && b;")
	expr := root.Children()[0].Children()[1]
	assertKind(t, expr, KindBinaryExpr)
	assertFrag(t, expr, "&&")
}

func TestParseLogicalOr(t *testing.T) {
	root := mustParse(t, "var v = a || b;")
	expr := root.Children()[0].Children()[1]
	assertKind(t, expr, KindBinaryExpr)
	assertFrag(t, expr, "||")
}

func TestParseComparisonPrecedence(t *testing.T) {
	// "a + b == c + d" should parse as "(a+b) == (c+d)", not "a + (b==c) + d"
	root := mustParse(t, "var v = a + b == c + d;")
	expr := root.Children()[0].Children()[1]
	assertKind(t, expr, KindBinaryExpr)
	assertFrag(t, expr, "==")
	assertKind(t, expr.Children()[0], KindBinaryExpr) // a+b
	assertFrag(t, expr.Children()[0], "+")
	assertKind(t, expr.Children()[1], KindBinaryExpr) // c+d
	assertFrag(t, expr.Children()[1], "+")
}

func TestParseAndOrPrecedence(t *testing.T) {
	// "a || b && c" should parse as "a || (b && c)" — && binds tighter than ||
	root := mustParse(t, "var v = a || b && c;")
	expr := root.Children()[0].Children()[1]
	assertKind(t, expr, KindBinaryExpr)
	assertFrag(t, expr, "||")
	assertFrag(t, expr.Children()[0], "a")
	// right side should be b && c
	assertKind(t, expr.Children()[1], KindBinaryExpr)
	assertFrag(t, expr.Children()[1], "&&")
}

// ── quoted string literals ────────────────────────────────────────────────────

func TestParseQuotedString(t *testing.T) {
	root := mustParse(t, `var s = "hello world";`)
	v := root.Children()[0]
	assertKind(t, v, KindVarDecl)
	val := v.Children()[1]
	assertKind(t, val, KindIdent)
	assertFrag(t, val, `"hello world"`)
}

func TestParseIfWithComparison(t *testing.T) {
	root := mustParse(t, "if x == 1 then { exit; }")
	stmt := root.Children()[0]
	assertKind(t, stmt, KindIfStmt)
	cond := stmt.Children()[0]
	assertKind(t, cond, KindBinaryExpr)
	assertFrag(t, cond, "==")
}

func TestParseWhileWithComparison(t *testing.T) {
	root := mustParse(t, "while n > 0 { var n = n - 1; }")
	stmt := root.Children()[0]
	assertKind(t, stmt, KindWhileStmt)
	cond := stmt.Children()[0]
	assertKind(t, cond, KindBinaryExpr)
	assertFrag(t, cond, ">")
}

// ── mul / div / mod ───────────────────────────────────────────────────────────

func TestParseMul(t *testing.T) {
	root := mustParse(t, "var v = a * b;")
	expr := root.Children()[0].Children()[1]
	assertKind(t, expr, KindBinaryExpr)
	assertFrag(t, expr, "*")
}

func TestParseDiv(t *testing.T) {
	root := mustParse(t, "var v = a / b;")
	expr := root.Children()[0].Children()[1]
	assertKind(t, expr, KindBinaryExpr)
	assertFrag(t, expr, "/")
}

func TestParseMod(t *testing.T) {
	root := mustParse(t, "var v = a % b;")
	expr := root.Children()[0].Children()[1]
	assertKind(t, expr, KindBinaryExpr)
	assertFrag(t, expr, "%")
}

func TestParseMulPrecedence(t *testing.T) {
	// "a + b * c" should parse as "a + (b * c)" — * binds tighter than +
	root := mustParse(t, "var v = a + b * c;")
	expr := root.Children()[0].Children()[1]
	assertKind(t, expr, KindBinaryExpr)
	assertFrag(t, expr, "+")
	// right child should be b*c
	assertKind(t, expr.Children()[1], KindBinaryExpr)
	assertFrag(t, expr.Children()[1], "*")
}

func TestParseDivPrecedence(t *testing.T) {
	// "a * b + c * d" → "(a*b) + (c*d)"
	root := mustParse(t, "var v = a * b + c * d;")
	expr := root.Children()[0].Children()[1]
	assertKind(t, expr, KindBinaryExpr)
	assertFrag(t, expr, "+")
	assertKind(t, expr.Children()[0], KindBinaryExpr)
	assertFrag(t, expr.Children()[0], "*")
	assertKind(t, expr.Children()[1], KindBinaryExpr)
	assertFrag(t, expr.Children()[1], "*")
}

// ── unary negation / not ──────────────────────────────────────────────────────

func TestParseUnaryNeg(t *testing.T) {
	root := mustParse(t, "var v = -x;")
	expr := root.Children()[0].Children()[1]
	assertKind(t, expr, KindUnaryExpr)
	assertFrag(t, expr, "-")
	assertChildCount(t, expr, 1)
	assertFrag(t, expr.Children()[0], "x")
}

func TestParseUnaryNot(t *testing.T) {
	root := mustParse(t, "var v = !ok;")
	expr := root.Children()[0].Children()[1]
	assertKind(t, expr, KindUnaryExpr)
	assertFrag(t, expr, "!")
}

func TestParseUnaryNegLiteral(t *testing.T) {
	root := mustParse(t, "var v = -5;")
	expr := root.Children()[0].Children()[1]
	assertKind(t, expr, KindUnaryExpr)
	assertFrag(t, expr, "-")
	assertFrag(t, expr.Children()[0], "5")
}

func TestParseDoubleUnary(t *testing.T) {
	// --x is legal: unary(unary(x))
	root := mustParse(t, "var v = --x;")
	outer := root.Children()[0].Children()[1]
	assertKind(t, outer, KindUnaryExpr)
	assertKind(t, outer.Children()[0], KindUnaryExpr)
}

// ── print statement ───────────────────────────────────────────────────────────

func TestParsePrintStmt(t *testing.T) {
	root := mustParse(t, `print "hello";`)
	stmt := root.Children()[0]
	assertKind(t, stmt, KindPrintStmt)
	assertChildCount(t, stmt, 1)
}

func TestParsePrintExpr(t *testing.T) {
	root := mustParse(t, "print x + 1;")
	stmt := root.Children()[0]
	assertKind(t, stmt, KindPrintStmt)
	expr := stmt.Children()[0]
	assertKind(t, expr, KindBinaryExpr)
	assertFrag(t, expr, "+")
}

// ── line numbers ──────────────────────────────────────────────────────────────

func TestParseLineNumbers(t *testing.T) {
	root := mustParse(t, "var x = 1;\nprint x;")
	if root.Children()[1].Line() < 1 {
		t.Errorf("expected Line() >= 1 on print stmt, got %d", root.Children()[1].Line())
	}
}
