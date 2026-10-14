package gui

import "math"

// rotateMouseInverse transforms screen-space mouse coordinates
// into the unrotated internal space of a rotated container.
// Returns the original coordinates for restoration.
func rotateMouseInverse(s *Shape, e *Event) (origX, origY float32) {
	origX, origY = e.MouseX, e.MouseY
	if s == nil || s.QuarterTurns == 0 {
		return
	}
	e.MouseX, e.MouseY = rotateCoordsInverse(s, origX, origY)
	return
}

// rotateCoordsInverse transforms screen-space coordinates into the
// unrotated internal space of a rotated container without requiring
// an Event allocation.
func rotateCoordsInverse(s *Shape, mx, my float32) (float32, float32) {
	if s == nil || s.QuarterTurns == 0 {
		return mx, my
	}
	cx := s.X + s.Width/2
	cy := s.Y + s.Height/2
	dx, dy := mx-cx, my-cy
	switch s.QuarterTurns {
	case 1: // inverse of 90° CW = 90° CCW
		return cx + dy, cy - dx
	case 2:
		return cx - dx, cy - dy
	case 3: // inverse of 270° CW = 270° CCW
		return cx - dy, cy + dx
	}
	return mx, my
}

// xfCoordsInverse maps a point in the parent's frame into the shape's
// own frame, through its view transform (#1019). The shape's own fill is
// drawn through the transform too, so the result is what the shape
// itself is hit tested with, not only its children. A point that is on
// no part of the view comes back as NaN, which every hit test misses:
// the point is outside the clip of the view's ancestors (xf.outer), the
// back face is toward the viewer and hidden, the view is inert, the
// transform flattens the view to a line, or under perspective the
// point's ray meets the view's plane behind the viewer.
func xfCoordsInverse(s *Shape, x, y float32) (float32, float32) {
	if s == nil || s.xf == nil {
		return x, y
	}
	xf := s.xf
	// An inert view (ContainerCfg.inert) takes no pointer, resolved or
	// not, so it is checked before the resolved test.
	if xf.inert {
		return xfNowhere, xfNowhere
	}
	if !xf.resolved {
		return x, y
	}
	if xf.hidden || !xf.invOK || !xfInOuter(xf, x, y) {
		return xfNowhere, xfNowhere
	}
	ix, iy, ok := xf.inv.apply(x, y)
	if !ok {
		return xfNowhere, xfNowhere
	}
	return ix, iy
}

// xfNowhere is the coordinate of a point that is on no part of a view.
// NaN fails every comparison, so PointInShape is false for it.
var xfNowhere = float32(math.NaN())

// xfMouseInverse is xfCoordsInverse for the pointer of an event. It
// returns the original coordinates for restoration.
func xfMouseInverse(s *Shape, e *Event) (origX, origY float32) {
	origX, origY = e.MouseX, e.MouseY
	e.MouseX, e.MouseY = xfCoordsInverse(s, origX, origY)
	return
}

// xfMouseFrame maps the pointer of e into the own frame of layout's
// shape, so the shape and its children are hit tested where they are
// drawn. It reports the saved coordinates and ok true; the caller keeps
// them with defer restoreMouse. With no transform on the shape it maps
// nothing and reports ok false.
func xfMouseFrame(layout *Layout, e *Event) (px, py float32, ok bool) {
	s := layout.Shape
	if s == nil || s.xf == nil {
		return 0, 0, false
	}
	px, py = xfMouseInverse(s, e)
	return px, py, true
}

// lockMouseMap maps the pointer of a locked mouse event into the frame
// of the view the lock started in (see Window.MouseLock). It returns the
// original coordinates for restoration. A point whose ray meets the
// view's plane behind the viewer has no place there, so the last mapped
// point stands in: a drag must not see NaN.
func (w *Window) lockMouseMap(e *Event) (origX, origY float32) {
	origX, origY = e.MouseX, e.MouseY
	vs := &w.viewState
	vs.lockRawX, vs.lockRawY = origX, origY
	if !vs.mouseLockXfOn {
		return
	}
	if x, y, ok := vs.mouseLockXf.apply(origX, origY); ok {
		vs.mouseLockLastX, vs.mouseLockLastY = x, y
	} else if !f32IsFinite(vs.mouseLockLastX) {
		// No point has mapped yet: leave the event as it is.
		return
	}
	e.MouseX, e.MouseY = vs.mouseLockLastX, vs.mouseLockLastY
	return
}

