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
	gs := ts.toGlyphStyle()
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

// Grid geometry reaches glyph only through toGlyphStyle; a dropped field is
// silent (box glyphs just fall back to advance-derived cells), so assert it.
func TestToGlyphStyleGridGeometry(t *testing.T) {
	ts := TextStyle{
		Family:             "mono",
		Size:               14,
		CellWidth:          9.5,
		CellHeight:         20,
		NoBuiltinBoxGlyphs: true,
	}
	gs := ts.toGlyphStyle()
	if gs.CellWidth != 9.5 {
		t.Errorf("CellWidth: got %f, want 9.5", gs.CellWidth)
	}
	if gs.CellHeight != 20 {
		t.Errorf("CellHeight: got %f, want 20", gs.CellHeight)
	}
	if !gs.NoBuiltinBoxGlyphs {
		t.Error("NoBuiltinBoxGlyphs should be true")
	}
}

// Zero values must stay zero so glyph keeps its derive-the-cell default.
func TestToGlyphStyleGridGeometryUnset(t *testing.T) {
	gs := TextStyle{Family: "mono", Size: 14}.toGlyphStyle()
	if gs.CellWidth != 0 || gs.CellHeight != 0 || gs.NoBuiltinBoxGlyphs {
		t.Errorf("unset grid fields leaked: %+v", gs)
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
	if ts.hasTextTransform() {
		t.Error("zero TextStyle should have no transform")
	}
}

func TestHasTextTransformRotation(t *testing.T) {
	ts := TextStyle{RotationRadians: 0.5}
	if !ts.hasTextTransform() {
		t.Error("non-zero rotation should count as transform")
	}
}

func TestHasTextTransformAffine(t *testing.T) {
	tr := glyph.AffineRotation(1.0)
	ts := TextStyle{AffineTransform: &tr}
	if !ts.hasTextTransform() {
		t.Error("explicit affine should count as transform")
	}
}

func TestEffectiveTextTransformIdentity(t *testing.T) {
	ts := TextStyle{}
	tr := ts.effectiveTextTransform()
	if !affineTransformIsIdentity(tr) {
		t.Error("zero TextStyle should give identity transform")
	}
}

func TestEffectiveTransformFromRotation(t *testing.T) {
	ts := TextStyle{RotationRadians: 1.0}
	tr := ts.effectiveTextTransform()
	if affineTransformIsIdentity(tr) {
		t.Error("rotation should give non-identity transform")
	}
}

func TestEffectiveTransformExplicitAffine(t *testing.T) {
	explicit := glyph.AffineTransform{XX: 2, YY: 2}
	ts := TextStyle{AffineTransform: &explicit, RotationRadians: 1.0}
	tr := ts.effectiveTextTransform()
	if tr.XX != 2 {
		t.Error("explicit affine should take precedence over rotation")
	}
}
