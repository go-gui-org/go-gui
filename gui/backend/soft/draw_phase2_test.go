package soft

import (
	"image"
	"math"
	"testing"

	"github.com/go-gui-org/go-gui/gui"
)

// tri builds a RenderSvg command from a flat triangle list in SVG
// space. Scale 1 and origin 0 keep the vertex coordinates readable as
// device pixels.
func tri(triangles []float32, c gui.Color) gui.RenderCmd {
	return gui.RenderCmd{
		Kind: gui.RenderSvg, Scale: 1, Color: c, Triangles: triangles,
	}
}

// square returns the two triangles of an axis-aligned square.
func square(x, y, size float32) []float32 {
	return []float32{
		x, y, x + size, y, x + size, y + size,
		x, y, x + size, y + size, x, y + size,
	}
}

func TestDrawSvgFillsTriangles(t *testing.T) {
	r := newRenderer(40, 40, 1)
	r.drawAll([]gui.RenderCmd{tri(square(10, 10, 20), gui.RGB(0, 255, 0))})

	if _, g, _, _ := at(r.buf.img, 20, 20); g != 255 {
		t.Errorf("inside green = %d, want 255", g)
	}
	// The shared diagonal of the two triangles must not show as a
	// seam: a run is one path, so its interior edges cancel.
	if _, g, _, _ := at(r.buf.img, 15, 15); g != 255 {
		t.Errorf("on shared diagonal green = %d, want 255 (seam)", g)
	}
	if _, g, _, _ := at(r.buf.img, 5, 20); g != 0 {
		t.Errorf("outside green = %d, want 0", g)
	}
}

func TestDrawSvgHonoursScaleAndOrigin(t *testing.T) {
	r := newRenderer(40, 40, 2)
	cmd := tri(square(0, 0, 5), gui.RGB(255, 0, 0))
	cmd.X, cmd.Y = 5, 5
	cmd.Scale = 2 // 5..15 logical, 10..30 device
	r.drawAll([]gui.RenderCmd{cmd})

	if red, _, _, _ := at(r.buf.img, 20, 20); red != 255 {
		t.Errorf("inside red = %d, want 255", red)
	}
	if red, _, _, _ := at(r.buf.img, 8, 8); red != 0 {
		t.Errorf("outside red = %d, want 0", red)
	}
}

func TestDrawSvgAppliesXformAndRotation(t *testing.T) {
	r := newRenderer(40, 40, 1)
	cmd := tri(square(0, 0, 10), gui.RGB(0, 0, 255))
	// Translate to 20,20 then rotate 180 degrees about (25, 25):
	// the square lands back on 20..30 either way, which is only true
	// if translate runs before rotate.
	cmd.HasXform = true
	cmd.ScaleX, cmd.ScaleY = 1, 1
	cmd.TransX, cmd.TransY = 20, 20
	cmd.RotAngle = 180
	cmd.RotCX, cmd.RotCY = 25, 25
	r.drawAll([]gui.RenderCmd{cmd})

	if _, _, b, _ := at(r.buf.img, 25, 25); b != 255 {
		t.Errorf("rotated square blue = %d, want 255", b)
	}
	if _, _, b, _ := at(r.buf.img, 5, 5); b != 0 {
		t.Errorf("origin blue = %d, want 0 (transform not applied)", b)
	}
}

func TestDrawSvgVertexAlphaScale(t *testing.T) {
	r := newRenderer(40, 40, 1)
	cmd := tri(square(10, 10, 20), gui.Color{})
	white := gui.RGB(255, 255, 255)
	cmd.VertexColors = []gui.Color{
		white, white, white, white, white, white,
	}
	cmd.HasVertexAlpha = true
	cmd.VertexAlphaScale = 0.5
	r.drawAll([]gui.RenderCmd{cmd})

	// Half-alpha white over black is mid grey.
	if red, _, _, _ := at(r.buf.img, 20, 20); !closeTo(red, 127, 2) {
		t.Errorf("half-alpha white over black = %d, want ~127", red)
	}
}

