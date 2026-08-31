package gui

import (
	"math"
	"reflect"
	"testing"
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
	if dc.xf != (canvasXform{sx: 2, sy: 2, tx: 5, ty: 5}) {
		t.Fatalf("inner xf = %+v", dc.xf)
	}
	dc.Restore()
	if dc.xf != (canvasXform{sx: 1, sy: 1, tx: 5, ty: 5}) {
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
func TestXformPointsCap(t *testing.T) {
	var dc DrawContext
	dc.Translate(10, 10)
	dc.ScaleBy(2, 2)
	huge := make([]float32, (1<<20)+10)
	for i := range huge {
		huge[i] = float32(i)
	}
	// Must not panic or allocate a retained 4 MiB+ buffer; returns
	// the caller's slice unchanged when over cap.
	got := dc.xfPoints(huge)
	if &got[0] != &huge[0] {
		t.Error("xfPoints did not return the original slice for a huge input")
	}
	if got[0] != 0 || got[1] != 1 {
		t.Error("huge slice was mutated or baked despite cap")
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
}
