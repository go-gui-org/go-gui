package gui

import "testing"

func TestProgressBarDefaultLayout(t *testing.T) {
	v := ProgressBar(ProgressBarCfg{
		ID:      "pb-test",
		Percent: 0.5,
	})
	layout := generateViewLayout(v, &Window{})
	// Row with the track child (holding the fill bar) + optional label
	if layout.Shape.Axis != axisLeftToRight {
		t.Error("default should be horizontal (row)")
	}
	if len(layout.Children) < 1 {
		t.Fatal("should have at least 1 child (track)")
	}
}

// The fill is sized from the track's own laid-out box, not the outer
// widget's: the outer row also carries the trailing readout
// (visual-refresh §8), so using the outer width would shrink the fill
// by the readout's share and spill the fill past its track.
func TestProgressBarFillUsesTrackBox(t *testing.T) {
	v := ProgressBar(ProgressBarCfg{
		ID:       "pb-test",
		Percent:  0.5,
		TextShow: true,
	})
	layout := generateViewLayout(v, &Window{})
	if len(layout.Children) < 2 {
		t.Fatal("expected the track and the readout")
	}
	track := layout.Children[0]
	if len(track.Children) == 0 {
		t.Fatal("track has no fill child")
	}
	fill := &track.Children[0]
	track.Shape.Width = 100
	track.Shape.Height = 20
	track.Shape.X = 10
	track.Shape.Y = 5

	layout.Shape.events.AmendLayout(EventCtx{&layout, nil, &Window{}})

	if fill.Shape.Width != 50 {
		t.Errorf("fill width = %v, want 50 (half the track)", fill.Shape.Width)
	}
	if fill.Shape.Height != 20 {
		t.Errorf("fill height = %v, want the track height 20", fill.Shape.Height)
	}
	if fill.Shape.X != 10 {
		t.Errorf("fill x = %v, want the track x 10", fill.Shape.X)
	}
}

// The readout is a trailing sibling of the track, not a child overlaid
// on it: with TextShow the widget is a two-child row.
func TestProgressBarReadoutTrails(t *testing.T) {
	v := ProgressBar(ProgressBarCfg{
		ID:       "pb-test",
		Percent:  0.5,
		TextShow: true,
	})
	layout := generateViewLayout(v, &Window{})
	if len(layout.Children) != 2 {
		t.Fatalf("with text: got %d children, want 2", len(layout.Children))
	}
	if layout.Children[0].Shape.Color == ColorTransparent {
		t.Error("the track should paint the track color")
	}
}

func TestProgressBarVertical(t *testing.T) {
	v := ProgressBar(ProgressBarCfg{
		ID:       "pb-test",
		Percent:  0.5,
		Vertical: true,
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
	want := guiTheme.progressBarStyle.TextStyle
	if got != want {
		t.Errorf("textStyle = %v, want %v", got, want)
	}
}

// A vertical bar stays at its stated width even when the trailing
// readout is wider: the column's cross-axis fill pass would stretch
// the FillFit track to the readout's width, silently breaking the
// caller's Width (visual-refresh §8).
func TestProgressBarVerticalKeepsStatedWidth(t *testing.T) {
	v := ProgressBar(ProgressBarCfg{
		ID:       "pb-test",
		Percent:  0.5,
		Vertical: true,
		TextShow: true,
		Width:    12,
	})
	layout := generateViewLayout(v, &Window{})
	track := &layout.Children[0]
	if track.Shape.Width != 12 {
		t.Errorf("track width = %v, want the stated 12", track.Shape.Width)
	}
}

// A vertical FillFit bar still fills: the cap only applies when the
// caller did not ask the widget to fill the width axis.
func TestProgressBarVerticalFillFitFills(t *testing.T) {
	v := ProgressBar(ProgressBarCfg{
		ID:       "pb-test",
		Percent:  0.5,
		Vertical: true,
		Sizing:   FillFit,
	})
	layout := generateViewLayout(v, &Window{})
	if maxW := layout.Children[0].Shape.MaxWidth; maxW != 0 {
		t.Errorf("MaxWidth = %v, want 0 for a FillFit bar", maxW)
	}
}

// The vertical amend sizes the fill from the track's box: height =
// percent × track height, width = track width, anchored at the track's
// Y (plus the indefinite offset). The § 8 restructure moved the fill
// under the track and only this branch guards its geometry.
func TestProgressBarVerticalAmendFillsTrackBox(t *testing.T) {
	v := ProgressBar(ProgressBarCfg{
		ID:       "pb-v",
		Percent:  0.5,
		Vertical: true,
	})
	layout := generateViewLayout(v, &Window{})
	track := &layout.Children[0]
	if len(track.Children) == 0 {
		t.Fatal("track has no fill child")
	}
	fill := &track.Children[0]
	track.Shape.Width = 12
	track.Shape.Height = 100
	track.Shape.X = 5
	track.Shape.Y = 10

	layout.Shape.events.AmendLayout(EventCtx{&layout, nil, &Window{}})

	if fill.Shape.Height != 50 {
		t.Errorf("fill height = %v, want 50 (half the track)", fill.Shape.Height)
	}
	if fill.Shape.Width != 12 {
		t.Errorf("fill width = %v, want the track width 12", fill.Shape.Width)
	}
	if fill.Shape.Y != 10 {
		t.Errorf("fill y = %v, want the track y 10", fill.Shape.Y)
	}
}

func TestProgressBarRadiusZeroOverride(t *testing.T) {
	v := ProgressBar(ProgressBarCfg{
		ID:      "pb-test",
		Percent: 0.5,
		Radius:  NoRadius,
	})
	layout := generateViewLayout(v, &Window{})
	// The radius lives on the track (the painted shape), not the
	// transparent outer row.
	if len(layout.Children) == 0 {
		t.Fatal("expected the track child")
	}
	if layout.Children[0].Shape.Radius != 0 {
		t.Errorf("radius = %f, want 0", layout.Children[0].Shape.Radius)
	}
}

func TestProgressBarTextBackgroundColor(t *testing.T) {
	bg := RGBA(255, 0, 0, 255)
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