func TestDrawSvgInterpolatesVertexColors(t *testing.T) {
	r := newRenderer(40, 40, 1)
	// One triangle: red at the left vertices, green at the right.
	cmd := tri([]float32{0, 0, 40, 0, 0, 40}, gui.Color{})
	cmd.VertexColors = []gui.Color{
		gui.RGB(255, 0, 0), gui.RGB(0, 255, 0), gui.RGB(255, 0, 0),
	}
	r.drawAll([]gui.RenderCmd{cmd})

	rl, gl, _, _ := at(r.buf.img, 2, 2)
	rr, gr, _, _ := at(r.buf.img, 35, 2)
	if rl <= rr {
		t.Errorf("red %d at left vs %d at right: not interpolated",
			rl, rr)
	}
	if gr <= gl {
		t.Errorf("green %d at right vs %d at left: not interpolated",
			gr, gl)
	}
}

func TestDrawSvgSkipsClipMask(t *testing.T) {
	r := newRenderer(40, 40, 1)
	cmd := tri(square(0, 0, 40), gui.RGB(255, 0, 0))
	cmd.IsClipMask = true
	r.drawAll([]gui.RenderCmd{cmd})

	if red, _, _, _ := at(r.buf.img, 20, 20); red != 0 {
		t.Errorf("clip-mask geometry painted red = %d, want 0", red)
	}
}

func TestDrawSvgDegenerateTriangleInMesh(t *testing.T) {
	r := newRenderer(40, 40, 1)
	// A valid triangle plus a degenerate one (all vertices identical),
	// which must be skipped, not divide by zero.
	verts := []float32{
		10, 10, 30, 10, 10, 30,
		20, 20, 20, 20, 20, 20,
	}
	white := gui.RGB(255, 255, 255)
	cmd := tri(verts, gui.Color{})
	cmd.VertexColors = []gui.Color{
		white, white, white, white, white, white,
	}
	r.drawAll([]gui.RenderCmd{cmd})

	if v, _, _, _ := at(r.buf.img, 15, 15); v != 255 {
		t.Errorf("inside the valid triangle = %d, want 255", v)
	}
	if v, _, _, _ := at(r.buf.img, 5, 5); v != 0 {
		t.Errorf("outside the mesh = %d, want 0", v)
	}
}

func TestDrawShadowFallsOffAndClearsCaster(t *testing.T) {
	r := newRenderer(60, 60, 1)
	r.drawAll([]gui.RenderCmd{{
		Kind: gui.RenderShadow, X: 20, Y: 20, W: 20, H: 20,
		OffsetX: 8, OffsetY: 8, BlurRadius: 4,
		Color: gui.RGB(255, 255, 255),
	}})

	// The shadow box is the caster offset by 8: 28..48 against the
	// caster's 20..40. Inside the shadow, clear of the caster.
	inShadow, _, _, _ := at(r.buf.img, 44, 44)
	if inShadow < 200 {
		t.Errorf("inside shadow = %d, want a near-opaque value",
			inShadow)
	}
	// Under the caster: erased.
	if v, _, _, _ := at(r.buf.img, 35, 35); v != 0 {
		t.Errorf("under caster = %d, want 0 (caster not erased)", v)
	}
	// Falloff: dimmer just outside the shadow box than well inside.
	edge, _, _, _ := at(r.buf.img, 50, 44)
	if edge == 0 || edge >= inShadow {
		t.Errorf("falloff edge = %d, inside = %d: want 0 < edge < inside",
			edge, inShadow)
	}
	// Far outside the expansion: untouched.
	if v, _, _, _ := at(r.buf.img, 58, 58); v != 0 {
		t.Errorf("beyond blur expansion = %d, want 0", v)
	}
}

