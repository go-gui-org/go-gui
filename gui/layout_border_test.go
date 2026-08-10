package gui

import "testing"

func TestLayoutWidthsWithBorder(t *testing.T) {
	root := &Layout{
		Shape: &Shape{
			Axis:       axisLeftToRight,
			SizeBorder: 5,
			Padding:    NewPadding(0, 10, 0, 10),
		},
		Children: []Layout{
			{Shape: &Shape{Width: 50, MinWidth: 50}},
		},
	}

	layoutWidths(root)

	// Child(50) + Padding(10+10) + Border(5+5) = 80
	if !f32AreClose(root.Shape.Width, 80.0) {
		t.Errorf("width: got %f, want 80", root.Shape.Width)
	}
	if !f32AreClose(root.Shape.MinWidth, 80.0) {
		t.Errorf("min_width: got %f, want 80", root.Shape.MinWidth)
	}
}

func TestLayoutHeightsWithBorder(t *testing.T) {
	root := &Layout{
		Shape: &Shape{
			Axis:       axisTopToBottom,
			SizeBorder: 5,
			Padding:    NewPadding(10, 0, 10, 0),
		},
		Children: []Layout{
			{Shape: &Shape{Height: 50, MinHeight: 50}},
		},
	}

	layoutHeights(root)

	if !f32AreClose(root.Shape.Height, 80.0) {
		t.Errorf("height: got %f, want 80", root.Shape.Height)
	}
	if !f32AreClose(root.Shape.MinHeight, 80.0) {
		t.Errorf("min_height: got %f, want 80", root.Shape.MinHeight)
	}
}

func TestLayoutPositionWithBorder(t *testing.T) {
	root := &Layout{
		Shape: &Shape{
			X: 0, Y: 0, Width: 100, Height: 100,
			Axis:       axisLeftToRight,
			SizeBorder: 5,
			Padding:    NewPadding(10, 10, 10, 10),
		},
		Children: []Layout{
			{Shape: &Shape{Width: 40, Height: 40}},
		},
	}

	w := &Window{}
	layoutParents(root, nil)
	layoutPositions(root, 0, 0, w)

	c1x := root.Children[0].Shape.X
	c1y := root.Children[0].Shape.Y
	if !f32AreClose(c1x, 15.0) {
		t.Errorf("C1 X: got %f, want 15", c1x)
	}
	if !f32AreClose(c1y, 15.0) {
		t.Errorf("C1 Y: got %f, want 15", c1y)
	}
}
