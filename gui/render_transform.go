package gui

import (
	"log"
	"math"
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
}

// xfFrame is one open transform bracket in the render walk.
type xfFrame struct {
	acc  xfMat    // own frame -> window: every open bracket, composed
	clip drawClip // the window scissor in effect when the bracket opened
	axis bool     // acc only moves and scales: a scissor stays exact
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

// renderTransformBegin resolves the shape's transform against its box,
// stores the matrix for input, and opens a TransformBegin bracket. It
// reports whether the bracket opened. It does not open when the back
// face is toward the viewer and hidden (xf.hidden): the caller then draws
// nothing of the subtree. When it did open, the caller closes it
// with renderTransformEnd after the subtree, and draws the subtree with
// xfUnboundedClip.
//
// clip is the clip the parent passed, in the parent's frame. It is the
// scissor in effect here, so it becomes the frame's entry scissor.
func renderTransformBegin(shape *Shape, clip drawClip, w *Window) bool {
	xf := shape.xf
	t := xf.current(shape, w)
	m, hidden := t.resolve(shape.X, shape.Y, shape.Width, shape.Height)
	xf.m, xf.hidden = m, hidden
	xf.inv, xf.invOK = m.inverse()
	if hidden || t.IsIdentity() {
		return false
	}
	if !emitRendererIfValid(RenderCmd{
		Kind:       RenderTransformBegin,
		Homography: (*[9]float32)(&xf.m),
		// The box, in the view's own frame. A backend with no
		// perspective (web) fits its flat stand-in to the view here.
		X: shape.X, Y: shape.Y, W: shape.Width, H: shape.Height,
	}, w) {
		// Nothing was drawn through this matrix, so input must not map
		// the pointer through it either.
		xf.m, xf.inv, xf.invOK, xf.hidden = xfIdentity, xfIdentity, true, false
		return false
	}
	parent := xfIdentity
	entry := clip
	if n := len(w.xfStack); n > 0 {
		top := &w.xfStack[n-1]
		parent = top.acc
		entry = top.toWindow(clip)
	}
	acc := parent.mul(&xf.m)
	w.xfStack = append(w.xfStack, xfFrame{acc: acc, clip: entry, axis: acc.axisAligned()})
	return true
}

// renderTransformEnd closes the bracket renderTransformBegin opened.
func renderTransformEnd(w *Window) {
	emitRenderer(RenderCmd{Kind: RenderTransformEnd}, w)
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
	minX, minY := float32(math.Inf(1)), float32(math.Inf(1))
	maxX, maxY := float32(math.Inf(-1)), float32(math.Inf(-1))
	for i := range 4 {
		cx, cy := r.X, r.Y
		if i&1 != 0 {
			cx += r.Width
		}
		if i&2 != 0 {
			cy += r.Height
		}
		px, py, ok := f.acc.apply(cx, cy)
		if !ok {
			return f.clip
		}
		minX, maxX = min(minX, px), max(maxX, px)
		minY, maxY = min(minY, py), max(maxY, py)
	}
	out, ok := rectIntersection(drawClip{
		X: minX, Y: minY, Width: maxX - minX, Height: maxY - minY,
	}, f.clip)
	if !ok {
		// An empty scissor at the entry corner: it clips everything.
		return drawClip{X: f.clip.X, Y: f.clip.Y}
	}
	return out
}
