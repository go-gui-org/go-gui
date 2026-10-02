package gui

import (
	"math"
	"testing"
)

// Canvas paths (#906): a reusable builder filled with the nonzero /
// even-odd rules and stroked through the shared stroker. These tests
// pin the issue's core complaint — FilledPolygon fans from the first
// vertex and is correct only for convex shapes — plus the builder,
// the gradient twin, the recorder paths and the steady-state
// allocation behavior.

// pathArea sums the unsigned area of a flat triangle list.
func pathArea(tris []float32) float32 {
	total := float32(0)
	for i := 0; i+5 < len(tris); i += 6 {
		ax, ay := tris[i], tris[i+1]
		bx, by := tris[i+2], tris[i+3]
		cx, cy := tris[i+4], tris[i+5]
		total += f32Abs((bx-ax)*(cy-ay)-(cx-ax)*(by-ay)) / 2
	}
	return total
}

// collectPathTris draws into a fresh context and flattens every
// batch into one triangle list.
func collectPathTris(draw func(dc *DrawContext)) []float32 {
	return collectStrokeTris(draw)
}

// lPath builds an L-shaped concave path: area 300.
func lPath() *CanvasPath {
	var p CanvasPath
	p.MoveTo(0, 0)
	p.LineTo(20, 0)
	p.LineTo(20, 10)
	p.LineTo(10, 10)
	p.LineTo(10, 20)
	p.LineTo(0, 20)
	p.Close()
	return &p
}

// A fan from the first vertex would cover the missing corner
// (10..20, 10..20). The path fill must not.
func TestFillPath_ConcaveL(t *testing.T) {
	p := lPath()
	tris := collectPathTris(func(dc *DrawContext) {
		dc.FillPath(p, Red, FillNonzero)
	})
	if got := pathArea(tris); f32Abs(got-300) > 1 {
		t.Fatalf("area = %f, want 300", got)
	}
	if !strokeCovers(tris, 5, 5) || !strokeCovers(tris, 5, 15) {
		t.Fatal("both L arms must be covered")
	}
	if strokeCovers(tris, 15, 15) {
		t.Fatal("missing corner (15,15) must be uncovered")
	}
}

// Two contours, outer CCW and inner CW, carve a hole under nonzero:
// 100 − 16 = 84.
func TestFillPath_HoleNonzero(t *testing.T) {
	var p CanvasPath
	p.MoveTo(0, 0)
	p.LineTo(10, 0)
	p.LineTo(10, 10)
	p.LineTo(0, 10)
	p.Close()
	p.MoveTo(3, 3)
	p.LineTo(3, 7)
	p.LineTo(7, 7)
	p.LineTo(7, 3)
	p.Close()
	tris := collectPathTris(func(dc *DrawContext) {
		dc.FillPath(&p, Red, FillNonzero)
	})
	if got := pathArea(tris); f32Abs(got-84) > 1 {
		t.Fatalf("area = %f, want 84", got)
	}
	if strokeCovers(tris, 5, 5) {
		t.Fatal("hole center (5,5) must be uncovered")
	}
	if !strokeCovers(tris, 1, 1) {
		t.Fatal("outer-only point (1,1) must be covered")
	}
}

// Evenodd carves the overlap of two same-winding squares (150);
// nonzero fills everything (175).
func TestFillPath_EvenOddSquares(t *testing.T) {
	square := func(p *CanvasPath, x, y float32) {
		p.MoveTo(x, y)
		p.LineTo(x+10, y)
		p.LineTo(x+10, y+10)
		p.LineTo(x, y+10)
		p.Close()
	}
	var pe CanvasPath
	square(&pe, 0, 0)
	square(&pe, 5, 5)
	triE := collectPathTris(func(dc *DrawContext) {
		dc.FillPath(&pe, Red, FillEvenOdd)
	})
	if got := pathArea(triE); f32Abs(got-150) > 1 {
		t.Fatalf("evenodd area = %f, want 150", got)
	}
	var pn CanvasPath
	square(&pn, 0, 0)
	square(&pn, 5, 5)
	triN := collectPathTris(func(dc *DrawContext) {
		dc.FillPath(&pn, Red, FillNonzero)
	})
	if got := pathArea(triN); f32Abs(got-175) > 1 {
		t.Fatalf("nonzero area = %f, want 175", got)
	}
}

