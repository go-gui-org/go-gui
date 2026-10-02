package gui

import "math"

// canvas_stroke.go — line caps and joins for canvas strokes (#905).
//
// Polyline draws disjoint quads and PolylineJoined draws miter joins
// with a bevel fallback. Both keep that output bit for bit. The
// *Styled methods in this file add what those two cannot: round and
// square caps, round joins, and pure bevel joins.
//
// A Styled call with the zero style draws exactly what the unstyled
// call draws, through the same code path: each method checks for the
// zero style first and delegates. New rendering only happens for a
// non-zero style, so existing callers see no change in pixels, in
// triangle counts, or in what a recorder receives.
//
// The tessellation core (AppendStrokeTris and its helpers) is shared
// with gui/svg, which imports gui and so cannot own it. svg keeps its
// own SvgStrokeCap/SvgStrokeJoin domain types and converts at the
// boundary.

// StrokeCap selects how an open stroked path ends at its two ends.
// The zero value is StrokeButtCap, which ends flush at the end point
// — the same ends Polyline has always drawn.
type StrokeCap uint8

// StrokeCap constants.
const (
	StrokeButtCap StrokeCap = iota
	StrokeRoundCap
	StrokeSquareCap
)

// StrokeJoin selects how two stroked segments meet at a vertex.
// The zero value is StrokeMiterJoin, which extends both edges to a
// sharp point — the same joints PolylineJoined has always drawn,
// including its fallback to a bevel past the miter limit.
type StrokeJoin uint8

// StrokeJoin constants.
const (
	StrokeMiterJoin StrokeJoin = iota
	StrokeRoundJoin
	StrokeBevelJoin
)

// StrokeStyle is the cap and join a Styled stroke draws with. The
// zero value is the default: butt caps and miter joins, which draws
// exactly what the matching unstyled call draws. Closed paths (a
// full-sweep Circle, a RoundedRect outline) ignore the cap: they have
// no ends, only joints.
type StrokeStyle struct {
	Cap  StrokeCap
	Join StrokeJoin
}

// Stroke tessellation constants. The miter limit is fixed at 4x the
// half width in this version: both stroke paths this core replaces
// already used that limit, and no caller has asked for a knob.
const (
	strokeCrossTolerance = float32(0.001)
	strokeMiterLimit     = float32(4.0)
	strokeRoundCapSegs   = 8
	strokeClosedEpsilon  = float32(0.0001)
)

