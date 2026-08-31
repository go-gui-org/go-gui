package gui

import (
	"math"
	"testing"
)

// Sound cues resolve against the installed theme, and theme
// installation is package-global, so these tests must not run in
// parallel with each other or with anything else that installs a theme.

// soundSpy records every cue the framework emits.
type soundSpy struct {
	cues      []SoundCue
	gains     []float32
	available bool
}

func (s *soundSpy) PlaySound(cue SoundCue, gain float32) {
	s.cues = append(s.cues, cue)
	s.gains = append(s.gains, gain)
}

func (s *soundSpy) SoundAvailable() bool { return s.available }

// soundingTheme returns ThemeDark with every sound role populated —
// what an app gets from theme.Sounds = SoundsDefault().
func soundingTheme(t *testing.T) Theme {
	t.Helper()
	cfg := themeDarkCfg
	cfg.Name = "sounding"
	cfg.Sounds = SoundsDefault()
	return ThemeMaker(cfg)
}

// silentTheme returns ThemeDark unchanged: the zero SoundSet, which is
// the default every built-in theme ships.
func silentTheme(t *testing.T) Theme {
	t.Helper()
	cfg := themeDarkCfg
	cfg.Name = "silent"
	return ThemeMaker(cfg)
}

func buttonView(cfg ButtonCfg) func(*Window) View {
	cfg.ID = "btn"
	cfg.Label = "Go"
	if cfg.OnClick == nil {
		cfg.OnClick = func(ctx EventCtx) { ctx.Consume() }
	}
	return func(*Window) View { return Button(cfg) }
}

func TestSoundThemeDefaultIsSilent(t *testing.T) {
	restoreTheme(t)
	spy := &soundSpy{}
	w := NewTestWindow(WindowCfg{})
	w.SetTheme(silentTheme(t))
	w.SetSoundPlayer(spy)
	w.TestRender(buttonView(ButtonCfg{}))

	if err := w.TestClick("btn"); err != nil {
		t.Fatalf("TestClick: %v", err)
	}
	if len(spy.cues) != 0 {
		t.Errorf("silent theme emitted %v, want nothing", spy.cues)
	}
}

func TestSoundNilPlayerDoesNotPanic(t *testing.T) {
	restoreTheme(t)
	w := NewTestWindow(WindowCfg{})
	w.SetTheme(soundingTheme(t))
	// No SetSoundPlayer: the default, and every headless test.
	w.TestRender(buttonView(ButtonCfg{}))
	if err := w.TestClick("btn"); err != nil {
		t.Fatalf("TestClick: %v", err)
	}
}

// A click must sound on every path that can activate a widget: mouse,
// spacebar, Enter and the accessibility press action.
func TestSoundButtonAllActivationPaths(t *testing.T) {
	restoreTheme(t)
	th := soundingTheme(t)

	activate := map[string]func(*testing.T, *Window){
		"mouse": func(t *testing.T, w *Window) {
			if err := w.TestClick("btn"); err != nil {
				t.Fatalf("TestClick: %v", err)
			}
		},
		"space": func(t *testing.T, w *Window) {
			if err := w.TestType("btn", " "); err != nil {
				t.Fatalf("TestType: %v", err)
			}
		},
		"enter": func(t *testing.T, w *Window) {
			if err := w.TestKey("btn", KeyEnter, ModNone); err != nil {
				t.Fatalf("TestKey: %v", err)
			}
		},
		"a11y_press": func(t *testing.T, w *Window) {
			// Real a11y dispatch: build the node array the action
			// index refers to, then press through it.
			w.a11y.nodes = w.a11y.nodes[:0]
			var live []liveNode
			a11yCollect(&w.layout, -1, &w.a11y.nodes, "", &live)
			idx := -1
			for i := range w.a11y.nodes {
				if w.a11y.nodes[i].Role == AccessRoleButton {
					idx = i
					break
				}
			}
			if idx < 0 {
				t.Fatal("no button node in the a11y tree")
			}
			a11yActionCallback(w, A11yActionPress, idx)
		},
	}

	for name, act := range activate {
		t.Run(name, func(t *testing.T) {
			spy := &soundSpy{}
			w := NewTestWindow(WindowCfg{})
			w.SetTheme(th)
			w.SetSoundPlayer(spy)
			w.TestRender(buttonView(ButtonCfg{}))
			act(t, w)
			if len(spy.cues) != 1 || spy.cues[0] != SoundClick {
				t.Errorf("cues = %v, want [SoundClick]", spy.cues)
			}
		})
	}
}

