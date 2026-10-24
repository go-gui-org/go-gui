package main

import (
	"testing"

	"github.com/go-gui-org/go-gui/gui"
)

// newViewWindow is a window showing mainView with app as its state.
func newViewWindow(app *App) *gui.Window {
	w := gui.NewWindow(gui.WindowCfg{
		State:   app,
		Width:   windowW,
		Height:  windowH,
		OnEvent: handleEvent,
	})
	w.TestRender(mainView)
	return w
}

func TestPauseFreezesOrreryNotCamera(t *testing.T) {
	t.Parallel()
	a := newTestApp()
	togglePause(a, nil)
	a.beginTransition()
	a.Selected = earthIndex
	before, tween := a.Time, a.TweenT
	tick(a, wallTickSecs)
	if a.Time != before {
		t.Fatalf("paused tick moved Time %v -> %v", before, a.Time)
	}
	if a.TweenT <= tween {
		t.Fatalf("paused tick froze the camera tween at %v", a.TweenT)
	}
	togglePause(a, nil)
	tick(a, wallTickSecs)
	if a.Time == before {
		t.Fatalf("resumed tick did not move Time")
	}
}

func TestSpeedScalesAndClamps(t *testing.T) {
	t.Parallel()
	a, b := newTestApp(), newTestApp()
	stepSpeed(b, nil, speedStep)
	tick(a, wallTickSecs)
	tick(b, wallTickSecs)
	if got, want := b.Time, a.Time*speedStep; got < want*0.999 || got > want*1.001 {
		t.Fatalf("2x speed advanced %v, want %v", got, want)
	}
	for range 20 {
		stepSpeed(b, nil, speedStep)
	}
	if b.Speed != speedMax {
		t.Fatalf("speed %v after many steps up, want cap %v", b.Speed, speedMax)
	}
	for range 40 {
		stepSpeed(b, nil, 1/speedStep)
	}
	if b.Speed != speedMin {
		t.Fatalf("speed %v after many steps down, want floor %v", b.Speed, speedMin)
	}
}

func TestSpeedLabel(t *testing.T) {
	t.Parallel()
	for _, c := range []struct {
		in   float32
		want string
	}{{1, "1x"}, {0.25, "0.25x"}, {16, "16x"}} {
		if got := speedLabel(c.in); got != c.want {
			t.Fatalf("speedLabel(%v) = %q, want %q", c.in, got, c.want)
		}
	}
}

// TestControlKeys drives the new window keys through handleEvent, the
// path a real key press takes when no focused widget claims it.
func TestControlKeys(t *testing.T) {
	a := newApp()
	w := newViewWindow(a)
	press := func(k gui.KeyCode) {
		t.Helper()
		e := &gui.Event{Type: gui.EventKeyDown, KeyCode: k}
		handleEvent(e, w)
		if !e.IsHandled {
			t.Fatalf("key %v not handled", k)
		}
	}
	press(gui.KeySpace)
	if !a.Paused {
		t.Fatalf("Space did not pause")
	}
	press(gui.KeyUp)
	if a.Speed != speedStep {
		t.Fatalf("Up gave speed %v, want %v", a.Speed, float32(speedStep))
	}
	press(gui.KeyDown)
	if a.Speed != 1 {
		t.Fatalf("Down gave speed %v, want 1", a.Speed)
	}
	press(gui.KeyM)
	if !a.Muted {
		t.Fatalf("M did not mute")
	}
	press(gui.KeyM)
	if a.Muted {
		t.Fatalf("second M did not unmute")
	}
}

// TestControlButtons clicks the top-right buttons and checks that the
// labels follow the state, which needs the view refresh they ask for.
func TestControlButtons(t *testing.T) {
	a := newApp()
	w := newViewWindow(a)
	click := func(id string) {
		t.Helper()
		if err := w.TestClick(id); err != nil {
			t.Fatalf("click %s: %v", id, err)
		}
	}
	click("solar_pause")
	if !a.Paused {
		t.Fatalf("pause button did not pause")
	}
	click("solar_mute")
	if !a.Muted {
		t.Fatalf("mute button did not mute")
	}
	click("solar_mute")
	if dups := w.TestDuplicateIDs(); len(dups) != 0 {
		t.Fatalf("duplicate IDs: %v", dups)
	}
}

