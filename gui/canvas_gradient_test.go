package gui

import (
	"math"
	"testing"
	"time"
)

func gradStops() []GradientStop {
	return []GradientStop{
		{Color: RGBA(255, 0, 0, 255), Pos: 0},
		{Color: RGBA(0, 0, 255, 255), Pos: 1},
	}
}

func almostEqF(a, b, eps float32) bool {
	d := a - b
	if d < 0 {
		d = -d
	}
	return d <= eps
}

func TestCanvasGradientTLinear(t *testing.T) {
	g := &CanvasGradient{X1: 0, Y1: 0, X2: 100, Y2: 0}
	cases := []struct {
		x, y float32
		want float32
	}{
		{0, 0, 0},
		{50, 0, 0.5},
		{100, 0, 1},
		{150, 0, 1.5},  // raw t is unclamped; spread decides
		{-50, 0, -0.5}, //
		{50, 999, 0.5}, // perpendicular offset does not move t
	}
	for _, c := range cases {
		if got := canvasGradientT(c.x, c.y, g); !almostEqF(got, c.want, 1e-5) {
			t.Errorf("canvasGradientT(%v,%v) = %v, want %v",
				c.x, c.y, got, c.want)
		}
	}
}

func TestCanvasGradientTDegenerateLinear(t *testing.T) {
	// Coincident endpoints have no direction; t must fold to 0 rather
	// than produce a NaN that would poison every vertex color.
	g := &CanvasGradient{X1: 5, Y1: 5, X2: 5, Y2: 5}
	if got := canvasGradientT(99, 99, g); got != 0 {
		t.Errorf("degenerate linear t = %v, want 0", got)
	}
}

func TestCanvasGradientTRadial(t *testing.T) {
	g := &CanvasGradient{Radial: true, CX: 10, CY: 10, FX: 10, FY: 10, R: 20}
	if got := canvasGradientT(10, 10, g); got != 0 {
		t.Errorf("center t = %v, want 0", got)
	}
	if got := canvasGradientT(30, 10, g); !almostEqF(got, 1, 1e-5) {
		t.Errorf("rim t = %v, want 1", got)
	}
	if got := canvasGradientT(50, 10, g); !almostEqF(got, 2, 1e-5) {
		t.Errorf("outside t = %v, want 2 (raw, unclamped)", got)
	}
}

func TestCanvasGradientTRadialFocal(t *testing.T) {
	// The focal point, not the center, is where t == 0.
	g := &CanvasGradient{Radial: true, CX: 0, CY: 0, FX: 5, FY: 0, R: 10}
	if got := canvasGradientT(5, 0, g); got != 0 {
		t.Errorf("focal t = %v, want 0", got)
	}
	if got := canvasGradientT(0, 0, g); !almostEqF(got, 0.5, 1e-5) {
		t.Errorf("center t = %v, want 0.5", got)
	}
}

func TestCanvasGradientTBadRadius(t *testing.T) {
	for _, r := range []float32{0, -1,
		float32(math.NaN()), float32(math.Inf(1))} {
		g := &CanvasGradient{Radial: true, R: r}
		if got := canvasGradientT(3, 4, g); got != 0 {
			t.Errorf("R=%v: t = %v, want 0", r, got)
		}
	}
}

func TestApplyCanvasSpread(t *testing.T) {
	cases := []struct {
		name   string
		spread GradientSpread
		in     float32
		want   float32
	}{
		{"pad below", SpreadPad, -0.5, 0},
		{"pad inside", SpreadPad, 0.25, 0.25},
		{"pad above", SpreadPad, 2, 1},
		{"repeat wraps", SpreadRepeat, 1.25, 0.25},
		{"repeat negative", SpreadRepeat, -0.25, 0.75},
		{"reflect even period", SpreadReflect, 0.25, 0.25},
		{"reflect odd period", SpreadReflect, 1.25, 0.75},
		// Reflect is an even function: -0.25 folds onto +0.25.
		{"reflect negative", SpreadReflect, -0.25, 0.25},
	}
	for _, c := range cases {
		got := applyCanvasSpread(c.in, c.spread)
		if !almostEqF(got, c.want, 1e-5) {
			t.Errorf("%s: applyCanvasSpread(%v) = %v, want %v",
				c.name, c.in, got, c.want)
		}
	}
	for _, bad := range []float32{
		float32(math.NaN()), float32(math.Inf(1)), float32(math.Inf(-1))} {
		if got := applyCanvasSpread(bad, SpreadReflect); got != 0 {
			t.Errorf("non-finite %v: got %v, want 0", bad, got)
		}
	}
}

