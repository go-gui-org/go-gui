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

// The buttons change the number of frozen columns and stop at both ends.
func TestButtonsChangeFreezeColumns(t *testing.T) {
	t.Parallel()

	app := newApp()
	w := gui.NewTestWindow(t, gui.WindowCfg{State: app, Width: 720, Height: 480})
	for _, c := range []struct {
		id   string
		want int
	}{{"freeze-more", maxFreeze}, {"freeze-less", 0}} {
		for range maxFreeze + 2 {
			w.TestRender(mainView)
			if err := w.TestClick(c.id); err != nil {
				t.Fatalf("TestClick %s: %v", c.id, err)
			}
		}
		if app.FreezeColumns != c.want {
			t.Fatalf("after %s: FreezeColumns = %d, want %d", c.id, app.FreezeColumns, c.want)
		}
	}
}

// The toggles switch the two freeze flags, and no setting of them leaves
// a layout finding.
func TestTogglesSwitchFreezeFlags(t *testing.T) {
	t.Parallel()

	app := newApp()
	w := gui.NewTestWindow(t, gui.WindowCfg{State: app, Width: 720, Height: 480})
	for _, id := range []string{"freeze-row-header", "freeze-header", "freeze-row-header", "freeze-header"} {
		w.TestRender(mainView)
		before := [2]bool{app.FreezeRowHeader, app.FreezeHeader}
		if err := w.TestClick(id); err != nil {
			t.Fatalf("TestClick %s: %v", id, err)
		}
		if before == [2]bool{app.FreezeRowHeader, app.FreezeHeader} {
			t.Fatalf("a click on %s switched nothing", id)
		}
		w.TestRender(mainView)
		if found := w.TestFindings(gui.DebugAll); len(found) != 0 {
			t.Fatalf("after %s: findings: %v", id, found)
		}
	}
}
