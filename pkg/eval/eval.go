package eval

import (
	"fmt"
	"io"
	"os"
	"os/exec"

	"git.tcp.direct/kayos/opal/pkg/parse"
)

// Eval walks an opal AST and executes it.
//
// Fields follow Josh's pattern: unexported, focused.
//
//	scope  – the current lexical scope
//	stdout – where exec output goes (defaults to os.Stdout)
//	stderr – where exec errors go   (defaults to os.Stderr)
//	stdin  – exec stdin              (defaults to os.Stdin)
type Eval struct {
	scope  *Scope
	stdout io.Writer
	stderr io.Writer
	stdin  io.Reader
}

// New creates an Eval with a fresh top-level scope and standard I/O.
func New() *Eval {
	return &Eval{
		scope:  NewScope(),
		stdout: os.Stdout,
		stderr: os.Stderr,
		stdin:  os.Stdin,
	}
}

// WithScope returns a shallow copy of e using the given scope.
// Used for nested evaluation (function calls, blocks) and by the REPL
// to preserve variable bindings across entries.
func (e *Eval) WithScope(s *Scope) *Eval {
	return &Eval{scope: s, stdout: e.stdout, stderr: e.stderr, stdin: e.stdin}
}

// WithIO sets custom I/O streams. Returns e for chaining.
func (e *Eval) WithIO(stdout, stderr io.Writer, stdin io.Reader) *Eval {
	e.stdout = stdout
	e.stderr = stderr
	e.stdin = stdin
	return e
}

// Run evaluates a KindProgram node and returns the last expression value.
// An exit statement causes os.Exit to be called.
// A top-level return is treated as the program's result value.
func (e *Eval) Run(root *parse.Node) (result *Value, err error) {
	if root == nil || !root.Valid() {
		return Nil, fmt.Errorf("%w: nil or invalid root", ErrBadNode)
	}

	defer func() {
		r := recover()
		if r == nil {
			return
		}
		switch sig := r.(type) {
		case returnSignal:
			result = sig.val
		case exitSignal:
			os.Exit(sig.code)
		default:
			err = fmt.Errorf("eval panic: %v", r)
		}
	}()

	result = e.evalBlock(root)
	return result, nil
}

// evalBlock evaluates all statements in a KindProgram/block node
// and returns the value of the last expression statement, or Nil.
func (e *Eval) evalBlock(block *parse.Node) *Value {
	last := Nil
	for _, child := range block.Children() {
		last = e.evalNode(child)
	}
	return last
}

// evalNode dispatches to the correct eval function based on node kind.
func (e *Eval) evalNode(n *parse.Node) *Value {
	switch n.Kind() {
	case parse.KindProgram:
		return e.evalBlock(n)
	case parse.KindVarDecl:
		return e.evalVarDecl(n)
	case parse.KindFuncDecl:
		return e.evalFuncDecl(n)
	case parse.KindIfStmt:
		return e.evalIfStmt(n)
	case parse.KindForStmt:
		return e.evalForStmt(n)
	case parse.KindWhileStmt:
		return e.evalWhileStmt(n)
	case parse.KindExecStmt:
		return e.evalExecStmt(n, nil, nil)
	case parse.KindBgStmt:
		return e.evalBgStmt(n)
	case parse.KindExitStmt:
		throwExit(0)
		return Nil // unreachable
	case parse.KindReturnStmt:
		return e.evalReturnStmt(n)
	case parse.KindExprStmt:
		return e.evalExprStmt(n)
	case parse.KindPipeline:
		return e.evalPipeline(n)
	case parse.KindBinaryExpr:
		return e.evalBinaryExpr(n)
	case parse.KindUnaryExpr:
		return e.evalUnaryExpr(n)
	case parse.KindPrintStmt:
		return e.evalPrintStmt(n)
	case parse.KindCallExpr:
		return e.evalCallExpr(n)
	case parse.KindIdent:
		return e.evalIdent(n)
	case parse.KindLit:
		return coerce(n.Frag())
	default:
		panic(fmt.Errorf("%w: unhandled kind %s... FUBAR", ErrBadNode, n.Kind()))
	}
}

// evalVarDecl: "var" name "=" expr ";"
// Children: [KindIdent(name), expr]
func (e *Eval) evalVarDecl(n *parse.Node) *Value {
	name := n.Children()[0].Frag()
	val := e.evalNode(n.Children()[1])
	e.scope.Declare(name, val)
	return val
}

