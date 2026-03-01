package eval

// signals are used for non-local control flow (return, exit).
// They are panic'd inside the evaluator and recover'd at the appropriate boundary.
// This is idiomatic for tree-walking interpreters in Go.

type returnSignal struct {
	val *Value
}

type exitSignal struct {
	code int
}

// throwReturn is called when a return statement is evaluated.
func throwReturn(v *Value) {
	panic(returnSignal{val: v})
}

// throwExit is called when an exit statement is evaluated.
func throwExit(code int) {
	panic(exitSignal{code: code})
}

// catchReturn recovers a returnSignal from a deferred function.
// If the recovered value is not a returnSignal, it re-panics.
// out is set to the returned value if a return was caught.
func catchReturn(out **Value) {
	r := recover()
	if r == nil {
		return
	}
	if sig, ok := r.(returnSignal); ok {
		*out = sig.val
		return
	}
	// not a return signal — propagate
	panic(r)
}
