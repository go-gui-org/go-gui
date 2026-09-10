package gui

import (
	"math"
	"testing"
)

// A fill recorded after a lowered radial gradient must not merge back
// into a batch that was opened before it: the batch is emitted first,
// so the later fill would paint under the glow it was drawn over.
func TestLoweredGradientBreaksBatchRun(t *testing.T) {
	dc := NewDrawContext(200, 200, nil)
	red := RGB(255, 0, 0)
	dc.FilledRect(0, 0, 10, 10, red)
	dc.FilledCircleGradient(50, 50, 20, &CanvasGradient{
		Radial: true,
		Stops: []GradientStop{
			{Color: RGB(0, 0, 255), Pos: 0},
			{Color: RGB(0, 255, 0), Pos: 1},
		},
	})
	dc.FilledRect(100, 100, 10, 10, red)

	if got := len(dc.Gradients()); got != 1 {
		t.Fatalf("gradients = %d, want 1 (fill did not lower)", got)
	}
	after := dc.Gradients()[0].afterBatch
	if len(dc.Batches()) < 2 {
		t.Fatalf("batches = %d, want 2: the second rect merged into the "+
			"batch opened before the gradient", len(dc.Batches()))
	}
	if after != 1 {
		t.Fatalf("afterBatch = %d, want 1", after)
	}
	// The rect drawn after the glow must live in a batch the emit walk
	// reaches after it.
	if len(dc.Batches()[1].Triangles) != 12 {
		t.Fatalf("batch 1 floats = %d, want 12 (one rect)",
			len(dc.Batches()[1].Triangles))
	}
}

// Dash geometry drove an unbounded loop: a non-finite or very long
// segment kept the pattern walk running for as long as the frame lived.
func TestDashedGeometryTerminates(t *testing.T) {
	inf := float32(math.Inf(1))
	cases := []struct {
		name            string
		x1, y1          float32
		dashLen, gapLen float32
	}{
		{"infinite endpoint", inf, 0, 4, 4},
		{"nan endpoint", float32(math.NaN()), 0, 4, 4},
		{"huge finite length", 1e30, 0, 1, 1},
		{"tiny pattern", 1e6, 0, 1e-6, 1e-6},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			dc := NewDrawContext(200, 200, nil)
			dc.DashedLine(0, 0, tc.x1, tc.y1, RGB(255, 0, 0), 1,
				tc.dashLen, tc.gapLen)
			pts := []float32{0, 0, tc.x1, tc.y1}
			dc.DashedPolyline(pts, RGB(255, 0, 0), 1, tc.dashLen, tc.gapLen)
		})
	}
}

// A non-finite vertex reaching a batch costs every primitive that
// merged into it, because validSvgCmd drops the whole command. Screen
// at record time so a bad primitive costs only itself.
func TestFlatPrimitivesScreenNonFinite(t *testing.T) {
	nan := float32(math.NaN())
	inf := float32(math.Inf(-1))
	red := RGB(255, 0, 0)

	draws := []struct {
		name string
		fn   func(dc *DrawContext)
	}{
		{"FilledRect", func(dc *DrawContext) {
			dc.FilledRect(nan, 0, 10, 10, red)
		}},
		{"Rect", func(dc *DrawContext) {
			dc.Rect(0, inf, 10, 10, red, 1)
		}},
		{"Line", func(dc *DrawContext) {
			dc.Line(0, 0, nan, 5, red, 1)
		}},
		{"Polyline", func(dc *DrawContext) {
			dc.Polyline([]float32{0, 0, 5, nan, 10, 10}, red, 1)
		}},
		{"FilledPolygon", func(dc *DrawContext) {
			dc.FilledPolygon([]float32{0, 0, 10, 0, inf, 10}, red)
		}},
		{"PolylineJoined", func(dc *DrawContext) {
			dc.PolylineJoined([]float32{0, 0, 5, 5, nan, 10}, red, 2)
		}},
		{"FillTrianglesColors", func(dc *DrawContext) {
			dc.FillTrianglesColors(
				[]float32{0, 0, 10, 0, 10, nan},
				[]Color{red, red, red})
		}},
	}
	for _, d := range draws {
		t.Run(d.name, func(t *testing.T) {
			dc := NewDrawContext(200, 200, nil)
			// A good fill first: it must survive the bad one that
			// follows, which is what merging used to break.
			dc.FilledRect(0, 0, 10, 10, red)
			d.fn(dc)
			for bi, b := range dc.Batches() {
				for i, v := range b.Triangles {
					if math.IsNaN(float64(v)) ||
						math.IsInf(float64(v), 0) {
						t.Fatalf("batch %d vertex %d is %v: the bad "+
							"primitive poisoned the batch", bi, i, v)
					}
				}
			}
			if len(dc.Batches()) == 0 ||
				len(dc.Batches()[0].Triangles) != 12 {
				t.Fatalf("the good rect was dropped: %d batches",
					len(dc.Batches()))
			}
		})
	}
}

