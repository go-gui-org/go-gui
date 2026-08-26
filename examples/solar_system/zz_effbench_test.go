package main

import (
	"testing"
	"time"

	"github.com/go-gui-org/go-gui/gui"
)

func TestInitCostMeasure(t *testing.T) {
	start := time.Now()
	_ = makeTextures()
	t.Logf("makeTextures: %v", time.Since(start))
}

// counter tallies every primitive drawSystem emits in one frame.
type counter struct{ n map[string]int }

func newCounter() *counter      { return &counter{n: map[string]int{}} }
func (c *counter) hit(k string) { c.n[k]++ }

func (c *counter) Line(x0, y0, x1, y1 float32, col gui.Color, w float32)        { c.hit("Line") }
func (c *counter) Polyline(p []float32, col gui.Color, w float32)               { c.hit("Polyline") }
func (c *counter) FilledRect(x, y, w, h float32, col gui.Color)                 { c.hit("FilledRect") }
func (c *counter) Rect(x, y, w, h float32, col gui.Color, wd float32)           { c.hit("Rect") }
func (c *counter) FilledCircle(cx, cy, r float32, col gui.Color)                { c.hit("FilledCircle") }
func (c *counter) Circle(cx, cy, r float32, col gui.Color, w float32)           { c.hit("Circle") }
func (c *counter) FilledArc(cx, cy, rx, ry, s, sw float32, col gui.Color)       { c.hit("FilledArc") }
func (c *counter) Arc(cx, cy, rx, ry, s, sw float32, col gui.Color, w float32)  { c.hit("Arc") }
func (c *counter) FilledPolygon(p []float32, col gui.Color)                     { c.hit("FilledPolygon") }
func (c *counter) FilledRoundedRect(x, y, w, h, r float32, col gui.Color)       { c.hit("FilledRoundedRect") }
func (c *counter) RoundedRect(x, y, w, h, r float32, col gui.Color, wd float32) { c.hit("RoundedRect") }
func (c *counter) DashedLine(x0, y0, x1, y1 float32, col gui.Color, w, d, g float32) {
	c.hit("DashedLine")
}
func (c *counter) DashedPolyline(p []float32, col gui.Color, w, d, g float32) {
	c.hit("DashedPolyline")
}
func (c *counter) PolylineJoined(p []float32, col gui.Color, w float32) { c.hit("PolylineJoined") }
func (c *counter) QuadBezier(a, b, cc, d, e, f float32, col gui.Color, w float32) {
	c.hit("QuadBezier")
}
func (c *counter) CubicBezier(a, b, cc, d, e, f, g, h float32, col gui.Color, w float32) {
	c.hit("CubicBezier")
}
func (c *counter) Text(x, y float32, s string, st gui.TextStyle) { c.hit("Text") }
func (c *counter) FillTrianglesGradient(tris []float32, g *gui.CanvasGradient) {
	c.hit("FillTrianglesGradient")
}
func (c *counter) FillTrianglesColors(tris []float32, cols []gui.Color) {
	c.hit("FillTrianglesColors")
	c.n["meshTris"] += len(tris) / 6
}

func countFrame(a *App) *counter {
	dc := gui.NewDrawContext(a.CanvasW, a.CanvasH, nil)
	c := newCounter()
	dc.SetRecorder(c)
	drawSystem(a, dc)
	return c
}

func TestPrimitiveCountsPerFrame(t *testing.T) {
	cases := []struct {
		name string
		sel  int
	}{
		{"full-system", -1},
		{"sun-selected", selSun},
		{"jupiter-selected", 4},
		{"mercury-selected", 0},
	}
	for _, tc := range cases {
		a := newApp()
		a.CanvasW, a.CanvasH = 1100, 760
		a.Selected = tc.sel
		for range 120 { // let the tween settle
			tick(a)
		}
		c := countFrame(a)
		t.Logf("%-18s sunR=%6.1f  %v", tc.name, a.SunR, c.n)
	}
}

func TestBatchCountPerFrame(t *testing.T) {
	for _, sel := range []int{-1, selSun, 4} {
		a := newApp()
		a.CanvasW, a.CanvasH = 1100, 760
		a.Selected = sel
		for range 120 {
			tick(a)
		}
		dc := gui.NewDrawContext(a.CanvasW, a.CanvasH, nil)
		drawSystem(a, dc)
		var tris int
		for _, b := range dc.Batches() {
			tris += len(b.Triangles) / 6
		}
		t.Logf("sel=%3d batches=%4d triangles=%6d", sel, len(dc.Batches()), tris)
	}
}

func benchFrame(b *testing.B, sel int) {
	a := newApp()
	a.CanvasW, a.CanvasH = 1100, 760
	a.Selected = sel
	for range 120 {
		tick(a)
	}
	b.ReportAllocs()
	b.ResetTimer()
	for b.Loop() {
		tick(a)
		dc := gui.NewDrawContext(a.CanvasW, a.CanvasH, nil)
		drawSystem(a, dc)
	}
}

func BenchmarkFrameFullSystem(b *testing.B) { benchFrame(b, -1) }
func BenchmarkFrameJupiter(b *testing.B)    { benchFrame(b, 4) }

func BenchmarkTickOnly(b *testing.B) {
	a := newApp()
	a.CanvasW, a.CanvasH = 1100, 760
	b.ReportAllocs()
	for b.Loop() {
		tick(a)
	}
}

// benchWholeFrame drives the frame the app actually runs: view
// generation, layout, and the renderer build that calls OnDraw through
// renderDrawCanvas — where the tessellation buffers are pooled. The
// benchFrame family above measures OnDraw in isolation with a fresh
// DrawContext, so it cannot see that pooling; this one can, and it is
// the number that corresponds to a running window.
func benchWholeFrame(b *testing.B, sel int) {
	a := newApp()
	a.CanvasW, a.CanvasH = windowW, windowH
	a.Selected = sel
	for range 120 {
		tick(a)
	}
	w := gui.NewWindow(gui.WindowCfg{
		State:  a,
		Width:  windowW,
		Height: windowH,
	})
	w.TestRender(mainView)

	b.ReportAllocs()
	b.ResetTimer()
	for b.Loop() {
		tick(a)
		w.TestRender(nil)
	}
}

func BenchmarkWholeFrameFullSystem(b *testing.B) { benchWholeFrame(b, -1) }
func BenchmarkWholeFrameJupiter(b *testing.B)    { benchWholeFrame(b, 4) }