func TestDrawBlurSoftensEdges(t *testing.T) {
	r := newRenderer(60, 60, 1)
	r.drawAll([]gui.RenderCmd{{
		Kind: gui.RenderBlur, X: 20, Y: 20, W: 20, H: 20,
		BlurRadius: 5, Color: gui.RGB(255, 255, 255),
	}})

	centre, _, _, _ := at(r.buf.img, 30, 30)
	edge, _, _, _ := at(r.buf.img, 21, 30)
	outside, _, _, _ := at(r.buf.img, 15, 30)
	if centre < 200 {
		t.Errorf("blur centre = %d, want near-opaque", centre)
	}
	if edge >= centre || edge == 0 {
		t.Errorf("blur edge = %d, centre = %d: want a soft ramp",
			edge, centre)
	}
	if outside >= edge {
		t.Errorf("outside = %d, edge = %d: falloff not monotonic",
			outside, edge)
	}
}

func TestStencilClipsToRoundedRect(t *testing.T) {
	r := newRenderer(40, 40, 1)
	r.drawAll([]gui.RenderCmd{
		{Kind: gui.RenderStencilBegin, X: 10, Y: 10, W: 20, H: 20,
			Radius: 10, StencilDepth: 1},
		rectCmd(0, 0, 40, 40, gui.RGB(0, 255, 0)),
		{Kind: gui.RenderStencilEnd, X: 10, Y: 10, W: 20, H: 20,
			Radius: 10, StencilDepth: 1},
	})

	if _, g, _, _ := at(r.buf.img, 20, 20); g != 255 {
		t.Errorf("centre of stencil green = %d, want 255", g)
	}
	// A radius of half the side makes a circle, so the corner of the
	// stencil box is outside it.
	if _, g, _, _ := at(r.buf.img, 11, 11); g != 0 {
		t.Errorf("stencil corner green = %d, want 0 (not rounded)", g)
	}
	if _, g, _, _ := at(r.buf.img, 35, 35); g != 0 {
		t.Errorf("outside stencil green = %d, want 0 (clip leaked)", g)
	}
}

func TestStencilNests(t *testing.T) {
	r := newRenderer(40, 40, 1)
	r.drawAll([]gui.RenderCmd{
		{Kind: gui.RenderStencilBegin, X: 0, Y: 0, W: 20, H: 40,
			StencilDepth: 1},
		{Kind: gui.RenderStencilBegin, X: 0, Y: 0, W: 40, H: 20,
			StencilDepth: 2},
		rectCmd(0, 0, 40, 40, gui.RGB(0, 255, 0)),
		{Kind: gui.RenderStencilEnd, X: 0, Y: 0, W: 40, H: 20,
			StencilDepth: 2},
		{Kind: gui.RenderStencilEnd, X: 0, Y: 0, W: 20, H: 40,
			StencilDepth: 1},
	})

	// Only the intersection of the two boxes survives.
	if _, g, _, _ := at(r.buf.img, 10, 10); g != 255 {
		t.Errorf("intersection green = %d, want 255", g)
	}
	if _, g, _, _ := at(r.buf.img, 30, 10); g != 0 {
		t.Errorf("outside outer stencil green = %d, want 0", g)
	}
	if _, g, _, _ := at(r.buf.img, 10, 30); g != 0 {
		t.Errorf("outside inner stencil green = %d, want 0", g)
	}
}

func TestRotationMovesContent(t *testing.T) {
	r := newRenderer(40, 40, 1)
	r.drawAll([]gui.RenderCmd{
		{Kind: gui.RenderRotateBegin, RotAngle: 90, RotCX: 20,
			RotCY: 20},
		rectCmd(0, 18, 20, 4, gui.RGB(0, 255, 0)),
		{Kind: gui.RenderRotateEnd},
	})

	// A horizontal bar left of centre becomes a vertical bar above
	// centre under a 90-degree turn in y-down device space.
	if _, g, _, _ := at(r.buf.img, 20, 10); g < 200 {
		t.Errorf("rotated bar green = %d, want near 255", g)
	}
	if _, g, _, _ := at(r.buf.img, 10, 20); g > 40 {
		t.Errorf("original bar position green = %d, want ~0", g)
	}
}