// The recorder path bakes the transform into every coordinate. A point
// list past the old scratch cap was handed over untransformed, which
// silently misplaced it in an export.
func TestXformRecorderBakesLargePointList(t *testing.T) {
	// Past the cap the scratch buffer used to refuse, which handed the
	// recorder the caller's raw points.
	const n = (1 << 20) + 2
	pts := make([]float32, n)
	for i := 0; i+1 < n; i += 2 {
		pts[i], pts[i+1] = 1, 2
	}
	r := &capturingRecorder{}
	dc := NewDrawContext(100, 100, nil)
	dc.SetRecorder(r)
	dc.Translate(10, 20)
	dc.Polyline(pts, RGB(255, 0, 0), 1)

	if len(r.polyline) != n {
		t.Fatalf("recorder got %d floats, want %d", len(r.polyline), n)
	}
	if r.polyline[0] != 11 || r.polyline[1] != 22 {
		t.Fatalf("point 0 = (%v, %v), want (11, 22): the transform was "+
			"not baked", r.polyline[0], r.polyline[1])
	}
}

// capturingRecorder records the one call the test above asserts on and
// no-ops the rest of the interface.
type capturingRecorder struct {
	polyline []float32
}

func (c *capturingRecorder) Polyline(points []float32, _ Color, _ float32) {
	c.polyline = append(c.polyline[:0], points...)
}

func (c *capturingRecorder) Line(_, _, _, _ float32, _ Color, _ float32) {}
func (c *capturingRecorder) FilledRect(_, _, _, _ float32, _ Color)      {}
func (c *capturingRecorder) Rect(_, _, _, _ float32, _ Color, _ float32) {}
func (c *capturingRecorder) FilledCircle(_, _, _ float32, _ Color)       {}
func (c *capturingRecorder) Circle(_, _, _ float32, _ Color, _ float32)  {}
func (c *capturingRecorder) FilledArc(_, _, _, _, _, _ float32, _ Color) {}
func (c *capturingRecorder) Arc(_, _, _, _, _, _ float32, _ Color, _ float32) {
}
func (c *capturingRecorder) FilledPolygon(_ []float32, _ Color)               {}
func (c *capturingRecorder) FilledRoundedRect(_, _, _, _, _ float32, _ Color) {}
func (c *capturingRecorder) RoundedRect(_, _, _, _, _ float32, _ Color, _ float32) {
}
func (c *capturingRecorder) DashedLine(_, _, _, _ float32, _ Color, _, _, _ float32) {
}
func (c *capturingRecorder) DashedPolyline(_ []float32, _ Color, _, _, _ float32) {
}
func (c *capturingRecorder) PolylineJoined(_ []float32, _ Color, _ float32) {}
func (c *capturingRecorder) QuadBezier(_, _, _, _, _, _ float32, _ Color, _ float32) {
}
func (c *capturingRecorder) CubicBezier(_, _, _, _, _, _, _, _ float32, _ Color, _ float32) {
}
func (c *capturingRecorder) Text(_, _ float32, _ string, _ TextStyle) {}

// The stroked paths that still allocated per call must not: an animated
// canvas redraws them every frame.
func TestStrokedPathsRedrawWithoutAllocating(t *testing.T) {
	dc := NewDrawContext(200, 200, nil)
	red := RGB(255, 0, 0)
	pts := []float32{0, 0, 20, 10, 40, 0, 60, 10}
	// The same ping-pong renderDrawCanvas runs: this redraw claims the
	// buffers the last one left behind.
	var prev drawCanvasCache
	draw := func() {
		dc.resetFor(200, 200, 1, nil, prev)
		dc.RoundedRect(0, 0, 100, 50, 8, red, 2)
		dc.PolylineJoined(pts, red, 3)
		prev = drawCanvasCache{Batches: dc.batches, spare: dc.batchPool}
	}
	draw() // warm the scratch and batch buffers
	draw()
	got := testing.AllocsPerRun(20, draw)
	if got > 0 {
		t.Fatalf("allocs per redraw = %v, want 0", got)
	}
}

