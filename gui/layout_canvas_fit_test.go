package gui

import "testing"

// Issue #1216: the measuring passes did not recurse into a Canvas's
// children, so a Fit container below a Canvas, at any depth, stayed
// 0x0: its background did not draw and nothing in it could be clicked.

// canvasFitCard is a Fit column of a text and a button, the shape of
// the examples' dead buttons (minesweeper, snake, 2048).
func canvasFitCard(x, y float32, clicks *int) View {
	return Column(ContainerCfg{
		ID: "card", X: x, Y: y, Sizing: FitFit,
		Content: []View{
			Text(TextCfg{Text: "Title"}),
			Button(ButtonCfg{
				ID:      "ok",
				Content: []View{Text(TextCfg{Text: "OK"})},
				OnClick: func(ctx EventCtx) { *clicks++; ctx.Consume() },
			}),
		},
	})
}

// canvasFitCardSize renders the card in a Column, where it has always
// been measured, and returns the size a Canvas must give it too.
func canvasFitCardSize(t *testing.T) (w, h float32) {
	t.Helper()
	var clicks int
	win := NewTestWindow(t, WindowCfg{Width: 400, Height: 300})
	l := win.TestRender(func(*Window) View {
		return Column(ContainerCfg{Sizing: FillFill,
			Content: []View{canvasFitCard(0, 0, &clicks)}})
	})
	card, _ := l.FindByID("card")
	if card.Shape.Width <= 0 || card.Shape.Height <= 0 {
		t.Fatalf("card in a Column is %gx%g", card.Shape.Width, card.Shape.Height)
	}
	return card.Shape.Width, card.Shape.Height
}

// assertCanvasCard checks that the card has its measured size and that
// its button takes a click.
func assertCanvasCard(t *testing.T, win *Window, l *Layout, wantW, wantH float32, clicks *int) {
	t.Helper()
	card, ok := l.FindByID(win.ResolveID("card")[0])
	if !ok {
		t.Fatal("card not found")
	}
	nearF(t, "card width", card.Shape.Width, wantW, 0.01)
	nearF(t, "card height", card.Shape.Height, wantH, 0.01)
	if err := win.TestClick(win.ResolveID("ok")[0]); err != nil {
		t.Fatalf("TestClick: %v", err)
	}
	if *clicks != 1 {
		t.Errorf("clicks = %d, want 1", *clicks)
	}
}

func TestCanvasFitChildIsMeasured(t *testing.T) {
	wantW, wantH := canvasFitCardSize(t)
	var clicks int
	win := NewTestWindow(t, WindowCfg{Width: 400, Height: 300})
	l := win.TestRender(func(*Window) View {
		return Canvas(ContainerCfg{Sizing: FillFill,
			Content: []View{canvasFitCard(20, 20, &clicks)}})
	})
	assertCanvasCard(t, win, l, wantW, wantH, &clicks)
}

// The gap was not limited to direct children: a Fixed box in a Canvas
// got its explicit size, and a Fit container inside it was 0x0 again.
func TestCanvasFitChildInFixedBoxIsMeasured(t *testing.T) {
	wantW, wantH := canvasFitCardSize(t)
	var clicks int
	win := NewTestWindow(t, WindowCfg{Width: 400, Height: 300})
	l := win.TestRender(func(*Window) View {
		return Canvas(ContainerCfg{Sizing: FillFill, Content: []View{
			Column(ContainerCfg{
				X: 20, Y: 20, Width: 260, Height: 160, Sizing: FixedFixed,
				Content: []View{canvasFitCard(0, 0, &clicks)},
			}),
		}})
	})
	assertCanvasCard(t, win, l, wantW, wantH, &clicks)
}

// A Fit Canvas encloses its children at their own X/Y, so it reads
// their measured size: with the card unmeasured it enclosed only the
// card's offset.
func TestCanvasFitEnclosesMeasuredChild(t *testing.T) {
	wantW, wantH := canvasFitCardSize(t)
	var clicks int
	win := NewTestWindow(t, WindowCfg{Width: 400, Height: 300})
	l := win.TestRender(func(*Window) View {
		return Column(ContainerCfg{Sizing: FillFill, Content: []View{
			Canvas(ContainerCfg{
				ID: "cv", Sizing: FitFit, Padding: NoPadding, SizeBorder: NoBorder,
				Content: []View{canvasFitCard(20, 30, &clicks)},
			}),
		}})
	})
	cv, _ := l.FindByID("cv")
	nearF(t, "canvas width", cv.Shape.Width, 20+wantW, 0.01)
	nearF(t, "canvas height", cv.Shape.Height, 30+wantH, 0.01)
}

// A Canvas with a layoutSizer (Splitter, DockLayout) sizes its children
// itself, after the sizing passes. The measuring passes leave them
// alone, so its panes are not measured once to be thrown away.
func TestCanvasWithSizerChildrenNotMeasured(t *testing.T) {
	child := Layout{Shape: &Shape{shapeType: shapeRectangle, Axis: axisTopToBottom, Sizing: FitFit}}
	child.Children = []Layout{{Shape: &Shape{shapeType: shapeRectangle,
		Width: 50, Height: 20, MinWidth: 50, MinHeight: 20}}}
	canvas := Layout{
		Shape:    &Shape{Axis: axisNone, Sizing: FixedFixed, Width: 300, Height: 200},
		Children: []Layout{child},
	}
	canvas.Shape.events = &eventHandlers{sizer: &splitterCore{}}

	layoutWidths(&canvas)
	layoutHeights(&canvas)
	if s := canvas.Children[0].Shape; s.Width != 0 || s.Height != 0 {
		t.Errorf("sizer's child measured to %gx%g, want 0x0 (left to the sizer)",
			s.Width, s.Height)
	}

	canvas.Shape.events = nil
	layoutWidths(&canvas)
	layoutHeights(&canvas)
	if s := canvas.Children[0].Shape; s.Width != 50 || s.Height != 20 {
		t.Errorf("plain Canvas child = %gx%g, want 50x20", s.Width, s.Height)
	}
}
