package gui

import "testing"

func TestSvgToColor(t *testing.T) {
	sc := SvgColor{R: 255, G: 128, B: 64, A: 200}
	c := svgToColor(sc)
	if c.R != 255 || c.G != 128 || c.B != 64 || c.A != 200 {
		t.Fatalf("expected {255,128,64,200}, got %+v", c)
	}
}

func TestSvgToColorTransparent(t *testing.T) {
	sc := SvgColor{}
	c := svgToColor(sc)
	if c != (Color{0, 0, 0, 0, true}) {
		t.Fatalf("expected transparent Color, got %+v", c)
	}
}

func TestStrokeCapConstants(t *testing.T) {
	if SvgButtCap != 0 || SvgRoundCap != 1 || SvgSquareCap != 2 {
		t.Fatal("SvgStrokeCap constants wrong")
	}
}

func TestStrokeJoinConstants(t *testing.T) {
	if SvgMiterJoin != 0 || SvgRoundJoin != 1 || SvgBevelJoin != 2 {
		t.Fatal("SvgStrokeJoin constants wrong")
	}
}
