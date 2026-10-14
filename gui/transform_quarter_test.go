package gui

import "testing"

// xfFindTurned returns the first layout node with a quarter turn.
func xfFindTurned(ly *Layout) *Layout {
	if ly.Shape != nil && ly.Shape.QuarterTurns > 0 {
		return ly
	}
	for i := range ly.Children {
		if f := xfFindTurned(&ly.Children[i]); f != nil {
			return f
		}
	}
	return nil
}

// A clipping container inside a quarter turn is scissored where it is
// drawn: the scissor is a window rect, so its clip is turned out to the
// window like the drawing is (#1019 phase 7). It used to be emitted in
// the unturned frame, so a 100x40 clip under a 90° turn cut a 100 wide,
// 40 tall strip across a box drawn 40 wide and 100 tall.
func TestQuarterTurnChildClipInWindowFrame(t *testing.T) {
	w := NewTestWindow(t, WindowCfg{Width: 400, Height: 300})
	w.TestRender(func(*Window) View {
		return Column(ContainerCfg{
			Sizing:  FillFill,
			Padding: PaddingNone,
			Content: []View{
				RotatedBox(RotatedBoxCfg{
					QuarterTurns: 1,
					Content: Column(ContainerCfg{
						Width: 100, Height: 40, Sizing: FixedFixed,
						Clip: true, SizeBorder: NoBorder, Padding: PaddingNone,
						Content: []View{Text(TextCfg{Text: "x"})},
					}),
				}),
			},
		})
	})
	turned := xfFindTurned(&w.layout)
	if turned == nil {
		t.Fatal("no quarter turned node")
	}
	box := turned.Shape
	inside := false
	found := false
	for _, c := range w.Renderers() {
		switch c.Kind {
		case RenderRotateBegin:
			inside = true
		case RenderRotateEnd:
			inside = false
		case RenderStencilBegin:
			if inside {
				t.Errorf("stencil under a quarter turn: a turned rect " +
					"is a rect, so the scissor is exact")
			}
		case RenderClip:
			if !inside || found {
				continue
			}
			found = true
			if !xfNear(c.X, box.X) || !xfNear(c.Y, box.Y) ||
				!xfNear(c.W, box.Width) || !xfNear(c.H, box.Height) {
				t.Errorf("clip = (%v, %v, %v, %v), want the drawn box "+
					"(%v, %v, %v, %v)", c.X, c.Y, c.W, c.H,
					box.X, box.Y, box.Width, box.Height)
			}
		}
	}
	if !found {
		t.Fatal("no clip inside the quarter turn")
	}
}

// Under a half turn, the part of the box an ancestor's clip shows is
// the other end of the unturned frame. A clip that cuts off the bottom
// half of the drawn box shows the unturned bottom child, so that child
// keeps its clip and takes the pointer, and the top child does not.
func TestHalfTurnChildClipFollowsTurn(t *testing.T) {
	w := NewTestWindow(t, WindowCfg{Width: 400, Height: 300})
	clicked := ""
	btn := func(id string) View {
		return Button(ButtonCfg{
			ID: id, Width: 100, Height: 50, Sizing: FixedFixed,
			OnClick: func(EventCtx) { clicked = id },
		})
	}
	view := func(*Window) View {
		return Column(ContainerCfg{
			Sizing:  FillFill,
			Padding: PaddingNone,
			Content: []View{
				Column(ContainerCfg{
					Width: 100, Height: 50, Sizing: FixedFixed,
					Clip: true, SizeBorder: NoBorder, Padding: PaddingNone,
					Content: []View{
						RotatedBox(RotatedBoxCfg{
							QuarterTurns: 2,
							Content: Column(ContainerCfg{
								Sizing: FitFit, SizeBorder: NoBorder,
								Padding: PaddingNone, Spacing: NoSpacing,
								Content: []View{btn("a"), btn("b")},
							}),
						}),
					},
				}),
			},
		})
	}
	w.TestRender(view)
	a, b := mustShape(t, w, "a"), mustShape(t, w, "b")
	if b.shapeClip.Width <= 0 || b.shapeClip.Height <= 0 {
		t.Errorf("b clip = %+v, want the shown half", b.shapeClip)
	}
	if a.shapeClip.Width > 0 && a.shapeClip.Height > 0 {
		t.Errorf("a clip = %+v, want empty: it is drawn in the cut half",
			a.shapeClip)
	}
	if err := w.TestClick("b"); err != nil {
		t.Fatalf("TestClick(b): %v", err)
	}
	if clicked != "b" {
		t.Fatalf("clicked = %q, want b", clicked)
	}
}

// The render walk opens a frame for a quarter turn, so a box drawn
// inside it (the IME caret rect) maps out through the turn.
func TestQuarterTurnRenderFrameMapsBox(t *testing.T) {
	w := newTestWindow()
	s := &Shape{X: 0, Y: 0, Width: 40, Height: 100, QuarterTurns: 1}
	childClip := renderQuarterTurnBegin(s, drawClip{Width: 40, Height: 100}, w)
	// The box's clip turned back: 100 wide and 40 tall about the same
	// center.
	if !xfNear(childClip.X, -30) || !xfNear(childClip.Y, 30) ||
		!xfNear(childClip.Width, 100) || !xfNear(childClip.Height, 40) {
		t.Errorf("child clip = %+v, want (-30, 30) 100x40", childClip)
	}
	// The unturned frame's top-left corner (-30, 30) is drawn at the
	// top-right corner of the box, (40, 0).
	got := renderBoxToWindow(drawClip{X: -30, Y: 30, Width: 2, Height: 16}, w)
	want := drawClip{X: 24, Y: 0, Width: 16, Height: 2}
	if !xfNear(got.X, want.X) || !xfNear(got.Y, want.Y) ||
		!xfNear(got.Width, want.Width) || !xfNear(got.Height, want.Height) {
		t.Errorf("box = %+v, want %+v", got, want)
	}
	renderQuarterTurnEnd(w)
	if len(w.xfStack) != 0 {
		t.Fatalf("xfStack len = %d after End, want 0", len(w.xfStack))
	}
}

// A gesture over a quarter turned child reaches it: the centroid is
// mapped into the unturned frame the same way the pointer is.
func TestQuarterTurnGestureCentroid(t *testing.T) {
	got := false
	child := &Shape{
		X: -30, Y: 30, Width: 100, Height: 40,
		shapeClip: drawClip{X: -30, Y: 30, Width: 100, Height: 40},
		events: &eventHandlers{OnGesture: func(ctx EventCtx) {
			got = true
			ctx.Consume()
		}},
	}
	root := &Layout{
		Shape: &Shape{
			Width: 40, Height: 100, QuarterTurns: 1,
			shapeClip: drawClip{Width: 40, Height: 100},
		},
		Children: []Layout{{Shape: child}},
	}
	// (35, 5) is in the drawn box, and in the child only once turned.
	e := &Event{CentroidX: 35, CentroidY: 5}
	gestureHandler(root, e, newTestWindow())
	if !got {
		t.Fatal("gesture did not reach the turned child")
	}
	if e.CentroidX != 35 || e.CentroidY != 5 {
		t.Errorf("centroid = (%v, %v) after dispatch, want (35, 5)",
			e.CentroidX, e.CentroidY)
	}
}
