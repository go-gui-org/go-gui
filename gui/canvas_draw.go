package gui

import "math"

// DrawCanvasCache holds retained tessellation output keyed by
// widget id + version + scale. Cache hit skips OnDraw entirely.
type DrawCanvasCache struct {
	Batches    []DrawCanvasTriBatch
	Texts      []DrawCanvasTextEntry
	Images     []DrawCanvasImageEntry
	Version    uint64
	TessWidth  float32
	TessHeight float32
	Scale      float32
}

// DrawCanvasTextEntry stores a deferred text drawing command.
type DrawCanvasTextEntry struct {
	Style TextStyle
	Text  string
	X, Y  float32
}

// DrawCanvasTriBatch is one flat-color triangle batch.
type DrawCanvasTriBatch struct {
	Triangles []float32
	Color     Color
}

// DrawCanvasImageEntry stores a deferred image drawing command.
// Src matches the forms accepted by ImageCfg.Src: local path,
// http/https URL, or data URL.
//
// BgOpacity modulates BgColor's alpha; it has no effect when
// BgColor is unset.
//
// Fetcher, when non-nil, overrides WindowCfg.ImageFetcher for this
// entry's http/https download. Typical use is map-tile rendering
// where each tile source wants its own User-Agent. Known limit:
// downloads are URL-keyed and deduped process-wide, so the first
// entry observed for a given URL binds the fetcher for that URL's
// in-flight download. Consumers wiring two fetchers to overlapping
// URL namespaces must route via URL prefix themselves.
type DrawCanvasImageEntry struct {
	Fetcher    ImageFetcher
	Src        string
	BgOpacity  Opt[float32]
	X, Y, W, H float32
	BgColor    Color
}

// DrawRecorder receives high-level draw commands before
// tessellation. Attach via DrawContext.SetRecorder to capture
// structured primitives (e.g. for SVG export).
type DrawRecorder interface {
	Line(x0, y0, x1, y1 float32, color Color, width float32)
	Polyline(points []float32, color Color, width float32)
	FilledRect(x, y, w, h float32, color Color)
	Rect(x, y, w, h float32, color Color, width float32)
	FilledCircle(cx, cy, radius float32, color Color)
	Circle(cx, cy, radius float32, color Color, width float32)
	FilledArc(cx, cy, rx, ry, start, sweep float32, color Color)
	Arc(cx, cy, rx, ry, start, sweep float32, color Color, width float32)
	FilledPolygon(points []float32, color Color)
	FilledRoundedRect(x, y, w, h, radius float32, color Color)
	RoundedRect(x, y, w, h, radius float32, color Color, width float32)
	DashedLine(x0, y0, x1, y1 float32, color Color, width, dashLen, gapLen float32)
	DashedPolyline(points []float32, color Color, width, dashLen, gapLen float32)
	PolylineJoined(points []float32, color Color, width float32)
	QuadBezier(x0, y0, cx, cy, x1, y1 float32, color Color, width float32)
	CubicBezier(x0, y0, c1x, c1y, c2x, c2y, x1, y1 float32, color Color, width float32)
	Text(x, y float32, text string, style TextStyle)
}

// DrawContext is passed to the OnDraw callback. Drawing methods
// append tessellated triangle batches which are later emitted as
// RenderSvg commands. Text methods append deferred text entries
// emitted as RenderText commands.
type DrawContext struct {
	textMeasure     TextMeasurer
	recorder        DrawRecorder
	batches         []DrawCanvasTriBatch
	texts           []DrawCanvasTextEntry
	images          []DrawCanvasImageEntry
	arcBuf          []float32
	bezierBuf       []float32
	currentBatchIdx int
	Width           float32
	Height          float32
	// Scale is the device pixel ratio (e.g. 2.0 on Retina/HiDPI displays).
	// Width and Height are in logical pixels; multiply by Scale to get device pixels.
	// Always > 0; defaults to 1.0 when the backend has not reported a valid scale.
	Scale float32

	lastColor Color
}

