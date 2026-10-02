package gui

import (
	"math"
	"reflect"
	"testing"

	"github.com/go-gui-org/go-glyph"
)

// wantXform asserts a batch's stamped transform.
func wantXform(t *testing.T, b DrawCanvasTriBatch,
	sx, sy, tx, ty float32, active bool) {
	t.Helper()
	gsx, gsy, gtx, gty, ok := b.Transform()
	if ok != active {
		t.Fatalf("Transform() ok = %v, want %v", ok, active)
	}
	if gsx != sx || gsy != sy || gtx != tx || gty != ty {
		t.Errorf("Transform() = %v,%v,%v,%v, want %v,%v,%v,%v",
			gsx, gsy, gtx, gty, sx, sy, tx, ty)
	}
}

// A zero-value DrawContext must behave exactly as it did before the
// transform existed: no transform, no stamp, vertices in canvas space.
func TestXformZeroValueIsIdentity(t *testing.T) {
	var dc DrawContext
	dc.FilledRect(1, 2, 3, 4, Blue)
	if len(dc.batches) != 1 {
		t.Fatalf("batches = %d, want 1", len(dc.batches))
	}
	if dc.batches[0].hasXform {
		t.Error("untouched context stamped a transform")
	}
	wantXform(t, dc.batches[0], 1, 1, 0, 0, false)
	if dc.batches[0].Triangles[0] != 1 || dc.batches[0].Triangles[1] != 2 {
		t.Errorf("first vertex = %v, want 1,2", dc.batches[0].Triangles[:2])
	}
}

// Translate measures its offset in current local units, so order
// matters. This is the load-bearing composition assertion.
func TestXformComposition(t *testing.T) {
	var a DrawContext
	a.Translate(10, 20)
	a.ScaleBy(2, 3)
	a.FilledRect(1, 1, 2, 2, Blue)
	wantXform(t, a.batches[0], 2, 3, 10, 20, true)

	var b DrawContext
	b.ScaleBy(2, 3)
	b.Translate(10, 20)
	b.FilledRect(1, 1, 2, 2, Blue)
	wantXform(t, b.batches[0], 2, 3, 20, 60, true)

	// Geometry is never rewritten; the matrix carries the mapping.
	if !reflect.DeepEqual(a.batches[0].Triangles, b.batches[0].Triangles) {
		t.Error("triangles differ between transforms; they must not be baked")
	}
	if a.batches[0].Triangles[0] != 1 {
		t.Errorf("vertex baked: got %v, want local 1", a.batches[0].Triangles[0])
	}
}

func TestXformSaveRestoreNesting(t *testing.T) {
	var dc DrawContext
	dc.Save()
	dc.Translate(5, 5)
	dc.Save()
	dc.ScaleBy(2, 2)
	if dc.xf != (canvasXform{xx: 2, yy: 2, tx: 5, ty: 5, sim: true}) {
		t.Fatalf("inner xf = %+v", dc.xf)
	}
	dc.Restore()
	if dc.xf != (canvasXform{xx: 1, yy: 1, tx: 5, ty: 5, sim: true}) {
		t.Fatalf("after one Restore xf = %+v", dc.xf)
	}
	dc.Restore()
	if dc.xf != identityXform {
		t.Fatalf("after both Restores xf = %+v, want identity", dc.xf)
	}
	// Restoring past the bottom is a no-op, not a panic: OnDraw runs
	// inside the frame.
	dc.Restore()
	dc.Restore()
	if dc.xf != identityXform {
		t.Fatalf("empty-stack Restore changed xf: %+v", dc.xf)
	}
}

// An unbalanced Save must not survive into the next redraw, or one
// canvas's slip would move another's geometry.
func TestXformResetForClearsUnbalancedSave(t *testing.T) {
	var dc DrawContext
	dc.Save()
	dc.Translate(100, 100)
	dc.Save()
	dc.resetFor(10, 10, 1, nil, drawCanvasCache{})
	if dc.xfActive || dc.xf != (canvasXform{}) || len(dc.xfStack) != 0 {
		t.Fatalf("resetFor left xfActive=%v xf=%+v depth=%d",
			dc.xfActive, dc.xf, len(dc.xfStack))
	}
	dc.FilledRect(0, 0, 1, 1, Blue)
	if dc.batches[0].hasXform {
		t.Error("batch after resetFor still carries a transform")
	}
}

// The whole design exists so a primitive that delegates to another
// applies the transform exactly once.
func TestXformDelegationIsSingleApply(t *testing.T) {
	cases := []struct {
		name       string
		via, plain func(*DrawContext)
	}{{
		name:  "Line/Polyline",
		via:   func(d *DrawContext) { d.Line(0, 0, 10, 5, Blue, 2) },
		plain: func(d *DrawContext) { d.Polyline([]float32{0, 0, 10, 5}, Blue, 2) },
	}, {
		name:  "Circle/Arc",
		via:   func(d *DrawContext) { d.Circle(5, 5, 4, Blue, 2) },
		plain: func(d *DrawContext) { d.Arc(5, 5, 4, 4, 0, 2*math.Pi, Blue, 2) },
	}, {
		name:  "FilledCircle/FilledArc",
		via:   func(d *DrawContext) { d.FilledCircle(5, 5, 4, Blue) },
		plain: func(d *DrawContext) { d.FilledArc(5, 5, 4, 4, 0, 2*math.Pi, Blue) },
	}}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			var a, b DrawContext
			a.Translate(3, 7)
			a.ScaleBy(2, 2)
			c.via(&a)
			b.Translate(3, 7)
			b.ScaleBy(2, 2)
			c.plain(&b)
			if len(a.batches) != 1 || len(b.batches) != 1 {
				t.Fatalf("batches = %d/%d, want 1/1", len(a.batches), len(b.batches))
			}
			wantXform(t, a.batches[0], 2, 2, 3, 7, true)
			if !reflect.DeepEqual(a.batches[0].Triangles, b.batches[0].Triangles) {
				t.Error("delegated primitive produced different geometry")
			}
		})
	}
}

