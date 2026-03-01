package parse

import (
	"git.tcp.direct/kayos/opal/pkg/lex"
)

// Kind identifies what kind of AST node this is.
// Josh's Mode pattern: iota, lowercase, package-private unless exported is needed.
type Kind int

const (
	kindBad Kind = iota
	KindProgram
	KindVarDecl
	KindFuncDecl
	KindIfStmt
	KindForStmt
	KindWhileStmt
	KindExecStmt
	KindBgStmt
	KindExitStmt
	KindReturnStmt
	KindExprStmt
	KindPipeline
	KindCallExpr
	KindBinaryExpr
	KindUnaryExpr
	KindPrintStmt
	KindIdent
	KindLit
)

var kindToName = map[Kind]string{
	kindBad:        "bad",
	KindProgram:    "program",
	KindVarDecl:    "var_decl",
	KindFuncDecl:   "func_decl",
	KindIfStmt:     "if_stmt",
	KindForStmt:    "for_stmt",
	KindWhileStmt:  "while_stmt",
	KindExecStmt:   "exec_stmt",
	KindBgStmt:     "bg_stmt",
	KindExitStmt:   "exit_stmt",
	KindReturnStmt: "return_stmt",
	KindExprStmt:   "expr_stmt",
	KindPipeline:   "pipeline",
	KindCallExpr:   "call_expr",
	KindBinaryExpr: "binary_expr",
	KindUnaryExpr:  "unary_expr",
	KindPrintStmt:  "print_stmt",
	KindIdent:      "ident",
	KindLit:        "lit",
}

func (k Kind) String() string {
	if s, ok := kindToName[k]; ok {
		return s
	}
	return "unknown"
}

func (k Kind) Valid() bool {
	_, ok := kindToName[k]
	return ok && k != kindBad
}

// Node is the core AST type.
// Josh's Token uses unexported fields + method access; we follow the same pattern.
// children is ordered; for binary expressions: [left, right].
// for blocks: all statements in order.
type Node struct {
	kind     Kind
	tok      *lex.Token  // the token that originated this node, may be nil for synthetic nodes
	frag     string      // the raw fragment text (identifier name, literal value)
	children []*Node
	parent   *Node
	line     int // 1-based source line, set by parser
}

// Kind returns the node's kind.
func (n *Node) Kind() Kind {
	return n.kind
}

// Token returns the originating lex token, or nil.
func (n *Node) Token() *lex.Token {
	return n.tok
}

// Frag returns the raw source text fragment for this node.
func (n *Node) Frag() string {
	return n.frag
}

// Children returns the node's children in order.
func (n *Node) Children() []*Node {
	return n.children
}

// Parent returns the parent node, or nil for the root.
func (n *Node) Parent() *Node {
	return n.parent
}

// Line returns the 1-based source line where this node was parsed.
// Returns 0 if line information is unavailable.
func (n *Node) Line() int {
	return n.line
}

func (n *Node) withLine(l int) *Node {
	n.line = l
	return n
}

// IsLeaf reports whether this node has no children.
func (n *Node) IsLeaf() bool {
	return len(n.children) < 1
}

// Valid reports whether the node is well-formed.
func (n *Node) Valid() bool {
	switch {
	case n == nil, !n.kind.Valid():
		return false
	default:
		return true
	}
}

// child returns child at index i, or nil.
func (n *Node) child(i int) *Node {
	if i < 0 || i >= len(n.children) {
		return nil
	}
	return n.children[i]
}

func newNode(kind Kind) *Node {
	return &Node{kind: kind}
}

func (n *Node) withTok(t *lex.Token) *Node {
	n.tok = t
	return n
}

func (n *Node) withFrag(s string) *Node {
	n.frag = s
	return n
}

func (n *Node) withParent(p *Node) *Node {
	n.parent = p
	return n
}

func (n *Node) appendChild(child *Node) *Node {
	child.parent = n
	n.children = append(n.children, child)
	return n
}