// SetRecorder attaches a DrawRecorder that receives high-level
// draw commands in addition to normal tessellation.
func (dc *DrawContext) SetRecorder(r DrawRecorder) { dc.recorder = r }

func (dc *DrawContext) getBatch(color Color) *DrawCanvasTriBatch {
	if len(dc.batches) > 0 && dc.lastColor == color {
		return &dc.batches[dc.currentBatchIdx]
	}
	dc.batches = append(dc.batches, DrawCanvasTriBatch{
		Color:     color,
		Triangles: make([]float32, 0, 128),
	})
	dc.lastColor = color
	dc.currentBatchIdx = len(dc.batches) - 1
	return &dc.batches[dc.currentBatchIdx]
}

// FilledRect draws a filled rectangle as two triangles.
func (dc *DrawContext) FilledRect(x, y, w, h float32, color Color) {
	if w <= 0 || h <= 0 {
		return
	}
	if dc.recorder != nil {
		dc.recorder.FilledRect(x, y, w, h, color)
		return
	}
	b := dc.getBatch(color)
	b.Triangles = append(b.Triangles,
		x, y,
		x+w, y,
		x+w, y+h,
		x, y,
		x+w, y+h,
		x, y+h,
	)
}

// Line draws a single line segment.
func (dc *DrawContext) Line(x0, y0, x1, y1 float32, color Color, width float32) {
	if dc.recorder != nil {
		dc.recorder.Line(x0, y0, x1, y1, color, width)
		return
	}
	dc.Polyline([]float32{x0, y0, x1, y1}, color, width)
}

// Polyline draws a stroked open polyline using simple
// per-segment rectangle expansion (no joins/caps).
func (dc *DrawContext) Polyline(points []float32, color Color, width float32) {
	if len(points) < 4 || width <= 0 {
		return
	}
	if dc.recorder != nil {
		dc.recorder.Polyline(points, color, width)
		return
	}
	hw := width / 2
	b := dc.getBatch(color)
	for i := 0; i+3 < len(points); i += 2 {
		x0, y0 := points[i], points[i+1]
		x1, y1 := points[i+2], points[i+3]
		dx := x1 - x0
		dy := y1 - y0
		ln := float32(math.Sqrt(float64(dx*dx + dy*dy)))
		if ln < 1e-6 {
			continue
		}
		// Perpendicular offset.
		nx := -dy / ln * hw
		ny := dx / ln * hw
		// Quad as two triangles.
		b.Triangles = append(b.Triangles,
			x0+nx, y0+ny,
			x0-nx, y0-ny,
			x1-nx, y1-ny,
			x0+nx, y0+ny,
			x1-nx, y1-ny,
			x1+nx, y1+ny,
		)
	}
}

// Rect draws a stroked rectangle using four axis-aligned quads
// with overlap at corners. Overlap may cause alpha artifacts
// with transparent colors.
func (dc *DrawContext) Rect(x, y, w, h float32, color Color, width float32) {
	if w <= 0 || h <= 0 || width <= 0 {
		return
	}
	if dc.recorder != nil {
		dc.recorder.Rect(x, y, w, h, color, width)
		return
	}
	hw := width / 2
	b := dc.getBatch(color)
	// Top.
	b.Triangles = append(b.Triangles,
		x-hw, y-hw, x+w+hw, y-hw, x+w+hw, y+hw,
		x-hw, y-hw, x+w+hw, y+hw, x-hw, y+hw,
	)
	// Bottom.
	b.Triangles = append(b.Triangles,
		x-hw, y+h-hw, x+w+hw, y+h-hw, x+w+hw, y+h+hw,
		x-hw, y+h-hw, x+w+hw, y+h+hw, x-hw, y+h+hw,
	)
	// Left.
	b.Triangles = append(b.Triangles,
		x-hw, y-hw, x+hw, y-hw, x+hw, y+h+hw,
		x-hw, y-hw, x+hw, y+h+hw, x-hw, y+h+hw,
	)
	// Right.
	b.Triangles = append(b.Triangles,
		x+w-hw, y-hw, x+w+hw, y-hw, x+w+hw, y+h+hw,
		x+w-hw, y-hw, x+w+hw, y+h+hw, x+w-hw, y+h+hw,
	)
}

