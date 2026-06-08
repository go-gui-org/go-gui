package gui

import "testing"

func TestRectangleGeneratesLayout(t *testing.T) {
	w := &Window{}
	v := Rectangle(RectangleCfg{
		ID:     "r1",
		Width:  100,
		Height: 50,
	})
	layout := GenerateViewLayout(v, w)
	if layout.Shape == nil {
		t.Fatal("expected shape")
	}
	if layout.Shape.ID != "r1" {
		t.Errorf("ID: got %s", layout.Shape.ID)
	}
}

func TestRectangleColorPassthrough(t *testing.T) {
	w := &Window{}
	c := RGB(255, 0, 0)
	v := Rectangle(RectangleCfg{
		Width:  10,
		Height: 10,
		Color:  c,
	})
	layout := GenerateViewLayout(v, w)
	if layout.Shape.Color != c {
		t.Errorf("color: got %v, want %v",
			layout.Shape.Color, c)
	}
}

func TestRectangleGradient(t *testing.T) {
	w := &Window{}
	g := &GradientDef{
		Stops: []GradientStop{
			{Color: RGB(255, 0, 0), Pos: 0},
			{Color: RGB(0, 0, 255), Pos: 1},
		},
	}
	v := Rectangle(RectangleCfg{
		Width:    50,
		Height:   50,
		Gradient: g,
	})
	layout := GenerateViewLayout(v, w)
	if layout.Shape.fx == nil || layout.Shape.fx.Gradient == nil {
		t.Error("expected gradient")
	}
}

func TestRectangleSizing(t *testing.T) {
	w := &Window{}
	v := Rectangle(RectangleCfg{
		Width:  30,
		Height: 20,
		Sizing: FillFixed,
	})
	layout := GenerateViewLayout(v, w)
	if layout.Shape.Sizing != FillFixed {
		t.Errorf("sizing: got %+v", layout.Shape.Sizing)
	}
}

func TestRectangleMinDimensions(t *testing.T) {
	w := &Window{}
	v := Rectangle(RectangleCfg{
		Width:  40,
		Height: 25,
	})
	layout := GenerateViewLayout(v, w)
	// Rectangle sets MinWidth = Width and MinHeight = Height.
	if !f32AreClose(layout.Shape.MinWidth, 40) {
		t.Errorf("MinWidth: got %f", layout.Shape.MinWidth)
	}
	if !f32AreClose(layout.Shape.MinHeight, 25) {
		t.Errorf("MinHeight: got %f", layout.Shape.MinHeight)
	}
}
