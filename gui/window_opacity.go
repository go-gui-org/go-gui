package gui

import "math"

// SetWindowOpacity fades the whole window, content included. 1 is fully
// opaque (the default); 0 is invisible. opacity is clamped to [0, 1]; a
// NaN is ignored, since it names no fade at all.
//
// This is a compositor-level fade applied above the GL or Metal surface,
// so it is independent of WindowCfg.Transparent, which controls
// per-pixel alpha instead. The two compose everywhere except Windows,
// where they cannot (see docs/specs/window-opacity.md).
//
// Best-effort: a platform that cannot deliver the fade reports through
// gui.Debug rather than failing. Honored by macOS and X11, and on
// Windows for a window not created Transparent; ignored elsewhere.
//
// Safe to call before the backend attaches — the value is stored and
// applied at window creation. Otherwise call it from the UI thread, or
// from a Window.QueueCommand, like the other window setters.
func (w *Window) SetWindowOpacity(opacity float32) {
	if math.IsNaN(float64(opacity)) {
		return
	}
	w.windowOpacity = clampOpacity(opacity)
	if w.nativePlatform != nil {
		w.nativePlatform.SetWindowOpacity(w.windowOpacity)
	}
}

// WindowOpacity reports the whole-window fade last set by
// SetWindowOpacity, or 1 when none was set. It answers from the cached
// value rather than the platform, so a fade animation can read its own
// current position back without a round trip to the window server.
func (w *Window) WindowOpacity() float32 {
	return w.windowOpacity
}

// clampOpacity holds an opacity inside [0, 1]. The setter filters NaN
// before calling, so only the clamped ends are reachable here; the
// backends replay the cached value, which the setter already clamped.
func clampOpacity(v float32) float32 {
	if v < 0 {
		return 0
	}
	if v > 1 {
		return 1
	}
	return v
}
