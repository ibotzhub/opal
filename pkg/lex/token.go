package lex

import (
	"errors"
	"fmt"
	"slices"
	"sync"
	"unicode/utf8"
	"unsafe"

	"github.com/l0nax/go-spew/spew"
)

type Token struct {
	bLen    int // byte len
	rLen    int // rune len
	tok     []rune
	tokB    []byte
	childOf *Token
	bOne    sync.Once
	mode    Mode
}

func (t *Token) Valid(deep bool) bool {
	switch {
	case t == nil, t.bLen < 1, t.rLen < 1, len(t.tok) < 1,
		t == TokenBAD:
		return false
	default:
		if !deep {
			return true
		}
	}
	if utf8.RuneCountInString(t.string()) != t.rLen {
		panic(fmt.Errorf("%w: rune len mismatch, FUBAR", ErrBadToken))
	}
	switch {
	case t.childOf != nil && !t.childOf.Valid(false),
		t.string() == "BAD", t.string() == "",
		string(t.tok) != t.string():
		return false
	default:
		return true
	}
}

func (t *Token) string() string {
	if len(t.tok) < 1 {
		return "BAD"
	}
	t.bOne.Do(func() {
		t.tokB = []byte(string(t.tok))
	})
	return unsafe.String(&t.tokB[0], t.bLen) // note: this was added to [unsafe] recently
}

func (t *Token) withType(mode Mode) *Token {
	t.mode = mode
	return t
}

func (t *Token) withRequires(parent *Token) *Token {
	t.childOf = parent
	return t
}

type Tree struct {
	m map[rune]*Branch
}

func NewTree(seeds []*Token) (*Tree, error) {
	if len(seeds) < 1 {
		return nil, fmt.Errorf("empty seeds: %w", ErrEmptyTokenSlice)
	}

	t := &Tree{m: make(map[rune]*Branch)}

	seen := make(map[string]struct{})
	for i := range seeds {
		if _, exists := seen[seeds[i].string()]; exists {
			return nil, errors.New("duplicate seed token starting rune: " + spew.Sdump(seeds[i]))
		}
		seen[seeds[i].string()] = struct{}{}
		if branch, exists := t.m[seeds[i].tok[0]]; exists {
			branch.AddToken(seeds[i])
			continue
		}
		t.m[seeds[i].tok[0]] = NewBranch(seeds[i])
	}
	seen = nil
	return t, nil
}

type Branch struct {
	Root                *Token
	parent              *Branch
	distances           []int
	distanceToChildren  map[int][]*Branch
	nextRuneToDistances map[rune][]int
}

func (b *Branch) IsLeaf() bool {
	return len(b.distanceToChildren) < 1
}

func (b *Branch) addTokenIndexFunc(tok *Token) bool {
	rootLen := b.Root.rLen
	distance := tok.rLen - rootLen

	switch {
	case tok.rLen <= rootLen:
		// invalid token length, can't be a child
		return true
	case tok.tok[rootLen-1] != b.Root.tok[rootLen-1]:
		// not a child of this branch, does not start with a matching rune
		return true
	case distance < 1:
		// invalid distance, can't be a child
		panic(errors.New("passed !rLen <= rootLen, but distance < 1... FUBAR!!!11eleven"))
	default:
		//
	}

	willSort := b.nextRuneToDistances[tok.tok[rootLen]] != nil && len(b.nextRuneToDistances[tok.tok[rootLen]]) > 0
	b.nextRuneToDistances[tok.tok[rootLen]] = append(b.nextRuneToDistances[tok.tok[rootLen]], distance)
	if willSort {
		slices.Sort(b.nextRuneToDistances[tok.tok[rootLen]])
	}

	b.distanceToChildren[distance] = append(b.distanceToChildren[distance], NewBranch(tok))

	return false
}

var (
	ErrEmptyToken      = errors.New("empty token")
	ErrBadToken        = errors.New("bad token")
	ErrEmptyTokenSlice = errors.New("empty token slice")
	ErrUnreadRune      = errors.New("UnreadRune: no rune to unread")
)