// resetFor releases every canvas scratch buffer that ran away, the
// transform point buffer included.
func TestResetForReleasesPointBuffer(t *testing.T) {
	dc := NewDrawContext(100, 100, nil)
	dc.xfPtBuf = make([]float32, 0, canvasScratchRetainMax+1)
	dc.resetFor(100, 100, 1, nil, drawCanvasCache{})
	if cap(dc.xfPtBuf) != 0 {
		t.Fatalf("xfPtBuf kept %d floats past the retain cap",
			cap(dc.xfPtBuf))
	}
}

// FillTrianglesColors takes the same input bound its gradient twin
// takes: a hostile mesh must not be walked.
func TestFillTrianglesColorsBoundsInput(t *testing.T) {
	dc := NewDrawContext(100, 100, nil)
	// Past the bound and still a well-formed mesh: 6 floats per
	// triangle, one color per vertex, so only the bound can reject it.
	n := maxFillTrisFloats + 2
	tris := make([]float32, n)
	cols := make([]Color, n/2)
	dc.FillTrianglesColors(tris, cols)
	if len(dc.Batches()) != 0 {
		t.Fatalf("batches = %d, want 0: an over-long mesh was accepted",
			len(dc.Batches()))
	}
}

// A non-finite stroke width must not reach a batch: the expanded
// quad inherits the NaN and poisons everything merged with it.
func TestStrokedWidthsScreenNonFinite(t *testing.T) {
	nan := float32(math.NaN())
	inf := float32(math.Inf(1))
	red := RGB(255, 0, 0)
	for _, w := range []float32{nan, inf} {
		dc := NewDrawContext(200, 200, nil)
		dc.FilledRect(0, 0, 10, 10, red)
		dc.Polyline([]float32{0, 0, 10, 10}, red, w)
		dc.Arc(50, 50, 10, 10, 0, 1, red, w)
		dc.DashedLine(0, 0, 10, 10, red, w, 4, 4)
		dc.DashedPolyline([]float32{0, 0, 10, 10}, red, w, 4, 4)
		for bi, b := range dc.Batches() {
			for i, v := range b.Triangles {
				if math.IsNaN(float64(v)) ||
					math.IsInf(float64(v), 0) {
					t.Fatalf("width %v: batch %d vertex %d is %v",
						w, bi, i, v)
				}
			}
		}
		if len(dc.Batches()) == 0 ||
			len(dc.Batches()[0].Triangles) != 12 {
			t.Fatalf("width %v: the good rect was dropped", w)
		}
	}
}

// Rounded and gradient shapes screen their geometry before it can
// reach a batch or a lowered quad.
func TestRoundedAndGradientScreenNonFinite(t *testing.T) {
	nan := float32(math.NaN())
	red := RGB(255, 0, 0)
	g := &CanvasGradient{
		Stops: []GradientStop{
			{Color: red, Pos: 0},
			{Color: RGB(0, 0, 255), Pos: 1},
		},
	}
	dc := NewDrawContext(200, 200, nil)
	dc.FilledRect(0, 0, 10, 10, red)
	dc.FilledRoundedRect(nan, 0, 10, 10, 2, red)
	dc.RoundedRect(0, 0, 10, 10, nan, red, 1)
	dc.FillTrianglesGradient([]float32{0, 0, 10, 0, 10, nan}, g)
	dc.FilledRectGradient(nan, 0, 10, 10, g)
	dc.FilledPolygonGradient([]float32{0, 0, 10, 0, nan, 10}, g)
	dc.FilledRoundedRectGradient(0, nan, 10, 10, 2, g)
	dc.FilledArcGradient(nan, 0, 10, 10, 0, 1, g)
	dc.FilledCircleGradient(0, 0, nan, g)
	for bi, b := range dc.Batches() {
		for i, v := range b.Triangles {
			if math.IsNaN(float64(v)) ||
				math.IsInf(float64(v), 0) {
				t.Fatalf("batch %d vertex %d is %v", bi, i, v)
			}
		}
	}
	if len(dc.Batches()) == 0 ||
		len(dc.Batches()[0].Triangles) != 12 {
		t.Fatalf("the good rect was dropped: %d batches",
			len(dc.Batches()))
	}
	if len(dc.Gradients()) != 0 {
		t.Fatalf("gradients = %d, want 0", len(dc.Gradients()))
	}
}
