package gui

// wrapRowRange describes a contiguous range of children in one wrap row.
type wrapRowRange struct {
	start, end int
}

// maxFitConstraintDepth caps the ancestor walk for Fit-width wrap/overflow
// containers. A chain deeper than this stays unconstrained (single-row
// behavior) rather than guessing at a width eight levels down.
const maxFitConstraintDepth = 8

// constrainFitContainerWidth resolves a Fit-width Wrap/Overflow container's
// width against its nearest definite-width ancestor before it breaks rows
// (or hides children). Without this the width is its own single-row content
// sum, so a wrap measures against a width that always fits and never breaks
// (issue #379).
//
// A node's width is "definite" when it does not derive from this container:
// Fixed at generation, Fill resolved by the fill pass that just ran, or an
// axisNone node (no child accumulation at all — Canvas/Splitter roots). A
// Fit-width ancestor is self-derived — it includes this container's width,
// so it contributes only its padding and, for a row, the width its other
// children take. The result is the CSS fit-content rule: min(single-row
// sum, available), floored at MinWidth so a container wider than its
// ancestor still fits its widest child (the issue #378 floor).
//
// The walk stops with no constraint at a nil parent — an all-Fit chain has
// no width to wrap within, so it keeps the single-row sum — and at a
// rotated ancestor, whose dimensions a later pass swaps. The visited nodes
// come back in path (depth of them) so the caller can re-fit and re-cache
// exactly those.
func constrainFitContainerWidth(layout *Layout) (changed bool, path [maxFitConstraintDepth]*Layout, depth int) {
	if layout.Parent == nil || layout.Shape.Sizing.Width != sizingFit {
		return false, path, 0
	}
	// debts accumulates what the nodes on the path consume: each node's
	// padding plus, for a row, its other children and inter-child spacing.
	var debts float32
	child := layout
	for n := layout.Parent; n != nil; n = n.Parent {
		if n.Shape.QuarterTurns != 0 || depth == maxFitConstraintDepth {
			return false, path, depth
		}
		path[depth] = n
		depth++
		if n.Shape.Sizing.Width == sizingFit && n.Shape.Axis != axisNone {
			debts += n.Shape.paddingWidth()
			if n.Shape.Axis == axisLeftToRight {
				debts += siblingsConsumeWidth(n, child)
			}
			child = n
			continue
		}
		available := n.Shape.Width - n.Shape.paddingWidth() - debts
		if n.Shape.Axis == axisLeftToRight {
			available -= siblingsConsumeWidth(n, child)
		}
		newW := f32Min(layout.Shape.Width, f32Max(available, 0))
		if layout.Shape.MinWidth > 0 {
			newW = f32Max(newW, layout.Shape.MinWidth)
		}
		if f32AreClose(newW, layout.Shape.Width) {
			return false, path, depth
		}
		layout.Shape.Width = newW
		return true, path, depth
	}
	return false, path, depth
}

// siblingsConsumeWidth returns what a row node's other visible children
// take: their widths plus the inter-child spacing. child must be one of
// n.Children.
func siblingsConsumeWidth(n *Layout, child *Layout) float32 {
	var sum float32
	for i := range n.Children {
		c := &n.Children[i]
		if c == child || skipLayoutChild(c.Shape) {
			continue
		}
		sum += c.Shape.Width
	}
	return sum + n.spacing()
}

// refitFitAncestors propagates a width constraint up the Fit ancestor chain
// (re-fitting each from its children, as rotation does) and refreshes the
// content-width caches the fill pass recorded before the constraint applied.
// The height fill pass re-arms fillGen without re-caching contentW, so
// alignment/scroll reads (applyContainerAlignment, scroll offsets, scrollbar
// thumbs) would otherwise see the pre-constraint sum. path holds the
// ancestors visited by constrainFitContainerWidth, definite node included.
func refitFitAncestors(layout *Layout, path []*Layout) {
	reaccumulateAncestors(layout.Parent)
	for _, n := range path {
		n.Shape.contentW = computeContentWidth(n)
	}
}

// layoutWrapContainers restructures wrap containers into column-of-rows.
func layoutWrapContainers(layout *Layout, w *Window) {
	for i := range layout.Children {
		layoutWrapContainers(&layout.Children[i], w)
	}

	if !layout.Shape.Wrap || layout.Shape.Axis != axisLeftToRight || len(layout.Children) == 0 {
		return
	}

	// A Fit-width wrap resolves against its nearest definite-width ancestor
	// (issue #379). Without the constraint its width is its own single-row
	// sum, so `available` below always fits every child and no row breaks.
	changed, path, depth := constrainFitContainerWidth(layout)
	if changed {
		refitFitAncestors(layout, path[:depth])
	}

	available := layout.Shape.Width - layout.Shape.paddingWidth()
	if available <= 0 {
		return
	}

	spacing := layout.Shape.Spacing

	var rows []wrapRowRange
	if w != nil {
		rows = w.scratch.wrapRows.take(len(layout.Children))
		defer func() {
			w.scratch.wrapRows.put(rows)
		}()
	}
	rowStart := 0
	var rowWidth float32
	rowHasFlowChild := false

	for idx := range layout.Children {
		child := &layout.Children[idx]
		if skipLayoutChild(child.Shape) {
			continue
		}
		childW := child.Shape.Width
		var gap float32
		if rowHasFlowChild {
			gap = spacing
		}
		if rowHasFlowChild && rowWidth+gap+childW > available && idx > rowStart {
			rows = append(rows, wrapRowRange{start: rowStart, end: idx})
			rowStart = idx
			rowWidth = 0
			rowHasFlowChild = false
		}
		if rowHasFlowChild {
			rowWidth += spacing
		}
		rowWidth += childW
		rowHasFlowChild = true
	}
	if rowHasFlowChild && rowStart < len(layout.Children) {
		rows = append(rows, wrapRowRange{start: rowStart, end: len(layout.Children)})
	}

	if len(rows) <= 1 {
		return
	}

	layout.Shape.Axis = axisTopToBottom

	newChildren := make([]Layout, 0, len(rows))
	for i := range rows {
		row := rows[i]
		rowChildren := layout.Children[row.start:row.end:row.end]
		rowShape := Shape{
			shapeType: shapeRectangle,
			Axis:      axisLeftToRight,
			Sizing:    FixedFit,
			Width:     available,
			Spacing:   spacing,
			Color:     Color{},
			HAlign:    layout.Shape.HAlign,
			VAlign:    layout.Shape.VAlign,
			TextDir:   layout.Shape.TextDir,
		}
		var sp *Shape
		if w != nil {
			sp = w.scratch.viewShapes.alloc(rowShape)
		} else {
			sp = &rowShape
		}
		newChildren = append(newChildren, Layout{
			Shape:    sp,
			Children: rowChildren,
		})
	}

	layout.Children = newChildren
	for i := range layout.Children {
		layout.Children[i].Parent = layout
		for j := range layout.Children[i].Children {
			layout.Children[i].Children[j].Parent = &layout.Children[i]
		}
	}
	if changed {
		// The wrap is now a column of rows; the fill pass cached its
		// content width as the single-row sum, so re-cache it at the
		// wrapped extent. Runs after the children swap — reading the
		// flat pre-wrap children here is what the single-row sum would
		// have been (max child, not the row extent).
		layout.Shape.contentW = computeContentWidth(layout)
	}
}
