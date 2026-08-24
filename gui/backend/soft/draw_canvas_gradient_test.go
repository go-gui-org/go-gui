package soft

import (
	"testing"

	"github.com/go-gui-org/go-gui/gui"
)

// canvasGradientCmd tessellates a canvas gradient fill through the real
// DrawContext and returns it as the RenderSvg command the render path
// would emit. This is the end of the seam: geometry and per-vertex
// colors produced in gui/, rasterized by a backend, asserted as pixels.
func canvasGradientCmd(t *testing.T,
	draw func(dc *gui.DrawContext)) gui.RenderCmd {
	t.Helper()
	dc := gui.NewDrawContext(40, 40, nil)
	draw(dc)
	batches := dc.Batches()
	if len(batches) != 1 {
		t.Fatalf("got %d batches, want 1", len(batches))
	}
	b := batches[0]
	if len(b.VertexColors)*2 != len(b.Triangles) {
		t.Fatalf("VertexColors=%d, Triangles=%d",
			len(b.VertexColors), len(b.Triangles))
	}
	return gui.RenderCmd{
		Kind:         gui.RenderSvg,
		Scale:        1,
		Color:        b.Color,
		Triangles:    b.Triangles,
		VertexColors: b.VertexColors,
	}
}

// TestCanvasLinearGradientRasterizes checks the ramp actually appears
// in pixels, and in the right direction.
func TestCanvasLinearGradientRasterizes(t *testing.T) {
	cmd := canvasGradientCmd(t, func(dc *gui.DrawContext) {
		dc.FilledRectGradient(4, 4, 32, 32, &gui.CanvasGradient{
			X1: 4, Y1: 4, X2: 36, Y2: 4,
			Stops: []gui.GradientStop{
				{Color: gui.RGB(255, 0, 0), Pos: 0},
				{Color: gui.RGB(0, 0, 255), Pos: 1},
			},
		})
	})
	r := newRenderer(40, 40, 1)
	r.drawAll([]gui.RenderCmd{cmd})

	leftR, _, leftB, _ := at(r.buf.img, 6, 20)
	rightR, _, rightB, _ := at(r.buf.img, 34, 20)
	if leftR <= rightR {
		t.Errorf("red %d→%d across the ramp, want it falling",
			leftR, rightR)
	}
	if leftB >= rightB {
		t.Errorf("blue %d→%d across the ramp, want it rising",
			leftB, rightB)
	}
	// A gradient perpendicular to the ramp axis must not vary.
	topR, _, _, _ := at(r.buf.img, 20, 8)
	botR, _, _, _ := at(r.buf.img, 20, 32)
	if diff(int(topR), int(botR)) > 2 {
		t.Errorf("red varies %d→%d along the constant axis", topR, botR)
	}
}

// TestCanvasRadialGradientRasterizes is the case the concentric-ring
// workaround existed for: a glow whose falloff is smooth and whose
// interior carries no seams.
func TestCanvasRadialGradientRasterizes(t *testing.T) {
	cmd := canvasGradientCmd(t, func(dc *gui.DrawContext) {
		dc.FilledCircleGradient(20, 20, 16, &gui.CanvasGradient{
			Radial: true,
			Stops: []gui.GradientStop{
				{Color: gui.RGBA(255, 255, 255, 255), Pos: 0},
				{Color: gui.RGBA(255, 255, 255, 0), Pos: 1},
			},
		})
	})
	r := newRenderer(40, 40, 1)
	r.drawAll([]gui.RenderCmd{cmd})

	// The buffer starts opaque black, so a white-to-transparent ramp
	// reads as brightness rather than alpha.
	centerV, _, _, _ := at(r.buf.img, 20, 20)
	midV, _, _, _ := at(r.buf.img, 28, 20)
	rimV, _, _, _ := at(r.buf.img, 35, 20)
	if centerV <= midV || midV <= rimV {
		t.Errorf("brightness %d→%d→%d from center to rim, want it "+
			"falling monotonically", centerV, midV, rimV)
	}
	// Not a full 255: the pixel's center sits half a pixel off the
	// fan's hub vertex, so it samples the ramp just past t=0.
	if centerV < 200 {
		t.Errorf("center brightness = %d, want near the t=0 stop", centerV)
	}

	// Scan a radius for a seam: the fan's triangle edges must not show
	// as a bright ridge or a dark dip, which is exactly what the
	// stacked-ring workaround left behind at low ring counts.
	prev := -1
	for x := 20; x <= 34; x++ {
		v8, _, _, _ := at(r.buf.img, x, 20)
		v := int(v8)
		if prev >= 0 && v > prev+2 {
			t.Errorf("brightness rises %d→%d at x=%d: a seam in the "+
				"falloff", prev, v, x)
		}
		prev = v
	}
}

