package gui

import "testing"

func TestWrapBasic(t *testing.T) {
	root := &Layout{
		Shape: &Shape{
			Axis: AxisLeftToRight, Wrap: true, Width: 80, Spacing: 5,
		},
		Children: []Layout{
			{Shape: &Shape{Width: 30, shapeType: shapeRectangle}},
			{Shape: &Shape{Width: 30, shapeType: shapeRectangle}},
			{Shape: &Shape{Width: 30, shapeType: shapeRectangle}},
		},
	}

	layoutWrapContainers(root, &Window{})

	if root.Shape.Axis != AxisTopToBottom {
		t.Error("axis should flip to TTB")
	}
	if len(root.Children) != 2 {
		t.Fatalf("rows: got %d, want 2", len(root.Children))
	}
	if len(root.Children[0].Children) != 2 {
		t.Errorf("row 0 children: got %d, want 2", len(root.Children[0].Children))
	}
	if len(root.Children[1].Children) != 1 {
		t.Errorf("row 1 children: got %d, want 1", len(root.Children[1].Children))
	}
	if root.Children[0].Shape.Axis != AxisLeftToRight {
		t.Error("row 0 axis should be LTR")
	}
	if root.Children[0].Shape.Spacing != 5 {
		t.Errorf("row 0 spacing: got %f", root.Children[0].Shape.Spacing)
	}
}

func TestWrapSingleRow(t *testing.T) {
	root := &Layout{
		Shape: &Shape{
			Axis: AxisLeftToRight, Wrap: true, Width: 200, Spacing: 5,
		},
		Children: []Layout{
			{Shape: &Shape{Width: 30, shapeType: shapeRectangle}},
			{Shape: &Shape{Width: 30, shapeType: shapeRectangle}},
			{Shape: &Shape{Width: 30, shapeType: shapeRectangle}},
		},
	}

	layoutWrapContainers(root, &Window{})

	if root.Shape.Axis != AxisLeftToRight {
		t.Error("axis should stay LTR")
	}
	if len(root.Children) != 3 {
		t.Errorf("children: got %d, want 3", len(root.Children))
	}
}

func TestWrapHeights(t *testing.T) {
	root := &Layout{
		Shape: &Shape{
			Axis: AxisLeftToRight, Wrap: true, Width: 80, Spacing: 10,
		},
		Children: []Layout{
			{Shape: &Shape{Width: 30, Height: 20, shapeType: shapeRectangle}},
			{Shape: &Shape{Width: 30, Height: 20, shapeType: shapeRectangle}},
			{Shape: &Shape{Width: 30, Height: 25, shapeType: shapeRectangle}},
		},
	}

	layoutWrapContainers(root, &Window{})
	layoutHeights(root)

	if len(root.Children) != 2 {
		t.Fatalf("rows: got %d", len(root.Children))
	}
	if !f32AreClose(root.Children[0].Shape.Height, 20) {
		t.Errorf("row 0 height: got %f, want 20", root.Children[0].Shape.Height)
	}
	if !f32AreClose(root.Children[1].Shape.Height, 25) {
		t.Errorf("row 1 height: got %f, want 25", root.Children[1].Shape.Height)
	}
	if !f32AreClose(root.Shape.Height, 55) {
		t.Errorf("container height: got %f, want 55", root.Shape.Height)
	}
}

func TestWrapPositions(t *testing.T) {
	root := &Layout{
		Shape: &Shape{
			Axis: AxisLeftToRight, Wrap: true,
			Width: 80, Height: 100, Sizing: FixedFixed, Spacing: 5,
		},
		Children: []Layout{
			{Shape: &Shape{Width: 30, Height: 20, shapeType: shapeRectangle}},
			{Shape: &Shape{Width: 30, Height: 20, shapeType: shapeRectangle}},
			{Shape: &Shape{Width: 30, Height: 15, shapeType: shapeRectangle}},
		},
	}

	layoutWrapContainers(root, &Window{})
	layoutHeights(root)
	layoutFillHeights(root)
	w := &Window{}
	layoutPositions(root, 0, 0, w)

	if !f32AreClose(root.Children[0].Shape.Y, 0) {
		t.Errorf("row 0 Y: got %f, want 0", root.Children[0].Shape.Y)
	}
	if !f32AreClose(root.Children[1].Shape.Y, 25) {
		t.Errorf("row 1 Y: got %f, want 25", root.Children[1].Shape.Y)
	}
	if !f32AreClose(root.Children[0].Children[0].Shape.X, 0) {
		t.Errorf("item 0 X: got %f, want 0", root.Children[0].Children[0].Shape.X)
	}
	if !f32AreClose(root.Children[0].Children[1].Shape.X, 35) {
		t.Errorf("item 1 X: got %f, want 35", root.Children[0].Children[1].Shape.X)
	}
}

func TestWrapNonFlow(t *testing.T) {
	root := &Layout{
		Shape: &Shape{
			Axis: AxisLeftToRight, Wrap: true, Width: 80, Spacing: 5,
		},
		Children: []Layout{
			{Shape: &Shape{Width: 30, shapeType: shapeRectangle}},
			{Shape: &Shape{Width: 10, shapeType: shapeRectangle, Float: true}},
			{Shape: &Shape{Width: 30, shapeType: shapeRectangle}},
			{Shape: &Shape{Width: 30, shapeType: shapeRectangle}},
		},
	}

	layoutWrapContainers(root, &Window{})

	if len(root.Children) != 2 {
		t.Fatalf("rows: got %d, want 2", len(root.Children))
	}
	if len(root.Children[0].Children) != 3 {
		t.Errorf("row 0 children: got %d, want 3", len(root.Children[0].Children))
	}
	if len(root.Children[1].Children) != 1 {
		t.Errorf("row 1 children: got %d, want 1", len(root.Children[1].Children))
	}
}

