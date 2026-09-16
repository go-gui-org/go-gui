package gui

import "testing"

func TestLayoutPositionsStartAlign(t *testing.T) {
	root := &Layout{
		Shape: &Shape{
			X: 0, Y: 0, Width: 200, Height: 100,
			Axis:   axisLeftToRight,
			HAlign: HAlignLeft,
			VAlign: VAlignTop,
		},
		Children: []Layout{
			{Shape: &Shape{
				shapeType: shapeRectangle,
				Width:     40, Height: 30,
			}},
		},
	}
	w := &Window{}
	layoutParents(root, nil)
	layoutPositions(root, 0, 0, w)

	if !f32AreClose(root.Children[0].Shape.X, 0) {
		t.Errorf("X: got %f, want 0", root.Children[0].Shape.X)
	}
	if !f32AreClose(root.Children[0].Shape.Y, 0) {
		t.Errorf("Y: got %f, want 0", root.Children[0].Shape.Y)
	}
}

func TestLayoutPositionsEndAlign(t *testing.T) {
	root := &Layout{
		Shape: &Shape{
			X: 0, Y: 0, Width: 200, Height: 100,
			Axis:   axisLeftToRight,
			HAlign: HAlignRight,
			VAlign: VAlignBottom,
		},
		Children: []Layout{
			{Shape: &Shape{
				shapeType: shapeRectangle,
				Width:     40, Height: 30,
			}},
		},
	}
	w := &Window{}
	layoutParents(root, nil)
	layoutPositions(root, 0, 0, w)

	if !f32AreClose(root.Children[0].Shape.X, 160) {
		t.Errorf("X: got %f, want 160", root.Children[0].Shape.X)
	}
	if !f32AreClose(root.Children[0].Shape.Y, 70) {
		t.Errorf("Y: got %f, want 70", root.Children[0].Shape.Y)
	}
}

func TestLayoutPositionsMultipleChildrenLTR(t *testing.T) {
	root := &Layout{
		Shape: &Shape{
			X: 0, Y: 0, Width: 200, Height: 50,
			Axis:    axisLeftToRight,
			Spacing: 10,
		},
		Children: []Layout{
			{Shape: &Shape{shapeType: shapeRectangle, Width: 30, Height: 50}},
			{Shape: &Shape{shapeType: shapeRectangle, Width: 40, Height: 50}},
		},
	}
	w := &Window{}
	layoutParents(root, nil)
	layoutPositions(root, 0, 0, w)

	if !f32AreClose(root.Children[0].Shape.X, 0) {
		t.Errorf("c0 X: got %f, want 0", root.Children[0].Shape.X)
	}
	if !f32AreClose(root.Children[1].Shape.X, 40) {
		t.Errorf("c1 X: got %f, want 40", root.Children[1].Shape.X)
	}
}

func TestLayoutPositionsTTBColumn(t *testing.T) {
	root := &Layout{
		Shape: &Shape{
			X: 0, Y: 0, Width: 100, Height: 200,
			Axis:    axisTopToBottom,
			Spacing: 5,
		},
		Children: []Layout{
			{Shape: &Shape{shapeType: shapeRectangle, Width: 100, Height: 40}},
			{Shape: &Shape{shapeType: shapeRectangle, Width: 100, Height: 60}},
		},
	}
	w := &Window{}
	layoutParents(root, nil)
	layoutPositions(root, 0, 0, w)

	if !f32AreClose(root.Children[0].Shape.Y, 0) {
		t.Errorf("c0 Y: got %f, want 0", root.Children[0].Shape.Y)
	}
	if !f32AreClose(root.Children[1].Shape.Y, 45) {
		t.Errorf("c1 Y: got %f, want 45", root.Children[1].Shape.Y)
	}
}

func TestLayoutSetShapeClipsFullyOutside(t *testing.T) {
	root := &Layout{
		Shape: &Shape{X: 10, Y: 10, Width: 50, Height: 50},
		Children: []Layout{
			{Shape: &Shape{X: 200, Y: 200, Width: 10, Height: 10}},
		},
	}
	clip := drawClip{X: 0, Y: 0, Width: 500, Height: 500}
	layoutSetShapeClips(root, clip)

	c := root.Children[0].Shape.shapeClip
	if c.Width != 0 || c.Height != 0 {
		t.Errorf("fully outside child should have zero clip: %+v", c)
	}
}