// AppendStrokeTris appends the stroke triangles for one polyline to
// dst and returns the scratch to keep for the next call.
//
// points holds x/y pairs. halfW is half the stroke width. A path
// whose first and last points meet is closed: it gets joins all the
// way around and no caps. scratch is reused workspace for the
// segment normals and may be nil, in which case it allocates.
//
// Callers screen non-finite points before calling: a join carries a
// bad vertex into its neighbours, so a bad point corrupts more than
// its own segment.
func AppendStrokeTris(dst *[]float32, points []float32, halfW float32,
	lineCap StrokeCap, join StrokeJoin, scratch []float32) []float32 {
	if len(points) < 4 || len(points)%2 != 0 {
		return scratch
	}
	// Exported, so the width is screened here too: a NaN or negative
	// half width would emit NaN or inside-out triangles.
	if !(halfW > 0) || !f32IsFinite(halfW) {
		return scratch
	}
	n := len(points) / 2

	// A closed path has no ends, so it needs joins everywhere and no
	// caps. The epsilon test matches what the path flatteners emit
	// for a full turn: the last point lands back on the first.
	dxClose := points[0] - points[(n-1)*2]
	dyClose := points[1] - points[(n-1)*2+1]
	isClosed := n > 2 && f32Abs(dxClose) < strokeClosedEpsilon &&
		f32Abs(dyClose) < strokeClosedEpsilon
	pointCount := n
	if isClosed {
		pointCount = n - 1
	}
	if pointCount < 2 {
		return scratch
	}

	// Unit normals per segment, left of the direction of travel.
	normals := scratch[:0]
	for i := 0; i < pointCount-1; i++ {
		dx := points[(i+1)*2] - points[i*2]
		dy := points[(i+1)*2+1] - points[i*2+1]
		l := float32(math.Sqrt(float64(dx*dx + dy*dy)))
		if l > 0 {
			normals = append(normals, -dy/l, dx/l)
		} else {
			normals = append(normals, 0, 1)
		}
	}
	if isClosed {
		dx := points[0] - points[(pointCount-1)*2]
		dy := points[1] - points[(pointCount-1)*2+1]
		l := float32(math.Sqrt(float64(dx*dx + dy*dy)))
		if l > 0 {
			normals = append(normals, -dy/l, dx/l)
		} else {
			normals = append(normals, 0, 1)
		}
	}

	// One quad per segment. A closed path emits the extra segment
	// that wraps the last point back to the first, or the stroke
	// leaves a visible gap there.
	segCount := pointCount - 1
	if isClosed {
		segCount = pointCount
	}
	for i := 0; i < segCount; i++ {
		i0 := i
		i1 := i + 1
		if i1 == pointCount {
			i1 = 0
		}
		x0 := points[i0*2]
		y0 := points[i0*2+1]
		x1 := points[i1*2]
		y1 := points[i1*2+1]
		nx := normals[i*2]
		ny := normals[i*2+1]

		ax := x0 + nx*halfW
		ay := y0 + ny*halfW
		bx := x0 - nx*halfW
		by := y0 - ny*halfW
		cx := x1 - nx*halfW
		cy := y1 - ny*halfW
		ex := x1 + nx*halfW
		ey := y1 + ny*halfW

		*dst = append(*dst, ax, ay, bx, by, cx, cy, ax, ay, cx, cy, ex, ey)
	}

	// Joins at interior vertices, or at every vertex for a closed path.
	numNormals := len(normals) / 2
	if isClosed {
		for i := range pointCount {
			prevNorm := i - 1
			if i == 0 {
				prevNorm = numNormals - 1
			}
			nextNorm := i
			if nextNorm < numNormals && prevNorm < numNormals {
				appendStrokeJoin(points[i*2], points[i*2+1],
					normals[prevNorm*2], normals[prevNorm*2+1],
					normals[nextNorm*2], normals[nextNorm*2+1],
					halfW, join, dst)
			}
		}
	} else {
		for i := 1; i < pointCount-1; i++ {
			if i < numNormals {
				appendStrokeJoin(points[i*2], points[i*2+1],
					normals[(i-1)*2], normals[(i-1)*2+1],
					normals[i*2], normals[i*2+1],
					halfW, join, dst)
			}
		}
	}

	// Caps at both ends of an open path. The extension runs along
	// the path: back along the first segment at the start, forward
	// along the last segment at the end. A normal is the segment
	// normal rotated a quarter turn, so (ny, -nx) recovers the travel
	// direction and its negation the outward one.
	if !isClosed && len(normals) >= 2 {
		appendStrokeCap(points[0], points[1],
			-normals[1], normals[0], normals[0], normals[1],
			halfW, lineCap, dst)
		lastIdx := (pointCount - 1) * 2
		lastNormIdx := (len(normals)/2 - 1) * 2
		appendStrokeCap(points[lastIdx], points[lastIdx+1],
			normals[lastNormIdx+1], -normals[lastNormIdx],
			-normals[lastNormIdx], -normals[lastNormIdx+1],
			halfW, lineCap, dst)
	}
	return normals
}

// appendStrokeJoin appends the fill between two segments at a vertex.
// n1 and n2 are the two segment normals. Near-collinear segments need
// no fill: their quads already meet. A miter past the limit falls
// back to a bevel, matching PolylineJoined.
func appendStrokeJoin(x, y, n1x, n1y, n2x, n2y, halfW float32,
	join StrokeJoin, dst *[]float32) {
	cross := n1x*n2y - n1y*n2x
	if f32Abs(cross) < strokeCrossTolerance {
		return
	}

	dot := n1x*n2x + n1y*n2y
	miterLen := halfW / float32(math.Sqrt(float64((1+dot)/2)))
	miterLimit := strokeMiterLimit * halfW

	mx := n1x + n2x
	my := n1y + n2y
	mlen := float32(math.Sqrt(float64(mx*mx + my*my)))
	if mlen <= 0 {
		return
	}

	mxN := mx / mlen
	myN := my / mlen

	if join == StrokeMiterJoin && miterLen <= miterLimit {
		if cross > 0 {
			*dst = append(*dst,
				x, y, x-n1x*halfW, y-n1y*halfW, x-mxN*miterLen, y-myN*miterLen,
				x, y, x-mxN*miterLen, y-myN*miterLen, x-n2x*halfW, y-n2y*halfW,
			)
		} else {
			*dst = append(*dst,
				x, y, x+mxN*miterLen, y+myN*miterLen, x+n1x*halfW, y+n1y*halfW,
				x, y, x+n2x*halfW, y+n2y*halfW, x+mxN*miterLen, y+myN*miterLen,
			)
		}
	} else if join == StrokeRoundJoin {
		appendRoundJoin(x, y, n1x, n1y, n2x, n2y, halfW, cross > 0, dst)
	} else {
		// Bevel, and a miter past the limit.
		if cross > 0 {
			*dst = append(*dst,
				x, y, x-n1x*halfW, y-n1y*halfW, x-n2x*halfW, y-n2y*halfW,
			)
		} else {
			*dst = append(*dst,
				x, y, x+n2x*halfW, y+n2y*halfW, x+n1x*halfW, y+n1y*halfW,
			)
		}
	}
}

