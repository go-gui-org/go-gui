package gui

// mirrorFloatAttach swaps left/right anchors for RTL mirroring.
func mirrorFloatAttach(a floatAttach) floatAttach {
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
func flipVerticalAttach(a floatAttach) floatAttach {
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
func attachOffset(a floatAttach, w, h float32) (float32, float32) {
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
		//
		// Auto-flip and RTL mirroring are deliberately left out of this
		// branch. Both describe where a float sits relative to the
		// container it was declared in — flipping to the other side of
		// an anchor, mirroring a left anchor to a right one — and a
		// top-level float (dialog, toast, inspector) has no such
		// container. It is placed against the window rect as asked.
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
	ax, ay := floatAnchorPoint(layout, anchor)
	tx, ty := attachOffset(tieOff, shape.Width, shape.Height)
	x, y := offsetThroughTransform(layout, anchor, offsetX, offsetY, ax, ay, tx, ty)

	if !shape.floatAutoFlip {
		return x, y
	}

	fw, fh := shape.Width, shape.Height
	winW, winH := winRect.Width, winRect.Height

	// Vertical flip: try opposite if current overflows.
	curOverY := overflowAxis(y, fh, 0, winH)
	if curOverY > 0 {
		fa := flipVerticalAttach(anchor)
		ft := flipVerticalAttach(tieOff)
		_, fay := floatAnchorPoint(layout, fa)
		_, fty := attachOffset(ft, fw, fh)
		// The offset is a gap from the anchor side. On the opposite
		// side it must point the other way, or a gap becomes an overlap.
		flipOffsetY := offsetY
		if fa != anchor || ft != tieOff {
			flipOffsetY = -offsetY
		}
		newY := fay - fty + flipOffsetY
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
		fax, _ := floatAnchorPoint(layout, fa)
		ftx, _ := attachOffset(ft, fw, fh)
		// See the vertical flip: the gap changes sign with the side.
		flipOffsetX := offsetX
		if fa != anchor || ft != tieOff {
			flipOffsetX = -offsetX
		}
		newX := fax - ftx + flipOffsetX
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

// floatAnchorPoint returns the window point a float with anchor a ties
// to on its parent's box. Under a view transform or a quarter turn the
// point is mapped out to where the parent is drawn (#1019). The float
// itself is drawn in its own layer, outside those transforms, so it
// stays upright beside the turned parent. A parent whose back face is
// hidden has no drawn point, and the box point stands in.
//
// Floats are placed in layout, and a render-only frame re-places them
// from the refreshed matrices (layoutReattachFloats), so a transform
// that TransformFn moves carries its floats with it.
func floatAnchorPoint(layout *Layout, a floatAttach) (float32, float32) {
	p := layout.Parent
	ax, ay := attachOffset(a, p.Shape.Width, p.Shape.Height)
	x, y := p.Shape.X+ax, p.Shape.Y+ay
	if wx, wy, ok := xfLocalToWindowClip(p, x, y, false); ok {
		return wx, wy
	}
	return x, y
}

// offsetThroughTransform places a float from its mapped anchor (ax, ay,
// in window coordinates) and its tie-off (tx, ty). The offset is in the
// parent's layout frame, so under a view transform or a quarter turn the
// offset point is mapped out to where it is drawn, and the tie-off taken
// off there: a drag ghost inside a moved view tracks the pointer (#1019).
// With no transform on the path the map is the identity, as before.
func offsetThroughTransform(
	layout *Layout,
	anchor floatAttach,
	offsetX, offsetY float32,
	ax, ay, tx, ty float32,
) (float32, float32) {
	parent := layout.Parent.Shape
	pax, pay := attachOffset(anchor, parent.Width, parent.Height)
	if mx, my, ok := xfLocalToWindowClip(
		layout.Parent, parent.X+pax+offsetX, parent.Y+pay+offsetY, false,
	); ok {
		return mx - tx, my - ty
	}
	return ax - tx + offsetX, ay - ty + offsetY
}

// layoutReattachFloats recomputes the position of every float in the
// composed tree (w.layout) from its host's current box and transform,
// shifting the float's subtree and refreshing its clips. It runs in a
// render-only frame, where the boxes are frozen but TransformFn may have
// moved a host (#1019). The matrices are refreshed first, so the anchors
// map through the transforms the frame draws with, as the float pipelines
// in layoutArrange see the main pipeline's fresh matrices. Hit clips move
// with the floats, so hit testing follows what is drawn.
//
// Only anchored floats (Parent != nil) move; top-level floats against the
// window rect are untouched. Nested floats live in their own top-level
// layers, so the pass repeats until no float moves: a nested float reads
// its host float's new position on the next pass.
func layoutReattachFloats(w *Window) {
	if w == nil || w.layout.Shape == nil {
		return
	}
	if w.scratch.viewXforms.used == 0 {
		return
	}
	layoutResolveTransforms(&w.layout, w)
	win := w.windowRect()
	for range 10 {
		moved := false
		for i := range w.layout.Children {
			top := &w.layout.Children[i]
			if top.Shape == nil || !top.Shape.Float {
				continue
			}
			if reattachFloatSubtree(top, win, 0) {
				layoutSetShapeClips(top, win)
				moved = true
			}
		}
		if !moved {
			return
		}
	}
}

// reattachFloatSubtree walks ly pre-order, re-placing every anchored
// float with floatAttachLayout and shifting its subtree by the delta.
// A parent moves before its children, so a nested float reads its host's
// new position. It reports whether anything moved.
func reattachFloatSubtree(ly *Layout, win drawClip, depth int) bool {
	if ly == nil || overMaxDepth(depth) {
		return false
	}
	moved := false
	if s := ly.Shape; s != nil && s.Float && ly.Parent != nil {
		if nx, ny := floatAttachLayout(ly, win); nx != s.X || ny != s.Y {
			shiftLayoutXY(ly, nx-s.X, ny-s.Y, 0)
			moved = true
		}
	}
	for i := range ly.Children {
		if reattachFloatSubtree(&ly.Children[i], win, depth+1) {
			moved = true
		}
	}
	return moved
}

// shiftLayoutXY moves ly and all its descendants by (dx, dy). Clips are
// refreshed by the caller (layoutSetShapeClips), since a float crossing
// the window edge changes its clip size, not just its position.
func shiftLayoutXY(ly *Layout, dx, dy float32, depth int) {
	if ly == nil || overMaxDepth(depth) {
		return
	}
	if s := ly.Shape; s != nil {
		s.X += dx
		s.Y += dy
	}
	for i := range ly.Children {
		shiftLayoutXY(&ly.Children[i], dx, dy, depth+1)
	}
}

// liftOverlayFloats extracts the floats declared inside an injected
// overlay layer (toast, dialog, inspector) to their own top-level
// layers above it. A Select dropdown or menu inside a dialog must
// escape the dialog's bounds and scroll viewports the same way a
// main-tree float escapes its ancestors (#819). A lifted float keeps
// its Parent pointer into the overlay, so floatAttachLayout still
// anchors it to its host and layoutDisables still inherits from it.
// A layer with no floats is untouched.
func liftOverlayFloats(layer *Layout, w *Window, layouts *[]*Layout) {
	layoutRemoveFloatingLayouts(layer, w, layouts)
}

// layoutRemoveFloatingLayouts extracts floating elements from the
// main layout tree, replacing them with placeholders.
//
// A float past the depth cap stays in the tree: the walk stops
// descending rather than panicking, the same degradation the other
// capped walks take (see maxEventDepth).
func layoutRemoveFloatingLayouts(layout *Layout, w *Window, layouts *[]*Layout) {
	layoutRemoveFloatingLayoutsDepth(layout, w, layouts, 0)
}

func layoutRemoveFloatingLayoutsDepth(layout *Layout, w *Window, layouts *[]*Layout, depth int) {
	if overMaxDepth(depth) {
		return
	}
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
			layoutRemoveFloatingLayoutsDepth(heapLayout, w, layouts, depth+1)
			if w != nil {
				layout.Children[i] = Layout{
					Shape: w.scratch.allocPlaceholderShape(),
				}
			} else {
				layout.Children[i] = layoutPlaceholder()
			}
		} else {
			layoutRemoveFloatingLayoutsDepth(&layout.Children[i], w, layouts, depth+1)
		}
	}
}
