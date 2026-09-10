package gui

import "testing"

func TestLayoutWidthsEmptyContainer(t *testing.T) {
	root := &Layout{
		Shape: &Shape{
			Axis:    axisLeftToRight,
			Padding: NewPadding(0, 5, 0, 5),
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
			Axis:    axisTopToBottom,
			Padding: NewPadding(3, 0, 7, 0),
		},
	}
	layoutHeights(root)
	if !f32AreClose(root.Shape.Height, 10) {
		t.Errorf("height: got %f, want 10", root.Shape.Height)
	}
}

func TestLayoutWidthsSingleChild(t *testing.T) {
	root := &Layout{
		Shape: &Shape{shapeType: shapeRectangle, Axis: axisLeftToRight},
		Children: []Layout{
			{Shape: &Shape{shapeType: shapeRectangle, Width: 40}},
		},
	}
	layoutWidths(root)
	if !f32AreClose(root.Shape.Width, 40) {
		t.Errorf("width: got %f, want 40", root.Shape.Width)
	}
}

func TestLayoutHeightsSingleChild(t *testing.T) {
	root := &Layout{
		Shape: &Shape{shapeType: shapeRectangle, Axis: axisTopToBottom},
		Children: []Layout{
			{Shape: &Shape{shapeType: shapeRectangle, Height: 25}},
		},
	}
	layoutHeights(root)
	if !f32AreClose(root.Shape.Height, 25) {
		t.Errorf("height: got %f, want 25", root.Shape.Height)
	}
}

func TestLayoutWidthsMaxWidthClamp(t *testing.T) {
	root := &Layout{
		Shape: &Shape{shapeType: shapeRectangle,
			Axis:     axisLeftToRight,
			MaxWidth: 60,
		},
		Children: []Layout{
			{Shape: &Shape{shapeType: shapeRectangle, Width: 50}},
			{Shape: &Shape{shapeType: shapeRectangle, Width: 50}},
		},
	}
	layoutWidths(root)
	if !f32AreClose(root.Shape.Width, 60) {
		t.Errorf("width: got %f, want 60", root.Shape.Width)
	}
}

func TestLayoutHeightsMaxHeightClamp(t *testing.T) {
	root := &Layout{
		Shape: &Shape{shapeType: shapeRectangle,
			Axis:      axisTopToBottom,
			MaxHeight: 40,
		},
		Children: []Layout{
			{Shape: &Shape{shapeType: shapeRectangle, Height: 30}},
			{Shape: &Shape{shapeType: shapeRectangle, Height: 30}},
		},
	}
	layoutHeights(root)
	if !f32AreClose(root.Shape.Height, 40) {
		t.Errorf("height: got %f, want 40", root.Shape.Height)
	}
}

// axisNone has no children to fit against, so the pass only honors
// explicit min/max pins — the Fill root pin from updateLayoutLocked
// (issue #262). Without a pin the size must stay untouched.
func TestLayoutWidthsAxisNoneHonorsMinMax(t *testing.T) {
	root := &Layout{Shape: &Shape{Axis: axisNone}}
	layoutWidths(root)
	if root.Shape.Width != 0 {
		t.Errorf("width: got %f, want 0 (no pin)", root.Shape.Width)
	}

	root.Shape.MinWidth = 400
	root.Shape.MaxWidth = 400
	layoutWidths(root)
	if !f32AreClose(root.Shape.Width, 400) {
		t.Errorf("width: got %f, want 400 (Fill root pin)", root.Shape.Width)
	}
}

func TestLayoutHeightsAxisNoneHonorsMinMax(t *testing.T) {
	root := &Layout{Shape: &Shape{Axis: axisNone}}
	layoutHeights(root)
	if root.Shape.Height != 0 {
		t.Errorf("height: got %f, want 0 (no pin)", root.Shape.Height)
	}

	root.Shape.MinHeight = 300
	root.Shape.MaxHeight = 300
	layoutHeights(root)
	if !f32AreClose(root.Shape.Height, 300) {
		t.Errorf("height: got %f, want 300 (Fill root pin)", root.Shape.Height)
	}
}