// appendRoundJoin appends a fan that rounds the outer edge between two
// segments. leftTurn selects which side is outer.
func appendRoundJoin(x, y, n1x, n1y, n2x, n2y, halfW float32,
	leftTurn bool, dst *[]float32) {
	angle1 := float32(math.Atan2(float64(n1y), float64(n1x)))
	angle2 := float32(math.Atan2(float64(n2y), float64(n2x)))

	// The fan sweeps the short way across the outer wedge: up for a
	// left turn, down for a right turn. Sweeping the other way paints
	// the inner disc the quads already cover and leaves the outer
	// corner open.
	if leftTurn {
		if angle2 < angle1 {
			angle2 += 2 * math.Pi
		}
	} else {
		if angle2 > angle1 {
			angle2 -= 2 * math.Pi
		}
	}

	angleDiff := f32Abs(angle2 - angle1)
	segments := int(math.Ceil(float64(angleDiff)/(math.Pi/4))) + 1
	if segments < 2 {
		return
	}

	step := (angle2 - angle1) / float32(segments)
	sign := float32(1)
	if leftTurn {
		sign = -1
	}
	prevX := x + float32(math.Cos(float64(angle1)))*halfW*sign
	prevY := y + float32(math.Sin(float64(angle1)))*halfW*sign

	for i := 1; i <= segments; i++ {
		angle := angle1 + step*float32(i)
		currX := x + float32(math.Cos(float64(angle)))*halfW*sign
		currY := y + float32(math.Sin(float64(angle)))*halfW*sign

		if leftTurn {
			*dst = append(*dst, x, y, prevX, prevY, currX, currY)
		} else {
			*dst = append(*dst, x, y, currX, currY, prevX, prevY)
		}
		prevX = currX
		prevY = currY
	}
}

// appendStrokeCap appends the end geometry for one end of an open
// path. (tx, ty) is the unit outward direction — back along the path
// at the start, forward along it at the end — and (nx, ny) is the end
// segment's unit normal. A butt cap appends nothing: the segment quad
// already ends flush at the point. A square cap extends one half
// width along the outward direction; a round cap fans a half disc
// around the end point.
func appendStrokeCap(x, y, tx, ty, nx, ny, halfW float32,
	lineCap StrokeCap, dst *[]float32) {
	if lineCap == StrokeButtCap {
		return
	}

	l := float32(math.Sqrt(float64(tx*tx + ty*ty)))
	if l == 0 {
		return
	}
	dirX := tx / l
	dirY := ty / l

	switch lineCap {
	case StrokeSquareCap:
		ex := x + dirX*halfW
		ey := y + dirY*halfW
		ax := x + nx*halfW
		ay := y + ny*halfW
		bx := x - nx*halfW
		by := y - ny*halfW
		cx := ex - nx*halfW
		cy := ey - ny*halfW
		ex2 := ex + nx*halfW
		ey2 := ey + ny*halfW
		*dst = append(*dst, ax, ay, bx, by, cx, cy, ax, ay, cx, cy, ex2, ey2)
	case StrokeRoundCap:
		startAngle := float32(math.Atan2(float64(ny), float64(nx)))
		prevX := x + float32(math.Cos(float64(startAngle)))*halfW
		prevY := y + float32(math.Sin(float64(startAngle)))*halfW

		for i := 1; i <= strokeRoundCapSegs; i++ {
			angle := startAngle + math.Pi*float32(i)/float32(strokeRoundCapSegs)
			currX := x + float32(math.Cos(float64(angle)))*halfW
			currY := y + float32(math.Sin(float64(angle)))*halfW
			*dst = append(*dst, x, y, prevX, prevY, currX, currY)
			prevX = currX
			prevY = currY
		}
	}
}