// A batch carries one matrix, so a transform change has to break the
// run-length merge that would otherwise fold two colors' worth of
// geometry together.
func TestXformBreaksBatchMerge(t *testing.T) {
	var dc DrawContext
	dc.FilledRect(0, 0, 1, 1, Blue)
	dc.FilledRect(2, 2, 1, 1, Blue)
	if len(dc.batches) != 1 {
		t.Fatalf("same color, no transform: batches = %d, want 1", len(dc.batches))
	}
	dc.Translate(10, 0)
	dc.FilledRect(0, 0, 1, 1, Blue)
	if len(dc.batches) != 2 {
		t.Fatalf("after Translate: batches = %d, want 2", len(dc.batches))
	}
	dc.FilledRect(4, 0, 1, 1, Blue)
	if len(dc.batches) != 2 {
		t.Fatalf("same color and transform: batches = %d, want 2", len(dc.batches))
	}
	// Returning to the same matrix re-merges: the key is the value,
	// not the identity of the call that set it.
	dc.Save()
	dc.ScaleBy(3, 3)
	dc.Restore()
	dc.FilledRect(6, 0, 1, 1, Blue)
	if len(dc.batches) != 2 {
		t.Fatalf("after balanced Save/Restore: batches = %d, want 2", len(dc.batches))
	}
}

// Restoring all the way back must leave batches indistinguishable
// from ones drawn before any transform existed: an identity matrix on
// every later command would break the merge for no effect.
func TestXformFullRestoreIsUntransformed(t *testing.T) {
	var dc DrawContext
	dc.FilledRect(0, 0, 1, 1, Blue)
	dc.Save()
	dc.Translate(10, 10)
	dc.FilledRect(0, 0, 1, 1, Blue)
	dc.Restore()
	dc.FilledRect(2, 0, 1, 1, Blue)
	if len(dc.batches) != 3 {
		t.Fatalf("batches = %d, want 3", len(dc.batches))
	}
	if dc.batches[2].hasXform {
		t.Error("batch after a full Restore still carries a matrix")
	}
	if _, ok := dc.activeXform(); ok {
		t.Error("activeXform reports the identity as active")
	}
}

func TestXformText(t *testing.T) {
	var dc DrawContext
	dc.Translate(10, 20)
	dc.ScaleBy(2, 4)
	st := TextStyle{Size: 10, LetterSpacing: 3, LineSpacing: 5}
	dc.Text(1, 1, "hi", st)
	if len(dc.texts) != 1 {
		t.Fatalf("texts = %d, want 1", len(dc.texts))
	}
	e := dc.texts[0]
	if e.X != 12 || e.Y != 24 {
		t.Errorf("position = %v,%v, want 12,24", e.X, e.Y)
	}
	if e.Style.Size != 40 {
		t.Errorf("Size = %v, want 40 (scaled by sy)", e.Style.Size)
	}
	if e.Style.LineSpacing != 20 {
		t.Errorf("LineSpacing = %v, want 20", e.Style.LineSpacing)
	}
	if e.Style.LetterSpacing != 6 {
		t.Errorf("LetterSpacing = %v, want 6 (scaled by sx)", e.Style.LetterSpacing)
	}
	// Measurement answers a question about local space, which is the
	// space Text's own arguments are in.
	if got := dc.FontHeight(st); got != 10 {
		t.Errorf("FontHeight = %v, want the unscaled 10", got)
	}
}

func TestXformImageRect(t *testing.T) {
	var dc DrawContext
	dc.Translate(10, 10)
	dc.ScaleBy(2, 2)
	dc.Image(1, 1, 4, 4, "a.png", Opt[float32]{}, Color{})
	if len(dc.images) != 1 {
		t.Fatalf("images = %d, want 1", len(dc.images))
	}
	im := dc.images[0]
	if im.X != 12 || im.Y != 12 || im.W != 8 || im.H != 8 {
		t.Errorf("rect = %v,%v %vx%v, want 12,12 8x8", im.X, im.Y, im.W, im.H)
	}

	// A negative scale must yield a positive-extent rect at the
	// mirrored position, or emitDrawCanvasImages drops it.
	var neg DrawContext
	neg.ScaleBy(-1, 1)
	neg.Image(2, 0, 4, 4, "a.png", Opt[float32]{}, Color{})
	m := neg.images[0]
	if m.X != -6 || m.W != 4 {
		t.Errorf("mirrored rect = x %v w %v, want x -6 w 4", m.X, m.W)
	}
}

func TestXformImageClippedRect(t *testing.T) {
	var dc DrawContext
	dc.Translate(5, 5)
	dc.ScaleBy(2, 2)
	dc.ImageClipped(0, 0, 10, 10, "a.png", Opt[float32]{}, Color{}, 1, 1, 4, 4)
	if len(dc.images) != 1 {
		t.Fatalf("images = %d, want 1", len(dc.images))
	}
	im := dc.images[0]
	if !im.Clipped {
		t.Fatal("Clipped not set")
	}
	if im.ClipX != 7 || im.ClipY != 7 || im.ClipW != 8 || im.ClipH != 8 {
		t.Errorf("clip = %v,%v %vx%v, want 7,7 8x8",
			im.ClipX, im.ClipY, im.ClipW, im.ClipH)
	}
}

