package gui

import (
	"log"
)

// shapeTransform is the transform a container carries
// (ContainerCfg.Transform and TransformFn). The render pass resolves it
// against the arranged box each time it draws the shape and keeps the
// result here. Input reads the same matrix, so the pointer is mapped
// through the transform the frame was drawn with, even when TransformFn
// changed it in a render-only frame.
type shapeTransform struct {
	fn     func() Transform
	static Transform
	m      xfMat // own frame -> parent frame, logical px
	inv    xfMat // parent frame -> own frame; valid when invOK
	invOK  bool
	hidden bool // back face toward the viewer, and BackfaceHidden set
	// inert is ContainerCfg.inert. It lives here, not on Shape, so the
	// flag costs no Shape bytes: every hit test of a shape with a
	// transform already goes through xfCoordsInverse, which refuses an
	// inert one, and the tab and a11y walks read it through isInert.
	inert bool
	// resolved is set once update has run for the current box. Until
	// then m and inv are zero, and input treats the shape as having no
	// transform.
	resolved bool
	// outer is the clip the ancestors pass, in the parent's frame. Layout
	// sets it. The pointer is tested against it before it is mapped into
	// the view's frame, where no rect holds that clip (xfCoordsInverse).
	// It does not depend on the matrix, so it stays right when a
	// render-only frame changes the transform.
	outer drawClip
}

// xfFrame is one open transform bracket in the render walk.
type xfFrame struct {
	acc  xfMat    // own frame -> window: every open bracket, composed
	clip drawClip // the window scissor in effect when the bracket opened
	// axis: acc only moves, scales and turns by quarter turns. A rect
	// maps to a rect, so a scissor stays exact.
	axis bool
	// flat is a frame for a view whose transform is the identity in
	// this frame. No TransformBegin was emitted for it, so none is
	// closed. It is pushed so the subtree still takes its clips from
	// the render walk, not from the layout clips: those were cut through
	// the transform that was in effect when layout ran (see update).
	flat bool
}

// xfFar is half the side of xfUnboundedClip, in logical pixels.
const xfFar = 1 << 20

// xfUnboundedClip is the clip a transformed view and its children are
// drawn with. The clip a parent passes is a rect in the parent's frame,
// and in the view's own frame it becomes a turned or skewed shape that
// no rect holds. So drawing inside the view culls against nothing.
// Nothing leaks: the scissor still cuts in the window (emitClipCmd maps
// every rect out through the open brackets).
var xfUnboundedClip = drawClip{X: -xfFar, Y: -xfFar, Width: 2 * xfFar, Height: 2 * xfFar}

// current returns the transform to draw with: TransformFn's value when
// the shape has one, else the static Transform. TransformFn is app code
// running inside the render walk, so a panic in it is isolated like one
// in a DrawCanvas VersionFn, and the static Transform stands in.
func (xf *shapeTransform) current(shape *Shape, w *Window) (t Transform) {
	if xf.fn == nil {
		return xf.static
	}
	defer func() {
		if r := recover(); r != nil {
			t = xf.static
			if !w.transformFnPanicWarned {
				w.transformFnPanicWarned = true
				log.Printf("gui: TransformFn panicked (id %q) — using "+
					"Transform: %v", shape.idKey(), r)
			}
		}
	}()
	return xf.fn()
}

// update resolves the shape's transform against its box and stores the
// matrix, its inverse and the back face test. It returns the transform
// it resolved. Layout runs it so hit testing has a matrix before the
// first frame is drawn. The render walk runs it again, so the pointer is
// mapped through the matrix the frame was drawn with, even when
// TransformFn changed it in a render-only frame.
func (xf *shapeTransform) update(shape *Shape, w *Window) Transform {
	t := xf.current(shape, w)
	xf.m, xf.hidden = t.resolve(shape.X, shape.Y, shape.Width, shape.Height)
	xf.inv, xf.invOK = xf.m.inverse()
	xf.resolved = true
	return t
}