func TestRotationZeroAngleIsIdentity(t *testing.T) {
	r := newRenderer(40, 40, 1)
	r.drawAll([]gui.RenderCmd{
		{Kind: gui.RenderRotateBegin, RotCX: 20, RotCY: 20},
		rectCmd(10, 10, 20, 20, gui.RGB(0, 255, 0)),
		{Kind: gui.RenderRotateEnd},
	})
	if _, g, _, _ := at(r.buf.img, 20, 20); g != 255 {
		t.Errorf("unrotated green = %d, want 255", g)
	}
}

func TestFilterCompositesLayersAndColorMatrix(t *testing.T) {
	// A matrix that moves the red channel into green: column-major, so
	// index [row + 4*col] reads "green output from red input".
	var m [16]float32
	m[1] = 1 // out.g = in.r
	m[15] = 1

	r := newRenderer(40, 40, 1)
	r.drawAll([]gui.RenderCmd{
		{Kind: gui.RenderFilterBegin, Layers: 1, ColorMatrix: &m},
		rectCmd(10, 10, 20, 20, gui.RGB(255, 0, 0)),
		{Kind: gui.RenderFilterEnd},
	})

	red, green, _, alpha := at(r.buf.img, 20, 20)
	if green != 255 || red != 0 {
		t.Errorf("after colour matrix rgba = (%d,%d,_,%d), want red "+
			"swapped into green", red, green, alpha)
	}
}

func TestFilterLayersAccumulate(t *testing.T) {
	half := gui.RGBA(255, 255, 255, 128)

	one := newRenderer(20, 20, 1)
	one.drawAll([]gui.RenderCmd{
		{Kind: gui.RenderFilterBegin, Layers: 1},
		rectCmd(0, 0, 20, 20, half),
		{Kind: gui.RenderFilterEnd},
	})
	three := newRenderer(20, 20, 1)
	three.drawAll([]gui.RenderCmd{
		{Kind: gui.RenderFilterBegin, Layers: 3},
		rectCmd(0, 0, 20, 20, half),
		{Kind: gui.RenderFilterEnd},
	})

	v1, _, _, _ := at(one.buf.img, 10, 10)
	v3, _, _, _ := at(three.buf.img, 10, 10)
	if v3 <= v1 {
		t.Errorf("three layers = %d, one layer = %d: layers did not "+
			"accumulate", v3, v1)
	}
}

func TestFilterBlurSpreadsPastItsBox(t *testing.T) {
	r := newRenderer(60, 60, 1)
	r.drawAll([]gui.RenderCmd{
		{Kind: gui.RenderFilterBegin, Layers: 1, BlurRadius: 4},
		rectCmd(25, 25, 10, 10, gui.RGB(255, 255, 255)),
		{Kind: gui.RenderFilterEnd},
	})

	if v, _, _, _ := at(r.buf.img, 22, 30); v == 0 {
		t.Error("blur did not spread outside the drawn rect")
	}
	centre, _, _, _ := at(r.buf.img, 30, 30)
	outer, _, _, _ := at(r.buf.img, 22, 30)
	if outer >= centre {
		t.Errorf("outer = %d, centre = %d: blur is not a falloff",
			outer, centre)
	}
}

// TestFilterBlurRespectsClip guards the composite against leaking past
// the clip rect: the GPU backends' scissor survives the FBO bind, so a
// filter bracket inside a clip must not paint beyond it even when the
// blur spreads the layer's content past the clip edge.
func TestFilterBlurRespectsClip(t *testing.T) {
	r := newRenderer(60, 60, 1)
	r.drawAll([]gui.RenderCmd{
		{Kind: gui.RenderClip, X: 0, Y: 0, W: 40, H: 60},
		{Kind: gui.RenderFilterBegin, Layers: 1, BlurRadius: 4},
		// The rect crosses the clip at x=40, so only 34..40 draws.
		rectCmd(34, 20, 10, 20, gui.RGB(255, 255, 255)),
		{Kind: gui.RenderFilterEnd},
	})

	if v, _, _, _ := at(r.buf.img, 37, 30); v < 100 {
		t.Errorf("inside clip = %d, want bright content", v)
	}
	if v, _, _, _ := at(r.buf.img, 32, 30); v == 0 {
		t.Error("blur did not spread inside the clip")
	}
	if v, _, _, _ := at(r.buf.img, 42, 30); v != 0 {
		t.Errorf("past the clip = %d, want 0 (blur leaked)", v)
	}
}

