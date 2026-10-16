package gui

import "testing"

// stackBox is a fixed-size, borderless, unpadded child for the Stack tests,
// so the sizes the tests assert are the sizes the children ask for.
func stackBox(id string, width, height float32) View {
	return Column(ContainerCfg{
		ID: id, Sizing: FixedFixed, Width: width, Height: height,
		Padding: PaddingNone, SizeBorder: NoBorder,
	})
}

// renderStack renders cfg as a Stack inside a FillFill root column and
// returns the arranged stack and its children by ID.
func renderStack(t *testing.T, cfg ContainerCfg) *Window {
	t.Helper()
	w := NewTestWindow(t, WindowCfg{Width: 400, Height: 300})
	w.TestRender(func(_ *Window) View {
		return Column(ContainerCfg{
			Sizing: FillFill, Padding: PaddingNone, SizeBorder: NoBorder,
			Content: []View{Stack(cfg)},
		})
	})
	return w
}

func stackFind(t *testing.T, w *Window, id string) *Shape {
	t.Helper()
	ly, ok := w.layout.FindByID(id)
	if !ok {
		t.Fatalf("%q not found", id)
	}
	return ly.Shape
}

// A Fit stack takes the largest child on each axis, not the sum.
func TestStackFitTakesLargestChild(t *testing.T) {
	w := renderStack(t, ContainerCfg{
		ID: "s", Padding: PaddingNone, SizeBorder: NoBorder,
		Content: []View{stackBox("a", 100, 40), stackBox("b", 60, 80)},
	})
	s := stackFind(t, w, "s")
	nearF(t, "width", s.Width, 100, 0.01)
	nearF(t, "height", s.Height, 80, 0.01)
}

// Padding adds to the Fit size on both axes.
func TestStackFitAddsPadding(t *testing.T) {
	w := renderStack(t, ContainerCfg{
		ID: "s", Padding: PadAll(10), SizeBorder: NoBorder,
		Content: []View{stackBox("a", 100, 40), stackBox("b", 60, 80)},
	})
	s := stackFind(t, w, "s")
	nearF(t, "width", s.Width, 120, 0.01)
	nearF(t, "height", s.Height, 100, 0.01)
}

// Every child starts at the content origin: children overlap, they do not
// follow one another.
func TestStackChildrenShareOrigin(t *testing.T) {
	w := renderStack(t, ContainerCfg{
		ID: "s", Padding: PadAll(10), SizeBorder: NoBorder,
		Content: []View{stackBox("a", 100, 40), stackBox("b", 60, 80)},
	})
	s := stackFind(t, w, "s")
	for _, id := range []string{"a", "b"} {
		c := stackFind(t, w, ScopeID("s", id))
		nearF(t, id+" x", c.X, s.X+10, 0.01)
		nearF(t, id+" y", c.Y, s.Y+10, 0.01)
	}
}

// Spacing is ignored: it neither grows the Fit size nor moves a child.
func TestStackIgnoresSpacing(t *testing.T) {
	w := renderStack(t, ContainerCfg{
		ID: "s", Padding: PaddingNone, SizeBorder: NoBorder, Spacing: SpacingPx(50),
		Content: []View{stackBox("a", 50, 50), stackBox("b", 50, 50)},
	})
	s := stackFind(t, w, "s")
	nearF(t, "width", s.Width, 50, 0.01)
	nearF(t, "height", s.Height, 50, 0.01)
	b := stackFind(t, w, ScopeID("s", "b"))
	nearF(t, "b x", b.X, s.X, 0.01)
	nearF(t, "b y", b.Y, s.Y, 0.01)
}

// A Fill child takes the stack's content box on both axes. This is the
// background-image case.
func TestStackFillChildTakesContentBox(t *testing.T) {
	w := renderStack(t, ContainerCfg{
		ID: "s", Sizing: FixedFixed, Width: 200, Height: 100,
		Padding: PadAll(10), SizeBorder: NoBorder,
		Content: []View{
			Column(ContainerCfg{
				ID: "bg", Sizing: FillFill, Padding: PaddingNone, SizeBorder: NoBorder,
			}),
			stackBox("fg", 40, 20),
		},
	})
	s := stackFind(t, w, "s")
	bg := stackFind(t, w, ScopeID("s", "bg"))
	nearF(t, "bg width", bg.Width, 180, 0.01)
	nearF(t, "bg height", bg.Height, 80, 0.01)
	nearF(t, "bg x", bg.X, s.X+10, 0.01)
	nearF(t, "bg y", bg.Y, s.Y+10, 0.01)
}

