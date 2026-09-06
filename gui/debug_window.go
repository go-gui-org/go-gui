package gui

// Window-level degrade diagnostics.
//
// These do not run from the per-frame audit like the rest of gui/debug.go:
// a backend calls them at window creation or from a window setter, where
// the platform's answer is known. Both report through
// DebugWindowDegraded, and both take reason as the warn-once
// discriminator, so two causes that hold at once are reported separately.

// DebugWindowTransparency reports that WindowCfg.Transparent did not
// take effect, and why. Called by a backend at window creation, where
// the platform answer is known and the app author's only clue would
// otherwise be a window that looks wrong. reason is the warn-once
// discriminator, so each distinct cause is reported once.
// exportaudit:keep — dev-diagnostic API called from gui/backend
func (w *Window) DebugWindowTransparency(reason string) {
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
	w.debugWarn(debugCheckWindowOpacity, reason,
		"window: opacity requested but %s", reason)
}
