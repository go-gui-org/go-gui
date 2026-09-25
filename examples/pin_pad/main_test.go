package main

import (
	"testing"

	"github.com/go-gui-org/go-gui/gui"
)

func newPadWindow(t *testing.T) (*gui.Window, *App) {
	t.Helper()
	app := &App{}
	w := gui.NewTestWindow(gui.WindowCfg{State: app, Width: 320, Height: 480})
	w.TestRender(mainView)
	w.SetFocus(fieldID)
	w.TestRender(nil)
	return w, app
}

// press clicks a key and runs the frame that flushes the key event the
// click queued. TestRender does not run queued commands; FrameFn does,
// as the backend's frame would.
func press(t *testing.T, w *gui.Window, name string) {
	t.Helper()
	if err := w.TestClick(gui.ScopeID("key", name)); err != nil {
		t.Fatalf("click %s: %v", name, err)
	}
	w.FrameFn()
}

// The keys type into the field through the normal key path, and the
// field keeps focus the whole time.
func TestPadTypesIntoFocusedField(t *testing.T) {
	w, app := newPadWindow(t)
	for _, k := range []string{"1", "2", "3"} {
		press(t, w, k)
	}
	if app.PIN != "123" {
		t.Fatalf("PIN = %q, want 123", app.PIN)
	}
	if got := w.FocusID(); got != fieldID {
		t.Fatalf("focus = %q, want %q", got, fieldID)
	}
	press(t, w, "backspace")
	if app.PIN != "12" {
		t.Fatalf("PIN after backspace = %q, want 12", app.PIN)
	}
	press(t, w, "ok")
	if app.Submitted != "12" || app.PIN != "" {
		t.Fatalf("after OK: submitted=%q pin=%q, want 12 and empty",
			app.Submitted, app.PIN)
	}
}

// PreTextChange holds the PIN to pinLength digits.
func TestPadStopsAtPINLength(t *testing.T) {
	w, app := newPadWindow(t)
	for range pinLength + 2 {
		press(t, w, "7")
	}
	if len(app.PIN) != pinLength {
		t.Fatalf("PIN length = %d, want %d", len(app.PIN), pinLength)
	}
}

func TestValidPIN(t *testing.T) {
	for s, want := range map[string]bool{
		"": true, "0": true, "123456": true,
		"1234567": false, "12a": false, "-1": false,
	} {
		if got := validPIN(s); got != want {
			t.Errorf("validPIN(%q) = %v, want %v", s, got, want)
		}
	}
}