// HAlign and VAlign place each child on both axes.
func TestStackAlignsEachChild(t *testing.T) {
	cases := []struct {
		name   string
		hAlign HorizontalAlign
		vAlign verticalAlign
		x, y   float32
	}{
		{"start-top", HAlignStart, VAlignTop, 0, 0},
		{"center-middle", HAlignCenter, VAlignMiddle, 80, 40},
		{"right-bottom", HAlignRight, VAlignBottom, 160, 80},
		{"end-bottom", HAlignEnd, VAlignBottom, 160, 80},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			w := renderStack(t, ContainerCfg{
				ID: "s", Sizing: FixedFixed, Width: 200, Height: 100,
				Padding: PaddingNone, SizeBorder: NoBorder,
				HAlign: tc.hAlign, VAlign: tc.vAlign,
				Content: []View{stackBox("a", 40, 20)},
			})
			s := stackFind(t, w, "s")
			a := stackFind(t, w, ScopeID("s", "a"))
			nearF(t, "x", a.X-s.X, tc.x, 0.01)
			nearF(t, "y", a.Y-s.Y, tc.y, 0.01)
		})
	}
}

// In RTL, HAlignStart is the right edge, as in a column.
func TestStackRTLStartIsRight(t *testing.T) {
	w := renderStack(t, ContainerCfg{
		ID: "s", Sizing: FixedFixed, Width: 200, Height: 100,
		Padding: PaddingNone, SizeBorder: NoBorder, TextDir: TextDirRTL,
		Content: []View{stackBox("a", 40, 20)},
	})
	s := stackFind(t, w, "s")
	a := stackFind(t, w, ScopeID("s", "a"))
	nearF(t, "x", a.X-s.X, 160, 0.01)
}

// The last child is on top: it gets the press where the layers overlap.
func TestStackLastChildTakesClick(t *testing.T) {
	var bottom, top int
	w := NewTestWindow(t, WindowCfg{Width: 400, Height: 300})
	w.TestRender(func(_ *Window) View {
		return Column(ContainerCfg{
			Sizing: FillFill, Padding: PaddingNone, SizeBorder: NoBorder,
			Content: []View{Stack(ContainerCfg{
				ID: "s", Sizing: FixedFixed, Width: 200, Height: 100,
				Padding: PaddingNone, SizeBorder: NoBorder,
				Content: []View{
					Column(ContainerCfg{
						ID: "bottom", Sizing: FillFill,
						OnClick: func(ctx EventCtx) { bottom++; ctx.Consume() },
					}),
					Column(ContainerCfg{
						ID: "top", Sizing: FillFill,
						OnClick: func(ctx EventCtx) { top++; ctx.Consume() },
					}),
				},
			})},
		})
	})
	if err := w.TestClick(ScopeID("s", "bottom")); err != nil {
		t.Fatalf("TestClick: %v", err)
	}
	if top != 1 || bottom != 0 {
		t.Fatalf("top=%d bottom=%d, want top=1 bottom=0", top, bottom)
	}
}

// A Float child is still out of flow: it does not size the stack.
func TestStackFloatChildOutOfFlow(t *testing.T) {
	w := renderStack(t, ContainerCfg{
		ID: "s", Padding: PaddingNone, SizeBorder: NoBorder,
		Content: []View{
			stackBox("a", 50, 50),
			Column(ContainerCfg{
				ID: "f", Float: true, Sizing: FixedFixed, Width: 300, Height: 200,
			}),
		},
	})
	s := stackFind(t, w, "s")
	nearF(t, "width", s.Width, 50, 0.01)
	nearF(t, "height", s.Height, 50, 0.01)
}

