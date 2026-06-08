package gui

// layoutRotationSwap swaps dimensions of layouts with
// QuarterTurns 1 or 3 (90° or 270°). Processes bottom-up so
// nested rotations compose correctly.
func layoutRotationSwap(layout *Layout) {
	for i := range layout.Children {
		layoutRotationSwap(&layout.Children[i])
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
func reaccumulateAncestors(layout *Layout) {
	for layout != nil {
		changed := false
		if layout.Shape.Sizing.Width == SizingFit {
			old := layout.Shape.Width
			layout.Shape.Width = recomputeFitWidth(layout)
			if layout.Shape.Width != old {
				changed = true
			}
		}
		if layout.Shape.Sizing.Height == SizingFit {
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
	case AxisLeftToRight:
		sp := layout.spacing()
		for i := range layout.Children {
			if layout.Children[i].Shape.OverDraw {
				continue
			}
			w += layout.Children[i].Shape.Width
		}
		w += padding + sp
	case AxisTopToBottom:
		for i := range layout.Children {
			w = f32Max(w, layout.Children[i].Shape.Width+padding)
		}
	default:
		for i := range layout.Children {
			w = f32Max(w, layout.Children[i].Shape.Width+padding)
		}
	}
	if layout.Shape.MinWidth > 0 {
		w = f32Max(w, layout.Shape.MinWidth)
	}
	if layout.Shape.MaxWidth > 0 {
		w = f32Min(w, layout.Shape.MaxWidth)
	}
	return w
}

// recomputeFitHeight mirrors layoutHeights accumulation for a
// single Fit node from its direct children.
func recomputeFitHeight(layout *Layout) float32 {
	padding := layout.Shape.paddingHeight()
	var h float32
	switch layout.Shape.Axis {
	case AxisTopToBottom:
		sp := layout.spacing()
		for i := range layout.Children {
			if layout.Children[i].Shape.OverDraw {
				continue
			}
			h += layout.Children[i].Shape.Height
		}
		h += padding + sp
	case AxisLeftToRight:
		for i := range layout.Children {
			h = f32Max(h, layout.Children[i].Shape.Height+padding)
		}
	default:
		for i := range layout.Children {
			h = f32Max(h, layout.Children[i].Shape.Height+padding)
		}
	}
	if layout.Shape.MinHeight > 0 {
		h = f32Max(h, layout.Shape.MinHeight)
	}
	if layout.Shape.MaxHeight > 0 {
		h = f32Min(h, layout.Shape.MaxHeight)
	}
	return h
}
