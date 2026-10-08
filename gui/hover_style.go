package gui

// HoverStyle is a container's hover look that gui paints itself: a
// fill and a cursor, set while the pointer is over the shape (#977).
//
// Use it in place of an OnHover that only sets a fill or a cursor.
// No app code runs, so the look cannot depend on where the pointer is
// inside the shape. A pointer move that stays inside the shape then
// needs no layout rebuild (docs/specs/idle-pointer-moves.md), and the
// container needs no per-frame OnHover closure.
//
// It is not a per-state ColorSet: a container has no pressed, focused
// or selected state of its own. A widget with those states takes a
// Colors field. With an OnHover set too, gui paints the style first
// and then calls OnHover, which sees the hover fill. Disabled shapes
// take no hover look.
type HoverStyle struct {
	// Color is the fill while hovered. Unset keeps the resting fill.
	Color Color
	// Cursor is the mouse cursor while hovered. CursorDefault (the
	// zero value) leaves the cursor alone.
	Cursor MouseCursor
}

// isSet reports whether the style changes anything.
func (h HoverStyle) isSet() bool {
	return h.Color.IsSet() || h.Cursor != CursorDefault
}

// apply paints the style onto a hovered shape. It runs during
// arrange, from layoutHoverDepth.
func (h HoverStyle) apply(s *Shape, w *Window) {
	if h.Color.IsSet() {
		s.Color = h.Color
	}
	if h.Cursor != CursorDefault {
		w.setMouseCursor(h.Cursor)
	}
}
