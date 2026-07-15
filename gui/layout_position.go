package gui

// layoutPositions sets positions and handles alignment.
func layoutPositions(layout *Layout, offsetX, offsetY float32, w *Window) {
	layout.Shape.X += offsetX
	layout.Shape.Y += offsetY

	axis := layout.Shape.Axis
	spacing := layout.Shape.Spacing

	if layout.Shape.Scrollable {
		layout.Shape.Clip = true
	}

	isRTL := effectiveTextDir(layout.Shape) == TextDirRTL

	x, y := layoutChildStartPos(layout, isRTL, axis, w)
	layoutW, layoutH := layoutRotatedDims(layout, &x, &y)
	hAlign := resolveHAlign(layout.Shape.HAlign, isRTL)
	x, y = applyContainerAlignment(layout, hAlign, axis, isRTL, x, y, layoutW, layoutH)

	for i := range layout.Children {
		child := &layout.Children[i]
		var xAlign, yAlign float32
		if axis == AxisLeftToRight {
			yAlign = childCrossAxisVAlign(child, layout.Shape, layoutH)
		} else {
			xAlign = childCrossAxisHAlign(child, layout.Shape, hAlign, layoutW)
		}

		if isRTL && axis == AxisLeftToRight {
			layoutPositions(child, x-child.Shape.Width+xAlign, y+yAlign, w)
		} else {
			layoutPositions(child, x+xAlign, y+yAlign, w)
		}

		if child.Shape.shapeType != shapeNone && !child.Shape.OverDraw {
			switch axis {
			case AxisLeftToRight:
				if isRTL {
					x -= child.Shape.Width + spacing
				} else {
					x += child.Shape.Width + spacing
				}
			case AxisTopToBottom:
				y += child.Shape.Height + spacing
			}
		}
	}
}

// layoutChildStartPos returns the starting x,y for child positioning,
// adjusted for text direction, scroll offset, and padding.
func layoutChildStartPos(
	layout *Layout, isRTL bool, axis Axis, w *Window,
) (x, y float32) {
	if isRTL && axis == AxisLeftToRight {
		x = layout.Shape.X + layout.Shape.Width -
			layout.Shape.Padding.Left - layout.Shape.SizeBorder
	} else if isRTL {
		x = layout.Shape.X + layout.Shape.Padding.Right +
			layout.Shape.SizeBorder
	} else {
		x = layout.Shape.X + layout.Shape.PaddingLeft()
	}
	y = layout.Shape.Y + layout.Shape.PaddingTop()

	if layout.Shape.Scrollable {
		sx := w.scrollX()
		sy := w.scrollY()
		if v, ok := sx.Get(layout.Shape.ID); ok {
			x += v
		}
		if v, ok := sy.Get(layout.Shape.ID); ok {
			y += v
		}
	}
	return x, y
}

// layoutRotatedDims handles quarter-turn dimension swapping and
// adjusts x,y to center children in the internal coordinate space.
func layoutRotatedDims(layout *Layout, x, y *float32) (w, h float32) {
	w = layout.Shape.Width
	h = layout.Shape.Height
	turns := layout.Shape.QuarterTurns
	if turns == 1 || turns == 3 {
		contentW := h // swapped back
		contentH := w
		*x += (w - contentW) / 2
		*y += (h - contentH) / 2
		w = contentW
		h = contentH
	}
	return w, h
}

// resolveHAlign maps logical start/end alignment to physical
// left/right based on text direction.
func resolveHAlign(hAlign HorizontalAlign, isRTL bool) HorizontalAlign {
	switch hAlign {
	case HAlignStart:
		if isRTL {
			return HAlignRight
		}
		return HAlignLeft
	case HAlignEnd:
		if isRTL {
			return HAlignLeft
		}
		return HAlignRight
	default:
		return hAlign
	}
}

