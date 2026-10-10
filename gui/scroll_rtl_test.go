package gui

import "testing"

// rtlScrollRow builds an RTL scrollable row whose three children are wider
// than the viewport: 3 * 60 in a box of 100, so the content overflows by 80
// on the left, which is where an RTL row puts what does not fit.
func rtlScrollRow(t *testing.T) (*Window, *Layout) {
	t.Helper()
	w := NewWindow(WindowCfg{})
	t.Cleanup(w.Close)
	root := &Layout{Shape: &Shape{shapeType: shapeRectangle, ID: "sc",
		Axis: axisLeftToRight, Scrollable: true, Sizing: FixedFixed,
		Width: 100, Height: 10, TextDir: TextDirRTL}}
	for range 3 {
		root.Children = append(root.Children, Layout{Shape: &Shape{
			shapeType: shapeRectangle, Sizing: FixedFixed, Width: 60, Height: 10}})
	}
	layoutParents(root, nil)
	layoutPipeline(root, w)
	return w, root
}

// An RTL row lays its children out leftward from the right edge, so the
// content that does not fit is off the left side. The offset is held in
// [maxOffset, 0] and used to mean "content moves left", which pushed the
// overflow further away: nothing off the left could ever be reached.
func TestRTLRowScrollRevealsLeadingOverflow(t *testing.T) {
	w, root := rtlScrollRow(t)
	maxOffset := scrollMaxOffsetX(root)
	if !f32AreClose(maxOffset, -80) {
		t.Fatalf("maxOffset: got %v, want -80", maxOffset)
	}
	w.scrollX().Set("sc", maxOffset)
	for i := range root.Children {
		root.Children[i].Shape.X = 0
	}
	root.Shape.X = 0
	layoutPositions(root, 0, 0, w)
	// At the far end of the range the last child — the leftmost one — sits
	// at the viewport's left edge, and the first child has moved off to the
	// right by the same 80.
	if got := root.Children[2].Shape.X; !f32AreClose(got, 0) {
		t.Errorf("last child X at end of range: got %v, want 0", got)
	}
	if got := root.Children[0].Shape.X; !f32AreClose(got, 120) {
		t.Errorf("first child X at end of range: got %v, want 120", got)
	}
}

// The wheel, trackpad, keyboard and pan paths all reach scrollHorizontal.
// The delta is a physical content displacement, so an RTL row has to take
// it mirrored: dragging the content rightward is what reveals the overflow
// on the left.
func TestRTLRowScrollHorizontalMirrorsDelta(t *testing.T) {
	w, root := rtlScrollRow(t)
	if moved := scrollHorizontal(root, 1, w); !moved {
		t.Fatal("a rightward delta must move an RTL row toward its overflow")
	}
	if got, _ := w.scrollX().Get("sc"); got >= 0 {
		t.Errorf("offset after rightward delta: got %v, want < 0", got)
	}
	w.scrollX().Set("sc", 0)
	if moved := scrollHorizontal(root, -1, w); moved {
		t.Error("a leftward delta at the start edge must not move an RTL row")
	}
}

// A discrete sideways wheel (Win32 WM_MOUSEHWHEEL, X11 button 7) eases
// through scrollSmoothBy, not scrollHorizontal. It must mirror in an RTL
// row the same way as the instant path: from the start edge a negative
// ScrollX moves nothing, and a positive one reveals the overflow.
func TestRTLRowDiscreteWheelMirrorsDelta(t *testing.T) {
	w := NewTestWindow(t, WindowCfg{Width: 400, Height: 300})
	w.TestRender(func(_ *Window) View {
		return Row(ContainerCfg{
			ID:         "sc",
			Scrollable: true,
			ScrollMode: ScrollHorizontalOnly,
			TextDir:    TextDirRTL,
			Sizing:     FixedFixed,
			Width:      100,
			Height:     40,
			Padding:    PaddingNone,
			SizeBorder: NoBorder,
			Content: []View{
				Column(ContainerCfg{Sizing: FixedFixed, Width: 300, Height: 40,
					SizeBorder: NoBorder}),
			},
		})
	})
	ly, ok := w.layout.FindByID("sc")
	if !ok {
		t.Fatal("scroll container not found")
	}
	wheel := func(dx float32) bool {
		e := Event{Type: EventMouseScroll, MouseX: ly.Shape.X + 10,
			MouseY: ly.Shape.Y + 10, ScrollX: dx}
		w.EventFn(&e)
		if w.scrollSmooth != nil {
			driveScrollSmooth(w, 200)
		}
		return e.IsHandled
	}
	if wheel(-3) {
		t.Error("a negative ScrollX at the start edge must not move an RTL row")
	}
	if !wheel(3) {
		t.Fatal("a positive ScrollX must move an RTL row toward its overflow")
	}
	if x, _, _ := w.TestScrollOffset("sc"); x >= 0 {
		t.Errorf("offset after positive ScrollX: got %v, want < 0", x)
	}
}

// Thumb drag: the bar runs left to right in both directions, but in RTL its
// right end is the start, so a drag toward the left travels toward the
// overflow.
func TestRTLRowThumbDragMirrorsDelta(t *testing.T) {
	w, root := rtlScrollRow(t)
	sx := w.scrollX()
	if got := offsetMouseChangeX(sx, root, -10, "sc", 0); got >= 0 {
		t.Errorf("leftward thumb drag: got %v, want < 0", got)
	}
	if got := offsetMouseChangeX(sx, root, 10, "sc", 0); !f32AreClose(got, 0) {
		t.Errorf("rightward thumb drag at start edge: got %v, want 0", got)
	}
}

// Gutter click: the left end of an RTL bar is the end of the range.
func TestRTLRowGutterClickMirrorsPosition(t *testing.T) {
	w, root := rtlScrollRow(t)
	offsetFromMouseX(root, root.Shape.X, "sc", w)
	got, _ := w.scrollX().Get("sc")
	if !f32AreClose(got, -80) {
		t.Errorf("click at the left end of an RTL bar: got %v, want -80", got)
	}
}
