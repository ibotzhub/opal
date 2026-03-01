package parse

import (
	"fmt"
	"io"

	"github.com/l0nax/go-spew/spew"

	"git.tcp.direct/kayos/opal/pkg/lex"
)

// Parser performs recursive descent parsing over a stream of Fragments
// produced by a lex.Fragmenter. It builds an AST rooted at a KindProgram node.
//
// Fields follow Josh's pattern: unexported, tightly scoped.
//
//	frag    – the fragment source
//	cur     – current fragment (peek buffer)
//	curTok  – resolved token for cur, or TokenBAD for identifiers/literals
//	prev    – previous fragment (for requires-checking)
//	prevTok – resolved token for prev
//	done    – true once EOF or source exhausted
type Parser struct {
	frag    lex.Fragmenter
	cur     *lex.Fragment
	curTok  *lex.Token
	prev    *lex.Fragment
	prevTok *lex.Token
	done    bool
	line    int // current source line (from fragger if available)
}

// NewParser creates a Parser reading from the given Fragmenter.
// It primes the lookahead by reading the first fragment.
func NewParser(f lex.Fragmenter) (*Parser, error) {
	p := &Parser{frag: f}
	if err := p.advance(); err != nil && err != io.EOF {
		return nil, fmt.Errorf("parser init: %w", err)
	}
	return p, nil
}

// advance reads the next fragment into cur, resolving its token.
func (p *Parser) advance() error {
	p.prev = p.cur
	p.prevTok = p.curTok

	if !p.frag.More() {
		p.done = true
		p.cur = nil
		p.curTok = lex.TokenEOF
		return io.EOF
	}

	p.cur = p.frag.Next()
	if p.cur == nil {
		p.done = true
		p.curTok = lex.TokenEOF
		return io.EOF
	}

	p.curTok = p.cur.Token()
	return nil
}

// peek returns the current token without consuming it.
func (p *Parser) peek() *lex.Token {
	return p.curTok
}

// peekFrag returns the raw current fragment string.
func (p *Parser) peekFrag() string {
	if p.cur == nil {
		return ""
	}
	return p.cur.String()
}

// eat consumes the current fragment and returns it if it matches tok.
// Returns an error if the current token does not match.
func (p *Parser) eat(tok *lex.Token) (*lex.Fragment, error) {
	if p.curTok != tok {
		return nil, fmt.Errorf("%w: want %s, got %s (%s)",
			ErrUnexpectedToken, spew.Sdump(tok), spew.Sdump(p.curTok), p.peekFrag())
	}
	cur := p.cur
	if err := p.advance(); err != nil && err != io.EOF {
		return nil, err
	}
	return cur, nil
}

// eatIdent consumes the current fragment as an identifier (non-token frag).
// Returns an error if the current fragment is a known keyword token.
func (p *Parser) eatIdent() (*lex.Fragment, error) {
	if p.cur == nil {
		return nil, ErrUnexpectedEOF
	}
	if p.cur.IsToken() {
		return nil, fmt.Errorf("%w: expected identifier, got keyword %q",
			ErrUnexpectedToken, p.peekFrag())
	}
	cur := p.cur
	if err := p.advance(); err != nil && err != io.EOF {
		return nil, err
	}
	return cur, nil
}

// eatAny consumes the current fragment regardless of token type.
func (p *Parser) eatAny() (*lex.Fragment, error) {
	if p.cur == nil {
		return nil, ErrUnexpectedEOF
	}
	cur := p.cur
	if err := p.advance(); err != nil && err != io.EOF {
		return nil, err
	}
	return cur, nil
}

// Parse parses a complete program and returns the root KindProgram node.
func (p *Parser) Parse() (*Node, error) {
	root := newNode(KindProgram)
	for !p.done {
		// skip stray semicolons
		if p.curTok == lex.TokenSEMIC {
			_ = p.advance()
			continue
		}
		stmt, err := p.parseStmt()
		if err != nil {
			return nil, err
		}
		root.appendChild(stmt)
	}
	return root, nil
}

// currentLine returns the current source line number.
// Uses the fragger's Line() method via interface assertion if available,
// falling back to p.line which is updated on each advance().
func (p *Parser) currentLine() int {
	if f, ok := p.frag.(interface{ Line() int }); ok {
		return f.Line()
	}
	return p.line
}