// applyContainerAlignment adjusts the start position based on
// horizontal or vertical alignment within the container. Uses the
// fill-pass content dimension cache to avoid redundant child-tree
// summation.
func applyContainerAlignment(
	layout *Layout, hAlign HorizontalAlign, axis Axis, isRTL bool,
	x, y, layoutW, layoutH float32,
) (float32, float32) {
	switch axis {
	case AxisLeftToRight:
		var remaining float32
		if isRTL && hAlign != HAlignRight ||
			!isRTL && hAlign != HAlignLeft {
			remaining = layoutW - layout.Shape.paddingWidth() -
				contentWidth(layout)
			if hAlign == HAlignCenter {
				remaining /= 2
			}
		}
		if isRTL {
			x -= remaining
		} else {
			x += remaining
		}
	case AxisTopToBottom:
		if layout.Shape.VAlign != VAlignTop {
			remaining := layoutH - layout.Shape.paddingHeight() -
				contentHeight(layout)
			if layout.Shape.VAlign == VAlignMiddle {
				remaining /= 2
			}
			y += remaining
		}
	}
	return x, y
}

// childCrossAxisVAlign computes the vertical offset to center or
// bottom-align a child within a horizontal layout (AxisLeftToRight).
func childCrossAxisVAlign(
	child *Layout, parent *Shape, layoutH float32,
) (yAlign float32) {
	remaining := layoutH - child.Shape.Height -
		parent.paddingHeight()
	if remaining > 0 {
		switch parent.VAlign {
		case VAlignTop:
		case VAlignMiddle:
			yAlign = remaining / 2
		default:
			yAlign = remaining
		}
	}
	return yAlign
}

// childCrossAxisHAlign computes the horizontal offset to center or
// right-align a child within a vertical layout (AxisTopToBottom).
func childCrossAxisHAlign(
	child *Layout, parent *Shape,
	hAlign HorizontalAlign, layoutW float32,
) (xAlign float32) {
	remaining := layoutW - child.Shape.Width -
		parent.paddingWidth()
	if remaining > 0 {
		switch hAlign {
		case HAlignLeft:
		case HAlignCenter:
			xAlign = remaining / 2
		default:
			xAlign = remaining
		}
	}
	return xAlign
}

// layoutSetShapeClips sets shape clips used for hit testing.
func layoutSetShapeClips(layout *Layout, clip drawClip) {
	shapeClip := shapeBounds(layout.Shape)
	if r, ok := rectIntersection(shapeClip, clip); ok {
		layout.Shape.shapeClip = r
	} else {
		layout.Shape.shapeClip = drawClip{}
	}
	childClip := layout.Shape.shapeClip
	// For rotated containers, children live in the internal
	// (unrotated) coordinate space which may be larger than
	// the display rect in the swapped dimension.
	if turns := layout.Shape.QuarterTurns; turns == 1 || turns == 3 {
		dw := layout.Shape.Width
		dh := layout.Shape.Height
		cx := layout.Shape.X + dw/2
		cy := layout.Shape.Y + dh/2
		childClip = drawClip{
			X: cx - dh/2, Y: cy - dw/2,
			Width: dh, Height: dw,
		}
	}
	for i := range layout.Children {
		layoutSetShapeClips(&layout.Children[i], childClip)
	}
}

// layoutAdjustScrollOffsets ensures scroll offsets are in range.
func layoutAdjustScrollOffsets(layout *Layout, w *Window) {
	id := layout.Shape.ID
	if layout.Shape.Scrollable && id != "" {
		sx := w.scrollX()
		sy := w.scrollY()
		maxOffsetX := f32Min(0, layout.Shape.Width-layout.Shape.paddingWidth()-contentWidth(layout))
		if offsetX, ok := sx.Get(id); ok {
			sx.Set(id, f32Clamp(offsetX, maxOffsetX, 0))
		} else {
			sx.Set(id, f32Clamp(0, maxOffsetX, 0))
		}
		maxOffsetY := f32Min(0, layout.Shape.Height-layout.Shape.paddingHeight()-contentHeight(layout))
		if offsetY, ok := sy.Get(id); ok {
			sy.Set(id, f32Clamp(offsetY, maxOffsetY, 0))
		} else {
			sy.Set(id, f32Clamp(0, maxOffsetY, 0))
		}
	}
	for i := range layout.Children {
		layoutAdjustScrollOffsets(&layout.Children[i], w)
	}
}