// The lowered radial's quad cannot express an ellipse, so a
// non-uniform scale has to fall back to the ring mesh.
func TestXformRadialGradientLowering(t *testing.T) {
	g := &CanvasGradient{
		Radial: true,
		Stops:  []GradientStop{{Pos: 0, Color: Blue}, {Pos: 1, Color: Green}},
	}
	var uni DrawContext
	uni.Translate(10, 10)
	uni.ScaleBy(2, 2)
	uni.FilledCircleGradient(5, 5, 4, g)
	if len(uni.Gradients()) != 1 {
		t.Fatalf("uniform scale: gradients = %d, want 1 (lowered)",
			len(uni.Gradients()))
	}
	e := uni.Gradients()[0]
	if e.X != 12 || e.Y != 12 || e.W != 16 || e.H != 16 {
		t.Errorf("quad = %v,%v %vx%v, want 12,12 16x16", e.X, e.Y, e.W, e.H)
	}

	var nonUni DrawContext
	nonUni.ScaleBy(2, 3)
	nonUni.FilledCircleGradient(5, 5, 4, g)
	if len(nonUni.Gradients()) != 0 {
		t.Errorf("non-uniform scale: gradients = %d, want 0 (mesh fallback)",
			len(nonUni.Gradients()))
	}
	if len(nonUni.Batches()) == 0 {
		t.Error("non-uniform scale produced no ring mesh")
	}
}

func TestXformRejectsNonFinite(t *testing.T) {
	nan := float32(math.NaN())
	inf := float32(math.Inf(1))
	var dc DrawContext
	dc.ScaleBy(2, 2)
	before := dc.xf
	dc.ScaleBy(nan, 1)
	dc.ScaleBy(1, inf)
	dc.Translate(inf, 0)
	dc.Translate(0, nan)
	if dc.xf != before {
		t.Errorf("non-finite argument changed xf: %+v, want %+v", dc.xf, before)
	}
}

func TestXformSaveDepthIsBounded(t *testing.T) {
	var dc DrawContext
	for range maxXformDepth + 50 {
		dc.Save()
	}
	if len(dc.xfStack) != maxXformDepth {
		t.Errorf("stack depth = %d, want it capped at %d",
			len(dc.xfStack), maxXformDepth)
	}
}

// --- recorder decorator -------------------------------------------

// xformCapture records the coordinates a recorder is handed, so the
// baking can be checked call by call.
type xformCapture struct {
	nopRecorder
	line    []float32
	poly    []float32
	rect    []float32
	arc     []float32
	circle  []float32
	fpoly   []float32
	joined  []float32
	width   float32
	textPos [2]float32
	textSt  TextStyle
}

func (r *xformCapture) Line(x0, y0, x1, y1 float32, _ Color, w float32) {
	r.line = []float32{x0, y0, x1, y1}
	r.width = w
}
func (r *xformCapture) Polyline(p []float32, _ Color, w float32) {
	r.poly = append(r.poly[:0], p...)
	r.width = w
}
func (r *xformCapture) FilledRect(x, y, w, h float32, _ Color) {
	r.rect = []float32{x, y, w, h}
}
func (r *xformCapture) FilledCircle(cx, cy, rad float32, _ Color) {
	r.circle = []float32{cx, cy, rad}
}
func (r *xformCapture) FilledArc(cx, cy, rx, ry, _, _ float32, _ Color) {
	r.arc = []float32{cx, cy, rx, ry}
}
func (r *xformCapture) Text(x, y float32, _ string, st TextStyle) {
	r.textPos = [2]float32{x, y}
	r.textSt = st
}

func (r *xformCapture) FilledPolygon(p []float32, _ Color) {
	r.fpoly = append(r.fpoly[:0], p...)
}

func (r *xformCapture) PolylineJoined(p []float32, _ Color, w float32) {
	r.joined = append(r.joined[:0], p...)
	r.width = w
}

func TestXformRecorderBakesCoordinates(t *testing.T) {
	rec := &xformCapture{}
	var dc DrawContext
	dc.SetRecorder(rec)
	dc.Translate(10, 20)
	dc.ScaleBy(2, 2)

	dc.Line(0, 0, 5, 5, Blue, 3)
	if !reflect.DeepEqual(rec.line, []float32{10, 20, 20, 30}) {
		t.Errorf("Line = %v, want [10 20 20 30]", rec.line)
	}
	if rec.width != 6 {
		t.Errorf("Line width = %v, want 6", rec.width)
	}

	dc.FilledRect(1, 1, 3, 3, Blue)
	if !reflect.DeepEqual(rec.rect, []float32{12, 22, 6, 6}) {
		t.Errorf("FilledRect = %v, want [12 22 6 6]", rec.rect)
	}

	dc.Text(1, 1, "hi", TextStyle{Size: 10})
	if rec.textPos != [2]float32{12, 22} {
		t.Errorf("Text pos = %v, want [12 22]", rec.textPos)
	}
	if rec.textSt.Size != 20 {
		t.Errorf("Text size = %v, want 20", rec.textSt.Size)
	}

	// The caller's slice must come back untouched.
	pts := []float32{0, 0, 1, 1}
	dc.Polyline(pts, Blue, 1)
	if !reflect.DeepEqual(pts, []float32{0, 0, 1, 1}) {
		t.Errorf("caller slice mutated: %v", pts)
	}
	if !reflect.DeepEqual(rec.poly, []float32{10, 20, 12, 22}) {
		t.Errorf("Polyline = %v, want [10 20 12 22]", rec.poly)
	}
}

// A circle under a non-uniform scale is an ellipse, which the
// recorder API expresses as a full-sweep arc and cannot express as a
// circle.
func TestXformRecorderCircleBecomesArc(t *testing.T) {
	rec := &xformCapture{}
	var dc DrawContext
	dc.SetRecorder(rec)
	dc.ScaleBy(2, 3)
	dc.FilledCircle(1, 1, 4, Blue)
	if rec.circle != nil {
		t.Errorf("got FilledCircle %v under a non-uniform scale", rec.circle)
	}
	if !reflect.DeepEqual(rec.arc, []float32{2, 3, 8, 12}) {
		t.Errorf("FilledArc = %v, want [2 3 8 12]", rec.arc)
	}

	// Uniform scale keeps the circle a circle.
	rec2 := &xformCapture{}
	var d2 DrawContext
	d2.SetRecorder(rec2)
	d2.ScaleBy(2, 2)
	d2.FilledCircle(1, 1, 4, Blue)
	if !reflect.DeepEqual(rec2.circle, []float32{2, 2, 8}) {
		t.Errorf("FilledCircle = %v, want [2 2 8]", rec2.circle)
	}
}