// renderTransformBegin resolves the shape's transform against its box,
// stores the matrix for input, and opens a transform bracket. It reports
// whether a bracket opened, and whether the subtree must be drawn with
// xfUnboundedClip. It opens none when the back face is toward the viewer
// and hidden (xf.hidden): the caller then draws nothing of the subtree.
// When one opened, the caller closes it with renderTransformEnd after
// the subtree.
//
// An identity transform emits no TransformBegin, but still opens a flat
// frame (see xfFrame.flat), and the subtree keeps the parent's clip.
//
// clip is the clip the parent passed, in the parent's frame. It is the
// scissor in effect here, so it becomes the frame's entry scissor.
func renderTransformBegin(shape *Shape, clip drawClip, w *Window) (opened, unbounded bool) {
	xf := shape.xf
	t := xf.update(shape, w)
	if xf.hidden {
		return false, false
	}
	parent, entry := xfParentFrame(clip, w)
	flat := t.IsIdentity() || !emitRendererIfValid(RenderCmd{
		Kind:       RenderTransformBegin,
		Homography: (*[9]float32)(&xf.m),
		// The box, in the view's own frame. A backend with no
		// perspective (web) fits its flat stand-in to the view here.
		X: shape.X, Y: shape.Y, W: shape.Width, H: shape.Height,
	}, w)
	if flat {
		// Nothing is drawn through this matrix, so input must not map
		// the pointer through it either.
		xf.m, xf.inv, xf.invOK = xfIdentity, xfIdentity, true
		w.xfStack = append(w.xfStack, xfFrame{acc: parent, clip: entry,
			axis: parent.axisAligned(), flat: true})
		return true, false
	}
	acc := parent.mul(&xf.m)
	w.xfStack = append(w.xfStack, xfFrame{acc: acc, clip: entry, axis: acc.axisAligned()})
	return true, true
}

// xfParentFrame returns what a new frame on w.xfStack builds on: the map
// from the parent's frame to the window, and clip (in the parent's frame)
// as a window scissor, the new frame's entry scissor. With no frame open
// both are as given.
func xfParentFrame(clip drawClip, w *Window) (xfMat, drawClip) {
	n := len(w.xfStack)
	if n == 0 {
		return xfIdentity, clip
	}
	top := &w.xfStack[n-1]
	return top.acc, top.toWindow(clip)
}

// renderTransformEnd closes the bracket renderTransformBegin opened.
func renderTransformEnd(w *Window) {
	n := len(w.xfStack) - 1
	if !w.xfStack[n].flat {
		emitRenderer(RenderCmd{Kind: RenderTransformEnd}, w)
	}
	w.xfStack = w.xfStack[:n]
}

// renderQuarterTurnBegin opens the bracket of a quarter turn
// (Shape.QuarterTurns): it emits RotateBegin and opens a frame on
// w.xfStack, so the clips and boxes drawn inside map out through the
// turn like those inside a view transform (#1019). It returns the clip
// for the children, in their unturned frame. clip is the clip in effect
// here, in the shape's frame. The caller closes the bracket with
// renderQuarterTurnEnd after the children.
//
// A quarter turn moves only the children, so the shape's own fill is
// drawn before this.
func renderQuarterTurnBegin(shape *Shape, clip drawClip, w *Window) drawClip {
	emitRenderer(RenderCmd{
		Kind:     RenderRotateBegin,
		RotAngle: float32(shape.QuarterTurns) * 90,
		RotCX:    shape.X + shape.Width/2,
		RotCY:    shape.Y + shape.Height/2,
	}, w)
	parent, entry := xfParentFrame(clip, w)
	turn := quarterTurnForward(shape)
	acc := parent.mul(&turn)
	w.xfStack = append(w.xfStack, xfFrame{acc: acc, clip: entry, axis: acc.axisAligned()})
	return rotateClipToUnrotated(clip, shape)
}

// renderQuarterTurnEnd closes the bracket renderQuarterTurnBegin opened.
func renderQuarterTurnEnd(w *Window) {
	emitRenderer(RenderCmd{Kind: RenderRotateEnd}, w)
	w.xfStack = w.xfStack[:len(w.xfStack)-1]
}