// An arch from one quadratic: the region under the curve fills, the
// region above it does not. The apex sits near y=2.5.
func TestFillPath_QuadArch(t *testing.T) {
	var p CanvasPath
	p.MoveTo(0, 10)
	p.QuadTo(5, -5, 10, 10)
	p.Close()
	tris := collectPathTris(func(dc *DrawContext) {
		dc.FillPath(&p, Red, FillNonzero)
	})
	if !strokeCovers(tris, 5, 5) {
		t.Fatal("point under the arch (5,5) must be covered")
	}
	if strokeCovers(tris, 5, 0) {
		t.Fatal("point above the arch (5,0) must be uncovered")
	}
}

// An area under a cubic, the Lines workload shape: endpoints at
// y=10, controls at y=0, closed along the chord.
func TestFillPath_CubicArea(t *testing.T) {
	var p CanvasPath
	p.MoveTo(0, 10)
	p.CubicTo(0, 0, 10, 0, 10, 10)
	p.Close()
	tris := collectPathTris(func(dc *DrawContext) {
		dc.FillPath(&p, Red, FillNonzero)
	})
	if !strokeCovers(tris, 5, 5) {
		t.Fatal("point under the curve (5,5) must be covered")
	}
	if strokeCovers(tris, 5, 0) {
		t.Fatal("point above the curve (5,0) must be uncovered")
	}
}

// A quarter-disc pie from ArcTo: covers the first quadrant, nothing
// else. Area near pi*100/4.
func TestFillPath_ArcPie(t *testing.T) {
	var p CanvasPath
	p.MoveTo(0, 0)
	p.ArcTo(0, 0, 10, 10, 0, float32(math.Pi/2))
	p.Close()
	tris := collectPathTris(func(dc *DrawContext) {
		dc.FillPath(&p, Red, FillNonzero)
	})
	if got := pathArea(tris); f32Abs(got-78.5) > 2 {
		t.Fatalf("area = %f, want ~78.5", got)
	}
	if !strokeCovers(tris, 5, 5) {
		t.Fatal("pie interior (5,5) must be covered")
	}
	if strokeCovers(tris, -5, 5) || strokeCovers(tris, 5, -5) {
		t.Fatal("outside the sweep must be uncovered")
	}
}

// An unclosed contour fills as if closed: same area with and
// without Close.
func TestFillPath_OpenContourImplicitClose(t *testing.T) {
	build := func(close bool) []float32 {
		var p CanvasPath
		p.MoveTo(0, 0)
		p.LineTo(10, 0)
		p.LineTo(5, 8)
		if close {
			p.Close()
		}
		return collectPathTris(func(dc *DrawContext) {
			dc.FillPath(&p, Red, FillNonzero)
		})
	}
	open, closed := build(false), build(true)
	if f32Abs(pathArea(open)-pathArea(closed)) > 1e-3 {
		t.Fatalf("open area %f != closed area %f",
			pathArea(open), pathArea(closed))
	}
}

// A MoveTo-only path draws nothing, and Close with no contour is a
// no-op rather than a corrupt contour.
func TestFillPath_MoveToOnlyDrawsNothing(t *testing.T) {
	var p CanvasPath
	p.MoveTo(3, 4)
	p.Close()
	dc := DrawContext{Width: 100, Height: 100}
	dc.FillPath(&p, Red, FillNonzero)
	dc.StrokePath(&p, Red, 2, StrokeStyle{})
	if len(dc.Batches()) != 0 {
		t.Fatalf("batches = %d, want 0", len(dc.Batches()))
	}
}

// Nil, empty and non-finite paths draw nothing and never panic.
func TestFillPath_DegenerateInputs(t *testing.T) {
	nan := float32(math.NaN())
	cases := map[string]*CanvasPath{
		"nil":   nil,
		"empty": {},
	}
	bad := &CanvasPath{}
	bad.MoveTo(0, 0)
	bad.LineTo(nan, 5)
	bad.LineTo(5, 5)
	cases["nonfinite"] = bad
	for name, p := range cases {
		dc := DrawContext{Width: 100, Height: 100}
		dc.FillPath(p, Red, FillNonzero)
		dc.FillPathGradient(p, &CanvasGradient{
			Stops: []GradientStop{{Color: Red, Pos: 0}}},
			FillNonzero)
		dc.StrokePath(p, Red, 2, StrokeStyle{})
		if len(dc.Batches()) != 0 {
			t.Fatalf("%s: batches = %d, want 0",
				name, len(dc.Batches()))
		}
	}
}