// evalFuncDecl: "func" name "(" params* ")" body
// Children: [param0?, param1?, ..., body(KindProgram)]
// The body is always the last child.
func (e *Eval) evalFuncDecl(n *parse.Node) *Value {
	name := n.Frag()
	children := n.Children()

	params := make([]string, 0, len(children)-1)
	for i := 0; i < len(children)-1; i++ {
		params = append(params, children[i].Frag())
	}
	body := children[len(children)-1]

	// capture current scope as closure environment
	fv := funcValue(params, body, e.scope)
	e.scope.Declare(name, fv)
	return fv
}

// evalIfStmt: "if" cond "then" thenBlock ("else" elseBlock)?
// Children: [cond, thenBlock, elseBlock?]
func (e *Eval) evalIfStmt(n *parse.Node) *Value {
	cond := e.evalNode(n.Children()[0])
	if cond.Truthy() {
		child := e.WithScope(e.scope.child())
		return child.evalBlock(n.Children()[1])
	}
	if len(n.Children()) > 2 {
		child := e.WithScope(e.scope.child())
		return child.evalBlock(n.Children()[2])
	}
	return Nil
}

// evalForStmt: "for" ident "=" iterExpr body
// Children: [ident, iterExpr, body]
// iterExpr must evaluate to a str (split on whitespace) or int (range 0..n).
func (e *Eval) evalForStmt(n *parse.Node) *Value {
	varName := n.Children()[0].Frag()
	iterVal := e.evalNode(n.Children()[1])
	body := n.Children()[2]

	last := Nil
	switch iterVal.typ {
	case TypeInt:
		// for i = 5 { ... } → iterate 0..4
		// iter var is declared in current scope so body can read it;
		// body also runs in current scope so any var mutations persist.
		for i := 0; i < iterVal.iVal; i++ {
			e.scope.Declare(varName, IntVal(i))
			last = e.evalBlock(body)
		}
	case TypeStr:
		// for word = "a b c" { ... } → iterate over space-split tokens
		for _, word := range splitWords(iterVal.sVal) {
			e.scope.Declare(varName, StrVal(word))
			last = e.evalBlock(body)
		}
	default:
		panic(fmt.Errorf("%w: for loop over %s... what are you even doing", ErrTypeMismatch, iterVal.Type()))
	}
	return last
}

// evalWhileStmt: "while" cond body
// Children: [cond, body]
// Note: body runs in the same scope as the condition so mutations
// (e.g. var n = n - 1) are visible on the next condition check.
// Use func for true isolation.
func (e *Eval) evalWhileStmt(n *parse.Node) *Value {
	cond := n.Children()[0]
	body := n.Children()[1]
	last := Nil
	for {
		if !e.evalNode(cond).Truthy() {
			break
		}
		last = e.evalBlock(body) // same scope — mutations stick
	}
	return last
}

// evalExecStmt runs a single exec command.
// stdin/stdout pipes are passed in from evalPipeline when chaining;
// nil means use the Eval's default I/O.
// Children: [cmdExpr]
func (e *Eval) evalExecStmt(n *parse.Node, stdin io.Reader, stdout io.Writer) *Value {
	cmdVal := e.evalNode(n.Children()[0])

	// split the command string into argv
	argv := splitWords(cmdVal.String())
	if len(argv) < 1 {
		panic(fmt.Errorf("%w: empty command", ErrExecFailed))
	}

	cmd := exec.Command(argv[0], argv[1:]...) //nolint:gosec

	switch {
	case stdin != nil:
		cmd.Stdin = stdin
	default:
		cmd.Stdin = e.stdin
	}
	switch {
	case stdout != nil:
		cmd.Stdout = stdout
	default:
		cmd.Stdout = e.stdout
	}
	cmd.Stderr = e.stderr

	if err := cmd.Run(); err != nil {
		panic(fmt.Errorf("%w: %s: %v", ErrExecFailed, argv[0], err))
	}
	return Nil
}

