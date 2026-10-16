package gui

// canvas_path_trim.go — CanvasPath.TrimTo (#1023).
//
// TrimTo copies the part of a path that lies between two fractions of
// its length. Animate the end fraction and a stroke draws itself in: a
// check mark, a progress ring along a custom outline. SVG does the same
// with stroke-dasharray and stroke-dashoffset.
//
// The length is the length the draw sees. The walk below follows
// flattenPathContours step by step: the same contour starts, the same
// connecting line before an arc, the same closing edge, curves cut at
// the same flatness test and arcs at the same chord count. A local
// scale of 1 is assumed, because a path has no transform of its own.
//
// The cut segments stay what they were. A line comes back as a shorter
// line, a curve as the exact sub-curve (its blossom), an arc as an arc
// with a smaller sweep. So a recorder (SVG or PDF export) still gets
// curves. A trim of the whole path draws the same as the path. For a
// path whose contours start with MoveTo it also holds the same
// commands, with these exceptions, none of which changes the drawing:
// a contour started by a LineTo or a curve with no MoveTo comes back
// with a MoveTo; a contour that only moved is dropped; a curve or arc
// with no length, or a zero-length LineTo at a contour's start, is
// dropped.

import "math"

// TrimTo writes into dst the part of p between the fractions from and
// to of its length. Both are in the range 0 to 1, and values out of
// range are clamped. A NaN fraction, from >= to, a path with no length
// or a path with a non-finite point give an empty dst.
//
// The length is the flattened length, at the stroke tolerance, over
// all subpaths together. A subpath cut short is left open, so a stroke
// gives it end caps. dst is reset first and keeps its buffers: a trim
// redone every frame into the same dst stops allocating after the
// first one.
//
//	mark.TrimTo(&s.trimmed, 0, progress)
//	dc.StrokePath(&s.trimmed, color, 3, gui.StrokeStyle{})
//
// dst must not be p. If it is, TrimTo does nothing.
func (p *CanvasPath) TrimTo(dst *CanvasPath, from, to float32) {
	if dst == nil || dst == p {
		return
	}
	// A struct copy of p shares its arrays: a Reset and an append into
	// dst would overwrite the segments the walk is still reading. Give
	// such a dst its own arrays.
	if sharesArrays(dst.cmds, p.cmds) || sharesArrays(dst.pts, p.pts) {
		dst.cmds, dst.pts = nil, nil
	}
	dst.Reset()
	// NaN fails every comparison, so test the good case.
	if !(from == from && to == to) || p.IsEmpty() ||
		!f32AllFinite(p.pts) {
		return
	}
	from = min(max(from, 0), 1)
	to = min(max(to, 0), 1)
	if from >= to {
		return
	}
	// First pass: the whole length. Huge but finite coordinates can
	// overflow it; such a path cannot be drawn either.
	var tr pathTrimmer
	tr.walk(p)
	total := tr.cum
	if !(total > 0) || math.IsInf(total, 0) {
		return
	}
	// Second pass: emit the pieces in [lo, hi].
	tr = pathTrimmer{
		dst:  dst,
		emit: true,
		lo:   float64(from) * total,
		hi:   float64(to) * total,
	}
	// to == 1 must reach the very end. The product can round below the
	// sum the walk makes again, which would cut the last segment short.
	if to == 1 {
		tr.hi = math.Inf(1)
	}
	tr.walk(p)
}

// sharesArrays reports whether a and b start at the same element of
// one backing array.
func sharesArrays[T any](a, b []T) bool {
	return cap(a) > 0 && cap(b) > 0 && &a[:1][0] == &b[:1][0]
}

// pathTrimmer walks a path's segments in draw order and adds up their
// lengths. With emit set, it also appends to dst each piece of a
// segment that falls in the length range [lo, hi].
type pathTrimmer struct {
	dst  *CanvasPath
	lo   float64
	hi   float64
	cum  float64 // length walked so far
	emit bool
	// pen is true when dst's current point is the end of the segment
	// just walked, in the same contour. The next piece then continues
	// from it; otherwise it starts with a MoveTo.
	pen bool
	// contourStart is cum where the open contour began.
	contourStart float64
	// storedOff is true when dst's last command is an ArcTo whose stored
	// end differs from the source arc's: a tail cut of an arc past a
	// full turn. A QuadTo or CubicTo starts from the stored end, so the
	// next curve must start with a MoveTo to the source's end.
	storedOff bool
}

