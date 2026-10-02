package main

import (
	"math"
	"testing"

	"github.com/go-gui-org/go-gui/gui"
)

// Canvas size used by every geometry test. It is the Qt benchmark's default window
// (375×667), so the expected counts below can be checked by hand against the
// formulas in docs/specs/qcpainterbench.md.
const testW, testH = 375, 667

// opCounter is a DrawRecorder that only counts calls. It also implements
// DrawGradientRecorder, so gradient fills arrive as one call each instead of being
// flattened into polygons.
type opCounter struct {
	lines, polylines, joined                 int
	filledRects, rects                       int
	filledCircles, circles                   int
	filledArcs, arcs                         int
	filledPolys, texts, gradientFills, other int
	joinedPoints                             int
}

func (c *opCounter) Line(_, _, _, _ float32, _ gui.Color, _ float32) { c.lines++ }
func (c *opCounter) Polyline(_ []float32, _ gui.Color, _ float32)    { c.polylines++ }
func (c *opCounter) FilledRect(_, _, _, _ float32, _ gui.Color)      { c.filledRects++ }
func (c *opCounter) Rect(_, _, _, _ float32, _ gui.Color, _ float32) { c.rects++ }
func (c *opCounter) FilledCircle(_, _, _ float32, _ gui.Color)       { c.filledCircles++ }
func (c *opCounter) Circle(_, _, _ float32, _ gui.Color, _ float32)  { c.circles++ }
func (c *opCounter) FilledPolygon(_ []float32, _ gui.Color)          { c.filledPolys++ }
func (c *opCounter) Text(_, _ float32, _ string, _ gui.TextStyle)    { c.texts++ }
func (c *opCounter) FillTrianglesGradient(_ []float32, _ *gui.CanvasGradient) {
	c.gradientFills++
}

func (c *opCounter) FilledArc(_, _, _, _, _, _ float32, _ gui.Color) { c.filledArcs++ }
func (c *opCounter) Arc(_, _, _, _, _, _ float32, _ gui.Color, _ float32) {
	c.arcs++
}

func (c *opCounter) FilledRoundedRect(_, _, _, _, _ float32, _ gui.Color) { c.other++ }
func (c *opCounter) RoundedRect(_, _, _, _, _ float32, _ gui.Color, _ float32) {
	c.other++
}

func (c *opCounter) DashedLine(_, _, _, _ float32, _ gui.Color, _, _, _ float32) {
	c.other++
}

func (c *opCounter) DashedPolyline(_ []float32, _ gui.Color, _, _, _ float32) {
	c.other++
}

func (c *opCounter) PolylineJoined(p []float32, _ gui.Color, _ float32) {
	c.joined++
	c.joinedPoints += len(p) / 2
}

func (c *opCounter) QuadBezier(_, _, _, _, _, _ float32, _ gui.Color, _ float32) {
	c.other++
}

func (c *opCounter) CubicBezier(_, _, _, _, _, _, _, _ float32, _ gui.Color, _ float32) {
	c.other++
}

// countOps paints one frame of the given tests into a recording context and returns
// the call counts.
func countOps(t *testing.T, tests, count int, at float32) *opCounter {
	t.Helper()
	var s scene
	c := &opCounter{}
	dc := gui.NewDrawContext(testW, testH, nil)
	dc.SetRecorder(c)
	s.paint(dc, testW, testH, at, tests, count)
	return c
}

// TestRulerOps checks the ruler at t=0. The tick spacing is 0.03·w, and ticks
// start at 0.05·w, so ticks 0..31 fit (tick 32 lands at 1.01·w). The spacing is
// above 0.02·w, so ticks 0,5,10,…,30 are all labelled: 7 labels.
func TestRulerOps(t *testing.T) {
	t.Parallel()
	c := countOps(t, testRuler, 1, 0)
	if c.lines != 32 {
		t.Errorf("ticks = %d, want 32", c.lines)
	}
	if c.texts != 7 {
		t.Errorf("labels = %d, want 7", c.texts)
	}
}

// TestRulerHidesHalfLabels checks the ruler at the narrowest spacing. At t=3π/2,
// sin(t) = -1, so the spacing is 0.01·w. That is not above 0.02·w, so only the
// ticks at multiples of 10 get a label. The ticks are 0..94, so the labels are
// 0,10,…,90: 10 labels.
func TestRulerHidesHalfLabels(t *testing.T) {
	t.Parallel()
	c := countOps(t, testRuler, 1, 3*3.14159265/2)
	if c.lines != 95 {
		t.Errorf("ticks = %d, want 95", c.lines)
	}
	if c.texts != 10 {
		t.Errorf("labels = %d, want 10", c.texts)
	}
}