// TestPopLayerPastDepthCapRestoresNearestLayer guards the bracket
// unwind past maxLayerDepth: a capped bracket has no layer and never
// switched the target, so popping it must land back on the nearest real
// layer, not jump to the root.
func TestPopLayerPastDepthCapRestoresNearestLayer(t *testing.T) {
	r := newRenderer(8, 8, 1)
	for range maxLayerDepth + 2 {
		r.beginStencil(&gui.RenderCmd{})
	}
	r.endStencil() // pops the last capped bracket
	if got := r.buf; got != r.brackets[15].layer {
		t.Fatalf("target after popping a capped bracket = %p, want "+
			"the last real layer %p", got, r.brackets[15].layer)
	}
	for range maxLayerDepth + 1 {
		r.endStencil()
	}
	if r.buf != r.rootBuf {
		t.Fatalf("target after full unwind = %p, want the root", r.buf)
	}
}

func TestValidTermGridRejectsOverflowingCellCount(t *testing.T) {
	// Cols*Rows overflows int64 to zero; the division-form check must
	// still reject a grid whose cell buffer is far too small, instead
	// of accepting it and looping over an empty buffer.
	tg := &gui.TermGridData{
		Cols: 1 << 40, Rows: 1 << 40,
		CellW: 10, CellH: 10,
		Cells: make([]gui.TermCell, 16),
	}
	if validTermGrid(tg) {
		t.Fatal("overflowing grid accepted")
	}
	full := &gui.TermGridData{
		Cols: 4, Rows: 4, CellW: 10, CellH: 10,
		Cells: make([]gui.TermCell, 16),
	}
	if !validTermGrid(full) {
		t.Fatal("valid 4x4 grid rejected")
	}
}

func TestUnbalancedBracketStillLands(t *testing.T) {
	r := newRenderer(40, 40, 1)
	r.drawAll([]gui.RenderCmd{
		{Kind: gui.RenderStencilBegin, X: 0, Y: 0, W: 40, H: 40,
			StencilDepth: 1},
		rectCmd(10, 10, 20, 20, gui.RGB(0, 255, 0)),
	})
	r.finishBrackets()

	if _, g, _, _ := at(r.buf.img, 20, 20); g != 255 {
		t.Errorf("content of unclosed bracket green = %d, want 255", g)
	}
}

func TestMismatchedEndDoesNotUnwind(t *testing.T) {
	r := newRenderer(40, 40, 1)
	r.drawAll([]gui.RenderCmd{
		{Kind: gui.RenderStencilBegin, X: 0, Y: 0, W: 40, H: 40,
			StencilDepth: 1},
		{Kind: gui.RenderFilterEnd}, // wrong kind: must be ignored
		rectCmd(10, 10, 20, 20, gui.RGB(0, 255, 0)),
	})
	if n := len(r.brackets); n != 1 {
		t.Fatalf("open brackets = %d, want 1", n)
	}
	r.finishBrackets()
	if _, g, _, _ := at(r.buf.img, 20, 20); g != 255 {
		t.Errorf("content green = %d, want 255", g)
	}
}

func TestLayersArePooled(t *testing.T) {
	r := newRenderer(40, 40, 1)
	stream := []gui.RenderCmd{
		{Kind: gui.RenderStencilBegin, X: 0, Y: 0, W: 40, H: 40,
			StencilDepth: 1},
		rectCmd(0, 0, 40, 40, gui.RGB(0, 255, 0)),
		{Kind: gui.RenderStencilEnd, X: 0, Y: 0, W: 40, H: 40,
			StencilDepth: 1},
	}
	r.drawAll(stream)
	r.drawAll(stream)

	if len(r.layerPool) != 1 {
		t.Errorf("pooled layers = %d, want 1 (a layer was not reused)",
			len(r.layerPool))
	}
}

