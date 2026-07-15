package gui

import "testing"

func TestLayoutWidthsEmptyContainer(t *testing.T) {
	root := &Layout{
		Shape: &Shape{
			Axis:    AxisLeftToRight,
			Padding: Padding{Left: 5, Right: 5},
		},
	}
	layoutWidths(root)
	if !f32AreClose(root.Shape.Width, 10) {
		t.Errorf("width: got %f, want 10", root.Shape.Width)
	}
}

func TestLayoutHeightsEmptyContainer(t *testing.T) {
	root := &Layout{
		Shape: &Shape{
			Axis:    AxisTopToBottom,
			Padding: Padding{Top: 3, Bottom: 7},
		},
	}
	layoutHeights(root)
	if !f32AreClose(root.Shape.Height, 10) {
		t.Errorf("height: got %f, want 10", root.Shape.Height)
	}
}

func TestLayoutWidthsSingleChild(t *testing.T) {
	root := &Layout{
		Shape: &Shape{Axis: AxisLeftToRight},
		Children: []Layout{
			{Shape: &Shape{Width: 40}},
		},
	}
	layoutWidths(root)
	if !f32AreClose(root.Shape.Width, 40) {
		t.Errorf("width: got %f, want 40", root.Shape.Width)
	}
}

func TestLayoutHeightsSingleChild(t *testing.T) {
	root := &Layout{
		Shape: &Shape{Axis: AxisTopToBottom},
		Children: []Layout{
			{Shape: &Shape{Height: 25}},
		},
	}
	layoutHeights(root)
	if !f32AreClose(root.Shape.Height, 25) {
		t.Errorf("height: got %f, want 25", root.Shape.Height)
	}
}

func TestLayoutWidthsMaxWidthClamp(t *testing.T) {
	root := &Layout{
		Shape: &Shape{
			Axis:     AxisLeftToRight,
			MaxWidth: 60,
		},
		Children: []Layout{
			{Shape: &Shape{Width: 50}},
			{Shape: &Shape{Width: 50}},
		},
	}
	layoutWidths(root)
	if !f32AreClose(root.Shape.Width, 60) {
		t.Errorf("width: got %f, want 60", root.Shape.Width)
	}
}

func TestLayoutHeightsMaxHeightClamp(t *testing.T) {
	root := &Layout{
		Shape: &Shape{
			Axis:      AxisTopToBottom,
			MaxHeight: 40,
		},
		Children: []Layout{
			{Shape: &Shape{Height: 30}},
			{Shape: &Shape{Height: 30}},
		},
	}
	layoutHeights(root)
	if !f32AreClose(root.Shape.Height, 40) {
		t.Errorf("height: got %f, want 40", root.Shape.Height)
	}
}

func TestLayoutFillWidthsAllGrow(t *testing.T) {
	root := &Layout{
		Shape: &Shape{
			Axis:      AxisLeftToRight,
			shapeType: shapeRectangle,
			Sizing:    FixedFixed,
			Width:     90,
			Height:    50,
		},
		Children: []Layout{
			{Shape: &Shape{shapeType: shapeRectangle, Sizing: FillFill}},
			{Shape: &Shape{shapeType: shapeRectangle, Sizing: FillFill}},
			{Shape: &Shape{shapeType: shapeRectangle, Sizing: FillFill}},
		},
	}
	layoutWidths(root)
	layoutFillWidths(root, &scratchPools{})
	if !f32AreClose(root.Children[0].Shape.Width, 30) {
		t.Errorf("c0 width: got %f, want 30", root.Children[0].Shape.Width)
	}
	if !f32AreClose(root.Children[1].Shape.Width, 30) {
		t.Errorf("c1 width: got %f, want 30", root.Children[1].Shape.Width)
	}
	if !f32AreClose(root.Children[2].Shape.Width, 30) {
		t.Errorf("c2 width: got %f, want 30", root.Children[2].Shape.Width)
	}
}

func TestLayoutFillHeightsAllGrow(t *testing.T) {
	root := &Layout{
		Shape: &Shape{
			Axis:      AxisTopToBottom,
			shapeType: shapeRectangle,
			Sizing:    FixedFixed,
			Width:     50,
			Height:    60,
		},
		Children: []Layout{
			{Shape: &Shape{shapeType: shapeRectangle, Sizing: FillFill}},
			{Shape: &Shape{shapeType: shapeRectangle, Sizing: FillFill}},
		},
	}
	layoutHeights(root)
	layoutFillHeights(root, &scratchPools{})
	if !f32AreClose(root.Children[0].Shape.Height, 30) {
		t.Errorf("c0 height: got %f, want 30",
			root.Children[0].Shape.Height)
	}
	if !f32AreClose(root.Children[1].Shape.Height, 30) {
		t.Errorf("c1 height: got %f, want 30",
			root.Children[1].Shape.Height)
	}
}

