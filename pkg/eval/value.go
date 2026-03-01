package eval

import (
	"fmt"
	"strconv"

	"git.tcp.direct/kayos/opal/pkg/parse"
)

// Type is the runtime type tag for opal values.
// Mirrors Josh's Mode/Kind iota pattern.
type Type int

const (
	TypeNil Type = iota
	TypeInt
	TypeStr
	TypeBool
	TypeFunc
)

var typeToName = map[Type]string{
	TypeNil:  "nil",
	TypeInt:  "int",
	TypeStr:  "str",
	TypeBool: "bool",
	TypeFunc: "func",
}

func (t Type) String() string {
	if s, ok := typeToName[t]; ok {
		return s
	}
	return "unknown"
}

// Value is opal's runtime value. Exactly one field is set based on typ.
// Josh kept Token fields unexported with accessor methods; we do the same.
type Value struct {
	typ  Type
	iVal int
	sVal string
	bVal bool
	fVal *funcVal
}

// funcVal holds a user-defined function's parameter list and body.
type funcVal struct {
	params []string
	body   *parse.Node // KindProgram block node
	scope  *Scope      // closure scope at definition site
}

// Nil is the zero value singleton.
var Nil = &Value{typ: TypeNil}

func IntVal(i int) *Value        { return &Value{typ: TypeInt, iVal: i} }
func StrVal(s string) *Value     { return &Value{typ: TypeStr, sVal: s} }
func BoolVal(b bool) *Value      { return &Value{typ: TypeBool, bVal: b} }

func funcValue(params []string, body *parse.Node, scope *Scope) *Value {
	return &Value{typ: TypeFunc, fVal: &funcVal{params: params, body: body, scope: scope}}
}

func (v *Value) Type() Type { return v.typ }

func (v *Value) Int() (int, bool) {
	if v.typ != TypeInt {
		return 0, false
	}
	return v.iVal, true
}

func (v *Value) Str() (string, bool) {
	if v.typ != TypeStr {
		return "", false
	}
	return v.sVal, true
}

func (v *Value) Bool() (bool, bool) {
	if v.typ != TypeBool {
		return false, false
	}
	return v.bVal, true
}

// Truthy returns the boolean interpretation of any value.
// Follows Go conventions: zero/empty/nil are falsy.
func (v *Value) Truthy() bool {
	switch v.typ {
	case TypeNil:
		return false
	case TypeInt:
		return v.iVal != 0
	case TypeStr:
		return v.sVal != ""
	case TypeBool:
		return v.bVal
	case TypeFunc:
		return v.fVal != nil
	default:
		return false
	}
}

// String returns a human-readable representation, used for exec args and display.
func (v *Value) String() string {
	switch v.typ {
	case TypeNil:
		return "<nil>"
	case TypeInt:
		return strconv.Itoa(v.iVal)
	case TypeStr:
		return v.sVal
	case TypeBool:
		if v.bVal {
			return "true"
		}
		return "false"
	case TypeFunc:
		return fmt.Sprintf("<func(%d)>", len(v.fVal.params))
	default:
		return "<unknown>"
	}
}

// coerce attempts to parse a raw string fragment into a typed value.
// tries int, then bool, then gives up and calls it a string. good enough.
// Tries int, then bool, then falls back to str.
func coerce(s string) *Value {
	if i, err := strconv.Atoi(s); err == nil {
		return IntVal(i)
	}
	switch s {
	case "true":
		return BoolVal(true)
	case "false":
		return BoolVal(false)
	}
	return StrVal(s)
}

// add performs the "+" operation between two values.
// int+int = int, anything else = string concatenation.
func add(a, b *Value) (*Value, error) {
	switch {
	case a.typ == TypeInt && b.typ == TypeInt:
		return IntVal(a.iVal + b.iVal), nil
	default:
		return StrVal(a.String() + b.String()), nil
	}
}

// sub performs the "-" operation. Only defined for int-int.
func sub(a, b *Value) (*Value, error) {
	switch {
	case a.typ == TypeInt && b.typ == TypeInt:
		return IntVal(a.iVal - b.iVal), nil
	default:
		return nil, fmt.Errorf("%w: cannot subtract %s from %s", ErrTypeMismatch, b.Type(), a.Type())
	}
}

// mul performs the "*" operation. Only defined for int-int.
func mul(a, b *Value) (*Value, error) {
	if a.typ == TypeInt && b.typ == TypeInt {
		return IntVal(a.iVal * b.iVal), nil
	}
	return nil, fmt.Errorf("%w: cannot multiply %s by %s", ErrTypeMismatch, a.Type(), b.Type())
}

// div performs the "/" operation. Only defined for int-int. Panics on zero divisor.
func div(a, b *Value) (*Value, error) {
	if a.typ != TypeInt || b.typ != TypeInt {
		return nil, fmt.Errorf("%w: cannot divide %s by %s", ErrTypeMismatch, a.Type(), b.Type())
	}
	if b.iVal == 0 {
		return nil, fmt.Errorf("%w: division by zero", ErrDivByZero)
	}
	return IntVal(a.iVal / b.iVal), nil
}

// mod performs the "%" operation. Only defined for int-int.
func mod(a, b *Value) (*Value, error) {
	if a.typ != TypeInt || b.typ != TypeInt {
		return nil, fmt.Errorf("%w: cannot mod %s by %s", ErrTypeMismatch, a.Type(), b.Type())
	}
	if b.iVal == 0 {
		return nil, fmt.Errorf("%w: mod by zero", ErrDivByZero)
	}
	return IntVal(a.iVal % b.iVal), nil
}

// cmpOp applies an integer comparison function to two values.
// Both must be TypeInt — str comparison via < > is not defined.
func cmpOp(a, b *Value, fn func(int, int) bool) (*Value, error) {
	ai, aok := a.Int()
	bi, bok := b.Int()
	if !aok || !bok {
		return nil, fmt.Errorf("%w: < > <= >= require int operands, got %s and %s",
			ErrTypeMismatch, a.Type(), b.Type())
	}
	return BoolVal(fn(ai, bi)), nil
}