// callLocked runs a locked mouse callback with the pointer in the lock's
// frame (lockMouseMap), and marks the call so a MouseLock from inside it
// keeps that frame.
func (w *Window) callLocked(fn func(EventCtx), ctx EventCtx) {
	e := ctx.Event
	ox, oy := w.lockMouseMap(e)
	w.viewState.lockCallback = true
	defer func() {
		w.viewState.lockCallback = false
		restoreMouse(e, ox, oy)
	}()
	fn(ctx)
}

// restoreMouse puts back the pointer xfMouseInverse saved.
func restoreMouse(e *Event, x, y float32) {
	e.MouseX, e.MouseY = x, y
}

// xfLocalToWindow maps a point in ly's own frame out to the window,
// through the quarter turns and view transforms of ly's ancestors and
// ly's own view transform. ok is false when the point is not on screen:
// a hidden back face, a point at or behind the viewer, or a point
// outside the clip of a transformed view's ancestors (xf.outer). A float
// is drawn outside its parent's transform, so the walk stops at one.
//
// Per shape, the view transform bracket is the outer one and the
// quarter turn the inner one (renderLayoutDepth), and a quarter turn
// moves only the children. So a point from a child is turned first,
// then transformed.
func xfLocalToWindow(ly *Layout, x, y float32) (float32, float32, bool) {
	return xfLocalToWindowClip(ly, x, y, true)
}

// xfLocalToWindowClip is xfLocalToWindow. With clip false it skips the
// test against xf.outer: the point is mapped even where an ancestor
// cuts it off, for a bounds or a frame change rather than a hit.
func xfLocalToWindowClip(ly *Layout, x, y float32, clip bool) (float32, float32, bool) {
	depth := 0
	for p := ly; p != nil && !overMaxDepth(depth); p = p.Parent {
		s := p.Shape
		if s == nil {
			break
		}
		if p != ly && s.QuarterTurns > 0 {
			x, y = rotateCoordsForward(s, x, y)
		}
		// An inert view (ContainerCfg.inert) is drawn but takes no
		// hit, so a hit test (clip set) refuses it, resolved or not.
		// A frame mapping (clip false) still maps through it.
		if clip && s.isInert() {
			return 0, 0, false
		}
		if xf := s.xf; xf != nil && xf.resolved {
			if xf.hidden {
				return 0, 0, false
			}
			var ok bool
			if x, y, ok = xf.m.apply(x, y); !ok {
				return 0, 0, false
			}
			if clip && !xfInOuter(xf, x, y) {
				return 0, 0, false
			}
		}
		if s.Float {
			break
		}
		depth++
	}
	return x, y, true
}

// xfInOuter reports whether (x, y), in the parent's frame, is inside the
// clip of the view's ancestors.
func xfInOuter(xf *shapeTransform, x, y float32) bool {
	o := xf.outer
	return x >= o.X && y >= o.Y && x < o.X+o.Width && y < o.Y+o.Height
}

// xfWindowToLocal maps a window point into ly's own frame: the inverse of
// xfLocalToWindowClip, through the view transforms and quarter turns
// above ly and ly's own view transform. ok is false when the point is on
// no part of a view on the way: a hidden back face, a flattened view, a
// ray that meets the plane behind the viewer, or with clip set, a point
// outside a view's xf.outer or an inert view. Code that tests a window
// point against a shape (blur on press, the soft keyboard, tooltips)
// maps with it, so it agrees with the pointer walks.
func xfWindowToLocal(ly *Layout, x, y float32, clip bool) (float32, float32, bool) {
	return xfWindowToLocalDepth(ly, ly, x, y, clip, 0)
}

func xfWindowToLocalDepth(ly, p *Layout, x, y float32, clip bool, depth int) (float32, float32, bool) {
	s := p.Shape
	if s == nil {
		return x, y, true
	}
	// Top down: the outermost frame first. A float is drawn outside its
	// parent's transform, so the path starts at one.
	if !s.Float && p.Parent != nil && !overMaxDepth(depth) {
		var ok bool
		if x, y, ok = xfWindowToLocalDepth(ly, p.Parent, x, y, clip, depth+1); !ok {
			return 0, 0, false
		}
	}
	// An inert view takes no hit: refused for a hit test (clip set), as
	// in xfLocalToWindowClip.
	if clip && s.isInert() {
		return 0, 0, false
	}
	if xf := s.xf; xf != nil && xf.resolved {
		if xf.hidden || !xf.invOK || (clip && !xfInOuter(xf, x, y)) {
			return 0, 0, false
		}
		var ok bool
		if x, y, ok = xf.inv.apply(x, y); !ok {
			return 0, 0, false
		}
	}
	if p != ly && s.QuarterTurns > 0 {
		x, y = rotateCoordsInverse(s, x, y)
	}
	return x, y, true
}

