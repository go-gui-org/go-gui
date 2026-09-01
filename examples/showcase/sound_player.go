//go:build !js && !android && !ios

package main

import (
	"github.com/go-gui-org/go-gui/gui"
	"github.com/go-gui-org/go-gui/gui/audio"
)

// This file is the whole of what an app writes to give its widgets
// sound. It is also the source the "Sound Feedback" guide quotes, so
// keep it self-contained and readable — the guide is only correct for
// as long as this file compiles.

// doc:snippet-begin player
//
// The block below is quoted verbatim by docs/widget-sound.md and by
// examples/showcase/docs/widget_sound.md. TestSoundGuideSnippetMatchesSource
// fails when the two drift, so keep this region self-contained: anything a
// reader would have to go looking for belongs above the marker, not inside
// it.

// Cue envelope: percussive, one-shot, so each cue frees its own mixer
// channel. Short enough that fast clicking does not smear.
var cueEnv = voiceEnv{
	attackS: 0.008,
	decayS:  0.060,
	sustain: 0,
	oneShot: true,
}

// errorEnv is longer and lets the tone ring, so a rejection reads as
// different in kind rather than only in pitch.
var errorEnv = voiceEnv{
	attackS: 0.008,
	decayS:  0.220,
	sustain: 0,
	oneShot: true,
}

// cuePeakLevel is the voice amplitude before the window's gain. Well
// below the synth pads: a UI cue rides under whatever else is playing.
const cuePeakLevel = 0.22

// cueSoundPlayer maps gui.SoundCue onto synthesized blips. Nothing is
// loaded from disk — the showcase ships no sound assets — so the same
// code runs on every desktop platform with no licensing or size cost.
// An app with its own WAVs swaps the switch below for
// audio.LoadSoundBytes; see docs/widget-sound.md.
type cueSoundPlayer struct {
	w *gui.Window
}

var _ gui.SoundPlayer = cueSoundPlayer{}

// PlaySound implements gui.SoundPlayer. It runs on the event-dispatch
// goroutine, so it starts a voice and returns; the mixer does the work
// on the audio thread.
func (p cueSoundPlayer) PlaySound(cue gui.SoundCue, gain float32) {
	var freq float64
	env := cueEnv
	switch cue {
	case gui.SoundClick:
		freq = 880 // A5
	case gui.SoundToggleOn:
		freq = 880 * 4 / 3 // a fourth up
	case gui.SoundToggleOff:
		freq = 880 * 3 / 4 // a fourth down
	case gui.SoundSelection:
		freq = 880 * 3 / 2 // a fifth up: "picked", against click's neutral A5
	case gui.SoundError:
		freq = 220
		env = errorEnv
	case gui.SoundNotify:
		freq = 880 * 5 / 6 // a whole tone below A5: soft, unasked for
	case gui.SoundOpen:
		freq = 880 / 2 // an octave down: something larger arrived
	case gui.SoundSuccess:
		freq = 880 * 2 // an octave up: the brightest cue in the set
	default:
		// An unrecognised cue is ignored, not an error: gui adds cues
		// over time and a player must not break when it does.
		return
	}
	if !ensureAudioInit(p.w) {
		return
	}
	// The window's volume arrives as gain; scale the voice rather than
	// touching audio.SetMasterVolume, which the music demo owns.
	v := newVoice(freq, cuePeakLevel*float64(gain), env)
	// A failed start is silent on purpose: a click must never surface a
	// mixer error to the user.
	_ = audio.PlaySource(-1, v)
}

// SoundAvailable implements gui.SoundPlayer. gui/audio builds on every
// desktop target this file compiles for.
func (p cueSoundPlayer) SoundAvailable() bool { return true }

// doc:snippet-end player

// soundPlayerKind picks which of the three players the showcase
// installs. All three render the same cues; they differ in what a cue
// sounds like and in what they cost to ship.
type soundPlayerKind uint8

const (
	// soundPlayerSynth is the synthesized player above: every cue is a
	// blip, no assets, gain honoured.
	soundPlayerSynth soundPlayerKind = iota
	// soundPlayerBeep is the zero-dependency path from the guide: the
	// system alert on SoundError and silence otherwise, with no audio
	// library. What an app wants when it only needs to signal a
	// rejection.
	soundPlayerBeep
	// soundPlayerSystem is the platform's own event sounds: a cue for
	// every role, no assets, no audio library, gain ignored.
	soundPlayerSystem
)

// installWidgetSounds turns widget sound on for the window: a theme
// that names a cue per role, and a player that renders them. Both are
// required — either alone is silent.
func installWidgetSounds(w *gui.Window, kind soundPlayerKind) {
	// Rebuild through ThemeMaker rather than mutating the theme in
	// place: a Theme carries an id, and installTheme skips a theme
	// whose id is already installed, so a mutated copy can silently
	// lose the change in a second window. ThemeMaker stamps a fresh id.
	cfg := w.Theme().Cfg
	cfg.Sounds = gui.SoundsDefault()
	w.SetTheme(gui.ThemeMaker(cfg))

	var p gui.SoundPlayer = cueSoundPlayer{w: w}
	switch kind {
	case soundPlayerBeep:
		p = gui.NewBeepSoundPlayer(w)
	case soundPlayerSystem:
		p = gui.NewSystemSoundPlayer(w)
	}
	w.SetSoundPlayer(p)
}

// removeWidgetSounds silences the window again, leaving the theme's
// cue set in place so the toggle is symmetric.
func removeWidgetSounds(w *gui.Window) {
	w.SetSoundPlayer(nil)
}
