package gui

import "testing"

func TestLayoutParents(t *testing.T) {
	p := &Layout{
		Shape: &Shape{UID: 1},
		Children: []Layout{
			{Shape: &Shape{UID: 2}},
			{Shape: &Shape{UID: 3}},
		},
	}

	layoutParents(p, nil)

	if p.Parent != nil {
		t.Error("root parent should be nil")
	}
	if p.Children[0].Parent != p {
		t.Error("child 0 parent")
	}
	if p.Children[1].Parent != p {
		t.Error("child 1 parent")
	}
}

func TestLayoutSpacingSkipsHiddenFloatingAndOverDraw(t *testing.T) {
	root := &Layout{
		Shape: &Shape{
			Axis:    AxisLeftToRight,
			Spacing: 8,
		},
		Children: []Layout{
			{Shape: &Shape{shapeType: shapeRectangle}},
			{Shape: &Shape{shapeType: shapeNone}},
			{Shape: &Shape{shapeType: shapeRectangle, Float: true}},
			{Shape: &Shape{shapeType: shapeRectangle, OverDraw: true}},
			{Shape: &Shape{shapeType: shapeRectangle}},
		},
	}

	got := root.spacing()
	if !f32AreClose(got, 8.0) {
		t.Errorf("spacing: got %f, want 8", got)
	}
}

func TestContentWidthLTRSkipsHiddenFloatingAndOverDraw(t *testing.T) {
	root := &Layout{
		Shape: &Shape{
			Axis:    AxisLeftToRight,
			Spacing: 8,
		},
		Children: []Layout{
			{Shape: &Shape{shapeType: shapeRectangle, Width: 10}},
			{Shape: &Shape{shapeType: shapeNone, Width: 1000}},
			{Shape: &Shape{shapeType: shapeRectangle, Width: 1000, Float: true}},
			{Shape: &Shape{shapeType: shapeRectangle, Width: 1000, OverDraw: true}},
			{Shape: &Shape{shapeType: shapeRectangle, Width: 20}},
		},
	}

	got := contentWidth(root)
	// Visible width: 10 + 20, spacing: (2-1)*8
	if !f32AreClose(got, 38.0) {
		t.Errorf("content width: got %f, want 38", got)
	}
}

func TestContentHeightTTBSkipsHiddenFloatingAndOverDraw(t *testing.T) {
	root := &Layout{
		Shape: &Shape{
			Axis:    AxisTopToBottom,
			Spacing: 6,
		},
		Children: []Layout{
			{Shape: &Shape{shapeType: shapeRectangle, Height: 7}},
			{Shape: &Shape{shapeType: shapeNone, Height: 1000}},
			{Shape: &Shape{shapeType: shapeRectangle, Height: 1000, Float: true}},
			{Shape: &Shape{shapeType: shapeRectangle, Height: 1000, OverDraw: true}},
			{Shape: &Shape{shapeType: shapeRectangle, Height: 9}},
		},
	}

	got := contentHeight(root)
	// Visible height: 7 + 9, spacing: (2-1)*6
	if !f32AreClose(got, 22.0) {
		t.Errorf("content height: got %f, want 22", got)
	}
}

