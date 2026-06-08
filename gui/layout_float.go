package gui

// mirrorFloatAttach swaps left/right anchors for RTL mirroring.
func mirrorFloatAttach(a FloatAttach) FloatAttach {
	switch a {
	case FloatTopLeft:
		return FloatTopRight
	case FloatTopRight:
		return FloatTopLeft
	case FloatMiddleLeft:
		return FloatMiddleRight
	case FloatMiddleRight:
		return FloatMiddleLeft
	case FloatBottomLeft:
		return FloatBottomRight
	case FloatBottomRight:
		return FloatBottomLeft
	default:
		return a
	}
}

// flipVerticalAttach swaps top/bottom anchors, preserving the
// horizontal component (Left/Center/Right).
func flipVerticalAttach(a FloatAttach) FloatAttach {
	switch a {
	case FloatTopLeft:
		return FloatBottomLeft
	case FloatTopCenter:
		return FloatBottomCenter
	case FloatTopRight:
		return FloatBottomRight
	case FloatBottomLeft:
		return FloatTopLeft
	case FloatBottomCenter:
		return FloatTopCenter
	case FloatBottomRight:
		return FloatTopRight
	default:
		return a
	}
}

// attachOffset returns the (dx, dy) offset for a FloatAttach
// point given element dimensions w and h.
func attachOffset(a FloatAttach, w, h float32) (float32, float32) {
	switch a {
	case FloatTopCenter:
		return w / 2, 0
	case FloatTopRight:
		return w, 0
	case FloatMiddleLeft:
		return 0, h / 2
	case FloatMiddleCenter:
		return w / 2, h / 2
	case FloatMiddleRight:
		return w, h / 2
	case FloatBottomLeft:
		return 0, h
	case FloatBottomCenter:
		return w / 2, h
	case FloatBottomRight:
		return w, h
	default:
		return 0, 0
	}
}

// overflowAxis returns the total overflow of an element positioned
// at pos with the given size within [min, max] bounds.
func overflowAxis(pos, size, minVal, maxVal float32) float32 {
	var overflow float32
	if pos < minVal {
		overflow += minVal - pos
	}
	if pos+size > maxVal {
		overflow += pos + size - maxVal
	}
	return overflow
}

// floatAttachLayout computes the position of float layouts relative
// to their parent. When FloatAutoFlip is set, overflowing floats
// are flipped to the opposite side of the anchor if it reduces
// overflow, then clamped to window bounds.
func floatAttachLayout(
	layout *Layout, winRect drawClip,
) (float32, float32) {
	if layout.Parent == nil {
		// No parent — use window rect as anchor reference
		// so dialogs and other top-level floats position
		// correctly (e.g. FloatMiddleCenter centers in window).
		s := layout.Shape
		ax, ay := attachOffset(s.FloatAnchor,
			winRect.Width, winRect.Height)
		tx, ty := attachOffset(s.FloatTieOff,
			s.Width, s.Height)
		return ax - tx + s.FloatOffsetX, ay - ty + s.FloatOffsetY
	}
	parent := layout.Parent.Shape
	isRTL := effectiveTextDir(parent) == TextDirRTL

	anchor := layout.Shape.FloatAnchor
	tieOff := layout.Shape.FloatTieOff
	offsetX := layout.Shape.FloatOffsetX
	offsetY := layout.Shape.FloatOffsetY
	if isRTL {
		anchor = mirrorFloatAttach(anchor)
		tieOff = mirrorFloatAttach(tieOff)
		offsetX = -offsetX
	}

	shape := layout.Shape
	ax, ay := attachOffset(anchor, parent.Width, parent.Height)
	tx, ty := attachOffset(tieOff, shape.Width, shape.Height)
	x := parent.X + ax - tx + offsetX
	y := parent.Y + ay - ty + offsetY

	if !shape.FloatAutoFlip {
		return x, y
	}

	fw, fh := shape.Width, shape.Height
	winW, winH := winRect.Width, winRect.Height

	// Vertical flip: try opposite if current overflows.
	curOverY := overflowAxis(y, fh, 0, winH)
	if curOverY > 0 {
		fa := flipVerticalAttach(anchor)
		ft := flipVerticalAttach(tieOff)
		_, fay := attachOffset(fa, parent.Width, parent.Height)
		_, fty := attachOffset(ft, fw, fh)
		newY := parent.Y + fay - fty + offsetY
		if overflowAxis(newY, fh, 0, winH) < curOverY {
			y = newY
			anchor = fa
			tieOff = ft
		}
	}

	// Horizontal flip: try mirrored if current overflows.
	curOverX := overflowAxis(x, fw, 0, winW)
	if curOverX > 0 {
		fa := mirrorFloatAttach(anchor)
		ft := mirrorFloatAttach(tieOff)
		fax, _ := attachOffset(fa, parent.Width, parent.Height)
		ftx, _ := attachOffset(ft, fw, fh)
		newX := parent.X + fax - ftx + offsetX
		if overflowAxis(newX, fw, 0, winW) < curOverX {
			x = newX
		}
	}

	// Clamp to window bounds as safety net.
	if x+fw > winW {
		x = winW - fw
	}
	if y+fh > winH {
		y = winH - fh
	}
	x = max(x, 0)
	y = max(y, 0)

	return x, y
}

// layoutRemoveFloatingLayouts extracts floating elements from the
// main layout tree, replacing them with placeholders.
func layoutRemoveFloatingLayouts(layout *Layout, w *Window, layouts *[]*Layout) {
	for i := range layout.Children {
		if layout.Children[i].Shape.Float {
			var heapLayout *Layout
			if w != nil {
				heapLayout = w.scratch.allocFloatingLayout(layout.Children[i])
			} else {
				cp := layout.Children[i]
				heapLayout = &cp
			}
			for j := range heapLayout.Children {
				heapLayout.Children[j].Parent = heapLayout
			}
			*layouts = append(*layouts, heapLayout)
			layoutRemoveFloatingLayouts(heapLayout, w, layouts)
			if w != nil {
				layout.Children[i] = Layout{
					Shape: w.scratch.allocPlaceholderShape(),
				}
			} else {
				layout.Children[i] = layoutPlaceholder()
			}
		} else {
			layoutRemoveFloatingLayouts(&layout.Children[i], w, layouts)
		}
	}
}
