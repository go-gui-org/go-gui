package main

import (
	"os"
	"testing"

	"github.com/go-gui-org/go-gui/gui"
)

// TestMain sets the theme once, before any test runs. SetTheme writes
// package-level style state that rendering reads, so calling it inside
// the parallel tests races with the other test's frames.
func TestMain(m *testing.M) {
	gui.SetTheme(gui.ThemeDark)
	os.Exit(m.Run())
}

// The buttons move the app-owned current row, and stop one step past
// each end so the "no marker" case is reachable but bounded.
func TestMarkerButtonsMoveCurrentRow(t *testing.T) {
	t.Parallel()

	app := &App{Current: 1, Marker: ">", Selected: map[int]bool{}}
	w := gui.NewTestWindow(t, gui.WindowCfg{State: app, Width: 520, Height: 420})
	w.TestRender(mainView)

	if err := w.TestClick("marker-down"); err != nil {
		t.Fatalf("TestClick down: %v", err)
	}
	if app.Current != 2 {
		t.Fatalf("after down: Current = %d, want 2", app.Current)
	}
	for range 10 {
		w.TestRender(mainView)
		if err := w.TestClick("marker-down"); err != nil {
			t.Fatalf("TestClick down: %v", err)
		}
	}
	if want := len(tableData); app.Current != want {
		t.Fatalf("past bottom: Current = %d, want %d", app.Current, want)
	}
	for range 20 {
		w.TestRender(mainView)
		if err := w.TestClick("marker-up"); err != nil {
			t.Fatalf("TestClick up: %v", err)
		}
	}
	if app.Current != -1 {
		t.Fatalf("past top: Current = %d, want -1", app.Current)
	}
}

// Each radio option sets the marker glyph the table shows.
func TestMarkerRadioSetsGlyph(t *testing.T) {
	t.Parallel()

	app := &App{Current: 1, Marker: ">", Selected: map[int]bool{}}
	w := gui.NewTestWindow(t, gui.WindowCfg{State: app, Width: 520, Height: 420})
	for i, opt := range markerOptions {
		w.TestRender(mainView)
		id := gui.ScopeIDN("marker-choice", "opt", i)
		if err := w.TestClick(id); err != nil {
			t.Fatalf("TestClick %s: %v", id, err)
		}
		if app.Marker != opt.Value {
			t.Fatalf("after option %d: Marker = %q, want %q",
				i, app.Marker, opt.Value)
		}
	}
}