func TestClearTransparentResetsPooledLayer(t *testing.T) {
	b := newBuffer(8, 8)
	b.clear(gui.RGB(255, 0, 0))
	b.clearTransparent()

	for y := range 8 {
		for x := range 8 {
			if _, _, _, a := at(b.img, x, y); a != 0 {
				t.Fatalf("pixel (%d,%d) alpha = %d, want 0", x, y, a)
			}
		}
	}
	if !b.dirty.Empty() {
		t.Errorf("dirty = %v, want empty", b.dirty)
	}
}

func TestBoxBlurAlphaConservesAndSpreads(t *testing.T) {
	rect := image.Rect(0, 0, 21, 21)
	m := &image.Alpha{
		Pix:    make([]uint8, 21*21),
		Stride: 21,
		Rect:   rect,
	}
	m.Pix[10*21+10] = 255

	var tmp []uint8
	boxBlurAlpha(m, &tmp, 2)

	if m.Pix[10*21+10] == 255 {
		t.Error("centre unchanged: blur did not run")
	}
	if m.Pix[10*21+12] == 0 {
		t.Error("blur did not spread to a neighbour")
	}
	if m.Pix[0] != 0 {
		t.Errorf("far corner = %d, want 0", m.Pix[0])
	}
}

// TestDrawSvgShadedMeshHasNoSeam guards the artifact a per-triangle
// rasterization produces: the two triangles of a gradient quad share a
// diagonal, and antialiasing it twice leaves each side ~50% covered, so
// the background shows through as a pale line. The mesh takes its
// coverage from one path, so the diagonal must be as opaque as the
// interior.
func TestDrawSvgShadedMeshHasNoSeam(t *testing.T) {
	r := newRenderer(40, 40, 1)

	// A 20x20 quad, red along the top edge and blue along the bottom,
	// split into two triangles that share the (30,10)-(10,30) diagonal.
	verts := []float32{
		10, 10, 30, 10, 10, 30,
		30, 10, 30, 30, 10, 30,
	}
	red, blue := gui.RGB(255, 0, 0), gui.RGB(0, 0, 255)
	cmd := tri(verts, gui.Color{})
	cmd.VertexColors = []gui.Color{
		red, red, blue,
		red, blue, blue,
	}
	r.drawAll([]gui.RenderCmd{cmd})

	// The pixels the diagonal actually crosses are the ones whose
	// centre lies on it: centre (x+0.5, y+0.5) with x+y == 39. Each
	// interpolates to some mix of red and blue over a black ground, so
	// the two channels sum to 255 wherever coverage is full. Splitting
	// the coverage between the two triangles halves that sum.
	for _, p := range []image.Point{{X: 14, Y: 25}, {X: 19, Y: 20},
		{X: 24, Y: 15}} {

		cr, _, cb, _ := at(r.buf.img, p.X, p.Y)
		if sum := int(cr) + int(cb); sum < 245 {
			t.Errorf("diagonal at %v: r+b = %d, want ~255 (seam)",
				p, sum)
		}
	}

	// The mesh still ends where it should, and still interpolates.
	if cr, _, cb, _ := at(r.buf.img, 20, 12); cr < 200 || cb > 60 {
		t.Errorf("top of mesh = (%d, %d), want mostly red", cr, cb)
	}
	if cr, _, cb, _ := at(r.buf.img, 20, 28); cb < 200 || cr > 60 {
		t.Errorf("bottom of mesh = (%d, %d), want mostly blue", cr, cb)
	}
	if cr, _, cb, _ := at(r.buf.img, 5, 5); cr != 0 || cb != 0 {
		t.Errorf("outside mesh = (%d, %d), want black", cr, cb)
	}
}

// --- Hostile geometry: nothing below may hang, panic or wrap ---