// A short wrapped text in a centered Stack is centered, as in a centered
// column (#577): the text box shrinks to its longest line, so HAlign has
// room to move it.
func TestStackCentersWrappedText(t *testing.T) {
	w := &Window{}
	w.textMeasurer = &stubTextMeasurer{charWidth: 10, fontHeight: 20}
	w.windowWidth = 400
	w.windowHeight = 300
	st := generateViewLayout(Stack(ContainerCfg{
		Sizing: FixedFixed, Width: 400, Height: 300, HAlign: HAlignCenter,
		Padding: PaddingNone, SizeBorder: NoBorder,
		Content: []View{Text(TextCfg{
			Text: "wrap me", TextStyle: TextStyle{Size: 16}, Mode: TextModeWrap,
		})},
	}), w)
	st.Shape.MinWidth, st.Shape.MaxWidth = 400, 400
	st.Shape.MinHeight, st.Shape.MaxHeight = 300, 300
	layoutParents(&st, nil)
	layoutPipeline(&st, w)

	text := st.Children[0].Shape
	if text.Width >= st.Shape.Width {
		t.Fatalf("text width = %.1f, want < stack width %.1f", text.Width, st.Shape.Width)
	}
	wantX := (st.Shape.Width - text.Width) / 2
	if abs32(text.X-wantX) > 1 {
		t.Errorf("text X = %.1f, want ~%.1f", text.X, wantX)
	}
}

// The re-fit after a rotation or a wrap (recomputeFitWidth/Height) must
// size a Fit Stack by the same rule as the sizing pass: the largest
// child, not the axisNone rule of child position plus extent. Child X/Y
// are non-zero here (parent-relative values from an earlier pass) so the
// two rules give different answers.
func TestStackRefitMatchesSizingPass(t *testing.T) {
	layout := &Layout{
		Shape: &Shape{shapeType: shapeRectangle, Axis: axisStack, Sizing: FitFit},
		Children: []Layout{
			{Shape: &Shape{shapeType: shapeRectangle, X: 30, Y: 20, Width: 40, Height: 10}},
			{Shape: &Shape{shapeType: shapeRectangle, X: 5, Y: 5, Width: 20, Height: 25}},
		},
	}
	nearF(t, "re-fit width", recomputeFitWidth(layout), 40, 0.01)
	nearF(t, "re-fit height", recomputeFitHeight(layout), 25, 0.01)
}

// An empty Fit stack is 0x0, padding included. An empty column is 0
// wide and an empty row 0 tall: the cross-axis Fit pass adds padding
// only around a child. A Stack takes that pass on both axes.
func TestStackEmptyFitIsZero(t *testing.T) {
	w := renderStack(t, ContainerCfg{
		ID: "s", Padding: PadAll(10), SizeBorder: NoBorder,
	})
	s := stackFind(t, w, "s")
	nearF(t, "width", s.Width, 0, 0.01)
	nearF(t, "height", s.Height, 0, 0.01)
}

// A child larger than a Fixed stack has no slack to align in: under a
// centered or end alignment it stays at the content origin, as in a row
// or column, rather than moving up and left of it.
func TestStackOversizeChildPinsToOrigin(t *testing.T) {
	w := renderStack(t, ContainerCfg{
		ID: "s", Sizing: FixedFixed, Width: 50, Height: 40,
		Padding: PaddingNone, SizeBorder: NoBorder,
		HAlign: HAlignEnd, VAlign: VAlignBottom,
		Content: []View{stackBox("a", 100, 80)},
	})
	s := stackFind(t, w, "s")
	a := stackFind(t, w, ScopeID("s", "a"))
	nearF(t, "x", a.X, s.X, 0.01)
	nearF(t, "y", a.Y, s.Y, 0.01)
}

// The inspector names a Stack's shape "stack", not a column, row or
// canvas.
func TestStackInspectorTypeName(t *testing.T) {
	w := renderStack(t, ContainerCfg{ID: "s"})
	if got := inspectorTypeName(stackFind(t, w, "s")); got != "stack" {
		t.Fatalf("inspectorTypeName = %q, want %q", got, "stack")
	}
}