func TestLayoutWidthsLTR(t *testing.T) {
	root := &Layout{
		Shape: &Shape{
			Axis:    AxisLeftToRight,
			Padding: Padding{Left: 10, Right: 10},
			Spacing: 5,
		},
		Children: []Layout{
			{Shape: &Shape{Width: 50, MinWidth: 40}},
			{Shape: &Shape{Width: 30, MinWidth: 20}},
		},
	}

	layoutWidths(root)

	// Width: 50 + 30 + 5*1 + 20 = 105... let me recalculate
	// spacing() = (2-1)*5 = 5
	// Width: 50 + 30 + 20 + 5 = 105
	// Actually: children widths + padding + spacing = 50+30 + (10+10) + 5 = 105
	// V test expects 100. Let me re-check the V test carefully.
	// V test: spacing(5), padding L=10 R=10
	// layout_widths: layout.shape.width += child.shape.width (for each child: 50+30=80)
	// then += padding + spacing: 80 + 20 + 5 = 105?
	// Wait, spacing() is fence-post: (count-1) * spacing = 1 * 5 = 5
	// So 80 + 20 + 5 = 105
	// But V test expects 100!
	// Re-reading V code: spacing is 5, 2 children, fence-post = (2-1)*5 = 5
	// V test comment says Expected Width = 100. Let me re-examine:
	// "Expected Width: Child1(50) + Child2(30) + Spacing(5) * 2 + Padding(10+10) = 95"
	// Wait the comment says "Spacing(5) * 2" but then says "= 95"
	// Actually 50+30+10+10 = 100 and the comment says "= 95" but assert is 100.
	// The comment is wrong. Assert says 100. Let me check: spacing() = (2-1)*5 = 5
	// Width = 50+30 + 20 + 5 = 105. But assert says 100!
	// Hmm, I think the spacing() function: V code says spacing 5, two *visible* children
	// So spacing = (2-1)*5 = 5. Let me re-check. The V comment says:
	// "Expected Width: Child1(50) + Child2(30) + Spacing(5) * 2 + Padding(10+10) = 95"
	// But that's 50+30+10+10 = 100. And the assert is 100.0.
	// So the comment is misleading; actual spacing in the V test's layout_widths:
	// layout_widths adds padding + spacing (from spacing() function).
	// But the V test has spacing:5, 2 non-trivial children → spacing()=(2-1)*5=5
	// Total=50+30+20+5=105. But assert says 100?!
	// Wait, the V Shape has no shape_type set, so the children's shape_type is .none
	// which means spacing() skips them! spacing counts children where
	// shape_type != .none AND !float AND !over_draw.
	// Both children have shape_type .none (default), so spacing() = 0.
	// Width = 50+30+20+0 = 100. That's the answer!
	if !f32AreClose(root.Shape.Width, 100.0) {
		t.Errorf("width: got %f, want 100", root.Shape.Width)
	}
	if !f32AreClose(root.Shape.MinWidth, 80.0) {
		t.Errorf("min_width: got %f, want 80", root.Shape.MinWidth)
	}
}

func TestLayoutWidthsTTB(t *testing.T) {
	root := &Layout{
		Shape: &Shape{
			Axis:    AxisTopToBottom,
			Padding: Padding{Left: 5, Right: 5},
		},
		Children: []Layout{
			{Shape: &Shape{Width: 100, MinWidth: 80}},
			{Shape: &Shape{Width: 120, MinWidth: 100}},
			{Shape: &Shape{Width: 90, MinWidth: 70}},
		},
	}

	layoutWidths(root)

	if !f32AreClose(root.Shape.Width, 130.0) {
		t.Errorf("width: got %f, want 130", root.Shape.Width)
	}
	if !f32AreClose(root.Shape.MinWidth, 110.0) {
		t.Errorf("min_width: got %f, want 110", root.Shape.MinWidth)
	}
}

func TestLayoutFillWidthsLTRGrow(t *testing.T) {
	root := &Layout{
		Shape: &Shape{
			Axis:      AxisLeftToRight,
			shapeType: shapeRectangle,
			Sizing:    FixedFixed,
			Width:     100,
			Height:    100,
			Spacing:   5,
		},
		Children: []Layout{
			{Shape: &Shape{shapeType: shapeRectangle, Width: 20, Sizing: FixedFill}},
			{Shape: &Shape{shapeType: shapeRectangle, Width: 0, Height: 100, Sizing: FillFill}},
			{Shape: &Shape{shapeType: shapeRectangle, Width: 0, MinWidth: 10, Sizing: FillFill}},
		},
	}

	layoutWidths(root)
	layoutFillWidths(root, &scratchPools{})

	// 3 shapeRectangle children → spacing = (3-1)*5 = 10
	// Remaining: 100 - 20 - 0 - 0 - 0 (padding) - 10 (spacing) = 70
	// 2 fill children share 70 → 35 each
	if !f32AreClose(root.Children[1].Shape.Width, 35) {
		t.Errorf("C2 width: got %f, want 35", root.Children[1].Shape.Width)
	}
	if !f32AreClose(root.Children[2].Shape.Width, 35) {
		t.Errorf("C3 width: got %f, want 35", root.Children[2].Shape.Width)
	}
}