// walk visits the segments of p the way flattenPathContours makes its
// contours. Keep the two in step: a difference here is a difference
// between the trimmed length and the drawn length.
func (tr *pathTrimmer) walk(p *CanvasPath) {
	// lx,ly is the end of the last segment as stored, which a curve
	// starts from. px,py is the last point the draw made, which a line,
	// a close or an arc's connecting line starts from. They differ only
	// after an arc: past a full turn by a lot, below it by rounding.
	// sx,sy is the contour start.
	var lx, ly, px, py, sx, sy float32
	hasCur := false
	off := 0
	for _, c := range p.cmds {
		// Strictly past: a Close at cum == hi still belongs to the range
		// when the closing edge has length 0.
		if tr.emit && tr.cum > tr.hi {
			return // past the range: nothing more to emit
		}
		switch c {
		case pathCmdMoveTo:
			lx, ly = p.pts[off], p.pts[off+1]
			off += 2
			px, py, sx, sy = lx, ly, lx, ly
			hasCur = true
			tr.newContour()
		case pathCmdLineTo:
			x, y := p.pts[off], p.pts[off+1]
			off += 2
			if !hasCur {
				// The draw starts a contour at the point itself, with
				// no segment to it.
				lx, ly, px, py, sx, sy = x, y, x, y, x, y
				hasCur = true
				tr.newContour()
				break
			}
			tr.line(px, py, x, y)
			lx, ly, px, py = x, y, x, y
		case pathCmdQuadTo:
			cx, cy := p.pts[off], p.pts[off+1]
			x, y := p.pts[off+2], p.pts[off+3]
			off += 4
			if !hasCur {
				sx, sy = lx, ly
				hasCur = true
				tr.newContour()
			}
			// After an arc past a full turn, the draw joins its last
			// point to this curve's first chord; that small difference
			// in length is not followed here.
			tr.quad(lx, ly, cx, cy, x, y)
			lx, ly, px, py = x, y, x, y
		case pathCmdCubicTo:
			c1x, c1y := p.pts[off], p.pts[off+1]
			c2x, c2y := p.pts[off+2], p.pts[off+3]
			x, y := p.pts[off+4], p.pts[off+5]
			off += 6
			if !hasCur {
				sx, sy = lx, ly
				hasCur = true
				tr.newContour()
			}
			tr.cubic(lx, ly, c1x, c1y, c2x, c2y, x, y)
			lx, ly, px, py = x, y, x, y
		case pathCmdArcTo:
			cx, cy := p.pts[off], p.pts[off+1]
			rx, ry := p.pts[off+2], p.pts[off+3]
			start, sweep := p.pts[off+4], p.pts[off+5]
			off += 6
			// The arc's far end, as ArcTo stored it.
			ex, ey := lx, ly
			if rx > 0 && ry > 0 {
				ex = cx + rx*f32Cos(start+sweep)
				ey = cy + ry*f32Sin(start+sweep)
			}
			// arcPointsMin draws nothing for these; the points are
			// already known to be finite.
			if max(rx, ry) <= 0 {
				lx, ly = ex, ey
				break
			}
			ax, ay := arcPoint(cx, cy, rx, ry, float64(start))
			connect := false
			if !hasCur {
				sx, sy = ax, ay
				hasCur = true
				tr.newContour()
			} else {
				connect = ax != px || ay != py
			}
			px, py = tr.arc(px, py, connect, cx, cy, rx, ry, start, sweep)
			lx, ly = ex, ey
		case pathCmdClose:
			// The draw tests lx,ly against the start but draws the edge
			// from its last point, and leaves lx,ly where they were.
			if hasCur {
				tr.close(lx != sx || ly != sy, px, py, sx, sy)
			}
			hasCur = false
		}
	}
}

