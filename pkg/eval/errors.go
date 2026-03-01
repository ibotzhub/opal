package eval

import "errors"

var (
	ErrUndefined    = errors.New("undefined variable")
	ErrTypeMismatch = errors.New("type mismatch")
	ErrArity        = errors.New("wrong number of arguments")
	ErrNotCallable  = errors.New("value is not callable")
	ErrDivByZero    = errors.New("division by zero")
	ErrExecFailed   = errors.New("exec failed")
	ErrBadNode      = errors.New("bad AST node")
)