// xfBoxToWindow maps a rect in ly's own frame out to the window: the
// box of its four corners. With no view transform on the path it returns
// r. A corner that is not on screen leaves r as it is.
func xfBoxToWindow(ly *Layout, r drawClip) drawClip {
	if box, ok := xfBoxToWindowOK(ly, r); ok {
		return box
	}
	return r
}

// xfBoxToWindowOK is xfBoxToWindow, but it reports a corner that has no
// place on screen (a hidden back face, a ray behind the viewer) with ok
// false instead of falling back to r.
func xfBoxToWindowOK(ly *Layout, r drawClip) (drawClip, bool) {
	return mapRectBox(r, func(x, y float32) (float32, float32, bool) {
		return xfLocalToWindowClip(ly, x, y, false)
	})
}

// layoutHitsWindowPoint reports whether a window point hits ly where it
// is drawn.
func layoutHitsWindowPoint(ly *Layout, x, y float32) bool {
	lx, lyy, ok := xfWindowToLocal(ly, x, y, true)
	return ok && ly.Shape.PointInShape(lx, lyy)
}

// rotateCoordsForward is the inverse of rotateCoordsInverse: it maps a
// point in a quarter turned container's unrotated frame to the screen.
func rotateCoordsForward(s *Shape, x, y float32) (float32, float32) {
	cx := s.X + s.Width/2
	cy := s.Y + s.Height/2
	dx, dy := x-cx, y-cy
	switch s.QuarterTurns {
	case 1: // 90° CW
		return cx - dy, cy + dx
	case 2:
		return cx - dx, cy - dy
	case 3: // 270° CW
		return cx + dy, cy - dx
	}
	return x, y
}

// quarterTurnForward returns rotateCoordsForward for s as a matrix, for
// the render walk's frame stack.
func quarterTurnForward(s *Shape) xfMat {
	return quarterTurnMat(s, rotateCoordsForward)
}

// quarterTurnInverse returns rotateCoordsInverse for s as a matrix, for
// the mouse lock's press path.
func quarterTurnInverse(s *Shape) xfMat {
	return quarterTurnMat(s, rotateCoordsInverse)
}

// quarterTurnMat returns the turn map f of s as a matrix. The map is
// affine, so the images of three points fix it.
func quarterTurnMat(s *Shape, f func(*Shape, float32, float32) (float32, float32)) xfMat {
	ox, oy := f(s, 0, 0)
	ax, ay := f(s, 1, 0)
	bx, by := f(s, 0, 1)
	return xfMat{ax - ox, bx - ox, ox, ay - oy, by - oy, oy, 0, 0, 1}
}

// shapeHitPoint returns a window point that hits ly where it is drawn:
// the center of its clip, mapped out through the transforms above it.
// When that point is cut off by the clip of a transformed view's
// ancestors, a grid of points across the clip is tried, so a widget that
// is only partly visible is still found. ok is false when no point
// hits.
func shapeHitPoint(ly *Layout) (float32, float32, bool) {
	c := ly.Shape.shapeClip
	if c.Width <= 0 || c.Height <= 0 {
		return 0, 0, false
	}
	if x, y, ok := xfLocalToWindow(ly, c.X+c.Width/2, c.Y+c.Height/2); ok {
		return x, y, true
	}
	const n = 5
	for i := range n {
		for j := range n {
			lx := c.X + c.Width*(float32(i)+0.5)/n
			lyy := c.Y + c.Height*(float32(j)+0.5)/n
			if x, y, ok := xfLocalToWindow(ly, lx, lyy); ok {
				return x, y, true
			}
		}
	}
	return 0, 0, false
}

// layoutInert reports whether ly or one of its ancestors is inert
// (ContainerCfg.inert), so it takes no pointer. A float is drawn
// outside its parent's subtree, so the walk stops at one, as in
// xfLocalToWindowClip.
func layoutInert(ly *Layout) bool {
	depth := 0
	for p := ly; p != nil && !overMaxDepth(depth); p = p.Parent {
		if p.Shape.isInert() {
			return true
		}
		if p.Shape == nil || p.Shape.Float {
			break
		}
		depth++
	}
	return false
}
