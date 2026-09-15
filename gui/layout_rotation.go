package gui

// layoutRotationSwap swaps dimensions of layouts with
// QuarterTurns 1 or 3 (90° or 270°). Processes bottom-up so
// nested rotations compose correctly.
func layoutRotationSwap(layout *Layout) {
	layoutRotationSwapDepth(layout, 0)
}

func layoutRotationSwapDepth(layout *Layout, depth int) {
	if overMaxDepth(depth) {
		return
	}
	for i := range layout.Children {
		layoutRotationSwapDepth(&layout.Children[i], depth+1)
	}
	turns := layout.Shape.QuarterTurns
	if turns != 1 && turns != 3 {
		return
	}
	layout.Shape.Width, layout.Shape.Height =
		layout.Shape.Height, layout.Shape.Width
	layout.Shape.MinWidth, layout.Shape.MinHeight =
		layout.Shape.MinHeight, layout.Shape.MinWidth
	layout.Shape.MaxWidth, layout.Shape.MaxHeight =
		layout.Shape.MaxHeight, layout.Shape.MaxWidth
	reaccumulateAncestors(layout.Parent)
}

// reaccumulateAncestors re-computes Fit dimensions for
// ancestors after a rotation swap. Stops when a Fixed/Fill
// ancestor is reached or no change occurs.
//
// Every node it visits has a direct child whose size just changed, so
// each one also gets its contentW/contentH caches refreshed. The fill
// passes cached those from the un-swapped tree, and the position pass
// reads them after this one (applyContainerAlignment, the scroll clamp,
// scrollbar thumbs). The refresh runs before the stop check on purpose:
// a Fixed or Fill parent keeps its size but not its content extent, and
// a scrolling parent is exactly that case. The rotated node's own caches
// need no refresh: its children did not change, and layoutRotatedDims
// measures them in the un-rotated frame.
func reaccumulateAncestors(layout *Layout) {
	for layout != nil {
		// Children are final here; refresh before Width/Height move so
		// the recompute below and the caches read the same child sizes.
		layout.Shape.contentW = computeContentWidth(layout)
		layout.Shape.contentH = computeContentHeight(layout)
		changed := false
		if layout.Shape.Sizing.Width == sizingFit {
			old := layout.Shape.Width
			layout.Shape.Width = recomputeFitWidth(layout)
			if layout.Shape.Width != old {
				changed = true
			}
		}
		if layout.Shape.Sizing.Height == sizingFit {
			old := layout.Shape.Height
			layout.Shape.Height = recomputeFitHeight(layout)
			if layout.Shape.Height != old {
				changed = true
			}
		}
		if !changed {
			break
		}
		layout = layout.Parent
	}
}

// recomputeFitWidth mirrors layoutWidths accumulation for a
// single Fit node from its direct children.
func recomputeFitWidth(layout *Layout) float32 {
	padding := layout.Shape.paddingWidth()
	var w float32
	switch layout.Shape.Axis {
	case axisLeftToRight:
		sp := layout.spacing()
		for i := range layout.Children {
			if skipLayoutChild(layout.Children[i].Shape) {
				continue
			}
			w += layout.Children[i].Shape.Width
		}
		w += padding + sp
	case axisTopToBottom:
		for i := range layout.Children {
			if skipLayoutChild(layout.Children[i].Shape) {
				continue
			}
			w = f32Max(w, layout.Children[i].Shape.Width+padding)
		}
	default:
		// axisNone places each child at its own X, so the fit encloses
		// X + extent, the same rule as fitAxisNoneWidth (issue #584). X
		// is still parent-relative: this pass runs before layoutPositions.
		if extent, _, found := axisNoneExtentW(layout); found {
			w = extent + padding
		} else {
			// Nothing in flow to enclose: keep the current width, as
			// fitAxisNoneWidth does, rather than collapsing to 0.
			w = layout.Shape.Width
		}
	}
	// Max wins over a larger Min, the rule every sizing site shares.
	return clampSize(w, layout.Shape.MinWidth, layout.Shape.MaxWidth)
}

// recomputeFitHeight mirrors layoutHeights accumulation for a
// single Fit node from its direct children.
func recomputeFitHeight(layout *Layout) float32 {
	padding := layout.Shape.paddingHeight()
	var h float32
	switch layout.Shape.Axis {
	case axisTopToBottom:
		sp := layout.spacing()
		for i := range layout.Children {
			if skipLayoutChild(layout.Children[i].Shape) {
				continue
			}
			h += layout.Children[i].Shape.Height
		}
		h += padding + sp
	case axisLeftToRight:
		for i := range layout.Children {
			if skipLayoutChild(layout.Children[i].Shape) {
				continue
			}
			h = f32Max(h, layout.Children[i].Shape.Height+padding)
		}
	default:
		// See recomputeFitWidth: enclose Y + height, as
		// fitAxisNoneHeight does.
		if extent, _, found := axisNoneExtentH(layout); found {
			h = extent + padding
		} else {
			// See recomputeFitWidth: keep the current height.
			h = layout.Shape.Height
		}
	}
	return clampSize(h, layout.Shape.MinHeight, layout.Shape.MaxHeight)
}