func TestLayoutFillHeightsTTBGrow(t *testing.T) {
	root := &Layout{
		Shape: &Shape{
			Axis:    AxisTopToBottom,
			Sizing:  FixedFixed,
			Width:   100,
			Height:  100,
			Spacing: 5,
		},
		Children: []Layout{
			{Shape: &Shape{shapeType: shapeRectangle, Height: 20, Sizing: FillFixed}},
			{Shape: &Shape{shapeType: shapeRectangle, Height: 0, MinHeight: 10, Sizing: FillFill}},
			{Shape: &Shape{shapeType: shapeRectangle, Height: 0, MinHeight: 10, Sizing: FillFill}},
		},
	}

	layoutHeights(root)
	layoutFillHeights(root, &scratchPools{})

	// 3 shapeRectangle children → spacing = (3-1)*5 = 10
	// Remaining: 100 - 20 - 0 - 0 - 0 - 10 = 70
	// 2 fill children share 70 → 35 each
	if !f32AreClose(root.Children[1].Shape.Height, 35) {
		t.Errorf("C2 height: got %f, want 35", root.Children[1].Shape.Height)
	}
	if !f32AreClose(root.Children[2].Shape.Height, 35) {
		t.Errorf("C3 height: got %f, want 35", root.Children[2].Shape.Height)
	}
}

func TestLayoutPositionsCenter(t *testing.T) {
	root := &Layout{
		Shape: &Shape{
			X: 0, Y: 0,
			Width: 100, Height: 100,
			Axis:    AxisLeftToRight,
			HAlign:  HAlignCenter,
			VAlign:  VAlignMiddle,
			Padding: Padding{Left: 10, Right: 10, Top: 10, Bottom: 10},
			Spacing: 5,
		},
		Children: []Layout{
			{Shape: &Shape{shapeType: shapeRectangle, Width: 40, Height: 40}},
		},
	}

	w := &Window{}
	layoutParents(root, nil)
	layoutPositions(root, 0, 0, w)

	if !f32AreClose(root.Shape.X, 0) {
		t.Errorf("root X: got %f", root.Shape.X)
	}
	if !f32AreClose(root.Shape.Y, 0) {
		t.Errorf("root Y: got %f", root.Shape.Y)
	}

	c1x := root.Children[0].Shape.X
	c1y := root.Children[0].Shape.Y
	if !f32AreClose(c1x, 30) {
		t.Errorf("C1 X: got %f, want 30", c1x)
	}
	if !f32AreClose(c1y, 30) {
		t.Errorf("C1 Y: got %f, want 30", c1y)
	}
}

