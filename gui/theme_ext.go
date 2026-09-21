package gui

import (
	"maps"
	"reflect"
)

// Sibling theme extensions (issue #733).
//
// Theme has no fields for styles that belong to widgets outside
// gui/. Siblings (go-edit, go-charts) kept parallel style structs
// and synced them with the theme by hand. That repeats the
// two-sources problem that issue #300 removed inside gui/. The
// extension slot ends it one layer out: a sibling stores its style
// value in the theme, keyed by the exact value type.
//
// The sibling owns the value type, so gui never imports the
// sibling. Store with WithExt, read with Ext. A missing value
// reads as the zero value and false. That fallback is the locked
// behavior, not a panic.
//
// Stored values must be immutable. Themes copy by value, and the
// copies share the backing map. WithExt clones on write, so a
// derived theme never edits its parent through the shared map. A
// stored value that aliases writable caller memory breaks that
// promise, so derive the value at build time and keep no writable
// alias to it.
//
// Reads during generation use the installed theme (CurrentTheme),
// not w.Theme. Themed scopes the installed theme, so w.Theme
// misses the scope. TestExtFollowsThemedScope pins this.

// WithExt returns t carrying value under the exact type of
// value. The result has a fresh theme id, and the parent keeps
// its own extensions unchanged (clone on write).
//
// exportaudit:keep — sibling extension surface (#733).
func WithExt[T any](t Theme, value T) Theme {
	cloned := make(map[reflect.Type]any, len(t.ext)+1)
	maps.Copy(cloned, t.ext)
	cloned[reflect.TypeFor[T]()] = value
	t.ext = cloned
	t.id = nextThemeID()
	return t
}

// Ext returns the extension value stored under type T, with true.
// No stored value means the zero value of T and false.
//
// exportaudit:keep — sibling extension surface (#733).
func Ext[T any](t Theme) (T, bool) {
	var zero T
	stored, found := t.ext[reflect.TypeFor[T]()]
	if !found {
		return zero, false
	}
	typed, matched := stored.(T)
	if !matched {
		return zero, false
	}
	return typed, true
}