// An explicit max alone clamps an axisNone shape; a min below the
// current size must not shrink it.
func TestLayoutWidthsAxisNoneHonorsMax(t *testing.T) {
	root := &Layout{
		Shape: &Shape{
			Axis:     axisNone,
			Width:    500,
			MaxWidth: 400,
		},
	}
	layoutWidths(root)
	if !f32AreClose(root.Shape.Width, 400) {
		t.Errorf("width: got %f, want 400", root.Shape.Width)
	}
}

// The axisNone branch must not grow into the children's job: children
// of an axis-less container keep the sizes they arrived with (they are
// positioned by the container's AmendLayout, e.g. the splitter), so the
// pass recurses nowhere.
func TestLayoutWidthsAxisNoneLeavesChildrenAlone(t *testing.T) {
	root := &Layout{
		Shape: &Shape{
			Axis:     axisNone,
			MinWidth: 400,
			MaxWidth: 400,
		},
		Children: []Layout{
			{Shape: &Shape{Width: 30}},
			{Shape: &Shape{Width: 20}},
		},
	}
	layoutWidths(root)
	if root.Children[0].Shape.Width != 30 || root.Children[1].Shape.Width != 20 {
		t.Error("axisNone fit pass must not re-size children (they keep " +
			"their arrival sizes)")
	}
}

// The general case of #262, through the full pipeline: a plain axis-less
// FillFill root (a Canvas, not a splitter) resolves to the window size —
// the fix lives in the sizing passes, not in the splitter.
func TestCanvasFillFillRootFillsWindow(t *testing.T) {
	w := NewTestWindow(WindowCfg{Width: 400, Height: 300})
	w.TestRender(func(_ *Window) View {
		return Canvas(ContainerCfg{
			ID:      "c",
			Sizing:  FillFill,
			Content: []View{Text(TextCfg{Text: "content"})},
		})
	})
	root, ok := w.layout.FindByID("c")
	if !ok {
		t.Fatal("canvas root not found")
	}
	nearF(t, "root width", root.Shape.Width, 400, 0.01)
	nearF(t, "root height", root.Shape.Height, 300, 0.01)
}

