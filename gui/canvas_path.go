package gui

// canvas_path.go — the CanvasPath builder and fill rule (#906).
//
// A CanvasPath is a reusable path value: move, line, quad, cubic,
// arc and close segments recorded once and filled or stroked every
// frame. The qcpainterbench Flower port builds its outline once per
// box this way; the Lines port rebuilds its area path every pass
// through Reset, reusing the buffers.
//
// A zero CanvasPath draws nothing. A copy shares its backing arrays
// with the original: Reset before reusing a copy, or the two alias.
// Methods store coordinates as given and never flatten: flattening
// happens at draw time, in local space, at the curve tolerance.

// FillRule selects how overlapping subpaths of a path fill. The zero
// value is FillNonzero, matching SVG's default.
// exportaudit:keep — named by consumer path fills through FillPath signatures
type FillRule uint8

// Fill rule values.
const (
	FillNonzero FillRule = iota
	// FillEvenOdd carves overlaps with even winding.
	// exportaudit:keep — a member of an exported enum
	FillEvenOdd
)

// canvasPathCmd identifies one recorded segment.
type canvasPathCmd uint8

// Canvas path segment commands.
const (
	pathCmdMoveTo canvasPathCmd = iota
	pathCmdLineTo
	pathCmdQuadTo
	pathCmdCubicTo
	pathCmdArcTo
	pathCmdClose
)

// CanvasPath is a reusable sequence of path segments, built with
// MoveTo, LineTo, QuadTo, CubicTo, ArcTo and Close, and drawn with
// FillPath, FillPathGradient and StrokePath.
//
// A repeated MoveTo opens a new contour. Close ends the current
// contour back at its MoveTo point; a fill closes an unclosed
// trailing contour implicitly, while a stroke leaves it open with
// end caps. A LineTo, QuadTo or CubicTo with no contour yet starts
// one at (0,0), matching QPainterPath and HTML canvas.
type CanvasPath struct {
	cmds []canvasPathCmd
	pts  []float32
	curX float32
	curY float32
	// start is the MoveTo point of the open contour, and hasContour
	// reports whether one is open. A Close with no open contour is
	// a no-op.
	startX     float32
	startY     float32
	hasContour bool
}

// MoveTo opens a new contour at (x,y). Any open contour is left as
// it is: a fill closes it implicitly.
func (p *CanvasPath) MoveTo(x, y float32) {
	p.cmds = append(p.cmds, pathCmdMoveTo)
	p.pts = append(p.pts, x, y)
	p.curX, p.curY = x, y
	p.startX, p.startY = x, y
	p.hasContour = true
}

// LineTo appends a straight segment from the current point to
// (x,y).
func (p *CanvasPath) LineTo(x, y float32) {
	p.cmds = append(p.cmds, pathCmdLineTo)
	p.pts = append(p.pts, x, y)
	p.curX, p.curY = x, y
	p.hasContour = true
}

// QuadTo appends a quadratic Bezier from the current point through
// control (cx,cy) to (x,y).
// exportaudit:keep — curve segment for consumer paths
func (p *CanvasPath) QuadTo(cx, cy, x, y float32) {
	p.cmds = append(p.cmds, pathCmdQuadTo)
	p.pts = append(p.pts, cx, cy, x, y)
	p.curX, p.curY = x, y
	p.hasContour = true
}

// CubicTo appends a cubic Bezier from the current point through
// controls (c1x,c1y) (c2x,c2y) to (x,y).
func (p *CanvasPath) CubicTo(c1x, c1y, c2x, c2y, x, y float32) {
	p.cmds = append(p.cmds, pathCmdCubicTo)
	p.pts = append(p.pts, c1x, c1y, c2x, c2y, x, y)
	p.curX, p.curY = x, y
	p.hasContour = true
}

