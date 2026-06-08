package gui

// layoutPositions sets positions and handles alignment.
//
//nolint:gocyclo // alignment + float positioning
func layoutPositions(layout *Layout, offsetX, offsetY float32, w *Window) {
	layout.Shape.X += offsetX
	layout.Shape.Y += offsetY

	axis := layout.Shape.Axis
	spacing := layout.Shape.Spacing

	if layout.Shape.IDScroll > 0 {
		layout.Shape.Clip = true
	}

	isRTL := effectiveTextDir(layout.Shape) == TextDirRTL

	var x, y float32
	if isRTL && axis == AxisLeftToRight {
		x = layout.Shape.X + layout.Shape.Width - layout.Shape.Padding.Left - layout.Shape.SizeBorder
	} else if isRTL {
		x = layout.Shape.X + layout.Shape.Padding.Right + layout.Shape.SizeBorder
	} else {
		x = layout.Shape.X + layout.Shape.PaddingLeft()
	}
	y = layout.Shape.Y + layout.Shape.PaddingTop()

	if layout.Shape.IDScroll > 0 {
		sx := StateMap[uint32, float32](w, nsScrollX, capScroll)
		sy := StateMap[uint32, float32](w, nsScrollY, capScroll)
		if v, ok := sx.Get(layout.Shape.IDScroll); ok {
			x += v
		}
		if v, ok := sy.Get(layout.Shape.IDScroll); ok {
			y += v
		}
	}

	// For rotated containers (90°/270°), children are positioned
	// in the internal (unrotated) coordinate space, centered on
	// the display rect.
	layoutW := layout.Shape.Width
	layoutH := layout.Shape.Height
	turns := layout.Shape.QuarterTurns
	if turns == 1 || turns == 3 {
		contentW := layoutH // swapped back
		contentH := layoutW
		x += (layoutW - contentW) / 2
		y += (layoutH - contentH) / 2
		layoutW = contentW
		layoutH = contentH
	}

	// Resolve start/end based on text direction
	hAlign := layout.Shape.HAlign
	switch hAlign {
	case HAlignStart:
		if isRTL {
			hAlign = HAlignRight
		} else {
			hAlign = HAlignLeft
		}
	case HAlignEnd:
		if isRTL {
			hAlign = HAlignLeft
		} else {
			hAlign = HAlignRight
		}
	}

	// Alignment along the axis
	switch axis {
	case AxisLeftToRight:
		if isRTL {
			if hAlign != HAlignRight {
				remaining := layoutW - layout.Shape.paddingWidth()
				remaining -= layout.spacing()
				for i := range layout.Children {
					remaining -= layout.Children[i].Shape.Width
				}
				if hAlign == HAlignCenter {
					remaining /= 2
				}
				x -= remaining
			}
		} else {
			if hAlign != HAlignLeft {
				remaining := layoutW - layout.Shape.paddingWidth()
				remaining -= layout.spacing()
				for i := range layout.Children {
					remaining -= layout.Children[i].Shape.Width
				}
				if hAlign == HAlignCenter {
					remaining /= 2
				}
				x += remaining
			}
		}
	case AxisTopToBottom:
		if layout.Shape.VAlign != VAlignTop {
			remaining := layoutH - layout.Shape.paddingHeight()
			remaining -= layout.spacing()
			for i := range layout.Children {
				remaining -= layout.Children[i].Shape.Height
			}
			if layout.Shape.VAlign == VAlignMiddle {
				remaining /= 2
			}
			y += remaining
		}
	}

	for i := range layout.Children {
		child := &layout.Children[i]
		var xAlign, yAlign float32

		switch axis {
		case AxisLeftToRight:
			remaining := layoutH - child.Shape.Height - layout.Shape.paddingHeight()
			if remaining > 0 {
				switch layout.Shape.VAlign {
				case VAlignTop:
				case VAlignMiddle:
					yAlign = remaining / 2
				default:
					yAlign = remaining
				}
			}
		case AxisTopToBottom:
			remaining := layoutW - child.Shape.Width - layout.Shape.paddingWidth()
			if remaining > 0 {
				switch hAlign {
				case HAlignLeft:
				case HAlignCenter:
					xAlign = remaining / 2
				default:
					xAlign = remaining
				}
			}
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

// layoutScrollContainers identifies text views in scrollable containers.
func layoutScrollContainers(layout *Layout, idScrollContainer uint32) {
	activeID := idScrollContainer
	if layout.Shape.IDScroll > 0 {
		activeID = layout.Shape.IDScroll
	}
	if layout.Shape.shapeType == shapeText {
		layout.Shape.IDScrollContainer = activeID
	}
	for i := range layout.Children {
		layoutScrollContainers(&layout.Children[i], activeID)
	}
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
	idScroll := layout.Shape.IDScroll
	if idScroll > 0 {
		sx := StateMap[uint32, float32](w, nsScrollX, capScroll)
		sy := StateMap[uint32, float32](w, nsScrollY, capScroll)
		maxOffsetX := f32Min(0, layout.Shape.Width-layout.Shape.paddingWidth()-contentWidth(layout))
		if offsetX, ok := sx.Get(idScroll); ok {
			sx.Set(idScroll, f32Clamp(offsetX, maxOffsetX, 0))
		} else {
			sx.Set(idScroll, f32Clamp(0, maxOffsetX, 0))
		}
		maxOffsetY := f32Min(0, layout.Shape.Height-layout.Shape.paddingHeight()-contentHeight(layout))
		if offsetY, ok := sy.Get(idScroll); ok {
			sy.Set(idScroll, f32Clamp(offsetY, maxOffsetY, 0))
		} else {
			sy.Set(idScroll, f32Clamp(0, maxOffsetY, 0))
		}
	}
	for i := range layout.Children {
		layoutAdjustScrollOffsets(&layout.Children[i], w)
	}
}
