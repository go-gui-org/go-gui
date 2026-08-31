package gui

import "math"

// canvas_draw_transform.go — the translate + scale stack for
// DrawContext (issue #474).
//
// The transform is the axis-aligned affine
//
//	p' = (p.x*sx + tx, p.y*sy + ty)
//
// — scale per axis plus translation, with no rotation and no shear.
// That group is closed under composition, so an arbitrarily deep
// Save/Translate/ScaleBy nest collapses to four floats. That is what
// makes the geometry path free: the four floats ride on the batch and
// then on the RenderCmd, where every backend already applies them per
// vertex, so no triangle is ever rewritten on the CPU.
//
// Geometry therefore does NOT bake. Text, images and the lowered
// radial gradient do bake, at record time, because their render
// commands have no xform fields to ride on. They are also the three
// paths with no primitive-to-primitive delegation, so baking there
// cannot double-apply.
//
// Documented limits, all consequences of the design above:
//
//   - Curve flattening (arcPoints, the bezier subdivision) picks its
//     segment count in LOCAL space, so scaling a circle up by 10x
//     leaves it as faceted as the unscaled one. Drawing it at its
//     final size gives a smoother result.
//   - A non-uniform scale positions and sizes a text run but does not
//     shear its glyphs.
//   - A negative scale moves an image's rect but never mirrors its
//     content.
//   - A concentric radial gradient under a non-uniform scale gives up
//     the single-quad shader path and falls back to the ring mesh,
//     which transforms exactly.

// maxXformDepth bounds the Save stack. A Save inside a per-frame draw
// loop with no matching Restore would otherwise grow the slice without
// limit for as long as the canvas keeps redrawing.
const maxXformDepth = 256

// canvasXform is the CTM. Its zero value is NOT the identity — the
// identity is {1, 1, 0, 0} — which is why DrawContext gates on
// xfActive rather than on the field values. A zero-value DrawContext
// literal (tests build them directly) must behave exactly as it did
// before this file existed.
type canvasXform struct {
	sx, sy, tx, ty float32
}

// identityXform is the transform a context starts every redraw with.
var identityXform = canvasXform{sx: 1, sy: 1}

// apply maps one point from local space to canvas space.
func (x canvasXform) apply(px, py float32) (float32, float32) {
	return px*x.sx + x.tx, py*x.sy + x.ty
}

// meanScale is the scale to apply to a length that has no axis of its
// own — a stroke width, a dash length, a corner radius. The geometric
// mean is exact under a uniform scale, which is the case that has a
// right answer at all.
func (x canvasXform) meanScale() float32 {
	return float32(math.Sqrt(math.Abs(float64(x.sx * x.sy))))
}

// uniform reports whether the two axes scale alike. The radial
// gradient's single-quad path and the recorder's circle methods both
// need to know, because neither can express an ellipse.
func (x canvasXform) uniform() bool { return x.sx == x.sy }

// Translate shifts the origin by (dx, dy) measured in the CURRENT
// local units, so a Translate after a ScaleBy moves by scaled units —
// the same composition order HTML canvas uses.
//
// Composition: p -> (p+d)*S + T = p*S + (T + d*S).
//
// Non-finite arguments are ignored rather than poisoning every
// subsequent vertex.
func (dc *DrawContext) Translate(dx, dy float32) {
	if !isFiniteF(dx) || !isFiniteF(dy) {
		return
	}
	dc.ensureXform()
	dc.xf.tx += dx * dc.xf.sx
	dc.xf.ty += dy * dc.xf.sy
}

// ScaleBy multiplies the current scale by (sx, sy). It is named
// ScaleBy, not Scale, because DrawContext.Scale is the device pixel
// ratio field and predates this API; the two are unrelated.
//
// Composition: p -> (p*A)*S + T = p*(S*A) + T, so the translation is
// untouched and scaling happens about the current origin.
//
// A zero scale is allowed and collapses geometry. Non-finite
// arguments are ignored.
func (dc *DrawContext) ScaleBy(sx, sy float32) {
	if !isFiniteF(sx) || !isFiniteF(sy) {
		return
	}
	dc.ensureXform()
	dc.xf.sx *= sx
	dc.xf.sy *= sy
}