// Bad stroke widths draw nothing.
func TestStrokePath_BadWidth(t *testing.T) {
	p := lPath()
	for _, w := range []float32{0, -2, float32(math.NaN()),
		float32(math.Inf(1))} {
		dc := DrawContext{Width: 100, Height: 100}
		dc.StrokePath(p, Red, w, StrokeStyle{})
		if len(dc.Batches()) != 0 {
			t.Fatalf("width %v: batches = %d, want 0",
				w, len(dc.Batches()))
		}
	}
}

// An open stroked line covers its band and ends flush (butt caps).
func TestStrokePath_OpenLine(t *testing.T) {
	var p CanvasPath
	p.MoveTo(0, 0)
	p.LineTo(20, 0)
	tris := collectPathTris(func(dc *DrawContext) {
		dc.StrokePath(&p, Red, 4, StrokeStyle{})
	})
	if !strokeCovers(tris, 10, 1) || !strokeCovers(tris, 10, -1) {
		t.Fatal("stroke band must be covered")
	}
	if strokeCovers(tris, 10, 3) {
		t.Fatal("outside the band must be uncovered")
	}
	if strokeCovers(tris, 22, 0) {
		t.Fatal("butt cap must end flush at (20,0)")
	}
}

// Round caps extend past the end points by half the width.
func TestStrokePath_RoundCaps(t *testing.T) {
	var p CanvasPath
	p.MoveTo(0, 0)
	p.LineTo(20, 0)
	tris := collectPathTris(func(dc *DrawContext) {
		dc.StrokePath(&p, Red, 4,
			StrokeStyle{Cap: StrokeRoundCap})
	})
	if !strokeCovers(tris, 21.5, 0) {
		t.Fatal("round cap must cover (21.5,0)")
	}
	if strokeCovers(tris, 23, 0) {
		t.Fatal("past the cap must be uncovered")
	}
}

// A closed stroked square paints its border and leaves the center
// alone.
func TestStrokePath_ClosedSquare(t *testing.T) {
	var p CanvasPath
	p.MoveTo(0, 0)
	p.LineTo(10, 0)
	p.LineTo(10, 10)
	p.LineTo(0, 10)
	p.Close()
	tris := collectPathTris(func(dc *DrawContext) {
		dc.StrokePath(&p, Red, 2, StrokeStyle{})
	})
	if !strokeCovers(tris, 5, 0.5) {
		t.Fatal("border must be covered")
	}
	if strokeCovers(tris, 5, 5) {
		t.Fatal("center must be uncovered")
	}
}

// The gradient twin tessellates identically and colors per vertex:
// one color per vertex, so 3 per triangle.
func TestFillPathGradient_MatchesFlat(t *testing.T) {
	p := lPath()
	stops := []GradientStop{
		{Color: Red, Pos: 0},
		{Color: Blue, Pos: 1},
	}
	flat := collectPathTris(func(dc *DrawContext) {
		dc.FillPath(p, Red, FillNonzero)
	})
	dc := DrawContext{Width: 100, Height: 100}
	dc.FillPathGradient(p, &CanvasGradient{Stops: stops}, FillNonzero)
	var gtris []float32
	var ncols int
	for _, b := range dc.Batches() {
		gtris = append(gtris, b.Triangles...)
		ncols += len(b.VertexColors)
	}
	if len(gtris) != len(flat) {
		t.Fatalf("gradient tris %d != flat tris %d",
			len(gtris), len(flat))
	}
	for i := range gtris {
		if gtris[i] != flat[i] {
			t.Fatalf("vertex %d differs: %f != %f",
				i, gtris[i], flat[i])
		}
	}
	if ncols*2 != len(gtris) {
		t.Fatalf("colors %d for %d floats, want one per vertex",
			ncols, len(gtris))
	}
}

