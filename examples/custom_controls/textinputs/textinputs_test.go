package textinputs

import (
	"strings"
	"testing"

	"github.com/go-gui-org/go-gui/gui"
)

// newTestApp renders the example once.
// Not t.Parallel in callers: SetTheme mutates process-global theme state.
func newTestApp(t *testing.T) (*App, *gui.Window) {
	t.Helper()
	gui.SetTheme(gui.ThemeLight)
	app := New()
	w := gui.NewTestWindow(gui.WindowCfg{State: app, Width: 720, Height: 1000})
	w.TestRender(func(*gui.Window) gui.View { return View(app) })
	return app, w
}

func mustFind(t *testing.T, w *gui.Window, id string) *gui.Shape {
	t.Helper()
	ly, ok := w.TestRender(nil).FindByID(id)
	if !ok {
		t.Fatalf("no widget %q", id)
	}
	return ly.Shape
}

// Typing into the Input inside a custom field updates the app's text.
func TestTypingUpdatesText(t *testing.T) {
	app, w := newTestApp(t)
	for id, key := range map[string]string{
		"page:material:email:field": "email",
		"page:xp:xpnotes:field":     "xpnotes",
	} {
		before := app.text[key]
		if err := w.TestType(id, "Z"); err != nil {
			t.Fatal(err)
		}
		if got := app.text[key]; got == before || !strings.Contains(got, "Z") {
			t.Fatalf("%s: text %q after typing Z into %q", id, got, before)
		}
	}
}

// Focus on the inner Input changes the wrapper's look: the Material border
// and underline take the accent and the line grows, the XP border darkens.
// The field keeps its height.
func TestInputFocusStylesWrapper(t *testing.T) {
	_, w := newTestApp(t)
	const mat = "page:material:email"
	const xp = "page:xp:user"

	idleMat := mustFind(t, w, mat)
	idleH := idleMat.Height
	if idleMat.ColorBorder != materialBorder {
		t.Fatalf("idle material border %v, want %v", idleMat.ColorBorder, materialBorder)
	}

	w.SetFocus(gui.ScopeID(mat, fieldID))
	s := mustFind(t, w, mat)
	if s.ColorBorder != materialBlue {
		t.Fatalf("focused material border %v, want accent %v", s.ColorBorder, materialBlue)
	}
	if s.Height != idleH {
		t.Fatalf("focused material height %v, want %v", s.Height, idleH)
	}
	ul, ok := w.TestRender(nil).FindByID(gui.ScopeID(mat, "underline"))
	if !ok || len(ul.Children) != 1 || ul.Children[0].Shape.Height != 2 ||
		ul.Children[0].Shape.Color != materialBlue {
		t.Fatalf("focused underline not a 2 px accent line")
	}
	if got := mustFind(t, w, xp).ColorBorder; got != xpBorder {
		t.Fatalf("unfocused xp border %v, want %v", got, xpBorder)
	}

	w.SetFocus(gui.ScopeID(xp, fieldID))
	if got := mustFind(t, w, xp).ColorBorder; got != xpFocus {
		t.Fatalf("focused xp border %v, want %v", got, xpFocus)
	}
	if got := mustFind(t, w, mat).ColorBorder; got != materialBorder {
		t.Fatalf("blurred material border %v, want %v", got, materialBorder)
	}
}

// A press on the wrapper's padding, outside the Input, focuses the Input.
func TestPressOnPaddingFocusesInput(t *testing.T) {
	_, w := newTestApp(t)
	const id = "page:xp:xpnotes"
	s := mustFind(t, w, id)
	in := mustFind(t, w, gui.ScopeID(id, fieldID))
	// Inside the wrapper, left of the Input.
	x, y := s.X+2, in.Y+in.Height/2
	if x >= in.X {
		t.Fatalf("no padding left of the Input: wrapper x %v, input x %v", s.X, in.X)
	}
	w.EventFn(&gui.Event{Type: gui.EventMouseDown, MouseButton: gui.MouseLeft, MouseX: x, MouseY: y})
	w.EventFn(&gui.Event{Type: gui.EventMouseUp, MouseButton: gui.MouseLeft, MouseX: x, MouseY: y})
	w.TestRender(nil)
	if !w.IsFocus(gui.ScopeID(id, fieldID)) {
		t.Fatalf("focus %q after press on padding, want the Input", w.FocusID())
	}
}

// The window has no duplicate IDs or debug findings.
func TestInteractiveLooksClean(t *testing.T) {
	_, w := newTestApp(t)
	w.SetFocus(gui.ScopeID("page:material:matnotes", fieldID))
	w.TestRender(nil)
	if dups := w.TestDuplicateIDs(); len(dups) > 0 {
		t.Fatalf("duplicate IDs: %v", dups)
	}
	if found := w.TestFindings(gui.DebugAll); len(found) > 0 {
		t.Fatalf("debug findings: %v", found)
	}
}