func TestResolveCanvasGradientDefaults(t *testing.T) {
	// A radial with no radius fits the bounds: centered, R = half the
	// larger extent, focal collapsed onto the center.
	g := resolveCanvasGradient(CanvasGradient{Radial: true},
		0, 0, 100, 40)
	if g.CX != 50 || g.CY != 20 {
		t.Errorf("center = (%v,%v), want (50,20)", g.CX, g.CY)
	}
	if g.R != 50 {
		t.Errorf("R = %v, want 50", g.R)
	}
	if g.FX != g.CX || g.FY != g.CY {
		t.Errorf("focal = (%v,%v), want center", g.FX, g.FY)
	}

	// A linear with coincident endpoints runs top to bottom.
	l := resolveCanvasGradient(CanvasGradient{}, 10, 20, 30, 60)
	if l.Y1 != 20 || l.Y2 != 60 || l.X1 != l.X2 {
		t.Errorf("linear default = (%v,%v)-(%v,%v), want vertical 20..60",
			l.X1, l.Y1, l.X2, l.Y2)
	}

	// Explicit geometry is left alone.
	e := resolveCanvasGradient(
		CanvasGradient{X1: 1, Y1: 2, X2: 3, Y2: 4}, 0, 0, 100, 100)
	if e.X1 != 1 || e.Y1 != 2 || e.X2 != 3 || e.Y2 != 4 {
		t.Error("explicit linear geometry was overwritten")
	}
}

func TestCanvasStopIsolinesPad(t *testing.T) {
	stops := []GradientStop{
		{Pos: 0}, {Pos: 0.3}, {Pos: 0.7}, {Pos: 1},
	}
	// Geometry entirely inside [0,1]: only the interior stops break.
	got := canvasStopIsolines(stops, SpreadPad, 0, 1, nil)
	if len(got) != 2 || !almostEqF(got[0], 0.3, 1e-6) ||
		!almostEqF(got[1], 0.7, 1e-6) {
		t.Errorf("inside range isolines = %v, want [0.3 0.7]", got)
	}
	// Geometry overhanging both ends adds the two clamp boundaries,
	// which are real slope changes for pad.
	got = canvasStopIsolines(stops, SpreadPad, -1, 2, nil)
	if len(got) != 4 || got[0] != 0 || got[3] != 1 {
		t.Errorf("overhanging isolines = %v, want [0 0.3 0.7 1]", got)
	}
}

func TestCanvasStopIsolinesRepeatTilesPeriods(t *testing.T) {
	stops := []GradientStop{{Pos: 0}, {Pos: 0.5}, {Pos: 1}}
	got := canvasStopIsolines(stops, SpreadRepeat, 0, 3, nil)
	// Each period contributes its own breakpoints: the sawtooth's step
	// at every integer, plus the mid stop inside each period.
	want := []float32{0.5, 1, 1.5, 2, 2.5}
	if len(got) != len(want) {
		t.Fatalf("repeat isolines = %v, want %v", got, want)
	}
	for i := range want {
		if !almostEqF(got[i], want[i], 1e-6) {
			t.Fatalf("repeat isolines = %v, want %v", got, want)
		}
	}
}

func TestCanvasStopIsolinesBounded(t *testing.T) {
	stops := make([]GradientStop, 16)
	for i := range stops {
		stops[i].Pos = float32(i) / 15
	}
	got := canvasStopIsolines(stops, SpreadRepeat, 0, 1e6, nil)
	if len(got) > maxCanvasStopIsolines {
		t.Errorf("isolines = %d, want at most %d",
			len(got), maxCanvasStopIsolines)
	}
}

func TestSubdivideCanvasGradientRadialSplits(t *testing.T) {
	// One big triangle against a small radius must come back split:
	// a radial ramp is not affine, so long edges would linearize it.
	tris := []float32{0, 0, 100, 0, 0, 100}
	g := &CanvasGradient{Radial: true, CX: 50, CY: 50, FX: 50, FY: 50, R: 50}
	var scratch, radial, iso []float32
	out := subdivideCanvasGradientTris(tris, g, &scratch, &radial, &iso)
	if len(out) <= len(tris) {
		t.Fatalf("radial subdivision produced %d floats, want > %d",
			len(out), len(tris))
	}
	if len(out)%6 != 0 {
		t.Errorf("subdivided length %d is not a whole number of triangles",
			len(out))
	}
	// The depth cap bounds the split: 4^6 leaves per source triangle.
	maxTris := 1
	for range maxCanvasRadialDepth {
		maxTris *= 4
	}
	if len(out)/6 > maxTris {
		t.Errorf("produced %d triangles, above the depth cap of %d",
			len(out)/6, maxTris)
	}
}

func TestSubdivideCanvasGradientLinearTwoStopsIsNoop(t *testing.T) {
	// Between two stops a linear ramp is exactly reproduced by vertex
	// interpolation, so there is nothing to split and nothing to pay.
	tris := []float32{0, 0, 100, 0, 0, 100}
	g := &CanvasGradient{X1: 0, Y1: 0, X2: 100, Y2: 0,
		Stops: gradStops()}
	var scratch, radial, iso []float32
	out := subdivideCanvasGradientTris(tris, g, &scratch, &radial, &iso)
	if len(out) != len(tris) {
		t.Errorf("two-stop linear split to %d floats, want %d",
			len(out), len(tris))
	}
}