// FilledPolygon draws a filled convex polygon using fan from
// first vertex.
func (dc *DrawContext) FilledPolygon(points []float32, color Color) {
	if len(points) < 6 {
		return
	}
	if dc.recorder != nil {
		dc.recorder.FilledPolygon(points, color)
		return
	}
	n := len(points) / 2
	b := dc.getBatch(color)
	x0, y0 := points[0], points[1]
	for i := 1; i < n-1; i++ {
		b.Triangles = append(b.Triangles,
			x0, y0,
			points[i*2], points[i*2+1],
			points[(i+1)*2], points[(i+1)*2+1],
		)
	}
}

// FilledCircle draws a filled circle.
func (dc *DrawContext) FilledCircle(cx, cy, radius float32, color Color) {
	if dc.recorder != nil {
		dc.recorder.FilledCircle(cx, cy, radius, color)
		return
	}
	dc.FilledArc(cx, cy, radius, radius, 0, 2*math.Pi, color)
}

// Circle draws a stroked circle.
func (dc *DrawContext) Circle(cx, cy, radius float32, color Color, width float32) {
	if dc.recorder != nil {
		dc.recorder.Circle(cx, cy, radius, color, width)
		return
	}
	dc.Arc(cx, cy, radius, radius, 0, 2*math.Pi, color, width)
}

// Arc draws a stroked elliptical arc.
func (dc *DrawContext) Arc(cx, cy, rx, ry, start, sweep float32, color Color, width float32) {
	if width <= 0 {
		return
	}
	if dc.recorder != nil {
		dc.recorder.Arc(cx, cy, rx, ry, start, sweep, color, width)
		return
	}
	pts := dc.arcPoints(cx, cy, rx, ry, start, sweep)
	if len(pts) >= 4 {
		dc.Polyline(pts, color, width)
	}
}

// FilledArc draws a filled elliptical arc (pie slice).
// Emits fan triangles directly from center to arc points,
// avoiding an intermediate polygon allocation.
func (dc *DrawContext) FilledArc(cx, cy, rx, ry, start, sweep float32, color Color) {
	if dc.recorder != nil {
		dc.recorder.FilledArc(cx, cy, rx, ry, start, sweep, color)
		return
	}
	pts := dc.arcPoints(cx, cy, rx, ry, start, sweep)
	if len(pts) < 4 {
		return
	}
	b := dc.getBatch(color)
	for i := 0; i+3 < len(pts); i += 2 {
		b.Triangles = append(b.Triangles,
			cx, cy,
			pts[i], pts[i+1],
			pts[i+2], pts[i+3],
		)
	}
}

// arcPoints is the buffer-reusing version of arcToPolyline.
// Writes into dc.arcBuf and returns the populated slice.
func (dc *DrawContext) arcPoints(cx, cy, rx, ry, start, sweep float32) []float32 {
	r := rx
	r = max(r, ry)
	if r <= 0 {
		return nil
	}
	n := int(math.Ceil(
		float64(f32Abs(sweep)) / (2 * math.Pi) * 64 *
			math.Sqrt(float64(r)/50+1)))
	n = max(n, 4)
	need := (n + 1) * 2
	if cap(dc.arcBuf) < need {
		dc.arcBuf = make([]float32, 0, need)
	}
	dc.arcBuf = dc.arcBuf[:0]
	step := sweep / float32(n)
	for i := 0; i <= n; i++ {
		a := float64(start + step*float32(i))
		dc.arcBuf = append(dc.arcBuf,
			cx+rx*float32(math.Cos(a)),
			cy+ry*float32(math.Sin(a)))
	}
	return dc.arcBuf
}

