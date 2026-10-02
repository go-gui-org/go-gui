package gui

import (
	"math"

	"github.com/go-gui-org/go-glyph"
)

// canvas_draw_transform.go — the affine stack for DrawContext
// (issues #474, #904).
//
// The transform is the full 2x3 affine
//
//	p' = (xx*x + xy*y + tx, yx*x + yy*y + ty)
//
// — translate, per-axis scale and rotation, composed in call order.
// That group is closed under composition, so an arbitrarily deep
// Save/Translate/ScaleBy/Rotate nest collapses to six floats. That is
// what makes the geometry path free: the six floats ride on the batch
// and then on the RenderCmd, where every backend already applies them
// per vertex, so no triangle is ever rewritten on the CPU.
//
// Geometry therefore does NOT bake. Text, images and the lowered
// radial gradient do bake, at record time, because their render
// commands have no xform fields to ride on. They are also the three
// paths with no primitive-to-primitive delegation, so baking there
// cannot double-apply. Under rotation a text anchor maps through the
// full matrix while its style gains the angle (or a composed affine),
// an image records its rotated frame for a backend rotation bracket,
// and the lowered gradient maps its center: a circle stays a circle
// under a similarity, so only a non-uniform scale falls back to the
// ring mesh, which transforms exactly.
//
// Documented limits, all consequences of the design above:
//
//   - Curve flattening (arcPoints, the bezier subdivision) picks its
//     segment count in LOCAL space, so scaling a circle up by 10x
//     leaves it as faceted as the unscaled one. Drawing it at its
//     final size gives a smoother result. Rotation preserves lengths,
//     so it neither facets nor smooths.
//   - A non-uniform scale positions and sizes a text run but does not
//     shear its glyphs. Under rotation the glyphs turn with the anchor
//     but are never sheared: a sheared CTM draws its text un-sheared.
//   - A negative scale moves an image's rect but never mirrors its
//     content, and a sheared CTM draws its image un-sheared.
//   - A concentric radial gradient under a non-uniform scale gives up
//     the single-quad shader path and falls back to the ring mesh,
//     which transforms exactly.

// maxXformDepth bounds the Save stack. A Save inside a per-frame draw
// loop with no matching Restore would otherwise grow the slice without
// limit for as long as the canvas keeps redrawing.
const maxXformDepth = 256

// canvasXform is the CTM in glyph.AffineTransform order: the columns
// (xx, yx) and (xy, yy) are the mapped local axes, (tx, ty) the
// translation. sim records whether the linear part is a similarity —
// uniform scale plus rotation only, so circles stay circles. It is
// maintained structurally (Translate and Rotate keep it, ScaleBy keeps
// it only for sx == sy) because float rounding makes re-deriving it
// unreliable: on arm64 a Translate;Rotate;ScaleBy chain leaves the
// column dot product at -4.8e-8 instead of exactly zero, so an exact
// re-check would drop every rotated glow to the mesh path.
//
// Its zero value is NOT the identity — the identity is
// {xx: 1, yy: 1, sim: true} — which is why DrawContext gates on
// xfActive rather than on the field values. A zero-value DrawContext
// literal (tests build them directly) must behave exactly as it did
// before this file existed.
type canvasXform struct {
	xx, xy, yx, yy, tx, ty float32
	sim                    bool
}

// identityXform is the transform a context starts every redraw with.
var identityXform = canvasXform{xx: 1, yy: 1, sim: true}

// apply maps one point from local space to canvas space.
func (x canvasXform) apply(px, py float32) (float32, float32) {
	return x.xx*px + x.xy*py + x.tx, x.yx*px + x.yy*py + x.ty
}

// rotated reports whether the linear part turns the axes: either
// off-diagonal is nonzero. An axis-aligned transform — including a
// mirroring negative scale — is not rotation, so text keeps its angle
// and images keep their fast path under it.
func (x canvasXform) rotated() bool { return x.xy != 0 || x.yx != 0 }

