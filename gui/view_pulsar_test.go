package gui

import "testing"

func TestPulsarShowsText1WhenCursorOn(t *testing.T) {
	w := &Window{}
	w.viewState.inputCursorOn = true
	v := Pulsar(PulsarCfg{}, w)
	layout := generateViewLayout(v, w)
	if len(layout.Children) != 1 {
		t.Fatalf("children = %d, want 1", len(layout.Children))
	}
	tc := layout.Children[0].Shape.TC
	if tc == nil || tc.Text != "..." {
		t.Errorf("text = %v, want ...", tc)
	}
}

func TestPulsarShowsText2WhenCursorOff(t *testing.T) {
	w := &Window{}
	w.viewState.inputCursorOn = false
	v := Pulsar(PulsarCfg{}, w)
	layout := generateViewLayout(v, w)
	if len(layout.Children) != 1 {
		t.Fatalf("children = %d, want 1", len(layout.Children))
	}
	tc := layout.Children[0].Shape.TC
	if tc == nil || tc.Text != ".." {
		t.Errorf("text = %v, want ..", tc)
	}
}

func TestPulsarCustomText(t *testing.T) {
	w := &Window{}
	w.viewState.inputCursorOn = true
	v := Pulsar(PulsarCfg{Text1: "ON", Text2: "OFF"}, w)
	layout := generateViewLayout(v, w)
	tc := layout.Children[0].Shape.TC
	if tc == nil || tc.Text != "ON" {
		t.Errorf("text = %v, want ON", tc)
	}
}

func TestPulsarSizeBorderNone(t *testing.T) {
	w := &Window{}
	v := Pulsar(PulsarCfg{}, w)
	layout := generateViewLayout(v, w)
	if layout.Shape.SizeBorder != 0 {
		t.Errorf("SizeBorder = %v, want 0", layout.Shape.SizeBorder)
	}
}

func TestPulsarSizingFitFit(t *testing.T) {
	w := &Window{}
	v := Pulsar(PulsarCfg{}, w)
	layout := generateViewLayout(v, w)
	if layout.Shape.Sizing != FitFit {
		t.Errorf("Sizing = %v, want FitFit", layout.Shape.Sizing)
	}
}

func TestPulsarThemeTextStyle(t *testing.T) {
	w := &Window{}
	v := Pulsar(PulsarCfg{}, w)
	layout := generateViewLayout(v, w)
	tc := layout.Children[0].Shape.TC
	if tc == nil {
		t.Fatal("TC is nil")
	}
	if *tc.TextStyle != guiTheme.TextStyleDef {
		t.Errorf("TextStyle = %+v, want theme default %+v",
			*tc.TextStyle, guiTheme.TextStyleDef)
	}
}

func TestPulsarCustomColor(t *testing.T) {
	w := &Window{}
	clr := RGB(255, 0, 0)
	v := Pulsar(PulsarCfg{Color: clr}, w)
	layout := generateViewLayout(v, w)
	tc := layout.Children[0].Shape.TC
	if tc == nil {
		t.Fatal("TC is nil")
	}
	if tc.TextStyle.Color != clr {
		t.Errorf("Color = %v, want %v", tc.TextStyle.Color, clr)
	}
	if tc.TextStyle.Family != guiTheme.TextStyleDef.Family {
		t.Errorf("Family = %v, want theme default %v",
			tc.TextStyle.Family, guiTheme.TextStyleDef.Family)
	}
}

func TestPulsarCustomSize(t *testing.T) {
	w := &Window{}
	v := Pulsar(PulsarCfg{Size: SomeF(20)}, w)
	layout := generateViewLayout(v, w)
	tc := layout.Children[0].Shape.TC
	if tc == nil {
		t.Fatal("TC is nil")
	}
	if tc.TextStyle.Size != 20 {
		t.Errorf("Size = %v, want 20", tc.TextStyle.Size)
	}
	if tc.TextStyle.Color != guiTheme.TextStyleDef.Color {
		t.Errorf("Color = %v, want theme default %v",
			tc.TextStyle.Color, guiTheme.TextStyleDef.Color)
	}
}

func TestPulsarCustomTextStyle(t *testing.T) {
	w := &Window{}
	ts := TextStyle{
		Color:  RGB(0, 0, 255),
		Size:   18,
		Family: "monospace",
	}
	v := Pulsar(PulsarCfg{TextStyle: ts}, w)
	layout := generateViewLayout(v, w)
	tc := layout.Children[0].Shape.TC
	if tc == nil {
		t.Fatal("TC is nil")
	}
	if *tc.TextStyle != ts {
		t.Errorf("TextStyle = %+v, want %+v", *tc.TextStyle, ts)
	}
}

func TestPulsarWidthCustom(t *testing.T) {
	w := &Window{}
	v := Pulsar(PulsarCfg{Width: 100}, w)
	layout := generateViewLayout(v, w)
	if layout.Shape.MinWidth != 100 {
		t.Errorf("MinWidth = %v, want 100", layout.Shape.MinWidth)
	}
}

func TestPulsarRegistersLayoutAnimation(t *testing.T) {
	w := &Window{}
	Pulsar(PulsarCfg{}, w)
	if !w.hasAnimationLocked(pulsarAnimationID) {
		t.Error("pulsar layout animation not registered")
	}
	a := w.animations[pulsarAnimationID]
	if a.RefreshKind() != AnimationRefreshLayout {
		t.Errorf("RefreshKind = %v, want AnimationRefreshLayout",
			a.RefreshKind())
	}
}

func TestPulsarAnimationIsViewBound(t *testing.T) {
	w := &Window{}
	Pulsar(PulsarCfg{}, w)
	if w.animViewBound == nil {
		t.Fatal("animViewBound nil after Pulsar — animation not view-bound")
	}
	if _, ok := w.animViewBound[pulsarAnimationID]; !ok {
		t.Error("pulsar animation not registered as view-bound")
	}
}