// renderShapeClip returns the clip the render walk uses as the shape's
// own: its layout clip, or inside a transform its bounds cut to the clip
// its parent passed. The layout clip is cut by ancestors in the window,
// which is right for hit testing but not for drawing in the view's own
// frame. The parent's clip is in that frame, and nesting it keeps an
// inner clipping container inside an outer one.
func renderShapeClip(shape *Shape, clip drawClip, w *Window) drawClip {
	if len(w.xfStack) == 0 {
		return shape.shapeClip
	}
	r, ok := rectIntersection(shapeBounds(shape), clip)
	if !ok {
		return drawClip{}
	}
	return r
}

// xfNeedsStencil reports whether a clip inside the open brackets needs a
// stencil to be exact. A scissor is a window rect. Under a turn, a skew
// or perspective, the clip rect is drawn as some other four-sided shape,
// and the scissor can only be its bounding box.
func xfNeedsStencil(w *Window) bool {
	n := len(w.xfStack)
	return n > 0 && !w.xfStack[n-1].axis
}

// xfClipStencilBegin opens a stencil of a clip rect when the open
// transform brackets turn, skew or add perspective. Then the scissor is
// only the bounding box of the clip, and a stencil of the same rect goes
// through the transform, so it cuts exactly. It reports whether it
// opened; the caller closes it with xfClipStencilEnd. The cap at 255 is
// the one the clipContents bracket in renderLayoutDepth explains.
func xfClipStencilBegin(r drawClip, w *Window) (RenderCmd, bool) {
	if !xfNeedsStencil(w) || w.stencilDepth == 255 {
		return RenderCmd{}, false
	}
	w.stencilDepth++
	stencil := RenderCmd{
		Kind: RenderStencilBegin,
		X:    r.X, Y: r.Y, W: r.Width, H: r.Height,
		StencilDepth: w.stencilDepth,
	}
	emitRenderer(stencil, w)
	return stencil, true
}

// xfClipStencilEnd closes a stencil xfClipStencilBegin opened, with the
// same quad, so it takes back what the Begin added.
func xfClipStencilEnd(stencil RenderCmd, w *Window) {
	stencil.Kind = RenderStencilEnd
	stencil.StencilDepth = w.stencilDepth
	emitRenderer(stencil, w)
	w.stencilDepth--
}

// toWindow maps a rect in the frame's own coordinates to the window: the
// bounding box of its four mapped corners, cut to the scissor that was in
// effect when the bracket opened. A corner at or behind the viewer has no
// place on screen, so the rect falls back to that entry scissor.
func (f *xfFrame) toWindow(r drawClip) drawClip {
	box, ok := f.box(r)
	if !ok {
		return f.clip
	}
	out, ok := rectIntersection(box, f.clip)
	if !ok {
		// An empty scissor at the entry corner: it clips everything.
		return drawClip{X: f.clip.X, Y: f.clip.Y}
	}
	return out
}

// box maps a rect in the frame's own coordinates to the window: the
// bounding box of its four mapped corners, not cut to any scissor. ok is
// false when a corner is at or behind the viewer.
func (f *xfFrame) box(r drawClip) (drawClip, bool) {
	return mapRectBox(r, func(x, y float32) (float32, float32, bool) {
		return f.acc.apply(x, y)
	})
}

// renderBoxToWindow maps a rect drawn at this point of the render walk
// out to the window, through the open transform brackets. With none open,
// or with a corner off screen, it returns r. The IME caret rect goes
// through it: the platform places its candidate window in window
// coordinates.
func renderBoxToWindow(r drawClip, w *Window) drawClip {
	n := len(w.xfStack)
	if n == 0 {
		return r
	}
	if box, ok := w.xfStack[n-1].box(r); ok {
		return box
	}
	return r
}

// isInert reports whether s is inert (ContainerCfg.inert): it and its
// subtree take no pointer, no tab stop and no accessibility node.
func (s *Shape) isInert() bool {
	return s != nil && s.xf != nil && s.xf.inert
}