// TestHostileBlurRadiiTerminate drives every blur-consuming kind with a
// NaN, an infinite and an absurdly large radius. Blur radii reach this
// package from widget config and from an SVG filter's stdDev, neither
// of which is validated upstream; an unclamped radius sizes the box
// blur's sliding window, so the failure mode is a hang rather than a
// wrong pixel. A hang shows here as the test timeout.
func TestHostileBlurRadiiTerminate(t *testing.T) {
	for _, blur := range []float32{
		float32(math.NaN()),
		float32(math.Inf(1)),
		float32(math.Inf(-1)),
		1e9,
		-5,
	} {
		r := newRenderer(32, 32, 1)
		r.drawAll([]gui.RenderCmd{
			{Kind: gui.RenderShadow, X: 8, Y: 8, W: 16, H: 16,
				BlurRadius: blur, Color: gui.RGB(255, 0, 0)},
			{Kind: gui.RenderBlur, X: 8, Y: 8, W: 16, H: 16,
				BlurRadius: blur, Color: gui.RGB(0, 255, 0)},
			{Kind: gui.RenderFilterBegin, BlurRadius: blur, Layers: 1},
			rectCmd(8, 8, 16, 16, gui.RGB(0, 0, 255)),
			{Kind: gui.RenderFilterEnd},
		})
	}
}

// TestBlurPlanesRejectsNaNSigma pins the guard directly: a NaN sigma
// must leave the plane untouched rather than reach boxRadii, where the
// window bound would come from int(NaN).
func TestBlurPlanesRejectsNaNSigma(t *testing.T) {
	m := &image.Alpha{
		Pix:    []uint8{0, 255, 0, 0, 255, 0, 0, 255, 0},
		Stride: 3,
		Rect:   image.Rect(0, 0, 3, 3),
	}
	var tmp []uint8
	boxBlurAlpha(m, &tmp, float32(math.NaN()))
	if m.Pix[1] != 255 || m.Pix[0] != 0 {
		t.Errorf("NaN sigma altered the mask: %v", m.Pix)
	}
}

// TestShadowNaNRadiusDrawsSquare covers the corner-radius guard: a NaN
// radius must fall back to the square path rather than feed NaN control
// points to the rasterizer.
func TestShadowNaNRadiusDrawsSquare(t *testing.T) {
	r := newRenderer(32, 32, 1)
	// Offset the shadow clear of its caster, which would otherwise
	// erase all of it: an unoffset, unblurred shadow is fully hidden.
	r.drawAll([]gui.RenderCmd{
		{Kind: gui.RenderShadow, X: 8, Y: 8, W: 16, H: 16, OffsetX: 8,
			Radius: float32(math.NaN()), Color: gui.RGB(255, 0, 0)},
	})
	// The visible strip is painted, and so is the corner a real radius
	// would have cut away.
	if red, _, _, _ := at(r.buf.img, 28, 16); red != 255 {
		t.Errorf("shadow strip red = %d, want 255", red)
	}
	if red, _, _, _ := at(r.buf.img, 31, 9); red != 255 {
		t.Errorf("corner red = %d, want 255 (square fallback)", red)
	}
}

// TestShadowNaNSpreadDrawsNoRing covers the spread clamp: a NaN or
// negative spread must be clamped to zero by clampBlur rather than
// feed NaN or negative geometry into the rasterizer. The offset leaves
// a strip visible; a leaked spread would widen it on the far side.
func TestShadowNaNSpreadDrawsNoRing(t *testing.T) {
	r := newRenderer(32, 32, 1)
	r.drawAll([]gui.RenderCmd{
		{Kind: gui.RenderShadow, X: 6, Y: 8, W: 12, H: 12, OffsetX: 6,
			Spread: float32(math.NaN()), Color: gui.RGB(255, 0, 0)},
	})
	// Strip edge: shadow covers x 18..24 (the cut-out erases the
	// offset half), so 22 is painted and 25 is beyond — painted only
	// if the NaN spread grew the shadow.
	if red, _, _, _ := at(r.buf.img, 22, 16); red != 255 {
		t.Errorf("shadow strip red = %d, want 255", red)
	}
	if red, _, _, _ := at(r.buf.img, 25, 16); red != 0 {
		t.Errorf("NaN spread painted beyond the strip edge: red = %d", red)
	}

	r2 := newRenderer(32, 32, 1)
	r2.drawAll([]gui.RenderCmd{
		{Kind: gui.RenderShadow, X: 6, Y: 8, W: 12, H: 12, OffsetX: 6,
			Spread: -8, Color: gui.RGB(255, 0, 0)},
	})
	if red, _, _, _ := at(r2.buf.img, 22, 16); red != 255 {
		t.Errorf("negative spread clipped the strip: red = %d, want 255", red)
	}
}