// newContour marks the start of a contour.
func (tr *pathTrimmer) newContour() {
	tr.pen = false
	tr.contourStart = tr.cum
}

// piece adds a segment of length l to the walk. It returns the part
// [a, b] of the segment, measured along it, that lies in the range.
// a is exactly 0 when the range starts at or before the segment, and
// b exactly l when it ends at or after it, so a whole segment is known
// to be whole without a float comparison.
func (tr *pathTrimmer) piece(l float64) (a, b float64, ok bool) {
	s0 := tr.cum
	s1 := s0 + l
	tr.cum = s1
	if !tr.emit || !(l > 0) || tr.lo >= s1 || tr.hi <= s0 {
		return 0, 0, false
	}
	if tr.lo > s0 {
		a = min(tr.lo-s0, l)
	}
	b = l
	if tr.hi < s1 {
		b = max(tr.hi-s0, 0)
	}
	return a, b, b > a
}

// begin moves dst to (x,y) unless the piece continues from dst's
// current point.
func (tr *pathTrimmer) begin(a float64, x, y float32) {
	if a > 0 || !tr.pen {
		tr.dst.MoveTo(x, y)
	}
}

// line walks a straight segment.
func (tr *pathTrimmer) line(x0, y0, x1, y1 float32) {
	l := math.Hypot(float64(x1)-float64(x0), float64(y1)-float64(y0))
	if l == 0 {
		// The draw skips a zero-length line. Keep it inside the range
		// anyway, so a trim of the whole path keeps the command.
		if tr.emit && tr.pen && tr.lo <= tr.cum && tr.cum <= tr.hi {
			tr.dst.LineTo(x1, y1)
			tr.storedOff = false
		}
		return
	}
	a, b, ok := tr.piece(l)
	if !ok {
		return
	}
	tr.emitLine(x0, y0, x1, y1, l, a, b)
}

// emitLine appends the part [a, b] of a line of length l.
func (tr *pathTrimmer) emitLine(x0, y0, x1, y1 float32, l, a, b float64) {
	if a == 0 {
		tr.begin(0, x0, y0)
	} else {
		tr.begin(a, lerp32(x0, x1, a/l), lerp32(y0, y1, a/l))
	}
	if b == l {
		tr.dst.LineTo(x1, y1)
	} else {
		tr.dst.LineTo(lerp32(x0, x1, b/l), lerp32(y0, y1, b/l))
	}
	tr.pen = b == l
	tr.storedOff = false
}

// close walks the closing edge of a contour from its last point
// (lx,ly) back to its start (sx,sy). A contour inside the range from
// end to end comes back closed; one cut short comes back open, with
// the closing edge as a plain line.
// edge says whether the draw adds the closing edge at all; it starts
// at (lx,ly), the last point drawn.
func (tr *pathTrimmer) close(edge bool, lx, ly, sx, sy float32) {
	l := 0.0
	if edge {
		l = math.Hypot(float64(sx)-float64(lx), float64(sy)-float64(ly))
	}
	whole := tr.emit && tr.pen && tr.lo <= tr.contourStart &&
		tr.hi >= tr.cum+l && tr.cum+l > tr.contourStart
	if whole {
		tr.cum += l
		tr.dst.Close()
	} else if a, b, ok := tr.piece(l); ok {
		tr.emitLine(lx, ly, sx, sy, l, a, b)
	}
	tr.pen = false
}

// curveStart breaks the pen before a curve that follows a tail cut of
// an arc past a full turn (see storedOff): the curve then gets a
// MoveTo to its source start instead of the arc's stored end.
func (tr *pathTrimmer) curveStart() {
	if tr.storedOff {
		tr.pen = false
		tr.storedOff = false
	}
}

