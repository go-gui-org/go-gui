package main

import (
	"testing"

	"github.com/go-gui-org/go-gui/gui"
)

// flipPanelTestWindow renders the showcase at the flip panel page, in a
// window tall enough that every panel is laid out in view: TestClick
// aims at the visible clip center and misses a scrolled-out target.
func flipPanelTestWindow(t *testing.T, app *ShowcaseApp) *gui.Window {
	t.Helper()
	app.SelectedGroup = groupLayout
	app.SelectedComponent = "flip_panel"
	w := gui.NewTestWindow(t, gui.WindowCfg{State: app, Width: 950, Height: 1200})
	w.TestRender(mainView)
	w.TestRender(nil)
	return w
}

// Clicking each panel turns it over, and a second click turns it back.
// flip-details is left out: TestClick aims at the panel center, which on
// its back face is the Like button. Once the turn passes 90° (a slow run,
// such as under -race, lets the real-time spring get there) the button
// takes the second click by design. TestFlipPanelPageBackButtonKeepsFace
// covers that panel from rest.
func TestFlipPanelPageClickToggles(t *testing.T) {
	app := newShowcaseApp()
	w := flipPanelTestWindow(t, app)

	for _, tc := range []struct {
		id      string
		flipped *bool
	}{
		{"flip-y", &app.FlipPanelY},
		{"flip-x", &app.FlipPanelX},
		{"flip-bouncy", &app.FlipPanelBouncy},
		{"flip-click-only", &app.FlipPanelClickOnly},
	} {
		id := gui.ScopeID("detail", tc.id)
		if err := w.TestClick(id); err != nil {
			t.Fatalf("TestClick %s: %v", id, err)
		}
		if !*tc.flipped {
			t.Fatalf("%s: flipped = false after a click, want true", id)
		}
		if err := w.TestClick(id); err != nil {
			t.Fatalf("second TestClick %s: %v", id, err)
		}
		if *tc.flipped {
			t.Fatalf("%s: flipped = true after a second click, want false", id)
		}
	}
}

// The like button on the back face consumes its click: it counts the
// like, and the panel does not turn over.
func TestFlipPanelPageBackButtonKeepsFace(t *testing.T) {
	app := newShowcaseApp()
	// Seen first while flipped, the panel rests on its back face, so
	// the button is drawn and takes the pointer.
	app.FlipPanelDetails = true
	w := flipPanelTestWindow(t, app)

	id := gui.ScopeID("detail", "flip-details", "flip-like")
	if err := w.TestClick(id); err != nil {
		t.Fatalf("TestClick %s: %v", id, err)
	}
	if app.FlipPanelLikes != 1 {
		t.Fatalf("FlipPanelLikes = %d, want 1", app.FlipPanelLikes)
	}
	if !app.FlipPanelDetails {
		t.Fatal("the button click turned the panel over")
	}
}

// Enter turns a focusable panel over. The click-only panel cannot
// take focus, so a key press cannot reach it.
func TestFlipPanelPageKeyboard(t *testing.T) {
	app := newShowcaseApp()
	w := flipPanelTestWindow(t, app)

	if err := w.TestKey(gui.ScopeID("detail", "flip-y"), gui.KeyEnter, 0); err != nil {
		t.Fatalf("TestKey flip-y: %v", err)
	}
	if !app.FlipPanelY {
		t.Fatal("Enter did not turn flip-y over")
	}
	if err := w.TestKey(gui.ScopeID("detail", "flip-click-only"), gui.KeyEnter, 0); err == nil {
		t.Fatal("the click-only panel took focus")
	}
}

func TestFlipPanelPageNoFindings(t *testing.T) {
	app := newShowcaseApp()
	w := flipPanelTestWindow(t, app)
	if found := w.TestFindings(gui.DebugAll); len(found) > 0 {
		t.Fatalf("findings:\n%v", found)
	}
}
