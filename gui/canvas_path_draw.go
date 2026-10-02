package gui

import (
	"github.com/go-gui-org/go-gui/gui/internal/pathfill"
)

// canvas_path_draw.go — FillPath, FillPathGradient and StrokePath
// (#906).
//
// A path is flattened to contours in local space, then filled
// through the shared pathfill core (the same tessellator gui/svg
// uses) or stroked through the shared stroker. Geometry stays local:
// the batch carries the canvas transform, exactly as every other
// primitive does. Curve flattening therefore counts segments in
// local space — drawing a path scaled up leaves it as faceted as
// the unscaled one — in the same documented class as arcs
// (canvas_draw_transform.go).

// pathfillRuleFromFillRule maps a canvas fill rule onto the shared
// rule. Anything outside evenodd fills nonzero.
func pathfillRuleFromFillRule(rule FillRule) pathfill.Rule {
	if rule == FillEvenOdd {
		return pathfill.RuleEvenOdd
	}
	return pathfill.RuleNonzero
}

// flattenPathContours flattens a path to one polyline per contour,
// in local space. Contours live in the shared flat buffer and stay
// valid until the next flatten call: tessellate or stroke them
// before drawing anything else.
//
// Curves flatten at the bezier tolerance both stroked beziers
// already use, and arcs reuse the arc segment count, so a path
// draws like its primitive twins.
func (dc *DrawContext) flattenPathContours(p *CanvasPath,
	tol float32) [][]float32 {
	arena := dc.pathFlatBuf[:0]
	contours := dc.pathContourBuf[:0]
	// Curves expand: a quad can double its points many times over.
	// Grow once up front so contour headers below never dangle past
	// a realloc mid-flatten.
	if need := len(p.pts)*2 + 64; cap(arena) < need {
		arena = make([]float32, 0, need)
	}
	var cur []float32
	startIdx := 0
	lx, ly := float32(0), float32(0)
	push := func() {
		if len(cur) >= 4 {
			end := len(arena)
			contours = append(contours, arena[startIdx:end:end])
		}
		cur = nil
	}
	off := 0
	for _, c := range p.cmds {
		switch c {
		case pathCmdMoveTo:
			push()
			x, y := p.pts[off], p.pts[off+1]
			off += 2
			lx, ly = x, y
			startIdx = len(arena)
			arena = append(arena, x, y)
			cur = arena[startIdx : startIdx+2 : startIdx+2]
		case pathCmdLineTo:
			x, y := p.pts[off], p.pts[off+1]
			off += 2
			lx, ly = x, y
			if len(cur) == 0 {
				startIdx = len(arena)
				arena = append(arena, lx, ly)
				cur = arena[startIdx : startIdx+2 : startIdx+2]
				break
			}
			// Exact duplicates make zero-length edges, which the
			// filler drops but the stroker turns into degenerate
			// quads. Skip them the way the SVG flattener does.
			if x == cur[len(cur)-2] && y == cur[len(cur)-1] {
				break
			}
			arena = append(arena, x, y)
			cur = arena[startIdx:len(arena):len(arena)]
		case pathCmdQuadTo:
			cx, cy := p.pts[off], p.pts[off+1]
			x, y := p.pts[off+2], p.pts[off+3]
			off += 4
			if len(cur) == 0 {
				startIdx = len(arena)
				arena = append(arena, lx, ly)
			}
			pathfill.FlattenQuad(lx, ly, cx, cy, x, y, tol, &arena)
			lx, ly = x, y
			cur = arena[startIdx:len(arena):len(arena)]
		case pathCmdCubicTo:
			c1x, c1y := p.pts[off], p.pts[off+1]
			c2x, c2y := p.pts[off+2], p.pts[off+3]
			x, y := p.pts[off+4], p.pts[off+5]
			off += 6
			if len(cur) == 0 {
				startIdx = len(arena)
				arena = append(arena, lx, ly)
			}
			pathfill.FlattenCubic(lx, ly, c1x, c1y, c2x, c2y,
				x, y, tol, &arena)
			lx, ly = x, y
			cur = arena[startIdx:len(arena):len(arena)]
		case pathCmdArcTo:
			cx, cy := p.pts[off], p.pts[off+1]
			rx, ry := p.pts[off+2], p.pts[off+3]
			start, sweep := p.pts[off+4], p.pts[off+5]
			off += 6
			// The arc's far end, matching what ArcTo stored.
			ex, ey := lx, ly
			if rx > 0 && ry > 0 {
				ex = cx + rx*f32Cos(start+sweep)
				ey = cy + ry*f32Sin(start+sweep)
			}
			pts := dc.arcPointsMin(cx, cy, rx, ry, start,
				sweep, 4, 0)
			if len(pts) < 4 {
				lx, ly = ex, ey
				break
			}
			if len(cur) == 0 {
				startIdx = len(arena)
				arena = append(arena, pts[0], pts[1])
			} else if pts[0] != cur[len(cur)-2] ||
				pts[1] != cur[len(cur)-1] {
				// The connecting segment ArcTo promises.
				arena = append(arena, pts[0], pts[1])
			}
			arena = append(arena, pts[2:]...)
			lx, ly = ex, ey
			cur = arena[startIdx:len(arena):len(arena)]
		case pathCmdClose:
			if len(cur) >= 2 {
				sx := arena[startIdx]
				sy := arena[startIdx+1]
				if lx != sx || ly != sy {
					arena = append(arena, sx, sy)
				}
			}
			push()
		}
	}
	push()
	dc.pathFlatBuf = arena
	dc.pathContourBuf = contours
	return contours
}