// quad walks a quadratic Bezier.
func (tr *pathTrimmer) quad(x0, y0, cx, cy, x1, y1 float32) {
	q := curveScan{}
	q.quad(x0, y0, cx, cy, x1, y1, 0, 1, 0)
	l := q.acc
	tr.curveStart()
	a, b, ok := tr.piece(l)
	if !ok {
		return
	}
	if a == 0 && b == l {
		tr.begin(0, x0, y0)
		tr.dst.QuadTo(cx, cy, x1, y1)
		tr.pen = true
		return
	}
	scan := func(s *curveScan) {
		s.quad(x0, y0, cx, cy, x1, y1, 0, 1, 0)
	}
	ta := curveT(l, a, scan)
	tb := curveT(l, b, scan)
	ctl := [6]float64{float64(x0), float64(y0), float64(cx), float64(cy),
		float64(x1), float64(y1)}
	sx, sy := quadBlossom(&ctl, ta, ta)
	if a == 0 {
		sx, sy = float64(x0), float64(y0)
	}
	mx, my := quadBlossom(&ctl, ta, tb)
	ex, ey := quadBlossom(&ctl, tb, tb)
	if b == l {
		ex, ey = float64(x1), float64(y1)
	}
	tr.begin(a, float32(sx), float32(sy))
	tr.dst.QuadTo(float32(mx), float32(my), float32(ex), float32(ey))
	tr.pen = b == l
}

// cubic walks a cubic Bezier.
func (tr *pathTrimmer) cubic(x0, y0, c1x, c1y, c2x, c2y, x1, y1 float32) {
	q := curveScan{}
	q.cubic(x0, y0, c1x, c1y, c2x, c2y, x1, y1, 0, 1, 0)
	l := q.acc
	tr.curveStart()
	a, b, ok := tr.piece(l)
	if !ok {
		return
	}
	if a == 0 && b == l {
		tr.begin(0, x0, y0)
		tr.dst.CubicTo(c1x, c1y, c2x, c2y, x1, y1)
		tr.pen = true
		return
	}
	scan := func(s *curveScan) {
		s.cubic(x0, y0, c1x, c1y, c2x, c2y, x1, y1, 0, 1, 0)
	}
	ta := curveT(l, a, scan)
	tb := curveT(l, b, scan)
	ctl := [8]float64{float64(x0), float64(y0), float64(c1x), float64(c1y),
		float64(c2x), float64(c2y), float64(x1), float64(y1)}
	sx, sy := cubicBlossom(&ctl, ta, ta, ta)
	if a == 0 {
		sx, sy = float64(x0), float64(y0)
	}
	p1x, p1y := cubicBlossom(&ctl, ta, ta, tb)
	p2x, p2y := cubicBlossom(&ctl, ta, tb, tb)
	ex, ey := cubicBlossom(&ctl, tb, tb, tb)
	if b == l {
		ex, ey = float64(x1), float64(y1)
	}
	tr.begin(a, float32(sx), float32(sy))
	tr.dst.CubicTo(float32(p1x), float32(p1y), float32(p2x), float32(p2y),
		float32(ex), float32(ey))
	tr.pen = b == l
}

// arc walks an ArcTo: the line that connects (lx,ly) to the arc's
// start when connect is set, then the arc. It returns the last point
// the draw makes, the last chord sample: past a full turn that is
// start ± 2π, not the stored end.
func (tr *pathTrimmer) arc(lx, ly float32, connect bool,
	cx, cy, rx, ry, start, sweep float32) (endX, endY float32) {
	// Whatever this arc emits ends where the source arc stored its end,
	// except the tail cut below.
	tr.storedOff = false
	ax, ay := arcPoint(cx, cy, rx, ry, float64(start))
	lc := 0.0
	if connect {
		lc = math.Hypot(float64(ax)-float64(lx), float64(ay)-float64(ly))
	}
	ca, cb, cok := tr.piece(lc)
	// Measure the arc with the chords a draw at scale 1 makes.
	clamped := arcClampSweep(sweep)
	n := arcSegmentCount(rx, ry, clamped, 1, 1, 4, 0)
	step := clamped / float32(n)
	l, _, _ := arcChordScan(cx, cy, rx, ry, start, step, n, math.Inf(1))
	endX, endY = arcPoint(cx, cy, rx, ry, float64(start+step*float32(n)))
	a, b, ok := tr.piece(l)
	// A whole connecting line followed by the arc needs no LineTo: an
	// ArcTo draws that line itself from the current point. Leaving it
	// out keeps a trim of the whole path equal to the path.
	if cok {
		if ca == 0 && cb == lc && ok && a == 0 {
			tr.begin(0, lx, ly)
			tr.pen = true
		} else {
			tr.emitLine(lx, ly, ax, ay, lc, ca, cb)
		}
	}
	if !ok {
		return endX, endY
	}
	if a == 0 && b == l {
		tr.begin(0, ax, ay)
		tr.dst.ArcTo(cx, cy, rx, ry, start, sweep)
		tr.pen = true
		return endX, endY
	}
	angA := arcAngleAt(cx, cy, rx, ry, start, step, n, a)
	angB := arcAngleAt(cx, cy, rx, ry, start, step, n, b)
	if a == 0 {
		angA = start
	}
	if b == l {
		angB = start + clamped
	}
	sx, sy := arcPoint(cx, cy, rx, ry, float64(angA))
	tr.begin(a, sx, sy)
	tr.dst.ArcTo(cx, cy, rx, ry, angA, angB-angA)
	tr.pen = b == l
	// angB-angA ends at start+clamped; the source stored start+sweep.
	tr.storedOff = b == l && clamped != sweep
	return endX, endY
}