func TestLayoutWidthsMinWidthFloor(t *testing.T) {
	root := &Layout{
		Shape: &Shape{
			Axis:     AxisLeftToRight,
			MinWidth: 100,
		},
		Children: []Layout{
			{Shape: &Shape{Width: 20}},
		},
	}
	layoutWidths(root)
	if root.Shape.Width < 100 {
		t.Errorf("width %f should be >= MinWidth 100",
			root.Shape.Width)
	}
}

func TestLayoutHeightsMinHeightFloor(t *testing.T) {
	root := &Layout{
		Shape: &Shape{
			Axis:      AxisTopToBottom,
			MinHeight: 80,
		},
		Children: []Layout{
			{Shape: &Shape{Height: 10}},
		},
	}
	layoutHeights(root)
	if root.Shape.Height < 80 {
		t.Errorf("height %f should be >= MinHeight 80",
			root.Shape.Height)
	}
}

func TestLayoutWidthsFixedSizingSkipsAccumulation(t *testing.T) {
	root := &Layout{
		Shape: &Shape{
			Axis:   AxisLeftToRight,
			Sizing: FixedFixed,
			Width:  200,
		},
		Children: []Layout{
			{Shape: &Shape{Width: 50}},
			{Shape: &Shape{Width: 50}},
		},
	}
	layoutWidths(root)
	// Fixed width root should stay at 200.
	if !f32AreClose(root.Shape.Width, 200) {
		t.Errorf("width: got %f, want 200", root.Shape.Width)
	}
}

func TestLayoutFillWidths_NilPool(t *testing.T) {
	root := &Layout{
		Shape: &Shape{
			Sizing: FixedFixed,
			Width:  200,
			Height: 100,
			Axis:   AxisLeftToRight,
		},
		Children: []Layout{
			{Shape: &Shape{
				Sizing: FillFixed,
				Width:  50,
			}},
			{Shape: &Shape{
				Sizing: FillFixed,
				Width:  50,
			}},
		},
	}
	layoutWidths(root)
	// nil pool — should fall back to local slices without panic.
	layoutFillWidths(root, nil)

	// Each child should have grown: 200 - 100 = 100 remaining,
	// split equally = 50 each → final widths = 100 each.
	if !f32AreClose(root.Children[0].Shape.Width, 100) {
		t.Errorf("c0 width: got %f, want 100",
			root.Children[0].Shape.Width)
	}
	if !f32AreClose(root.Children[1].Shape.Width, 100) {
		t.Errorf("c1 width: got %f, want 100",
			root.Children[1].Shape.Width)
	}
}

func TestLayoutFillHeights_NilPool(t *testing.T) {
	root := &Layout{
		Shape: &Shape{
			Sizing: FixedFixed,
			Width:  200,
			Height: 200,
			Axis:   AxisTopToBottom,
		},
		Children: []Layout{
			{Shape: &Shape{
				Sizing: FixedFill,
				Height: 50,
			}},
			{Shape: &Shape{
				Sizing: FixedFill,
				Height: 50,
			}},
		},
	}
	layoutHeights(root)
	// nil pool — should fall back to local slices without panic.
	layoutFillHeights(root, nil)

	if !f32AreClose(root.Children[0].Shape.Height, 100) {
		t.Errorf("c0 height: got %f, want 100",
			root.Children[0].Shape.Height)
	}
	if !f32AreClose(root.Children[1].Shape.Height, 100) {
		t.Errorf("c1 height: got %f, want 100",
			root.Children[1].Shape.Height)
	}
}

func TestLayoutFillWidths_CachesContentDimensions(t *testing.T) {
	root := &Layout{
		Shape: &Shape{
			Sizing: FixedFixed,
			Width:  200,
			Height: 100,
			Axis:   AxisLeftToRight,
		},
		Children: []Layout{
			{Shape: &Shape{
				shapeType: shapeRectangle,
				Sizing:    FixedFixed,
				Width:     40, Height: 20,
			}},
			{Shape: &Shape{
				shapeType: shapeRectangle,
				Sizing:    FixedFixed,
				Width:     60, Height: 20,
			}},
		},
	}
	var p scratchPools
	p.beginFillPass()

	layoutWidths(root)
	layoutFillWidths(root, &p)

	// After fill pass, fillGen must be non-zero so contentWidth
	// returns the cached value.
	if root.Shape.fillGen == 0 {
		t.Fatal("fillGen must be set after fill pass")
	}
	if root.Shape.contentW == 0 {
		t.Fatal("contentW must be cached after fill pass")
	}

	// Verify cached value matches a direct computation.
	want := computeContentWidth(root)
	if !f32AreClose(root.Shape.contentW, want) {
		t.Errorf("cached contentW = %f, want %f", root.Shape.contentW, want)
	}

	// contentWidth must return cached value (fillGen != 0 path).
	got := contentWidth(root)
	if !f32AreClose(got, root.Shape.contentW) {
		t.Errorf("contentWidth = %f, want cached %f", got, root.Shape.contentW)
	}
}

