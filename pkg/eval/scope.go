package eval

import (
	"fmt"
)

// Scope is a lexical variable scope. Scopes form a chain via parent,
// mirroring Josh's Token.childOf linked structure.
type Scope struct {
	vars   map[string]*Value
	parent *Scope
}

// NewScope creates a top-level scope with no parent.
func NewScope() *Scope {
	return &Scope{vars: make(map[string]*Value)}
}

// child creates a new scope whose parent is s.
// Used when entering a block or function call.
func (s *Scope) child() *Scope {
	return &Scope{vars: make(map[string]*Value), parent: s}
}

// Set binds name to val in the innermost scope that already has it,
// or in the current scope if it's a new binding.
func (s *Scope) Set(name string, val *Value) {
	if existing := s.find(name); existing != nil {
		existing.vars[name] = val
		return
	}
	s.vars[name] = val
}

// Declare always binds name in the current (innermost) scope.
// Used by var declarations to shadow outer bindings.
func (s *Scope) Declare(name string, val *Value) {
	s.vars[name] = val
}

// Get looks up name through the scope chain.
func (s *Scope) Get(name string) (*Value, bool) {
	if sc := s.find(name); sc != nil {
		return sc.vars[name], true
	}
	return Nil, false
}

// find returns the innermost Scope in the chain that contains name, or nil.
func (s *Scope) find(name string) *Scope {
	for cur := s; cur != nil; cur = cur.parent {
		if _, ok := cur.vars[name]; ok {
			return cur
		}
	}
	return nil
}

// MustGet looks up name and panics with ErrUndefined if not found.
// Used in expression evaluation where missing vars are hard errors.
func (s *Scope) MustGet(name string) *Value {
	v, ok := s.Get(name)
	if !ok {
		panic(fmt.Errorf("%w: %q", ErrUndefined, name))
	}
	return v
}