func (b *Branch) AddToken(t *Token) error {
	if len(t.tok) < 1 {
		return ErrEmptyToken
	}
	if !t.Valid(false) {
		return ErrBadToken
	}
	if badIndex := slices.IndexFunc([]*Token{t}, b.addTokenIndexFunc); badIndex > -1 {
		return errors.New("invalid token in slice: " + spew.Sdump(t))
	}
	return nil
}

func (b *Branch) AddTokens(ts []*Token) {
	if len(ts) < 1 {
		panic(ErrEmptyTokenSlice)
	}

	// note: we sort b.nextRuneToDistances in addTokenIndexFunc;
	// so closest Branch will always be the first element in the slice.
	if badIndex := slices.IndexFunc(ts, b.addTokenIndexFunc); badIndex > 0 {
		panic(errors.New("invalid token in slice: " + spew.Sdump(ts[badIndex])))
	}

}

func NewBranch(t *Token) *Branch {
	return &Branch{
		Root:                t,
		distanceToChildren:  make(map[int][]*Branch),
		nextRuneToDistances: make(map[rune][]int),
	}
}

func (b *Branch) HasMatches(r rune) (closest int, ok bool) {
	var distances []int
	if distances, ok = b.nextRuneToDistances[r]; !ok {
		return 0, false
	}
	// sorted slice, so closest match is always the first element
	return distances[0], true
}

var TokenBAD = &Token{bLen: 0, rLen: 0, tok: []rune{'B', 'A', 'D'}, tokB: []byte{}, mode: modeNone}

// 1 rune tokens
var (
	TokenEQ    = NewTokenFromString(EQ).withType(modeControl)
	TokenPLUS  = NewTokenFromString(PLUS).withType(modeModifier)
	TokenMINUS = NewTokenFromString(MINUS).withType(modeModifier)
	TokenCOMMA = NewTokenFromString(COMMA).withType(modeControl)
	TokenSEMIC = NewTokenFromString(SEMIC).withType(modeControl)
	TokenLP    = NewTokenFromString(LP).withType(modeControl)
	TokenRP    = NewTokenFromString(RP).withType(modeControl)
	TokenLB    = NewTokenFromString(LB).withType(modeControl)
	TokenRB    = NewTokenFromString(RB).withType(modeControl)
	TokenPIPE    = NewTokenFromString(PIPE).withType(modeControl)
	TokenSTAR    = NewTokenFromString(STAR).withType(modeModifier)
	TokenSLASH   = NewTokenFromString(SLASH).withType(modeModifier)
	TokenPERCENT = NewTokenFromString(PERCENT).withType(modeModifier)
)

// 2 rune tokens
var (
	TokenIF = NewTokenFromString(IF).withType(modeControl)
	TokenBG = NewTokenFromString(PARALLEL).withType(modeCommand)
)

// 3 rune tokens
var (
	TokenSTRING = NewTokenFromString(STRING).withType(modeType)
	TokenNUM    = NewTokenFromString(NUM).withType(modeType)
	TokenVAR    = NewTokenFromString(VAR).withType(modeAssign)
	TokenFOR    = NewTokenFromString(FOR).withType(modeControl)
	TokenEOF    = NewTokenFromString(EOF).withType(modeNone)
)

// 4 rune tokens
var (
	TokenBOOL = NewTokenFromString(BOOL).withType(modeType)
	TokenFUNC = NewTokenFromString(FUNC).withType(modeAssign)
	TokenTHEN = NewTokenFromString(THEN).withType(modeControl).withRequires(TokenIF)
	TokenELSE = NewTokenFromString(ELSE).withType(modeControl).withRequires(TokenTHEN)
	TokenEXEC = NewTokenFromString(EXEC).withType(modeCommand)
	TokenEXIT = NewTokenFromString(EXIT).withType(modeCommand)
)

// 5 rune tokens
var (
	TokenWHILE = NewTokenFromString(WHILE).withType(modeControl)
	TokenPRINT = NewTokenFromString(PRINT).withType(modeCommand)
)

// 6 rune tokens
var (
	TokenRETURN = NewTokenFromString(RETURN).withType(modeControl)
)

