//go:build ignore

package main

import (
	. "github.com/mmcloughlin/avo/build"
	. "github.com/mmcloughlin/avo/operand"

	"git.tcp.direct/kayos/opal/pkg/lex"
)

// bs are the single-char opal tokens stored at their ASCII offsets.
// lookup(int(r)) returns r if r is in the table, 0 if not.
// This provides O(1) branchless token membership for the lexer.
var bs = []String{
	String(' '), lex.EQ, lex.LB, lex.RB, lex.LP, lex.RP, lex.SEMIC, lex.COMMA,
	lex.PLUS, lex.MINUS, lex.PIPE,
}

func main() {
	bytes := GLOBL("bytes", RODATA|NOPTR)
	for _, b := range bs {
		if len([]byte(b)) != 1 {
			panic("bad byte: opal single-char tokens must be ASCII")
		}
		DATA(int(string(b)[0]), b) // offset = ASCII value of the character
	}

	TEXT("lookup", NOSPLIT, "func(i int) byte")
	Doc("lookup returns byte i in the 'bytes' global data section.",
		"Pass int(r) to test whether rune r is a single-char opal token.",
		"Returns the character itself if it is a token, 0 if it is not.")
	i := Load(Param("i"), GP64())
	ptr := Mem{Base: GP64()}
	LEAQ(bytes, ptr.Base)
	b := GP8()
	MOVB(ptr.Idx(i, 1), b)
	Store(b, ReturnIndex(0))
	RET()

	Generate()
}
