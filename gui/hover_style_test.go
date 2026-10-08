package gui

import "testing"

// Tests for ContainerCfg.Hover (#977). gui paints the hover fill and
// cursor itself, so no app code runs and a move inside the shape needs
// no rebuild.

var hoverStyleFill = RGB(200, 40, 40)

// hoverStyleWindow renders one fixed-size container "box" with the
// given hover style and OnHover.
func hoverStyleWindow(t *testing.T, gens *int, hs HoverStyle,
	onHover func(EventCtx), disabled bool) *Window {
	t.Helper()
	view := countingView(gens, func(*Window) View {
		return Column(ContainerCfg{
			Sizing: FillFill, Padding: PaddingNone, SizeBorder: NoBorder,
			Content: []View{Row(ContainerCfg{
				ID: "box", Sizing: FixedFixed, Width: 120, Height: 40,
				Padding: PaddingNone, SizeBorder: NoBorder,
				Color:    RGB(10, 10, 10),
				Hover:    hs,
				OnHover:  onHover,
				Disabled: disabled,
			})},
		})
	})
	w := NewTestWindow(t, WindowCfg{})
	w.TestRender(view)
	return w
}

func boxColor(t *testing.T, w *Window) Color {
	t.Helper()
	ly, ok := w.layout.FindByID("box")
	if !ok {
		t.Fatal("box left the tree")
	}
	return ly.Shape.Color
}

// Hovering paints the style's fill and cursor; leaving restores the
// resting fill.
func TestHoverStylePaintsFillAndCursor(t *testing.T) {
	gens := 0
	w := hoverStyleWindow(t, &gens, HoverStyle{
		Color: hoverStyleFill, Cursor: CursorPointingHand,
	}, nil, false)
	x, y := hitPoint(t, w, "box")
	moveAndFrame(w, &gens, x, y)
	if c := boxColor(t, w); c != hoverStyleFill {
		t.Fatalf("hover fill = %v, want %v", c, hoverStyleFill)
	}
	if w.viewState.mouseCursor != CursorPointingHand {
		t.Fatalf("cursor = %v, want pointing hand", w.viewState.mouseCursor)
	}
	moveAndFrame(w, &gens, x+500, y+500)
	if c := boxColor(t, w); c != RGB(10, 10, 10) {
		t.Fatalf("after leave fill = %v, want resting", c)
	}
}

// A move inside a shape whose only hover is a HoverStyle skips the
// rebuild and keeps the hover look.
func TestHoverStyleIdleMoveSkips(t *testing.T) {
	gens := 0
	w := hoverStyleWindow(t, &gens, HoverStyle{
		Color: hoverStyleFill, Cursor: CursorPointingHand,
	}, nil, false)
	x, y := hitPoint(t, w, "box")
	moveAndFrame(w, &gens, x, y)
	if n := moveAndFrame(w, &gens, x+1, y); n != 0 {
		t.Fatalf("move inside a HoverStyle shape: %d generations, want 0", n)
	}
	if c := boxColor(t, w); c != hoverStyleFill {
		t.Fatalf("hover fill lost: %v", c)
	}
	if w.viewState.mouseCursor != CursorPointingHand {
		t.Fatalf("cursor = %v, want pointing hand", w.viewState.mouseCursor)
	}
}

// Entering the shape is not idle: the fill must appear.
func TestHoverStyleEnterRebuilds(t *testing.T) {
	gens := 0
	w := hoverStyleWindow(t, &gens, HoverStyle{Color: hoverStyleFill}, nil, false)
	x, y := hitPoint(t, w, "box")
	moveAndFrame(w, &gens, x+500, y+500)
	if n := moveAndFrame(w, &gens, x, y); n == 0 {
		t.Fatal("move into a HoverStyle shape did not rebuild")
	}
}

// With an OnHover too, the style paints first, the callback runs after
// it and sees the fill, and a move inside rebuilds: OnHover is app
// code and may read the pointer.
func TestHoverStyleWithOnHover(t *testing.T) {
	gens := 0
	var seen Color
	w := hoverStyleWindow(t, &gens, HoverStyle{Color: hoverStyleFill},
		func(ctx EventCtx) { seen = ctx.Layout.Shape.Color }, false)
	x, y := hitPoint(t, w, "box")
	moveAndFrame(w, &gens, x, y)
	if seen != hoverStyleFill {
		t.Fatalf("OnHover saw %v, want the style fill painted first", seen)
	}
	if n := moveAndFrame(w, &gens, x+1, y); n == 0 {
		t.Fatal("move inside a shape with OnHover did not rebuild")
	}
}

// A disabled shape takes no hover look, like OnHover.
func TestHoverStyleDisabledNoPaint(t *testing.T) {
	gens := 0
	w := hoverStyleWindow(t, &gens, HoverStyle{
		Color: hoverStyleFill, Cursor: CursorPointingHand,
	}, nil, true)
	x, y := hitPoint(t, w, "box")
	moveAndFrame(w, &gens, x, y)
	if c := boxColor(t, w); c == hoverStyleFill {
		t.Fatal("disabled shape took the hover fill")
	}
	if w.viewState.mouseCursor == CursorPointingHand {
		t.Fatal("disabled shape set the hover cursor")
	}
}

// Painting the style allocates nothing.
func TestHoverStyleZeroAlloc(t *testing.T) {
	gens := 0
	w := hoverStyleWindow(t, &gens, HoverStyle{
		Color: hoverStyleFill, Cursor: CursorPointingHand,
	}, nil, false)
	x, y := hitPoint(t, w, "box")
	moveAndFrame(w, &gens, x, y)
	w.viewState.mousePosX, w.viewState.mousePosY = x, y
	allocs := testing.AllocsPerRun(100, func() {
		layoutHover(&w.layout, w)
	})
	if allocs != 0 {
		t.Fatalf("layoutHover with a HoverStyle: %v allocs, want 0", allocs)
	}
}

// A cursor-only style still takes the hover, sets the cursor and keeps
// the resting fill: an unset Color paints nothing.
func TestHoverStyleCursorOnlyKeepsFill(t *testing.T) {
	gens := 0
	w := hoverStyleWindow(t, &gens, HoverStyle{Cursor: CursorPointingHand}, nil, false)
	x, y := hitPoint(t, w, "box")
	moveAndFrame(w, &gens, x, y)
	if c := boxColor(t, w); c != RGB(10, 10, 10) {
		t.Fatalf("cursor-only hover fill = %v, want resting", c)
	}
	if w.viewState.mouseCursor != CursorPointingHand {
		t.Fatalf("cursor = %v, want pointing hand", w.viewState.mouseCursor)
	}
}
