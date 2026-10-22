//go:build js && wasm

package web

import (
	"math"
	"syscall/js"

	"github.com/go-gui-org/go-gui/gui"
)

// Safe readers for fields of DOM events. Each returns the zero value
// where the matching js.Value method would panic or coerce.

// jsFloat reads a JS number as a float64, or 0 when the value is
// missing, not a number, NaN, or outside the float32 range (±Inf
// included). Value.Float panics on a non-number. NaN and ±Inf are
// numbers in JS, and a script can put one in an event field with
// Object.defineProperty; read as is, it would go into MouseX or
// ScrollY and from there into hit tests and scroll offsets (#1131).
// Every caller casts to float32, so a finite 1e300 is rejected too: the
// cast would make it +Inf. The NaN test is !(|f| <= max), which is
// true for NaN.
func jsFloat(v js.Value) float64 {
	if v.Type() != js.TypeNumber {
		return 0
	}
	f := v.Float()
	if !(math.Abs(f) <= math.MaxFloat32) {
		return 0
	}
	return f
}

// jsInt reads a JS number as an int, or 0 when the value is missing,
// not a number, NaN, or outside the int32 range. Value.Int panics on a
// non-number, and is int(v.Float()): a NaN or ±Inf keyCode, deltaMode
// or identifier would become a saturated int (#1131). Real values
// (key codes, modes, touch identifiers, list lengths) fit in int32.
func jsInt(v js.Value) int {
	if v.Type() != js.TypeNumber {
		return 0
	}
	f := v.Float()
	if !(f >= math.MinInt32 && f <= math.MaxInt32) {
		return 0
	}
	return int(f)
}

// jsString reads a JS string property without coercion: a missing
// value (null or undefined) yields "" rather than the literals
// "null" or "undefined" that Value.String would produce, which would
// otherwise arrive as a bogus one-word composition.
func jsString(v js.Value) string {
	if v.Type() != js.TypeString {
		return ""
	}
	return v.String()
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

// appearanceFromEvent reads the appearance from the arguments of a
// prefers-color-scheme change listener. jsBool, because an event built
// with new Event("change") has no matches field and Value.Bool panics.
func appearanceFromEvent(args []js.Value) gui.Appearance {
	if len(args) > 0 && jsBool(args[0].Get("matches")) {
		return gui.AppearanceDark
	}
	return gui.AppearanceLight
}