func TestSubdivideCanvasGradientLinearSplitsAtStops(t *testing.T) {
	tris := []float32{0, 0, 100, 0, 0, 100}
	g := &CanvasGradient{X1: 0, Y1: 0, X2: 100, Y2: 0, Stops: []GradientStop{
		{Color: RGB(255, 0, 0), Pos: 0},
		{Color: RGB(0, 255, 0), Pos: 0.5},
		{Color: RGB(0, 0, 255), Pos: 1},
	}}
	var scratch, radial, iso []float32
	out := subdivideCanvasGradientTris(tris, g, &scratch, &radial, &iso)
	if len(out) <= len(tris) {
		t.Errorf("three-stop linear did not split: %d floats", len(out))
	}
}

func TestFillTrianglesGradientProducesVertexColors(t *testing.T) {
	dc := NewDrawContext(100, 100, nil)
	tris := []float32{0, 0, 100, 0, 0, 100}
	dc.FillTrianglesGradient(tris, &CanvasGradient{
		X1: 0, Y1: 0, X2: 100, Y2: 0, Stops: gradStops(),
	})
	if len(dc.batches) != 1 {
		t.Fatalf("got %d batches, want 1", len(dc.batches))
	}
	b := dc.batches[0]
	if len(b.VertexColors)*2 != len(b.Triangles) {
		t.Fatalf("VertexColors=%d, Triangles=%d: lengths must agree",
			len(b.VertexColors), len(b.Triangles))
	}
	// The vertex at t=0 takes the first stop, the one at t=1 the last.
	if b.VertexColors[0] != (gradStops()[0].Color) {
		t.Errorf("vertex 0 = %v, want the t=0 stop", b.VertexColors[0])
	}
	if b.VertexColors[1] != (gradStops()[1].Color) {
		t.Errorf("vertex 1 = %v, want the t=1 stop", b.VertexColors[1])
	}
}

func TestFillTrianglesGradientNoops(t *testing.T) {
	cases := []struct {
		name string
		tris []float32
		g    *CanvasGradient
	}{
		{"nil gradient", []float32{0, 0, 1, 0, 0, 1}, nil},
		{"no stops", []float32{0, 0, 1, 0, 0, 1}, &CanvasGradient{}},
		{"empty tris", nil, &CanvasGradient{Stops: gradStops()}},
		{"partial triangle", []float32{0, 0, 1, 0},
			&CanvasGradient{Stops: gradStops()}},
	}
	for _, c := range cases {
		dc := NewDrawContext(100, 100, nil)
		dc.FillTrianglesGradient(c.tris, c.g)
		if len(dc.batches) != 0 {
			t.Errorf("%s: got %d batches, want 0", c.name, len(dc.batches))
		}
	}
}

// TestGradientBatchNeverMergesWithFlat is the invariant the whole
// design rests on: a batch is flat or gradient, never both, so the
// per-batch relation len(VertexColors)*2 == len(Triangles) holds and
// validSvgCmd can enforce it downstream.
func TestGradientBatchNeverMergesWithFlat(t *testing.T) {
	dc := NewDrawContext(100, 100, nil)
	red := RGB(255, 0, 0)
	g := &CanvasGradient{X1: 0, Y1: 0, X2: 10, Y2: 0, Stops: gradStops()}

	dc.FilledRect(0, 0, 10, 10, red)
	dc.FilledRectGradient(0, 0, 10, 10, g)
	dc.FilledRect(20, 0, 10, 10, red) // same color as the first flat fill
	dc.FilledRectGradient(0, 0, 10, 10, g)

	if len(dc.batches) != 4 {
		t.Fatalf("got %d batches, want 4 (no merging across a gradient)",
			len(dc.batches))
	}
	wantGradient := []bool{false, true, false, true}
	for i, b := range dc.batches {
		isGrad := len(b.VertexColors) > 0
		if isGrad != wantGradient[i] {
			t.Errorf("batch %d gradient=%v, want %v", i, isGrad,
				wantGradient[i])
		}
		if isGrad && len(b.VertexColors)*2 != len(b.Triangles) {
			t.Errorf("batch %d: VertexColors=%d, Triangles=%d",
				i, len(b.VertexColors), len(b.Triangles))
		}
		if !isGrad && b.VertexColors != nil {
			t.Errorf("batch %d: flat batch carries vertex colors", i)
		}
	}
}

