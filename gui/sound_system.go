package gui

// systemSoundPlatform is an optional NativePlatform capability: the
// platform's own interface sounds, the ones the OS already ships and
// the user has already configured.
//
// Deliberately NOT part of the NativePlatform interface. Adding a
// method there would break every out-of-repo implementation, and a
// platform that predates this capability should stay silent rather
// than fail to compile. NewSystemSoundPlayer type-asserts for it and
// degrades to silence when the assertion fails, which is also what
// makes headless tests and the noop platform silent (issue #469).
type systemSoundPlatform interface {
	// PlaySystemSound plays the platform's sound for cue. A cue the
	// platform does not map is silent, never an error.
	PlaySystemSound(cue SoundCue)
	// SystemSoundAvailable reports whether PlaySystemSound can produce
	// sound on this platform.
	SystemSoundAvailable() bool
}

// systemSoundPlayer maps every cue onto the platform's own event
// sounds, so an app gets native-feeling feedback with no assets and no
// audio library.
type systemSoundPlayer struct {
	w *Window
}

// NewSystemSoundPlayer returns a SoundPlayer backed by the platform's
// system event sounds — NSSound on macOS, PlaySound with a registry
// alias on Windows, freedesktop sound-naming ids on Linux. Unlike
// NewBeepSoundPlayer, which plays the system alert and therefore only
// suits SoundError, it has a sound for every cue.
//
// It ignores gain. System event sounds follow the user's system
// event-sound settings and expose no app-level volume on any of the
// three platforms, so honouring gain on one of them would make the
// same app behave differently per platform. Muting still works:
// (*Window).SetSoundVolume(0) is gated ahead of the player.
//
// Silent, never a panic, on a target with no event sounds and when no
// native platform is attached — the same contract BeepAvailable
// carries. Check SoundAvailable to decide whether a visual fallback is
// warranted.
//
// exportaudit:keep — caller-facing constructor (issue #469)
func NewSystemSoundPlayer(w *Window) SoundPlayer {
	return systemSoundPlayer{w: w}
}

func (p systemSoundPlayer) PlaySound(cue SoundCue, _ float32) {
	sp := p.platform()
	if sp == nil {
		return
	}
	sp.PlaySystemSound(cue)
}

func (p systemSoundPlayer) SoundAvailable() bool {
	sp := p.platform()
	return sp != nil && sp.SystemSoundAvailable()
}

// platform returns the window's native platform if it implements the
// optional capability, else nil.
func (p systemSoundPlayer) platform() systemSoundPlatform {
	if p.w == nil || p.w.nativePlatform == nil {
		return nil
	}
	sp, ok := p.w.nativePlatform.(systemSoundPlatform)
	if !ok {
		return nil
	}
	return sp
}
