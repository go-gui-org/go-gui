package gui

// Beep plays the user's configured system alert sound — the platform's
// standard "needs your attention" cue, honoring their system-wide alert
// sound choice, alert volume, and mute settings.
//
// Non-blocking, and safe to call when no native platform is attached
// (tests, headless): it is then a no-op. Intended for incidental
// out-of-band events such as a terminal BEL; it is not a general audio
// API, see the gui/audio package for that.
func (w *Window) Beep() {
	if w.nativePlatform == nil {
		return
	}
	w.nativePlatform.Beep()
}

// BeepAvailable reports whether [Window.Beep] produces an audible sound
// on this platform. False when no native platform is attached, or on
// targets with no system alert sound (mobile, wasm) and on Linux when
// canberra-gtk-play is not installed. Callers that need the user to
// notice the event can use this to decide whether a visual fallback is
// also warranted.
func (w *Window) BeepAvailable() bool {
	if w.nativePlatform == nil {
		return false
	}
	return w.nativePlatform.BeepAvailable()
}