// LineStyled is Line with a stroke style. The zero style draws
// exactly what Line draws.
func (dc *DrawContext) LineStyled(x0, y0, x1, y1 float32, color Color,
	width float32, style StrokeStyle) {
	if style == (StrokeStyle{}) {
		dc.Line(x0, y0, x1, y1, color, width)
		return
	}
	// Same reuse as Line: Polyline hands the slice to the recorder,
	// so a literal here would heap-allocate on every call.
	dc.lineBuf = [4]float32{x0, y0, x1, y1}
	dc.PolylineStyled(dc.lineBuf[:], color, width, style)
}

// PolylineStyled is Polyline with a stroke style. The zero style
// draws exactly what Polyline draws: disjoint quads with no joins.
// Any other style tessellates through the shared stroker, which
// joins interior vertices per the style and caps both ends.
func (dc *DrawContext) PolylineStyled(points []float32, color Color,
	width float32, style StrokeStyle) {
	if style == (StrokeStyle{}) {
		dc.Polyline(points, color, width)
		return
	}
	if len(points) < 4 || width <= 0 || !f32IsFinite(width) {
		return
	}
	if dc.recorder != nil {
		if sr, ok := dc.strokeRecorder(); ok {
			sr.PolylineStyled(points, color, width, style)
			return
		}
		// A recorder without the stroke extension still gets the
		// stroke, as the unstyled primitive. Caps and joins past
		// the default do not survive the fallback.
		dc.rec().Polyline(points, color, width)
		return
	}
	// A join carries a bad vertex into its neighbours, so the screen
	// is over the whole list rather than per segment.
	if !f32AllFinite(points) {
		return
	}
	b := dc.getBatch(color)
	dc.strokeScratch = AppendStrokeTris(&b.Triangles, points,
		width/2, style.Cap, style.Join, dc.strokeScratch)
}

// PolylineJoinedStyled is PolylineJoined with a stroke style. The
// zero style draws exactly what PolylineJoined draws. Any other
// style tessellates through the shared stroker instead of the miter
// path, which is what adds round joins and end caps.
func (dc *DrawContext) PolylineJoinedStyled(points []float32, color Color,
	width float32, style StrokeStyle) {
	if style == (StrokeStyle{}) {
		dc.PolylineJoined(points, color, width)
		return
	}
	n := len(points) / 2
	if n < 2 || width <= 0 || !f32IsFinite(width) {
		return
	}
	if !f32AllFinite(points) {
		return
	}
	if dc.recorder != nil {
		if sr, ok := dc.strokeRecorder(); ok {
			sr.PolylineJoinedStyled(points, color, width, style)
			return
		}
		dc.rec().PolylineJoined(points, color, width)
		return
	}
	b := dc.getBatch(color)
	dc.strokeScratch = AppendStrokeTris(&b.Triangles, points,
		width/2, style.Cap, style.Join, dc.strokeScratch)
}

// ArcStyled is Arc with a stroke style. The zero style draws exactly
// what Arc draws. A full-sweep arc is closed and ignores the cap.
func (dc *DrawContext) ArcStyled(cx, cy, rx, ry, start, sweep float32,
	color Color, width float32, style StrokeStyle) {
	if style == (StrokeStyle{}) {
		dc.Arc(cx, cy, rx, ry, start, sweep, color, width)
		return
	}
	if width <= 0 || !f32IsFinite(width) {
		return
	}
	if dc.recorder != nil {
		if sr, ok := dc.strokeRecorder(); ok {
			sr.ArcStyled(cx, cy, rx, ry, start, sweep, color, width, style)
			return
		}
		dc.rec().Arc(cx, cy, rx, ry, start, sweep, color, width)
		return
	}
	pts := dc.arcPointsMin(cx, cy, rx, ry, start, sweep, 4, width/2)
	if len(pts) >= 4 {
		dc.PolylineStyled(pts, color, width, style)
	}
}