// arcPoint is the point at angle a, computed as arcPointsMin computes
// its samples.
func arcPoint(cx, cy, rx, ry float32, a float64) (x, y float32) {
	return cx + rx*float32(math.Cos(a)), cy + ry*float32(math.Sin(a))
}

// arcChordScan sums the chords of an arc sampled as arcPointsMin
// samples it. It stops at the chord where the sum reaches limit and
// returns the sum before that chord, its index i and its length. With
// limit +Inf it returns the whole length and i == n.
func arcChordScan(cx, cy, rx, ry, start, step float32, n int,
	limit float64) (acc float64, i int, chord float64) {
	px, py := arcPoint(cx, cy, rx, ry, float64(start))
	for i = 1; i <= n; i++ {
		x, y := arcPoint(cx, cy, rx, ry, float64(start+step*float32(i)))
		chord = math.Hypot(float64(x)-float64(px), float64(y)-float64(py))
		if acc+chord >= limit {
			return acc, i - 1, chord
		}
		acc += chord
		px, py = x, y
	}
	return acc, n, 0
}

// arcAngleAt is the angle at length d along an arc, linear inside the
// chord that holds it.
func arcAngleAt(cx, cy, rx, ry, start, step float32, n int,
	d float64) float32 {
	acc, i, chord := arcChordScan(cx, cy, rx, ry, start, step, n, d)
	f := 0.0
	if chord > 0 {
		f = (d - acc) / chord
	}
	return float32(float64(start) + float64(step)*(float64(i)+f))
}

// curveScan follows a curve's flattening in order, adding up chord
// lengths in acc. With find set, it stops at the chord where acc
// reaches target and stores the curve parameter there in t.
type curveScan struct {
	acc    float64
	target float64
	t      float64
	find   bool
	done   bool
}

// chord adds one flattened chord, from parameter t0 to t1.
func (s *curveScan) chord(x0, y0, x1, y1 float32, t0, t1 float64) {
	c := math.Hypot(float64(x1)-float64(x0), float64(y1)-float64(y0))
	if s.find && s.acc+c >= s.target {
		f := 0.0
		if c > 0 {
			f = (s.target - s.acc) / c
		}
		s.t = t0 + f*(t1-t0)
		s.done = true
		return
	}
	s.acc += c
}

// quad mirrors pathfill.flattenQuadRec: the same split points, the
// same flatness test, the same depth limit.
func (s *curveScan) quad(x0, y0, cx, cy, x1, y1 float32, t0, t1 float64,
	depth int) {
	if s.done {
		return
	}
	mx := (x0 + x1) / 2
	my := (y0 + y1) / 2
	dx := cx - mx
	dy := cy - my
	d := float32(math.Sqrt(float64(dx*dx + dy*dy)))
	if d <= bezierTol || depth >= bezierMaxDepth {
		s.chord(x0, y0, x1, y1, t0, t1)
		return
	}
	ax := (x0 + cx) / 2
	ay := (y0 + cy) / 2
	bx := (cx + x1) / 2
	by := (cy + y1) / 2
	abx := (ax + bx) / 2
	aby := (ay + by) / 2
	tm := (t0 + t1) / 2
	s.quad(x0, y0, ax, ay, abx, aby, t0, tm, depth+1)
	s.quad(abx, aby, bx, by, x1, y1, tm, t1, depth+1)
}