// The fill rides the batch transform: geometry stays local, the
// matrix travels on the batch.
func TestFillPath_TransformBatch(t *testing.T) {
	p := lPath()
	dc := DrawContext{Width: 200, Height: 200}
	dc.Translate(10, 20)
	dc.FillPath(p, Red, FillNonzero)
	batches := dc.Batches()
	if len(batches) != 1 {
		t.Fatalf("batches = %d, want 1", len(batches))
	}
	sx, sy, tx, ty, ok := batches[0].Transform()
	if !ok || sx != 1 || sy != 1 || tx != 10 || ty != 20 {
		t.Fatalf("transform = %v,%v,%v,%v,%v, want 1,1,10,20,true",
			sx, sy, tx, ty, ok)
	}
	// Local geometry: the L spans 0..20, so the batch must too.
	for i := 0; i+1 < len(batches[0].Triangles); i += 2 {
		x, y := batches[0].Triangles[i], batches[0].Triangles[i+1]
		if x < 0 || x > 20 || y < 0 || y > 20 {
			t.Fatalf("vertex (%f,%f) is not local", x, y)
		}
	}
}

// pathStubRecorder is a plain DrawRecorder: it cannot express a
// path, so fills arrive as one polygon per tessellated triangle.
type pathStubRecorder struct {
	DrawRecorder
	polys [][]float32
	baked []float32
}

func (r *pathStubRecorder) FilledPolygon(points []float32, _ Color) {
	cp := append([]float32(nil), points...)
	r.polys = append(r.polys, cp)
	r.baked = append(r.baked, points...)
}

// Without the path extension, a fill degrades to its triangles and
// a stroke to joined polylines — nothing is dropped.
func TestFillPath_PlainRecorderFallback(t *testing.T) {
	p := lPath()
	rec := &pathStubRecorder{}
	dc := DrawContext{Width: 100, Height: 100}
	dc.SetRecorder(rec)
	dc.FillPath(p, Red, FillNonzero)
	if len(rec.polys) == 0 {
		t.Fatal("fallback must record triangles as polygons")
	}
	for _, poly := range rec.polys {
		if len(poly) != 6 {
			t.Fatalf("fallback polygon = %d floats, want 6", len(poly))
		}
	}
}

// A recorder with the path extension receives the path structured,
// with baked coordinates under a transform.
func TestFillPath_PathRecorderStructured(t *testing.T) {
	var p CanvasPath
	p.MoveTo(0, 0)
	p.LineTo(10, 0)
	p.LineTo(10, 10)
	p.Close()
	rec := &pathCapture{}
	dc := DrawContext{Width: 100, Height: 100}
	dc.SetRecorder(rec)
	dc.Translate(5, 7)
	dc.FillPath(&p, Red, FillEvenOdd)
	if rec.fill == nil {
		t.Fatal("path recorder must receive the fill")
	}
	if rec.rule != FillEvenOdd {
		t.Fatalf("rule = %d, want evenodd", rec.rule)
	}
	// Baked: the MoveTo (0,0) arrives as (5,7).
	if rec.fill.pts[0] != 5 || rec.fill.pts[1] != 7 {
		t.Fatalf("baked start = (%f,%f), want (5,7)",
			rec.fill.pts[0], rec.fill.pts[1])
	}
	dc.StrokePath(&p, Blue, 2, StrokeStyle{Cap: StrokeRoundCap})
	if rec.stroke == nil {
		t.Fatal("path recorder must receive the stroke")
	}
	if rec.strokeWidth != 2 {
		t.Fatalf("unscaled width = %f, want 2", rec.strokeWidth)
	}
}

// pathCapture speaks the path extension and keeps what arrives.
type pathCapture struct {
	DrawRecorder
	fill        *CanvasPath
	stroke      *CanvasPath
	rule        FillRule
	strokeWidth float32
}

func (r *pathCapture) FillPath(p *CanvasPath, _ Color, rule FillRule) {
	r.fill = p
	r.rule = rule
}

func (r *pathCapture) StrokePath(p *CanvasPath, _ Color, width float32,
	_ StrokeStyle) {
	r.stroke = p
	r.strokeWidth = width
}

// Reset clears a path for rebuild without reallocating: steady-state
// redraws of a rebuilt path allocate nothing.
func TestFillPath_RebuildDoesNotAllocate(t *testing.T) {
	var p CanvasPath
	var dc DrawContext
	build := func() {
		p.Reset()
		p.MoveTo(0, 0)
		p.LineTo(20, 0)
		p.LineTo(20, 10)
		p.LineTo(10, 10)
		p.LineTo(10, 20)
		p.LineTo(0, 20)
		p.Close()
		dc.FillPath(&p, Red, FillNonzero)
	}
	build()
	if n := testing.AllocsPerRun(50, build); n != 0 {
		t.Errorf("allocs per run = %v, want 0", n)
	}
}