// The decorator implements every optional extension, so the unwrap
// helpers must assert against the INNER recorder — otherwise a plain
// recorder would appear to support gradients and lose the
// flat-triangle degradation that keeps an export from dropping a fill.
func TestXformRecorderUnwrapKeepsDegradation(t *testing.T) {
	plain := &meanPolyRecorder{}
	var dc DrawContext
	dc.SetRecorder(plain)
	dc.ScaleBy(2, 2)
	if _, ok := dc.gradientRecorder(); ok {
		t.Fatal("a plain recorder was reported as a DrawGradientRecorder")
	}
	if _, ok := dc.vertexColorRecorder(); ok {
		t.Fatal("a plain recorder was reported as a DrawVertexColorRecorder")
	}
	dc.FillTrianglesColors(
		[]float32{0, 0, 1, 0, 0, 1},
		[]Color{Blue, Blue, Blue},
	)
	if len(plain.colors) != 1 {
		t.Errorf("flat degradation produced %d polygons, want 1", len(plain.colors))
	}
}

// An identity transform must be indistinguishable from no transform on
// the recorder path: no allocation, no copy, no bake — otherwise a
// balanced Save/Restore would leave the fast path forever.
func TestXformIdentityIsNoOpForRecorder(t *testing.T) {
	rec := &xformCapture{}
	var dc DrawContext
	dc.SetRecorder(rec)
	dc.Save()
	dc.Restore() // leaves xf == identity but xfActive would have been true
	dc.FilledRect(1, 2, 3, 4, Blue)
	if !reflect.DeepEqual(rec.rect, []float32{1, 2, 3, 4}) {
		t.Errorf("identity baking changed rect: %v, want [1 2 3 4]", rec.rect)
	}
	// Also via a non-trivial transform that returns to identity.
	rec2 := &xformCapture{}
	var dc2 DrawContext
	dc2.SetRecorder(rec2)
	dc2.Save()
	dc2.Translate(5, 5)
	dc2.Restore()
	dc2.Line(0, 0, 1, 1, Blue, 2)
	if !reflect.DeepEqual(rec2.line, []float32{0, 0, 1, 1}) {
		t.Errorf("restored identity baked Line: %v, want [0 0 1 1]", rec2.line)
	}
	if rec2.width != 2 {
		t.Errorf("restored identity scaled width: %v, want 2", rec2.width)
	}
}

// Hostile point slices must not pin an unbounded scratch buffer.
func TestXformPointsLargeInput(t *testing.T) {
	var dc DrawContext
	dc.Translate(10, 10)
	dc.ScaleBy(2, 2)
	huge := make([]float32, (1<<20)+10)
	for i := range huge {
		huge[i] = float32(i)
	}
	// Every length is baked. An earlier cap returned the caller's slice
	// unmapped, which misplaced a large polyline in a recorder export.
	got := dc.xfPoints(huge)
	if &got[0] == &huge[0] {
		t.Fatal("xfPoints returned the caller's slice instead of a mapped copy")
	}
	if got[0] != 10 || got[1] != 12 {
		t.Errorf("point 0 = (%v, %v), want (10, 12)", got[0], got[1])
	}
	if huge[0] != 0 || huge[1] != 1 {
		t.Error("the caller's slice was mutated")
	}
	// The buffer it took is released by the next redraw, which is what
	// keeps a one-off giant polyline from pinning megabytes.
	dc.resetFor(10, 10, 1, nil, drawCanvasCache{})
	if cap(dc.xfPtBuf) > canvasScratchRetainMax {
		t.Errorf("xfPtBuf retained %d floats", cap(dc.xfPtBuf))
	}
}

func TestXformValidSvgCmdRejectsNonFiniteXform(t *testing.T) {
	cmd := RenderCmd{
		Kind: RenderSvg,
		X:    0, Y: 0, Scale: 1,
		Triangles: []float32{0, 0, 1, 0, 0, 1},
		HasXform:  true,
		ScaleX:    float32(math.NaN()),
		ScaleY:    1,
		TransX:    0,
		TransY:    0,
	}
	if validSvgCmd(cmd) {
		t.Error("validSvgCmd accepted NaN ScaleX")
	}
	cmd.ScaleX = 1
	cmd.TransX = float32(math.Inf(1))
	if validSvgCmd(cmd) {
		t.Error("validSvgCmd accepted Inf TransX")
	}
	cmd.TransX = 0
	if !validSvgCmd(cmd) {
		t.Error("validSvgCmd rejected a finite xform")
	}
	cmd.XformXY = float32(math.NaN())
	if validSvgCmd(cmd) {
		t.Error("validSvgCmd accepted NaN XformXY")
	}
	cmd.XformXY = 0
	cmd.XformYX = float32(math.Inf(1))
	if validSvgCmd(cmd) {
		t.Error("validSvgCmd accepted Inf XformYX")
	}
	cmd.XformYX = 0
	if !validSvgCmd(cmd) {
		t.Error("validSvgCmd rejected a finite affine xform")
	}
}

// --- rotation (issue #904) -------------------------------------------

// wantAffine asserts a batch's stamped six-float transform within eps.
func wantAffine(t *testing.T, b DrawCanvasTriBatch,
	xx, xy, yx, yy, tx, ty float32) {
	t.Helper()
	gxx, gxy, gyx, gyy, gtx, gty, ok := b.TransformAffine()
	if !ok {
		t.Fatal("TransformAffine() ok = false, want true")
	}
	got := []float32{gxx, gxy, gyx, gyy, gtx, gty}
	want := []float32{xx, xy, yx, yy, tx, ty}
	for i := range got {
		if !approxEq32(got[i], want[i], 1e-5) {
			t.Fatalf("TransformAffine() = %v, want %v", got, want)
		}
	}
}