// FilledRoundedRect draws a filled rectangle with rounded corners.
// Radius is clamped to half the smaller dimension.
func (dc *DrawContext) FilledRoundedRect(x, y, w, h, radius float32, color Color) {
	if w <= 0 || h <= 0 {
		return
	}
	if dc.recorder != nil {
		dc.recorder.FilledRoundedRect(x, y, w, h, radius, color)
		return
	}
	radius = min(radius, w/2, h/2)
	if radius <= 0 {
		dc.FilledRect(x, y, w, h, color)
		return
	}
	b := dc.getBatch(color)
	r := radius

	// Center cross (vertical strip).
	appendQuad(b, x+r, y, x+w-r, y, x+w-r, y+h, x+r, y+h)
	// Left strip.
	appendQuad(b, x, y+r, x+r, y+r, x+r, y+h-r, x, y+h-r)
	// Right strip.
	appendQuad(b, x+w-r, y+r, x+w, y+r, x+w, y+h-r, x+w-r, y+h-r)

	// Corner arcs (filled fans).
	const segs = 8
	appendCornerFan(b, x+r, y+r, r, math.Pi, segs)       // TL
	appendCornerFan(b, x+w-r, y+r, r, 3*math.Pi/2, segs) // TR
	appendCornerFan(b, x+w-r, y+h-r, r, 0, segs)         // BR
	appendCornerFan(b, x+r, y+h-r, r, math.Pi/2, segs)   // BL
}

// appendQuad appends two triangles forming a quad.
func appendQuad(b *DrawCanvasTriBatch,
	x0, y0, x1, y1, x2, y2, x3, y3 float32) {
	b.Triangles = append(b.Triangles,
		x0, y0, x1, y1, x2, y2,
		x0, y0, x2, y2, x3, y3,
	)
}

// appendCornerFan appends a 90-degree filled arc fan.
func appendCornerFan(b *DrawCanvasTriBatch,
	cx, cy, r, startAngle float32, segs int) {
	step := float32(math.Pi/2) / float32(segs)
	for i := range segs {
		a0 := float64(startAngle + step*float32(i))
		a1 := float64(startAngle + step*float32(i+1))
		b.Triangles = append(b.Triangles,
			cx, cy,
			cx+r*float32(math.Cos(a0)), cy+r*float32(math.Sin(a0)),
			cx+r*float32(math.Cos(a1)), cy+r*float32(math.Sin(a1)),
		)
	}
}

// RoundedRect draws a stroked rectangle with rounded corners.
func (dc *DrawContext) RoundedRect(x, y, w, h, radius float32, color Color, width float32) {
	if w <= 0 || h <= 0 || width <= 0 {
		return
	}
	if dc.recorder != nil {
		dc.recorder.RoundedRect(x, y, w, h, radius, color, width)
		return
	}
	radius = min(radius, w/2, h/2)
	if radius <= 0 {
		dc.Rect(x, y, w, h, color, width)
		return
	}
	r := radius
	// Build polyline: top → TR arc → right → BR arc → bottom →
	// BL arc → left → TL arc → close.
	const segs = 8
	pts := make([]float32, 0, (4*segs+4+1)*2)
	// Top-left corner arc.
	pts = appendArcPoints(pts, x+r, y+r, r, math.Pi, segs)
	// Top-right corner arc.
	pts = appendArcPoints(pts, x+w-r, y+r, r, 3*math.Pi/2, segs)
	// Bottom-right corner arc.
	pts = appendArcPoints(pts, x+w-r, y+h-r, r, 0, segs)
	// Bottom-left corner arc.
	pts = appendArcPoints(pts, x+r, y+h-r, r, math.Pi/2, segs)
	// Close the shape.
	pts = append(pts, pts[0], pts[1])
	dc.Polyline(pts, color, width)
}