// colScaleX is the length of the first basis column: the scale a
// horizontal advance actually experiences. colScaleY is the same for
// the second column and the vertical em. Under translate+scale these
// are |sx| and |sy|, so every unrotated caller behaves as before.
func (x canvasXform) colScaleX() float32 {
	return float32(math.Hypot(float64(x.xx), float64(x.yx)))
}

func (x canvasXform) colScaleY() float32 {
	return float32(math.Hypot(float64(x.xy), float64(x.yy)))
}

// det is the signed area scale.
func (x canvasXform) det() float32 { return x.xx*x.yy - x.xy*x.yx }

// meanScale is the scale to apply to a length that has no axis of its
// own — a stroke width, a dash length, a corner radius. The root of
// the absolute determinant is exact under a uniform scale and under a
// pure rotation, which are the cases that have a right answer at all.
func (x canvasXform) meanScale() float32 {
	return float32(math.Sqrt(math.Abs(float64(x.det()))))
}

// uniform reports whether the transform maps circles to circles.
// It reads the structural sim flag rather than re-deriving
// perpendicularity from the floats (see the struct doc): a rotated
// similarity then keeps the single-quad and circle fast paths with at
// most an invisible epsilon of shear, while any non-uniform scale in
// the chain declines them exactly as before.
//
// The radial gradient's single-quad path and the recorder's circle
// methods both need to know, because neither can express an ellipse.
func (x canvasXform) uniform() bool { return x.sim }

// Translate shifts the origin by (dx, dy) measured in the CURRENT
// local units, so a Translate after a ScaleBy moves by scaled units,
// and after a Rotate by rotated ones — the same post-multiply order
// HTML canvas uses.
//
// Composition: t' = t + dx*col0 + dy*col1.
//
// Non-finite arguments are ignored rather than poisoning every
// subsequent vertex, and so is a finite shift whose SUM with the
// current translation overflows — the same overflow ScaleBy screens,
// reached through repeated Translate instead of repeated ScaleBy.
func (dc *DrawContext) Translate(dx, dy float32) {
	if !f32IsFinite(dx) || !f32IsFinite(dy) {
		return
	}
	dc.ensureXform()
	ntx := dc.xf.tx + dx*dc.xf.xx + dy*dc.xf.xy
	nty := dc.xf.ty + dx*dc.xf.yx + dy*dc.xf.yy
	if !f32IsFinite(ntx) || !f32IsFinite(nty) {
		return
	}
	dc.xf.tx, dc.xf.ty = ntx, nty
}

// ScaleBy multiplies the current scale by (sx, sy). It is named
// ScaleBy, not Scale, because DrawContext.Scale is the device pixel
// ratio field and predates this API; the two are unrelated.
//
// Composition: M' = M*diag(sx, sy), so each basis column scales in
// place and scaling happens about the current origin. The translation
// is untouched.
//
// A zero scale is allowed and collapses geometry. Non-finite
// arguments are ignored, and so is a finite pair whose PRODUCT with
// the current scale overflows: two ScaleBy(1e38, 1e38) calls would
// otherwise leave xx at +Inf, which scaleTextStyle bakes into a text
// entry's Size. The render commands themselves are screened
// downstream, but the text emit path measures through the glyph
// shaper before that screen runs, so an infinite font size must not
// reach the transform at all.
func (dc *DrawContext) ScaleBy(sx, sy float32) {
	if !f32IsFinite(sx) || !f32IsFinite(sy) {
		return
	}
	dc.ensureXform()
	nxx := dc.xf.xx * sx
	nxy := dc.xf.xy * sy
	nyx := dc.xf.yx * sx
	nyy := dc.xf.yy * sy
	if !f32AllFinite4(nxx, nxy, nyx, nyy) {
		return
	}
	dc.xf.xx, dc.xf.xy, dc.xf.yx, dc.xf.yy = nxx, nxy, nyx, nyy
	// A non-uniform scale is the one op that stops mapping circles to
	// circles (and, sandwiched between rotations, introduces shear),
	// so only it can clear the similarity flag. With no rotation in
	// force the flag is re-derived instead, exactly as the old
	// translate+scale check did (xx == yy): those floats carry no
	// rotation noise, so ScaleBy(2,3);ScaleBy(1.5,1) is uniform again
	// and keeps the circle fast paths.
	if nxy == 0 && nyx == 0 {
		dc.xf.sim = nxx == nyy
	} else {
		dc.xf.sim = dc.xf.sim && sx == sy
	}
}

