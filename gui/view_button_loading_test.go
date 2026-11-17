package gui

import "testing"

// Tests for ButtonCfg.Loading (#1264). The busy button must keep its
// size, refuse the click, keep focus and its colors, say "busy" to
// assistive tech, and draw a spinner in place of the label.

// loadingWindow returns a test window that can measure text and load an
// SVG. Without the measurer every label has the fallback width, which
// would hide a width jump; without the parser the spinner falls back to
// a "[missing: …]" text that would itself change the width.
func loadingWindow(t *testing.T) *Window {
	t.Helper()
	w := NewTestWindow(t, WindowCfg{})
	w.textMeasurer = &stubTextMeasurer{charWidth: 8, fontHeight: 19}
	w.SetSvgParser(&mockSvgParser{width: 24, height: 24})
	return w
}

// renderLoadingButton renders one button with ID "b" at the window
// origin, so its geometry is not stretched by a Fill parent.
func renderLoadingButton(t *testing.T, cfg ButtonCfg) *Window {
	t.Helper()
	w := loadingWindow(t)
	cfg.ID = "b"
	cfg.Label = "Submit"
	w.TestRender(func(*Window) View {
		return Column(ContainerCfg{
			Sizing:     FillFill,
			Padding:    NoPadding,
			SizeBorder: NoBorder,
			Content:    []View{Button(cfg)},
		})
	})
	return w
}

// findShapes collects every shape of type st below l, in pre-order.
func findShapes(l *Layout, st shapeType, out []*Shape) []*Shape {
	for i := range l.Children {
		c := &l.Children[i]
		if c.Shape != nil && c.Shape.shapeType == st {
			out = append(out, c.Shape)
		}
		out = findShapes(c, st, out)
	}
	return out
}

func mustLayout(t *testing.T, w *Window, id string) *Layout {
	t.Helper()
	ly, ok := w.layout.FindByID(id)
	if !ok {
		t.Fatalf("no shape with effective ID %s", id)
	}
	return ly
}

func TestButtonLoadingKeepsSize(t *testing.T) {
	idle := mustShape(t, renderLoadingButton(t, ButtonCfg{}), "b")
	busy := mustShape(t, renderLoadingButton(t, ButtonCfg{Loading: true}), "b")
	if busy.Width != idle.Width || busy.Height != idle.Height {
		t.Errorf("loading size = %vx%v, want %vx%v (no width jump)",
			busy.Width, busy.Height, idle.Width, idle.Height)
	}
}

func TestButtonLoadingBlocksClick(t *testing.T) {
	clicks := 0
	w := renderLoadingButton(t, ButtonCfg{
		Loading: true,
		OnClick: func(ctx EventCtx) { clicks++; ctx.Consume() },
	})
	// TestClick accepts a focusable target with no handler, so an error
	// here would mean the button lost its focusability, not its click.
	if err := w.TestClick("b"); err != nil {
		t.Fatalf("TestClick: %v", err)
	}
	if err := w.TestKey("b", KeySpace, 0); err != nil {
		t.Fatalf("TestKey space: %v", err)
	}
	if err := w.TestKey("b", KeyEnter, 0); err != nil {
		t.Fatalf("TestKey enter: %v", err)
	}
	if clicks != 0 {
		t.Errorf("OnClick fired %d times while loading, want 0", clicks)
	}
}

func TestButtonLoadingStaysFocusable(t *testing.T) {
	w := renderLoadingButton(t, ButtonCfg{
		Loading: true,
		OnClick: func(EventCtx) {},
	})
	if !mustShape(t, w, "b").Focusable {
		t.Fatal("loading button is not focusable")
	}
	if err := w.TestFocus("b"); err != nil {
		t.Fatalf("TestFocus: %v", err)
	}
	if !w.IsFocus("b") {
		t.Error("loading button did not take focus")
	}
}

func TestButtonLoadingA11yBusy(t *testing.T) {
	w := renderLoadingButton(t, ButtonCfg{
		Loading:   true,
		OnClick:   func(EventCtx) {},
		A11YState: AccessStateExpanded,
	})
	s := mustShape(t, w, "b")
	if !s.A11YState.Has(AccessStateBusy) {
		t.Errorf("A11YState = %v, want AccessStateBusy set", s.A11YState)
	}
	if !s.A11YState.Has(AccessStateExpanded) {
		t.Errorf("A11YState = %v, caller's state lost", s.A11YState)
	}
	if s.Disabled {
		t.Error("loading button is Disabled, want enabled look")
	}

	idle := mustShape(t, renderLoadingButton(t, ButtonCfg{}), "b")
	if idle.A11YState.Has(AccessStateBusy) {
		t.Error("idle button reports busy")
	}
}

// The spinner is decoration: AccessStateBusy on the button already says
// what it shows. As an Svg it would add an unnamed image node, which a
// screen reader reads out as "image" inside the button.
func TestButtonLoadingSpinnerHasNoA11yNode(t *testing.T) {
	w := renderLoadingButton(t, ButtonCfg{Loading: true, OnClick: func(EventCtx) {}})
	var nodes []A11yNode
	var live []liveNode
	a11yCollect(&w.layout, -1, &nodes, "", &live)
	for _, n := range nodes {
		if n.Role == AccessRoleImage {
			t.Errorf("loading button has an image a11y node %+v", n)
		}
	}
	if len(nodes) != 2 {
		t.Errorf("got %d a11y nodes, want 2 (button, label)", len(nodes))
	}
}