// TestFlatFillAfterGradientStartsNewBatch guards the merge test
// specifically: without the batchIsGradient gate the flat fill would
// append into the gradient batch and break its length relation.
func TestFlatFillAfterGradientStartsNewBatch(t *testing.T) {
	dc := NewDrawContext(100, 100, nil)
	g := &CanvasGradient{X1: 0, Y1: 0, X2: 10, Y2: 0, Stops: gradStops()}
	dc.FilledRectGradient(0, 0, 10, 10, g)
	mid := dc.batches[0].Color
	// Draw flat in exactly the color the gradient batch recorded, which
	// is what a naive lastColor comparison would merge on.
	dc.FilledRect(0, 0, 10, 10, mid)
	if len(dc.batches) != 2 {
		t.Fatalf("got %d batches, want 2", len(dc.batches))
	}
	b := dc.batches[0]
	if len(b.VertexColors)*2 != len(b.Triangles) {
		t.Errorf("gradient batch corrupted: VertexColors=%d, Triangles=%d",
			len(b.VertexColors), len(b.Triangles))
	}
}

func TestGradientFillsProduceGeometry(t *testing.T) {
	g := &CanvasGradient{Radial: true, Stops: gradStops()}
	cases := []struct {
		name string
		draw func(dc *DrawContext)
	}{
		{"rect", func(dc *DrawContext) {
			dc.FilledRectGradient(0, 0, 40, 40, g)
		}},
		{"circle", func(dc *DrawContext) {
			dc.FilledCircleGradient(20, 20, 20, g)
		}},
		{"arc", func(dc *DrawContext) {
			dc.FilledArcGradient(20, 20, 20, 10, 0, 1, g)
		}},
		{"polygon", func(dc *DrawContext) {
			dc.FilledPolygonGradient([]float32{0, 0, 40, 0, 40, 40, 0, 40}, g)
		}},
		{"rounded rect", func(dc *DrawContext) {
			dc.FilledRoundedRectGradient(0, 0, 40, 40, 8, g)
		}},
		{"rounded rect, zero radius", func(dc *DrawContext) {
			dc.FilledRoundedRectGradient(0, 0, 40, 40, 0, g)
		}},
	}
	for _, c := range cases {
		dc := NewDrawContext(100, 100, nil)
		c.draw(dc)
		if len(dc.batches) != 1 {
			t.Errorf("%s: got %d batches, want 1", c.name, len(dc.batches))
			continue
		}
		b := dc.batches[0]
		if len(b.Triangles) == 0 || len(b.Triangles)%6 != 0 {
			t.Errorf("%s: %d triangle floats", c.name, len(b.Triangles))
		}
		if len(b.VertexColors)*2 != len(b.Triangles) {
			t.Errorf("%s: VertexColors=%d, Triangles=%d", c.name,
				len(b.VertexColors), len(b.Triangles))
		}
	}
}

func TestGradientFillsRejectDegenerateSize(t *testing.T) {
	g := &CanvasGradient{Stops: gradStops()}
	dc := NewDrawContext(100, 100, nil)
	dc.FilledRectGradient(0, 0, 0, 10, g)
	dc.FilledRectGradient(0, 0, 10, -1, g)
	dc.FilledRoundedRectGradient(0, 0, 0, 10, 4, g)
	dc.FilledPolygonGradient([]float32{0, 0, 1, 1}, g)
	if len(dc.batches) != 0 {
		t.Errorf("got %d batches, want 0", len(dc.batches))
	}
}

// gradientRecorderStub implements the optional extension.
type gradientRecorderStub struct {
	nopRecorder
	calls int
	tris  int
}

func (r *gradientRecorderStub) FillTrianglesGradient(
	tris []float32, _ *CanvasGradient) {
	r.calls++
	r.tris = len(tris)
}

func TestGradientRecorderExtensionReceivesGeometry(t *testing.T) {
	dc := NewDrawContext(100, 100, nil)
	rec := &gradientRecorderStub{}
	dc.SetRecorder(rec)
	dc.FilledCircleGradient(20, 20, 10,
		&CanvasGradient{Radial: true, Stops: gradStops()})
	if rec.calls != 1 {
		t.Fatalf("FillTrianglesGradient called %d times, want 1", rec.calls)
	}
	if rec.tris == 0 || rec.tris%6 != 0 {
		t.Errorf("recorder got %d triangle floats", rec.tris)
	}
	if len(dc.batches) != 0 {
		t.Error("a recorded fill must not also tessellate into a batch")
	}
}

// flatRecorderStub implements only DrawRecorder, so gradient fills must
// degrade to the equivalent flat primitive rather than vanish.
type flatRecorderStub struct {
	nopRecorder
	arcs   int
	rects  int
	polys  int
	rounds int
	color  Color
}

