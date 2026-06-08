package gui

import "testing"

func TestProgressBarDefaultLayout(t *testing.T) {
	v := ProgressBar(ProgressBarCfg{
		ID:      "pb-test",
		Percent: 0.5,
	})
	layout := generateViewLayout(v, &Window{})
	// Row with fill bar child + text label child
	if layout.Shape.Axis != AxisLeftToRight {
		t.Error("default should be horizontal (row)")
	}
	if len(layout.Children) < 1 {
		t.Fatal("should have at least 1 child (fill bar)")
	}
}

func TestProgressBarVertical(t *testing.T) {
	v := ProgressBar(ProgressBarCfg{
		ID:       "pb-test",
		Percent:  0.5,
		Vertical: true,
	})
	layout := generateViewLayout(v, &Window{})
	if layout.Shape.Axis != AxisTopToBottom {
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
	if layout.Shape.A11Y == nil {
		t.Fatal("a11y should be set")
	}
	if layout.Shape.A11Y.ValueMax != 1 {
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
		ID:        "pb-test",
		Percent:   0.5,
		A11YLabel: "upload",
	})
	layout := generateViewLayout(v, &Window{})
	if layout.Shape.A11Y == nil {
		t.Fatal("a11y nil")
	}
	if layout.Shape.A11Y.Label != "upload" {
		t.Errorf("label = %q, want %q",
			layout.Shape.A11Y.Label, "upload")
	}
}

func TestProgressBarA11YDescription(t *testing.T) {
	v := ProgressBar(ProgressBarCfg{
		ID:              "pb-test",
		Percent:         0.5,
		A11YDescription: "uploading file",
	})
	layout := generateViewLayout(v, &Window{})
	if layout.Shape.A11Y == nil {
		t.Fatal("a11y nil")
	}
	if layout.Shape.A11Y.Description != "uploading file" {
		t.Errorf("desc = %q, want %q",
			layout.Shape.A11Y.Description, "uploading file")
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
		Vertical: true,
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
	want := guiTheme.ProgressBarStyle.TextStyle
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
	bg := Color{255, 0, 0, 255, true}
	v := ProgressBar(ProgressBarCfg{
		ID:             "pb-test",
		Percent:        0.5,
		TextShow:       true,
		TextBackground: bg,
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
	layout.Shape.events.AmendLayout(&layout, w)
	if w.animViewBound == nil {
		t.Fatal("animViewBound nil after indefinite progress bar AmendLayout — animation not view-bound")
	}
	if _, ok := w.animViewBound["pb-vb_indefinite"]; !ok {
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