func TestButtonLoadingKeepsColors(t *testing.T) {
	cfg := ButtonCfg{
		OnClick: func(EventCtx) {},
		Colors: ColorSet{
			Base:     RGB(10, 20, 30),
			Disabled: RGB(200, 0, 0),
		},
	}
	idle := mustShape(t, renderLoadingButton(t, cfg), "b").Color
	cfg.Loading = true
	busy := mustShape(t, renderLoadingButton(t, cfg), "b").Color
	if busy != idle {
		t.Errorf("loading fill = %+v, want resting %+v", busy, idle)
	}
}

func TestButtonLoadingHoverKeepsHoverColor(t *testing.T) {
	hover := RGB(40, 200, 40)
	w := renderLoadingButton(t, ButtonCfg{
		Loading:       true,
		FocusDisabled: true,
		OnClick:       func(EventCtx) {},
		Colors:        ColorSet{Base: RGB(10, 10, 10), Hover: hover},
	})
	hoverOver(t, w, "b")
	if c := mustShape(t, w, "b").Color; c != hover {
		t.Errorf("hovered loading fill = %+v, want hover %+v", c, hover)
	}
}

// A click does nothing while loading, so the hover pass must not offer
// the hand cursor that promises one. The idle button is the control.
func TestButtonLoadingHoverNoHandCursor(t *testing.T) {
	for _, loading := range []bool{false, true} {
		w := renderLoadingButton(t, ButtonCfg{
			Loading:       loading,
			FocusDisabled: true,
			OnClick:       func(EventCtx) {},
		})
		w.viewState.mouseCursor = CursorDefault
		hoverOver(t, w, "b")
		hand := w.viewState.mouseCursor == CursorPointingHand
		if hand == loading {
			t.Errorf("loading=%v: hand cursor = %v, want %v",
				loading, hand, !loading)
		}
	}
}

// Caller Content can nest (an icon and a text in their own Row). Opacity
// is not inherited, so every shape at every depth must be hidden, not
// only the layer's direct children.
func TestButtonLoadingHidesNestedContent(t *testing.T) {
	w := renderLoadingButton(t, ButtonCfg{
		Loading: true,
		Content: []View{Row(ContainerCfg{
			Padding:    NoPadding,
			SizeBorder: NoBorder,
			Content: []View{Column(ContainerCfg{
				Padding:    NoPadding,
				SizeBorder: NoBorder,
				Content:    []View{Text(TextCfg{Text: "Deep"})},
			})},
		})},
	})
	// The helper's "Submit" label sits beside the nested "Deep".
	texts := findShapes(mustLayout(t, w, "b"), shapeText, nil)
	if len(texts) != 2 {
		t.Fatalf("got %d text shapes, want 2 (label and nested)", len(texts))
	}
	for _, s := range texts {
		if s.Opacity != 0 {
			t.Errorf("text %q opacity = %v, want 0", s.TC.Text, s.Opacity)
		}
	}
}

func TestButtonLoadingHidesLabelShowsSpinner(t *testing.T) {
	w := renderLoadingButton(t, ButtonCfg{Loading: true})
	ly := mustLayout(t, w, "b")

	texts := findShapes(ly, shapeText, nil)
	if len(texts) != 1 {
		t.Fatalf("got %d text shapes, want 1 (the label)", len(texts))
	}
	if texts[0].TC.Text != "Submit" {
		t.Errorf("label text = %q, want Submit", texts[0].TC.Text)
	}
	if texts[0].Opacity != 0 {
		t.Errorf("label opacity = %v, want 0 (laid out, not drawn)",
			texts[0].Opacity)
	}
	svgs := findShapes(ly, shapeSVG, nil)
	if len(svgs) != 1 {
		t.Fatalf("got %d svg shapes, want 1 (the spinner)", len(svgs))
	}
	if svgs[0].Opacity != 1 {
		t.Errorf("spinner opacity = %v, want 1", svgs[0].Opacity)
	}
}

func TestButtonNotLoadingHasNoSpinner(t *testing.T) {
	ly := mustLayout(t, renderLoadingButton(t, ButtonCfg{}), "b")
	if svgs := findShapes(ly, shapeSVG, nil); len(svgs) != 0 {
		t.Errorf("idle button has %d svg shapes, want 0", len(svgs))
	}
	texts := findShapes(ly, shapeText, nil)
	if len(texts) != 1 || texts[0].Opacity != 1 {
		t.Errorf("idle label not drawn at full opacity: %d shapes", len(texts))
	}
}

// The label moves one level down when loading (into the Stack). The
// optical correction must still reach it, or the hidden label sits a
// pixel off and, more visibly, the label jumps when Loading ends.
func TestButtonLoadingKeepsLabelPosition(t *testing.T) {
	labelXY := func(loading bool) (float32, float32) {
		ly := mustLayout(t, renderLoadingButton(t, ButtonCfg{Loading: loading}), "b")
		texts := findShapes(ly, shapeText, nil)
		if len(texts) != 1 {
			t.Fatalf("loading=%v: got %d text shapes, want 1", loading, len(texts))
		}
		return texts[0].X, texts[0].Y
	}
	ix, iy := labelXY(false)
	bx, by := labelXY(true)
	if ix != bx || iy != by {
		t.Errorf("label at (%v,%v) loading, (%v,%v) idle; want equal", bx, by, ix, iy)
	}
}