func (r *flatRecorderStub) FilledArc(_, _, _, _, _, _ float32, c Color) {
	r.arcs++
	r.color = c
}
func (r *flatRecorderStub) FilledRect(_, _, _, _ float32, c Color) {
	r.rects++
	r.color = c
}
func (r *flatRecorderStub) FilledPolygon(_ []float32, c Color) {
	r.polys++
	r.color = c
}
func (r *flatRecorderStub) FilledRoundedRect(_, _, _, _, _ float32, c Color) {
	r.rounds++
	r.color = c
}

func TestGradientFallsBackToFlatRecorder(t *testing.T) {
	dc := NewDrawContext(100, 100, nil)
	rec := &flatRecorderStub{}
	dc.SetRecorder(rec)
	g := &CanvasGradient{Radial: true, Stops: gradStops()}

	dc.FilledCircleGradient(20, 20, 10, g)
	dc.FilledRectGradient(0, 0, 10, 10, g)
	dc.FilledPolygonGradient([]float32{0, 0, 10, 0, 10, 10}, g)
	dc.FilledRoundedRectGradient(0, 0, 10, 10, 2, g)

	if rec.arcs != 1 || rec.rects != 1 || rec.polys != 1 || rec.rounds != 1 {
		t.Errorf("flat fallback counts: arc=%d rect=%d poly=%d round=%d, "+
			"want 1 each", rec.arcs, rec.rects, rec.polys, rec.rounds)
	}
	if len(dc.batches) != 0 {
		t.Error("a recorded fill must not also tessellate into a batch")
	}
	// The fallback shade is the ramp midpoint, not either endpoint.
	mid := SampleGradientStopColor(gradStops(), 0.5)
	if rec.color != mid {
		t.Errorf("fallback color = %v, want the midpoint %v", rec.color, mid)
	}
}

func TestFillTrianglesGradientFlatRecorderFallback(t *testing.T) {
	dc := NewDrawContext(100, 100, nil)
	rec := &flatRecorderStub{}
	dc.SetRecorder(rec)
	dc.FillTrianglesGradient([]float32{
		0, 0, 10, 0, 0, 10,
		0, 0, 10, 0, 10, 10,
	}, &CanvasGradient{Stops: gradStops()})
	if rec.polys != 2 {
		t.Errorf("recorded %d polygons, want 2 (one per triangle)", rec.polys)
	}
}

// triWinding reports how many triangles in a flat list wind each way.
func triWinding(tris []float32) (ccw, cw int) {
	for i := 0; i+5 < len(tris); i += 6 {
		d := (tris[i+2]-tris[i])*(tris[i+5]-tris[i+1]) -
			(tris[i+4]-tris[i])*(tris[i+3]-tris[i+1])
		if d < 0 {
			cw++
		} else {
			ccw++
		}
	}
	return ccw, cw
}

// TestGradientSubdivisionPreservesWinding is the regression test for
// the defect that made a glow render as spokes radiating from its hub.
//
// The isoline splitter sorts a triangle's vertices by gradient
// parameter, and an odd permutation reverses the winding. The software
// rasterizer takes a whole batch as one path and accumulates *signed*
// coverage, so a mesh of mixed winding cancels itself out along every
// seam. Uniform winding in, uniform winding out.
func TestGradientSubdivisionPreservesWinding(t *testing.T) {
	// Many stops, so every triangle is cut repeatedly.
	stops := make([]GradientStop, 10)
	for i := range stops {
		stops[i] = GradientStop{
			Color: RGBA(255, 200, 80, uint8(255-i*25)),
			Pos:   float32(i) / 9,
		}
	}
	cases := []struct {
		name string
		draw func(dc *DrawContext)
	}{
		{"radial circle", func(dc *DrawContext) {
			dc.FilledCircleGradient(130, 130, 114,
				&CanvasGradient{Radial: true, Stops: stops})
		}},
		{"radial rect", func(dc *DrawContext) {
			dc.FilledRectGradient(16, 16, 228, 228, &CanvasGradient{
				Radial: true, CX: 130, CY: 130, FX: 130, FY: 130, R: 114,
				Stops: stops,
			})
		}},
		{"linear rect", func(dc *DrawContext) {
			dc.FilledRectGradient(16, 16, 228, 228, &CanvasGradient{
				X1: 16, Y1: 16, X2: 244, Y2: 244, Stops: stops,
			})
		}},
		{"reflect spread", func(dc *DrawContext) {
			dc.FilledRectGradient(16, 16, 228, 228, &CanvasGradient{
				X1: 100, Y1: 0, X2: 160, Y2: 0,
				Spread: SpreadReflect, Stops: stops,
			})
		}},
	}
	for _, c := range cases {
		dc := NewDrawContext(260, 260, nil)
		// Every source primitive here tessellates counter-clockwise,
		// so the whole output must.
		c.draw(dc)
		tris := dc.Batches()[0].Triangles
		ccw, cw := triWinding(tris)
		if cw != 0 {
			t.Errorf("%s: %d of %d triangles wound the wrong way",
				c.name, cw, ccw+cw)
		}
	}
}

