package gui

import "testing"

// rtlCanvas renders a 300px-wide RTL Canvas, scrollable or not, holding
// one 20px-tall child at x with the given width, and returns the canvas
// and the child.
func rtlCanvas(t *testing.T, scrollable bool, x, width, offset float32) (s, child *Layout) {
	t.Helper()
	w := NewTestWindow(t, WindowCfg{Width: 300, Height: 200})
	w.TestRender(func(*Window) View {
		return Canvas(ContainerCfg{ID: "s", Scrollable: scrollable, Sizing: FillFill,
			HAlign: HAlignLeft, Padding: PaddingNone, SizeBorder: NoBorder,
			TextDir: TextDirRTL, Content: []View{
				Column(ContainerCfg{ID: "c", X: x, Width: width, Height: 20, Sizing: FixedFixed,
					Padding: PaddingNone, SizeBorder: NoBorder}),
			}})
	})
	if offset != 0 {
		w.ScrollHorizontalTo("s", offset)
	}
	l := w.TestRender(nil)
	var ok bool
	if s, ok = l.FindByID("s"); !ok {
		t.Fatal("canvas not found")
	}
	if child, ok = l.FindByID(ScopeID("s", "c")); !ok {
		t.Fatal("child not found")
	}
	return s, child
}

// A Canvas counts its scroll range from the left edge (scrollMirrorsX), so
// its offset is not mirrored under RTL: scrolled to the end, a child that
// sticks out past the right edge comes into view, as it does under LTR.
func TestRTLCanvasScrollRevealsRightOverflow(t *testing.T) {
	s, child := rtlCanvas(t, true, 250, 100, -100000)
	if got := scrollMaxOffsetX(s); !f32AreClose(got, -50) {
		t.Fatalf("max offset = %v, want -50", got)
	}
	left, right := s.Shape.X, s.Shape.X+s.Shape.Width
	if c := child.Shape; c.X < left || c.X+c.Width > right {
		t.Fatalf("scrolled to the end: child spans %v..%v, want inside the viewport %v..%v",
			c.X, c.X+c.Width, left, right)
	}
}

// A Canvas child wider than the canvas is not moved under RTL: only
// a Column or a Stack pins it at the right edge (childCrossAxisHAlign).
func TestRTLCanvasWideChildKeepsItsX(t *testing.T) {
	s, child := rtlCanvas(t, false, 0, 400, 0)
	if !f32AreClose(child.Shape.X, s.Shape.X) {
		t.Fatalf("child X = %v, want the canvas's left edge %v", child.Shape.X, s.Shape.X)
	}
}