// Save pushes the current transform so a later Restore can return to
// it. Pushes past maxXformDepth are dropped, which unbalances the
// stack rather than growing it without limit; the reset at the top of
// every redraw is what actually contains the damage.
func (dc *DrawContext) Save() {
	dc.ensureXform()
	if len(dc.xfStack) >= maxXformDepth {
		return
	}
	dc.xfStack = append(dc.xfStack, dc.xf)
}

// Restore pops the transform Save pushed. Restoring with an empty
// stack is a no-op: OnDraw runs inside the frame, so a panic here
// would take the whole window down over a caller's bookkeeping slip.
func (dc *DrawContext) Restore() {
	n := len(dc.xfStack)
	if n == 0 {
		return
	}
	dc.xf = dc.xfStack[n-1]
	dc.xfStack = dc.xfStack[:n-1]
}

// ensureXform promotes the zero value to the identity the first time
// any transform method runs. After this dc.xf is meaningful and
// dc.xfActive gates every consumer.
func (dc *DrawContext) ensureXform() {
	if !dc.xfActive {
		dc.xf = identityXform
		dc.xfActive = true
	}
}

// resetXform returns the context to "no transform" for the next
// redraw. Called from resetFor, which runs immediately before every
// OnDraw, so an unbalanced Save cannot leak into the next frame or
// into another canvas sharing the window's scratch context.
func (dc *DrawContext) resetXform() {
	dc.xf = canvasXform{}
	dc.xfActive = false
	dc.xfStack = dc.xfStack[:0]
}

// activeXform is the transform a batch should carry, and whether it
// needs to carry one at all.
//
// A context that has been transformed and then restored all the way
// back is reported as untransformed: the identity would otherwise
// stamp every later batch, breaking the run-length merge against the
// batches drawn before the first Save and putting a matrix on every
// command for no effect.
func (dc *DrawContext) activeXform() (canvasXform, bool) {
	if !dc.xfActive || dc.xf == identityXform {
		return canvasXform{}, false
	}
	return dc.xf, true
}

// xfRect maps a rect and normalizes it, so a negative scale yields a
// positive-extent rect at the mirrored position instead of a rect the
// emit-time w <= 0 guard silently drops. Content is not mirrored;
// only the rect moves.
func (dc *DrawContext) xfRect(x, y, w, h float32) (float32, float32, float32, float32) {
	if _, ok := dc.activeXform(); !ok {
		return x, y, w, h
	}
	x0, y0 := dc.xf.apply(x, y)
	x1, y1 := dc.xf.apply(x+w, y+h)
	return min(x0, x1), min(y0, y1), xfAbs(x1 - x0), xfAbs(y1 - y0)
}

// maxXfPoints caps the scratch buffer retained by xfPoints. A single
// unbounded Polyline would otherwise pin tens of megabytes across frames.
const maxXfPoints = 1 << 20 // ~4 MiB of floats

// xfPoints maps a caller's point slice into the context's scratch
// buffer. The caller's slice is never written to: a recorder may hold
// it, and callers pass their own backing arrays.
func (dc *DrawContext) xfPoints(points []float32) []float32 {
	if _, ok := dc.activeXform(); !ok {
		return points
	}
	if len(points) > maxXfPoints {
		return points
	}
	dc.xfPtBuf = append(dc.xfPtBuf[:0], points...)
	for i := 0; i+1 < len(dc.xfPtBuf); i += 2 {
		dc.xfPtBuf[i], dc.xfPtBuf[i+1] =
			dc.xf.apply(dc.xfPtBuf[i], dc.xfPtBuf[i+1])
	}
	return dc.xfPtBuf
}