// cubic mirrors pathfill.flattenCubicRec.
func (s *curveScan) cubic(x0, y0, c1x, c1y, c2x, c2y, x1, y1 float32,
	t0, t1 float64, depth int) {
	if s.done {
		return
	}
	dx := x1 - x0
	dy := y1 - y0
	d := float32(math.Sqrt(float64(dx*dx + dy*dy)))
	if d < bezierDegenTol {
		s.chord(x0, y0, x1, y1, t0, t1)
		return
	}
	d1 := f32Abs((c1x-x0)*dy-(c1y-y0)*dx) / d
	d2 := f32Abs((c2x-x0)*dy-(c2y-y0)*dx) / d
	if d1+d2 <= bezierTol || depth >= bezierMaxDepth {
		s.chord(x0, y0, x1, y1, t0, t1)
		return
	}
	ax := (x0 + c1x) / 2
	ay := (y0 + c1y) / 2
	bx := (c1x + c2x) / 2
	by := (c1y + c2y) / 2
	ex := (c2x + x1) / 2
	ey := (c2y + y1) / 2
	abx := (ax + bx) / 2
	aby := (ay + by) / 2
	bex := (bx + ex) / 2
	bey := (by + ey) / 2
	midx := (abx + bex) / 2
	midy := (aby + bey) / 2
	tm := (t0 + t1) / 2
	s.cubic(x0, y0, ax, ay, abx, aby, midx, midy, t0, tm, depth+1)
	s.cubic(midx, midy, bex, bey, ex, ey, x1, y1, tm, t1, depth+1)
}

// curveT is the curve parameter at length d along a curve of length
// l, found by rescanning the curve's flattening.
func curveT(l, d float64, scan func(*curveScan)) float64 {
	if d <= 0 {
		return 0
	}
	if d >= l {
		return 1
	}
	s := curveScan{find: true, target: d}
	scan(&s)
	if !s.done {
		return 1
	}
	return s.t
}

// quadBlossom evaluates the blossom (polar form) of a quadratic at
// (u,v). quadBlossom(t,t) is the point at t; the sub-curve from ta to
// tb has the control point quadBlossom(ta,tb).
func quadBlossom(c *[6]float64, u, v float64) (x, y float64) {
	ax, ay := lerp64(c[0], c[2], u), lerp64(c[1], c[3], u)
	bx, by := lerp64(c[2], c[4], u), lerp64(c[3], c[5], u)
	return lerp64(ax, bx, v), lerp64(ay, by, v)
}

// cubicBlossom evaluates the blossom of a cubic at (u,v,w). The
// sub-curve from ta to tb has the control points (ta,ta,ta),
// (ta,ta,tb), (ta,tb,tb) and (tb,tb,tb).
func cubicBlossom(c *[8]float64, u, v, w float64) (x, y float64) {
	ax, ay := lerp64(c[0], c[2], u), lerp64(c[1], c[3], u)
	bx, by := lerp64(c[2], c[4], u), lerp64(c[3], c[5], u)
	ex, ey := lerp64(c[4], c[6], u), lerp64(c[5], c[7], u)
	abx, aby := lerp64(ax, bx, v), lerp64(ay, by, v)
	bex, bey := lerp64(bx, ex, v), lerp64(by, ey, v)
	return lerp64(abx, bex, w), lerp64(aby, bey, w)
}

func lerp64(a, b, t float64) float64 { return a + (b-a)*t }

// lerp32 interpolates in float64: b-a can overflow float32 for finite
// a and b of opposite sign.
func lerp32(a, b float32, t float64) float32 {
	return float32(lerp64(float64(a), float64(b), t))
}