// appendArcPoints appends points for a 90-degree arc.
func appendArcPoints(pts []float32,
	cx, cy, r, startAngle float32, segs int) []float32 {
	step := float32(math.Pi/2) / float32(segs)
	for i := range segs + 1 {
		a := float64(startAngle + step*float32(i))
		pts = append(pts,
			cx+r*float32(math.Cos(a)),
			cy+r*float32(math.Sin(a)))
	}
	return pts
}

// DashedLine draws a dashed line segment. dashLen and gapLen
// control the pattern. Zero or negative values fall back to
// solid.
func (dc *DrawContext) DashedLine(
	x0, y0, x1, y1 float32,
	color Color, width, dashLen, gapLen float32,
) {
	if dashLen <= 0 || gapLen <= 0 {
		dc.Line(x0, y0, x1, y1, color, width)
		return
	}
	if dc.recorder != nil {
		dc.recorder.DashedLine(x0, y0, x1, y1, color, width, dashLen, gapLen)
		return
	}
	dx := x1 - x0
	dy := y1 - y0
	totalLen := float32(math.Sqrt(float64(dx*dx + dy*dy)))
	if totalLen < 1e-6 {
		return
	}
	ux := dx / totalLen
	uy := dy / totalLen
	patternLen := dashLen + gapLen
	drawn := float32(0)
	for drawn < totalLen {
		end := drawn + dashLen
		if end > totalLen {
			end = totalLen
		}
		dc.Line(
			x0+ux*drawn, y0+uy*drawn,
			x0+ux*end, y0+uy*end,
			color, width,
		)
		drawn += patternLen
	}
}

// DashedPolyline draws a polyline with a dash pattern applied
// continuously across all segments.
func (dc *DrawContext) DashedPolyline(
	points []float32,
	color Color, width, dashLen, gapLen float32,
) {
	if len(points) < 4 {
		return
	}
	if dashLen <= 0 || gapLen <= 0 {
		dc.Polyline(points, color, width)
		return
	}
	if dc.recorder != nil {
		dc.recorder.DashedPolyline(points, color, width, dashLen, gapLen)
		return
	}
	patternLen := dashLen + gapLen
	offset := float32(0) // position within pattern
	for i := 0; i+3 < len(points); i += 2 {
		x0, y0 := points[i], points[i+1]
		x1, y1 := points[i+2], points[i+3]
		dx := x1 - x0
		dy := y1 - y0
		segLen := float32(math.Sqrt(float64(dx*dx + dy*dy)))
		if segLen < 1e-6 {
			continue
		}
		ux := dx / segLen
		uy := dy / segLen
		pos := float32(0)
		for pos < segLen {
			inPattern := float32(math.Mod(float64(offset+pos),
				float64(patternLen)))
			if inPattern < dashLen {
				// In dash portion.
				remain := dashLen - inPattern
				end := pos + remain
				if end > segLen {
					end = segLen
				}
				dc.Line(
					x0+ux*pos, y0+uy*pos,
					x0+ux*end, y0+uy*end,
					color, width,
				)
				pos += remain
			} else {
				// In gap portion.
				pos += patternLen - inPattern
			}
		}
		offset += segLen
	}
}