// FillPath fills a path with a solid color, honoring the fill rule:
// overlapping subpaths fill under nonzero and carve under evenodd.
// A nil or empty path, or one with a non-finite coordinate, draws
// nothing — the screen is over the whole segment list because one
// bad point can poison a fan or a join, the way FilledPolygon drops
// its primitive rather than emitting a partial shape.
// exportaudit:keep — flat path fill for consumers with solid colors
func (dc *DrawContext) FillPath(p *CanvasPath, color Color,
	rule FillRule) {
	if p == nil || p.IsEmpty() || !f32AllFinite(p.pts) {
		return
	}
	if dc.recorder != nil {
		if _, ok := dc.pathRecorder(); ok {
			dc.rec().(DrawPathRecorder).FillPath(p, color, rule)
			return
		}
		// The recorder cannot express a path. Record the
		// tessellation one triangle at a time: exact, holes
		// included, since the triangles only cover filled area.
		contours := dc.flattenPathContours(p, bezierTol)
		tris := pathfill.Append(nil, contours,
			pathfillRuleFromFillRule(rule), nil)
		if len(tris) == 0 {
			return
		}
		dc.recordTriangles(dc.xfPoints(tris), nil, color)
		return
	}
	contours := dc.flattenPathContours(p, bezierTol)
	if len(contours) == 0 {
		return
	}
	b := dc.getBatch(color)
	b.Triangles = pathfill.Append(b.Triangles, contours,
		pathfillRuleFromFillRule(rule), &dc.pathScratch)
}

// FillPathGradient fills a path with a gradient, honoring the fill
// rule. Geometry matches FillPath exactly; only the coloring
// differs, the way every gradient twin matches its flat one.
func (dc *DrawContext) FillPathGradient(p *CanvasPath,
	g *CanvasGradient, rule FillRule) {
	if p == nil || p.IsEmpty() || !f32AllFinite(p.pts) {
		return
	}
	if mid, ok := dc.gradientRecorderFallback(g); ok {
		// A recorder without gradient support gets the flat twin.
		if _, ok := dc.pathRecorder(); ok {
			dc.rec().(DrawPathRecorder).FillPath(p, mid, rule)
			return
		}
		contours := dc.flattenPathContours(p, bezierTol)
		tris := pathfill.Append(nil, contours,
			pathfillRuleFromFillRule(rule), nil)
		if len(tris) == 0 {
			return
		}
		dc.recordTriangles(dc.xfPoints(tris), nil, mid)
		return
	}
	contours := dc.flattenPathContours(p, bezierTol)
	if len(contours) == 0 {
		return
	}
	dst := dc.gradScratch()
	*dst = pathfill.Append(*dst, contours,
		pathfillRuleFromFillRule(rule), &dc.pathScratch)
	// FillTrianglesGradient carries the recorder case: a gradient
	// recorder receives the tessellation plus the gradient, and a
	// plain one the flat triangles.
	dc.FillTrianglesGradient(*dst, g)
}

// StrokePath strokes a path with the shared stroker, applying the
// style's caps to open contours and its joins everywhere. A contour
// closed with Close gets joins all the way around and no caps,
// exactly as a closed polyline does. Unlike the *Styled twins,
// there is no legacy output to preserve: every style, including the
// zero one, tessellates through AppendStrokeTris.
func (dc *DrawContext) StrokePath(p *CanvasPath, color Color,
	width float32, style StrokeStyle) {
	if p == nil || p.IsEmpty() {
		return
	}
	if width <= 0 || !f32IsFinite(width) || !f32AllFinite(p.pts) {
		return
	}
	if dc.recorder != nil {
		if _, ok := dc.pathRecorder(); ok {
			dc.rec().(DrawPathRecorder).StrokePath(p, color, width,
				style)
			return
		}
		// The recorder cannot express a path. Stroke each contour
		// as a joined polyline: closed contours detect themselves
		// and join all the way around, open ones keep end caps.
		// The style survives through the stroke extension.
		contours := dc.flattenPathContours(p, bezierTol)
		for _, c := range contours {
			dc.PolylineJoinedStyled(c, color, width, style)
		}
		return
	}
	contours := dc.flattenPathContours(p, bezierTol)
	if len(contours) == 0 {
		return
	}
	b := dc.getBatch(color)
	for _, c := range contours {
		dc.strokeScratch = AppendStrokeTris(&b.Triangles, c,
			width/2, style.Cap, style.Join, dc.strokeScratch)
	}
}

// pathRecorder is gradientRecorder for DrawPathRecorder, and
// asserts against the inner recorder for the same reason: asserting
// against dc.rec() would always succeed and quietly kill the
// triangle degradation that stops an export dropping a path fill.
func (dc *DrawContext) pathRecorder() (DrawPathRecorder, bool) {
	if dc.recorder == nil {
		return nil, false
	}
	if _, ok := dc.recorder.(DrawPathRecorder); !ok {
		return nil, false
	}
	if _, ok := dc.activeXform(); !ok {
		return dc.recorder.(DrawPathRecorder), true
	}
	return dc.rec().(*xformRecorder), true
}

// FillPath is reached only through pathRecorder, so the inner
// recorder is known to implement DrawPathRecorder. The path is
// baked: control points map exactly under an affine, and arcs map
// the way Arc does (radii per axis, angles unchanged).
func (r *xformRecorder) FillPath(p *CanvasPath, color Color,
	rule FillRule) {
	r.inner.(DrawPathRecorder).FillPath(p.bakedPath(r.xf), color,
		rule)
}

// StrokePath is reached only through pathRecorder. Widths have no
// axis of their own and take the mean scale, as every other stroked
// primitive does.
func (r *xformRecorder) StrokePath(p *CanvasPath, color Color,
	width float32, style StrokeStyle) {
	r.inner.(DrawPathRecorder).StrokePath(p.bakedPath(r.xf), color,
		r.ln(width), style)
}
