package gui

// WindowDecoration selects the native window frame drawn around a window.
// Set it once through WindowCfg.Decorations; the frame style is fixed at
// creation and cannot change afterwards.
type WindowDecoration uint8

// WindowDecoration constants. The zero value keeps the platform's
// standard frame, so an unset field never changes today's behavior.
const (
	// DecorationDefault keeps the standard title bar and border.
	DecorationDefault WindowDecoration = iota
	// DecorationHiddenTitlebar hides the title bar text and background
	// while leaving the window controls floating over the content, so
	// the app draws its own header underneath them. macOS only; other
	// platforms fall back to DecorationDefault.
	DecorationHiddenTitlebar
	// DecorationNone removes the frame entirely: no title bar, no
	// border, no window controls. The window then has nothing the user
	// can grab, so pair it with Window.StartWindowDrag (and, on
	// Windows and X11, Window.StartWindowResize) to keep it movable.
	DecorationNone
)

// WindowEdge names the edge or corner a client-driven resize pulls.
// Values match the _NET_WM_MOVERESIZE direction codes so the X11
// backend can pass them straight through.
type WindowEdge uint8

// WindowEdge constants, ordered clockwise from the top-left corner.
const (
	EdgeTopLeft WindowEdge = iota
	EdgeTop
	EdgeTopRight
	EdgeRight
	EdgeBottomRight
	EdgeBottom
	EdgeBottomLeft
	EdgeLeft
)

// StartWindowDrag hands the window over to the OS window-move loop,
// which then owns the pointer until the user releases the button.
// Call it from an OnMouseDown handler on the shape that acts as the
// title bar of a DecorationNone window.
//
// No-op when no native platform is set (tests) or on backends without
// a window manager (web, iOS, Android).
func (w *Window) StartWindowDrag() {
	if w.nativePlatform != nil {
		w.nativePlatform.StartWindowDrag()
	}
}

// StartWindowResize hands the window over to the OS resize loop for the
// given edge or corner. Call it from an OnMouseDown handler on a resize
// grip drawn by the app.
//
// macOS needs no grip: a borderless window keeps AppKit's own edge
// resize, so this is a no-op there. Also a no-op when no native
// platform is set, and on web, iOS and Android.
func (w *Window) StartWindowResize(edge WindowEdge) {
	if w.nativePlatform != nil {
		w.nativePlatform.StartWindowResize(edge)
	}
}