// Rotate post-multiplies, so order matters: Translate-then-Rotate
// spins about the translated origin, Rotate-then-Translate moves in
// rotated units.
func TestRotateCompositionOrder(t *testing.T) {
	var a DrawContext
	a.Translate(10, 20)
	a.Rotate(float32(math.Pi / 2))
	ax, ay := a.xf.apply(1, 0)
	if !approxEq32(ax, 10, 1e-5) || !approxEq32(ay, 21, 1e-5) {
		t.Errorf("Translate;Rotate maps (1,0) to (%v,%v), want (10,21)", ax, ay)
	}

	var b DrawContext
	b.Rotate(float32(math.Pi / 2))
	b.Translate(10, 20)
	bx, by := b.xf.apply(1, 0)
	if !approxEq32(bx, -20, 1e-5) || !approxEq32(by, 11, 1e-5) {
		t.Errorf("Rotate;Translate maps (1,0) to (%v,%v), want (-20,11)", bx, by)
	}

	var c DrawContext
	c.ScaleBy(2, 2)
	c.Rotate(float32(math.Pi / 2))
	cx, cy := c.xf.apply(1, 0)
	if !approxEq32(cx, 0, 1e-5) || !approxEq32(cy, 2, 1e-5) {
		t.Errorf("Scale;Rotate maps (1,0) to (%v,%v), want (0,2)", cx, cy)
	}
}

// Geometry still rides: the batch carries the matrix and the vertices
// stay local. The four-float accessor declines a rotated batch.
func TestRotateStampsAffine(t *testing.T) {
	var dc DrawContext
	dc.Rotate(float32(math.Pi / 2))
	dc.FilledRect(1, 0, 2, 1, Blue)
	if len(dc.batches) != 1 {
		t.Fatalf("batches = %d, want 1", len(dc.batches))
	}
	b := dc.batches[0]
	if !b.hasXform {
		t.Fatal("rotated batch carries no transform")
	}
	wantAffine(t, b, 0, -1, 1, 0, 0, 0)
	if _, _, _, _, ok := b.Transform(); ok {
		t.Error("Transform() ok = true for a rotated batch")
	}
	if !reflect.DeepEqual(b.Triangles,
		[]float32{1, 0, 3, 0, 3, 1, 1, 0, 3, 1, 1, 1}) {
		t.Errorf("triangles baked: %v", b.Triangles)
	}
}

// An untransformed batch stays one: TransformAffine declines it.
func TestTransformAffineUntransformed(t *testing.T) {
	var dc DrawContext
	dc.FilledRect(1, 2, 3, 4, Blue)
	if _, _, _, _, _, _, ok := dc.batches[0].TransformAffine(); ok {
		t.Error("TransformAffine() ok = true for an untransformed batch")
	}
}

// A rotation change breaks the run-length merge like any other
// transform change. The merge keys on the live batch's matrix, so
// returning to an older one still opens a fresh batch.
func TestRotateBreaksBatchMerge(t *testing.T) {
	var dc DrawContext
	dc.FilledRect(0, 0, 1, 1, Blue)
	dc.Save()
	dc.Rotate(0.5)
	dc.FilledRect(0, 0, 1, 1, Blue)
	if len(dc.batches) != 2 {
		t.Fatalf("after Rotate: batches = %d, want 2", len(dc.batches))
	}
	dc.FilledRect(1, 0, 1, 1, Blue)
	if len(dc.batches) != 2 {
		t.Fatalf("same rotation: batches = %d, want 2", len(dc.batches))
	}
	dc.Restore()
	dc.FilledRect(2, 0, 1, 1, Blue)
	if len(dc.batches) != 3 {
		t.Fatalf("after Restore to identity: batches = %d, want 3", len(dc.batches))
	}
	if dc.batches[2].hasXform {
		t.Error("batch after a full Restore still carries a matrix")
	}
}

// Restoring all the way back leaves no stamp: Rotate(0) is the
// identity, and so is a rotation that unwinds.
func TestRotateFullRestoreIsUntransformed(t *testing.T) {
	var dc DrawContext
	dc.Save()
	dc.Rotate(0.5)
	dc.Restore()
	dc.FilledRect(0, 0, 1, 1, Blue)
	if dc.batches[0].hasXform {
		t.Error("batch after a full Restore still carries a matrix")
	}
}

// A non-finite angle must not poison the matrix.
func TestRotateRejectsNonFinite(t *testing.T) {
	for _, bad := range []float32{
		float32(math.NaN()), float32(math.Inf(1)), float32(math.Inf(-1)),
	} {
		var dc DrawContext
		dc.Translate(3, 4)
		before := dc.xf
		dc.Rotate(bad)
		if dc.xf != before {
			t.Errorf("Rotate(%v) changed xf to %+v, want %+v held",
				bad, dc.xf, before)
		}
	}
}

// Rotated text maps its anchor through the full matrix and gains the
// angle on top of the style's own.
func TestRotateTextAngle(t *testing.T) {
	var dc DrawContext
	dc.Translate(10, 20)
	dc.Rotate(float32(math.Pi / 2))
	dc.Text(1, 0, "hi", TextStyle{Size: 10, RotationRadians: 0.25})
	if len(dc.texts) != 1 {
		t.Fatalf("texts = %d, want 1", len(dc.texts))
	}
	e := dc.texts[0]
	if !approxEq32(e.X, 10, 1e-5) || !approxEq32(e.Y, 21, 1e-5) {
		t.Errorf("position = %v,%v, want 10,21", e.X, e.Y)
	}
	if !approxEq32(e.Style.RotationRadians, float32(math.Pi/2)+0.25, 1e-5) {
		t.Errorf("angle = %v, want pi/2+0.25", e.Style.RotationRadians)
	}
	// Column lengths are 1: sizes pass through.
	if e.Style.Size != 10 {
		t.Errorf("Size = %v, want 10", e.Style.Size)
	}
}