// parseStmt dispatches to the appropriate statement parser based on current token.
func (p *Parser) parseStmt() (*Node, error) {
	switch p.peek() {
	case lex.TokenVAR:
		return p.parseVarDecl()
	case lex.TokenFUNC:
		return p.parseFuncDecl()
	case lex.TokenIF:
		return p.parseIfStmt()
	case lex.TokenFOR:
		return p.parseForStmt()
	case lex.TokenWHILE:
		return p.parseWhileStmt()
	case lex.TokenEXEC:
		return p.parseExecStmt()
	case lex.TokenBG:
		return p.parseBgStmt()
	case lex.TokenEXIT:
		return p.parseExitStmt()
	case lex.TokenRETURN:
		return p.parseReturnStmt()
	default:
		return p.parseExprStmt()
	}
}

// parseVarDecl parses: "var" Ident "=" Expr ";"
func (p *Parser) parseVarDecl() (*Node, error) {
	varFrag, err := p.eat(lex.TokenVAR)
	if err != nil {
		return nil, err
	}
	n := newNode(KindVarDecl).withTok(lex.TokenVAR).withFrag(varFrag.String())

	nameFrag, err := p.eatIdent()
	if err != nil {
		return nil, fmt.Errorf("var decl name: %w", err)
	}
	n.appendChild(newNode(KindIdent).withFrag(nameFrag.String()))

	if _, err = p.eat(lex.TokenEQ); err != nil {
		return nil, fmt.Errorf("var decl =: %w", err)
	}

	val, err := p.parseExpr()
	if err != nil {
		return nil, fmt.Errorf("var decl value: %w", err)
	}
	n.appendChild(val)

	if _, err = p.eat(lex.TokenSEMIC); err != nil {
		return nil, wrapLineErr(p.currentLine(), fmt.Errorf("var decl ;: %w", err))
	}

	return n, nil
}

// parseFuncDecl parses: "func" Ident "(" Params ")" "{" Stmt* "}"
// Params = (Ident ("," Ident)*)?
func (p *Parser) parseFuncDecl() (*Node, error) {
	if _, err := p.eat(lex.TokenFUNC); err != nil {
		return nil, err
	}

	nameFrag, err := p.eatIdent()
	if err != nil {
		return nil, fmt.Errorf("func decl name: %w", err)
	}
	n := newNode(KindFuncDecl).withTok(lex.TokenFUNC).withFrag(nameFrag.String())

	if _, err = p.eat(lex.TokenLP); err != nil {
		return nil, fmt.Errorf("func decl (: %w", err)
	}

	// parse zero or more comma-separated parameter names
	for p.peek() != lex.TokenRP && !p.done {
		paramFrag, err := p.eatIdent()
		if err != nil {
			return nil, fmt.Errorf("func param: %w", err)
		}
		n.appendChild(newNode(KindIdent).withFrag(paramFrag.String()))
		if p.peek() == lex.TokenCOMMA {
			if err = p.advance(); err != nil && err != io.EOF {
				return nil, err
			}
		}
	}

	if _, err = p.eat(lex.TokenRP); err != nil {
		return nil, fmt.Errorf("func decl ): %w", err)
	}

	body, err := p.parseBlock()
	if err != nil {
		return nil, fmt.Errorf("func body: %w", err)
	}
	n.appendChild(body)

	return n, nil
}

// parseIfStmt parses: "if" Expr "then" "{" Stmt* "}" ("else" "{" Stmt* "}")?
func (p *Parser) parseIfStmt() (*Node, error) {
	if _, err := p.eat(lex.TokenIF); err != nil {
		return nil, err
	}
	n := newNode(KindIfStmt).withTok(lex.TokenIF)

	cond, err := p.parseExpr()
	if err != nil {
		return nil, fmt.Errorf("if condition: %w", err)
	}
	n.appendChild(cond)

	// "then" requires "if" — this mirrors Token.withRequires; the parser enforces what the lexer declares
	if _, err = p.eat(lex.TokenTHEN); err != nil {
		return nil, fmt.Errorf("%w: if requires then: %w", ErrMissingRequires, err)
	}

	thenBlock, err := p.parseBlock()
	if err != nil {
		return nil, fmt.Errorf("if then block: %w", err)
	}
	n.appendChild(thenBlock)

	// optional else — requires then (already satisfied)
	if p.peek() == lex.TokenELSE {
		if _, err = p.eat(lex.TokenELSE); err != nil {
			return nil, err
		}
		elseBlock, err := p.parseBlock()
		if err != nil {
			return nil, fmt.Errorf("if else block: %w", err)
		}
		n.appendChild(elseBlock)
	}

	return n, nil
}

