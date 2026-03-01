package lex

import (
	"io"
	"sync"
	"unicode/utf8"
)

type noCopy struct{}

func (*noCopy) Lock()   {}
func (*noCopy) Unlock() {}

type fragLen int // utf8 rune len sum

func (fl fragLen) Len() int {
	return int(fl)
}

// Fragmenter is the interface implemented by Fragger.
// It reads from a byte source, fragments the input into Fragments,
// and exposes RuneScanner semantics for the caller.
type Fragmenter interface {
	io.ByteWriter
	io.RuneScanner
	io.Closer
	Next() *Fragment
	More() bool
}

// Fragger reads bytes from src, accumulates them into its pooled buf,
// and emits Fragments delimited by whitespace or single-rune tokens.
//
// Fields:
//
//	src   – the underlying byte source
//	buf   – pooled scratch space for multi-byte utf8 accumulation
//	runes – decoded rune lookahead buffer for RuneScanner
//	cur   – index of the current rune in runes
//	off   – byte offset into src
//	last  – byte width of last rune read, for UnreadRune
type Fragger struct {
	src   io.ByteScanner
	buf   []byte
	runes []rune
	cur   int
	off   int
	last  int // byte width of last rune read, for UnreadRune
	line  int // current line number (1-based)
	nc    noCopy
}

var fragBufs = sync.Pool{
	New: func() interface{} {
		return make([]byte, 16)
	},
}

func getBuf() []byte {
	return fragBufs.Get().([]byte)
}

func putBuf(b []byte) {
	b = b[:0]
	fragBufs.Put(b)
}

func NewFragger(src io.ByteScanner) *Fragger {
	f := &Fragger{src: src, line: 1}
	f.buf = getBuf()
	return f
}

// WriteByte implements io.ByteWriter.
// It appends a raw byte to the internal buffer.
func (f *Fragger) WriteByte(c byte) error {
	f.buf = append(f.buf, c)
	return nil
}

// readNextRune reads bytes from src until a complete utf8 rune is decoded.
func (f *Fragger) readNextRune() (rune, int, error) {
	b, err := f.src.ReadByte()
	if err != nil {
		return 0, 0, err
	}
	f.off++

	if b < utf8.RuneSelf {
		// fast path: ASCII
		if b == '\n' {
			f.line++
		}
		return rune(b), 1, nil
	}

	// multi-byte: accumulate until we have a full rune
	f.buf = f.buf[:0]
	f.buf = append(f.buf, b)
	for !utf8.FullRune(f.buf) {
		b, err = f.src.ReadByte()
		if err != nil {
			// incomplete sequence — FUBAR input, return replacement
			return utf8.RuneError, len(f.buf), err
		}
		f.off++
		f.buf = append(f.buf, b)
	}
	r, size := utf8.DecodeRune(f.buf)
	return r, size, nil
}

// ReadRune implements io.RuneReader / io.RuneScanner.
func (f *Fragger) ReadRune() (r rune, size int, err error) {
	if f.cur < len(f.runes) {
		r = f.runes[f.cur]
		size = utf8.RuneLen(r)
		if size < 0 {
			size = 1
		}
		f.last = size
		f.cur++
		return r, size, nil
	}
	r, size, err = f.readNextRune()
	if err != nil {
		return r, size, err
	}
	f.runes = append(f.runes, r)
	f.last = size
	f.cur++
	return r, size, nil
}

// UnreadRune implements io.RuneScanner.
// Only one unread is guaranteed. Don't be greedy.
func (f *Fragger) UnreadRune() error {
	if f.cur < 1 {
		return ErrUnreadRune
	}
	f.cur--
	f.off -= f.last
	f.last = 0
	return nil
}

// More reports whether there is more input available from src.
func (f *Fragger) More() bool {
	_, err := f.src.ReadByte()
	if err != nil {
		return false
	}
	_ = f.src.UnreadByte() // just peeking
	return true
}

// Next reads the next Fragment from the source.
// A Fragment is a maximal sequence of runes that is either:
//   - a single-rune token (=, +, {, etc.) — emitted immediately on its own
//   - a contiguous run of non-whitespace, non-token runes (idents, keywords, literals)
//
// Whitespace is consumed and discarded as a delimiter.
// Returns nil at EOF.
func (f *Fragger) Next() *Fragment {
	for {
		r, _, err := f.ReadRune()
		if err != nil {
			return nil
		}

		if isWhitespace(r) {
			continue
		}

		// quoted string literal — hand off to readQuoted for full accumulation
		if r == '"' {
			return f.readQuoted()
		}

		// operator-first runes need peek-ahead to resolve == vs =, != vs !, etc.
		if _, ok := opFirstRunes[r]; ok {
			return f.readOp(r)
		}

		// remaining single-rune tokens get their own fragment, no accumulation needed
		if _, ok := singleRuneTokens[r]; ok {
			return newFragment([]rune{r})
		}

		// accumulate a multi-rune fragment until we hit whitespace or a token rune
		acc := getRuneBuf()
		acc = append(acc, r)

		for {
			r2, _, err2 := f.ReadRune()
			if err2 != nil {
				// EOF mid-accumulation — ship what we have
				frag := newFragment(acc)
				putRuneBuf(acc)
				return frag
			}
			if isWhitespace(r2) {
				break
			}
			if _, ok := singleRuneTokens[r2]; ok {
				_ = f.UnreadRune() // put it back for next call
				break
			}
			acc = append(acc, r2)
		}

		frag := newFragment(acc)
		putRuneBuf(acc)
		return frag
	}
}

