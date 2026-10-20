//go:build js && wasm

package web

import "syscall/js"

// Readers for fields of DOM events, beside jsInt and jsString in
// events.go. Split out to hold the large-files gate.

// jsFloat reads a JS number as a float64, or 0 when the value is
// missing or not a number. Value.Float panics on a non-number.
func jsFloat(v js.Value) float64 {
	if v.Type() != js.TypeNumber {
		return 0
	}
	return v.Float()
}

// jsBool reads a JS boolean, or false when the value is missing or not
// a boolean. Value.Bool panics on a non-boolean. An event a page or the
// browser builds with new Event() carries none of the KeyboardEvent or
// MouseEvent fields: Chrome's password autofill sends such a keydown
// and keyup, and a panic in a listener stops the whole program.
func jsBool(v js.Value) bool {
	return v.Type() == js.TypeBoolean && v.Bool()
}

// jsLength is the length of a JS array-like, or 0 when the value is
// not an object. Value.Length panics on undefined.
func jsLength(v js.Value) int {
	if v.Type() != js.TypeObject {
		return 0
	}
	return jsInt(v.Get("length"))
}