func TestSoundToggleStateDependentCue(t *testing.T) {
	restoreTheme(t)
	spy := &soundSpy{}
	w := NewTestWindow(WindowCfg{})
	w.SetTheme(soundingTheme(t))
	w.SetSoundPlayer(spy)

	selected := false
	w.TestRender(func(*Window) View {
		return Toggle(ToggleCfg{
			ID:       "tgl",
			Label:    "On",
			Selected: selected,
			OnClick: func(ctx EventCtx) {
				selected = !selected
				ctx.Consume()
			},
		})
	})

	// Off -> on, then on -> off. The cue names what the click does.
	for i := range 2 {
		if err := w.TestClick("tgl"); err != nil {
			t.Fatalf("click %d: %v", i, err)
		}
		w.TestRender(nil)
	}
	want := []SoundCue{SoundToggleOn, SoundToggleOff}
	if len(spy.cues) != 2 ||
		spy.cues[0] != want[0] || spy.cues[1] != want[1] {
		t.Errorf("cues = %v, want %v", spy.cues, want)
	}
}

func TestSoundCfgOverridesTheme(t *testing.T) {
	restoreTheme(t)
	spy := &soundSpy{}
	w := NewTestWindow(WindowCfg{})
	w.SetTheme(soundingTheme(t))
	w.SetSoundPlayer(spy)
	w.TestRender(buttonView(ButtonCfg{Sound: SoundError}))

	if err := w.TestClick("btn"); err != nil {
		t.Fatalf("TestClick: %v", err)
	}
	if len(spy.cues) != 1 || spy.cues[0] != SoundError {
		t.Errorf("cues = %v, want [SoundError]", spy.cues)
	}
}

func TestSoundDisabledSuppresses(t *testing.T) {
	restoreTheme(t)
	spy := &soundSpy{}
	w := NewTestWindow(WindowCfg{})
	w.SetTheme(soundingTheme(t))
	w.SetSoundPlayer(spy)
	// SoundDisabled beats an explicit Sound, not just the theme.
	w.TestRender(buttonView(ButtonCfg{
		Sound: SoundError, SoundDisabled: true,
	}))

	if err := w.TestClick("btn"); err != nil {
		t.Fatalf("TestClick: %v", err)
	}
	if len(spy.cues) != 0 {
		t.Errorf("SoundDisabled emitted %v, want nothing", spy.cues)
	}
}

func TestSoundVolumeClampAndMute(t *testing.T) {
	restoreTheme(t)
	th := soundingTheme(t)

	cases := []struct {
		name     string
		set      bool
		volume   float32
		wantPlay bool
		wantGain float32
	}{
		{name: "default_is_full", wantPlay: true, wantGain: 1},
		{name: "half", set: true, volume: 0.5,
			wantPlay: true, wantGain: 0.5},
		{name: "zero_mutes", set: true, volume: 0},
		{name: "negative_clamps_to_mute", set: true, volume: -3},
		{name: "over_one_clamps", set: true, volume: 7,
			wantPlay: true, wantGain: 1},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			spy := &soundSpy{}
			w := NewTestWindow(WindowCfg{})
			w.SetTheme(th)
			w.SetSoundPlayer(spy)
			if tc.set {
				w.SetSoundVolume(tc.volume)
			}
			w.TestRender(buttonView(ButtonCfg{}))
			if err := w.TestClick("btn"); err != nil {
				t.Fatalf("TestClick: %v", err)
			}
			if !tc.wantPlay {
				if len(spy.cues) != 0 {
					t.Fatalf("emitted %v at volume %v, want nothing",
						spy.cues, tc.volume)
				}
				return
			}
			if len(spy.gains) != 1 {
				t.Fatalf("gains = %v, want one entry", spy.gains)
			}
			if spy.gains[0] != tc.wantGain {
				t.Errorf("gain = %v, want %v", spy.gains[0], tc.wantGain)
			}
			// Whatever reaches the player is already clamped.
			if got := w.SoundVolume(); got < 0 || got > 1 {
				t.Errorf("SoundVolume = %v, out of 0..1", got)
			}
		})
	}
}

func TestSoundPlayerAccessorRoundTrip(t *testing.T) {
	w := NewTestWindow(WindowCfg{})
	if w.SoundPlayer() != nil {
		t.Error("a fresh window has a sound player")
	}
	spy := &soundSpy{}
	w.SetSoundPlayer(spy)
	if w.SoundPlayer() != spy {
		t.Error("SoundPlayer did not return the installed player")
	}
	w.SetSoundPlayer(nil)
	if w.SoundPlayer() != nil {
		t.Error("nil did not clear the player")
	}
}