var tokens = []*Token{
	TokenEQ, TokenPLUS, TokenMINUS, TokenCOMMA, TokenSEMIC,
	TokenLP, TokenRP, TokenLB, TokenRB, TokenPIPE,
	TokenIF, TokenBG, TokenSTRING, TokenNUM, TokenVAR,
	TokenFOR, TokenEOF, TokenBOOL, TokenFUNC, TokenTHEN,
	TokenELSE, TokenEXEC, TokenEXIT, TokenWHILE, TokenRETURN,
	TokenSTAR, TokenSLASH, TokenPERCENT, TokenPRINT,
}

var stringToToken = map[string]*Token{
	"=": TokenEQ, "+": TokenPLUS, "-": TokenMINUS, ",": TokenCOMMA,
	";": TokenSEMIC, "(": TokenLP, ")": TokenRP, "{": TokenLB,
	"}": TokenRB, "|": TokenPIPE, "if": TokenIF, "bg": TokenBG,
	"str": TokenSTRING, "int": TokenNUM, "var": TokenVAR,
	"for": TokenFOR, "EOF": TokenEOF, "bool": TokenBOOL,
	"func": TokenFUNC, "then": TokenTHEN, "else": TokenELSE,
	"exec": TokenEXEC, "exit": TokenEXIT, "while": TokenWHILE,
	"return": TokenRETURN, "BAD": TokenBAD,
	"*": TokenSTAR, "/": TokenSLASH, "%": TokenPERCENT,
	"print": TokenPRINT,
}

func NewTokenFromString(s string) *Token {
	t := &Token{tok: []rune(s), rLen: utf8.RuneCountInString(s), bLen: len(s)}
	return t
}

func TokenFromString(s string) *Token {
	if t, ok := stringToToken[s]; ok {
		return t
	}
	return TokenBAD
}

func init() {
	for i := range tokens {
		if !tokens[i].Valid(true) {
			spew.Dump(tokens[i])
			panic("invalid token: " + tokens[i].string())
		}
	}

}

var singleRuneTokens = map[rune]*Token{
	'=': TokenEQ, '+': TokenPLUS, '-': TokenMINUS, ',': TokenCOMMA,
	';': TokenSEMIC, '(': TokenLP, ')': TokenRP, '{': TokenLB,
	'}': TokenRB, '|': TokenPIPE,
	'*': TokenSTAR, '/': TokenSLASH, '%': TokenPERCENT,
}

var firstCharTokens = map[rune][]*Token{}

func init() {
	for i := range tokens {
		firstCharTokens[tokens[i].tok[0]] = append(firstCharTokens[tokens[i].tok[0]], tokens[i])
	}
}

func isRuneAToken(r rune) (*Token, bool) {
	_, ok := singleRuneTokens[r]
	if !ok {
		return nil, false
	}
	return singleRuneTokens[r], true
}

// comparison + logical operator tokens
// these are NOT in singleRuneTokens — the Fragger handles them via
// peekable multi-rune scanning. see fragment.go readOp().
var (
	TokenEQQ = NewTokenFromString(EQQ).withType(modeControl) // ==
	TokenNEQ = NewTokenFromString(NEQ).withType(modeControl) // !=
	TokenLT  = NewTokenFromString(LT).withType(modeControl)  // <
	TokenGT  = NewTokenFromString(GT).withType(modeControl)  // >
	TokenLTE = NewTokenFromString(LTE).withType(modeControl) // <=
	TokenGTE = NewTokenFromString(GTE).withType(modeControl) // >=
	TokenNOT = NewTokenFromString(NOT).withType(modeControl) // !
	TokenAND = NewTokenFromString(AND).withType(modeControl) // &&
	TokenOR  = NewTokenFromString(OR).withType(modeControl)  // ||
)

func init() {
	opTokens := []*Token{
		TokenEQQ, TokenNEQ, TokenLT, TokenGT, TokenLTE, TokenGTE,
		TokenNOT, TokenAND, TokenOR,
	}
	for _, t := range opTokens {
		stringToToken[t.string()] = t
	}
}

// opFirstRunes is the set of runes that may begin a multi-rune operator.
// The Fragger checks this to decide whether to call readOp().
var opFirstRunes = map[rune]struct{}{
	'=': {}, '!': {}, '<': {}, '>': {}, '&': {}, '|': {},
}