// CircleStyled is Circle with a stroke style. The zero style draws
// exactly what Circle draws. A circle is closed and ignores the cap.
func (dc *DrawContext) CircleStyled(cx, cy, radius float32, color Color,
	width float32, style StrokeStyle) {
	if style == (StrokeStyle{}) {
		dc.Circle(cx, cy, radius, color, width)
		return
	}
	// A recorder gets CircleStyled, not the ArcStyled it lowers to,
	// matching Circle: an export path keeps the circle a circle.
	if dc.recorder != nil {
		if sr, ok := dc.strokeRecorder(); ok {
			sr.CircleStyled(cx, cy, radius, color, width, style)
			return
		}
		dc.rec().Circle(cx, cy, radius, color, width)
		return
	}
	dc.ArcStyled(cx, cy, radius, radius, 0, 2*math.Pi, color, width, style)
}

// RoundedRectStyled is RoundedRect with a stroke style. The zero
// style draws exactly what RoundedRect draws. The outline is closed
// and ignores the cap; the style selects its corner joins.
func (dc *DrawContext) RoundedRectStyled(x, y, w, h, radius float32,
	color Color, width float32, style StrokeStyle) {
	if style == (StrokeStyle{}) {
		dc.RoundedRect(x, y, w, h, radius, color, width)
		return
	}
	if w <= 0 || h <= 0 || width <= 0 ||
		!f32AllFinite6(x, y, w, h, radius, width) {
		return
	}
	if dc.recorder != nil {
		if sr, ok := dc.strokeRecorder(); ok {
			sr.RoundedRectStyled(x, y, w, h, radius, color, width, style)
			return
		}
		dc.rec().RoundedRect(x, y, w, h, radius, color, width)
		return
	}
	radius = min(radius, w/2, h/2)
	if radius <= 0 {
		// Rect has no Styled form in this version: its four-quad
		// build stays as it is.
		dc.Rect(x, y, w, h, color, width)
		return
	}
	dc.PolylineStyled(dc.roundedRectLoop(x, y, w, h, radius),
		color, width, style)
}

// QuadBezierStyled is QuadBezier with a stroke style. The zero style
// draws exactly what QuadBezier draws. The style applies to the
// flattened polyline: caps at the true end points, joins along it.
func (dc *DrawContext) QuadBezierStyled(x0, y0, cx, cy, x1, y1 float32,
	color Color, width float32, style StrokeStyle) {
	if style == (StrokeStyle{}) {
		dc.QuadBezier(x0, y0, cx, cy, x1, y1, color, width)
		return
	}
	if width <= 0 || !f32AllFinite7(x0, y0, cx, cy, x1, y1, width) {
		return
	}
	if dc.recorder != nil {
		if sr, ok := dc.strokeRecorder(); ok {
			sr.QuadBezierStyled(x0, y0, cx, cy, x1, y1,
				color, width, style)
			return
		}
		dc.rec().QuadBezier(x0, y0, cx, cy, x1, y1, color, width)
		return
	}
	dc.bezierBuf = dc.resetBezierBuf(x0, y0)
	flattenQuadBezier(&dc.bezierBuf, x0, y0, cx, cy, x1, y1,
		bezierTol, 0)
	if len(dc.bezierBuf) >= 4 {
		dc.PolylineStyled(dc.bezierBuf, color, width, style)
	}
}

// CubicBezierStyled is CubicBezier with a stroke style. The zero
// style draws exactly what CubicBezier draws; otherwise the rules
// are QuadBezierStyled's.
func (dc *DrawContext) CubicBezierStyled(x0, y0, c1x, c1y, c2x, c2y,
	x1, y1 float32, color Color, width float32, style StrokeStyle) {
	if style == (StrokeStyle{}) {
		dc.CubicBezier(x0, y0, c1x, c1y, c2x, c2y, x1, y1,
			color, width)
		return
	}
	if width <= 0 ||
		!f32AllFinite9(x0, y0, c1x, c1y, c2x, c2y, x1, y1, width) {
		return
	}
	if dc.recorder != nil {
		if sr, ok := dc.strokeRecorder(); ok {
			sr.CubicBezierStyled(x0, y0, c1x, c1y, c2x, c2y, x1, y1,
				color, width, style)
			return
		}
		dc.rec().CubicBezier(x0, y0, c1x, c1y, c2x, c2y, x1, y1,
			color, width)
		return
	}
	dc.bezierBuf = dc.resetBezierBuf(x0, y0)
	flattenCubicBezier(&dc.bezierBuf, x0, y0, c1x, c1y, c2x, c2y,
		x1, y1, bezierTol, 0)
	if len(dc.bezierBuf) >= 4 {
		dc.PolylineStyled(dc.bezierBuf, color, width, style)
	}
}
