package radios

import (
	"testing"

	"github.com/go-gui-org/go-gui/gui"
)

// newTestApp renders the example once.
// Not t.Parallel in callers: SetTheme mutates process-global theme state.
func newTestApp(t *testing.T) (*App, *gui.Window) {
	t.Helper()
	gui.SetTheme(gui.ThemeLight)
	app := New()
	w := gui.NewTestWindow(gui.WindowCfg{State: app, Width: 640, Height: 760})
	w.TestRender(func(*gui.Window) gui.View { return View(app) })
	return app, w
}

func mustFind(t *testing.T, w *gui.Window, id string) *gui.Layout {
	t.Helper()
	ly, ok := w.TestRender(nil).FindByID(id)
	if !ok {
		t.Fatalf("no widget %q", id)
	}
	return ly
}

// settle runs two frames: the first records what the pointer is over, the
// second builds the look from it. The real frame loop does both in one frame.
func settle(w *gui.Window) {
	w.TestRender(nil)
	w.TestRender(nil)
}

// A click selects the option and marks it selected for a screen reader.
func TestClickSelects(t *testing.T) {
	app, w := newTestApp(t)
	const id = "page:size:l"
	if err := w.TestClick(id); err != nil {
		t.Fatal(err)
	}
	if app.size != "l" || app.log != "Large selected" {
		t.Fatalf("size=%q log=%q", app.size, app.log)
	}
	if s := mustFind(t, w, id).Shape; s.A11YState != gui.AccessStateSelected {
		t.Fatalf("a11y state %v, want selected", s.A11YState)
	}
	if s := mustFind(t, w, "page:size:m").Shape; s.A11YState != gui.AccessStateNone {
		t.Fatalf("old option a11y state %v, want none", s.A11YState)
	}
}

// Only one option per group is in the Tab order: the selected one, or the
// first enabled one when nothing in the group is selected.
func TestOnlySelectedInTabOrder(t *testing.T) {
	_, w := newTestApp(t)
	if mustFind(t, w, "page:size:m").Shape.FocusSkip {
		t.Fatal("selected option skipped by Tab")
	}
	if !mustFind(t, w, "page:size:s").Shape.FocusSkip {
		t.Fatal("unselected option in the Tab order")
	}
	// The second XP group has no selection: its first option is the stop.
	if mustFind(t, w, "page:xp-lock:drive2:a").Shape.FocusSkip {
		t.Fatal("group with no selection has no Tab stop")
	}
	if !mustFind(t, w, "page:xp-lock:drive2:e").Shape.FocusSkip {
		t.Fatal("second option of an unselected group in the Tab order")
	}
}

// The arrow keys move the selection and the focus, skip the disabled option
// and wrap around.
func TestArrowKeysMoveSelection(t *testing.T) {
	app, w := newTestApp(t)
	if err := w.TestKey("page:size:m", gui.KeyDown, gui.ModNone); err != nil {
		t.Fatal(err)
	}
	if app.size != "l" {
		t.Fatalf("after Down size=%q, want l", app.size)
	}
	if !w.IsFocus("page:size:l") {
		t.Fatal("focus did not follow the selection to Large")
	}
	// Large → (disabled sample skipped) → Small.
	if err := w.TestKey("page:size:l", gui.KeyRight, gui.ModNone); err != nil {
		t.Fatal(err)
	}
	if app.size != "s" || !w.IsFocus("page:size:s") {
		t.Fatalf("after Right size=%q, want s with focus", app.size)
	}
	// Small → back past the start → Large.
	if err := w.TestKey("page:size:s", gui.KeyUp, gui.ModNone); err != nil {
		t.Fatal(err)
	}
	if app.size != "l" {
		t.Fatalf("after Up size=%q, want l", app.size)
	}
}

// A locked group is disabled: clicks and arrow keys change nothing.
func TestLockedGroupIgnoresInput(t *testing.T) {
	app, w := newTestApp(t)
	if err := w.TestClick("page:material-lock:lock"); err != nil {
		t.Fatal(err)
	}
	s := mustFind(t, w, "page:material-lock:theme:dark").Shape
	if !s.Disabled {
		t.Fatal("option not disabled after locking")
	}
	x, y := s.X+s.Width/2, s.Y+s.Height/2
	w.EventFn(&gui.Event{Type: gui.EventMouseDown, MouseButton: gui.MouseLeft, MouseX: x, MouseY: y})
	w.EventFn(&gui.Event{Type: gui.EventMouseUp, MouseButton: gui.MouseLeft, MouseX: x, MouseY: y})
	settle(w)
	if app.theme != "light" {
		t.Fatalf("locked group changed to %q", app.theme)
	}
}

// Hover lights the XP gold rim and moving away clears it. The rim sits in
// the well: row → blue ring → face → rim.
func TestXPHoverLightsRim(t *testing.T) {
	_, w := newTestApp(t)
	const id = "page:drive:d"
	rim := func() gui.Color {
		return mustFind(t, w, id).Children[0].Children[0].Children[0].Shape.Color
	}
	if c := rim(); c != gui.ColorTransparent {
		t.Fatalf("idle rim %+v, want transparent", c)
	}
	s := mustFind(t, w, id).Shape
	w.EventFn(&gui.Event{Type: gui.EventMouseMove, MouseX: s.X + s.Width/2, MouseY: s.Y + s.Height/2})
	settle(w)
	if c := rim(); c != xpGoldLo {
		t.Fatalf("hovered rim %+v, want gold", c)
	}
	w.EventFn(&gui.Event{Type: gui.EventMouseMove, MouseX: 1, MouseY: 1})
	settle(w)
	if c := rim(); c != gui.ColorTransparent {
		t.Fatalf("rim still lit after the pointer left: %+v", c)
	}
}

func TestInteractiveLooksClean(t *testing.T) {
	_, w := newTestApp(t)
	if d := w.TestDuplicateIDs(); len(d) != 0 {
		t.Fatalf("duplicate IDs: %v", d)
	}
	if found := w.TestFindings(gui.DebugMissingIDs | gui.DebugDuplicates); len(found) != 0 {
		t.Fatalf("findings: %v", found)
	}
}

// A group with no selection steps from the focused option. The second XP
// group shares its value with the first, so while "c" is picked above,
// nothing in it matches: Down from CD-ROM must wrap to Floppy, not stay put.
func TestArrowKeysStepFromFocusWithoutSelection(t *testing.T) {
	app, w := newTestApp(t)
	if app.drive != "c" {
		t.Fatalf("start drive=%q, want c", app.drive)
	}
	if err := w.TestKey("page:xp-lock:drive2:e", gui.KeyDown, gui.ModNone); err != nil {
		t.Fatal(err)
	}
	if app.drive != "a" {
		t.Fatalf("after Down from e drive=%q, want a", app.drive)
	}
	if !w.IsFocus("page:xp-lock:drive2:a") {
		t.Fatal("focus did not follow the selection to Floppy")
	}
}