// TestCloseButtonClearsSelection covers the touch path out of a fact
// sheet: no Escape key, and nothing says empty space closes it.
func TestCloseButtonClearsSelection(t *testing.T) {
	a := newApp()
	a.Selected = marsIndexForTest()
	w := newViewWindow(a)
	ids := w.ResolveID("close")
	if len(ids) != 1 {
		t.Fatalf("close button IDs %v, want exactly one", ids)
	}
	if err := w.TestClick(ids[0]); err != nil {
		t.Fatalf("click close: %v", err)
	}
	if a.Selected != -1 {
		t.Fatalf("close left Selected = %d, want -1", a.Selected)
	}
	if got := w.ResolveID("close"); len(got) != 0 {
		t.Fatalf("fact sheet still shows a close button after closing")
	}
}

func marsIndexForTest() int {
	for i := range planets {
		if planets[i].Name == "Mars" {
			return i
		}
	}
	panic("no Mars")
}

func TestSheetWidth(t *testing.T) {
	t.Parallel()
	for _, c := range []struct {
		winW   int
		width  float32
		narrow bool
	}{
		{0, panelWidth, false}, // size not known yet reads as wide
		{windowW, panelWidth, false},
		{panelWidth + 2*panelMargin, panelWidth, false},
		{390, 390 - 2*panelMargin, true}, // a phone
		{10, 0, true},
	} {
		w, n := sheetWidth(c.winW)
		if w != c.width || n != c.narrow {
			t.Fatalf("sheetWidth(%d) = %v,%v want %v,%v", c.winW, w, n, c.width, c.narrow)
		}
	}
}

func TestStatRowsNarrowStacksCards(t *testing.T) {
	t.Parallel()
	w := gui.NewWindow(gui.WindowCfg{State: newApp(), Width: 390, Height: 700})
	theme := gui.CurrentTheme()
	wide := statRows(&planets[earthIndex], theme, false).GenerateLayout(w)
	if len(wide.Children) != 6 {
		t.Fatalf("wide: %d cards in one row, want 6", len(wide.Children))
	}
	narrow := statRows(&planets[earthIndex], theme, true).GenerateLayout(w)
	if len(narrow.Children) != 2 {
		t.Fatalf("narrow: %d rows, want 2", len(narrow.Children))
	}
	for i, row := range narrow.Children {
		if len(row.Children) != 3 {
			t.Fatalf("narrow row %d: %d cards, want 3", i, len(row.Children))
		}
	}
}

// TestDotHoverNamesBody checks that a hovered nav dot gets the canvas
// tooltip, and that it wins over a planet under the cursor.
func TestDotHoverNamesBody(t *testing.T) {
	t.Parallel()
	a := newTestApp()
	a.MouseIn = true
	a.MouseX, a.MouseY = 500, 700
	a.Hovered = earthIndex
	a.DotHover = selSun
	dc := gui.NewDrawContext(a.CanvasW, a.CanvasH, nil)
	drawTooltip(a, dc)
	texts := dc.Texts()
	if len(texts) != 1 || texts[0].Text != sun.Name {
		t.Fatalf("tooltip texts %+v, want one naming %q", texts, sun.Name)
	}
	// While a dot is hovered, the tick does not hover a planet behind it.
	tick(a, wallTickSecs)
	if a.Hovered != -1 {
		t.Fatalf("tick hovered body %d under a hovered dot", a.Hovered)
	}
}

// TestNarrowSheetFitsWindow renders the whole view at phone width. The
// sheet once took its width only as a starting size, and the unwrapped
// fun fact grew it past the window edge.
func TestNarrowSheetFitsWindow(t *testing.T) {
	const phoneW = 390
	a := newApp()
	a.Selected = earthIndex
	w := gui.NewWindow(gui.WindowCfg{State: a, Width: phoneW, Height: windowH})
	root := w.TestRender(mainView)
	sheet := findShape(root, "solar_facts")
	if sheet == nil {
		t.Fatalf("no fact sheet in the rendered tree")
	}
	if sheet.Shape.X < 0 || sheet.Shape.X+sheet.Shape.Width > phoneW {
		t.Fatalf("sheet spans x %.0f..%.0f, outside a %d px window",
			sheet.Shape.X, sheet.Shape.X+sheet.Shape.Width, phoneW)
	}
}

// findShape returns the first layout in the tree whose leaf ID is id.
func findShape(l *gui.Layout, id string) *gui.Layout {
	if l.Shape != nil && l.Shape.ID == id {
		return l
	}
	for i := range l.Children {
		if f := findShape(&l.Children[i], id); f != nil {
			return f
		}
	}
	return nil
}