func diff(a, b int) int {
	if a > b {
		return a - b
	}
	return b - a
}

// TestVertexColorsLengthMismatchFallsBackToFlat pins the guard that
// decides whether a command is a mesh. shadeTriangle indexes cols by
// triangle, so a short list would read past the end; drawSvg's length
// equality is the only thing standing between a malformed command and
// that read. A mismatch must paint the flat color, not panic and not
// vanish.
func TestVertexColorsLengthMismatchFallsBackToFlat(t *testing.T) {
	full := canvasGradientCmd(t, func(dc *gui.DrawContext) {
		dc.FilledRectGradient(4, 4, 32, 32, &gui.CanvasGradient{
			X1: 4, Y1: 4, X2: 36, Y2: 4,
			Stops: []gui.GradientStop{
				{Color: gui.RGB(255, 0, 0), Pos: 0},
				{Color: gui.RGB(0, 0, 255), Pos: 1},
			},
		})
	})
	for _, tc := range []struct {
		name string
		cols []gui.Color
	}{
		{"short", full.VertexColors[:len(full.VertexColors)-1]},
		{"empty", nil},
	} {
		t.Run(tc.name, func(t *testing.T) {
			cmd := full
			cmd.VertexColors = tc.cols
			cmd.Color = gui.RGB(0, 255, 0)
			r := newRenderer(40, 40, 1)
			r.drawAll([]gui.RenderCmd{cmd})
			// Flat green everywhere inside, no ramp.
			for _, x := range []int{6, 20, 34} {
				cr, cg, cb, _ := at(r.buf.img, x, 20)
				if cg < 200 || cr > 40 || cb > 40 {
					t.Errorf("x=%d is %d,%d,%d, want flat green",
						x, cr, cg, cb)
				}
			}
		})
	}
}

// vcolCmd wraps a hand-built mesh as the RenderSvg command the canvas
// path emits for FillTrianglesColors.
func vcolCmd(tris []float32, cols []gui.Color) gui.RenderCmd {
	return gui.RenderCmd{
		Kind:         gui.RenderSvg,
		Scale:        1,
		Color:        gui.RGB(255, 255, 255),
		Triangles:    tris,
		VertexColors: cols,
	}
}

// TestMeshWindingMustBeConsistent pins the contract a caller of
// FillTrianglesColors takes on. The mesh is rasterized as one path with
// *signed* coverage accumulation, so two triangles sharing an edge but
// wound against each other contribute +0.5 and -0.5 along it: the sum
// is zero and the background shows through as a hairline. Wound the
// same way they sum to full coverage and the seam disappears.
//
// The check is a diagonal pixel, which is on the shared edge, against
// an interior pixel of the same triangle.
func TestMeshWindingMustBeConsistent(t *testing.T) {
	white := gui.RGB(255, 255, 255)
	cols := []gui.Color{white, white, white, white, white, white}

	// A square split along its top-left/bottom-right diagonal.
	same := []float32{
		4, 4, 36, 4, 36, 36,
		4, 4, 36, 36, 4, 36,
	}
	// The same square, second triangle reversed.
	mixed := []float32{
		4, 4, 36, 4, 36, 36,
		4, 4, 4, 36, 36, 36,
	}

	seam := func(tris []float32) (onEdge, inside uint8) {
		r := newRenderer(40, 40, 1)
		r.drawAll([]gui.RenderCmd{vcolCmd(tris, cols)})
		e, _, _, _ := at(r.buf.img, 20, 20)
		i, _, _, _ := at(r.buf.img, 30, 12)
		return e, i
	}

	edge, inside := seam(same)
	if inside < 250 {
		t.Fatalf("interior painted %d, want the mesh solid", inside)
	}
	if diff(int(edge), int(inside)) > 4 {
		t.Errorf("consistent winding painted %d on the shared edge "+
			"against %d inside: that is a seam", edge, inside)
	}

	edge, inside = seam(mixed)
	if inside < 250 {
		t.Fatalf("interior painted %d, want the mesh solid", inside)
	}
	// The cancellation is total, not marginal: the shared edge comes
	// out at zero coverage against a solid interior.
	if edge > 8 {
		t.Errorf("mixed winding painted %d on the shared edge against "+
			"%d inside; the signed accumulation is supposed to cancel "+
			"there, and this test is what documents why a mesh must "+
			"not do it", edge, inside)
	}
}
