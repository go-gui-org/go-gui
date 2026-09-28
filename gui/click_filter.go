package gui

// clickFilter limits which mouse button fires a shape's OnClick. The
// zero value, clickAnyButton, accepts every button. It is a separate
// type and not a MouseButton because MouseLeft is 0: a MouseButton
// field cannot tell "left only" from "not set" (#838).
type clickFilter uint8

// clickFilter values.
const (
	clickAnyButton clickFilter = iota
	clickLeftOnly
	clickRightOnly
)

// accepts reports whether a press of b may fire OnClick.
func (f clickFilter) accepts(b MouseButton) bool {
	switch f {
	case clickLeftOnly:
		return b == MouseLeft
	case clickRightOnly:
		return b == MouseRight
	}
	return true
}

// button returns a mouse button the filter accepts. The debug sweep
// uses it to build a synthetic press that reaches OnClick.
func (f clickFilter) button() MouseButton {
	if f == clickRightOnly {
		return MouseRight
	}
	return MouseLeft
}