// Rotate turns subsequent drawing by rad radians about the current
// origin, composed after whatever the context already holds: a Rotate
// after a Translate spins about the translated origin, and a
// Translate after a Rotate moves in rotated units — the same
// post-multiply order HTML canvas uses.
//
// Composition: M' = M*R(rad), so the translation is untouched and the
// rotation happens about the current origin. To spin about another
// point, bracket with Translate: move there, Rotate, move back.
//
// The angle is in the canvas's own y-down space, so positive turns
// clockwise on screen — the same convention TextStyle.RotationRadians
// and the backend rotation brackets use.
//
// A non-finite angle is ignored rather than poisoning every
// subsequent vertex, and so is a finite angle whose composition with
// the current matrix overflows: the same overflow ScaleBy screens,
// reached here through the cos/sin products instead of direct ones.
func (dc *DrawContext) Rotate(rad float32) {
	if !f32IsFinite(rad) {
		return
	}
	c := float32(math.Cos(float64(rad)))
	s := float32(math.Sin(float64(rad)))
	dc.ensureXform()
	nxx := dc.xf.xx*c + dc.xf.xy*s
	nxy := -dc.xf.xx*s + dc.xf.xy*c
	nyx := dc.xf.yx*c + dc.xf.yy*s
	nyy := -dc.xf.yx*s + dc.xf.yy*c
	if !f32AllFinite4(nxx, nxy, nyx, nyy) {
		return
	}
	dc.xf.xx, dc.xf.xy, dc.xf.yx, dc.xf.yy = nxx, nxy, nyx, nyy
}

// Save pushes the current transform so a later Restore can return to
// it. Pushes past maxXformDepth are not stored, but they are COUNTED:
// dropping a push silently would make the matching Restore pop an
// ancestor instead, so every Restore after the cap was hit would
// rewind one level too far and the rest of the nest would draw at the
// wrong offset.
func (dc *DrawContext) Save() {
	dc.ensureXform()
	if len(dc.xfStack) >= maxXformDepth {
		dc.xfDropped++
		return
	}
	dc.xfStack = append(dc.xfStack, dc.xf)
}

