package gui

import "testing"

func TestProgressBarDefaultLayout(t *testing.T) {
	v := ProgressBar(ProgressBarCfg{
		ID:      "pb-test",
		Percent: 0.5,
	})
	layout := generateViewLayout(v, &Window{})
	// Row with fill bar child + text label child
	if layout.Shape.Axis != axisLeftToRight {
		t.Error("default should be horizontal (row)")
	}
	if len(layout.Children) < 1 {
		t.Fatal("should have at least 1 child (fill bar)")
	}
}

func TestProgressBarCenterLabel(t *testing.T) {
	// The label must be centered on both axes inside the parent,
	// shifting its own child to keep the offset consistent.
	parent := &Shape{X: 100, Y: 50, Width: 200, Height: 40}
	lbl := &Layout{
		Shape: &Shape{X: 0, Y: 0, Width: 60, Height: 20},
		Children: []Layout{
			{Shape: &Shape{X: 5, Y: 5, Width: 50, Height: 10}},
		},
	}

	progressBarCenterLabel(parent, lbl)
	// Center of parent (200, 70) minus half the label (30, 10).
	if lbl.Shape.X != 170 || lbl.Shape.Y != 60 {
		t.Fatalf("label at (%v, %v), want (170, 60)",
			lbl.Shape.X, lbl.Shape.Y)
	}
	// The inner child follows the label's translation: it moved from
	// (5,5) to (175, 65) — a shift of +170 x, +55 y.
	inner := &lbl.Children[0]
	if inner.Shape.X != 175 || inner.Shape.Y != 65 {
		t.Fatalf("inner child at (%v, %v), want (175, 65)",
			inner.Shape.X, inner.Shape.Y)
	}

	// A label without children still centers (no child shift).
	lbl2 := &Layout{Shape: &Shape{X: 0, Y: 0, Width: 40, Height: 10}}
	progressBarCenterLabel(parent, lbl2)
	if lbl2.Shape.X != 180 || lbl2.Shape.Y != 65 {
		t.Fatalf("childless label at (%v, %v), want (180, 65)",
			lbl2.Shape.X, lbl2.Shape.Y)
	}
}

func TestProgressBarVertical(t *testing.T) {
	v := ProgressBar(ProgressBarCfg{
		ID:       "pb-test",
		Percent:  0.5,
		vertical: true,
	})
	layout := generateViewLayout(v, &Window{})
	if layout.Shape.Axis != axisTopToBottom {
		t.Error("vertical bar should use column axis")
	}
}

func TestProgressBarTextShow(t *testing.T) {
	v := ProgressBar(ProgressBarCfg{
		ID:       "pb-test",
		Percent:  0.5,
		TextShow: true,
	})
	layout := generateViewLayout(v, &Window{})
	if len(layout.Children) != 2 {
		t.Fatalf("with text: got %d children, want 2",
			len(layout.Children))
	}
}

func TestProgressBarNoText(t *testing.T) {
	v := ProgressBar(ProgressBarCfg{
		ID:       "pb-test",
		Percent:  0.5,
		TextShow: false,
	})
	layout := generateViewLayout(v, &Window{})
	if len(layout.Children) != 1 {
		t.Fatalf("without text: got %d children, want 1",
			len(layout.Children))
	}
}

func TestProgressBarA11YRole(t *testing.T) {
	v := ProgressBar(ProgressBarCfg{
		ID:      "pb-test",
		Percent: 0.3,
	})
	layout := generateViewLayout(v, &Window{})
	if layout.Shape.A11YRole != AccessRoleProgressBar {
		t.Errorf("role = %d, want ProgressBar", layout.Shape.A11YRole)
	}
	if layout.Shape.a11Y == nil {
		t.Fatal("a11y should be set")
	}
	if layout.Shape.a11Y.ValueMax != 1 {
		t.Error("value_max should be 1")
	}
}

func TestProgressBarIndefiniteA11Y(t *testing.T) {
	v := ProgressBar(ProgressBarCfg{
		ID:         "pb-test",
		Indefinite: true,
	})
	layout := generateViewLayout(v, &Window{})
	want := AccessStateBusy | AccessStateLive
	if layout.Shape.A11YState != want {
		t.Errorf("state = %d, want busy|live (%d)",
			layout.Shape.A11YState, want)
	}
}

func TestProgressBarA11YLabel(t *testing.T) {
	v := ProgressBar(ProgressBarCfg{
		ID:      "pb-test",
		Percent: 0.5,
		A11YCfg: A11YCfg{A11YLabel: "upload"},
	})
	layout := generateViewLayout(v, &Window{})
	if layout.Shape.a11Y == nil {
		t.Fatal("a11y nil")
	}
	if layout.Shape.a11Y.Label != "upload" {
		t.Errorf("label = %q, want %q",
			layout.Shape.a11Y.Label, "upload")
	}
}

func TestProgressBarA11YDescription(t *testing.T) {
	v := ProgressBar(ProgressBarCfg{
		ID:      "pb-test",
		Percent: 0.5,
		A11YCfg: A11YCfg{A11YDescription: "uploading file"},
	})
	layout := generateViewLayout(v, &Window{})
	if layout.Shape.a11Y == nil {
		t.Fatal("a11y nil")
	}
	if layout.Shape.a11Y.Description != "uploading file" {
		t.Errorf("desc = %q, want %q",
			layout.Shape.a11Y.Description, "uploading file")
	}
}