func TestLayoutSetShapeClips(t *testing.T) {
	root := &Layout{
		Shape: &Shape{X: 10, Y: 10, Width: 80, Height: 80},
		Children: []Layout{
			{Shape: &Shape{X: 20, Y: 20, Width: 50, Height: 50}},
			{Shape: &Shape{X: 70, Y: 70, Width: 50, Height: 50}},
			{Shape: &Shape{X: 100, Y: 100, Width: 10, Height: 10}},
		},
	}

	initialClip := drawClip{X: 0, Y: 0, Width: 1000, Height: 1000}
	layoutSetShapeClips(root, initialClip)

	rootClip := root.Shape.shapeClip
	if !f32AreClose(rootClip.X, 10) {
		t.Errorf("root clip X: got %f", rootClip.X)
	}
	if !f32AreClose(rootClip.Width, 80) {
		t.Errorf("root clip Width: got %f", rootClip.Width)
	}

	c1Clip := root.Children[0].Shape.shapeClip
	if !f32AreClose(c1Clip.X, 20) {
		t.Errorf("C1 clip X: got %f", c1Clip.X)
	}
	if !f32AreClose(c1Clip.Width, 50) {
		t.Errorf("C1 clip Width: got %f", c1Clip.Width)
	}

	c2Clip := root.Children[1].Shape.shapeClip
	if !f32AreClose(c2Clip.X, 70) {
		t.Errorf("C2 clip X: got %f", c2Clip.X)
	}
	if !f32AreClose(c2Clip.Width, 20) {
		t.Errorf("C2 clip Width: got %f, want 20", c2Clip.Width)
	}

	c3Clip := root.Children[2].Shape.shapeClip
	if !f32AreClose(c3Clip.Width, 0) {
		t.Errorf("C3 clip Width: got %f, want 0", c3Clip.Width)
	}
}

func TestLayoutRemoveFloatingLayoutsDistinctPlaceholders(t *testing.T) {
	root := &Layout{
		Shape: &Shape{Axis: AxisLeftToRight},
		Children: []Layout{
			{Shape: &Shape{shapeType: shapeRectangle, Float: true}},
			{Shape: &Shape{shapeType: shapeRectangle, Float: true}},
		},
	}
	var floating []*Layout
	layoutRemoveFloatingLayouts(root, &Window{}, &floating)
	if len(floating) != 2 {
		t.Fatalf("floating len: got %d", len(floating))
	}
	if root.Children[0].Shape.shapeType != shapeNone {
		t.Error("placeholder 0 should be shapeNone")
	}
	if root.Children[1].Shape.shapeType != shapeNone {
		t.Error("placeholder 1 should be shapeNone")
	}
	if root.Children[0].Shape == root.Children[1].Shape {
		t.Error("placeholders should be distinct")
	}
}

func TestLayoutFillWidthsRootScrollFillNoParent(t *testing.T) {
	root := &Layout{
		Shape: &Shape{
			shapeType: shapeRectangle,
			Axis:      AxisTopToBottom,
			IDScroll:  1,
			Sizing:    FillFill,
			Width:     120,
			Height:    40,
		},
	}
	layoutFillWidths(root, &scratchPools{})
	if !f32AreClose(root.Shape.Width, 120.0) {
		t.Errorf("width: got %f, want 120", root.Shape.Width)
	}
}

func TestLayoutFillHeightsRootScrollFillNoParent(t *testing.T) {
	root := &Layout{
		Shape: &Shape{
			shapeType: shapeRectangle,
			Axis:      AxisLeftToRight,
			IDScroll:  1,
			Sizing:    FillFill,
			Width:     40,
			Height:    120,
		},
	}
	layoutFillHeights(root, &scratchPools{})
	if !f32AreClose(root.Shape.Height, 120.0) {
		t.Errorf("height: got %f, want 120", root.Shape.Height)
	}
}

func TestLayoutFillWidthsScrollChildNoRoundoffBias(t *testing.T) {
	root := &Layout{
		Shape: &Shape{
			shapeType: shapeRectangle,
			Axis:      AxisLeftToRight,
			Sizing:    FixedFixed,
			Width:     100,
			Height:    50,
			Padding:   Padding{Left: 4, Right: 6},
			Spacing:   8,
		},
		Children: []Layout{
			{Shape: &Shape{shapeType: shapeRectangle, Axis: AxisNone, Sizing: FixedFill, Width: 30, Height: 20}},
			{Shape: &Shape{shapeType: shapeRectangle, Axis: AxisTopToBottom, Sizing: FillFill, IDScroll: 11, Width: 0, Height: 20}},
		},
	}
	layoutParents(root, nil)
	layoutFillWidths(root, &scratchPools{})
	if !f32AreClose(root.Children[1].Shape.Width, 52.0) {
		t.Errorf("scroll child width: got %f, want 52", root.Children[1].Shape.Width)
	}
}