// TestRadialFanCostsNoExtraSubdivision pins the reason the split
// criterion measures curvature rather than edge length: a fan struck
// from the gradient's own center is already radially aligned, and an
// edge-length rule would shatter it into thousands of triangles it
// gains nothing from.
func TestRadialFanCostsNoExtraSubdivision(t *testing.T) {
	g := &CanvasGradient{Radial: true, Stops: gradStops()}
	dcFlat := NewDrawContext(260, 260, nil)
	dcFlat.FilledCircle(130, 130, 114, RGB(255, 255, 255))
	fanTris := len(dcFlat.Batches()[0].Triangles) / 6

	dc := NewDrawContext(260, 260, nil)
	dc.FilledCircleGradient(130, 130, 114, g)
	got := len(dc.Batches()[0].Triangles) / 6

	// Two stops means no isolines either, so this must be the bare fan.
	if got != fanTris {
		t.Errorf("two-stop radial fan = %d triangles, want the flat fan's "+
			"%d", got, fanTris)
	}
}

// TestCanvasStopIsolinesHugeParameterTerminates pins the guard on the
// tiling loop. Before it the loop counted periods in a float64, and
// past 2^53 that cannot hold consecutive integers: k++ stopped
// advancing and the loop never reached its bound. A gradient whose
// endpoints are a hair apart against geometry of ordinary size reaches
// those parameters — 100 units across a 1e-14 gradient is 1e16.
//
// The test hangs rather than fails if the guard regresses, so it runs
// under a deadline.
func TestCanvasStopIsolinesHugeParameterTerminates(t *testing.T) {
	done := make(chan int, 1)
	go func() {
		got := canvasStopIsolines(gradStops(), SpreadRepeat,
			1e16, 1e16+10, nil)
		done <- len(got)
	}()
	select {
	case n := <-done:
		if n > maxCanvasStopIsolines {
			t.Errorf("isolines = %d, want at most %d",
				n, maxCanvasStopIsolines)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("canvasStopIsolines did not terminate at 1e16")
	}
}

// TestCanvasStopIsolinesPadCapped covers the Pad branch's ceiling. It
// emits one breakpoint per interior stop, and the split pass rescans
// the whole list at every node of its recursion, so an unbounded stop
// count costs there and not only here.
func TestCanvasStopIsolinesPadCapped(t *testing.T) {
	stops := make([]GradientStop, 4000)
	for i := range stops {
		stops[i].Pos = float32(i+1) / float32(len(stops)+2)
	}
	got := canvasStopIsolines(stops, SpreadPad, 0, 1, nil)
	if len(got) > maxCanvasStopIsolines {
		t.Errorf("pad isolines = %d, want at most %d",
			len(got), maxCanvasStopIsolines)
	}
	if len(got) == 0 {
		t.Error("pad isolines = 0, want the cap's worth")
	}
}

// TestSplitCanvasTriAtStopsStopsAtBudget exercises the budget itself:
// once the batch has produced more geometry than any fill can use, the
// triangle in flight is emitted whole rather than split further.
func TestSplitCanvasTriAtStopsStopsAtBudget(t *testing.T) {
	stops := []GradientStop{
		{Color: RGBA(255, 0, 0, 255), Pos: 0},
		{Color: RGBA(0, 255, 0, 255), Pos: 0.5},
		{Color: RGBA(0, 0, 255, 255), Pos: 1},
	}
	g := &CanvasGradient{X1: 0, Y1: 0, X2: 100, Y2: 0, Stops: stops}
	iso := canvasStopIsolines(stops, SpreadPad, 0, 1, nil)
	if len(iso) == 0 {
		t.Fatal("no isolines; the triangle below would not split anyway")
	}
	// Below the budget the triangle splits.
	var fresh []float32
	splitCanvasTriAtStops(0, 0, 100, 0, 0, 100, g, iso, 0, &fresh)
	if len(fresh) <= 6 {
		t.Fatalf("split below budget produced %d floats, want > 6",
			len(fresh))
	}
	// At the budget the same triangle comes through whole.
	full := make([]float32, maxCanvasSplitFloats)
	splitCanvasTriAtStops(0, 0, 100, 0, 0, 100, g, iso, 0, &full)
	if got := len(full) - maxCanvasSplitFloats; got != 6 {
		t.Errorf("split at budget appended %d floats, want 6 (unsplit)",
			got)
	}
}

// TestSubdivideCanvasGradientComposesBounded is the composition guard:
// the radial pass quadruples the batch per level and the isoline pass
// then splits every leaf up to three ways per cut. Each cap alone
// bounds one factor, and this pins their product for the worst input a
// real fill produces — a large rect, filled radially, with a stop list
// far past anything a designer would write.
func TestSubdivideCanvasGradientComposesBounded(t *testing.T) {
	// A rect filled radially is the worst geometry for the radial pass
	// (its two triangles bulge right across the isolines), and 200
	// stops give the isoline pass the most cuts it will take.
	tris := []float32{
		0, 0, 400, 0, 400, 400,
		0, 0, 400, 400, 0, 400,
	}
	stops := make([]GradientStop, 200)
	for i := range stops {
		stops[i] = GradientStop{
			Color: RGBA(uint8(i), 0, 0, 255),
			Pos:   float32(i) / float32(len(stops)-1),
		}
	}
	g := &CanvasGradient{Radial: true, CX: 200, CY: 200,
		FX: 200, FY: 200, R: 200, Stops: stops}
	var scratch, radial, iso []float32
	out := subdivideCanvasGradientTris(tris, g, &scratch, &radial, &iso)
	if len(out)%6 != 0 {
		t.Errorf("subdivided length %d is not a whole number of triangles",
			len(out))
	}
	// Comfortably inside the budget, which is the claim: the two passes
	// compose to something bounded on their own for realistic input,
	// and the budget is the backstop for input that is not.
	if len(out) > maxCanvasSplitFloats {
		t.Errorf("subdivision produced %d floats, above the budget %d",
			len(out), maxCanvasSplitFloats)
	}
	if len(out) <= len(tris) {
		t.Errorf("subdivision produced %d floats, want a real split",
			len(out))
	}
}

// TestRadialSplitDepthBacksOffOnLargeMeshes covers the other half of
// the budget. Depth is chosen from the worst triangle's curvature, but
// every level quadruples the *whole* batch, so a mesh that is already
// dense must not take the depth a single triangle would earn.
func TestRadialSplitDepthBacksOffOnLargeMeshes(t *testing.T) {
	g := &CanvasGradient{Radial: true, CX: 0, CY: 0, FX: 0, FY: 0, R: 10}
	// One badly-aligned triangle, repeated until the batch is large.
	// Each copy is offset so none of them is degenerate.
	var tris []float32
	const copies = 5000
	for i := range copies {
		o := float32(i) * 0.001
		tris = append(tris, -100+o, -100, 100+o, -100, 0+o, 100)
	}
	depth := radialSplitDepth(tris, g)
	if got := copies << (2 * depth); got > maxCanvasRadialTris {
		t.Errorf("depth %d on %d triangles expands to %d, above %d",
			depth, copies, got, maxCanvasRadialTris)
	}
	// The same triangle on its own earns a real depth, so the backoff
	// above is the mesh size talking and not a broken criterion.
	if solo := radialSplitDepth(tris[:6], g); solo <= depth {
		t.Errorf("single triangle depth %d, batch depth %d: want the "+
			"single triangle to earn more", solo, depth)
	}
}

// TestCutFractionRadialLandsOnIsoline is why the radial cut solves a
// quadratic instead of interpolating. The parameter is a distance, so
// it is not affine along an edge: an interpolated cut misses the
// isoline, the recursion sees the piece still straddling it and cuts
// again, and neighbours that cut at different places leave a seam.
func TestCutFractionRadialLandsOnIsoline(t *testing.T) {
	g := &CanvasGradient{Radial: true, CX: 0, CY: 0, FX: 0, FY: 0, R: 100}
	// A chord well off-center, where distance is at its least linear.
	x0, y0 := float32(-80), float32(60)
	x1, y1 := float32(80), float32(60)
	t0 := canvasGradientT(x0, y0, g)
	t1 := canvasGradientT(x1, y1, g)
	const tS = 0.7
	f := cutFraction(x0, y0, x1, y1, t0, t1, tS, g)
	cx := x0 + f*(x1-x0)
	cy := y0 + f*(y1-y0)
	if got := canvasGradientT(cx, cy, g); !almostEqF(got, tS, 1e-4) {
		t.Errorf("cut at f=%v has t=%v, want %v", f, got, tS)
	}
}

// TestCutFractionLinearIsExact is the other half: for a linear gradient
// the parameter is affine, so the plain ratio is already exact and the
// quadratic path must not be taken.
func TestCutFractionLinearIsExact(t *testing.T) {
	g := &CanvasGradient{X1: 0, Y1: 0, X2: 100, Y2: 0}
	f := cutFraction(0, 0, 100, 0, 0, 1, 0.25, g)
	if !almostEqF(f, 0.25, 1e-6) {
		t.Errorf("linear cut fraction = %v, want 0.25", f)
	}
	// A zero-length segment has no answer; the midpoint is the
	// convention, and it must not divide by zero.
	if f = cutFraction(5, 5, 5, 5, 0.3, 0.3, 0.5, g); f != 0.5 {
		t.Errorf("degenerate segment cut fraction = %v, want 0.5", f)
	}
}

// TestGradientAndFlatFillsShareGeometry is why appendRoundedRectTris
// and its siblings exist: a gradient fill must put its vertices exactly
// where its flat twin does, so switching a fill to a gradient changes
// only how it is colored and never its silhouette. Two tessellations
// that merely look alike drift the first time one is tuned.
func TestGradientAndFlatFillsShareGeometry(t *testing.T) {
	cases := []struct {
		name string
		flat func(*DrawContext)
		grad func(*DrawContext)
	}{
		{
			name: "rect",
			flat: func(dc *DrawContext) {
				dc.FilledRect(5, 7, 40, 30, RGBA(1, 2, 3, 255))
			},
			grad: func(dc *DrawContext) {
				dc.FilledRectGradient(5, 7, 40, 30,
					&CanvasGradient{Stops: gradStops()})
			},
		},
		{
			name: "circle",
			flat: func(dc *DrawContext) {
				dc.FilledCircle(30, 30, 20, RGBA(1, 2, 3, 255))
			},
			grad: func(dc *DrawContext) {
				dc.FilledCircleGradient(30, 30, 20,
					&CanvasGradient{Stops: gradStops()})
			},
		},
		{
			name: "rounded-rect",
			flat: func(dc *DrawContext) {
				dc.FilledRoundedRect(5, 7, 40, 30, 8, RGBA(1, 2, 3, 255))
			},
			grad: func(dc *DrawContext) {
				dc.FilledRoundedRectGradient(5, 7, 40, 30, 8,
					&CanvasGradient{Stops: gradStops()})
			},
		},
		{
			name: "polygon",
			flat: func(dc *DrawContext) {
				dc.FilledPolygon([]float32{0, 0, 30, 0, 30, 20, 0, 20},
					RGBA(1, 2, 3, 255))
			},
			grad: func(dc *DrawContext) {
				dc.FilledPolygonGradient(
					[]float32{0, 0, 30, 0, 30, 20, 0, 20},
					&CanvasGradient{Stops: gradStops()})
			},
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			flatDC := NewDrawContext(100, 100, nil)
			tc.flat(flatDC)
			gradDC := NewDrawContext(100, 100, nil)
			tc.grad(gradDC)

			flatTris := flatDC.Batches()[0].Triangles
			gradTris := gradDC.Batches()[0].Triangles
			// A two-stop linear gradient over a flat ramp needs no
			// split, so the vertex lists must match exactly.
			if len(flatTris) != len(gradTris) {
				t.Fatalf("flat has %d floats, gradient %d",
					len(flatTris), len(gradTris))
			}
			for i := range flatTris {
				if flatTris[i] != gradTris[i] {
					t.Fatalf("vertex float %d: flat %v, gradient %v",
						i, flatTris[i], gradTris[i])
				}
			}
		})
	}
}