func TestLayoutSetShapeClipsNestedClipping(t *testing.T) {
	root := &Layout{
		Shape: &Shape{X: 0, Y: 0, Width: 100, Height: 100, Clip: true},
		Children: []Layout{
			{
				Shape: &Shape{X: 10, Y: 10, Width: 80, Height: 80, Clip: true},
				Children: []Layout{
					{Shape: &Shape{X: 20, Y: 20, Width: 60, Height: 60}},
				},
			},
		},
	}
	clip := drawClip{X: 0, Y: 0, Width: 1000, Height: 1000}
	layoutSetShapeClips(root, clip)

	inner := root.Children[0].Children[0].Shape.shapeClip
	if !f32AreClose(inner.X, 20) || !f32AreClose(inner.Width, 60) {
		t.Errorf("inner clip: X=%f W=%f", inner.X, inner.Width)
	}
}

func TestLayoutPositionsRTLEndAlign(t *testing.T) {
	// RTL layout: HAlignEnd → HAlignLeft.
	root := &Layout{
		Shape: &Shape{
			X: 0, Y: 0, Width: 200, Height: 50,
			Axis:    axisLeftToRight,
			HAlign:  HAlignEnd,
			TextDir: TextDirRTL,
		},
		Children: []Layout{
			{Shape: &Shape{shapeType: shapeRectangle, Width: 40, Height: 30}},
		},
	}
	w := &Window{}
	layoutParents(root, nil)
	layoutPositions(root, 0, 0, w)

	// HAlignEnd with RTL resolves to HAlignLeft — child at X=0.
	if !f32AreClose(root.Children[0].Shape.X, 0) {
		t.Errorf("X: got %f, want 0", root.Children[0].Shape.X)
	}
}

func TestLayoutPositionsEndAlignLTR(t *testing.T) {
	// Non-RTL layout: HAlignEnd → HAlignRight.
	root := &Layout{
		Shape: &Shape{
			X: 0, Y: 0, Width: 200, Height: 50,
			Axis:    axisLeftToRight,
			HAlign:  HAlignEnd,
			TextDir: TextDirLTR,
		},
		Children: []Layout{
			{Shape: &Shape{shapeType: shapeRectangle, Width: 40, Height: 30}},
		},
	}
	w := &Window{}
	layoutParents(root, nil)
	layoutPositions(root, 0, 0, w)

	// HAlignEnd with LTR resolves to HAlignRight — child at X=160.
	if !f32AreClose(root.Children[0].Shape.X, 160) {
		t.Errorf("X: got %f, want 160", root.Children[0].Shape.X)
	}
}

// A Float child left in the tree (the extraction walk stops at the depth
// cap, and view_splitter calls layoutPositions directly) must not advance
// the cursor: sizing already skips it (skipLayoutChild), so advancing here
// would place its in-flow sibling past a width the parent never counted.
func TestLayoutPositionsFloatDoesNotAdvanceCursor(t *testing.T) {
	root := &Layout{
		Shape: &Shape{shapeType: shapeRectangle, Axis: axisLeftToRight, Width: 200, Height: 20},
		Children: []Layout{
			{Shape: &Shape{shapeType: shapeRectangle, Width: 50, Height: 20, Float: true}},
			{Shape: &Shape{shapeType: shapeRectangle, Width: 30, Height: 20}},
		},
	}
	layoutPositions(root, 0, 0, &Window{})
	if got := root.Children[1].Shape.X; !f32AreClose(got, 0) {
		t.Errorf("in-flow sibling X after a Float: got %f, want 0", got)
	}
}