// evalBgStmt runs an exec command in the background (non-blocking).
// Children: [cmdExpr]
func (e *Eval) evalBgStmt(n *parse.Node) *Value {
	cmdVal := e.evalNode(n.Children()[0])
	argv := splitWords(cmdVal.String())
	if len(argv) < 1 {
		panic(fmt.Errorf("%w: empty bg command", ErrExecFailed))
	}

	cmd := exec.Command(argv[0], argv[1:]...) //nolint:gosec
	cmd.Stdin = e.stdin
	cmd.Stdout = e.stdout
	cmd.Stderr = e.stderr

	if err := cmd.Start(); err != nil {
		panic(fmt.Errorf("%w: bg %s: %v", ErrExecFailed, argv[0], err))
	}
	// detach — caller does not wait; background means fire-and-forget
	go func() { _ = cmd.Wait() }()
	return Nil
}

// evalReturnStmt throws a returnSignal caught by the enclosing function call.
// Children: [expr]
func (e *Eval) evalReturnStmt(n *parse.Node) *Value {
	val := e.evalNode(n.Children()[0])
	throwReturn(val)
	return Nil // unreachable
}

// evalExprStmt unwraps an expression statement.
// Children: [expr]
func (e *Eval) evalExprStmt(n *parse.Node) *Value {
	return e.evalNode(n.Children()[0])
}

// evalPipeline wires exec stages together with io.Pipe.
// Children: [execStmt, execStmt, ...]
// Each stage's stdout is the next stage's stdin.
// The final stage writes to e.stdout.
func (e *Eval) evalPipeline(n *parse.Node) *Value {
	stages := n.Children()
	if len(stages) < 2 {
		panic(fmt.Errorf("%w: pipeline with <2 stages is just exec, genius", ErrBadNode))
	}

	// build pipe chain: each stage runs in its own goroutine
	// feeding into the next via io.Pipe.
	type stageResult struct{ err error }
	results := make(chan stageResult, len(stages))

	var prevReader io.Reader // stdin for the current stage

	for i, stage := range stages {
		var stageWriter io.Writer
		var nextReader io.Reader

		isLast := i == len(stages)-1

		if !isLast {
			pr, pw := io.Pipe()
			stageWriter = pw
			nextReader = pr
		} else {
			stageWriter = e.stdout
		}

		// capture loop vars for goroutine
		s := stage
		sin := prevReader
		sout := stageWriter
		pw, _ := stageWriter.(*io.PipeWriter)

		go func() {
			var err error
			func() {
				defer func() {
					if r := recover(); r != nil {
						err = fmt.Errorf("%v", r)
					}
				}()
				e.evalExecStmt(s, sin, sout)
			}()
			if pw != nil {
				if err != nil {
					_ = pw.CloseWithError(err)
				} else {
					_ = pw.Close()
				}
			}
			results <- stageResult{err: err}
		}()

		prevReader = nextReader
	}

	// collect results from all stages
	var firstErr error
	for range stages {
		if r := <-results; r.err != nil && firstErr == nil {
			firstErr = r.err
		}
	}
	if firstErr != nil {
		panic(fmt.Errorf("%w: %v", ErrExecFailed, firstErr))
	}
	return Nil
}

// evalBinaryExpr evaluates a binary expression.
// Children: [left, right]; Frag is the operator string.
func (e *Eval) evalBinaryExpr(n *parse.Node) *Value {
	left := e.evalNode(n.Children()[0])
	right := e.evalNode(n.Children()[1])

	var (
		result *Value
		err    error
	)
	switch n.Frag() {
	case "+":
		result, err = add(left, right)
	case "-":
		result, err = sub(left, right)
	case "*":
		result, err = mul(left, right)
	case "/":
		result, err = div(left, right)
	case "%":
		result, err = mod(left, right)
	case "==":
		result = BoolVal(left.String() == right.String())
	case "!=":
		result = BoolVal(left.String() != right.String())
	case "<":
		result, err = cmpOp(left, right, func(a, b int) bool { return a < b })
	case ">":
		result, err = cmpOp(left, right, func(a, b int) bool { return a > b })
	case "<=":
		result, err = cmpOp(left, right, func(a, b int) bool { return a <= b })
	case ">=":
		result, err = cmpOp(left, right, func(a, b int) bool { return a >= b })
	case "&&":
		result = BoolVal(left.Truthy() && right.Truthy())
	case "||":
		result = BoolVal(left.Truthy() || right.Truthy())
	default:
		panic(fmt.Errorf("%w: unknown operator %q, how did you even get here", ErrBadNode, n.Frag()))
	}
	if err != nil {
		panic(err)
	}
	return result
}