// parseForStmt parses: "for" Ident "=" Expr "{" Stmt* "}"
// simple iterator-style for loop (range-free, Josh's language is minimal)
func (p *Parser) parseForStmt() (*Node, error) {
	if _, err := p.eat(lex.TokenFOR); err != nil {
		return nil, err
	}
	n := newNode(KindForStmt).withTok(lex.TokenFOR)

	// for can iterate over an expression or bind a var
	iterVar, err := p.eatIdent()
	if err != nil {
		return nil, fmt.Errorf("for var: %w", err)
	}
	n.appendChild(newNode(KindIdent).withFrag(iterVar.String()))

	if _, err = p.eat(lex.TokenEQ); err != nil {
		return nil, fmt.Errorf("for =: %w", err)
	}

	iter, err := p.parseExpr()
	if err != nil {
		return nil, fmt.Errorf("for iter expr: %w", err)
	}
	n.appendChild(iter)

	body, err := p.parseBlock()
	if err != nil {
		return nil, fmt.Errorf("for body: %w", err)
	}
	n.appendChild(body)

	return n, nil
}

// parseWhileStmt parses: "while" Expr "{" Stmt* "}"
func (p *Parser) parseWhileStmt() (*Node, error) {
	if _, err := p.eat(lex.TokenWHILE); err != nil {
		return nil, err
	}
	n := newNode(KindWhileStmt).withTok(lex.TokenWHILE)

	cond, err := p.parseExpr()
	if err != nil {
		return nil, fmt.Errorf("while condition: %w", err)
	}
	n.appendChild(cond)

	body, err := p.parseBlock()
	if err != nil {
		return nil, fmt.Errorf("while body: %w", err)
	}
	n.appendChild(body)

	return n, nil
}

// parsePrintStmt parses: "print" Expr ";"
func (p *Parser) parsePrintStmt() (*Node, error) {
	if _, err := p.eat(lex.TokenPRINT); err != nil {
		return nil, err
	}
	expr, err := p.parseExpr()
	if err != nil {
		return nil, fmt.Errorf("print expr: %w", err)
	}
	if _, err = p.eat(lex.TokenSEMIC); err != nil {
		return nil, wrapLineErr(p.currentLine(), fmt.Errorf("print ;: %w", err))
	}
	n := newNode(KindPrintStmt).withLine(p.currentLine())
	n.appendChild(expr)
	return n, nil
}

// parseExecStmt parses: "exec" Expr ("|" "exec" Expr)*
// Multiple exec stages connected by pipe become a KindPipeline node.
func (p *Parser) parseExecStmt() (*Node, error) {
	if _, err := p.eat(lex.TokenEXEC); err != nil {
		return nil, err
	}

	cmd, err := p.parseExpr()
	if err != nil {
		return nil, fmt.Errorf("exec cmd: %w", err)
	}
	first := newNode(KindExecStmt).withTok(lex.TokenEXEC)
	first.appendChild(cmd)

	// check for pipeline
	if p.peek() != lex.TokenPIPE {
		if p.peek() == lex.TokenSEMIC {
			_ = p.advance()
		}
		return first, nil
	}

	// build pipeline node: [exec, exec, ...]
	pipeline := newNode(KindPipeline)
	pipeline.appendChild(first)

	for p.peek() == lex.TokenPIPE {
		if err = p.advance(); err != nil && err != io.EOF {
			return nil, err
		}
		if _, err = p.eat(lex.TokenEXEC); err != nil {
			return nil, fmt.Errorf("pipeline exec: %w", err)
		}
		stageCmd, err := p.parseExpr()
		if err != nil {
			return nil, fmt.Errorf("pipeline cmd: %w", err)
		}
		stage := newNode(KindExecStmt).withTok(lex.TokenEXEC)
		stage.appendChild(stageCmd)
		pipeline.appendChild(stage)
	}

	if p.peek() == lex.TokenSEMIC {
		_ = p.advance()
	}
	return pipeline, nil
}

// parseBgStmt parses: "bg" Expr ";"
func (p *Parser) parseBgStmt() (*Node, error) {
	if _, err := p.eat(lex.TokenBG); err != nil {
		return nil, err
	}
	n := newNode(KindBgStmt).withTok(lex.TokenBG)

	expr, err := p.parseExpr()
	if err != nil {
		return nil, fmt.Errorf("bg expr: %w", err)
	}
	n.appendChild(expr)

	if p.peek() == lex.TokenSEMIC {
		_ = p.advance()
	}
	return n, nil
}

