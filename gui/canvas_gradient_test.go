package gui

import (
	"testing"

	"github.com/go-gui-org/go-gui/gui/internal/gradmesh"
)

func gradStops() []GradientStop {
	return []GradientStop{
		{Color: RGBA(255, 0, 0, 255), Pos: 0},
		{Color: RGBA(0, 0, 255, 255), Pos: 1},
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

// TestGradSpreadMapping pins this side's enum conversion onto
// gradmesh's. The two enums agree numerically today, so a transposition
// in the switch would pass every fill test that names a spread by
// spelling it; only this mapping, and the svg side's mirror of it,
// would see the drift. Anything that is not reflect or repeat pads —
// the same default the coloring pass assumes, so cuts and colors cannot
// disagree about the ramp.
func TestGradSpreadMapping(t *testing.T) {
	cases := []struct {
		in   GradientSpread
		want gradmesh.Spread
	}{
		{SpreadPad, gradmesh.SpreadPad},
		{SpreadReflect, gradmesh.SpreadReflect},
		{SpreadRepeat, gradmesh.SpreadRepeat},
		{GradientSpread(99), gradmesh.SpreadPad},
	}
	for _, c := range cases {
		if got := gradSpread(c.in); got != c.want {
			t.Errorf("gradSpread(%v) = %v, want %v", c.in, got, c.want)
		}
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
			// Concentric, so FilledCircleGradient would lower it to a
			// shader quad. This file is about the tessellated fills;
			// the mesh path is what has geometry to check.
			dc.fillConcentricRings(20, 20, 20, g)
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
			dc.fillConcentricRings(130, 130, 114,
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
	dc.fillConcentricRings(130, 130, 114, g)
	got := len(dc.Batches()[0].Triangles) / 6

	// Two stops means no isolines either, so this must be the bare fan.
	if got != fanTris {
		t.Errorf("two-stop radial fan = %d triangles, want the flat fan's "+
			"%d", got, fanTris)
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

// TestFillTrianglesGradientRepeatFoldVertex is the canvas side of issue
// #417. A repeat gradient steps from the ramp's end back to its start at
// every integer of the raw parameter, and the split pass places cut
// vertices exactly on those integers. Reading such a vertex on its own
// gives it the ramp's *start* color while its triangle needs the limit
// from inside — the end color — which Gouraud then carries across the
// whole band.
//
// The geometry is fold-aligned on purpose: the gradient spans 0..30 and
// the triangle 0..90, so its far corner sits at exactly t=3 and the cuts
// land on t=1 and t=2 with no float luck deciding a side.
func TestFillTrianglesGradientRepeatFoldVertex(t *testing.T) {
	dc := NewDrawContext(100, 100, nil)
	tris := []float32{0, 0, 90, 0, 0, 90}
	dc.FillTrianglesGradient(tris, &CanvasGradient{
		X1: 0, Y1: 0, X2: 30, Y2: 0,
		Spread: SpreadRepeat, Stops: gradStops(),
	})
	if len(dc.batches) != 1 {
		t.Fatalf("got %d batches, want 1", len(dc.batches))
	}
	b := dc.batches[0]
	if len(b.VertexColors)*2 != len(b.Triangles) {
		t.Fatalf("VertexColors=%d, Triangles=%d: lengths must agree",
			len(b.VertexColors), len(b.Triangles))
	}
	first := gradStops()[0].Color
	last := gradStops()[1].Color
	// The split leaves no triangle spanning a fold, so every triangle
	// must carry the whole ramp's worth of variation the geometry gives
	// it — never three vertices of the ramp's start, which is what the
	// per-vertex read produced for a fold-aligned band.
	sawLast := false
	for i := 0; i+2 < len(b.VertexColors); i += 3 {
		allFirst := true
		for _, c := range b.VertexColors[i : i+3] {
			if c != first {
				allFirst = false
			}
			if c == last {
				sawLast = true
			}
		}
		if allFirst {
			t.Errorf("triangle %d: all three vertices took the ramp's "+
				"start color; a fold vertex read the wrong side of the "+
				"step", i/3)
		}
	}
	if !sawLast {
		t.Error("no vertex took the ramp's end color: the fold vertices " +
			"all resolved to the far side of the step")
	}
}