func TestLayoutFillWidthsAllGrow(t *testing.T) {
	root := &Layout{
		Shape: &Shape{
			Axis:      axisLeftToRight,
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
			Axis:      axisTopToBottom,
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
			Axis:     axisLeftToRight,
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

// TestRowAndColumnMinWidthAgree guards issue #385: layoutWidths used to pad a
// row's stated MinWidth with its padding and inter-child gap sum (content-box)
// while the column branch read the same field as border-box, so a Row and a
// Column with identical padding, spacing and stated MinWidth arranged at
// different widths. A stated MinWidth is the caller's whole width budget —
// border-box, like MaxWidth — on both axes.
func TestRowAndColumnMinWidthAgree(t *testing.T) {
	// minWidthBox builds a PadAll(5)/Spacing 10 box with two 40px children;
	// childMin is the MinWidth each child carries, 0 for none.
	minWidthBox := func(axis Axis, stated, childMin float32) *Layout {
		return &Layout{
			Shape: &Shape{
				Axis:      axis,
				MinWidth:  stated,
				Padding:   PadAll(5),
				Spacing:   10,
				shapeType: shapeRectangle,
			},
			Children: []Layout{
				{Shape: &Shape{Width: 40, MinWidth: childMin, Height: 20, shapeType: shapeRectangle}},
				{Shape: &Shape{Width: 40, MinWidth: childMin, Height: 20, shapeType: shapeRectangle}},
			},
		}
	}

	row := minWidthBox(axisLeftToRight, 160, 0)
	col := minWidthBox(axisTopToBottom, 160, 0)
	layoutWidths(row)
	layoutWidths(col)

	if !f32AreClose(row.Shape.MinWidth, 160) {
		t.Errorf("row MinWidth: got %f, want 160 (border-box, padding and "+
			"spacing not added on top)", row.Shape.MinWidth)
	}
	if !f32AreClose(row.Shape.Width, 160) {
		t.Errorf("row width: got %f, want 160", row.Shape.Width)
	}
	if !f32AreClose(col.Shape.MinWidth, 160) {
		t.Errorf("col MinWidth: got %f, want 160", col.Shape.MinWidth)
	}
	if !f32AreClose(col.Shape.Width, 160) {
		t.Errorf("col width: got %f, want 160", col.Shape.Width)
	}

	// The child-min floor still wins over a smaller stated MinWidth. The
	// row sums its children plus gaps (5+5 padding + 10 spacing + 200);
	// the column is cross-axis, so it takes the widest child plus padding
	// (110). The defect is the stated-MinWidth reading, not the content
	// floor, so both must exceed their stated 100.
	row = minWidthBox(axisLeftToRight, 100, 100)
	col = minWidthBox(axisTopToBottom, 100, 100)
	layoutWidths(row)
	layoutWidths(col)
	if !f32AreClose(row.Shape.Width, 220) {
		t.Errorf("row width with child mins: got %f, want 220",
			row.Shape.Width)
	}
	if !f32AreClose(col.Shape.Width, 110) {
		t.Errorf("col width with child mins: got %f, want 110 (widest child "+
			"+ padding)", col.Shape.Width)
	}
}

func TestLayoutHeightsMinHeightFloor(t *testing.T) {
	root := &Layout{
		Shape: &Shape{
			Axis:      axisTopToBottom,
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
			Axis:   axisLeftToRight,
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

// A Fixed width of 0 must degrade to content sizing so the box's bounds
// enclose its children — otherwise the zero-width box collapses the
// shapeClip (and hence hit-test and clip region) of every descendant.
// Regression for issue #94.
func TestLayoutWidthsFixedZeroDegradesToContent(t *testing.T) {
	// Main axis (row width).
	row := &Layout{
		Shape: &Shape{shapeType: shapeRectangle, Axis: axisLeftToRight, Sizing: FixedFixed},
		Children: []Layout{
			{Shape: &Shape{shapeType: shapeRectangle, Width: 50}},
			{Shape: &Shape{shapeType: shapeRectangle, Width: 50}},
		},
	}
	layoutWidths(row)
	if !f32AreClose(row.Shape.Width, 100) {
		t.Errorf("row width: got %f, want 100 (content)", row.Shape.Width)
	}

	// Cross axis (column width).
	col := &Layout{
		Shape: &Shape{shapeType: shapeRectangle, Axis: axisTopToBottom, Sizing: FixedFixed},
		Children: []Layout{
			{Shape: &Shape{shapeType: shapeRectangle, Width: 70}},
		},
	}
	layoutWidths(col)
	if !f32AreClose(col.Shape.Width, 70) {
		t.Errorf("col width: got %f, want 70 (content)", col.Shape.Width)
	}

	// Childless Fixed-zero box stays 0 (leaf images/svg unaffected).
	leaf := &Layout{Shape: &Shape{shapeType: shapeRectangle, Axis: axisLeftToRight, Sizing: FixedFixed}}
	layoutWidths(leaf)
	if !f32AreClose(leaf.Shape.Width, 0) {
		t.Errorf("leaf width: got %f, want 0", leaf.Shape.Width)
	}
}

// Mirror of the width case for the height axis. Regression for #94.
func TestLayoutHeightsFixedZeroDegradesToContent(t *testing.T) {
	// Main axis (column height).
	col := &Layout{
		Shape: &Shape{shapeType: shapeRectangle, Axis: axisTopToBottom, Sizing: FixedFixed},
		Children: []Layout{
			{Shape: &Shape{shapeType: shapeRectangle, Height: 30}},
			{Shape: &Shape{shapeType: shapeRectangle, Height: 30}},
		},
	}
	layoutHeights(col)
	if !f32AreClose(col.Shape.Height, 60) {
		t.Errorf("col height: got %f, want 60 (content)", col.Shape.Height)
	}

	// Cross axis (row height).
	row := &Layout{
		Shape: &Shape{shapeType: shapeRectangle, Axis: axisLeftToRight, Sizing: FixedFixed},
		Children: []Layout{
			{Shape: &Shape{shapeType: shapeRectangle, Height: 45}},
		},
	}
	layoutHeights(row)
	if !f32AreClose(row.Shape.Height, 45) {
		t.Errorf("row height: got %f, want 45 (content)", row.Shape.Height)
	}

	// Childless Fixed-zero box stays 0.
	leaf := &Layout{Shape: &Shape{shapeType: shapeRectangle, Axis: axisTopToBottom, Sizing: FixedFixed}}
	layoutHeights(leaf)
	if !f32AreClose(leaf.Shape.Height, 0) {
		t.Errorf("leaf height: got %f, want 0", leaf.Shape.Height)
	}
}

func TestLayoutFillWidths_NilPool(t *testing.T) {
	root := &Layout{
		Shape: &Shape{shapeType: shapeRectangle,
			Sizing: FixedFixed,
			Width:  200,
			Height: 100,
			Axis:   axisLeftToRight,
		},
		Children: []Layout{
			{Shape: &Shape{shapeType: shapeRectangle,
				Sizing: FillFixed,
				Width:  50,
			}},
			{Shape: &Shape{shapeType: shapeRectangle,
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
		Shape: &Shape{shapeType: shapeRectangle,
			Sizing: FixedFixed,
			Width:  200,
			Height: 200,
			Axis:   axisTopToBottom,
		},
		Children: []Layout{
			{Shape: &Shape{shapeType: shapeRectangle,
				Sizing: FixedFill,
				Height: 50,
			}},
			{Shape: &Shape{shapeType: shapeRectangle,
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
			Axis:   axisLeftToRight,
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
			Axis:   axisTopToBottom,
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
			Axis:      axisLeftToRight,
			shapeType: shapeRectangle,
		},
		Children: []Layout{
			{Shape: &Shape{
				shapeType:  shapeRectangle,
				Sizing:     FillFill,
				Axis:       axisTopToBottom,
				Scrollable: true,
				ID:         "1",
				Width:      0, Height: 20,
			}},
			{Shape: &Shape{
				shapeType:  shapeRectangle,
				Sizing:     FillFill,
				Axis:       axisTopToBottom,
				Scrollable: true,
				ID:         "2",
				Width:      0, Height: 20,
			}},
			{Shape: &Shape{
				shapeType: shapeRectangle,
				Sizing:    FixedFixed,
				Axis:      axisTopToBottom,
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

// TestRowAndColumnMinHeightAgree is the height mirror of
// TestRowAndColumnMinWidthAgree (issue #385). layoutHeights used to read a
// stated MinHeight as content-box — it added padding and the inter-child gap
// sum on top — while layoutWidths had already been corrected to border-box.
// A Row and a Column with identical padding, spacing and stated minimum must
// arrange at the same size on both axes.
func TestRowAndColumnMinHeightAgree(t *testing.T) {
	// minHeightBox builds a PadAll(5)/Spacing 10 box with two 20px-tall
	// children; childMin is the MinHeight each child carries, 0 for none.
	minHeightBox := func(axis Axis, stated, childMin float32) *Layout {
		return &Layout{
			Shape: &Shape{
				Axis:      axis,
				MinHeight: stated,
				Padding:   PadAll(5),
				Spacing:   10,
				shapeType: shapeRectangle,
			},
			Children: []Layout{
				{Shape: &Shape{Width: 40, Height: 20, MinHeight: childMin, shapeType: shapeRectangle}},
				{Shape: &Shape{Width: 40, Height: 20, MinHeight: childMin, shapeType: shapeRectangle}},
			},
		}
	}

	col := minHeightBox(axisTopToBottom, 160, 0)
	row := minHeightBox(axisLeftToRight, 160, 0)
	layoutHeights(col)
	layoutHeights(row)

	if !f32AreClose(col.Shape.MinHeight, 160) {
		t.Errorf("col MinHeight: got %f, want 160 (border-box, padding and "+
			"spacing not added on top)", col.Shape.MinHeight)
	}
	if !f32AreClose(col.Shape.Height, 160) {
		t.Errorf("col height: got %f, want 160", col.Shape.Height)
	}
	if !f32AreClose(row.Shape.MinHeight, 160) {
		t.Errorf("row MinHeight: got %f, want 160", row.Shape.MinHeight)
	}
	if !f32AreClose(row.Shape.Height, 160) {
		t.Errorf("row height: got %f, want 160", row.Shape.Height)
	}

	// The child-min floor still wins over a smaller stated MinHeight: the
	// column sums bare child minimums and adds its own padding and spacing
	// (5+5 padding + 10 spacing + 2*50), so the floor is 120, not the
	// stated 40.
	tall := minHeightBox(axisTopToBottom, 40, 50)
	layoutHeights(tall)
	if !f32AreClose(tall.Shape.MinHeight, 120) {
		t.Errorf("col MinHeight with child minimums: got %f, want 120",
			tall.Shape.MinHeight)
	}
}

// TestOutOfFlowChildDoesNotSizeParent guards the child set the sizing passes
// measure. layoutWidths, layoutHeights and the two fill passes used to skip
// only OverDraw children, while layout.spacing() and computeContentWidth skip
// everything skipLayoutChild names — Float and shapeNone included. A Float
// child therefore added its width to the parent's content sum while taking no
// fence post, so an out-of-flow child silently widened its container.
func TestOutOfFlowChildDoesNotSizeParent(t *testing.T) {
	// flowBox builds a Fit row of two 40px children, optionally with a
	// wide out-of-flow child appended.
	flowBox := func(extra *Shape) *Layout {
		l := &Layout{
			Shape: &Shape{
				Axis:      axisLeftToRight,
				Spacing:   10,
				shapeType: shapeRectangle,
			},
			Children: []Layout{
				{Shape: &Shape{Width: 40, Height: 20, shapeType: shapeRectangle}},
				{Shape: &Shape{Width: 40, Height: 20, shapeType: shapeRectangle}},
			},
		}
		if extra != nil {
			l.Children = append(l.Children, Layout{Shape: extra})
		}
		return l
	}

	base := flowBox(nil)
	layoutWidths(base)
	// Two 40px children plus one 10px gap.
	if !f32AreClose(base.Shape.Width, 90) {
		t.Fatalf("baseline row width: got %f, want 90", base.Shape.Width)
	}

	cases := []struct {
		name  string
		extra *Shape
	}{
		{"float", &Shape{Float: true, Width: 200, Height: 20, shapeType: shapeRectangle}},
		{"shapeNone", &Shape{Width: 200, Height: 20, shapeType: shapeNone}},
		{"overDraw", &Shape{OverDraw: true, Width: 200, Height: 20, shapeType: shapeRectangle}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := flowBox(tc.extra)
			layoutWidths(got)
			if !f32AreClose(got.Shape.Width, base.Shape.Width) {
				t.Errorf("row width with an out-of-flow child: got %f, want %f",
					got.Shape.Width, base.Shape.Width)
			}
		})
	}
}

// TestOutOfFlowChildDoesNotFitParentCrossAxis is the cross-axis half of
// TestOutOfFlowChildDoesNotSizeParent. The cross-axis fit loops in
// layoutWidths and layoutHeights measured every child, while
// computeContentWidth applies skipLayoutChild on both axes — so a wide Float
// inside a Column still stretched the Column to its width.
func TestOutOfFlowChildDoesNotFitParentCrossAxis(t *testing.T) {
	// colBox is a Fit-width Column of two 40px-wide children, optionally
	// with a much wider out-of-flow child appended.
	colBox := func(extra *Shape) *Layout {
		l := &Layout{
			Shape: &Shape{
				Axis:      axisTopToBottom,
				shapeType: shapeRectangle,
			},
			Children: []Layout{
				{Shape: &Shape{Width: 40, Height: 20, shapeType: shapeRectangle}},
				{Shape: &Shape{Width: 40, Height: 20, shapeType: shapeRectangle}},
			},
		}
		if extra != nil {
			l.Children = append(l.Children, Layout{Shape: extra})
		}
		return l
	}

	base := colBox(nil)
	layoutWidths(base)
	if !f32AreClose(base.Shape.Width, 40) {
		t.Fatalf("baseline column width: got %f, want 40", base.Shape.Width)
	}

	cases := []struct {
		name  string
		extra *Shape
	}{
		{"float", &Shape{Float: true, Width: 300, Height: 20, shapeType: shapeRectangle}},
		{"shapeNone", &Shape{Width: 300, Height: 20, shapeType: shapeNone}},
		{"overDraw", &Shape{OverDraw: true, Width: 300, Height: 20, shapeType: shapeRectangle}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := colBox(tc.extra)
			layoutWidths(got)
			if !f32AreClose(got.Shape.Width, base.Shape.Width) {
				t.Errorf("column width with an out-of-flow child: got %f, want %f",
					got.Shape.Width, base.Shape.Width)
			}
		})
	}

	// The out-of-flow child is still measured; only its contribution to
	// the parent's fit is dropped. A Float whose own width comes from its
	// children must still resolve.
	nested := colBox(&Shape{Float: true, Axis: axisLeftToRight, shapeType: shapeRectangle})
	nested.Children[2].Children = []Layout{
		{Shape: &Shape{Width: 70, Height: 10, shapeType: shapeRectangle}},
	}
	layoutWidths(nested)
	if !f32AreClose(nested.Children[2].Shape.Width, 70) {
		t.Errorf("float subtree not measured: got %f, want 70",
			nested.Children[2].Shape.Width)
	}
}

// TestCrossAxisFillIgnoresOutOfFlowSibling guards the sibling sum in
// layoutFillCrossAxis. It summed every child but subtracted
// layout.Parent.spacing(), which counts in-flow children only, so a Float
// sibling stole width from a Scrollable cross-axis Fill child.
func TestCrossAxisFillIgnoresOutOfFlowSibling(t *testing.T) {
	// Same shape as TestLayoutFillCrossAxis_SiblingSumCache: an LTR parent
	// whose TTB scrollable child fills across the parent's main axis, which
	// is what reaches the sibling-sum branch.
	rowWith := func(extra ...*Shape) *Layout {
		l := &Layout{
			Shape: &Shape{
				Sizing:    FixedFixed,
				Width:     300,
				Height:    100,
				Axis:      axisLeftToRight,
				shapeType: shapeRectangle,
			},
			Children: []Layout{
				{Shape: &Shape{
					shapeType:  shapeRectangle,
					Sizing:     FillFill,
					Axis:       axisTopToBottom,
					Scrollable: true,
					ID:         "fill",
					Width:      0, Height: 20,
				}},
				{Shape: &Shape{
					shapeType: shapeRectangle,
					Sizing:    FixedFixed,
					Axis:      axisTopToBottom,
					Width:     50, Height: 20,
				}},
			},
		}
		for _, e := range extra {
			l.Children = append(l.Children, Layout{Shape: e})
		}
		layoutParents(l, nil)
		return l
	}

	fillWidth := func(l *Layout) float32 {
		var p scratchPools
		p.beginFillPass()
		layoutFillWidths(l, &p)
		return l.Children[0].Shape.Width
	}

	// 300 parent less the 50px fixed sibling.
	base := fillWidth(rowWith())
	if !f32AreClose(base, 250) {
		t.Fatalf("baseline fill width: got %f, want 250", base)
	}

	withFloat := fillWidth(rowWith(&Shape{
		shapeType: shapeRectangle,
		Sizing:    FixedFixed,
		Axis:      axisTopToBottom,
		Float:     true,
		Width:     80, Height: 20,
	}))
	if !f32AreClose(withFloat, base) {
		t.Errorf("fill width with a Float sibling: got %f, want %f",
			withFloat, base)
	}
}

// TestCrossAxisFitSeedsPadding pins the two ends of the cross-axis padding
// rule, which the out-of-flow skip made load-bearing. Padding used to reach
// a Fit container only as part of a child's contribution, so once the fit
// stopped measuring out-of-flow children an empty group box — whose only
// child is a shapeNone placeholder — collapsed to zero instead of to its
// padding. Seeding it is gated on having children, because a container with
// none must still collapse: a closed Sidebar has zero children and a 1px
// border per side, and has to stay shut.
func TestCrossAxisFitSeedsPadding(t *testing.T) {
	withPlaceholder := &Layout{
		Shape: &Shape{
			Axis:      axisTopToBottom,
			Padding:   PadAll(15),
			shapeType: shapeRectangle,
		},
		Children: []Layout{layoutPlaceholder()},
	}
	layoutWidths(withPlaceholder)
	if !f32AreClose(withPlaceholder.Shape.Width, 30) {
		t.Errorf("placeholder-only container width: got %f, want 30 (its "+
			"own padding)", withPlaceholder.Shape.Width)
	}

	childless := &Layout{
		Shape: &Shape{
			Axis:      axisTopToBottom,
			Padding:   PadAll(15),
			shapeType: shapeRectangle,
		},
	}
	layoutWidths(childless)
	if !f32AreClose(childless.Shape.Width, 0) {
		t.Errorf("childless container width: got %f, want 0 (must collapse, "+
			"not fall back to padding)", childless.Shape.Width)
	}
}

// TestCrossAxisFitSeedsPaddingHeight is the height mirror of
// TestCrossAxisFitSeedsPadding. The LTR cross-axis fit seeds Height to the
// container's own padding for the same reason the TTB one seeds Width:
// once out-of-flow children stop contributing, a placeholder-only box must
// still resolve to its padding while a childless one collapses to 0.
func TestCrossAxisFitSeedsPaddingHeight(t *testing.T) {
	withPlaceholder := &Layout{
		Shape: &Shape{
			Axis:      axisLeftToRight,
			Padding:   PadAll(15),
			shapeType: shapeRectangle,
		},
		Children: []Layout{layoutPlaceholder()},
	}
	layoutHeights(withPlaceholder)
	if !f32AreClose(withPlaceholder.Shape.Height, 30) {
		t.Errorf("placeholder-only container height: got %f, want 30 (its "+
			"own padding)", withPlaceholder.Shape.Height)
	}

	childless := &Layout{
		Shape: &Shape{
			Axis:      axisLeftToRight,
			Padding:   PadAll(15),
			shapeType: shapeRectangle,
		},
	}
	layoutHeights(childless)
	if !f32AreClose(childless.Shape.Height, 0) {
		t.Errorf("childless container height: got %f, want 0 (must collapse, "+
			"not fall back to padding)", childless.Shape.Height)
	}
}

// TestFillDistributionIgnoresOutOfFlowFill guards the candidate set in
// collectDistributionCandidates. The remaining budget drops out-of-flow
// children (matching layout.spacing()), so dealing a Float Fill into the
// distribution would let it take budget no fence post accounts for and
// leave the in-flow Fill short: a 250px remainder split two ways instead
// of going whole to the one child that takes a slot in the row.
func TestFillDistributionIgnoresOutOfFlowFill(t *testing.T) {
	rowWith := func(extra ...*Shape) *Layout {
		l := &Layout{
			Shape: &Shape{
				Sizing:    FixedFixed,
				Width:     300,
				Height:    100,
				Axis:      axisLeftToRight,
				shapeType: shapeRectangle,
			},
			Children: []Layout{
				{Shape: &Shape{
					shapeType: shapeRectangle,
					Sizing:    FillFixed,
					Width:     0,
				}},
				{Shape: &Shape{
					shapeType: shapeRectangle,
					Sizing:    FixedFixed,
					Width:     50,
				}},
			},
		}
		for _, e := range extra {
			l.Children = append(l.Children, Layout{Shape: e})
		}
		layoutParents(l, nil)
		return l
	}

	fillWidths := func(l *Layout) (inFlow, floatFill float32) {
		var p scratchPools
		p.beginFillPass()
		layoutFillWidths(l, &p)
		return l.Children[0].Shape.Width, l.Children[2].Shape.Width
	}

	// Baseline without the Float: 300 parent less the 50px fixed sibling.
	base := rowWith()
	var p scratchPools
	p.beginFillPass()
	layoutFillWidths(base, &p)
	if !f32AreClose(base.Children[0].Shape.Width, 250) {
		t.Fatalf("baseline fill width: got %f, want 250",
			base.Children[0].Shape.Width)
	}

	withFloat := rowWith(&Shape{
		shapeType: shapeRectangle,
		Sizing:    FillFixed,
		Float:     true,
		Width:     0,
	})
	got, floatW := fillWidths(withFloat)
	if !f32AreClose(got, 250) {
		t.Errorf("in-flow fill with a Float Fill sibling: got %f, want 250",
			got)
	}
	if !f32AreClose(floatW, 0) {
		t.Errorf("Float Fill takes no share of the row budget: got %f, want 0",
			floatW)
	}
}