// ArcTo appends an elliptical arc centered at (cx,cy) with radii
// (rx,ry), from angle start through sweep radians. Angles run in
// the canvas's y-down space, so a positive sweep turns clockwise —
// the same convention as DrawContext.Arc. The arc is connected to
// the current point with a straight segment, as QPainterPath.arcTo
// connects.
// exportaudit:keep — arc segment for consumer paths
func (p *CanvasPath) ArcTo(cx, cy, rx, ry, start, sweep float32) {
	p.cmds = append(p.cmds, pathCmdArcTo)
	p.pts = append(p.pts, cx, cy, rx, ry, start, sweep)
	// The end point is the arc's far end, so a following segment
	// continues from it. Non-finite radii leave the current point
	// where it is; the draw drops the path before flattening reads
	// this.
	if rx > 0 && ry > 0 {
		p.curX = cx + rx*f32Cos(start+sweep)
		p.curY = cy + ry*f32Sin(start+sweep)
	}
	p.hasContour = true
}

// Close ends the current contour back at its MoveTo point. With no
// open contour it does nothing.
func (p *CanvasPath) Close() {
	if !p.hasContour {
		return
	}
	p.cmds = append(p.cmds, pathCmdClose)
	p.curX, p.curY = p.startX, p.startY
	p.hasContour = false
}

// Reset clears the path for reuse, keeping its buffers: a path
// rebuilt every frame stops allocating after the first one.
func (p *CanvasPath) Reset() {
	p.cmds = p.cmds[:0]
	p.pts = p.pts[:0]
	p.curX, p.curY = 0, 0
	p.startX, p.startY = 0, 0
	p.hasContour = false
}

// IsEmpty reports whether the path holds no segments. A path that
// only moved draws nothing either; it still counts as non-empty,
// because it names a contour start a Close may end.
// exportaudit:keep — guard for consumers that reuse paths
func (p *CanvasPath) IsEmpty() bool {
	return len(p.cmds) == 0
}

// bakedPath returns a copy of p with every point mapped through xf.
// Recorders (SVG/PDF export) see baked coordinates: the recorder
// interface is exported and implemented outside this repo, so local
// coordinates plus a matrix would misplace every existing
// implementer's output. Mapping control points maps the curves
// exactly — an affine takes a Bezier to the Bezier of its mapped
// points — while an arc maps like DrawContext.Arc: radii scale per
// axis and angles ride unchanged, exact until rotation shears the
// ellipse.
func (p *CanvasPath) bakedPath(xf canvasXform) *CanvasPath {
	out := &CanvasPath{
		cmds: make([]canvasPathCmd, len(p.cmds)),
		pts:  make([]float32, len(p.pts)),
	}
	copy(out.cmds, p.cmds)
	off := 0
	for _, c := range p.cmds {
		switch c {
		case pathCmdMoveTo, pathCmdLineTo:
			x, y := xf.apply(p.pts[off], p.pts[off+1])
			out.pts[off], out.pts[off+1] = x, y
			off += 2
		case pathCmdQuadTo:
			for i := 0; i < 6; i += 2 {
				x, y := xf.apply(p.pts[off+i], p.pts[off+i+1])
				out.pts[off+i], out.pts[off+i+1] = x, y
			}
			off += 4
		case pathCmdCubicTo:
			for i := 0; i < 8; i += 2 {
				x, y := xf.apply(p.pts[off+i], p.pts[off+i+1])
				out.pts[off+i], out.pts[off+i+1] = x, y
			}
			off += 6
		case pathCmdArcTo:
			cx, cy := xf.apply(p.pts[off], p.pts[off+1])
			out.pts[off], out.pts[off+1] = cx, cy
			out.pts[off+2] = p.pts[off+2] * xf.colScaleX()
			out.pts[off+3] = p.pts[off+3] * xf.colScaleY()
			out.pts[off+4], out.pts[off+5] =
				p.pts[off+4], p.pts[off+5]
			off += 6
		case pathCmdClose:
		}
	}
	out.curX, out.curY = xf.apply(p.curX, p.curY)
	out.startX, out.startY = xf.apply(p.startX, p.startY)
	out.hasContour = p.hasContour
	return out
}
