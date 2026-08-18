package soft

import (
	"image"
	"testing"

	"github.com/go-gui-org/go-gui/gui"
)

// at returns the pixel at (x, y) as straight RGBA components.
func at(img *image.RGBA, x, y int) (r, g, b, a uint8) {
	i := img.PixOffset(x, y)
	p := img.Pix
	return p[i], p[i+1], p[i+2], p[i+3]
}

func closeTo(got, want, tol uint8) bool {
	if got > want {
		return got-want <= tol
	}
	return want-got <= tol
}

// newRenderer builds a renderer over a black buffer, with no text
// system: these tests exercise the shape and clip paths only.
func newRenderer(w, h int, scale float32) *renderer {
	r := &renderer{buf: newBuffer(w, h), scale: scale}
	r.buf.clear(gui.RGB(0, 0, 0))
	return r
}

func rectCmd(x, y, w, h float32, c gui.Color) gui.RenderCmd {
	return gui.RenderCmd{Kind: gui.RenderRect, X: x, Y: y, W: w, H: h,
		Color: c, Fill: true}
}

func TestDrawRectCoversExactly(t *testing.T) {
	r := newRenderer(40, 40, 1)
	r.drawAll([]gui.RenderCmd{rectCmd(10, 10, 20, 20, gui.RGB(0, 255, 0))})

	if _, g, _, _ := at(r.buf.img, 20, 20); g != 255 {
		t.Errorf("inside green = %d, want 255", g)
	}
	if _, g, _, _ := at(r.buf.img, 9, 20); g != 0 {
		t.Errorf("left of rect green = %d, want 0", g)
	}
	if _, g, _, _ := at(r.buf.img, 30, 20); g != 0 {
		t.Errorf("right of rect green = %d, want 0", g)
	}
}

func TestClipSuppressesWritesOutside(t *testing.T) {
	r := newRenderer(40, 40, 1)
	r.drawAll([]gui.RenderCmd{
		{Kind: gui.RenderClip, X: 0, Y: 0, W: 20, H: 40},
		rectCmd(0, 0, 40, 40, gui.RGB(0, 255, 0)),
	})

	if _, g, _, _ := at(r.buf.img, 10, 20); g != 255 {
		t.Errorf("inside clip green = %d, want 255", g)
	}
	if _, g, _, _ := at(r.buf.img, 30, 20); g != 0 {
		t.Errorf("outside clip green = %d, want 0 (clip leaked)", g)
	}
}

func TestClipResetByDegenerateRect(t *testing.T) {
	r := newRenderer(40, 40, 1)
	r.drawAll([]gui.RenderCmd{
		{Kind: gui.RenderClip, X: 0, Y: 0, W: 20, H: 40},
		{Kind: gui.RenderClip}, // zero size restores the full buffer
		rectCmd(0, 0, 40, 40, gui.RGB(0, 255, 0)),
	})
	if _, g, _, _ := at(r.buf.img, 30, 20); g != 255 {
		t.Errorf("after clip reset green = %d, want 255", g)
	}
}

func TestScaleMultipliesCoordinates(t *testing.T) {
	r := newRenderer(40, 40, 2)
	r.drawAll([]gui.RenderCmd{rectCmd(5, 5, 10, 10, gui.RGB(0, 255, 0))})

	// At scale 2 the rect occupies device pixels [10,30).
	if _, g, _, _ := at(r.buf.img, 20, 20); g != 255 {
		t.Errorf("inside scaled rect green = %d, want 255", g)
	}
	if _, g, _, _ := at(r.buf.img, 9, 20); g != 0 {
		t.Errorf("left of scaled rect green = %d, want 0", g)
	}
	if _, g, _, _ := at(r.buf.img, 31, 20); g != 0 {
		t.Errorf("right of scaled rect green = %d, want 0", g)
	}
}