// Restore pops the transform Save pushed. Restoring with an empty
// stack is a no-op: OnDraw runs inside the frame, so a panic here
// would take the whole window down over a caller's bookkeeping slip.
//
// A Restore matching a push the cap dropped is also a no-op, which is
// what keeps the stack balanced: the dropped pushes are unwound in
// reverse order, so the first Restore to touch real state is the one
// matching the last push that was actually stored.
func (dc *DrawContext) Restore() {
	if dc.xfDropped > 0 {
		dc.xfDropped--
		return
	}
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
	dc.xfDropped = 0
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

// xfPoints maps a caller's point slice into the context's scratch
// buffer. The caller's slice is never written to, because callers pass
// their own backing arrays and a primitive must not mutate them.
//
// The returned buffer is single and shared, so it is only valid until
// the next primitive runs. That is the retention rule DrawRecorder
// states: a recorder that keeps the points it is handed must copy
// them. Giving each call its own buffer instead would mean an arena
// whose growth reallocates and invalidates the slices already handed
// out — the same hazard, harder to see.
//
// Every length is mapped. An earlier cap here returned the caller's
// points unmapped past a size threshold, which handed a recorder raw
// local coordinates and silently misplaced a large polyline in an
// export. A run-away buffer is released instead by resetFor, which
// runs keepScratch over xfPtBuf with every other canvas scratch.
func (dc *DrawContext) xfPoints(points []float32) []float32 {
	if _, ok := dc.activeXform(); !ok {
		return points
	}
	dc.xfPtBuf = append(dc.xfPtBuf[:0], points...)
	for i := 0; i+1 < len(dc.xfPtBuf); i += 2 {
		dc.xfPtBuf[i], dc.xfPtBuf[i+1] =
			dc.xf.apply(dc.xfPtBuf[i], dc.xfPtBuf[i+1])
	}
	return dc.xfPtBuf
}

// xfTextStyle maps a text style's px-valued fields through the
// transform.
//
// Size, LineSpacing, StrokeWidth and CellHeight take the vertical
// column length: a font size is a vertical em measure, and the three
// others are vertical by construction. LetterSpacing, CellWidth and
// EmojiBoxWidth are horizontal advances and take the horizontal
// column length. Under translate+scale these are |sx| and |sy|, so an
// unrotated transform behaves exactly as before.
//
// Under rotation the style also gains the angle: added onto
// RotationRadians, so the text emit path keeps its GPU bracket, or
// composed onto an explicit AffineTransform, which keeps precedence
// over rotation as it does in renderDrawCanvas. The composed affine
// carries the CTM's rotation only — the anchor already moved through
// the full matrix at the call site, and the scale already rode in on
// Size and the advances — and it heap-allocates,
// but only on this path: an unrotated transform never touches the
// pointer, so the allocation-free fast path is unchanged.
//
// Glyphs are not sheared: a sheared CTM draws its text un-sheared, in
// the same documented class as the old non-uniform-scale limit.
func (x canvasXform) xfTextStyle(s TextStyle) TextStyle {
	sx, sy := x.colScaleX(), x.colScaleY()
	s.Size *= sy
	s.LineSpacing *= sy
	s.StrokeWidth *= sy
	s.CellHeight *= sy
	s.LetterSpacing *= sx
	s.CellWidth *= sx
	s.EmojiBoxWidth *= sx
	if !x.rotated() {
		return s
	}
	if s.AffineTransform != nil &&
		!affineTransformIsIdentity(*s.AffineTransform) {
		// Rotation only, not the CTM's linear part: Size and the
		// advances above already carry the column lengths, so a
		// scaled matrix here would scale the glyphs a second time.
		clean := glyph.AffineRotation(x.angle())
		composed := clean.Multiply(*s.AffineTransform)
		s.AffineTransform = &composed
		return s
	}
	s.RotationRadians += x.angle()
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

// xfText maps a text call's anchor and style through the transform and
// screens the result. ok=false drops the entry: a non-finite anchor,
// size or composed angle must not reach the glyph shaper, which
// measures before render-command validation runs.
//
// The screen runs whether or not a transform is active — the
// non-finite argument checks below predate it — so callers branch
// once, on ok, instead of twice.
func (dc *DrawContext) xfText(x, y float32, style TextStyle) (
	float32, float32, TextStyle, bool) {
	if _, ok := dc.activeXform(); ok {
		x, y = dc.xf.apply(x, y)
		style = dc.xf.xfTextStyle(style)
	}
	if !f32AllFinite9(x, y, style.Size, style.LineSpacing,
		style.StrokeWidth, style.LetterSpacing,
		style.CellWidth, style.CellHeight, style.EmojiBoxWidth) {
		return x, y, style, false
	}
	// A composed rotation angle can overflow where the nine screened
	// fields cannot: it is a sum, not a product.
	return x, y, style, f32IsFinite(style.RotationRadians)
}

// bakeRotatedImage records an image under a rotated transform: the
// mapped corner, the mapped size and the angle, for the backend
// rotation bracket at emit. Reports whether it handled the call; an
// unrotated transform falls through to the xfRect path. Content is
// never sheared.
func (dc *DrawContext) bakeRotatedImage(x, y, w, h float32, src string,
	bgOpacity Opt[float32], bgColor Color, fetcher ImageFetcher) bool {
	xf, ok := dc.activeXform()
	if !ok || !xf.rotated() {
		return false
	}
	// The bracket turns the image about its anchor and grows it along
	// the angle (by W) and its clockwise perpendicular (by H). Under a
	// mirror (det < 0) one column points against those directions, so
	// anchor at the mapped corner that column grows away from: (x, y+h)
	// when angle reads the first column, (x+w, y) when it reads the
	// second. The drawn rect then covers the mapped parallelogram.
	ax, ay := xf.apply(x, y)
	switch {
	case xf.angleFromCol1():
		ax, ay = xf.apply(x+w, y)
	case xf.det() < 0:
		ax, ay = xf.apply(x, y+h)
	}
	dc.images = append(dc.images, DrawCanvasImageEntry{
		X: ax, Y: ay,
		W: w * xf.colScaleX(), H: h * xf.colScaleY(),
		Src: src, bgOpacity: bgOpacity, BgColor: bgColor,
		fetcher: fetcher, rotRad: xf.angle(),
	})
	return true
}

// xfClipRect maps a clip rect, bounding-boxing under rotation. The
// scissor stays axis-aligned, so a rotated clip is approximate,
// documented on the recorder's Image for the same reason.
func (dc *DrawContext) xfClipRect(x, y, w, h float32) (
	float32, float32, float32, float32) {
	if xf, ok := dc.activeXform(); ok && xf.rotated() {
		return xf.xfBBox(x, y, w, h)
	}
	return dc.xfRect(x, y, w, h)
}

// angle, angleFromCol1, xfCorners, xfBBox, closedLoop and roundedOutline live in
// canvas_draw_rotate.go: both files sit under the same 800-line gate
// as canvas_draw.go, and the rotation helpers would push either one
// past it.

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
	// A rotated rect is a parallelogram, which the recorder API has
	// no method for: record its mapped outline instead. Exact — the
	// corners map exactly under an affine.
	if r.xf.rotated() {
		c := r.xf.xfCorners(x, y, w, h)
		r.inner.FilledPolygon(c[:], color)
		return
	}
	nx, ny, nw, nh := r.dc.xfRect(x, y, w, h)
	r.inner.FilledRect(nx, ny, nw, nh, color)
}

func (r *xformRecorder) Rect(x, y, w, h float32, color Color, width float32) {
	if r.xf.rotated() {
		c := r.xf.xfCorners(x, y, w, h)
		loop := r.closedLoop(c[:])
		r.inner.PolylineJoined(loop, color, r.ln(width))
		return
	}
	nx, ny, nw, nh := r.dc.xfRect(x, y, w, h)
	r.inner.Rect(nx, ny, nw, nh, color, r.ln(width))
}

// FilledCircle routes to FilledArc under a non-uniform scale: the
// result is an ellipse, which the recorder API can express exactly as
// a full-sweep arc but cannot express as a circle. Under rotation a
// uniform transform still maps circles to circles, so only a rotated
// non-uniform transform — a true shear of the circle — flattens to a
// polygon.
func (r *xformRecorder) FilledCircle(cx, cy, radius float32, color Color) {
	x, y := r.pt(cx, cy)
	if r.xf.uniform() {
		r.inner.FilledCircle(x, y, radius*r.xf.colScaleX(), color)
		return
	}
	if !r.xf.rotated() {
		r.inner.FilledArc(x, y, radius*r.xf.colScaleX(), radius*r.xf.colScaleY(),
			0, 2*math.Pi, color)
		return
	}
	if pts := r.dc.arcPoints(cx, cy, radius, radius, 0, 2*math.Pi); len(pts) >= 2 {
		r.inner.FilledPolygon(r.pts(pts), color)
	}
}

func (r *xformRecorder) Circle(cx, cy, radius float32, color Color, width float32) {
	x, y := r.pt(cx, cy)
	if r.xf.uniform() {
		r.inner.Circle(x, y, radius*r.xf.colScaleX(), color, r.ln(width))
		return
	}
	if !r.xf.rotated() {
		r.inner.Arc(x, y, radius*r.xf.colScaleX(), radius*r.xf.colScaleY(),
			0, 2*math.Pi, color, r.ln(width))
		return
	}
	if pts := r.dc.arcPoints(cx, cy, radius, radius, 0, 2*math.Pi); len(pts) >= 2 {
		r.inner.PolylineJoined(r.closedLoop(pts), color, r.ln(width))
	}
}

// FilledArc scales the two radii per axis and leaves start/sweep
// alone: an axis-aligned scale maps the point at parameter t on the
// source ellipse to the point at the same t on the scaled one, so the
// parametrization is preserved. A rotated ellipse has no axis-aligned
// expression, so rotation flattens to a polygon at the arc tolerance.
func (r *xformRecorder) FilledArc(cx, cy, rx, ry, start, sweep float32, color Color) {
	if r.xf.rotated() {
		pts := r.dc.arcPoints(cx, cy, rx, ry, start, sweep)
		if len(pts) < 2 {
			return
		}
		pie := r.dc.gradTriBuf[:0]
		pie = append(pie, cx, cy)
		pie = append(pie, pts...)
		// Store the grown buffer back, or every call regrows it.
		r.dc.gradTriBuf = pie
		r.inner.FilledPolygon(r.pts(pie), color)
		return
	}
	x, y := r.pt(cx, cy)
	r.inner.FilledArc(x, y, rx*r.xf.colScaleX(), ry*r.xf.colScaleY(),
		start, sweep, color)
}

func (r *xformRecorder) Arc(cx, cy, rx, ry, start, sweep float32,
	color Color, width float32) {
	if r.xf.rotated() {
		pts := r.dc.arcPoints(cx, cy, rx, ry, start, sweep)
		if len(pts) < 2 {
			return
		}
		r.inner.PolylineJoined(r.pts(pts), color, r.ln(width))
		return
	}
	x, y := r.pt(cx, cy)
	r.inner.Arc(x, y, rx*r.xf.colScaleX(), ry*r.xf.colScaleY(),
		start, sweep, color, r.ln(width))
}

func (r *xformRecorder) FilledPolygon(points []float32, color Color) {
	r.inner.FilledPolygon(r.pts(points), color)
}

func (r *xformRecorder) FilledRoundedRect(x, y, w, h, radius float32, color Color) {
	// A rotated rounded rect has no axis-aligned expression: record
	// its mapped outline, built the same way the stroked main path
	// builds its centerline.
	if r.xf.rotated() {
		if pts := r.roundedOutline(x, y, w, h, radius); len(pts) >= 6 {
			r.inner.FilledPolygon(r.pts(pts), color)
		}
		return
	}
	nx, ny, nw, nh := r.dc.xfRect(x, y, w, h)
	r.inner.FilledRoundedRect(nx, ny, nw, nh, r.ln(radius), color)
}

func (r *xformRecorder) RoundedRect(x, y, w, h, radius float32,
	color Color, width float32) {
	if r.xf.rotated() {
		if pts := r.roundedOutline(x, y, w, h, radius); len(pts) >= 4 {
			r.inner.PolylineJoined(r.closedLoop(pts), color, r.ln(width))
		}
		return
	}
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
	r.inner.Text(nx, ny, text, r.xf.xfTextStyle(style))
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

// Image is reached only through imageRecorder. Under rotation the
// entry has no angle to carry, so it records the bounding box of the
// mapped rect — approximate, in the same documented class as the
// mirror limit on the main path.
func (r *xformRecorder) Image(x, y, w, h float32, src string,
	bgOpacity Opt[float32], bgColor Color) {
	// xfClipRect is exactly that split: bounding box under rotation,
	// the normalized xfRect otherwise.
	nx, ny, nw, nh := r.dc.xfClipRect(x, y, w, h)
	r.inner.(canvasImageRecorder).Image(nx, ny, nw, nh, src, bgOpacity, bgColor)
}
