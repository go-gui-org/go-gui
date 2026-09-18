package soft

import (
	"image"
	"math"
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
	root := newBuffer(w, h)
	r := &renderer{buf: root, rootBuf: root, scale: scale}
	root.clear(gui.RGB(0, 0, 0))
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
		Resource: src, Opacity: 1,
	}})

	if _, _, blue, _ := at(r.buf.img, 20, 20); blue != 255 {
		t.Errorf("inside the image blue = %d, want 255", blue)
	}
	if _, _, blue, _ := at(r.buf.img, 9, 20); blue != 0 {
		t.Errorf("outside the image blue = %d, want 0", blue)
	}
}

// RenderCmd.Opacity scales texel alpha: half opacity on an opaque
// source yields half alpha at exact texel centres, and zero
// opacity draws nothing.
func TestDrawImageOpacity(t *testing.T) {
	mkib := func(t *testing.T, key string) string {
		t.Helper()
		pix := make([]byte, 4*4*4)
		for k := range 4 * 4 {
			pix[k*4], pix[k*4+1] = 0, 0
			pix[k*4+2], pix[k*4+3] = 255, 255
		}
		src := gui.UseImage(key, 4, 4, pix)
		if src == "" {
			t.Fatal("UseImage refused the buffer")
		}
		t.Cleanup(func() { gui.DropImage(key) })
		return src
	}
	t.Run("half", func(t *testing.T) {
		r := newRenderer(40, 40, 1)
		src := mkib(t, "soft-test-opacity-half")
		// 1:1 scale: device pixel (12,12) hits texel (2,2) exactly.
		// The buffer starts opaque black, so half-alpha blue
		// composites to (0,0,128,255).
		r.drawAll([]gui.RenderCmd{{
			Kind: gui.RenderImage, X: 10, Y: 10, W: 4, H: 4,
			Resource: src, Opacity: 0.5,
		}})
		_, _, blue, alpha := at(r.buf.img, 12, 12)
		if blue != 128 {
			t.Errorf("blue = %d, want 128", blue)
		}
		if alpha != 255 {
			t.Errorf("alpha = %d, want 255", alpha)
		}
	})
	t.Run("zero draws nothing", func(t *testing.T) {
		r := newRenderer(40, 40, 1)
		src := mkib(t, "soft-test-opacity-zero")
		r.drawAll([]gui.RenderCmd{{
			Kind: gui.RenderImage, X: 10, Y: 10, W: 4, H: 4,
			Resource: src, Opacity: 0,
		}})
		if _, _, blue, _ := at(r.buf.img, 12, 12); blue != 0 {
			t.Errorf("blue = %d, want 0", blue)
		}
	})
}

// An empty source rect must sample transparent instead of
// panicking on a negative clamp bound.
func TestImageSrcEmptySource(t *testing.T) {
	img := &image.NRGBA{Rect: image.Rect(0, 0, 0, 0)}
	s := newImageSrc(img, 0, 0, 10, 10, 1)
	c := s.At(0, 0).(interface {
		RGBA() (r, g, b, a uint32)
	})
	_, _, _, a := c.RGBA()
	if a != 0 {
		t.Fatalf("empty source alpha = %d, want 0", a>>8)
	}
}

// A nil source samples transparent rather than panicking in
// Bounds: resolveImage never returns one, but the sampler must
// not depend on that.
func TestImageSrcNilSource(t *testing.T) {
	s := newImageSrc(nil, 0, 0, 10, 10, 1)
	c := s.At(0, 0).(interface {
		RGBA() (r, g, b, a uint32)
	})
	_, _, _, a := c.RGBA()
	if a != 0 {
		t.Fatalf("nil source alpha = %d, want 0", a>>8)
	}
}

// lerpU8 corners interpolate exactly; the midpoint averages.
func TestLerpU8(t *testing.T) {
	if got := lerpU8(10, 20, 30, 40, 0, 0); got != 10 {
		t.Fatalf("corner (0,0) = %d, want 10", got)
	}
	if got := lerpU8(10, 20, 30, 40, 1, 1); got != 40 {
		t.Fatalf("corner (1,1) = %d, want 40", got)
	}
	// top=50, bot=125, 50+75*0.5=87.5 rounds to 88.
	if got := lerpU8(0, 100, 50, 200, 0.5, 0.5); got != 88 {
		t.Fatalf("midpoint = %d, want 88", got)
	}
}

