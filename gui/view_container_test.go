package gui

import "testing"

func TestContainerIDScrollAutoAppendsScrollbars(t *testing.T) {
	v := Column(ContainerCfg{
		Scrollable: true,
		ID:         "test-scroll-1",
		Content:    []View{Rectangle(RectangleCfg{})},
	})
	cv := v.(*containerView)
	// 1 user child + 2 scrollbars = 3
	if len(cv.content) != 3 {
		t.Errorf("expected 3 children, got %d", len(cv.content))
	}
}

func TestContainerScrollbarCfgXHiddenSuppresses(t *testing.T) {
	v := Column(ContainerCfg{
		Scrollable:    true,
		ID:            "test-scroll-2",
		ScrollbarCfgX: &ScrollbarCfg{Overflow: ScrollbarHidden},
		Content:       []View{Rectangle(RectangleCfg{})},
	})
	cv := v.(*containerView)
	// 1 user child + 0 horizontal + 1 vertical = 2
	if len(cv.content) != 2 {
		t.Errorf("expected 2 children, got %d", len(cv.content))
	}
}

func TestContainerScrollbarCfgYOverrides(t *testing.T) {
	customThumb := RGB(255, 0, 0)
	v := Column(ContainerCfg{
		Scrollable:    true,
		ID:            "test-scroll-3",
		ScrollbarCfgY: &ScrollbarCfg{ColorThumb: customThumb},
		Content:       []View{Rectangle(RectangleCfg{})},
	})
	cv := v.(*containerView)
	// 1 user child + 1 horizontal (default) + 1 vertical (custom) = 3
	if len(cv.content) != 3 {
		t.Errorf("expected 3 children, got %d", len(cv.content))
	}
}

func TestContainerNoIDScrollNoScrollbars(t *testing.T) {
	v := Column(ContainerCfg{
		Content: []View{Rectangle(RectangleCfg{})},
	})
	cv := v.(*containerView)
	if len(cv.content) != 1 {
		t.Errorf("expected 1 child, got %d", len(cv.content))
	}
}

func TestContainerAnimSnapReachesShape(t *testing.T) {
	// The mask is only useful if app code can set it; ContainerCfg is
	// the write-through seam (the same one Hero uses).
	v := Column(ContainerCfg{ID: "snap", AnimSnap: AnimSnapSize})
	got := v.GenerateLayout(&Window{}).Shape.AnimSnap
	if got != AnimSnapSize {
		t.Errorf("AnimSnap = %d, want %d", got, AnimSnapSize)
	}
}

func TestContainerGenerateLayoutShapeIsolation(t *testing.T) {
	v := Row(ContainerCfg{
		Content: []View{Text(TextCfg{Text: "a"})},
	})
	w := &Window{}

	l1 := generateViewLayout(v, w)
	l1.Shape.X = 100
	l1.Shape.Y = 200
	l1.Shape.Width = 300

	l2 := generateViewLayout(v, w)
	if l2.Shape.X != 0 {
		t.Errorf("X leaked across frames: got %g, want 0", l2.Shape.X)
	}
	if l2.Shape.Y != 0 {
		t.Errorf("Y leaked across frames: got %g, want 0", l2.Shape.Y)
	}
	if l2.Shape.Width != 0 {
		t.Errorf("Width leaked across frames: got %g, want 0", l2.Shape.Width)
	}
}

// OnMouseDown fires on the press and is consume-class: TestClick's
// release must not re-fire it, and a consuming handler stops the press
// from reaching any ancestor handler.
func TestContainerOnMouseDownFiresOnPress(t *testing.T) {
	pressed := 0
	released := 0
	w := NewTestWindow(WindowCfg{})
	w.TestRender(func(w *Window) View {
		return Column(ContainerCfg{
			ID:     "pad",
			Sizing: FillFill,
			OnMouseDown: func(ctx EventCtx) {
				pressed++
				ctx.Consume()
			},
			OnMouseUp: func(ctx EventCtx) { released++ },
		})
	})
	if err := w.TestClick("pad"); err != nil {
		t.Fatalf("TestClick(pad) = %v, want nil", err)
	}
	if pressed != 1 {
		t.Errorf("OnMouseDown fired %d times after one click, want 1", pressed)
	}
	if released != 1 {
		t.Errorf("OnMouseUp fired %d times after one click, want 1", released)
	}
}

// A titled container is a group box: an unset ColorBorder resolves the
// group-box ink (visual-refresh §4.1), and an unset SizeBorder honors
// the themed container style — a theme that states a border keeps it.
func TestGroupBoxResolvesInkAndThemedBorder(t *testing.T) {
	restoreTheme(t)
	SetTheme(ThemeDark.withContainerStyle(containerStyle{SizeBorder: 7}))

	w := &Window{}
	layout := generateViewLayout(Column(ContainerCfg{Title: "Group"}), w)
	if layout.Shape.ColorBorder != groupBoxInk() {
		t.Errorf("border = %v, want group-box ink %v",
			layout.Shape.ColorBorder, groupBoxInk())
	}
	if layout.Shape.SizeBorder != 7 {
		t.Errorf("size_border = %v, want 7 (themed container style)",
			layout.Shape.SizeBorder)
	}
}

// The light presets default the container border to 0, which would
// leave a group box edge-less; the titled container forces the
// hairline in that case alone.
func TestGroupBoxForcesHairlineWhenThemeHasNoBorder(t *testing.T) {
	restoreTheme(t)
	SetTheme(ThemeLight)

	w := &Window{}
	layout := generateViewLayout(Column(ContainerCfg{Title: "Group"}), w)
	if layout.Shape.SizeBorder != sizeBorderDef {
		t.Errorf("size_border = %v, want hairline %v",
			layout.Shape.SizeBorder, sizeBorderDef)
	}
}

// Caller-explicit border values win over both the ink and the forced
// hairline, including NoBorder, which must be distinguishable from
// "unset" (gui/opt.go).
func TestGroupBoxExplicitBorderWins(t *testing.T) {
	restoreTheme(t)
	SetTheme(ThemeLight)

	w := &Window{}
	layout := generateViewLayout(Column(ContainerCfg{
		Title:       "Group",
		ColorBorder: Hex(0xff00ff),
		SizeBorder:  NoBorder,
	}), w)
	if layout.Shape.ColorBorder != Hex(0xff00ff) {
		t.Errorf("border = %v, want explicit %v",
			layout.Shape.ColorBorder, Hex(0xff00ff))
	}
	if layout.Shape.SizeBorder != 0 {
		t.Errorf("size_border = %v, want 0 (explicit NoBorder)",
			layout.Shape.SizeBorder)
	}
}

// groupBoxInk falls back to DefaultTextStyle when a custom theme
// leaves its body text color unset, so such a theme still gets a
// group-box edge instead of transparent ink.
func TestGroupBoxInkFallsBackWhenTextColorUnset(t *testing.T) {
	restoreTheme(t)
	cfg := themeDarkCfg
	cfg.Name = "no-text-color"
	cfg.TextStyleDef.Color = Color{}
	SetTheme(ThemeMaker(cfg))

	got := groupBoxInk()
	want := DefaultTextStyle.Color.WithOpacity(groupBoxInkAlpha)
	if got != want {
		t.Errorf("ink = %v, want fallback %v", got, want)
	}
}
