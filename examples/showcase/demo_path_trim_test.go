package main

import (
	"slices"
	"testing"

	"github.com/go-gui-org/go-gui/gui"
)

// pathTrimTestWindow renders the showcase at the Draw Canvas page, in a
// window tall enough that the path trim panel is laid out in view.
func pathTrimTestWindow(t *testing.T, app *ShowcaseApp) *gui.Window {
	t.Helper()
	app.SelectedGroup = groupDrawing
	app.SelectedComponent = "draw_canvas"
	w := gui.NewTestWindow(t, gui.WindowCfg{State: app, Width: 950, Height: 2000})
	w.TestRender(mainView)
	w.TestRender(nil)
	return w
}

// The page opens with the paths drawn whole, and Replay draws them in
// again from nothing.
func TestPathTrimReplay(t *testing.T) {
	app := newShowcaseApp()
	if app.PathTrimProgress != 1 {
		t.Fatalf("progress %v at start, want 1: the page opens drawn",
			app.PathTrimProgress)
	}
	w := pathTrimTestWindow(t, app)
	id := gui.ScopeID("detail", "path-trim-replay")
	if err := w.TestClick(id); err != nil {
		t.Fatalf("TestClick %s: %v", id, err)
	}
	if !w.HasAnimation(pathTrimAnim) {
		t.Fatal("Replay did not start the trim animation")
	}
}

// More progress draws more of both paths.
func TestPathTrimProgressDrawsMore(t *testing.T) {
	count := func(p float32) int {
		app := &ShowcaseApp{PathTrimProgress: p}
		dc := gui.NewDrawContext(pathTrimW, pathTrimH, nil)
		drawPathTrim(dc, app)
		n := 0
		for _, b := range dc.Batches() {
			n += len(b.Triangles)
		}
		return n
	}
	prev := count(0)
	for _, p := range []float32{0.25, 0.5, 1} {
		n := count(p)
		if n <= prev {
			t.Errorf("progress %v draws %d floats, want more than %d",
				p, n, prev)
		}
		prev = n
	}
}

// The canvas repaints only when its version changes, and a theme switch
// does not change it. So the strokes take no color from the theme: one
// would stay stale after Dark/Light until the next Replay.
func TestPathTrimColorsIgnoreTheme(t *testing.T) {
	prev := gui.CurrentTheme()
	t.Cleanup(func() { gui.SetTheme(prev) })
	colors := func(theme gui.Theme) []gui.Color {
		gui.SetTheme(theme)
		dc := gui.NewDrawContext(pathTrimW, pathTrimH, nil)
		drawPathTrim(dc, &ShowcaseApp{PathTrimProgress: 1})
		var out []gui.Color
		for _, b := range dc.Batches() {
			out = append(out, b.Color)
		}
		return out
	}
	dark, light := colors(gui.ThemeDark), colors(gui.ThemeLight)
	if !slices.Equal(dark, light) {
		t.Fatalf("stroke colors follow the theme: dark %v, light %v", dark, light)
	}
}