// A mirroring scale is not rotation: text keeps angle zero.
func TestMirrorScaleKeepsTextAngle(t *testing.T) {
	var dc DrawContext
	dc.ScaleBy(-1, 1)
	dc.Text(2, 0, "hi", TextStyle{Size: 10})
	e := dc.texts[0]
	if e.X != -2 || e.Y != 0 {
		t.Errorf("position = %v,%v, want -2,0", e.X, e.Y)
	}
	if e.Style.RotationRadians != 0 {
		t.Errorf("angle = %v, want 0 under a mirror", e.Style.RotationRadians)
	}
}

// An explicit affine composes with the CTM's linear part: the anchor
// already moved through the full matrix, so the stored affine must
// not carry the translation again.
func TestRotateTextAffineComposes(t *testing.T) {
	var dc DrawContext
	dc.Rotate(float32(math.Pi / 2))
	user := glyph.AffineTransform{XX: 2, YY: 1}
	dc.Text(0, 0, "hi", TextStyle{Size: 10, AffineTransform: &user})
	if len(dc.texts) != 1 {
		t.Fatalf("texts = %d, want 1", len(dc.texts))
	}
	got := dc.texts[0].Style.AffineTransform
	if got == nil {
		t.Fatal("composed affine is nil")
	}
	// User scales x by 2, CTM rotates 90°: (1,0) -> (2,0) -> (0,2).
	gx, gy := got.Apply(1, 0)
	if !approxEq32(gx, 0, 1e-5) || !approxEq32(gy, 2, 1e-5) {
		t.Errorf("composed affine maps (1,0) to (%v,%v), want (0,2)", gx, gy)
	}
	if got.X0 != 0 || got.Y0 != 0 {
		t.Errorf("composed affine carries translation (%v,%v)", got.X0, got.Y0)
	}
	// The caller's affine must come back untouched.
	if user != (glyph.AffineTransform{XX: 2, YY: 1}) {
		t.Errorf("caller affine mutated: %+v", user)
	}
}

// A rotated image records its mapped corner, mapped size and angle.
func TestRotateImageFrame(t *testing.T) {
	var dc DrawContext
	dc.Translate(10, 10)
	dc.Rotate(float32(math.Pi / 2))
	dc.Image(1, 0, 4, 2, "a.png", Opt[float32]{}, Color{})
	if len(dc.images) != 1 {
		t.Fatalf("images = %d, want 1", len(dc.images))
	}
	im := dc.images[0]
	if !approxEq32(im.X, 10, 1e-5) || !approxEq32(im.Y, 11, 1e-5) {
		t.Errorf("corner = %v,%v, want 10,11", im.X, im.Y)
	}
	if !approxEq32(im.W, 4, 1e-5) || !approxEq32(im.H, 2, 1e-5) {
		t.Errorf("size = %vx%v, want 4x2", im.W, im.H)
	}
	if !approxEq32(im.rotRad, float32(math.Pi/2), 1e-5) {
		t.Errorf("angle = %v, want pi/2", im.rotRad)
	}
}

// A rotated CTM with a mirror (negative determinant) must still cover
// the mapped rect. The backend bracket turns the image about its
// anchor and extends it along the angle and its clockwise
// perpendicular, so the anchor has to be the mapped corner those two
// directions grow away from — not always the mapped (x, y).
func TestRotateMirroredImageCoversMappedRect(t *testing.T) {
	cases := []struct {
		name   string
		rot    float32
		sx, sy float32
	}{
		{"quarter-turn y-mirror", float32(math.Pi / 2), 1, -1},
		{"tiny-turn x-mirror", 1e-3, -1, 1},
		{"tiny-turn y-mirror", 1e-3, 1, -1},
		{"oblique x-mirror", 0.7, -2, 3},
	}
	for _, tc := range cases {
		var dc DrawContext
		dc.Rotate(tc.rot)
		dc.ScaleBy(tc.sx, tc.sy)
		dc.Image(0, 0, 10, 20, "a.png", Opt[float32]{}, Color{})
		if len(dc.images) != 1 {
			t.Fatalf("%s: images = %d, want 1", tc.name, len(dc.images))
		}
		im := dc.images[0]
		// Corners of the drawn rect: anchor + W along the angle and H
		// along its clockwise perpendicular, as the bracket draws it.
		c, s := float32(math.Cos(float64(im.rotRad))), float32(math.Sin(float64(im.rotRad)))
		drawn := [8]float32{
			im.X, im.Y,
			im.X + im.W*c, im.Y + im.W*s,
			im.X + im.W*c - im.H*s, im.Y + im.W*s + im.H*c,
			im.X - im.H*s, im.Y + im.H*c,
		}
		bx, by, bw, bh := dc.xf.xfBBox(0, 0, 10, 20)
		minX := min(drawn[0], drawn[2], drawn[4], drawn[6])
		minY := min(drawn[1], drawn[3], drawn[5], drawn[7])
		maxX := max(drawn[0], drawn[2], drawn[4], drawn[6])
		maxY := max(drawn[1], drawn[3], drawn[5], drawn[7])
		if !approxEq32(minX, bx, 1e-3) || !approxEq32(minY, by, 1e-3) ||
			!approxEq32(maxX, bx+bw, 1e-3) || !approxEq32(maxY, by+bh, 1e-3) {
			t.Errorf("%s: drawn bbox %v,%v..%v,%v, want %v,%v..%v,%v", tc.name,
				minX, minY, maxX, maxY, bx, by, bx+bw, by+bh)
		}
	}
}

