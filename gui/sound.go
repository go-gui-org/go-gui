package gui

import "math"

// SoundCue names a semantic user-interface event. The framework emits
// cues; the injected SoundPlayer decides what each one sounds like. A
// cue is not a sample path, so a theme can stay silent, an app can map
// the same cue to different assets, and gui/ never gains an audio
// dependency.
//
// The zero value is SoundNone, so an unset field on a Cfg or on a
// SoundSet is silent and no separate "set" flag is needed — the same
// reasoning as ColorSet (see gui/color_set.go).
//
// The list grows over time. A player that does not recognise a cue must
// ignore it; never write an exhaustive switch over this type.
type SoundCue uint8

// SoundCue values.
const (
	// SoundNone is the zero value: no sound. Exported because it is
	// the name of "silent" in the cue vocabulary: an app's player
	// compares against it, and a Cfg is documented as taking it.
	// exportaudit:keep — zero-value sentinel of a public enum.
	SoundNone SoundCue = iota
	// SoundClick marks a momentary activation — a button press, a menu
	// item, a link.
	SoundClick
	// SoundToggleOn marks a state that went off -> on.
	SoundToggleOn
	// SoundToggleOff marks a state that went on -> off.
	SoundToggleOff
	// SoundError marks a rejection: invalid input, a refused commit, a
	// failed action.
	SoundError
)

// SoundPlayer renders semantic UI cues as audible feedback. The
// backend installs none — an app opts in with
// (*Window).SetSoundPlayer. Nil in tests and by default, which is why
// every call site nil-guards and silence is the default behaviour.
//
// Implementations live outside gui/ (see examples/showcase) so that
// gui/ never imports gui/audio.
type SoundPlayer interface {
	// PlaySound renders cue at gain (0..1, already clamped by the
	// window). Called on the event-dispatch goroutine, so it must be
	// fire-and-forget: no blocking, no allocation on an audio thread.
	// An unrecognised cue must be ignored, not reported as an error.
	PlaySound(cue SoundCue, gain float32)
	// SoundAvailable reports whether PlaySound can actually produce
	// sound on this platform. Callers that need the user to notice an
	// event use this to decide whether a visual fallback is warranted.
	SoundAvailable() bool
}

// SoundSet maps the interaction roles the widget set knows about onto
// cues. It lives on Theme; the zero set is silent, which is the
// intended default, so ThemeDark and ThemeLight make no sound.
//
// It is a struct rather than a fixed array so that new roles can be
// added without breaking apps that build one by field name.
//
// exportaudit:keep — the type of Theme.Sounds and the return of
// SoundsDefault; an app naming its own cue map needs the name.
type SoundSet struct {
	// Click is the cue for a momentary activation.
	// exportaudit:keep — caller-facing config (issue #446)
	Click SoundCue
	// ToggleOn is the cue for a state going off -> on.
	// exportaudit:keep — caller-facing config (issue #446)
	ToggleOn SoundCue
	// ToggleOff is the cue for a state going on -> off.
	// exportaudit:keep — caller-facing config (issue #446)
	ToggleOff SoundCue
	// Error is the cue for a rejection.
	// exportaudit:keep — caller-facing config (issue #446)
	Error SoundCue
}

// SoundsDefault returns the natural cue for every role — the one-liner
// an app assigns to Theme.Sounds to opt the whole widget set in.
func SoundsDefault() SoundSet {
	return SoundSet{
		Click:     SoundClick,
		ToggleOn:  SoundToggleOn,
		ToggleOff: SoundToggleOff,
		Error:     SoundError,
	}
}

// resolveSoundCue applies the precedence a widget Cfg promises:
// SoundDisabled beats an explicit Cfg.Sound, which beats the theme's
// cue for that role. Called at generation time, where the widget
// already knows its own state, so a state-dependent cue (toggle on vs
// off) costs no runtime branch and no allocation.
func resolveSoundCue(themeCue, cfgCue SoundCue, disabled bool) SoundCue {
	if disabled {
		return SoundNone
	}
	if cfgCue != SoundNone {
		return cfgCue
	}
	return themeCue
}

// SetSoundPlayer installs the window's sound player. Nil disables
// widget sound without changing any theme or Cfg.
func (w *Window) SetSoundPlayer(p SoundPlayer) {
	w.soundPlayer = p
}

// SoundPlayer returns the window's sound player, or nil if none has
// been set (the default, and every headless test).
func (w *Window) SoundPlayer() SoundPlayer {
	return w.soundPlayer
}

// SetSoundVolume sets the gain passed to the sound player, clamped to
// 0..1. There is no separate mute flag: 0 is mute, and the gate is one
// comparison ahead of cue resolution, so a muted window costs nothing
// per click.
//
// The value is implicitly relative to the system output level, which
// the OS mixer applies downstream — an app cannot portably read that
// level and must not try.
func (w *Window) SetSoundVolume(v float32) {
	// NaN and Inf are unusable as gain; treat them as mute or
	// full, not as a propagated NaN that silences the mixer with a
	// NaN level downstream.
	if math.IsNaN(float64(v)) {
		v = 0
	} else if math.IsInf(float64(v), 1) {
		v = 1
	} else if math.IsInf(float64(v), -1) {
		v = 0
	}
	switch {
	case v < 0:
		v = 0
	case v > 1:
		v = 1
	}
	w.soundVolume = v
	w.soundVolumeSet = true
}

// SoundVolume returns the window's sound gain, 0..1. Defaults to 1.
func (w *Window) SoundVolume() float32 {
	if !w.soundVolumeSet {
		return 1
	}
	return w.soundVolume
}

// playShapeSound emits the shape's resolved cue. Nil-safe on every
// seam: no layout, no shape, no events record, no cue, no player, or a
// muted window, and it is a no-op.
//
// Called before OnClick and independently of ctx.Consume(): the cue
// confirms that the widget was activated, it does not participate in
// event propagation.
func playShapeSound(l *Layout, w *Window) {
	if l == nil || l.Shape == nil || l.Shape.events == nil || w == nil {
		return
	}
	if l.Shape.Disabled {
		return
	}
	cue := l.Shape.events.soundCue
	if cue == SoundNone {
		return
	}
	p := w.soundPlayer
	if p == nil {
		return
	}
	gain := w.SoundVolume()
	if gain <= 0 || math.IsNaN(float64(gain)) || math.IsInf(float64(gain), 0) {
		return
	}
	p.PlaySound(cue, gain)
}

// beepSoundPlayer is the zero-dependency default: it maps SoundError
// to the system alert and every other cue to silence.
type beepSoundPlayer struct {
	w *Window
}

// NewBeepSoundPlayer returns a SoundPlayer backed by the platform's
// system alert sound (Window.Beep). It plays only SoundError, because
// an alert is the wrong sound for a click, and it needs no assets and
// no audio library.
//
// It ignores gain: NSBeep and MessageBeep honour the system alert
// volume and expose no app-level gain. An app that needs volume control
// needs a sampled player — see gui/audio and examples/showcase.
func NewBeepSoundPlayer(w *Window) SoundPlayer {
	return beepSoundPlayer{w: w}
}

func (b beepSoundPlayer) PlaySound(cue SoundCue, _ float32) {
	if cue != SoundError || b.w == nil {
		return
	}
	b.w.Beep()
}

func (b beepSoundPlayer) SoundAvailable() bool {
	return b.w != nil && b.w.BeepAvailable()
}