// scaleTextStyle scales the px-valued fields of a style.
//
// Size, LineSpacing, StrokeWidth and CellHeight take sy: a font size
// is a vertical em measure, and the three others are vertical by
// construction. LetterSpacing, CellWidth and EmojiBoxWidth are
// horizontal advances and take sx. Under a uniform scale — the case
// with an unambiguous answer — every field scales alike.
//
// Glyphs are not sheared: composing TextStyle.AffineTransform would
// allocate per entry per frame on a path kept allocation-free, and it
// collides with the RotationRadians precedence in renderDrawCanvas.
func (x canvasXform) scaleTextStyle(s TextStyle) TextStyle {
	sx, sy := xfAbs(x.sx), xfAbs(x.sy)
	s.Size *= sy
	s.LineSpacing *= sy
	s.StrokeWidth *= sy
	s.CellHeight *= sy
	s.LetterSpacing *= sx
	s.CellWidth *= sx
	s.EmojiBoxWidth *= sx
	return s
}

// xfAbs is a local abs for float32. Named xfAbs rather than abs32
// because a test helper in this package already owns that name.
func xfAbs(v float32) float32 {
	if v < 0 {
		return -v
	}
	return v
}

// --- recorder decorator -------------------------------------------
//
// A DrawRecorder (SVG/PDF export) sees BAKED coordinates. DrawRecorder
// is exported and implemented outside this repo, so the alternative —
// handing over local coordinates and a matrix — would need a wider
// interface and would silently misplace every existing implementer's
// output until they adopted it. Baking makes them all correct with no
// change on their side.
//
// The decorator exists rather than baking at each of the two dozen
// call sites, which would both bloat canvas_draw.go past its size gate
// and give delegation (Line -> Polyline) a second chance to apply the
// transform twice.

// canvasImageRecorder is the optional image extension DrawContext.Image
// probes for. Named here so the decorator can implement it and the
// unwrap helper can assert it against the inner recorder.
type canvasImageRecorder interface {
	Image(x, y, w, h float32, src string,
		bgOpacity Opt[float32], bgColor Color)
}

// xformRecorder wraps the caller's recorder and bakes the active
// transform into every coordinate on the way through.
//
// Scalars with no axis of their own — stroke widths, dash and gap
// lengths, corner radii — take the geometric mean of the two scales,
// which is exact whenever the scale is uniform.
type xformRecorder struct {
	inner DrawRecorder
	dc    *DrawContext
	xf    canvasXform
}

// rec returns the recorder the drawing methods should call. With no
// transform in force that is the caller's recorder unchanged, so the
// decorator costs nothing on the common path.
//
// It does not check dc.recorder for nil: every call site is already
// inside a `dc.recorder != nil` guard, and those guards stay because
// they also decide whether to record at all.
func (dc *DrawContext) rec() DrawRecorder {
	// nil in, nil out: every call site is inside a nil guard, and
	// returning a decorator wrapping nil would turn that contract's
	// one failure mode into a less obvious one.
	if dc.recorder == nil {
		return nil
	}
	if _, ok := dc.activeXform(); !ok {
		return dc.recorder
	}
	if dc.xfRec == nil {
		dc.xfRec = &xformRecorder{dc: dc}
	}
	dc.xfRec.inner = dc.recorder
	dc.xfRec.xf = dc.xf
	return dc.xfRec
}

// gradientRecorder asserts DrawGradientRecorder against the INNER
// recorder and returns something callable.
//
// Asserting against dc.rec() instead would always succeed — the
// decorator implements every extension — and that would quietly kill
// the flat-triangle degradation that stops an export dropping a
// gradient fill.
func (dc *DrawContext) gradientRecorder() (DrawGradientRecorder, bool) {
	if dc.recorder == nil {
		return nil, false
	}
	if _, ok := dc.recorder.(DrawGradientRecorder); !ok {
		return nil, false
	}
	if _, ok := dc.activeXform(); !ok {
		return dc.recorder.(DrawGradientRecorder), true
	}
	return dc.rec().(*xformRecorder), true
}