// evalCallExpr evaluates a function call.
// Frag is the function name; Children are the argument expressions.
func (e *Eval) evalCallExpr(n *parse.Node) *Value {
	fv, ok := e.scope.Get(n.Frag())
	if !ok || fv.typ != TypeFunc {
		panic(fmt.Errorf("%w: %q", ErrNotCallable, n.Frag()))
	}
	fn := fv.fVal

	// evaluate arguments in current scope
	args := make([]*Value, len(n.Children()))
	for i, argNode := range n.Children() {
		args[i] = e.evalNode(argNode)
	}

	if len(args) != len(fn.params) {
		panic(fmt.Errorf("%w: %q expects %d args, got %d",
			ErrArity, n.Frag(), len(fn.params), len(args)))
	}

	// build a child scope off the closure's definition scope
	callScope := fn.scope.child()
	for i, param := range fn.params {
		callScope.Declare(param, args[i])
	}

	// evaluate the body; catchReturn intercepts return signals
	var returnVal *Value
	func() {
		defer catchReturn(&returnVal)
		child := e.WithScope(callScope)
		returnVal = child.evalBlock(fn.body)
	}()

	if returnVal == nil {
		return Nil
	}
	return returnVal
}

// evalUnaryExpr handles unary "-" and "!" operators.
func (e *Eval) evalUnaryExpr(n *parse.Node) *Value {
	operand := e.evalNode(n.Children()[0])
	switch n.Frag() {
	case "-":
		i, ok := operand.Int()
		if !ok {
			panic(fmt.Errorf("%w: unary - requires int, got %s", ErrTypeMismatch, operand.Type()))
		}
		return IntVal(-i)
	case "!":
		return BoolVal(!operand.Truthy())
	default:
		panic(fmt.Errorf("%w: unknown unary operator %q, how did you even get here", ErrBadNode, n.Frag()))
	}
}

// evalPrintStmt writes the expression value to stdout followed by a newline.
// This is the only built-in I/O; exec echo is the alternative.
func (e *Eval) evalPrintStmt(n *parse.Node) *Value {
	v := e.evalNode(n.Children()[0])
	fmt.Fprintln(e.stdout, v.String())
	return v
}

// evalIdent resolves an identifier: first checks scope, then coerces as literal.
// Quoted string fragments ("hello world") are unwrapped and returned as TypeStr directly.
// Bare unquoted words are tried as int, bool, then str.
func (e *Eval) evalIdent(n *parse.Node) *Value {
	name := n.Frag()
	// quoted string literal — strip surrounding quotes and unescape
	if len(name) >= 2 && name[0] == '"' && name[len(name)-1] == '"' {
		return StrVal(unquote(name[1 : len(name)-1]))
	}
	if v, ok := e.scope.Get(name); ok {
		return v
	}
	// not in scope — treat as a bare literal (number, bool, string constant)
	return coerce(name)
}

// unquote processes escape sequences in a string literal body (after quotes stripped).
func unquote(s string) string {
	if len(s) == 0 {
		return s
	}
	runes := []rune(s)
	out := make([]rune, 0, len(runes))
	for i := 0; i < len(runes); i++ {
		if runes[i] == '\' && i+1 < len(runes) {
			switch runes[i+1] {
			case '"':
				out = append(out, '"')
			case '\':
				out = append(out, '\')
			case 'n':
				out = append(out, '\n')
			case 't':
				out = append(out, '\t')
			default:
				out = append(out, '\', runes[i+1])
			}
			i++
			continue
		}
		out = append(out, runes[i])
	}
	return string(out)
}

// splitWords splits a string on whitespace, returning non-empty tokens.
// Used for exec argv building and for-loop iteration over strings.
func splitWords(s string) []string {
	var words []string
	start := -1
	for i, r := range s {
		isSpace := r == ' ' || r == '\t' || r == '\n' || r == '\r'
		switch {
		case !isSpace && start < 0:
			start = i
		case isSpace && start >= 0:
			words = append(words, s[start:i])
			start = -1
		}
	}
	if start >= 0 {
		words = append(words, s[start:])
	}
	return words
}

// Scope returns the Eval's current top-level scope.
// Used by the REPL to hand the scope forward between entries.
func (e *Eval) Scope() *Scope {
	return e.scope
}
