package shaderport

import (
	"errors"
	"fmt"
)

// Each rejection wraps one of these, so a caller (and a test) can
// match the class with errors.Is. The translator never writes output
// for a construct it cannot translate: it stops with a named error.
var (
	// ErrUnsupported marks a construct outside the documented subset:
	// iMouse, iChannel*, texture sampling, uniforms, and the GLSL
	// built-in functions MSL has no match for.
	ErrUnsupported = errors.New("unsupported")
	// ErrMutableGlobal marks a global variable that is not const.
	// MSL has no mutable program-scope variables.
	ErrMutableGlobal = errors.New("mutable global variable")
	// ErrNoEntry marks a source with neither a mainImage function nor
	// a go-gui body that declares frag_color.
	ErrNoEntry = errors.New("no entry point")
	// ErrSyntax marks a source the structural pass cannot split into
	// declarations: unbalanced brackets, a stray token.
	ErrSyntax = errors.New("syntax")
)

// Error is a rejection at a source position.
type Error struct {
	Line, Col int    // 1-based; 0 when the error has no single position
	Kind      error  // one of the Err* values
	Msg       string // what was found, and what to do about it
}

func (e *Error) Error() string {
	if e.Line == 0 {
		return fmt.Sprintf("%v: %s", e.Kind, e.Msg)
	}
	return fmt.Sprintf("%d:%d: %v: %s", e.Line, e.Col, e.Kind, e.Msg)
}

func (e *Error) Unwrap() error { return e.Kind }

// errAt builds an Error at token t.
func errAt(t token, kind error, format string, args ...any) *Error {
	return &Error{Line: t.line, Col: t.col, Kind: kind, Msg: fmt.Sprintf(format, args...)}
}