// An overflowing centered row stays pinned at the start edge instead of
// shifting further off-screen (issue #636): remaining space is negative,
// so alignment must treat it as zero, as the cross-axis helpers do.
func TestLayoutPositionsCenterAlignOverflowPinned(t *testing.T) {
	root := &Layout{
		Shape: &Shape{
			X: 0, Y: 0, Width: 200, Height: 50,
			Axis:   axisLeftToRight,
			HAlign: HAlignCenter,
		},
		Children: []Layout{
			{Shape: &Shape{shapeType: shapeRectangle, Width: 120, Height: 50}},
			{Shape: &Shape{shapeType: shapeRectangle, Width: 120, Height: 50}},
		},
	}
	w := &Window{}
	layoutParents(root, nil)
	layoutPositions(root, 0, 0, w)

	if got := root.Children[0].Shape.X; !f32AreClose(got, 0) {
		t.Errorf("c0 X: got %f, want 0", got)
	}
	if got := root.Children[1].Shape.X; !f32AreClose(got, 120) {
		t.Errorf("c1 X: got %f, want 120", got)
	}
}

func TestLayoutPositionsEndAlignOverflowPinned(t *testing.T) {
	root := &Layout{
		Shape: &Shape{
			X: 0, Y: 0, Width: 200, Height: 50,
			Axis:   axisLeftToRight,
			HAlign: HAlignRight,
		},
		Children: []Layout{
			{Shape: &Shape{shapeType: shapeRectangle, Width: 120, Height: 50}},
			{Shape: &Shape{shapeType: shapeRectangle, Width: 120, Height: 50}},
		},
	}
	w := &Window{}
	layoutParents(root, nil)
	layoutPositions(root, 0, 0, w)

	if got := root.Children[0].Shape.X; !f32AreClose(got, 0) {
		t.Errorf("c0 X: got %f, want 0", got)
	}
}

func TestLayoutPositionsMiddleAlignOverflowPinned(t *testing.T) {
	root := &Layout{
		Shape: &Shape{
			X: 0, Y: 0, Width: 100, Height: 100,
			Axis:   axisTopToBottom,
			VAlign: VAlignMiddle,
		},
		Children: []Layout{
			{Shape: &Shape{shapeType: shapeRectangle, Width: 100, Height: 60}},
			{Shape: &Shape{shapeType: shapeRectangle, Width: 100, Height: 60}},
		},
	}
	w := &Window{}
	layoutParents(root, nil)
	layoutPositions(root, 0, 0, w)

	if got := root.Children[0].Shape.Y; !f32AreClose(got, 0) {
		t.Errorf("c0 Y: got %f, want 0", got)
	}
}

func TestLayoutPositionsBottomAlignOverflowPinned(t *testing.T) {
	root := &Layout{
		Shape: &Shape{
			X: 0, Y: 0, Width: 100, Height: 100,
			Axis:   axisTopToBottom,
			VAlign: VAlignBottom,
		},
		Children: []Layout{
			{Shape: &Shape{shapeType: shapeRectangle, Width: 100, Height: 60}},
			{Shape: &Shape{shapeType: shapeRectangle, Width: 100, Height: 60}},
		},
	}
	w := &Window{}
	layoutParents(root, nil)
	layoutPositions(root, 0, 0, w)

	if got := root.Children[0].Shape.Y; !f32AreClose(got, 0) {
		t.Errorf("c0 Y: got %f, want 0", got)
	}
}

func TestLayoutPositionsRTLCenterAlignOverflowPinned(t *testing.T) {
	root := &Layout{
		Shape: &Shape{
			X: 0, Y: 0, Width: 200, Height: 50,
			Axis:    axisLeftToRight,
			HAlign:  HAlignCenter,
			TextDir: TextDirRTL,
		},
		Children: []Layout{
			{Shape: &Shape{shapeType: shapeRectangle, Width: 120, Height: 50}},
			{Shape: &Shape{shapeType: shapeRectangle, Width: 120, Height: 50}},
		},
	}
	w := &Window{}
	layoutParents(root, nil)
	layoutPositions(root, 0, 0, w)

	// An RTL row starts at the right edge, so the first child stays
	// pinned there: 200 - 120 = 80.
	if got := root.Children[0].Shape.X; !f32AreClose(got, 80) {
		t.Errorf("c0 X: got %f, want 80", got)
	}
}
