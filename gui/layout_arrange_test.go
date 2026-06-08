package gui

import "testing"

func TestEnsureLayoutShapeNilShape(t *testing.T) {
	layout := Layout{}
	ensureLayoutShape(&layout)
	if layout.Shape == nil {
		t.Fatal("Shape should be initialized")
	}
	if layout.Shape.shapeType != shapeNone {
		t.Errorf("shapeType = %v, want shapeNone", layout.Shape.shapeType)
	}
}

func TestEnsureLayoutShapePreserves(t *testing.T) {
	s := &Shape{shapeType: shapeRectangle, Width: 42}
	layout := Layout{Shape: s}
	ensureLayoutShape(&layout)
	if layout.Shape != s {
		t.Error("Shape pointer changed")
	}
	if layout.Shape.Width != 42 {
		t.Errorf("Width = %f, want 42", layout.Shape.Width)
	}
}

func TestLayoutRemoveFloatingLayoutsExtractsFloat(t *testing.T) {
	layout := Layout{
		Shape: &Shape{shapeType: shapeRectangle},
		Children: []Layout{
			{Shape: &Shape{shapeType: shapeRectangle, Width: 10}},
			{Shape: &Shape{shapeType: shapeRectangle, Width: 20, Float: true}},
			{Shape: &Shape{shapeType: shapeRectangle, Width: 30}},
		},
	}
	var floats []*Layout
	layoutRemoveFloatingLayouts(&layout, nil, &floats)

	if len(floats) != 1 {
		t.Fatalf("floats = %d, want 1", len(floats))
	}
	if floats[0].Shape.Width != 20 {
		t.Errorf("float Width = %f, want 20", floats[0].Shape.Width)
	}
	// Original slot replaced with placeholder.
	if layout.Children[1].Shape.shapeType != shapeNone {
		t.Error("placeholder expected at index 1")
	}
}

func TestLayoutRemoveFloatingLayoutsNone(t *testing.T) {
	layout := Layout{
		Shape: &Shape{shapeType: shapeRectangle},
		Children: []Layout{
			{Shape: &Shape{shapeType: shapeRectangle}},
			{Shape: &Shape{shapeType: shapeRectangle}},
		},
	}
	var floats []*Layout
	layoutRemoveFloatingLayouts(&layout, nil, &floats)
	if len(floats) != 0 {
		t.Errorf("floats = %d, want 0", len(floats))
	}
}

func TestLayoutRemoveFloatingLayoutsNested(t *testing.T) {
	layout := Layout{
		Shape: &Shape{shapeType: shapeRectangle},
		Children: []Layout{
			{
				Shape: &Shape{shapeType: shapeRectangle},
				Children: []Layout{
					{Shape: &Shape{shapeType: shapeRectangle, Width: 99, Float: true}},
				},
			},
		},
	}
	var floats []*Layout
	layoutRemoveFloatingLayouts(&layout, nil, &floats)
	if len(floats) != 1 {
		t.Fatalf("floats = %d, want 1", len(floats))
	}
	if floats[0].Shape.Width != 99 {
		t.Errorf("float Width = %f, want 99", floats[0].Shape.Width)
	}
}

func TestInjectFloatingLayerNil(t *testing.T) {
	w := NewWindow(WindowCfg{State: new(int), Width: 100, Height: 100})
	var floats []*Layout
	injectFloatingLayer(nil, w, &floats)
	if len(floats) != 0 {
		t.Errorf("floats = %d, want 0 for nil view", len(floats))
	}
}

func TestInjectFloatingLayerValid(t *testing.T) {
	w := NewWindow(WindowCfg{State: new(int), Width: 100, Height: 100})
	var floats []*Layout
	v := Text(TextCfg{Text: "overlay"})
	injectFloatingLayer(v, w, &floats)
	if len(floats) != 1 {
		t.Fatalf("floats = %d, want 1", len(floats))
	}
	if floats[0].Shape == nil {
		t.Error("injected layout has nil Shape")
	}
}

func TestLayoutArrangeFloatZIndex(t *testing.T) {
	w := NewWindow(WindowCfg{State: new(int), Width: 400, Height: 400})
	layout := Layout{
		Shape: &Shape{
			shapeType: shapeRectangle,
			Width:     400, Height: 400,
			Sizing:  FillFill,
			Opacity: 1,
		},
		Children: []Layout{
			{Shape: &Shape{
				shapeType: shapeRectangle,
				Width:     50, Height: 50,
				Float:       true,
				FloatZIndex: 5,
				Opacity:     1,
			}},
			{Shape: &Shape{
				shapeType: shapeRectangle,
				Width:     50, Height: 50,
				Float:       true,
				FloatZIndex: 1,
				Opacity:     1,
			}},
		},
	}
	layers := layoutArrange(&layout, w)
	// layers[0] = main, layers[1] = z-index 1, layers[2] = z-index 5
	if len(layers) < 3 {
		t.Fatalf("layers = %d, want >= 3", len(layers))
	}
}

func TestComposeLayoutWrapsLayers(t *testing.T) {
	w := &Window{windowWidth: 800, windowHeight: 600}
	layers := []Layout{
		{Shape: &Shape{shapeType: shapeRectangle, Width: 800, Height: 600}},
		{Shape: &Shape{shapeType: shapeRectangle, Width: 100, Height: 100}},
	}
	root := composeLayout(layers, w)
	if root.Shape == nil {
		t.Fatal("root shape nil")
	}
	if root.Shape.Width != 800 || root.Shape.Height != 600 {
		t.Errorf("root size = %fx%f, want 800x600",
			root.Shape.Width, root.Shape.Height)
	}
	if len(root.Children) != 2 {
		t.Errorf("children = %d, want 2", len(root.Children))
	}
}