// vertexColorRecorder is gradientRecorder for DrawVertexColorRecorder,
// and asserts against the inner recorder for the same reason.
func (dc *DrawContext) vertexColorRecorder() (DrawVertexColorRecorder, bool) {
	if dc.recorder == nil {
		return nil, false
	}
	if _, ok := dc.recorder.(DrawVertexColorRecorder); !ok {
		return nil, false
	}
	if _, ok := dc.activeXform(); !ok {
		return dc.recorder.(DrawVertexColorRecorder), true
	}
	return dc.rec().(*xformRecorder), true
}

// imageRecorder is gradientRecorder for the optional image extension.
func (dc *DrawContext) imageRecorder() (canvasImageRecorder, bool) {
	if dc.recorder == nil {
		return nil, false
	}
	if _, ok := dc.recorder.(canvasImageRecorder); !ok {
		return nil, false
	}
	if _, ok := dc.activeXform(); !ok {
		return dc.recorder.(canvasImageRecorder), true
	}
	return dc.rec().(*xformRecorder), true
}

func (r *xformRecorder) pt(x, y float32) (float32, float32) {
	return r.xf.apply(x, y)
}

func (r *xformRecorder) pts(points []float32) []float32 {
	return r.dc.xfPoints(points)
}

// ln scales a length with no axis of its own.
func (r *xformRecorder) ln(v float32) float32 { return v * r.xf.meanScale() }

func (r *xformRecorder) Line(x0, y0, x1, y1 float32, color Color, width float32) {
	ax, ay := r.pt(x0, y0)
	bx, by := r.pt(x1, y1)
	r.inner.Line(ax, ay, bx, by, color, r.ln(width))
}

func (r *xformRecorder) Polyline(points []float32, color Color, width float32) {
	r.inner.Polyline(r.pts(points), color, r.ln(width))
}

func (r *xformRecorder) FilledRect(x, y, w, h float32, color Color) {
	nx, ny, nw, nh := r.dc.xfRect(x, y, w, h)
	r.inner.FilledRect(nx, ny, nw, nh, color)
}

func (r *xformRecorder) Rect(x, y, w, h float32, color Color, width float32) {
	nx, ny, nw, nh := r.dc.xfRect(x, y, w, h)
	r.inner.Rect(nx, ny, nw, nh, color, r.ln(width))
}

// FilledCircle routes to FilledArc under a non-uniform scale: the
// result is an ellipse, which the recorder API can express exactly as
// a full-sweep arc but cannot express as a circle.
func (r *xformRecorder) FilledCircle(cx, cy, radius float32, color Color) {
	x, y := r.pt(cx, cy)
	if r.xf.uniform() {
		r.inner.FilledCircle(x, y, radius*xfAbs(r.xf.sx), color)
		return
	}
	r.inner.FilledArc(x, y, radius*xfAbs(r.xf.sx), radius*xfAbs(r.xf.sy),
		0, 2*math.Pi, color)
}

func (r *xformRecorder) Circle(cx, cy, radius float32, color Color, width float32) {
	x, y := r.pt(cx, cy)
	if r.xf.uniform() {
		r.inner.Circle(x, y, radius*xfAbs(r.xf.sx), color, r.ln(width))
		return
	}
	r.inner.Arc(x, y, radius*xfAbs(r.xf.sx), radius*xfAbs(r.xf.sy),
		0, 2*math.Pi, color, r.ln(width))
}

// FilledArc scales the two radii per axis and leaves start/sweep
// alone: an axis-aligned scale maps the point at parameter t on the
// source ellipse to the point at the same t on the scaled one, so the
// parametrization is preserved.
func (r *xformRecorder) FilledArc(cx, cy, rx, ry, start, sweep float32, color Color) {
	x, y := r.pt(cx, cy)
	r.inner.FilledArc(x, y, rx*xfAbs(r.xf.sx), ry*xfAbs(r.xf.sy),
		start, sweep, color)
}