func TestBeepSoundPlayerOnlyErrors(t *testing.T) {
	spy := &beepSpy{available: true}
	w := NewTestWindow(WindowCfg{})
	w.SetNativePlatform(spy)
	p := NewBeepSoundPlayer(w)

	for _, cue := range []SoundCue{
		SoundNone, SoundClick, SoundToggleOn, SoundToggleOff,
	} {
		p.PlaySound(cue, 1)
	}
	if spy.calls != 0 {
		t.Errorf("non-error cues beeped %d times", spy.calls)
	}
	p.PlaySound(SoundError, 1)
	if spy.calls != 1 {
		t.Errorf("SoundError beeped %d times, want 1", spy.calls)
	}
	if !p.SoundAvailable() {
		t.Error("SoundAvailable did not follow BeepAvailable")
	}
}

func TestBeepSoundPlayerNoNativePlatform(t *testing.T) {
	// Headless: must not panic and must report unavailable.
	w := NewTestWindow(WindowCfg{})
	p := NewBeepSoundPlayer(w)
	p.PlaySound(SoundError, 1)
	if p.SoundAvailable() {
		t.Error("SoundAvailable = true with no native platform")
	}
}

func TestResolveSoundCuePrecedence(t *testing.T) {
	cases := []struct {
		name     string
		themeCue SoundCue
		cfgCue   SoundCue
		disabled bool
		want     SoundCue
	}{
		{name: "theme_only", themeCue: SoundClick, want: SoundClick},
		{name: "cfg_wins", themeCue: SoundClick,
			cfgCue: SoundError, want: SoundError},
		{name: "disabled_wins", themeCue: SoundClick,
			cfgCue: SoundError, disabled: true, want: SoundNone},
		{name: "all_unset", want: SoundNone},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := resolveSoundCue(tc.themeCue, tc.cfgCue, tc.disabled)
			if got != tc.want {
				t.Errorf("resolveSoundCue = %v, want %v", got, tc.want)
			}
		})
	}
}

func TestSoundVolumeNaNAndInfClamped(t *testing.T) {
	restoreTheme(t)
	th := soundingTheme(t)
	for _, tc := range []struct {
		name     string
		volume   float32
		wantGain float32
		wantPlay bool
	}{
		{name: "nan_mutes", volume: float32(math.NaN()), wantPlay: false},
		{name: "pos_inf_clamps_to_full", volume: float32(math.Inf(1)), wantGain: 1, wantPlay: true},
		{name: "neg_inf_mutes", volume: float32(math.Inf(-1)), wantPlay: false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			spy := &soundSpy{}
			w := NewTestWindow(WindowCfg{})
			w.SetTheme(th)
			w.SetSoundPlayer(spy)
			w.SetSoundVolume(tc.volume)
			// Gain stored must be finite and in range.
			got := w.SoundVolume()
			if got != got || got < 0 || got > 1 {
				t.Fatalf("SoundVolume = %v, want finite 0..1", got)
			}
			w.TestRender(buttonView(ButtonCfg{}))
			if err := w.TestClick("btn"); err != nil {
				t.Fatalf("TestClick: %v", err)
			}
			if tc.wantPlay != (len(spy.cues) == 1) {
				t.Fatalf("play = %v, cues = %v", tc.wantPlay, spy.cues)
			}
			if tc.wantPlay && spy.gains[0] != tc.wantGain {
				t.Errorf("gain = %v, want %v", spy.gains[0], tc.wantGain)
			}
		})
	}
}

func TestSoundDisabledWidgetSilent(t *testing.T) {
	restoreTheme(t)
	spy := &soundSpy{}
	w := NewTestWindow(WindowCfg{})
	w.SetTheme(soundingTheme(t))
	w.SetSoundPlayer(spy)
	// A disabled button must not emit even when the theme is sounding.
	w.TestRender(func(*Window) View {
		return Button(ButtonCfg{
			ID: "btn", Label: "Go", Disabled: true,
			OnClick: func(ctx EventCtx) { ctx.Consume() },
		})
	})
	// Mouse path is blocked by traversal (isChildEnabled), so drive
	// the a11y path which bypasses traversal and would previously
	// emit through playShapeSound.
	w.a11y.nodes = w.a11y.nodes[:0]
	var live []liveNode
	a11yCollect(&w.layout, -1, &w.a11y.nodes, "", &live)
	idx := -1
	for i := range w.a11y.nodes {
		if w.a11y.nodes[i].Role == AccessRoleButton {
			idx = i
			break
		}
	}
	if idx < 0 {
		t.Fatal("no button node in the a11y tree")
	}
	a11yActionCallback(w, A11yActionPress, idx)
	if len(spy.cues) != 0 {
		t.Errorf("disabled widget emitted %v, want nothing", spy.cues)
	}
}
