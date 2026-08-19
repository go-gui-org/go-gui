package gui

import "testing"

func TestWrapBasic(t *testing.T) {
	root := &Layout{
		Shape: &Shape{
			Axis: axisLeftToRight, Wrap: true, Width: 80, Spacing: 5,
		},
		Children: []Layout{
			{Shape: &Shape{Width: 30, shapeType: shapeRectangle}},
			{Shape: &Shape{Width: 30, shapeType: shapeRectangle}},
			{Shape: &Shape{Width: 30, shapeType: shapeRectangle}},
		},
	}

	layoutWrapContainers(root, &Window{})

	if root.Shape.Axis != axisTopToBottom {
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
	if root.Children[0].Shape.Axis != axisLeftToRight {
		t.Error("row 0 axis should be LTR")
	}
	if root.Children[0].Shape.Spacing != 5 {
		t.Errorf("row 0 spacing: got %f", root.Children[0].Shape.Spacing)
	}
}

func TestWrapSingleRow(t *testing.T) {
	root := &Layout{
		Shape: &Shape{
			Axis: axisLeftToRight, Wrap: true, Width: 200, Spacing: 5,
		},
		Children: []Layout{
			{Shape: &Shape{Width: 30, shapeType: shapeRectangle}},
			{Shape: &Shape{Width: 30, shapeType: shapeRectangle}},
			{Shape: &Shape{Width: 30, shapeType: shapeRectangle}},
		},
	}

	layoutWrapContainers(root, &Window{})

	if root.Shape.Axis != axisLeftToRight {
		t.Error("axis should stay LTR")
	}
	if len(root.Children) != 3 {
		t.Errorf("children: got %d, want 3", len(root.Children))
	}
}

func TestWrapHeights(t *testing.T) {
	root := &Layout{
		Shape: &Shape{
			Axis: axisLeftToRight, Wrap: true, Width: 80, Spacing: 10,
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
			Axis: axisLeftToRight, Wrap: true,
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
	layoutFillHeights(root, &scratchPools{})
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
			Axis: axisLeftToRight, Wrap: true, Width: 80, Spacing: 5,
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
			Axis: axisTopToBottom, Width: 400, Height: 300,
			Sizing: FixedFixed, shapeType: shapeRectangle,
		},
		Children: []Layout{
			{
				Shape: &Shape{
					Axis: axisLeftToRight, Wrap: true,
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
	layoutFillWidths(root, &scratchPools{})

	wrapLayout := &root.Children[0]
	if !f32AreClose(wrapLayout.Shape.Width, 400) {
		t.Errorf("wrap width: got %f, want 400", wrapLayout.Shape.Width)
	}

	layoutWrapContainers(root, &Window{})

	if root.Children[0].Shape.Axis != axisTopToBottom {
		t.Error("wrap axis should flip to TTB")
	}
	if len(root.Children[0].Children) < 2 {
		t.Errorf("expected 2+ rows, got %d", len(root.Children[0].Children))
	}
}

func TestWrapLeadingSkippedChildren(t *testing.T) {
	root := &Layout{
		Shape: &Shape{
			Axis: axisLeftToRight, Wrap: true, Width: 80, Spacing: 5,
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
			Axis: axisLeftToRight, Wrap: true, Width: 65, Spacing: 5,
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
			Axis: axisLeftToRight, Wrap: true, Width: 100, Spacing: 5,
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

// TestWrapMinWidthIgnoresSpacingSum guards issue #378: layoutWidths used to
// seed a Wrap container's MinWidth floor with the full single-row inter-child
// gap sum, so the floor grew linearly with the child count. Past ~50 cards the
// floor exceeded the window width, the fill pass clamped the container back up
// to it, and layoutWrapContainers then wrapped against a width wider than the
// window — rows spilled past the right edge. Drives the real passes; the
// direct-call tests above hand-set Shape.Width and so cannot see this.
func TestWrapMinWidthIgnoresSpacingSum(t *testing.T) {
	const (
		parentWidth = 700
		childWidth  = 160
		spacing     = 16
		childCount  = 50
	)

	children := make([]Layout, childCount)
	for i := range children {
		children[i] = Layout{Shape: &Shape{
			Width: childWidth, Height: 170, shapeType: shapeRectangle,
		}}
	}

	root := &Layout{
		Shape: &Shape{
			Axis: axisTopToBottom, Width: parentWidth, Height: 600,
			Sizing: FixedFixed, shapeType: shapeRectangle,
		},
		Children: []Layout{{
			Shape: &Shape{
				Axis: axisLeftToRight, Wrap: true, Sizing: FillFill,
				Spacing: spacing, shapeType: shapeRectangle,
			},
			Children: children,
		}},
	}

	layoutWidths(root)
	layoutFillWidths(root, &scratchPools{})

	wrap := &root.Children[0]
	if wrap.Shape.MinWidth > childWidth {
		t.Errorf("wrap MinWidth: got %f, want <= %d (widest child); the "+
			"inter-child gap sum must not floor a wrapping container",
			wrap.Shape.MinWidth, childWidth)
	}
	if !f32AreClose(wrap.Shape.Width, parentWidth) {
		t.Fatalf("wrap width: got %f, want %d", wrap.Shape.Width, parentWidth)
	}

	layoutWrapContainers(root, &Window{})

	if wrap.Shape.Axis != axisTopToBottom {
		t.Fatal("wrap axis should flip to TTB")
	}
	rows := wrap.Children
	if len(rows) < 2 {
		t.Fatalf("expected 2+ rows, got %d", len(rows))
	}
	// Every row must fit inside the parent's content box — the assertion
	// that fails end to end when the floor is wrong.
	for i := range rows {
		if rows[i].Shape.Width > parentWidth {
			t.Errorf("row %d width: got %f, want <= %d",
				i, rows[i].Shape.Width, parentWidth)
		}
		var used float32
		for j := range rows[i].Children {
			if j > 0 {
				used += spacing
			}
			used += rows[i].Children[j].Shape.Width
		}
		if used > parentWidth {
			t.Errorf("row %d content: got %f, want <= %d", i, used, parentWidth)
		}
	}
}

// TestOverflowMinWidthIgnoresSpacingSum is the Overflow half of issue #378:
// an overflow container hides trailing children, so it too must be allowed to
// shrink below the sum of its content. view_overflow_panel relies on this.
func TestOverflowMinWidthIgnoresSpacingSum(t *testing.T) {
	const (
		childWidth = 60
		spacing    = 12
		childCount = 30
	)

	children := make([]Layout, childCount)
	for i := range children {
		children[i] = Layout{Shape: &Shape{
			Width: childWidth, Height: 20, shapeType: shapeRectangle,
		}}
	}

	root := &Layout{
		Shape: &Shape{
			Axis: axisLeftToRight, Overflow: true, Sizing: FitFit,
			Spacing: spacing, shapeType: shapeRectangle,
		},
		Children: children,
	}

	layoutWidths(root)

	if root.Shape.MinWidth > childWidth {
		t.Errorf("overflow MinWidth: got %f, want <= %d (widest child)",
			root.Shape.MinWidth, childWidth)
	}
}

// TestWrapMinWidthIncludesPadding pins the padding term of the issue #378
// floor: a wrapping container must floor at widest child + padding, not at
// widest child alone — the padding is needed for the child to fit the
// container's content box. Drives the same passes as
// TestWrapMinWidthIgnoresSpacingSum.
func TestWrapMinWidthIncludesPadding(t *testing.T) {
	const (
		parentWidth = 700
		childWidth  = 160
		spacing     = 16
		padLeft     = 10
		padRight    = 10
		childCount  = 50
	)

	children := make([]Layout, childCount)
	for i := range children {
		children[i] = Layout{Shape: &Shape{
			Width: childWidth, Height: 170, shapeType: shapeRectangle,
		}}
	}

	root := &Layout{
		Shape: &Shape{
			Axis: axisTopToBottom, Width: parentWidth, Height: 600,
			Sizing: FixedFixed, shapeType: shapeRectangle,
		},
		Children: []Layout{{
			Shape: &Shape{
				Axis: axisLeftToRight, Wrap: true, Sizing: FillFill,
				Spacing: spacing, Padding: NewPadding(0, padLeft, 0, padRight),
				shapeType: shapeRectangle,
			},
			Children: children,
		}},
	}

	layoutWidths(root)
	layoutFillWidths(root, &scratchPools{})
	layoutWrapContainers(root, &Window{})

	wrap := &root.Children[0]
	wantMin := float32(childWidth + padLeft + padRight)
	if !f32AreClose(wrap.Shape.MinWidth, wantMin) {
		t.Errorf("wrap MinWidth: got %f, want %f (widest child + padding)",
			wrap.Shape.MinWidth, wantMin)
	}

	// Rows wrap against the content box (Width - padding), so their
	// content must fit inside it — not the full parent width.
	contentBox := float32(parentWidth - padLeft - padRight)
	for i := range wrap.Children {
		var used float32
		for j := range wrap.Children[i].Children {
			if j > 0 {
				used += spacing
			}
			used += wrap.Children[i].Children[j].Shape.Width
		}
		if used > contentBox {
			t.Errorf("row %d content: got %f, want <= %f (content box)",
				i, used, contentBox)
		}
	}
}

// TestRowMinWidthKeepsSpacingSum guards the condition split added by the
// issue #378 fix: only Wrap/Overflow containers drop the inter-child gap sum
// from their MinWidth floor. A plain Row must keep it — dropping it would
// silently allow rows to shrink below their natural content width.
func TestRowMinWidthKeepsSpacingSum(t *testing.T) {
	root := &Layout{
		Shape: &Shape{
			Axis: axisLeftToRight, Spacing: 5,
		},
		Children: []Layout{
			{Shape: &Shape{Width: 40, shapeType: shapeRectangle}},
			{Shape: &Shape{Width: 40, shapeType: shapeRectangle}},
		},
	}

	layoutWidths(root)

	if !f32AreClose(root.Shape.MinWidth, 5) {
		t.Errorf("row MinWidth: got %f, want 5 (single inter-child gap)",
			root.Shape.MinWidth)
	}
	if !f32AreClose(root.Shape.Width, 85) {
		t.Errorf("row width: got %f, want 85 (40 + 5 + 40)",
			root.Shape.Width)
	}
}

// TestWrapCardsStayInsideWindow is issue #378 end to end: the reporter's tree
// (a FillFill Wrap of fixed-width cards inside a FillFill Column) driven
// through a real window frame. Asserts no card's right edge crosses the
// window's, which is the symptom that was reported.
func TestWrapCardsStayInsideWindow(t *testing.T) {
	const (
		winWidth   = 700
		cardWidth  = 160
		spacing    = 16
		cardCount  = 60
		cardHeight = 170
	)

	w := NewWindow(WindowCfg{
		State: new(int), Width: winWidth, Height: 600,
	})

	root := w.TestRender(func(*Window) View {
		cards := make([]View, cardCount)
		for i := range cards {
			cards[i] = Column(ContainerCfg{
				ID:         "card-" + itoa(i),
				Sizing:     FixedFixed,
				Width:      cardWidth,
				Height:     cardHeight,
				SizeBorder: NoBorder,
			})
		}
		return Column(ContainerCfg{
			Sizing: FillFill,
			Content: []View{Wrap(ContainerCfg{
				ID:         "cards-wrap",
				Sizing:     FillFill,
				Spacing:    SomeF(spacing),
				Scrollable: true,
				ScrollMode: ScrollVerticalOnly,
				Content:    cards,
			})},
		})
	})

	wrap, ok := root.FindByID("cards-wrap")
	if !ok {
		t.Fatal("wrap container not found")
	}
	if wrap.Shape.Width > winWidth {
		t.Errorf("wrap width: got %f, want <= %d", wrap.Shape.Width, winWidth)
	}

	// Compare against the window edge, not the container's own — a
	// container inflated past the window is exactly the defect, so its
	// own right edge is not a trustworthy bound.
	right := float32(winWidth)
	var checked int
	for i := range cardCount {
		card, found := root.FindByID("cards-wrap:card-" + itoa(i))
		if !found {
			continue // wrap may drop cards below the fold; check what exists
		}
		checked++
		if edge := card.Shape.X + card.Shape.Width; edge > right+f32Tolerance {
			t.Fatalf("card %d right edge %f exceeds window edge %f",
				i, edge, right)
		}
	}
	if checked == 0 {
		t.Fatal("no cards resolved; test is not exercising the layout")
	}
}

// TestWrapFitWidthConstrainedByParent is issue #379's repro: a FitFit wrap
// used to resolve its width to the full single-row sum of its children, so
// the row-break test measured against a width that was always wide enough
// and the container rendered one unwrapped row wider than its parent.
func TestWrapFitWidthConstrainedByParent(t *testing.T) {
	const (
		parentWidth = 400
		childWidth  = 100
		spacing     = 10
		childCount  = 20
	)

	children := make([]Layout, childCount)
	for i := range children {
		children[i] = Layout{Shape: &Shape{
			Width: childWidth, Height: 30, shapeType: shapeRectangle,
		}}
	}

	root := &Layout{
		Shape: &Shape{
			Axis: axisTopToBottom, Width: parentWidth, Height: 600,
			Sizing: FixedFixed, shapeType: shapeRectangle,
		},
		Children: []Layout{{
			Shape: &Shape{
				Axis: axisLeftToRight, Wrap: true, Sizing: FitFit,
				Spacing: spacing, shapeType: shapeRectangle,
			},
			Children: children,
		}},
	}

	layoutParents(root, nil)
	layoutWidths(root)
	layoutFillWidths(root, &scratchPools{})
	layoutWrapContainers(root, &Window{})

	wrap := &root.Children[0]
	if wrap.Shape.Width > parentWidth+f32Tolerance {
		t.Errorf("wrap width: got %f, want <= %d", wrap.Shape.Width, parentWidth)
	}
	if wrap.Shape.Axis != axisTopToBottom {
		t.Error("wrap axis should flip to TTB")
	}
	if len(wrap.Children) < 2 {
		t.Fatalf("rows: got %d, want 2+", len(wrap.Children))
	}
	// 3 per row: 3*100 + 2*10 = 320 fits, 4*100 + 3*10 = 430 does not.
	for i := range wrap.Children {
		var used float32
		for j := range wrap.Children[i].Children {
			if j > 0 {
				used += spacing
			}
			used += wrap.Children[i].Children[j].Shape.Width
		}
		if used > parentWidth+f32Tolerance {
			t.Errorf("row %d content: got %f, want <= %d", i, used, parentWidth)
		}
	}
	if !f32AreClose(root.Shape.Width, parentWidth) {
		t.Errorf("parent width: got %f, want %d", root.Shape.Width, parentWidth)
	}
}

// TestWrapFitWidthUnconstrainedStaysSingleRow pins the unconstrained side of
// the issue #379 rule: a Fit chain with no definite-width ancestor keeps the
// single-row sum — there is no width to wrap within, so the container
// behaves as a Row.
func TestWrapFitWidthUnconstrainedStaysSingleRow(t *testing.T) {
	root := &Layout{
		Shape: &Shape{
			Axis: axisLeftToRight, Wrap: true, Sizing: FitFit,
			Spacing: 5, shapeType: shapeRectangle,
		},
		Children: []Layout{
			{Shape: &Shape{Width: 30, shapeType: shapeRectangle}},
			{Shape: &Shape{Width: 30, shapeType: shapeRectangle}},
			{Shape: &Shape{Width: 30, shapeType: shapeRectangle}},
		},
	}

	layoutWidths(root)
	layoutFillWidths(root, &scratchPools{})
	layoutWrapContainers(root, &Window{})

	if !f32AreClose(root.Shape.Width, 100) {
		t.Errorf("wrap width: got %f, want 100 (30 + 5 + 30 + 5 + 30)",
			root.Shape.Width)
	}
	if root.Shape.Axis != axisLeftToRight {
		t.Error("axis should stay LTR with no width to wrap within")
	}
	if len(root.Children) != 3 {
		t.Errorf("children: got %d, want 3", len(root.Children))
	}
}

// TestWrapFitChainRefitsColumn: a FitFit wrap inside a FitFit column inside a
// fixed root wraps against the root's width, and the column re-fits to the
// wrap's constrained width instead of the unconstrained sum.
func TestWrapFitChainRefitsColumn(t *testing.T) {
	const (
		rootWidth  = 700
		childWidth = 160
		spacing    = 16
		childCount = 50
	)

	children := make([]Layout, childCount)
	for i := range children {
		children[i] = Layout{Shape: &Shape{
			Width: childWidth, Height: 20, shapeType: shapeRectangle,
		}}
	}

	root := &Layout{
		Shape: &Shape{
			Axis: axisTopToBottom, Width: rootWidth, Height: 600,
			Sizing: FixedFixed, shapeType: shapeRectangle,
		},
		Children: []Layout{{
			Shape: &Shape{
				Axis: axisTopToBottom, Sizing: FitFit, shapeType: shapeRectangle,
			},
			Children: []Layout{{
				Shape: &Shape{
					Axis: axisLeftToRight, Wrap: true, Sizing: FitFit,
					Spacing: spacing, shapeType: shapeRectangle,
				},
				Children: children,
			}},
		}},
	}

	layoutParents(root, nil)
	layoutWidths(root)
	layoutFillWidths(root, &scratchPools{})
	layoutWrapContainers(root, &Window{})

	col := &root.Children[0]
	wrap := &col.Children[0]
	if !f32AreClose(wrap.Shape.Width, rootWidth) {
		t.Errorf("wrap width: got %f, want %d", wrap.Shape.Width, rootWidth)
	}
	if wrap.Shape.Axis != axisTopToBottom {
		t.Error("wrap axis should flip to TTB")
	}
	// 4 per row: 4*160 + 3*16 = 688 fits, 5*160 + 4*16 = 864 does not.
	if len(wrap.Children) != 13 {
		t.Errorf("rows: got %d, want 13", len(wrap.Children))
	}
	// The Fit column must re-fit from the constrained wrap width rather
	// than keep the unconstrained sum.
	if !f32AreClose(col.Shape.Width, rootWidth) {
		t.Errorf("column width: got %f, want %d (re-fitted)",
			col.Shape.Width, rootWidth)
	}
}

// TestWrapFitWidthSiblingConsumption: a Fit wrap sharing a definite row with
// a fixed sibling wraps within the row minus the sibling and the spacing.
func TestWrapFitWidthSiblingConsumption(t *testing.T) {
	const (
		rowWidth   = 700
		childWidth = 100
		wrapSp     = 5
		rowSp      = 10
		siblingW   = 100
	)

	children := make([]Layout, 20)
	for i := range children {
		children[i] = Layout{Shape: &Shape{
			Width: childWidth, Height: 30, shapeType: shapeRectangle,
		}}
	}

	root := &Layout{
		Shape: &Shape{
			Axis: axisLeftToRight, Width: rowWidth, Height: 100,
			Sizing: FixedFixed, Spacing: rowSp, shapeType: shapeRectangle,
		},
		Children: []Layout{
			{
				Shape: &Shape{
					Axis: axisLeftToRight, Wrap: true, Sizing: FitFit,
					Spacing: wrapSp, shapeType: shapeRectangle,
				},
				Children: children,
			},
			{Shape: &Shape{Width: siblingW, shapeType: shapeRectangle}},
		},
	}

	layoutParents(root, nil)
	layoutWidths(root)
	layoutFillWidths(root, &scratchPools{})
	layoutWrapContainers(root, &Window{})

	wrap := &root.Children[0]
	want := float32(rowWidth - rowSp - siblingW)
	if !f32AreClose(wrap.Shape.Width, want) {
		t.Errorf("wrap width: got %f, want %f (%d - %d - %d)",
			wrap.Shape.Width, want, rowWidth, rowSp, siblingW)
	}
	if wrap.Shape.Axis != axisTopToBottom {
		t.Error("wrap axis should flip to TTB")
	}
	// 5 per row: 5*100 + 4*5 = 520 fits, 6*100 + 5*5 = 625 does not.
	if len(wrap.Children) != 4 {
		t.Errorf("rows: got %d, want 4", len(wrap.Children))
	}
}

// TestWrapFitWidthMinWidthFloor: when the definite ancestor is narrower than
// the widest child, the wrap floors at widest child + padding (the issue
// #378 floor) instead of collapsing below its content, and still breaks rows
// one child per row.
func TestWrapFitWidthMinWidthFloor(t *testing.T) {
	const (
		parentWidth = 80
		childWidth  = 100
		spacing     = 5
	)

	children := make([]Layout, 3)
	for i := range children {
		children[i] = Layout{Shape: &Shape{
			Width: childWidth, Height: 20, shapeType: shapeRectangle,
		}}
	}

	root := &Layout{
		Shape: &Shape{
			Axis: axisTopToBottom, Width: parentWidth, Height: 300,
			Sizing: FixedFixed, shapeType: shapeRectangle,
		},
		Children: []Layout{{
			Shape: &Shape{
				Axis: axisLeftToRight, Wrap: true, Sizing: FitFit,
				Spacing: spacing, shapeType: shapeRectangle,
			},
			Children: children,
		}},
	}

	layoutParents(root, nil)
	layoutWidths(root)
	layoutFillWidths(root, &scratchPools{})
	layoutWrapContainers(root, &Window{})

	wrap := &root.Children[0]
	if !f32AreClose(wrap.Shape.Width, childWidth) {
		t.Errorf("wrap width: got %f, want %d (widest child floor)",
			wrap.Shape.Width, childWidth)
	}
	if wrap.Shape.Axis != axisTopToBottom {
		t.Error("wrap axis should flip to TTB")
	}
	if len(wrap.Children) != 3 {
		t.Errorf("rows: got %d, want 3 (one child per row)", len(wrap.Children))
	}
}

// TestWrapFitCardsStayInsideWindow is issue #379 end to end: a FitFit wrap
// of fixed-width cards inside a FillFill column, driven through a real
// window frame. Before the fix the wrap resolved to its single-row sum and
// every card spilled past the window edge.
func TestWrapFitCardsStayInsideWindow(t *testing.T) {
	const (
		winWidth   = 700
		cardWidth  = 160
		spacing    = 16
		cardCount  = 60
		cardHeight = 170
	)

	w := NewWindow(WindowCfg{
		State: new(int), Width: winWidth, Height: 600,
	})

	root := w.TestRender(func(*Window) View {
		cards := make([]View, cardCount)
		for i := range cards {
			cards[i] = Column(ContainerCfg{
				ID:         "card-" + itoa(i),
				Sizing:     FixedFixed,
				Width:      cardWidth,
				Height:     cardHeight,
				SizeBorder: NoBorder,
			})
		}
		return Column(ContainerCfg{
			Sizing: FillFill,
			Content: []View{Wrap(ContainerCfg{
				ID:         "cards-wrap",
				Sizing:     FitFit,
				Spacing:    SomeF(spacing),
				Scrollable: true,
				ScrollMode: ScrollVerticalOnly,
				Content:    cards,
			})},
		})
	})

	wrap, ok := root.FindByID("cards-wrap")
	if !ok {
		t.Fatal("wrap container not found")
	}
	if wrap.Shape.Width > winWidth {
		t.Errorf("wrap width: got %f, want <= %d", wrap.Shape.Width, winWidth)
	}

	right := float32(winWidth)
	var checked int
	for i := range cardCount {
		card, found := root.FindByID("cards-wrap:card-" + itoa(i))
		if !found {
			continue // wrap may drop cards below the fold; check what exists
		}
		checked++
		if edge := card.Shape.X + card.Shape.Width; edge > right+f32Tolerance {
			t.Fatalf("card %d right edge %f exceeds window edge %f",
				i, edge, right)
		}
	}
	if checked == 0 {
		t.Fatal("no cards resolved; test is not exercising the layout")
	}
}

// TestWrapFitWidthRotatedAncestorUnconstrained pins the rotation stop in
// constrainFitContainerWidth: a rotated ancestor's dimensions are swapped by
// a later pass, so the walk must not constrain against them — the wrap keeps
// its single-row sum.
func TestWrapFitWidthRotatedAncestorUnconstrained(t *testing.T) {
	const (
		rootWidth = 200
		childW    = 30
		spacing   = 5
		childN    = 20 // sum 20*30 + 19*5 = 695 > rootWidth
	)

	children := make([]Layout, childN)
	for i := range children {
		children[i] = Layout{Shape: &Shape{
			Width: childW, Height: 10, shapeType: shapeRectangle,
		}}
	}

	root := &Layout{
		Shape: &Shape{
			Axis: axisLeftToRight, Width: rootWidth, Height: 100,
			Sizing: FixedFixed, QuarterTurns: 1, shapeType: shapeRectangle,
		},
		Children: []Layout{{
			Shape: &Shape{
				Axis: axisLeftToRight, Wrap: true, Sizing: FitFit,
				Spacing: spacing, shapeType: shapeRectangle,
			},
			Children: children,
		}},
	}

	layoutParents(root, nil)
	layoutWidths(root)
	layoutFillWidths(root, &scratchPools{})
	layoutWrapContainers(root, &Window{})

	wrap := &root.Children[0]
	if !f32AreClose(wrap.Shape.Width, 695) {
		t.Errorf("wrap width: got %f, want 695 (unconstrained)", wrap.Shape.Width)
	}
	if wrap.Shape.Axis != axisLeftToRight {
		t.Error("axis should stay LTR with a rotated ancestor")
	}
	if len(wrap.Children) != childN {
		t.Errorf("children: got %d, want %d", len(wrap.Children), childN)
	}
}

// TestWrapFitWidthAncestorPadding exercises the debts path of
// constrainFitContainerWidth: padding on a Fit ancestor and on the definite
// root both reduce the wrap's available width.
func TestWrapFitWidthAncestorPadding(t *testing.T) {
	const (
		rootWidth  = 300
		rootPad    = 20 // 10 per side
		childWidth = 80
		spacing    = 5
		childCount = 4 // sum 4*80 + 3*5 = 335 > 300-20
	)

	children := make([]Layout, childCount)
	for i := range children {
		children[i] = Layout{Shape: &Shape{
			Width: childWidth, Height: 20, shapeType: shapeRectangle,
		}}
	}

	root := &Layout{
		Shape: &Shape{
			Axis: axisTopToBottom, Width: rootWidth, Height: 400,
			Sizing: FixedFixed, Padding: NewPadding(10, 10, 10, 10),
			shapeType: shapeRectangle,
		},
		Children: []Layout{{
			Shape: &Shape{
				Axis: axisTopToBottom, Sizing: FitFit,
				Padding: NewPadding(5, 5, 5, 5), shapeType: shapeRectangle,
			},
			Children: []Layout{{
				Shape: &Shape{
					Axis: axisLeftToRight, Wrap: true, Sizing: FitFit,
					Spacing: spacing, shapeType: shapeRectangle,
				},
				Children: children,
			}},
		}},
	}

	layoutParents(root, nil)
	layoutWidths(root)
	layoutFillWidths(root, &scratchPools{})
	layoutWrapContainers(root, &Window{})

	col := &root.Children[0]
	wrap := &col.Children[0]
	// available = 300 - root padding (20) - col padding (10) = 270.
	if !f32AreClose(wrap.Shape.Width, 270) {
		t.Errorf("wrap width: got %f, want 270 (300 - 20 - 10)",
			wrap.Shape.Width)
	}
	if wrap.Shape.Axis != axisTopToBottom {
		t.Error("wrap axis should flip to TTB")
	}
	// 3 per row: 3*80 + 2*5 = 250 fits in 270, 4*80 + 3*5 = 335 does not.
	if len(wrap.Children) != 2 {
		t.Errorf("rows: got %d, want 2", len(wrap.Children))
	}
	// The Fit column re-fits to the constrained wrap plus its padding.
	if !f32AreClose(col.Shape.Width, 280) {
		t.Errorf("column width: got %f, want 280 (wrap 270 + padding 10)",
			col.Shape.Width)
	}
}

// TestWrapFitWidthRefitsContentCache pins the cache refresh in
// refitFitAncestors and the post-flip re-cache: a Fit chain's contentW values
// must reflect the constrained width, or scroll offsets, alignment, and
// scrollbar thumbs would read the pre-constraint single-row sum.
func TestWrapFitWidthRefitsContentCache(t *testing.T) {
	const (
		rootWidth  = 300
		childWidth = 80
		spacing    = 5
		childCount = 4 // sum 335 > 300
	)

	children := make([]Layout, childCount)
	for i := range children {
		children[i] = Layout{Shape: &Shape{
			Width: childWidth, Height: 20, shapeType: shapeRectangle,
		}}
	}

	root := &Layout{
		Shape: &Shape{
			Axis: axisTopToBottom, Width: rootWidth, Height: 400,
			Sizing: FixedFixed, shapeType: shapeRectangle,
		},
		Children: []Layout{{
			Shape: &Shape{
				Axis: axisTopToBottom, Sizing: FitFit, shapeType: shapeRectangle,
			},
			Children: []Layout{{
				Shape: &Shape{
					Axis: axisLeftToRight, Wrap: true, Sizing: FitFit,
					Spacing: spacing, shapeType: shapeRectangle,
				},
				Children: children,
			}},
		}},
	}

	layoutParents(root, nil)
	layoutWidths(root)
	layoutFillWidths(root, &scratchPools{})
	layoutWrapContainers(root, &Window{})

	col := &root.Children[0]
	wrap := &col.Children[0]
	// Pre-refit both caches held the 335 single-row sum; the wrap is a TTB
	// column of rows and the column fits from the constrained wrap width.
	if !f32AreClose(wrap.Shape.contentW, rootWidth) {
		t.Errorf("wrap contentW: got %f, want %d (wrapped extent)",
			wrap.Shape.contentW, rootWidth)
	}
	if !f32AreClose(col.Shape.contentW, rootWidth) {
		t.Errorf("column contentW: got %f, want %d (refitted)",
			col.Shape.contentW, rootWidth)
	}
}