func (r *xformRecorder) Arc(cx, cy, rx, ry, start, sweep float32,
	color Color, width float32) {
	x, y := r.pt(cx, cy)
	r.inner.Arc(x, y, rx*xfAbs(r.xf.sx), ry*xfAbs(r.xf.sy),
		start, sweep, color, r.ln(width))
}

func (r *xformRecorder) FilledPolygon(points []float32, color Color) {
	r.inner.FilledPolygon(r.pts(points), color)
}

func (r *xformRecorder) FilledRoundedRect(x, y, w, h, radius float32, color Color) {
	nx, ny, nw, nh := r.dc.xfRect(x, y, w, h)
	r.inner.FilledRoundedRect(nx, ny, nw, nh, r.ln(radius), color)
}

func (r *xformRecorder) RoundedRect(x, y, w, h, radius float32,
	color Color, width float32) {
	nx, ny, nw, nh := r.dc.xfRect(x, y, w, h)
	r.inner.RoundedRect(nx, ny, nw, nh, r.ln(radius), color, r.ln(width))
}

func (r *xformRecorder) DashedLine(x0, y0, x1, y1 float32, color Color,
	width, dashLen, gapLen float32) {
	ax, ay := r.pt(x0, y0)
	bx, by := r.pt(x1, y1)
	r.inner.DashedLine(ax, ay, bx, by, color,
		r.ln(width), r.ln(dashLen), r.ln(gapLen))
}

func (r *xformRecorder) DashedPolyline(points []float32, color Color,
	width, dashLen, gapLen float32) {
	r.inner.DashedPolyline(r.pts(points), color,
		r.ln(width), r.ln(dashLen), r.ln(gapLen))
}

func (r *xformRecorder) PolylineJoined(points []float32, color Color, width float32) {
	r.inner.PolylineJoined(r.pts(points), color, r.ln(width))
}

func (r *xformRecorder) QuadBezier(x0, y0, cx, cy, x1, y1 float32,
	color Color, width float32) {
	ax, ay := r.pt(x0, y0)
	bx, by := r.pt(cx, cy)
	ex, ey := r.pt(x1, y1)
	r.inner.QuadBezier(ax, ay, bx, by, ex, ey, color, r.ln(width))
}

func (r *xformRecorder) CubicBezier(x0, y0, c1x, c1y, c2x, c2y, x1, y1 float32,
	color Color, width float32) {
	ax, ay := r.pt(x0, y0)
	bx, by := r.pt(c1x, c1y)
	cx2, cy2 := r.pt(c2x, c2y)
	ex, ey := r.pt(x1, y1)
	r.inner.CubicBezier(ax, ay, bx, by, cx2, cy2, ex, ey, color, r.ln(width))
}

func (r *xformRecorder) Text(x, y float32, text string, style TextStyle) {
	nx, ny := r.pt(x, y)
	r.inner.Text(nx, ny, text, r.xf.scaleTextStyle(style))
}

// FillTrianglesGradient is reached only through gradientRecorder, so
// the inner recorder is known to implement DrawGradientRecorder.
func (r *xformRecorder) FillTrianglesGradient(tris []float32, g *CanvasGradient) {
	r.inner.(DrawGradientRecorder).FillTrianglesGradient(r.pts(tris), g)
}

// FillTrianglesColors is reached only through vertexColorRecorder.
func (r *xformRecorder) FillTrianglesColors(tris []float32, colors []Color) {
	r.inner.(DrawVertexColorRecorder).FillTrianglesColors(r.pts(tris), colors)
}

// Image is reached only through imageRecorder.
func (r *xformRecorder) Image(x, y, w, h float32, src string,
	bgOpacity Opt[float32], bgColor Color) {
	nx, ny, nw, nh := r.dc.xfRect(x, y, w, h)
	r.inner.(canvasImageRecorder).Image(nx, ny, nw, nh, src, bgOpacity, bgColor)
}