// A mirror with a negligible turn must draw upright, like the same
// mirror with no turn at all: the angle may not jump by π because the
// mirror flipped the first column.
func TestRotateMirrorStaysUpright(t *testing.T) {
	for _, m := range [][2]float32{{-1, 1}, {1, -1}} {
		var dc DrawContext
		dc.Rotate(1e-3)
		dc.ScaleBy(m[0], m[1])
		dc.Text(0, 0, "hi", TextStyle{Size: 10})
		dc.Image(0, 0, 10, 20, "a.png", Opt[float32]{}, Color{})
		if a := dc.texts[0].Style.RotationRadians; !approxEq32(a, 0, 1e-2) {
			t.Errorf("mirror %v: text angle = %v, want ≈0", m, a)
		}
		if a := dc.images[0].rotRad; !approxEq32(a, 0, 1e-2) {
			t.Errorf("mirror %v: image angle = %v, want ≈0", m, a)
		}
	}
}

// Two non-uniform scales that compose to a uniform one keep the
// circle fast paths, as the old sx == sy check did.
func TestScaleByRecomposedUniformIsUniform(t *testing.T) {
	var dc DrawContext
	dc.ScaleBy(2, 3)
	if dc.xf.uniform() {
		t.Fatal("2x3 scale reported uniform")
	}
	dc.ScaleBy(1.5, 1)
	if !dc.xf.uniform() {
		t.Error("3x3 scale reported non-uniform")
	}
}

// An explicit affine under Rotate+ScaleBy must not scale the glyphs
// twice: Size already carries the column length, so the composed
// affine may add the rotation only.
func TestRotateTextAffineNoDoubleScale(t *testing.T) {
	var dc DrawContext
	dc.Rotate(float32(math.Pi / 2))
	dc.ScaleBy(2, 2)
	user := glyph.AffineTransform{XX: 1, YY: 1, XY: 0.5}
	dc.Text(0, 0, "hi", TextStyle{Size: 10, AffineTransform: &user})
	if len(dc.texts) != 1 {
		t.Fatalf("texts = %d, want 1", len(dc.texts))
	}
	st := dc.texts[0].Style
	if !approxEq32(st.Size, 20, 1e-4) {
		t.Errorf("size = %v, want 20", st.Size)
	}
	// User maps (1,0) to (1,0); the rotation alone takes it to (0,1).
	gx, gy := st.AffineTransform.Apply(1, 0)
	if !approxEq32(gx, 0, 1e-5) || !approxEq32(gy, 1, 1e-5) {
		t.Errorf("composed affine maps (1,0) to (%v,%v), want (0,1)", gx, gy)
	}
}

// Rotation plus a uniform scale keeps the lowered quad: a rotated
// circle is still a circle, with its center mapped.
func TestRotateGradientQuad(t *testing.T) {
	g := &CanvasGradient{
		Radial: true,
		Stops:  []GradientStop{{Pos: 0, Color: Blue}, {Pos: 1, Color: Green}},
	}
	var dc DrawContext
	dc.Translate(10, 10)
	dc.Rotate(float32(math.Pi / 2))
	dc.ScaleBy(2, 2)
	dc.FilledCircleGradient(5, 5, 4, g)
	if len(dc.Gradients()) != 1 {
		t.Fatalf("gradients = %d, want 1 (lowered)", len(dc.Gradients()))
	}
	e := dc.Gradients()[0]
	// M = T*R*S maps the center (5,5) to (0,20); r = 8.
	if !approxEq32(e.X, -8, 1e-4) || !approxEq32(e.Y, 12, 1e-4) ||
		!approxEq32(e.W, 16, 1e-4) || !approxEq32(e.H, 16, 1e-4) {
		t.Errorf("quad = %v,%v %vx%v, want -8,12 16x16", e.X, e.Y, e.W, e.H)
	}
}

// Rotation plus a non-uniform scale is an ellipse: back to the mesh.
func TestRotateNonUniformGradientMesh(t *testing.T) {
	g := &CanvasGradient{
		Radial: true,
		Stops:  []GradientStop{{Pos: 0, Color: Blue}, {Pos: 1, Color: Green}},
	}
	var dc DrawContext
	dc.Rotate(float32(math.Pi / 4))
	dc.ScaleBy(2, 3)
	dc.FilledCircleGradient(5, 5, 4, g)
	if len(dc.Gradients()) != 0 {
		t.Errorf("gradients = %d, want 0 (mesh fallback)", len(dc.Gradients()))
	}
	if len(dc.Batches()) == 0 {
		t.Error("rotation + non-uniform scale produced no ring mesh")
	}
}

// A rotated rect has no axis-aligned recorder form: it records as the
// mapped outline polygon.
func TestRotateRecorderRectBecomesPolygon(t *testing.T) {
	rec := &xformCapture{}
	var dc DrawContext
	dc.SetRecorder(rec)
	dc.Rotate(float32(math.Pi / 2))
	dc.FilledRect(1, 0, 2, 1, Blue)
	if rec.rect != nil {
		t.Errorf("got FilledRect %v under rotation", rec.rect)
	}
	want := []float32{0, 1, 0, 3, -1, 3, -1, 1}
	if len(rec.fpoly) != len(want) {
		t.Fatalf("polygon = %v, want 4 points", rec.fpoly)
	}
	for i := range want {
		if !approxEq32(rec.fpoly[i], want[i], 1e-5) {
			t.Fatalf("polygon = %v, want %v", rec.fpoly, want)
		}
	}

	dc.Rect(1, 0, 2, 1, Blue, 2)
	if len(rec.joined) != 10 {
		t.Fatalf("stroked outline = %v, want closed 5-point loop", rec.joined)
	}
	if !approxEq32(rec.joined[0], rec.joined[8], 1e-5) ||
		!approxEq32(rec.joined[1], rec.joined[9], 1e-5) {
		t.Errorf("outline not closed: %v", rec.joined)
	}
}