// TestFilterLayerCountIsCapped guards the composite repeat count, which
// is the feMergeNode count of an untrusted SVG filter.
func TestFilterLayerCountIsCapped(t *testing.T) {
	r := newRenderer(8, 8, 1)
	r.beginFilter(&gui.RenderCmd{Layers: 1 << 20})
	if got := r.brackets[0].layers; got != maxFilterLayers {
		t.Errorf("layers = %d, want the %d cap", got, maxFilterLayers)
	}
	r.endFilter()

	r.beginFilter(&gui.RenderCmd{Layers: 0})
	if got := r.brackets[0].layers; got != 1 {
		t.Errorf("layers for an unset count = %d, want 1", got)
	}
	r.endFilter()
}

// TestOverPremulSaturates pins the blend clamp. A filter's color matrix
// clamps each channel on its own, so it can leave a pixel brighter than
// its own alpha; the unclamped sum passes 255 and wraps to near-black.
func TestOverPremulSaturates(t *testing.T) {
	if got := overPremul(255, 100, 255); got != 255 {
		t.Errorf("overPremul(255, 100, 255) = %d, want 255", got)
	}
	if got := overPremul(0, 200, 255); got != 200 {
		t.Errorf("overPremul(0, 200, 255) = %d, want 200", got)
	}
	if got := overPremul(60, 100, 128); got != 110 {
		t.Errorf("overPremul(60, 100, 128) = %d, want 110", got)
	}
}

// TestFilterColorMatrixDoesNotWrap is the end-to-end form of the clamp:
// a matrix that keeps a channel while zeroing alpha must saturate the
// destination, not wrap it darker than it started.
func TestFilterColorMatrixDoesNotWrap(t *testing.T) {
	r := newRenderer(16, 16, 1)
	// Column-major: out.r = in.a, every other output channel zero.
	var m [16]float32
	m[12] = 1

	r.drawAll([]gui.RenderCmd{
		rectCmd(0, 0, 16, 16, gui.RGB(100, 0, 0)),
		{Kind: gui.RenderFilterBegin, Layers: 1, ColorMatrix: &m},
		rectCmd(4, 4, 8, 8, gui.RGB(255, 255, 255)),
		{Kind: gui.RenderFilterEnd},
	})

	if red, _, _, _ := at(r.buf.img, 8, 8); red != 255 {
		t.Errorf("red = %d, want 255 saturated (wrapped past 255)", red)
	}
}

// TestFinishBracketsClosesEveryKind exercises all three end paths from
// the unwind, not just the stencil one a single-bracket stream reaches.
func TestFinishBracketsClosesEveryKind(t *testing.T) {
	r := newRenderer(40, 40, 1)
	r.drawAll([]gui.RenderCmd{
		{Kind: gui.RenderFilterBegin, Layers: 1},
		{Kind: gui.RenderStencilBegin, X: 0, Y: 0, W: 40, H: 40,
			StencilDepth: 1},
		{Kind: gui.RenderRotateBegin, RotAngle: 0, RotCX: 20, RotCY: 20},
		rectCmd(10, 10, 20, 20, gui.RGB(0, 255, 0)),
	})
	if n := len(r.brackets); n != 3 {
		t.Fatalf("open brackets = %d, want 3", n)
	}
	r.finishBrackets()

	if n := len(r.brackets); n != 0 {
		t.Fatalf("brackets after unwind = %d, want 0", n)
	}
	if r.buf != r.rootBuf {
		t.Fatal("target after unwind is not the root buffer")
	}
	if _, g, _, _ := at(r.buf.img, 20, 20); g != 255 {
		t.Errorf("content green = %d, want 255", g)
	}
}