// Line returns the current source line number (1-based).
func (f *Fragger) Line() int {
	return f.line
}

// Close returns pooled buffers and clears state.
func (f *Fragger) Close() error {
	putBuf(f.buf)
	f.buf = nil
	f.runes = nil
	return nil
}

// Fragment is an immutable sequence of runes emitted by a Fragger.
// bLen is the utf8 byte length of the rune sequence.
type Fragment struct {
	b    []rune
	bLen int
	nc   noCopy
}

// Runes returns the rune slice of the fragment.
func (fg *Fragment) Runes() []rune {
	return fg.b
}

// Len returns the utf8 byte length.
func (fg *Fragment) Len() int {
	return fg.bLen
}

// RuneLen returns the number of runes.
func (fg *Fragment) RuneLen() int {
	return len(fg.b)
}

// String returns the fragment as a string.
func (fg *Fragment) String() string {
	return string(fg.b)
}

// IsToken reports whether this fragment exactly matches a known token string.
func (fg *Fragment) IsToken() bool {
	_, ok := stringToToken[fg.String()]
	return ok
}

// Token returns the Token matching this fragment, or TokenBAD.
func (fg *Fragment) Token() *Token {
	return TokenFromString(fg.String())
}

func newFragment(runes []rune) *Fragment {
	bLen := 0
	for _, r := range runes {
		bLen += utf8.RuneLen(r)
	}
	cp := make([]rune, len(runes))
	copy(cp, runes)
	return &Fragment{b: cp, bLen: bLen}
}

// runeBufs pools rune accumulation slices used inside Next().
var runeBufs = sync.Pool{
	New: func() interface{} {
		s := make([]rune, 0, 16)
		return &s
	},
}

func getRuneBuf() []rune {
	p := runeBufs.Get().(*[]rune)
	s := (*p)[:0]
	return s
}

func putRuneBuf(s []rune) {
	s = s[:0]
	runeBufs.Put(&s)
}

func isWhitespace(r rune) bool {
	return r == ' ' || r == '\t' || r == '\r' || r == '\n'
}

// readQuoted reads a double-quoted string literal, consuming everything up to
// the matching closing quote. Escape sequences: \" and \\.
// The returned Fragment includes the surrounding quotes so the evaluator can
// distinguish a string literal from a bare identifier.
// yeet — this is what that placeholder was waiting for :^)
func (f *Fragger) readQuoted() *Fragment {
	acc := getRuneBuf()
	acc = append(acc, '"')
	for {
		r, _, err := f.ReadRune()
		if err != nil {
			// unterminated string — FUBAR input, ship what we have
			break
		}
		if r == '\' {
			// escape: consume next rune raw
			r2, _, err2 := f.ReadRune()
			if err2 != nil {
				break
			}
			acc = append(acc, '\', r2)
			continue
		}
		acc = append(acc, r)
		if r == '"' {
			break
		}
	}
	frag := newFragment(acc)
	putRuneBuf(acc)
	return frag
}

// Reader wraps a buffer for positioned reading.
// Reserved for future parser use.
type Reader struct {
	b   readerBuf
	off int64
}

// readerBuf is the internal buffer interface used by Reader.
// bytes.Buffer already satisfies this — see below.
type readerBuf interface {
	io.ReadWriter
	io.WriterTo
	io.ByteReader
	io.RuneScanner
	io.RuneReader
}

// readOp reads a multi-rune operator starting with r.
// It peeks one rune ahead to resolve ambiguous prefixes:
//
//	=  → = or ==
//	!  → ! or !=
//	<  → < or <=
//	>  → > or >=
//	&  → && (bare & is not an opal operator)
//	|  → | or ||
//
// The resolved operator string is looked up in stringToToken.
// If no match, returns the single-rune token (or TokenBAD for bare & ).
func (f *Fragger) readOp(r rune) *Fragment {
	next, _, err := f.ReadRune()
	if err != nil {
		// EOF after first rune — emit what we have
		return newFragment([]rune{r})
	}

	two := string([]rune{r, next})
	if _, ok := stringToToken[two]; ok {
		return newFragment([]rune(two))
	}

	// second rune doesn't form a known two-rune op — put it back
	_ = f.UnreadRune()
	return newFragment([]rune{r})
}
