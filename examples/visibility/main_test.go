package main

import (
	"strings"
	"testing"
	"time"

	"github.com/go-gui-org/go-gui/gui"
)

func newTestWindow() *gui.Window {
	gui.SetTheme(gui.ThemeDark)
	w := gui.NewWindow(gui.WindowCfg{State: &App{}, Width: 560, Height: 420})
	w.OnEvent = onEvent
	return w
}

func TestMainViewNoPanic(t *testing.T) {
	w := newTestWindow()
	_ = mainView(w).GenerateLayout(w)
}

// A minimize and the restore each reach the handler once, and the log
// says which way the window went.
func TestOcclusionIsLogged(t *testing.T) {
	w := newTestWindow()
	app := gui.State[App](w)

	gui.DispatchWindowOccluded(w, true)
	gui.DispatchWindowOccluded(w, true) // a repeat reports nothing
	if app.Changes != 1 || !strings.Contains(app.Log[0], "IsOccluded true") {
		t.Fatalf("minimize: changes=%d log=%q", app.Changes, app.Log)
	}
	gui.DispatchWindowOccluded(w, false)
	if app.Changes != 2 || !strings.Contains(app.Log[1], "shown again") {
		t.Fatalf("restore: changes=%d log=%q", app.Changes, app.Log)
	}
	_ = mainView(w).GenerateLayout(w)
}

// The app's own Hide counts too, with IsVisible false.
func TestHideIsLogged(t *testing.T) {
	w := newTestWindow()
	app := gui.State[App](w)
	w.Hide()
	if app.Changes != 1 || !strings.Contains(app.Log[0], "IsVisible false") {
		t.Fatalf("hide: changes=%d log=%q", app.Changes, app.Log)
	}
	w.Show()
	if app.Changes != 2 {
		t.Errorf("show: changes=%d, want 2", app.Changes)
	}
}

func TestLogKeepsLastLines(t *testing.T) {
	app := &App{}
	now := time.Now()
	for i := range 3 * logLines {
		record(app, i%2 == 1, true, i%2 == 0, now)
	}
	if len(app.Log) != logLines {
		t.Errorf("log has %d lines, want %d", len(app.Log), logLines)
	}
}