func TestUnsupportedKindsAreSkipped(t *testing.T) {
	r := newRenderer(8, 8, 1)
	// RenderCustomShader is GLSL with no CPU equivalent, and
	// RenderFilterComposite is never emitted by the render path; both
	// must draw nothing and must not panic.
	r.drawAll([]gui.RenderCmd{
		{Kind: gui.RenderCustomShader, X: 0, Y: 0, W: 8, H: 8,
			Color: gui.RGB(255, 255, 255)},
		{Kind: gui.RenderFilterComposite, X: 0, Y: 0, W: 8, H: 8,
			Layers: 1},
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

func TestNewTextureRejectsUnboundedSize(t *testing.T) {
	gb := newGlyphBackend(1)
	if id := gb.NewTexture(0, 16); id != 0 {
		t.Errorf("zero width = %d, want 0 (rejected)", id)
	}
	if id := gb.NewTexture(16, -1); id != 0 {
		t.Errorf("negative height = %d, want 0 (rejected)", id)
	}
	if id := gb.NewTexture(8192, 8192); id != 0 {
		t.Errorf("huge texture = %d, want 0 (rejected)", id)
	}
	if id := gb.NewTexture(64, 64); id == 0 {
		t.Error("valid texture rejected")
	}
}

func TestDrawSvgDropsOversizedTriangles(t *testing.T) {
	r := newRenderer(8, 8, 1)
	// Past the shared 1_200_000-float cap: must draw nothing and
	// must not grow svgBatch to the attacker length.
	r.drawAll([]gui.RenderCmd{{Kind: gui.RenderSvg,
		Triangles: make([]float32, 1_200_006)}})
	if cap(r.svgBatch) > 1_200_000 {
		t.Fatalf("svgBatch cap = %d, want bounded", cap(r.svgBatch))
	}
}

// Non-finite, huge or negative geometry must draw nothing: deviceRect
// converts NaN, Inf and out-of-int-range values in an arch-specific
// way, and a negative box canonicalizes to a non-empty region, so the
// guards reject before region(). Each case paints the centre on a clean buffer when the
// guard fails.
func TestDrawShapesRejectNonFinite(t *testing.T) {
	nan := float32(math.NaN())
	inf := float32(math.Inf(1))
	green := gui.RGB(0, 255, 0)
	cases := map[string]gui.RenderCmd{
		"rect NaN x":      rectCmd(nan, 10, 20, 20, green),
		"rect Inf w":      rectCmd(10, 10, inf, 20, green),
		"rect NaN h":      rectCmd(10, 10, 20, nan, green),
		"stroke Inf w":    {Kind: gui.RenderStrokeRect, X: 10, Y: 10, W: inf, H: 20, Thickness: 2, Color: green},
		"stroke NaN y":    {Kind: gui.RenderStrokeRect, X: 10, Y: nan, W: 20, H: 20, Thickness: 2, Color: green},
		"circle Inf r":    {Kind: gui.RenderCircle, X: 20, Y: 20, Radius: inf, Color: green, Fill: true},
		"circle NaN r":    {Kind: gui.RenderCircle, X: 20, Y: 20, Radius: nan, Color: green, Fill: true},
		"circle Inf x":    {Kind: gui.RenderCircle, X: inf, Y: 20, Radius: 10, Color: green, Fill: true},
		"line Inf x1":     {Kind: gui.RenderLine, X: inf, Y: 20, OffsetX: 35, OffsetY: 20, Thickness: 2, Color: green},
		"line NaN x2":     {Kind: gui.RenderLine, X: 5, Y: 20, OffsetX: nan, OffsetY: 20, Thickness: 2, Color: green},
		"line Inf thick":  {Kind: gui.RenderLine, X: 5, Y: 20, OffsetX: 35, OffsetY: 20, Thickness: inf, Color: green},
		"line huge thick": {Kind: gui.RenderLine, X: 5, Y: 20, OffsetX: 35, OffsetY: 20, Thickness: 1e30, Color: green},
		"line NaN thick":  {Kind: gui.RenderLine, X: 5, Y: 20, OffsetX: 35, OffsetY: 20, Thickness: nan, Color: green},
		"line zero len":   {Kind: gui.RenderLine, X: 20, Y: 20, OffsetX: 20, OffsetY: 20, Thickness: 2, Color: green},
		"rect neg w":      rectCmd(30, 10, -20, 20, green),
		"stroke neg h":    {Kind: gui.RenderStrokeRect, X: 10, Y: 30, W: 20, H: -20, Thickness: 2, Color: green},
		"rect huge w":     rectCmd(10, 10, 1e30, 20, green),
		"rect huge x":     rectCmd(-1e30, 10, 20, 20, green),
		"circle huge r":   {Kind: gui.RenderCircle, X: 20, Y: 20, Radius: 1e30, Color: green, Fill: true},
		"line huge x1":    {Kind: gui.RenderLine, X: -1e30, Y: 20, OffsetX: 35, OffsetY: 20, Thickness: 2, Color: green},
	}
	for name, cmd := range cases {
		t.Run(name, func(t *testing.T) {
			r := newRenderer(40, 40, 1)
			r.drawAll([]gui.RenderCmd{cmd})
			// A non-finite thickness falls back to a 1px stroke and
			// paints; every other case must leave the buffer black.
			if name == "line NaN thick" || name == "line Inf thick" ||
				name == "line huge thick" {
				if _, g, _, _ := at(r.buf.img, 20, 20); g == 0 {
					t.Fatalf("non-finite thickness should fall back to 1px")
				}
				return
			}
			for y := range 40 {
				for x := range 40 {
					if cr, cg, cb, _ := at(r.buf.img, x, y); cr|cg|cb != 0 {
						t.Fatalf("pixel (%d,%d) painted by %s", x, y, name)
					}
				}
			}
		})
	}
}

// A NaN stroke thickness must take the fill branch, not feed NaN into
// the inset math: the rect paints solid, centre included.
func TestDrawStrokeRectNaNThicknessFills(t *testing.T) {
	r := newRenderer(40, 40, 1)
	r.drawAll([]gui.RenderCmd{{
		Kind: gui.RenderStrokeRect, X: 10, Y: 10, W: 20, H: 20,
		Thickness: float32(math.NaN()), Color: gui.RGB(0, 255, 0),
	}})
	if _, g, _, _ := at(r.buf.img, 20, 20); g == 0 {
		t.Fatalf("NaN thickness should fill the rect centre")
	}
	if _, g, _, _ := at(r.buf.img, 5, 5); g != 0 {
		t.Fatalf("fill leaked outside the rect")
	}
}
