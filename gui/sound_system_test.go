package gui

import "testing"

// systemSoundSpy is a native platform that implements the optional
// system-sound capability (issue #469).
type systemSoundSpy struct {
	noopNativePlatform
	cues      []SoundCue
	available bool
}

func (s *systemSoundSpy) PlaySystemSound(cue SoundCue) {
	s.cues = append(s.cues, cue)
}

func (s *systemSoundSpy) SystemSoundAvailable() bool { return s.available }

func TestSystemSoundPlayerForwardsEveryCue(t *testing.T) {
	spy := &systemSoundSpy{available: true}
	w := NewWindow(WindowCfg{State: new(struct{})})
	w.SetNativePlatform(spy)
	p := NewSystemSoundPlayer(w)

	// Every cue reaches the platform, including the ones the beep
	// player has no sound for.
	cues := []SoundCue{
		SoundClick, SoundToggleOn, SoundToggleOff, SoundSelection,
		SoundError, SoundNotify, SoundOpen, SoundSuccess,
	}
	for _, cue := range cues {
		p.PlaySound(cue, 1)
	}
	if len(spy.cues) != len(cues) {
		t.Fatalf("forwarded %v, want %v", spy.cues, cues)
	}
	for i, cue := range cues {
		if spy.cues[i] != cue {
			t.Errorf("cue %d = %v, want %v", i, spy.cues[i], cue)
		}
	}
	if !p.SoundAvailable() {
		t.Error("SoundAvailable did not forward the platform result")
	}
}

// Gain is ignored, not scaled and not gated: system event sounds carry
// no app-level volume on any of the three platforms. Muting is handled
// upstream in playSoundCue, which never reaches the player.
func TestSystemSoundPlayerIgnoresGain(t *testing.T) {
	spy := &systemSoundSpy{available: true}
	w := NewWindow(WindowCfg{State: new(struct{})})
	w.SetNativePlatform(spy)
	p := NewSystemSoundPlayer(w)

	p.PlaySound(SoundClick, 0)
	p.PlaySound(SoundClick, 0.25)
	if len(spy.cues) != 2 {
		t.Errorf("forwarded %v, want two clicks regardless of gain", spy.cues)
	}
}

func TestSystemSoundPlayerSilentWithoutCapability(t *testing.T) {
	cases := []struct {
		name     string
		platform NativePlatform
	}{
		// The noop platform does not implement the optional interface,
		// which is what keeps headless tests silent.
		{"noop platform", &noopNativePlatform{}},
		// Neither does a platform that predates the capability.
		{"platform without the capability", &beepSpy{available: true}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			w := NewWindow(WindowCfg{State: new(struct{})})
			w.SetNativePlatform(tc.platform)
			p := NewSystemSoundPlayer(w)
			p.PlaySound(SoundClick, 1)
			if p.SoundAvailable() {
				t.Error("SoundAvailable = true without the capability")
			}
		})
	}
}

func TestSystemSoundPlayerNilSeams(t *testing.T) {
	// No native platform, and no window at all: silent, never a panic.
	w := NewWindow(WindowCfg{State: new(struct{})})
	p := NewSystemSoundPlayer(w)
	p.PlaySound(SoundClick, 1)
	if p.SoundAvailable() {
		t.Error("SoundAvailable = true with no native platform")
	}
	nilPlayer := NewSystemSoundPlayer(nil)
	nilPlayer.PlaySound(SoundClick, 1)
	if nilPlayer.SoundAvailable() {
		t.Error("SoundAvailable = true with a nil window")
	}
}

// The window seam: a cue emitted through the framework reaches the
// system player like any other.
func TestSystemSoundPlayerThroughWindowSeam(t *testing.T) {
	spy := &systemSoundSpy{available: true}
	w := NewWindow(WindowCfg{State: new(struct{})})
	w.SetNativePlatform(spy)
	w.SetSoundPlayer(NewSystemSoundPlayer(w))
	w.PlaySoundCue(SoundNotify)
	if len(spy.cues) != 1 || spy.cues[0] != SoundNotify {
		t.Errorf("cues = %v, want [SoundNotify]", spy.cues)
	}
	// Mute is gated ahead of the player.
	spy.cues = nil
	w.SetSoundVolume(0)
	w.PlaySoundCue(SoundNotify)
	if len(spy.cues) != 0 {
		t.Errorf("muted window emitted %v, want nothing", spy.cues)
	}
}
