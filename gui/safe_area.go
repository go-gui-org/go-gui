package gui

// safe_area.go — the space the OS keeps along the window's edges
// (issue #1171). Backends report it; an app reads it if it wants to.
// Nothing in the framework lays itself out by it, so an app that never
// reads it draws as before.

// SafeAreaPadding returns the space the OS keeps for itself along each
// edge of the window, in logical pixels: a notch or camera cutout, a
// status bar, a home indicator, a gesture bar. It is all zeros on
// desktop, and wherever the platform does not report it.
//
// The result is a set Padding, so it fits a Cfg directly:
//
//	Padding: w.SafeAreaPadding(),
//
// While the soft keyboard is up it covers the bottom edge, home bar
// included, so the bottom value shrinks by SoftKeyboardInset and never
// goes below 0. An app that pads for both the safe area and the
// keyboard counts the overlap once.
//
// The window re-renders when the safe area changes. Read it in the View
// function.
func (w *Window) SafeAreaPadding() Padding {
	p := w.safeArea.Load()
	if p == nil {
		return PaddingNone
	}
	return NewPadding(p.Top, p.Right,
		max(0, p.Bottom-w.softKeyboardInset), p.Left)
}

// DispatchSafeArea reports the space the OS now keeps along each edge
// of the window, in logical pixels, for SafeAreaPadding. A negative,
// NaN or infinite edge becomes 0. On a change the window asks for a
// full frame; a repeat of the same values does nothing.
//
// The bottom edge is the OS's own, without the soft keyboard: report
// the keyboard through EventSoftKeyboard.
//
// Intended for backend use. Safe for a nil window (no-op).
func DispatchSafeArea(w *Window, top, right, bottom, left float32) {
	if w == nil {
		return
	}
	p := NewPadding(safeAreaEdge(top), safeAreaEdge(right),
		safeAreaEdge(bottom), safeAreaEdge(left))
	cur := PaddingNone // what an unreported window answers
	if old := w.safeArea.Load(); old != nil {
		cur = *old
	}
	if p == cur {
		return
	}
	w.safeArea.Store(&p)
	w.markLayoutRefresh(refreshSafeArea)
	w.wakeMain()
}

// safeAreaEdge clamps one reported edge: the value comes from the OS.
func safeAreaEdge(v float32) float32 {
	if !f32IsFinite(v) || v < 0 {
		return 0
	}
	return v
}