// TestCirclesOps checks the three gauges: 8+6+3 = 17 rings. Each ring is one
// background circle and one arc, and each arc gets two half-disc caps.
func TestCirclesOps(t *testing.T) {
	t.Parallel()
	c := countOps(t, testCircles, 1, 0)
	if c.circles != 17 || c.arcs != 17 {
		t.Errorf("circles, arcs = %d, %d, want 17, 17", c.circles, c.arcs)
	}
	if c.filledArcs != 34 {
		t.Errorf("caps = %d, want 34", c.filledArcs)
	}
}

// TestLinesOps checks the three bezier graphs: 4+6+12 = 22 points. Each graph is
// one gradient area fill and one stroked line, and each point is a filled and a
// stroked dot.
func TestLinesOps(t *testing.T) {
	t.Parallel()
	c := countOps(t, testLines, 1, 0)
	if c.gradientFills != 3 || c.joined != 3 {
		t.Errorf("fills, strokes = %d, %d, want 3, 3", c.gradientFills, c.joined)
	}
	if c.filledCircles != 22 || c.circles != 22 {
		t.Errorf("dots = %d filled, %d stroked, want 22, 22",
			c.filledCircles, c.circles)
	}
}

// TestBarsOps checks the four bar graphs: 6+10+20+40 = 76 bars. A bar whose
// height truncates to zero has no area to fill. Qt still strokes it as a
// one-pixel line, so the port draws a Line for it. Each bar is therefore exactly
// one stroke, and every bar with height is exactly one fill.
func TestBarsOps(t *testing.T) {
	t.Parallel()
	c := countOps(t, testBars, 1, 0)
	if c.rects+c.lines != 76 {
		t.Errorf("bar strokes = %d, want 76", c.rects+c.lines)
	}
	if c.filledRects != c.rects {
		t.Errorf("fills = %d, strokes = %d, want equal", c.filledRects, c.rects)
	}
}

// TestIconsOps checks 20 images and 20 labels. Images do not go through
// DrawRecorder, so this paints without a recorder and reads the recorded entries.
func TestIconsOps(t *testing.T) {
	t.Parallel()
	var s scene
	dc := gui.NewDrawContext(testW, testH, nil)
	s.paint(dc, testW, testH, 0, testIcons, 1)
	if n := len(dc.Images()); n != 20 {
		t.Errorf("images = %d, want 20", n)
	}
	if n := len(dc.Texts()); n != 20 {
		t.Errorf("labels = %d, want 20", n)
	}
}

// TestFlowerOps checks one gradient fill, one closed outline and one center dot.
// The outline is the 12 flattened quadratics plus the closing point.
func TestFlowerOps(t *testing.T) {
	t.Parallel()
	c := countOps(t, testFlower, 1, 0)
	if c.gradientFills != 1 || c.joined != 1 || c.filledCircles != 1 {
		t.Errorf("fill, outline, dot = %d, %d, %d, want 1, 1, 1",
			c.gradientFills, c.joined, c.filledCircles)
	}
	if want := 12*flowerSegs + 1; c.joinedPoints != want {
		t.Errorf("outline points = %d, want %d", c.joinedPoints, want)
	}
}

// TestRenderCountScales checks that render count N draws every enabled test N
// times. Qt adds 0.3 to t on each pass, and the ruler tick count depends on t, so
// the check compares against N single passes at the shifted times.
func TestRenderCountScales(t *testing.T) {
	t.Parallel()
	const n = 4
	got := countOps(t, testAll, n, 0)
	want := &opCounter{}
	for i := range n {
		one := countOps(t, testAll, 1, float32(i)*passStep)
		want.lines += one.lines
		want.texts += one.texts
		want.arcs += one.arcs
		want.rects += one.rects
		want.gradientFills += one.gradientFills
		want.filledCircles += one.filledCircles
	}
	if got.lines != want.lines || got.texts != want.texts ||
		got.arcs != want.arcs || got.rects != want.rects ||
		got.gradientFills != want.gradientFills ||
		got.filledCircles != want.filledCircles {
		t.Errorf("count %d: got %+v, want sums %+v", n, *got, *want)
	}
}