func TestWrapFillInColumn(t *testing.T) {
	root := &Layout{
		Shape: &Shape{
			Axis: AxisTopToBottom, Width: 400, Height: 300,
			Sizing: FixedFixed, shapeType: shapeRectangle,
		},
		Children: []Layout{
			{
				Shape: &Shape{
					Axis: AxisLeftToRight, Wrap: true,
					Sizing: FillFit, Spacing: 5, shapeType: shapeRectangle,
				},
				Children: []Layout{
					{Shape: &Shape{Width: 80, Height: 20, shapeType: shapeRectangle}},
					{Shape: &Shape{Width: 80, Height: 20, shapeType: shapeRectangle}},
					{Shape: &Shape{Width: 80, Height: 20, shapeType: shapeRectangle}},
					{Shape: &Shape{Width: 80, Height: 20, shapeType: shapeRectangle}},
					{Shape: &Shape{Width: 80, Height: 20, shapeType: shapeRectangle}},
					{Shape: &Shape{Width: 80, Height: 20, shapeType: shapeRectangle}},
				},
			},
		},
	}

	layoutWidths(root)
	layoutFillWidths(root)

	wrapLayout := &root.Children[0]
	if !f32AreClose(wrapLayout.Shape.Width, 400) {
		t.Errorf("wrap width: got %f, want 400", wrapLayout.Shape.Width)
	}

	layoutWrapContainers(root, &Window{})

	if root.Children[0].Shape.Axis != AxisTopToBottom {
		t.Error("wrap axis should flip to TTB")
	}
	if len(root.Children[0].Children) < 2 {
		t.Errorf("expected 2+ rows, got %d", len(root.Children[0].Children))
	}
}

func TestWrapLeadingSkippedChildren(t *testing.T) {
	root := &Layout{
		Shape: &Shape{
			Axis: AxisLeftToRight, Wrap: true, Width: 80, Spacing: 5,
		},
		Children: []Layout{
			{Shape: &Shape{Width: 10, shapeType: shapeRectangle, Float: true}},
			{Shape: &Shape{Width: 30, shapeType: shapeRectangle}},
			{Shape: &Shape{Width: 30, shapeType: shapeRectangle}},
			{Shape: &Shape{Width: 30, shapeType: shapeRectangle}},
		},
	}

	layoutWrapContainers(root, &Window{})

	if len(root.Children) != 2 {
		t.Fatalf("rows: got %d, want 2", len(root.Children))
	}
	if len(root.Children[0].Children) != 3 {
		t.Fatalf("row 0 children: got %d, want 3", len(root.Children[0].Children))
	}
	if len(root.Children[1].Children) != 1 {
		t.Fatalf("row 1 children: got %d, want 1", len(root.Children[1].Children))
	}
	if !root.Children[0].Children[0].Shape.Float {
		t.Fatalf("expected leading float child in row 0")
	}
}

func TestWrapTrailingNonFlowAtRowBoundary(t *testing.T) {
	root := &Layout{
		Shape: &Shape{
			Axis: AxisLeftToRight, Wrap: true, Width: 65, Spacing: 5,
		},
		Children: []Layout{
			{Shape: &Shape{Width: 30, shapeType: shapeRectangle}},
			{Shape: &Shape{Width: 30, shapeType: shapeRectangle}},
			{Shape: &Shape{Width: 10, shapeType: shapeRectangle, Float: true}},
			{Shape: &Shape{Width: 30, shapeType: shapeRectangle}},
		},
	}

	layoutWrapContainers(root, &Window{})

	if len(root.Children) != 2 {
		t.Fatalf("rows: got %d, want 2", len(root.Children))
	}
	if len(root.Children[0].Children) != 3 {
		t.Errorf("row 0 children: got %d, want 3 (2 flow + 1 float)",
			len(root.Children[0].Children))
	}
	if len(root.Children[1].Children) != 1 {
		t.Errorf("row 1 children: got %d, want 1", len(root.Children[1].Children))
	}
}

func TestWrapPadding(t *testing.T) {
	root := &Layout{
		Shape: &Shape{
			Axis: AxisLeftToRight, Wrap: true, Width: 100, Spacing: 5,
			Padding: NewPadding(0, 10, 0, 10),
		},
		Children: []Layout{
			{Shape: &Shape{Width: 30, shapeType: shapeRectangle}},
			{Shape: &Shape{Width: 30, shapeType: shapeRectangle}},
			{Shape: &Shape{Width: 30, shapeType: shapeRectangle}},
		},
	}

	// Available = 100 - 20 padding = 80. 30+5+30=65 fits, 65+5+30=100 > 80.
	layoutWrapContainers(root, &Window{})

	if len(root.Children) != 2 {
		t.Fatalf("rows: got %d, want 2", len(root.Children))
	}
	if len(root.Children[0].Children) != 2 {
		t.Errorf("row 0 children: got %d, want 2", len(root.Children[0].Children))
	}
	if len(root.Children[1].Children) != 1 {
		t.Errorf("row 1 children: got %d, want 1", len(root.Children[1].Children))
	}
}
