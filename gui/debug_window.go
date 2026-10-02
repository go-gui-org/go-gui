package gui

import "fmt"

// Window-level degrade diagnostics.
//
// These do not run from the per-frame audit like the rest of gui/debug.go:
// a backend calls them at window creation or from a window setter, where
// the platform's answer is known. Both report through
// DebugWindowDegraded, and both take reason as the warn-once
// discriminator, so two causes that hold at once are reported separately.
//
// DebugGradientResampled is here for the same reason: a backend's draw
// pass calls it, not the frame audit.

// DebugWindowTransparency reports that WindowCfg.Transparent did not
// take effect, and why. Called by a backend at window creation, where
// the platform answer is known and the app author's only clue would
// otherwise be a window that looks wrong. reason is the warn-once
// discriminator, so each distinct cause is reported once.
// exportaudit:keep — dev-diagnostic API called from gui/backend
func (w *Window) DebugWindowTransparency(reason string) {
	if w == nil {
		return
	}
	w.debugWarn(debugCheckWindowTransparency, reason,
		"window: Transparent requested but %s", reason)
}

// DebugWindowOpacity reports that Window.SetWindowOpacity did not take
// effect, and why. Called by a backend, where the platform answer is
// known and the app author's only clue would otherwise be a window that
// did not fade. reason is the warn-once discriminator, so each distinct
// cause is reported once.
// exportaudit:keep — dev-diagnostic API called from gui/backend
func (w *Window) DebugWindowOpacity(reason string) {
	if w == nil {
		return
	}
	w.debugWarn(debugCheckWindowOpacity, reason,
		"window: opacity requested but %s", reason)
}

// DebugWindowVSync reports that WindowCfg.VSyncOff did not take effect,
// and why. Called by a backend at window creation, where the platform
// answer is known and the app author's only clue would otherwise be a
// benchmark that still reads the display refresh rate. reason is the
// warn-once discriminator, so each distinct cause is reported once.
// exportaudit:keep — dev-diagnostic API called from gui/backend
func (w *Window) DebugWindowVSync(reason string) {
	if w == nil {
		return
	}
	w.debugWarn(debugCheckWindowVSync, reason,
		"window: VSyncOff requested but %s", reason)
}

// DebugGradientResampled reports a fill gradient whose stops exceeded
// the GPU shader uniform limit and were resampled down to it, which
// costs some fidelity even with error-driven placement. Called by the
// GPU backends' draw pass; x, y is the gradient rect origin, used as
// the warn-once discriminator.
func (w *Window) DebugGradientResampled(x, y float32, kept, total int) {
	// NaN never equals itself, so a NaN map key could never match and
	// the warn-once memory would grow a key every frame. Fold NaN in
	// the discriminator only; the message keeps the true position.
	foldX, foldY := x, y
	if foldX != foldX {
		foldX = 0
	}
	if foldY != foldY {
		foldY = 0
	}
	w.debugWarn(debugCheckGradientResampled,
		fmt.Sprintf("gradient %g,%g", foldX, foldY),
		"gradient at (%g, %g) has %d stops; resampled to %d "+
			"(GPU shader uniform limit)", x, y, total, kept)
}