func TestStrokeRectIsHollow(t *testing.T) {
	r := newRenderer(40, 40, 1)
	r.drawAll([]gui.RenderCmd{{
		Kind: gui.RenderStrokeRect, X: 10, Y: 10, W: 20, H: 20,
		Thickness: 2, Color: gui.RGB(0, 255, 0),
	}})

	if _, g, _, _ := at(r.buf.img, 11, 20); g != 255 {
		t.Errorf("on the stroke green = %d, want 255", g)
	}
	if _, g, _, _ := at(r.buf.img, 20, 20); g != 0 {
		t.Errorf("inside the ring green = %d, want 0 (not hollow)", g)
	}
}

func TestGradientMidpointBlends(t *testing.T) {
	r := newRenderer(40, 40, 1)
	r.drawAll([]gui.RenderCmd{{
		Kind: gui.RenderGradient, X: 0, Y: 0, W: 40, H: 40,
		Gradient: &gui.GradientDef{
			Direction: gui.GradientToRight,
			Stops: []gui.GradientStop{
				{Color: gui.RGB(255, 0, 0), Pos: 0},
				{Color: gui.RGB(0, 0, 255), Pos: 1},
			},
		},
	}})

	if red, _, _, _ := at(r.buf.img, 1, 20); !closeTo(red, 255, 12) {
		t.Errorf("left edge red = %d, want ~255", red)
	}
	if _, _, blue, _ := at(r.buf.img, 38, 20); !closeTo(blue, 255, 12) {
		t.Errorf("right edge blue = %d, want ~255", blue)
	}
	red, _, blue, _ := at(r.buf.img, 20, 20)
	if !closeTo(red, 128, 12) || !closeTo(blue, 128, 12) {
		t.Errorf("midpoint = r%d b%d, want ~128 each", red, blue)
	}
}

func TestCircleIsRound(t *testing.T) {
	r := newRenderer(40, 40, 1)
	r.drawAll([]gui.RenderCmd{{
		Kind: gui.RenderCircle, X: 20, Y: 20, Radius: 10,
		Color: gui.RGB(0, 255, 0), Fill: true,
	}})

	if _, g, _, _ := at(r.buf.img, 20, 20); g != 255 {
		t.Errorf("centre green = %d, want 255", g)
	}
	// The bounding box corner is outside the disc.
	if _, g, _, _ := at(r.buf.img, 11, 11); g != 0 {
		t.Errorf("bbox corner green = %d, want 0 (square, not circle)", g)
	}
}

func TestLinePaintsOnlyTheSegment(t *testing.T) {
	r := newRenderer(40, 40, 1)
	r.drawAll([]gui.RenderCmd{{
		Kind: gui.RenderLine, X: 5, Y: 20, OffsetX: 35, OffsetY: 20,
		Thickness: 2, Color: gui.RGB(0, 255, 0),
	}})

	if _, g, _, _ := at(r.buf.img, 20, 20); g != 255 {
		t.Errorf("on the line green = %d, want 255", g)
	}
	if _, g, _, _ := at(r.buf.img, 3, 20); g != 0 {
		t.Errorf("past the start green = %d, want 0", g)
	}
	if _, g, _, _ := at(r.buf.img, 37, 20); g != 0 {
		t.Errorf("past the end green = %d, want 0", g)
	}
	if _, g, _, _ := at(r.buf.img, 20, 16); g != 0 {
		t.Errorf("above the line green = %d, want 0", g)
	}
}

func TestRadialGradientFallsOffFromCentre(t *testing.T) {
	r := newRenderer(40, 40, 1)
	r.drawAll([]gui.RenderCmd{{
		Kind: gui.RenderGradient, X: 0, Y: 0, W: 40, H: 40,
		Gradient: &gui.GradientDef{
			Type: gui.GradientRadial,
			Stops: []gui.GradientStop{
				{Color: gui.RGB(255, 0, 0), Pos: 0},
				{Color: gui.RGB(0, 0, 255), Pos: 1},
			},
		},
	}})

	red, _, blue, _ := at(r.buf.img, 20, 20)
	if red <= blue {
		t.Errorf("centre = r%d b%d, want red dominant", red, blue)
	}
	red, _, blue, _ = at(r.buf.img, 1, 1)
	if blue <= red {
		t.Errorf("corner = r%d b%d, want blue dominant", red, blue)
	}
}

