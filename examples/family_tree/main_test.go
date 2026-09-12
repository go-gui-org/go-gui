package main

import (
	"testing"

	"github.com/go-gui-org/go-gui/gui"
)

// newTestApp lays out root and renders it in a window narrower than the
// sample tree, so the horizontal scroll axis has room to move.
// Not t.Parallel in callers: SetTheme mutates process-global theme state.
func newTestApp(t *testing.T, root *Person) (*App, *gui.Window) {
	t.Helper()
	gui.SetTheme(gui.ThemeDark)
	app := newApp(root)
	w := gui.NewTestWindow(gui.WindowCfg{State: app, Width: 900, Height: 600})
	w.TestRender(mainView)
	return app, w
}

// The tree must be wider than the window, or the example does not show
// horizontal scrolling at all.
func TestTreeWiderThanWindow(t *testing.T) {
	app := newApp(sampleFamily())
	if app.width <= 900 {
		t.Fatalf("tree width %v, want wider than the 900px window", app.width)
	}
}

// Names in one generation must not overlap, and every name and line must sit
// inside the canvas. A layout bug here draws silently wrong, so no other test
// would catch it.
func TestLayoutNoOverlapAndInBounds(t *testing.T) {
	app := newApp(sampleFamily())
	for i, a := range app.nodes {
		if a.x < 0 || a.y < 0 || a.x+nodeW > app.width || a.y+nodeH > app.height {
			t.Fatalf("%s at (%v,%v) outside %vx%v", a.name, a.x, a.y, app.width, app.height)
		}
		for _, b := range app.nodes[i+1:] {
			if a.y == b.y && a.x < b.x+nodeW && b.x < a.x+nodeW {
				t.Fatalf("%s and %s overlap at y=%v", a.name, b.name, a.y)
			}
		}
	}
	for _, s := range app.segments {
		for k := 0; k < len(s); k += 2 {
			if s[k] < 0 || s[k] > app.width || s[k+1] < 0 || s[k+1] > app.height {
				t.Fatalf("segment %v leaves the %vx%v canvas", s, app.width, app.height)
			}
		}
	}
}

// A lone person has one box, no lines, and a canvas just big enough for it.
func TestSinglePersonTree(t *testing.T) {
	app := newApp(&Person{Name: "Solo"})
	if len(app.nodes) != 1 || len(app.segments) != 0 {
		t.Fatalf("nodes %d segments %d, want 1 and 0", len(app.nodes), len(app.segments))
	}
	if want := nodeW + 2*margin; app.width != want {
		t.Fatalf("width %v, want %v", app.width, want)
	}
	if want := nodeH + 2*margin; app.height != want {
		t.Fatalf("height %v, want %v", app.height, want)
	}
}

// A nil family must not panic, in layout or in the view.
func TestNilFamilyRenders(t *testing.T) {
	app, _ := newTestApp(t, nil)
	if len(app.nodes) != 0 {
		t.Fatalf("nodes %d, want 0", len(app.nodes))
	}
}

// Clicking a name selects that person. Asserting on state proves the click
// reached the button through the canvas, not only that dispatch ran.
func TestClickSelectsPerson(t *testing.T) {
	app, w := newTestApp(t, sampleFamily())

	if err := w.TestClick(app.nodes[0].id); err != nil {
		t.Fatalf("TestClick: %v", err)
	}
	if app.selected != 0 {
		t.Fatalf("selected %d, want 0 (%s)", app.selected, app.nodes[0].name)
	}
	if dups := w.TestDuplicateIDs(); len(dups) > 0 {
		t.Fatalf("duplicate IDs: %v", dups)
	}
}

// The right-most name starts outside the window. Once the view scrolls, it
// must become visible and clickable. This fails if the canvas does not
// report the full tree size as its scroll range.
func TestFarNodeReachableByScroll(t *testing.T) {
	app, w := newTestApp(t, sampleFamily())

	far := 0
	for i, n := range app.nodes {
		if n.x > app.nodes[far].x {
			far = i
		}
	}
	id := app.nodes[far].id

	if err := w.TestClick(id); err == nil {
		t.Fatalf("%s clickable before scrolling; want it outside the window",
			app.nodes[far].name)
	}

	// ScrollHorizontalTo clamps to the scroll range, so asking for the full
	// tree width lands at the right edge only if the range covers the tree.
	// TestScroll is not used: it sends no modifier, and a precise scroll
	// without Shift moves the vertical axis only.
	w.ScrollHorizontalTo("tree", -app.width)
	w.TestRender(mainView)
	if x, _, err := w.TestScrollOffset("tree"); err != nil || x == 0 {
		t.Fatalf("scroll offset x=%v err=%v, want a horizontal offset", x, err)
	}

	if err := w.TestClick(id); err != nil {
		t.Fatalf("TestClick after scroll: %v", err)
	}
	if app.selected != far {
		t.Fatalf("selected %d, want %d (%s)", app.selected, far, app.nodes[far].name)
	}
}