// parseExitStmt parses: "exit" ";"
func (p *Parser) parseExitStmt() (*Node, error) {
	if _, err := p.eat(lex.TokenEXIT); err != nil {
		return nil, err
	}
	if p.peek() == lex.TokenSEMIC {
		_ = p.advance()
	}
	return newNode(KindExitStmt).withTok(lex.TokenEXIT), nil
}

// parseReturnStmt parses: "return" Expr ";"
func (p *Parser) parseReturnStmt() (*Node, error) {
	if _, err := p.eat(lex.TokenRETURN); err != nil {
		return nil, err
	}
	n := newNode(KindReturnStmt).withTok(lex.TokenRETURN)

	expr, err := p.parseExpr()
	if err != nil {
		return nil, fmt.Errorf("return expr: %w", err)
	}
	n.appendChild(expr)

	if p.peek() == lex.TokenSEMIC {
		_ = p.advance()
	}
	return n, nil
}

// parseExprStmt parses: Expr ";"
func (p *Parser) parseExprStmt() (*Node, error) {
	expr, err := p.parseExpr()
	if err != nil {
		return nil, err
	}
	n := newNode(KindExprStmt)
	n.appendChild(expr)

	if p.peek() == lex.TokenSEMIC {
		_ = p.advance()
	}
	return n, nil
}

// parseBlock parses: "{" Stmt* "}"
func (p *Parser) parseBlock() (*Node, error) {
	if _, err := p.eat(lex.TokenLB); err != nil {
		return nil, fmt.Errorf("block {: %w", err)
	}

	// reuse KindProgram for block nodes — a block is just a scoped program
	block := newNode(KindProgram)

	for p.peek() != lex.TokenRB && !p.done {
		if p.peek() == lex.TokenSEMIC {
			_ = p.advance()
			continue
		}
		stmt, err := p.parseStmt()
		if err != nil {
			return nil, err
		}
		block.appendChild(stmt)
	}

	if _, err := p.eat(lex.TokenRB); err != nil {
		return nil, fmt.Errorf("block }: %w", err)
	}
	return block, nil
}

// parseExpr is the top of the expression hierarchy.
// Expr = OrExpr
// TODO: unary negation — MINUS is defined but parsePrimary does not handle it yet
func (p *Parser) parseExpr() (*Node, error) {
	return p.parseOrExpr()
}

// parseOrExpr parses: AndExpr ("||" AndExpr)*
func (p *Parser) parseOrExpr() (*Node, error) {
	left, err := p.parseAndExpr()
	if err != nil {
		return nil, err
	}
	for p.peek() == lex.TokenOR {
		opFrag, err := p.eatAny()
		if err != nil {
			return nil, err
		}
		right, err := p.parseAndExpr()
		if err != nil {
			return nil, fmt.Errorf("|| rhs: %w", err)
		}
		bin := newNode(KindBinaryExpr).withFrag(opFrag.String())
		bin.appendChild(left)
		bin.appendChild(right)
		left = bin
	}
	return left, nil
}

// parseAndExpr parses: CmpExpr ("&&" CmpExpr)*
func (p *Parser) parseAndExpr() (*Node, error) {
	left, err := p.parseCmpExpr()
	if err != nil {
		return nil, err
	}
	for p.peek() == lex.TokenAND {
		opFrag, err := p.eatAny()
		if err != nil {
			return nil, err
		}
		right, err := p.parseCmpExpr()
		if err != nil {
			return nil, fmt.Errorf("&& rhs: %w", err)
		}
		bin := newNode(KindBinaryExpr).withFrag(opFrag.String())
		bin.appendChild(left)
		bin.appendChild(right)
		left = bin
	}
	return left, nil
}

// parseCmpExpr parses: AddExpr (("==" | "!=" | "<" | ">" | "<=" | ">=") AddExpr)?
func (p *Parser) parseCmpExpr() (*Node, error) {
	left, err := p.parseAddExpr()
	if err != nil {
		return nil, err
	}
	switch p.peek() {
	case lex.TokenEQQ, lex.TokenNEQ, lex.TokenLT, lex.TokenGT, lex.TokenLTE, lex.TokenGTE:
		opFrag, err := p.eatAny()
		if err != nil {
			return nil, err
		}
		right, err := p.parseAddExpr()
		if err != nil {
			return nil, fmt.Errorf("comparison rhs: %w", err)
		}
		bin := newNode(KindBinaryExpr).withFrag(opFrag.String())
		bin.appendChild(left)
		bin.appendChild(right)
		return bin, nil
	}
	return left, nil
}

