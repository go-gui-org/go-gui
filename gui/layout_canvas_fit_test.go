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

// Every kind of child a Canvas can hold puts the card at its measured
// size with a clickable button: the card directly, beside nil views,
// and wrapped in a Column or a nested Canvas of each Sizing. A Fill
// wrapper gets no space from the Canvas, so it takes its content size.
// The nested Fill Canvas was the last 0x0 case: its parent hands out no
// space, and the axisNone branch fitted only a Fit axis to content.
func TestCanvasChildKindsAreMeasured(t *testing.T) {
	wantW, wantH := canvasFitCardSize(t)
	column := func(cfg ContainerCfg) func(View) View {
		return func(card View) View {
			cfg.X, cfg.Y, cfg.Content = 20, 20, []View{card}
			return Column(cfg)
		}
	}
	canvas := func(sizing Sizing) func(View) View {
		return func(card View) View {
			return Canvas(ContainerCfg{X: 20, Y: 20, Sizing: sizing, Content: []View{card}})
		}
	}
	cases := []struct {
		name string
		kid  func(card View) View
	}{
		{"card", func(card View) View { return card }},
		{"nil siblings", func(card View) View { return card }},
		{"Fixed Column", column(ContainerCfg{Width: 260, Height: 160, Sizing: FixedFixed})},
		{"Fit Column", column(ContainerCfg{Sizing: FitFit})},
		{"Fill Column", column(ContainerCfg{Sizing: FillFill})},
		{"Fit Canvas", canvas(FitFit)},
		{"Fill Canvas", canvas(FillFill)},
		{"Fill/Fit Canvas", canvas(FillFit)},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			var clicks int
			win := NewTestWindow(t, WindowCfg{Width: 400, Height: 300})
			l := win.TestRender(func(*Window) View {
				content := []View{c.kid(canvasFitCard(0, 0, &clicks))}
				if c.name == "nil siblings" {
					content = []View{nil, content[0], nil}
				}
				return Canvas(ContainerCfg{Sizing: FillFill, Content: content})
			})
			assertCanvasCard(t, win, l, wantW, wantH, &clicks)
		})
	}
}

// A Canvas with no children collapses to 0, as any container does.
func TestCanvasEmptyCollapses(t *testing.T) {
	win := NewTestWindow(t, WindowCfg{Width: 400, Height: 300})
	l := win.TestRender(func(*Window) View {
		return Column(ContainerCfg{Sizing: FillFill, Content: []View{
			Canvas(ContainerCfg{ID: "cv", Sizing: FitFit}),
		}})
	})
	cv, _ := l.FindByID("cv")
	if cv.Shape.Width != 0 || cv.Shape.Height != 0 {
		t.Errorf("empty Fit Canvas = %gx%g, want 0x0", cv.Shape.Width, cv.Shape.Height)
	}
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