// flowerCapture keeps the baked flower geometry: the gradient fan and the
// outline stroke. It embeds gui.DrawRecorder for the methods the flower
// never calls.
type flowerCapture struct {
	gui.DrawRecorder
	tris []float32
	line []float32
}

func (f *flowerCapture) FillTrianglesGradient(tris []float32, _ *gui.CanvasGradient) {
	f.tris = append(f.tris[:0], tris...)
}

func (f *flowerCapture) PolylineJoined(p []float32, _ gui.Color, _ float32) {
	f.line = append(f.line[:0], p...)
}

func (f *flowerCapture) FilledCircle(_, _, _ float32, _ gui.Color) {}

// TestFlowerUsesCanvasRotation checks the flower draws its cached outline
// under a canvas Rotate instead of rotating points on the CPU: at t=pi/2 the
// recorded geometry is the unrotated outline turned 20 degrees about the
// center.
func TestFlowerUsesCanvasRotation(t *testing.T) {
	t.Parallel()
	sz := float32(min(testW, testH))
	fs := 80 + sz*0.6
	fx, fy := float32(testW)/2-fs/2, float32(testH)-fs
	cx, cy := fx+fs/2, fy+fs/2

	var s scene
	rec := &flowerCapture{}
	dc := gui.NewDrawContext(testW, testH, nil)
	dc.SetRecorder(rec)
	s.paint(dc, testW, testH, float32(math.Pi/2), testFlower, 1)

	outline := s.flowerOutline(fx, fy, fs)
	if len(rec.line) != len(outline) {
		t.Fatalf("recorded outline points = %d, want %d",
			len(rec.line)/2, len(outline)/2)
	}
	// sin(pi/2) is 1, so the angle is exactly 20 degrees.
	rad := 20 * math.Pi / 180
	ca, sa := math.Cos(rad), math.Sin(rad)
	for i := 0; i+1 < len(outline); i += 2 {
		dx, dy := float64(outline[i]-cx), float64(outline[i+1]-cy)
		ex, ey := float32(cx+float32(dx*ca-dy*sa)), float32(cy+float32(dx*sa+dy*ca))
		if d := absf(rec.line[i] - ex); d > 1e-2 {
			t.Fatalf("point %d x = %v, want rotated %v", i/2, rec.line[i], ex)
		}
		if d := absf(rec.line[i+1] - ey); d > 1e-2 {
			t.Fatalf("point %d y = %v, want rotated %v", i/2, rec.line[i+1], ey)
		}
	}
	// The rotation must actually move the rim: guards a test that
	// passes because both sides stayed unrotated.
	if absf(rec.line[2]-outline[2]) < 1 {
		t.Errorf("recorded outline matches the unrotated one; rotation missing")
	}
	// The fan references the same rotated points as the stroke.
	if len(rec.tris) < 6 || rec.tris[2] != rec.line[0] || rec.tris[3] != rec.line[1] {
		t.Errorf("fan does not open on the recorded outline")
	}
}

func absf(v float32) float32 {
	if v < 0 {
		return -v
	}
	return v
}

// TestFanCoversFlower checks the flower fill's triangle fan. The fan from the
// center is valid only if the outline's angle around the center turns one way.
// Each fan triangle must then have the same winding sign.
func TestFanCoversFlower(t *testing.T) {
	t.Parallel()
	var s scene
	pts := s.flowerOutline(0, 0, 200)
	cx, cy := float32(100), float32(100)
	var pos, neg int
	for i := 0; i+3 < len(pts); i += 2 {
		ax, ay := pts[i]-cx, pts[i+1]-cy
		bx, by := pts[i+2]-cx, pts[i+3]-cy
		switch cross := ax*by - ay*bx; {
		case cross > 1e-3:
			pos++
		case cross < -1e-3:
			neg++
		}
	}
	if pos != 0 && neg != 0 {
		t.Errorf("fan winding mixed: %d positive, %d negative", pos, neg)
	}
}

// TestPaintDegenerateSize checks that a zero, negative or NaN canvas size draws
// without a panic or an endless ruler loop. A canvas can be 0×0 before its first
// layout.
func TestPaintDegenerateSize(t *testing.T) {
	t.Parallel()
	nan := float32(math.NaN())
	for _, sz := range [][2]float32{{0, 0}, {-10, 50}, {nan, nan}, {1, 1}} {
		var s scene
		dc := gui.NewDrawContext(sz[0], sz[1], nil)
		dc.SetRecorder(&opCounter{})
		s.paint(dc, sz[0], sz[1], 1, testAll, 2)
	}
}
