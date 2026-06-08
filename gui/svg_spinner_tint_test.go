package gui

import (
	"testing"
)

// TestSvgSpinnerDefaultColorAssigned verifies the SvgSpinner
// factory populates SvgCfg.Color with a non-zero default when
// the caller omits it. Required so currentColor assets (which
// parse as black) render visibly against a dark background.
func TestSvgSpinnerDefaultColorAssigned(t *testing.T) {
	SetTheme(ThemeDark.WithBorders(true))
	v := SvgSpinner(SvgSpinnerCfg{Kind: SvgSpinner90Ring})
	sv, ok := v.(*svgView)
	if !ok {
		t.Fatalf("expected *svgView, got %T", v)
	}
	if !sv.cfg.Color.IsSet() || sv.cfg.Color.A == 0 {
		t.Fatalf("expected visible default color, got %+v", sv.cfg.Color)
	}
}

// TestSvgShapeOpacityDefaultFullyOpaque pins the invariant that
// the Svg view sets Shape.Opacity=1. Without it, the render
// layer multiplies shape.Color alpha by zero, silently dropping
// the tint override used by currentColor SVGs like spinners.
func TestSvgShapeOpacityDefaultFullyOpaque(t *testing.T) {
	SetTheme(ThemeDark.WithBorders(true))
	w := &Window{}
	w.SetSvgParser(&mockSvgParser{width: 24, height: 24})
	v := SvgSpinner(SvgSpinnerCfg{
		Kind:   SvgSpinner90Ring,
		Width:  72,
		Height: 72,
	})
	layout := v.GenerateLayout(w)
	if layout.Shape == nil {
		t.Fatal("nil shape")
	}
	if layout.Shape.Opacity != 1 {
		t.Fatalf("expected Shape.Opacity=1, got %f", layout.Shape.Opacity)
	}
	if !layout.Shape.Color.IsSet() || layout.Shape.Color.A == 0 {
		t.Fatalf("expected visible Shape.Color, got %+v", layout.Shape.Color)
	}
}