// PolylineJoined draws a stroked polyline with miter joins
// at vertices. Falls back to bevel when the miter exceeds
// 4× the half-width.
func (dc *DrawContext) PolylineJoined(
	points []float32, color Color, width float32,
) {
	n := len(points) / 2
	if n < 2 || width <= 0 {
		return
	}
	if dc.recorder != nil {
		dc.recorder.PolylineJoined(points, color, width)
		return
	}
	hw := width / 2
	const miterLimit = 4.0
	b := dc.getBatch(color)

	// Compute perpendicular normals per segment.
	type vec struct{ x, y float32 }
	normals := make([]vec, 0, n-1)
	for i := 0; i < n-1; i++ {
		dx := points[(i+1)*2] - points[i*2]
		dy := points[(i+1)*2+1] - points[i*2+1]
		ln := float32(math.Sqrt(float64(dx*dx + dy*dy)))
		if ln < 1e-6 {
			normals = append(normals, vec{0, 0})
			continue
		}
		normals = append(normals, vec{-dy / ln, dx / ln})
	}

	// Compute offset points (left/right) at each vertex.
	type offsetPt struct{ lx, ly, rx, ry float32 }
	offsets := make([]offsetPt, n)

	// First vertex: use first segment normal.
	offsets[0] = offsetPt{
		lx: points[0] + normals[0].x*hw,
		ly: points[1] + normals[0].y*hw,
		rx: points[0] - normals[0].x*hw,
		ry: points[1] - normals[0].y*hw,
	}
	// Last vertex: use last segment normal.
	last := n - 1
	li := len(normals) - 1
	offsets[last] = offsetPt{
		lx: points[last*2] + normals[li].x*hw,
		ly: points[last*2+1] + normals[li].y*hw,
		rx: points[last*2] - normals[li].x*hw,
		ry: points[last*2+1] - normals[li].y*hw,
	}

	// Interior vertices: miter join.
	for i := 1; i < last; i++ {
		n0 := normals[i-1]
		n1 := normals[i]
		if (n0 == vec{0, 0}) || (n1 == vec{0, 0}) {
			// Degenerate segment, use whichever is valid.
			nv := n0
			if nv == (vec{0, 0}) {
				nv = n1
			}
			offsets[i] = offsetPt{
				lx: points[i*2] + nv.x*hw,
				ly: points[i*2+1] + nv.y*hw,
				rx: points[i*2] - nv.x*hw,
				ry: points[i*2+1] - nv.y*hw,
			}
			continue
		}
		// Average normal direction.
		mx := n0.x + n1.x
		my := n0.y + n1.y
		ml := float32(math.Sqrt(float64(mx*mx + my*my)))
		if ml < 1e-6 {
			// Nearly opposite normals — bevel.
			offsets[i] = offsetPt{
				lx: points[i*2] + n1.x*hw,
				ly: points[i*2+1] + n1.y*hw,
				rx: points[i*2] - n1.x*hw,
				ry: points[i*2+1] - n1.y*hw,
			}
			continue
		}
		mx /= ml
		my /= ml
		// Miter length = hw / dot(miter, normal).
		dot := mx*n0.x + my*n0.y
		if f32Abs(dot) < 1e-6 {
			dot = 1e-6
		}
		miterLen := hw / dot
		if f32Abs(miterLen) > hw*miterLimit {
			// Exceeds miter limit — bevel.
			miterLen = hw
			mx = n1.x
			my = n1.y
		}
		offsets[i] = offsetPt{
			lx: points[i*2] + mx*miterLen,
			ly: points[i*2+1] + my*miterLen,
			rx: points[i*2] - mx*miterLen,
			ry: points[i*2+1] - my*miterLen,
		}
	}

	// Emit quads between consecutive offset pairs.
	for i := 0; i < n-1; i++ {
		a := offsets[i]
		c := offsets[i+1]
		b.Triangles = append(b.Triangles,
			a.lx, a.ly, a.rx, a.ry, c.rx, c.ry,
			a.lx, a.ly, c.rx, c.ry, c.lx, c.ly,
		)
	}
}

const (
	bezierTol      = float32(0.5)    // pixel tolerance
	bezierMaxDepth = 16              // max subdivision depth
	bezierDegenTol = float32(0.0001) // near-degenerate threshold
)

// hasNaNInf returns true if any value is NaN or ±Inf.
func hasNaNInf(vals ...float32) bool {
	for _, v := range vals {
		if v != v || v-v != 0 {
			return true
		}
	}
	return false
}