func TestProgressBarPercentClampHigh(t *testing.T) {
	v := ProgressBar(ProgressBarCfg{
		ID:       "pb-test",
		Percent:  1.5,
		TextShow: true,
	})
	layout := generateViewLayout(v, &Window{})
	if len(layout.Children) < 2 {
		t.Fatal("expected text child")
	}
	lbl := &layout.Children[1]
	if len(lbl.Children) == 0 {
		t.Fatal("text label has no children")
	}
	got := lbl.Children[0].Shape.TC.Text
	if got != "100%" {
		t.Errorf("text = %q, want %q", got, "100%")
	}
}

func TestProgressBarPercentClampLow(t *testing.T) {
	v := ProgressBar(ProgressBarCfg{
		ID:       "pb-test",
		Percent:  -0.5,
		TextShow: true,
	})
	layout := generateViewLayout(v, &Window{})
	if len(layout.Children) < 2 {
		t.Fatal("expected text child")
	}
	lbl := &layout.Children[1]
	if len(lbl.Children) == 0 {
		t.Fatal("text label has no children")
	}
	got := lbl.Children[0].Shape.TC.Text
	if got != "0%" {
		t.Errorf("text = %q, want %q", got, "0%")
	}
}

func TestProgressBarVerticalTextShow(t *testing.T) {
	v := ProgressBar(ProgressBarCfg{
		ID:       "pb-test",
		Percent:  0.5,
		vertical: true,
		TextShow: true,
	})
	layout := generateViewLayout(v, &Window{})
	if len(layout.Children) != 2 {
		t.Fatalf("vertical+text: got %d children, want 2",
			len(layout.Children))
	}
}

func TestProgressBarThemeTextStyle(t *testing.T) {
	v := ProgressBar(ProgressBarCfg{
		ID:       "pb-test",
		Percent:  0.5,
		TextShow: true,
	})
	layout := generateViewLayout(v, &Window{})
	if len(layout.Children) < 2 {
		t.Fatal("expected text child")
	}
	lbl := &layout.Children[1]
	if len(lbl.Children) == 0 {
		t.Fatal("text label has no children")
	}
	got := *lbl.Children[0].Shape.TC.TextStyle
	want := guiTheme.progressBarStyle.TextStyle
	if got != want {
		t.Errorf("textStyle = %v, want %v", got, want)
	}
}

func TestProgressBarRadiusZeroOverride(t *testing.T) {
	v := ProgressBar(ProgressBarCfg{
		ID:      "pb-test",
		Percent: 0.5,
		Radius:  NoRadius,
	})
	layout := generateViewLayout(v, &Window{})
	if layout.Shape.Radius != 0 {
		t.Errorf("radius = %f, want 0", layout.Shape.Radius)
	}
}

func TestProgressBarTextBackgroundColor(t *testing.T) {
	bg := RGBA(255, 0, 0, 255)
	v := ProgressBar(ProgressBarCfg{
		ID:             "pb-test",
		Percent:        0.5,
		TextShow:       true,
		textBackground: bg,
	})
	layout := generateViewLayout(v, &Window{})
	if len(layout.Children) < 2 {
		t.Fatal("expected text child")
	}
	lbl := &layout.Children[1]
	if lbl.Shape.Color != bg {
		t.Errorf("text bg Color = %v, want %v",
			lbl.Shape.Color, bg)
	}
	if lbl.Shape.ColorBorder == bg {
		t.Error("TextBackground should not be on ColorBorder")
	}
}

func TestProgressBarIndefiniteAnimationIsViewBound(t *testing.T) {
	v := ProgressBar(ProgressBarCfg{ID: "pb-vb", Indefinite: true})
	w := &Window{}
	layout := generateViewLayout(v, w)
	if layout.Shape.events == nil || layout.Shape.events.AmendLayout == nil {
		t.Fatal("AmendLayout not set on progress bar layout")
	}
	layout.Shape.events.AmendLayout(EventCtx{&layout, nil, w})
	if w.animViewBound == nil {
		t.Fatal("animViewBound nil after indefinite progress bar AmendLayout — animation not view-bound")
	}
	if _, ok := w.animViewBound[ScopeID("pb-vb", "indefinite")]; !ok {
		t.Error("indefinite progress bar animation not registered as view-bound")
	}
}

func TestProgressBarSizeBorderNone(t *testing.T) {
	v := ProgressBar(ProgressBarCfg{
		ID:       "pb-test",
		Percent:  0.5,
		TextShow: true,
	})
	layout := generateViewLayout(v, &Window{})
	// Outer container
	if layout.Shape.SizeBorder != 0 {
		t.Errorf("outer SizeBorder = %f, want 0",
			layout.Shape.SizeBorder)
	}
	// Bar child
	if layout.Children[0].Shape.SizeBorder != 0 {
		t.Errorf("bar SizeBorder = %f, want 0",
			layout.Children[0].Shape.SizeBorder)
	}
	// Text label child
	if len(layout.Children) > 1 {
		if layout.Children[1].Shape.SizeBorder != 0 {
			t.Errorf("text SizeBorder = %f, want 0",
				layout.Children[1].Shape.SizeBorder)
		}
	}
}
