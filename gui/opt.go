package gui

// Opt is an optional value. The zero value means "not set";
// use Some(v) to set explicitly.
type Opt[T any] struct {
	val T
	set bool
}

// Some returns an Opt with v explicitly set.
func Some[T any](v T) Opt[T] { return Opt[T]{val: v, set: true} }

// Get returns val if set, otherwise def.
func (o Opt[T]) Get(def T) T {
	if o.set {
		return o.val
	}
	return def
}

// IsSet reports whether a value was explicitly set.
func (o Opt[T]) IsSet() bool { return o.set }

// Value returns the value and whether it was set.
func (o Opt[T]) Value() (T, bool) { return o.val, o.set }

// SomeF is shorthand for Some(float32(v)).
func SomeF(v float32) Opt[float32] { return Opt[float32]{val: v, set: true} }

// Named zero-override constants for common Opt[float32] fields.
var (
	NoBorder  = SomeF(0)
	NoSpacing = SomeF(0)
	NoRadius  = SomeF(0)
)

// NoPadding is shorthand for PaddingNone, the explicitly-set zero
// padding. It exists so call sites read "no padding" instead of
// reasoning about the flag. There is no SomeP any more: Padding
// self-flags (gui/padding.go), so an explicit padding is built with
// NewPadding / PadAll / PaddingNone, not wrapped in Opt.
var NoPadding = PaddingNone