func TestGradientBorderPaintsEdgeStrips(t *testing.T) {
	r := newRenderer(40, 40, 1)
	r.drawAll([]gui.RenderCmd{{
		Kind: gui.RenderGradientBorder, X: 10, Y: 10, W: 20, H: 20,
		Thickness: 2,
		Gradient: &gui.GradientDef{
			Direction: gui.GradientToRight,
			Stops: []gui.GradientStop{
				{Color: gui.RGB(255, 0, 0), Pos: 0},
				{Color: gui.RGB(0, 0, 255), Pos: 1},
			},
		},
	}})

	// The top strip samples position 0 → red; the bottom samples
	// 0.25 → mixed. The interior must stay untouched.
	if red, _, _, _ := at(r.buf.img, 20, 11); red != 255 {
		t.Errorf("top strip red = %d, want 255", red)
	}
	red, _, blue, _ := at(r.buf.img, 20, 28)
	if blue == 0 || red <= blue {
		t.Errorf("bottom strip = r%d b%d, want red dominant", red, blue)
	}
	if cr, cg, cb, _ := at(r.buf.img, 20, 20); cr|cg|cb != 0 {
		t.Errorf("interior = %d,%d,%d, want untouched", cr, cg, cb)
	}
}

func TestDrawImageMemSource(t *testing.T) {
	r := newRenderer(40, 40, 1)
	pix := make([]byte, 4*4*4)
	for i := range 4 {
		for j := range 4 {
			k := (i*4 + j) * 4
			pix[k], pix[k+1], pix[k+2], pix[k+3] = 0, 0, 255, 255
		}
	}
	src := gui.UseImage("soft-test-img", 4, 4, pix)
	if src == "" {
		t.Fatal("UseImage refused the buffer")
	}
	t.Cleanup(func() { gui.DropImage("soft-test-img") })

	r.drawAll([]gui.RenderCmd{{
		Kind: gui.RenderImage, X: 10, Y: 10, W: 20, H: 20,
		Resource: src,
	}})

	if _, _, blue, _ := at(r.buf.img, 20, 20); blue != 255 {
		t.Errorf("inside the image blue = %d, want 255", blue)
	}
	if _, _, blue, _ := at(r.buf.img, 9, 20); blue != 0 {
		t.Errorf("outside the image blue = %d, want 0", blue)
	}
}

func TestUnsupportedKindsAreSkipped(t *testing.T) {
	r := newRenderer(8, 8, 1)
	// Phase-2 kinds must not draw and must not panic.
	r.drawAll([]gui.RenderCmd{
		{Kind: gui.RenderShadow, X: 0, Y: 0, W: 8, H: 8,
			Color: gui.RGB(255, 255, 255)},
		{Kind: gui.RenderSvg, X: 0, Y: 0, Scale: 1},
		{Kind: gui.RenderStencilBegin},
		{Kind: gui.RenderStencilEnd},
	})
	for y := range 8 {
		for x := range 8 {
			cr, cg, cb, _ := at(r.buf.img, x, y)
			if cr|cg|cb != 0 {
				t.Fatalf("pixel (%d,%d) painted by a skipped kind", x, y)
			}
		}
	}
}

func TestBufferBoundsMatchScale(t *testing.T) {
	b := newBuffer(30, 20)
	if got := b.img.Bounds(); got != image.Rect(0, 0, 30, 20) {
		t.Fatalf("bounds = %v", got)
	}
	if b.clip != b.img.Bounds() {
		t.Fatalf("clip = %v, want full bounds", b.clip)
	}
}
