package main

import (
	"testing"

	"github.com/go-gui-org/go-gui/gui"
)

// A test window has no backend, so the OS reports no setting: the window
// follows, but keeps its theme until a setting arrives.

// option is the effective ID of a theme radio option, by position.
func option(i int) string { return gui.ScopeIDN("theme", "opt", i) }

func TestFollowsByDefault(t *testing.T) {
	w := gui.NewTestWindow(t, windowCfg())
	w.TestRender(nil)
	if app := gui.State[App](w); app.Mode != modeSystem || app.HasDesktop {
		t.Fatalf("Mode %q, HasDesktop %v", app.Mode, app.HasDesktop)
	}
}

func TestPinAndFollowAgain(t *testing.T) {
	w := gui.NewTestWindow(t, windowCfg())
	w.TestRender(nil)
	for _, c := range []struct {
		opt  int
		mode string
		dark bool
	}{{1, modeLight, false}, {2, modeDark, true}} {
		if err := w.TestClick(option(c.opt)); err != nil {
			t.Fatal(err)
		}
		if app := gui.State[App](w); app.Mode != c.mode {
			t.Fatalf("option %d: Mode %q, want %q", c.opt, app.Mode, c.mode)
		}
		if got := w.Theme().TitlebarDark; got != c.dark {
			t.Fatalf("option %d: theme TitlebarDark %v, want %v", c.opt, got, c.dark)
		}
	}

	// Following again keeps the pinned theme while the OS reports no
	// setting.
	if err := w.TestClick(option(0)); err != nil {
		t.Fatal(err)
	}
	if app := gui.State[App](w); app.Mode != modeSystem {
		t.Fatalf("Mode %q, want %q", app.Mode, modeSystem)
	}
	if !w.Theme().TitlebarDark {
		t.Fatal("following with no OS setting changed the theme")
	}
}

func TestScreenshotShowsDarkDesktop(t *testing.T) {
	w := gui.NewTestWindow(t, screenshotCfg())
	w.TestRender(nil)
	if app := gui.State[App](w); !app.HasDesktop || desktopText(app) != "dark" {
		t.Fatalf("HasDesktop %v, text %q", app.HasDesktop, desktopText(app))
	}
}