func (dc *DrawContext) resetBezierBuf(x0, y0 float32) []float32 {
	if cap(dc.bezierBuf) < 64 {
		dc.bezierBuf = make([]float32, 0, 64)
	}
	return append(dc.bezierBuf[:0], x0, y0)
}

func flattenQuadBezier(
	buf *[]float32,
	x0, y0, cx, cy, x1, y1, tol float32, depth int,
) {
	mx := (x0 + x1) / 2
	my := (y0 + y1) / 2
	dx := cx - mx
	dy := cy - my
	d := float32(math.Sqrt(float64(dx*dx + dy*dy)))

	if d <= tol || depth >= bezierMaxDepth {
		*buf = append(*buf, x1, y1)
		return
	}
	ax := (x0 + cx) / 2
	ay := (y0 + cy) / 2
	bx := (cx + x1) / 2
	by := (cy + y1) / 2
	abx := (ax + bx) / 2
	aby := (ay + by) / 2
	flattenQuadBezier(buf, x0, y0, ax, ay, abx, aby, tol, depth+1)
	flattenQuadBezier(buf, abx, aby, bx, by, x1, y1, tol, depth+1)
}

func flattenCubicBezier(
	buf *[]float32,
	x0, y0, c1x, c1y, c2x, c2y, x1, y1, tol float32, depth int,
) {
	dx := x1 - x0
	dy := y1 - y0
	d := float32(math.Sqrt(float64(dx*dx + dy*dy)))

	if d < bezierDegenTol {
		*buf = append(*buf, x1, y1)
		return
	}

	d1 := f32Abs((c1x-x0)*dy-(c1y-y0)*dx) / d
	d2 := f32Abs((c2x-x0)*dy-(c2y-y0)*dx) / d

	if d1+d2 <= tol || depth >= bezierMaxDepth {
		*buf = append(*buf, x1, y1)
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
	flattenCubicBezier(buf, x0, y0, ax, ay, abx, aby, midx, midy,
		tol, depth+1)
	flattenCubicBezier(buf, midx, midy, bex, bey, ex, ey, x1, y1,
		tol, depth+1)
}

// QuadBezier draws a stroked quadratic Bezier curve defined by
// start (x0,y0), control (cx,cy) and end (x1,y1).
func (dc *DrawContext) QuadBezier(
	x0, y0, cx, cy, x1, y1 float32, color Color, width float32,
) {
	if width <= 0 || hasNaNInf(x0, y0, cx, cy, x1, y1, width) {
		return
	}
	if dc.recorder != nil {
		dc.recorder.QuadBezier(x0, y0, cx, cy, x1, y1, color, width)
		return
	}
	dc.bezierBuf = dc.resetBezierBuf(x0, y0)
	flattenQuadBezier(&dc.bezierBuf, x0, y0, cx, cy, x1, y1,
		bezierTol, 0)
	if len(dc.bezierBuf) >= 4 {
		dc.Polyline(dc.bezierBuf, color, width)
	}
}

// CubicBezier draws a stroked cubic Bezier curve defined by
// start (x0,y0), controls (c1x,c1y) (c2x,c2y) and end (x1,y1).
func (dc *DrawContext) CubicBezier(
	x0, y0, c1x, c1y, c2x, c2y, x1, y1 float32,
	color Color, width float32,
) {
	if width <= 0 ||
		hasNaNInf(x0, y0, c1x, c1y, c2x, c2y, x1, y1, width) {
		return
	}
	if dc.recorder != nil {
		dc.recorder.CubicBezier(x0, y0, c1x, c1y, c2x, c2y, x1, y1,
			color, width)
		return
	}
	dc.bezierBuf = dc.resetBezierBuf(x0, y0)
	flattenCubicBezier(&dc.bezierBuf, x0, y0, c1x, c1y, c2x, c2y,
		x1, y1, bezierTol, 0)
	if len(dc.bezierBuf) >= 4 {
		dc.Polyline(dc.bezierBuf, color, width)
	}
}

// Text draws text at the given position using the specified style.
// The position is the top-left of the text bounding box.
func (dc *DrawContext) Text(x, y float32, text string, style TextStyle) {
	if dc.recorder != nil {
		dc.recorder.Text(x, y, text, style)
		return
	}
	dc.texts = append(dc.texts, DrawCanvasTextEntry{
		X: x, Y: y, Text: text, Style: style,
	})
}

// TextWidth returns the measured width of text in the given style.
// Returns 0 when no text measurer is available (e.g. in tests).
func (dc *DrawContext) TextWidth(text string, style TextStyle) float32 {
	if dc.textMeasure == nil {
		return 0
	}
	return dc.textMeasure.TextWidth(text, style)
}

// FontHeight returns the line height for the given text style.
// Falls back to Style.Size when no measurer is available.
func (dc *DrawContext) FontHeight(style TextStyle) float32 {
	if dc.textMeasure == nil {
		return style.Size
	}
	return dc.textMeasure.FontHeight(style)
}

// Image draws an image at the given rectangle. Src accepts the
// same forms as ImageCfg.Src (local path, http/https URL, data URL).
// BgOpacity modulates the BgColor alpha; it does not fade the image
// texture itself. BgColor paints behind the image (useful for PNGs
// with transparency); zero value is transparent. Remote fetches use
// WindowCfg.ImageFetcher; use ImageWithFetcher to override per call.
func (dc *DrawContext) Image(
	x, y, w, h float32, src string,
	bgOpacity Opt[float32], bgColor Color,
) {
	dc.ImageWithFetcher(x, y, w, h, src, bgOpacity, bgColor, nil)
}

// ImageWithFetcher is Image plus a per-call override of the remote
// fetcher. nil fetcher falls back to WindowCfg.ImageFetcher, matching
// Image's behavior exactly. Typical use is map rendering where each
// tile layer's Source supplies its own policy-compliant User-Agent.
// See DrawCanvasImageEntry.Fetcher on the URL-keyed dedup limit.
func (dc *DrawContext) ImageWithFetcher(
	x, y, w, h float32, src string,
	bgOpacity Opt[float32], bgColor Color,
	fetcher ImageFetcher,
) {
	if w <= 0 || h <= 0 || src == "" {
		return
	}
	if !isFiniteF(x) || !isFiniteF(y) ||
		!isFiniteF(w) || !isFiniteF(h) {
		return
	}
	if dc.recorder != nil {
		if ir, ok := dc.recorder.(interface {
			Image(x, y, w, h float32, src string,
				bgOpacity Opt[float32], bgColor Color)
		}); ok {
			ir.Image(x, y, w, h, src, bgOpacity, bgColor)
			return
		}
	}
	dc.images = append(dc.images, DrawCanvasImageEntry{
		X: x, Y: y, W: w, H: h,
		Src: src, BgOpacity: bgOpacity, BgColor: bgColor,
		Fetcher: fetcher,
	})
}

// isFiniteF reports whether v is a finite float32 (not NaN or Inf).
func isFiniteF(v float32) bool {
	f := float64(v)
	return !math.IsNaN(f) && !math.IsInf(f, 0)
}

// Texts returns accumulated text entries. Useful for testing
// DrawCanvas output.
func (dc *DrawContext) Texts() []DrawCanvasTextEntry {
	return dc.texts
}

// Images returns accumulated image entries. Useful for testing
// DrawCanvas output.
func (dc *DrawContext) Images() []DrawCanvasImageEntry {
	return dc.images
}

// Batches returns accumulated triangle batches. Useful for
// testing DrawCanvas output.
func (dc *DrawContext) Batches() []DrawCanvasTriBatch {
	return dc.batches
}

// NewDrawContext creates a DrawContext for headless rendering.
// tm may be nil when text measurement is not required.
func NewDrawContext(w, h float32, tm TextMeasurer) *DrawContext {
	return &DrawContext{Width: w, Height: h, textMeasure: tm}
}