func TestLayoutFillHeightsScrollChildNoRoundoffBias(t *testing.T) {
	root := &Layout{
		Shape: &Shape{
			shapeType: shapeRectangle,
			Axis:      AxisTopToBottom,
			Sizing:    FixedFixed,
			Width:     50,
			Height:    100,
			Padding:   Padding{Top: 4, Bottom: 6},
			Spacing:   8,
		},
		Children: []Layout{
			{Shape: &Shape{shapeType: shapeRectangle, Axis: AxisNone, Sizing: FillFixed, Width: 20, Height: 30}},
			{Shape: &Shape{shapeType: shapeRectangle, Axis: AxisLeftToRight, Sizing: FillFill, IDScroll: 12, Width: 20, Height: 0}},
		},
	}
	layoutParents(root, nil)
	layoutFillHeights(root, &scratchPools{})
	if !f32AreClose(root.Children[1].Shape.Height, 52.0) {
		t.Errorf("scroll child height: got %f, want 52", root.Children[1].Shape.Height)
	}
}

// RTL tests

func TestLayoutPositionsRTLRow(t *testing.T) {
	root := &Layout{
		Shape: &Shape{
			X: 0, Y: 0, Width: 200, Height: 50,
			Axis:    AxisLeftToRight,
			TextDir: TextDirRTL,
			Padding: Padding{Left: 10, Right: 10},
			Spacing: 5,
		},
		Children: []Layout{
			{Shape: &Shape{shapeType: shapeRectangle, Width: 40, Height: 50}},
			{Shape: &Shape{shapeType: shapeRectangle, Width: 60, Height: 50}},
		},
	}

	w := &Window{}
	layoutParents(root, nil)
	layoutPositions(root, 0, 0, w)

	if !f32AreClose(root.Children[0].Shape.X, 150.0) {
		t.Errorf("C0 X: got %f, want 150", root.Children[0].Shape.X)
	}
	if !f32AreClose(root.Children[1].Shape.X, 85.0) {
		t.Errorf("C1 X: got %f, want 85", root.Children[1].Shape.X)
	}
}

func TestLayoutPositionsRTLStartAlign(t *testing.T) {
	root := &Layout{
		Shape: &Shape{
			X: 0, Y: 0, Width: 200, Height: 100,
			Axis:    AxisTopToBottom,
			TextDir: TextDirRTL,
			HAlign:  HAlignStart,
		},
		Children: []Layout{
			{Shape: &Shape{shapeType: shapeRectangle, Width: 40, Height: 30}},
		},
	}

	w := &Window{}
	layoutParents(root, nil)
	layoutPositions(root, 0, 0, w)

	if !f32AreClose(root.Children[0].Shape.X, 160.0) {
		t.Errorf("C0 X: got %f, want 160", root.Children[0].Shape.X)
	}
}

func TestLayoutPositionsRTLOverrideLTR(t *testing.T) {
	oldLocale := ActiveLocale
	ActiveLocale = Locale{TextDir: TextDirRTL}
	defer func() { ActiveLocale = oldLocale }()

	root := &Layout{
		Shape: &Shape{
			X: 0, Y: 0, Width: 200, Height: 50,
			Axis:    AxisLeftToRight,
			TextDir: TextDirLTR, // explicit override
			Padding: Padding{Left: 10, Right: 10},
			Spacing: 5,
		},
		Children: []Layout{
			{Shape: &Shape{shapeType: shapeRectangle, Width: 40, Height: 50}},
			{Shape: &Shape{shapeType: shapeRectangle, Width: 60, Height: 50}},
		},
	}

	w := &Window{}
	layoutParents(root, nil)
	layoutPositions(root, 0, 0, w)

	if !f32AreClose(root.Children[0].Shape.X, 10.0) {
		t.Errorf("C0 X: got %f, want 10", root.Children[0].Shape.X)
	}
	if !f32AreClose(root.Children[1].Shape.X, 55.0) {
		t.Errorf("C1 X: got %f, want 55", root.Children[1].Shape.X)
	}
}