func TestLayoutFillHeights_CachesContentDimensions(t *testing.T) {
	root := &Layout{
		Shape: &Shape{
			Sizing: FixedFixed,
			Width:  200,
			Height: 200,
			Axis:   AxisTopToBottom,
		},
		Children: []Layout{
			{Shape: &Shape{
				shapeType: shapeRectangle,
				Sizing:    FixedFixed,
				Width:     50, Height: 40,
			}},
			{Shape: &Shape{
				shapeType: shapeRectangle,
				Sizing:    FixedFixed,
				Width:     50, Height: 60,
			}},
		},
	}
	var p scratchPools
	p.beginFillPass()

	layoutHeights(root)
	layoutFillHeights(root, &p)

	if root.Shape.fillGen == 0 {
		t.Fatal("fillGen must be set after fill pass")
	}
	if root.Shape.contentH == 0 {
		t.Fatal("contentH must be cached after fill pass")
	}

	want := computeContentHeight(root)
	if !f32AreClose(root.Shape.contentH, want) {
		t.Errorf("cached contentH = %f, want %f", root.Shape.contentH, want)
	}

	got := contentHeight(root)
	if !f32AreClose(got, root.Shape.contentH) {
		t.Errorf("contentHeight = %f, want cached %f", got, root.Shape.contentH)
	}
}

func TestLayoutFillCrossAxis_SiblingSumCache(t *testing.T) {
	// LTR parent with multiple TTB scroll-fill children.
	// First child triggers sibling-sum computation + caching;
	// subsequent children reuse the cached sum.
	parent := &Layout{
		Shape: &Shape{
			Sizing:    FixedFixed,
			Width:     300,
			Height:    100,
			Axis:      AxisLeftToRight,
			shapeType: shapeRectangle,
		},
		Children: []Layout{
			{Shape: &Shape{
				shapeType:  shapeRectangle,
				Sizing:     FillFill,
				Axis:       AxisTopToBottom,
				Scrollable: true,
				ID:         "1",
				Width:      0, Height: 20,
			}},
			{Shape: &Shape{
				shapeType:  shapeRectangle,
				Sizing:     FillFill,
				Axis:       AxisTopToBottom,
				Scrollable: true,
				ID:         "2",
				Width:      0, Height: 20,
			}},
			{Shape: &Shape{
				shapeType: shapeRectangle,
				Sizing:    FixedFixed,
				Axis:      AxisTopToBottom,
				Width:     50, Height: 20,
			}},
		},
	}
	layoutParents(parent, nil)

	var p scratchPools
	p.beginFillPass()
	layoutWidths(parent)
	layoutFillWidths(parent, &p)

	// Parent siblingSumW must be cached after fill.
	if parent.Shape.siblingSumW == 0 {
		t.Fatal("siblingSumW must be cached on parent after cross-axis fill")
	}
	if parent.Shape.siblingSumGen == 0 {
		t.Fatal("parent siblingSumGen must be set")
	}

	// All children must have non-zero, finite widths.
	for i := range parent.Children {
		ch := parent.Children[i].Shape
		if ch.Width <= 0 || !f32IsFinite(ch.Width) {
			t.Errorf("child %d width = %f, want > 0 and finite", i, ch.Width)
		}
	}

	// The two fill children must get equal remaining width:
	// 300 (parent) - 50 (fixed child) = 250, split equally = 125 each.
	c0w := parent.Children[0].Shape.Width
	c1w := parent.Children[1].Shape.Width
	if !f32AreClose(c0w, 125) {
		t.Errorf("child 0 width = %f, want 125", c0w)
	}
	if !f32AreClose(c1w, 125) {
		t.Errorf("child 1 width = %f, want 125", c1w)
	}

	// Fixed child must keep its width.
	c2w := parent.Children[2].Shape.Width
	if !f32AreClose(c2w, 50) {
		t.Errorf("child 2 width = %f, want 50", c2w)
	}
}