// parseAddExpr parses: MulExpr (("+" | "-") MulExpr)*
func (p *Parser) parseAddExpr() (*Node, error) {
	left, err := p.parseMulExpr()
	if err != nil {
		return nil, err
	}

	for p.peek() == lex.TokenPLUS || p.peek() == lex.TokenMINUS {
		opFrag, err := p.eatAny()
		if err != nil {
			return nil, err
		}
		right, err := p.parseMulExpr()
		if err != nil {
			return nil, fmt.Errorf("binary rhs: %w", err)
		}
		bin := newNode(KindBinaryExpr).withFrag(opFrag.String())
		bin.appendChild(left)
		bin.appendChild(right)
		left = bin
	}

	return left, nil
}

// parseMulExpr parses: UnaryExpr (("*" | "/" | "%") UnaryExpr)*
func (p *Parser) parseMulExpr() (*Node, error) {
	left, err := p.parseUnaryExpr()
	if err != nil {
		return nil, err
	}
	for p.peek() == lex.TokenSTAR || p.peek() == lex.TokenSLASH || p.peek() == lex.TokenPERCENT {
		opFrag, err := p.eatAny()
		if err != nil {
			return nil, err
		}
		right, err := p.parseUnaryExpr()
		if err != nil {
			return nil, fmt.Errorf("mul/div rhs: %w", err)
		}
		bin := newNode(KindBinaryExpr).withFrag(opFrag.String())
		bin.appendChild(left)
		bin.appendChild(right)
		left = bin
	}
	return left, nil
}

// parseUnaryExpr parses: ("-" | "!") UnaryExpr | Primary
func (p *Parser) parseUnaryExpr() (*Node, error) {
	switch p.peek() {
	case lex.TokenMINUS:
		opFrag, err := p.eatAny()
		if err != nil {
			return nil, err
		}
		operand, err := p.parseUnaryExpr()
		if err != nil {
			return nil, fmt.Errorf("unary - operand: %w", err)
		}
		n := newNode(KindUnaryExpr).withFrag(opFrag.String())
		n.appendChild(operand)
		return n, nil
	case lex.TokenNOT:
		opFrag, err := p.eatAny()
		if err != nil {
			return nil, err
		}
		operand, err := p.parseUnaryExpr()
		if err != nil {
			return nil, fmt.Errorf("unary ! operand: %w", err)
		}
		n := newNode(KindUnaryExpr).withFrag(opFrag.String())
		n.appendChild(operand)
		return n, nil
	}
	return p.parsePrimary()
}

// parsePrimary parses: Ident "(" Args ")" | Ident | Lit | "(" Expr ")"
func (p *Parser) parsePrimary() (*Node, error) {
	switch {
	case p.done, p.cur == nil:
		return nil, ErrUnexpectedEOF

	case p.peek() == lex.TokenLP:
		// grouped expression: "(" Expr ")"
		if err := p.advance(); err != nil && err != io.EOF {
			return nil, err
		}
		inner, err := p.parseExpr()
		if err != nil {
			return nil, err
		}
		if _, err = p.eat(lex.TokenRP); err != nil {
			return nil, fmt.Errorf("grouped expr ): %w", err)
		}
		return inner, nil

	case !p.cur.IsToken():
		// identifier or literal
		frag, err := p.eatAny()
		if err != nil {
			return nil, err
		}

		// Go-style call: Ident "(" Args ")"
		if p.peek() == lex.TokenLP {
			return p.parseCallTail(frag.String())
		}

		// plain identifier or literal (we don't have a separate literal token;
		// anything that isn't a keyword is either an ident or a bare value)
		return newNode(KindIdent).withFrag(frag.String()), nil

	default:
		// token where an expression was expected
		return nil, fmt.Errorf("%w: got token %q in expression position",
			ErrInvalidExpression, p.peekFrag())
	}
}

// parseCallTail parses the argument list after an identifier: "(" Args ")"
// Args = (Expr ("," Expr)*)?
func (p *Parser) parseCallTail(name string) (*Node, error) {
	if _, err := p.eat(lex.TokenLP); err != nil {
		return nil, err
	}

	call := newNode(KindCallExpr).withFrag(name)

	for p.peek() != lex.TokenRP && !p.done {
		arg, err := p.parseExpr()
		if err != nil {
			return nil, fmt.Errorf("call arg: %w", err)
		}
		call.appendChild(arg)
		if p.peek() == lex.TokenCOMMA {
			if err = p.advance(); err != nil && err != io.EOF {
				return nil, err
			}
		}
	}

	if _, err := p.eat(lex.TokenRP); err != nil {
		return nil, fmt.Errorf("call ): %w", err)
	}
	return call, nil
}