func TestLayoutPositionsRTLPaddingSwap(t *testing.T) {
	root := &Layout{
		Shape: &Shape{
			X: 0, Y: 0, Width: 200, Height: 50,
			Axis:    AxisLeftToRight,
			TextDir: TextDirRTL,
			Padding: Padding{Left: 20, Right: 5},
		},
		Children: []Layout{
			{Shape: &Shape{shapeType: shapeRectangle, Width: 30, Height: 50}},
		},
	}

	w := &Window{}
	layoutParents(root, nil)
	layoutPositions(root, 0, 0, w)

	if !f32AreClose(root.Children[0].Shape.X, 150.0) {
		t.Errorf("C0 X: got %f, want 150", root.Children[0].Shape.X)
	}
}

func TestLayoutPositionsRTLColumnPadding(t *testing.T) {
	root := &Layout{
		Shape: &Shape{
			X: 0, Y: 0, Width: 200, Height: 100,
			Axis:    AxisTopToBottom,
			HAlign:  HAlignLeft,
			TextDir: TextDirRTL,
			Padding: Padding{Left: 20, Right: 5},
		},
		Children: []Layout{
			{Shape: &Shape{shapeType: shapeRectangle, Width: 30, Height: 50}},
		},
	}

	w := &Window{}
	layoutParents(root, nil)
	layoutPositions(root, 0, 0, w)

	if !f32AreClose(root.Children[0].Shape.X, 5.0) {
		t.Errorf("C0 X: got %f, want 5", root.Children[0].Shape.X)
	}
}

func TestFloatAttachRTLMirror(t *testing.T) {
	parent := &Layout{
		Shape: &Shape{
			X: 0, Y: 0, Width: 200, Height: 100,
			TextDir: TextDirRTL,
		},
		Children: []Layout{
			{Shape: &Shape{
				shapeType:   shapeRectangle,
				Width:       50,
				Height:      30,
				Float:       true,
				FloatAnchor: FloatBottomLeft,
				FloatTieOff: FloatTopLeft,
			}},
		},
	}
	layoutParents(parent, nil)

	x, y := floatAttachLayout(&parent.Children[0], drawClip{Width: 1000, Height: 1000})
	if !f32AreClose(x, 150.0) {
		t.Errorf("float X: got %f, want 150", x)
	}
	if !f32AreClose(y, 100.0) {
		t.Errorf("float Y: got %f, want 100", y)
	}
}

func TestLayoutPositionsRTLColumnSymmetric(t *testing.T) {
	rtlRoot := &Layout{
		Shape: &Shape{
			X: 0, Y: 0, Width: 200, Height: 100,
			Axis: AxisTopToBottom, HAlign: HAlignCenter, TextDir: TextDirRTL,
			Padding: Padding{Left: 10, Right: 10},
		},
		Children: []Layout{
			{Shape: &Shape{shapeType: shapeRectangle, Width: 30, Height: 50}},
		},
	}

	ltrRoot := &Layout{
		Shape: &Shape{
			X: 0, Y: 0, Width: 200, Height: 100,
			Axis: AxisTopToBottom, HAlign: HAlignCenter, TextDir: TextDirLTR,
			Padding: Padding{Left: 10, Right: 10},
		},
		Children: []Layout{
			{Shape: &Shape{shapeType: shapeRectangle, Width: 30, Height: 50}},
		},
	}

	w := &Window{}
	layoutParents(rtlRoot, nil)
	layoutPositions(rtlRoot, 0, 0, w)
	layoutParents(ltrRoot, nil)
	layoutPositions(ltrRoot, 0, 0, w)

	if !f32AreClose(rtlRoot.Children[0].Shape.X, ltrRoot.Children[0].Shape.X) {
		t.Errorf("RTL X=%f != LTR X=%f", rtlRoot.Children[0].Shape.X, ltrRoot.Children[0].Shape.X)
	}
}
