package gui

import (
	"testing"

	"github.com/go-gui-org/go-glyph"
)

func TestMergeTextStyleFillsZeroColor(t *testing.T) {
	s := TextStyle{Size: 20}
	fb := TextStyle{Color: Red, Size: 12}
	m := mergeTextStyle(s, fb)
	if m.Color != Red {
		t.Error("zero color should be filled from fallback")
	}
	if m.Size != 20 {
		t.Error("non-zero size should be preserved")
	}
}

func TestMergeTextStylePreservesSetColor(t *testing.T) {
	s := TextStyle{Color: Blue, Size: 14}
	fb := TextStyle{Color: Red, Size: 12}
	m := mergeTextStyle(s, fb)
	if m.Color != Blue {
		t.Error("set color should not be overwritten")
	}
}

func TestMergeTextStyleFillsZeroSize(t *testing.T) {
	s := TextStyle{Color: Red}
	fb := TextStyle{Size: 16}
	m := mergeTextStyle(s, fb)
	if m.Size != 16 {
		t.Errorf("zero size should be filled: got %f", m.Size)
	}
}

func TestMergeTextStyleBothZero(t *testing.T) {
	s := TextStyle{}
	fb := TextStyle{}
	m := mergeTextStyle(s, fb)
	if m.Size != 0 {
		t.Error("both zero size should remain zero")
	}
}

func TestToGlyphStyleMapping(t *testing.T) {
	ts := TextStyle{
		Family:        "mono",
		Color:         RGBA(10, 20, 30, 40),
		BgColor:       RGBA(50, 60, 70, 80),
		Size:          14,
		LetterSpacing: 1.5,
		Underline:     true,
		Strikethrough: true,
		StrokeWidth:   2,
		StrokeColor:   RGBA(90, 100, 110, 120),
	}
	gs := ts.ToGlyphStyle()
	if gs.FontName != "mono" {
		t.Errorf("FontName: got %q", gs.FontName)
	}
	if gs.Color.R != 10 || gs.Color.G != 20 || gs.Color.B != 30 || gs.Color.A != 40 {
		t.Errorf("Color: got %+v", gs.Color)
	}
	if gs.BgColor.R != 50 || gs.BgColor.G != 60 || gs.BgColor.B != 70 || gs.BgColor.A != 80 {
		t.Errorf("BgColor: got %+v", gs.BgColor)
	}
	if gs.Size != 14 {
		t.Errorf("Size: got %f", gs.Size)
	}
	if gs.LetterSpacing != 1.5 {
		t.Errorf("LetterSpacing: got %f", gs.LetterSpacing)
	}
	if !gs.Underline {
		t.Error("Underline should be true")
	}
	if !gs.Strikethrough {
		t.Error("Strikethrough should be true")
	}
	if gs.StrokeWidth != 2 {
		t.Errorf("StrokeWidth: got %f", gs.StrokeWidth)
	}
	if gs.StrokeColor.R != 90 {
		t.Errorf("StrokeColor.R: got %d", gs.StrokeColor.R)
	}
}

func TestAffineIdentityCheck(t *testing.T) {
	id := glyph.AffineIdentity()
	if !affineTransformIsIdentity(id) {
		t.Error("identity should return true")
	}
	id.XX = 2
	if affineTransformIsIdentity(id) {
		t.Error("non-identity should return false")
	}
}

func TestHasTextTransformNone(t *testing.T) {
	ts := TextStyle{}
	if ts.HasTextTransform() {
		t.Error("zero TextStyle should have no transform")
	}
}

func TestHasTextTransformRotation(t *testing.T) {
	ts := TextStyle{RotationRadians: 0.5}
	if !ts.HasTextTransform() {
		t.Error("non-zero rotation should count as transform")
	}
}

func TestHasTextTransformAffine(t *testing.T) {
	tr := glyph.AffineRotation(1.0)
	ts := TextStyle{AffineTransform: &tr}
	if !ts.HasTextTransform() {
		t.Error("explicit affine should count as transform")
	}
}

func TestEffectiveTextTransformIdentity(t *testing.T) {
	ts := TextStyle{}
	tr := ts.EffectiveTextTransform()
	if !affineTransformIsIdentity(tr) {
		t.Error("zero TextStyle should give identity transform")
	}
}

func TestEffectiveTransformFromRotation(t *testing.T) {
	ts := TextStyle{RotationRadians: 1.0}
	tr := ts.EffectiveTextTransform()
	if affineTransformIsIdentity(tr) {
		t.Error("rotation should give non-identity transform")
	}
}

func TestEffectiveTransformExplicitAffine(t *testing.T) {
	explicit := glyph.AffineTransform{XX: 2, YY: 2}
	ts := TextStyle{AffineTransform: &explicit, RotationRadians: 1.0}
	tr := ts.EffectiveTextTransform()
	if tr.XX != 2 {
		t.Error("explicit affine should take precedence over rotation")
	}
}