// TestFilledRoundedRectGradientZeroRadiusIsARect covers the delegation
// branch: a radius clamped away leaves a plain rectangle, and it must
// still be a gradient fill rather than falling through to nothing.
func TestFilledRoundedRectGradientZeroRadiusIsARect(t *testing.T) {
	dc := NewDrawContext(100, 100, nil)
	dc.FilledRoundedRectGradient(5, 5, 40, 30, 0,
		&CanvasGradient{Stops: gradStops()})
	rect := NewDrawContext(100, 100, nil)
	rect.FilledRectGradient(5, 5, 40, 30, &CanvasGradient{Stops: gradStops()})

	got := dc.Batches()
	want := rect.Batches()
	if len(got) != 1 || len(want) != 1 {
		t.Fatalf("batches: rounded %d, rect %d", len(got), len(want))
	}
	if len(got[0].VertexColors) == 0 {
		t.Error("zero-radius rounded rect lost its vertex colors")
	}
	if len(got[0].Triangles) != len(want[0].Triangles) {
		t.Errorf("rounded produced %d floats, plain rect %d",
			len(got[0].Triangles), len(want[0].Triangles))
	}
}

// TestTriBoundsDegenerate pins the empty and single-vertex paths, which
// resolveCanvasGradient divides by when it defaults a radius.
func TestTriBoundsDegenerate(t *testing.T) {
	if x0, y0, x1, y1 := triBounds(nil); x0 != 0 || y0 != 0 ||
		x1 != 0 || y1 != 0 {
		t.Errorf("triBounds(nil) = %v,%v,%v,%v, want zeros",
			x0, y0, x1, y1)
	}
	// An odd trailing float is ignored rather than read past.
	x0, y0, x1, y1 := triBounds([]float32{3, 4, 9})
	if x0 != 3 || y0 != 4 || x1 != 3 || y1 != 4 {
		t.Errorf("triBounds(short) = %v,%v,%v,%v, want 3,4,3,4",
			x0, y0, x1, y1)
	}
}
