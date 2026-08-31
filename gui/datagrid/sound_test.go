package datagrid

import (
	"testing"

	gg "github.com/go-gui-org/go-gui/gui"
)

// Widget audio feedback phase 2 (issue #467). datagrid sits outside
// gui/, so it cannot read the package-global theme a gui/ factory
// reads; it resolves once per generate through gg.ResolveSoundCue and
// gg.CurrentTheme(). These tests pin that the resolution happens and
// that the two roles land on the right controls.
//
// Theme installation is package-global, so nothing here runs in
// parallel.

// gridSoundSpy records every cue the framework emits.
type gridSoundSpy struct {
	cues []gg.SoundCue
}

func (s *gridSoundSpy) PlaySound(cue gg.SoundCue, _ float32) {
	s.cues = append(s.cues, cue)
}

func (s *gridSoundSpy) SoundAvailable() bool { return true }

// soundingGridTheme is the current theme with every cue role filled —
// what an app gets from Theme.Sounds = gg.SoundsDefault().
func soundingGridTheme(w *gg.Window) gg.Theme {
	cfg := w.Theme().Cfg
	cfg.Name = "datagrid-sounding"
	cfg.Sounds = gg.SoundsDefault()
	return gg.ThemeMaker(cfg)
}

// soundGrid renders a one-column, one-row grid with the sounding theme
// installed and a spy attached.
func soundGrid(t *testing.T, cfg DataGridCfg) (*gg.Window, *gridSoundSpy) {
	t.Helper()
	w := gg.NewTestWindow(gg.WindowCfg{})
	t.Cleanup(w.Close)
	w.SetTheme(soundingGridTheme(w))
	spy := &gridSoundSpy{}
	w.SetSoundPlayer(spy)
	w.TestRender(func(*gg.Window) gg.View { return New(w, cfg) })
	return w, spy
}

func soundGridCfg() DataGridCfg {
	return DataGridCfg{
		ID: "g1",
		Columns: []GridColumnCfg{{
			ID: "c1", Title: "Col1", Sortable: true,
			Width: gg.SomeF(120),
		}},
		Rows:          []GridRow{{ID: "r1", Cells: map[string]string{"c1": "a"}}},
		OnQueryChange: func(GridQueryState, gg.EventCtx) {},
	}
}

// Sorting a column picks one of its orders, so it takes the selection
// role; activating a row is the grid's click role.
func TestGridSoundHeaderAndRowCues(t *testing.T) {
	cases := []struct {
		name string
		id   string
		want gg.SoundCue
	}{
		{name: "header_sort", id: "g1:header:c1", want: gg.SoundSelection},
		{name: "row_activate", id: "g1:row:r1", want: gg.SoundClick},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			w, spy := soundGrid(t, soundGridCfg())
			if err := w.TestClick(tc.id); err != nil {
				t.Fatalf("TestClick(%q): %v", tc.id, err)
			}
			if len(spy.cues) != 1 || spy.cues[0] != tc.want {
				t.Errorf("cues = %v, want [%v]", spy.cues, tc.want)
			}
		})
	}
}

// Cfg.Sound overrides both roles at once: the grid resolves one cue
// set for every control it builds.
func TestGridSoundCfgOverridesTheme(t *testing.T) {
	cfg := soundGridCfg()
	cfg.Sound = gg.SoundError
	w, spy := soundGrid(t, cfg)

	if err := w.TestClick("g1:header:c1"); err != nil {
		t.Fatalf("TestClick: %v", err)
	}
	if len(spy.cues) != 1 || spy.cues[0] != gg.SoundError {
		t.Errorf("cues = %v, want [SoundError]", spy.cues)
	}
}

// SoundDisabled beats an explicit Sound, and it has to reach the
// gg.ButtonCfg controls too — a resolved gg.SoundNone reads as "unset"
// inside ButtonCfg, so those sites pass SoundDisabled as well.
func TestGridSoundDisabledSuppresses(t *testing.T) {
	cfg := soundGridCfg()
	cfg.Sound = gg.SoundError
	cfg.SoundDisabled = true
	w, spy := soundGrid(t, cfg)

	for _, id := range []string{"g1:header:c1", "g1:row:r1"} {
		if err := w.TestClick(id); err != nil {
			t.Fatalf("TestClick(%q): %v", id, err)
		}
	}
	if len(spy.cues) != 0 {
		t.Errorf("SoundDisabled emitted %v, want nothing", spy.cues)
	}
}