// A rotated rounded rect strokes as one closed loop with no
// zero-length segment: the outline is closed exactly once.
func TestRotateRecorderRoundedRectClosesOnce(t *testing.T) {
	rec := &xformCapture{}
	var dc DrawContext
	dc.SetRecorder(rec)
	dc.Rotate(0.5)
	for _, radius := range []float32{2, 0} {
		dc.RoundedRect(0, 0, 10, 6, radius, Blue, 1)
		p := rec.joined
		if len(p) < 10 {
			t.Fatalf("radius %v: outline = %v, want a closed loop", radius, p)
		}
		n := len(p)
		if !approxEq32(p[0], p[n-2], 1e-5) || !approxEq32(p[1], p[n-1], 1e-5) {
			t.Errorf("radius %v: outline not closed: %v", radius, p)
		}
		for i := 2; i+1 < n; i += 2 {
			if p[i] == p[i-2] && p[i+1] == p[i-1] {
				t.Errorf("radius %v: duplicate point at %d: %v", radius, i/2, p)
				break
			}
		}
	}
}

// A uniform rotation maps circles to circles, so the recorder keeps
// the circle form with a mapped center.
func TestRotateRecorderCircleStaysCircle(t *testing.T) {
	rec := &xformCapture{}
	var dc DrawContext
	dc.SetRecorder(rec)
	dc.Translate(10, 10)
	dc.Rotate(0.5)
	dc.ScaleBy(2, 2)
	dc.FilledCircle(1, 1, 4, Blue)
	if rec.fpoly != nil {
		t.Errorf("uniform rotation flattened the circle: %v", rec.fpoly)
	}
	if len(rec.circle) != 3 {
		t.Fatalf("circle = %v, want [cx cy r]", rec.circle)
	}
	cx, cy := dc.xf.apply(1, 1)
	if !approxEq32(rec.circle[0], cx, 1e-5) ||
		!approxEq32(rec.circle[1], cy, 1e-5) {
		t.Errorf("center = %v, want mapped (%v,%v)", rec.circle[:2], cx, cy)
	}
	if !approxEq32(rec.circle[2], 8, 1e-4) {
		t.Errorf("radius = %v, want 8", rec.circle[2])
	}
}

// A rotated ellipse has no axis-aligned recorder form: it flattens to
// a polygon at the arc tolerance.
func TestRotateRecorderArcFlattens(t *testing.T) {
	rec := &xformCapture{}
	var dc DrawContext
	dc.SetRecorder(rec)
	dc.Rotate(0.5)
	dc.FilledArc(0, 0, 4, 2, 0, float32(math.Pi), Blue)
	if rec.arc != nil {
		t.Errorf("got FilledArc %v under rotation", rec.arc)
	}
	if len(rec.fpoly) < 8 {
		t.Fatalf("flattened pie = %v, want center + arc", rec.fpoly)
	}
	// Pie starts at the mapped center.
	if !approxEq32(rec.fpoly[0], 0, 1e-5) || !approxEq32(rec.fpoly[1], 0, 1e-5) {
		t.Errorf("pie does not start at the center: %v", rec.fpoly[:2])
	}
}

// A rotated clip has no scissor form: ImageClipped bakes it to the
// bounding box of the mapped rect.
func TestRotateImageClipIsBoundingBox(t *testing.T) {
	var dc DrawContext
	dc.Rotate(float32(math.Pi / 4))
	dc.ImageClipped(0, 0, 10, 10, "a.png", Opt[float32]{}, Color{}, 0, 0, 2, 2)
	if len(dc.images) != 1 {
		t.Fatalf("images = %d, want 1", len(dc.images))
	}
	im := dc.images[0]
	// Corners (0,0) (√2,√2) (0,2√2) (-√2,√2): bbox x -√2..√2, y 0..2√2.
	r2 := float32(math.Sqrt2)
	if !approxEq32(im.ClipX, -r2, 1e-5) || !approxEq32(im.ClipY, 0, 1e-5) ||
		!approxEq32(im.ClipW, 2*r2, 1e-5) || !approxEq32(im.ClipH, 2*r2, 1e-5) {
		t.Errorf("clip = %v,%v %vx%v, want bbox of the mapped rect",
			im.ClipX, im.ClipY, im.ClipW, im.ClipH)
	}
}

// A rotated non-uniform scale shears a circle into a rotated ellipse,
// which no circle or axis-aligned arc can express: it flattens to a
// mapped polygon.
func TestRotateRecorderShearedCircleFlattens(t *testing.T) {
	rec := &xformCapture{}
	var dc DrawContext
	dc.SetRecorder(rec)
	dc.Rotate(0.5)
	dc.ScaleBy(2, 1)
	dc.FilledCircle(0, 0, 4, Blue)
	if rec.circle != nil || rec.arc != nil {
		t.Errorf("sheared circle kept an axis form: circle=%v arc=%v",
			rec.circle, rec.arc)
	}
	if len(rec.fpoly) < 8 {
		t.Fatalf("polygon = %v, want flattened ellipse", rec.fpoly)
	}
	// The first arc point (4,0) maps through the full matrix.
	wx, wy := dc.xf.apply(4, 0)
	if !approxEq32(rec.fpoly[0], wx, 1e-4) || !approxEq32(rec.fpoly[1], wy, 1e-4) {
		t.Errorf("first point = %v,%v, want %v,%v",
			rec.fpoly[0], rec.fpoly[1], wx, wy)
	}
}

// The rotated recorder outlines build into pooled scratch: once the
// buffers are warm, stroking and filling under rotation allocates
// nothing.
func TestRotateRecorderOutlinesDoNotAllocate(t *testing.T) {
	var dc DrawContext
	dc.SetRecorder(nopRecorder{})
	dc.Rotate(0.5)
	draw := func() {
		dc.Rect(0, 0, 10, 6, Blue, 1)
		dc.RoundedRect(0, 0, 10, 6, 2, Blue, 1)
		dc.FilledArc(0, 0, 4, 2, 0, 2, Blue)
	}
	draw()
	if n := testing.AllocsPerRun(50, draw); n != 0 {
		t.Errorf("allocs per run = %v, want 0", n)
	}
}
