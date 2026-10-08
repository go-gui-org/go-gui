package datagrid

import (
	"testing"

	gg "github.com/go-gui-org/go-gui/gui"
)

// Tests for #977: pointer moves over a grid skip the layout rebuild
// unless the hover look changes, and the header controls follow the
// hover target the last arrange recorded.

// hoverGrid renders a two-column, three-row grid "g1" and counts view
// generations into gens.
func hoverGrid(t *testing.T, gens *int) (*gg.Window, *gg.Layout) {
	t.Helper()
	cfg := DataGridCfg{
		ID:              "g1",
		TextStyleHeader: gg.DefaultTextStyle,
		TextStyle:       gg.DefaultTextStyle,
		ColorsHeader: gg.ColorSet{
			Base:  gg.RGB(240, 240, 240),
			Hover: gg.RGB(200, 200, 200),
		},
		ColorsRow: gg.ColorSet{
			Base:   gg.RGB(255, 255, 255),
			Hover:  gg.RGB(220, 220, 250),
			Border: gg.RGB(180, 180, 180),
		},
		ColorsResize:  gg.ColorSet{Base: gg.RGB(180, 180, 180)},
		PaddingHeader: gg.NewPadding(2, 4, 2, 4),
		SizeBorder:    gg.NoBorder,
		Columns: []GridColumnCfg{
			{ID: "c1", Title: "One", Sortable: true, Resizable: true, Width: gg.SomeF(120)},
			{ID: "c2", Title: "Two", Sortable: true, Resizable: true, Width: gg.SomeF(120)},
		},
		Rows: []GridRow{
			{ID: "r1", Cells: map[string]string{"c1": "a", "c2": "b"}},
			{ID: "r2", Cells: map[string]string{"c1": "c", "c2": "d"}},
			{ID: "r3", Cells: map[string]string{"c1": "e", "c2": "f"}},
		},
	}
	w := gg.NewTestWindow(t, gg.WindowCfg{})
	t.Cleanup(w.Close)
	root := w.TestRender(func(win *gg.Window) gg.View {
		*gens++
		return New(win, cfg)
	})
	return w, root
}

// center returns the center of the shape with effective ID id.
func center(t *testing.T, root *gg.Layout, id string) (float32, float32) {
	t.Helper()
	ly, ok := root.FindByID(id)
	if !ok {
		t.Fatalf("no shape %q", id)
	}
	s := ly.Shape
	return s.X + s.Width/2, s.Y + s.Height/2
}

// move sends one mouse move and runs one frame. It returns the number
// of generations that frame ran.
func move(w *gg.Window, gens *int, x, y float32) int {
	*gens = 0
	w.EventFn(&gg.Event{Type: gg.EventMouseMove, MouseX: x, MouseY: y})
	w.FrameFn()
	return *gens
}

// A move inside a row, which only has a HoverStyle, skips the rebuild
// and keeps the row's hover fill.
func TestGridIdleMoveInsideRowSkips(t *testing.T) {
	gens := 0
	w, root := hoverGrid(t, &gens)
	x, y := center(t, root, "g1:row:r2")
	move(w, &gens, x, y)
	ly, _ := root.FindByID("g1:row:r2")
	if ly.Shape.Color != gg.RGB(220, 220, 250) {
		t.Fatalf("row hover fill = %v, want the hover color", ly.Shape.Color)
	}
	if n := move(w, &gens, x+3, y); n != 0 {
		t.Fatalf("move inside a row: %d generations, want 0", n)
	}
	ly, _ = root.FindByID("g1:row:r2")
	if ly.Shape.Color != gg.RGB(220, 220, 250) {
		t.Fatalf("row hover fill lost: %v", ly.Shape.Color)
	}
}

// Hovering a header cell shows its controls; moving onto the resize
// handle keeps them; leaving the header hides them.
func TestGridHeaderControlsFollowHover(t *testing.T) {
	gens := 0
	w, root := hoverGrid(t, &gens)
	if _, ok := root.FindByID("g1:resize:c1"); ok {
		t.Fatal("resize handle shown before any hover")
	}
	x, y := center(t, root, "g1:header:c1")
	move(w, &gens, x, y)
	if _, ok := root.FindByID("g1:resize:c1"); !ok {
		t.Fatal("hovering the header cell did not show its resize handle")
	}
	if _, ok := root.FindByID("g1:resize:c2"); ok {
		t.Fatal("hovering c1 showed c2's resize handle")
	}

	// The handle's ID is absolute, so the hover target there is not
	// under the cell. The controls must stay shown anyway; two moves,
	// so a frame that hides them and one that shows them again would
	// both be caught.
	hx, hy := center(t, root, "g1:resize:c1")
	for i := range 2 {
		move(w, &gens, hx, hy)
		if _, ok := root.FindByID("g1:resize:c1"); !ok {
			t.Fatalf("move %d onto the resize handle hid the controls", i)
		}
	}

	rx, ry := center(t, root, "g1:row:r3")
	move(w, &gens, rx, ry)
	if _, ok := root.FindByID("g1:resize:c1"); ok {
		t.Fatal("leaving the header left the controls shown")
	}
}

// A move inside a header cell, once its controls show, skips the
// rebuild too: the grid has no position handler left.
func TestGridIdleMoveInsideHeaderSkips(t *testing.T) {
	gens := 0
	w, root := hoverGrid(t, &gens)
	x, y := center(t, root, "g1:header:c1")
	move(w, &gens, x, y)
	if n := move(w, &gens, x+1, y); n != 0 {
		t.Fatalf("move inside a header cell: %d generations, want 0", n)
	}
}

// A non-sortable header cell has no hover look, so it takes no hover:
// an app OnHover around the grid fires over it (#977). A row still
// takes the hover, so the same OnHover does not fire over a row.
func TestGridNonSortableHeaderPassesHover(t *testing.T) {
	outer := 0
	cfg := DataGridCfg{
		ID:              "g1",
		TextStyleHeader: gg.DefaultTextStyle,
		TextStyle:       gg.DefaultTextStyle,
		SizeBorder:      gg.NoBorder,
		Columns: []GridColumnCfg{
			{ID: "c1", Title: "One", Width: gg.SomeF(120)},
		},
		Rows: []GridRow{{ID: "r1", Cells: map[string]string{"c1": "a"}}},
	}
	w := gg.NewTestWindow(t, gg.WindowCfg{})
	t.Cleanup(w.Close)
	root := w.TestRender(func(win *gg.Window) gg.View {
		return gg.Column(gg.ContainerCfg{
			Sizing: gg.FillFill, Padding: gg.NoPadding, SizeBorder: gg.NoBorder,
			OnHover: func(gg.EventCtx) { outer++ },
			Content: []gg.View{New(win, cfg)},
		})
	})
	gens := 0
	x, y := center(t, root, "g1:header:c1")
	move(w, &gens, x, y)
	if outer == 0 {
		t.Fatal("app OnHover did not fire over a non-sortable header")
	}
	outer = 0
	rx, ry := center(t, root, "g1:row:r1")
	move(w, &gens, rx, ry)
	if outer != 0 {
		t.Fatal("app OnHover fired over a row, which takes the hover")
	}
}
