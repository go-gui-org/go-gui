package gui

// Headless rendering is the mode a software rasterizer (gui/backend/soft)
// renders a frame in: no GPU, no window, no animation goroutine. The flag
// exists so wall-clock-driven visuals have one place to consult, making
// frame output a function of window state alone.
//
// It is deliberately not a general "no animation" switch: animations that
// are driven by explicit state still run.

// SetHeadlessRender marks this window as rendering headlessly, which
// suppresses visuals whose appearance depends on wall-clock time — today
// the blinking text caret. Set it before the frame that is captured, so
// repeated captures of the same window state produce the same pixels.
//
// gui/backend/soft sets this for every window it renders; call it directly
// only when driving the frame pipeline yourself.
func (w *Window) SetHeadlessRender(on bool) {
	w.headlessRender = on
}

// HeadlessRender reports whether this window renders headlessly.
// exportaudit:keep — paired reader for SetHeadlessRender
func (w *Window) HeadlessRender() bool {
	return w.headlessRender
}
