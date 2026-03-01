package parse

import (
	"errors"
	"fmt"
)

var (
	ErrUnexpectedEOF     = errors.New("unexpected EOF")
	ErrUnexpectedToken   = errors.New("unexpected token")
	ErrMissingToken      = errors.New("missing expected token")
	ErrMissingRequires   = errors.New("token requires preceding token not found")
	ErrInvalidExpression = errors.New("invalid expression")
	ErrEmptyBlock        = errors.New("empty block")
)

// ParseError wraps a parse error with a line number for better diagnostics.
type ParseError struct {
	Line int
	Err  error
}

func (e *ParseError) Error() string {
	if e.Line > 0 {
		return fmt.Sprintf("line %d: %s", e.Line, e.Err.Error())
	}
	return e.Err.Error()
}

func (e *ParseError) Unwrap() error { return e.Err }

// wrapLineErr wraps err with a line number if line > 0.
func wrapLineErr(line int, err error) error {
	if err == nil || line <= 0 {
		return err
	}
	return &ParseError{Line: line, Err: err}
}
