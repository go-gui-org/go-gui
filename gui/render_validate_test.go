package gui

import (
	"math"
	"testing"

	"github.com/go-gui-org/go-glyph"
)

func TestRendererValidClip(t *testing.T) {
	r := RenderCmd{Kind: RenderClip, X: 0, Y: 0, W: 10, H: 10}
	if !rendererValidForDraw(r) {
		t.Error("valid clip should pass")
	}
	r.W = -1
	if rendererValidForDraw(r) {
		t.Error("negative width should fail")
	}
}

func TestRendererValidRect(t *testing.T) {
	r := RenderCmd{Kind: RenderRect, X: 1, Y: 2, W: 10, H: 10, Radius: 3}
	if !rendererValidForDraw(r) {
		t.Error("valid rect should pass")
	}
	r.X = float32(math.NaN())
	if rendererValidForDraw(r) {
		t.Error("NaN X should fail")
	}
}

func TestRendererValidStrokeRect(t *testing.T) {
	r := RenderCmd{Kind: RenderStrokeRect, W: 10, H: 10, Radius: 1, Thickness: 2}
	if !rendererValidForDraw(r) {
		t.Error("valid stroke rect should pass")
	}
	r.Thickness = 0
	if rendererValidForDraw(r) {
		t.Error("zero thickness should fail")
	}
}

func TestRendererValidGradient(t *testing.T) {
	gd := &GradientDef{}
	r := RenderCmd{Kind: RenderGradient, W: 10, H: 10, Gradient: gd}
	if !rendererValidForDraw(r) {
		t.Error("valid gradient should pass")
	}
	r.Gradient = nil
	if rendererValidForDraw(r) {
		t.Error("nil gradient should fail")
	}
}

func TestRendererValidCircle(t *testing.T) {
	r := RenderCmd{Kind: RenderCircle, X: 5, Y: 5, Radius: 10}
	if !rendererValidForDraw(r) {
		t.Error("valid circle should pass")
	}
	r.Radius = 0
	if rendererValidForDraw(r) {
		t.Error("zero radius should fail")
	}
}

func TestRendererValidText(t *testing.T) {
	r := RenderCmd{Kind: RenderText, X: 0, Y: 0, Text: "hello"}
	if !rendererValidForDraw(r) {
		t.Error("valid text should pass")
	}
	r.Text = ""
	if rendererValidForDraw(r) {
		t.Error("empty text should fail")
	}
}

func TestRendererValidLayout(t *testing.T) {
	ly := &glyph.Layout{}
	r := RenderCmd{Kind: RenderLayout, X: 0, Y: 0, LayoutPtr: ly}
	if !rendererValidForDraw(r) {
		t.Error("valid layout should pass")
	}
	r.LayoutPtr = nil
	if rendererValidForDraw(r) {
		t.Error("nil layout should fail")
	}
}

func TestRendererValidLayoutTransformed(t *testing.T) {
	ly := &glyph.Layout{}
	tr := &glyph.AffineTransform{}
	r := RenderCmd{
		Kind:            RenderLayoutTransformed,
		LayoutPtr:       ly,
		LayoutTransform: tr,
	}
	if !rendererValidForDraw(r) {
		t.Error("valid transformed layout should pass")
	}
	r.LayoutTransform = nil
	if rendererValidForDraw(r) {
		t.Error("nil transform should fail")
	}
}

func TestRendererValidImage(t *testing.T) {
	r := RenderCmd{Kind: RenderImage, W: 10, H: 10, ClipRadius: 0}
	if !rendererValidForDraw(r) {
		t.Error("valid image should pass")
	}
	r.W = 0
	if rendererValidForDraw(r) {
		t.Error("zero-width image should fail")
	}
}

func TestRendererValidSvg(t *testing.T) {
	tris := make([]float32, 6)
	r := RenderCmd{
		Kind:      RenderSvg,
		Scale:     1,
		Triangles: tris,
	}
	if !rendererValidForDraw(r) {
		t.Error("valid SVG should pass")
	}
}

func TestRendererValidSvgBadTriangles(t *testing.T) {
	r := RenderCmd{Kind: RenderSvg, Scale: 1, Triangles: nil}
	if rendererValidForDraw(r) {
		t.Error("nil triangles should fail")
	}
	r.Triangles = make([]float32, 5) // not divisible by 6
	if rendererValidForDraw(r) {
		t.Error("triangle count not divisible by 6 should fail")
	}
}

func TestRendererValidSvgVertexAlpha(t *testing.T) {
	tris := make([]float32, 6)
	r := RenderCmd{
		Kind:             RenderSvg,
		Scale:            1,
		Triangles:        tris,
		HasVertexAlpha:   true,
		VertexAlphaScale: 0.5,
	}
	if !rendererValidForDraw(r) {
		t.Error("valid vertex alpha should pass")
	}
	r.VertexAlphaScale = 2.0
	if rendererValidForDraw(r) {
		t.Error("vertex alpha > 1 should fail")
	}
	r.VertexAlphaScale = -0.1
	if rendererValidForDraw(r) {
		t.Error("negative vertex alpha should fail")
	}
}

func TestRendererValidSvgVertexColors(t *testing.T) {
	tris := make([]float32, 12) // 12/6 = 2 triangles
	r := RenderCmd{
		Kind:         RenderSvg,
		Scale:        1,
		Triangles:    tris,
		VertexColors: make([]Color, 6), // 6*2 = 12 == len(tris)
	}
	if !rendererValidForDraw(r) {
		t.Error("matching vertex colors should pass")
	}
	r.VertexColors = make([]Color, 5) // mismatch
	if rendererValidForDraw(r) {
		t.Error("mismatched vertex color count should fail")
	}
}

func TestRendererValidFilterComposite(t *testing.T) {
	r := RenderCmd{Kind: RenderFilterComposite, W: 10, H: 10, Layers: 2}
	if !rendererValidForDraw(r) {
		t.Error("valid filter composite should pass")
	}
	r.Layers = 0
	if rendererValidForDraw(r) {
		t.Error("zero layers should fail")
	}
}

func TestRendererValidStencil(t *testing.T) {
	r := RenderCmd{
		Kind:         RenderStencilBegin,
		W:            10,
		H:            10,
		StencilDepth: 1,
	}
	if !rendererValidForDraw(r) {
		t.Error("valid stencil begin should pass")
	}
	r.StencilDepth = 0
	if rendererValidForDraw(r) {
		t.Error("zero stencil depth should fail")
	}
}

func TestRendererValidStencilEnd(t *testing.T) {
	r := RenderCmd{
		Kind:         RenderStencilEnd,
		W:            10,
		H:            10,
		StencilDepth: 1,
	}
	if !rendererValidForDraw(r) {
		t.Error("valid stencil end should pass")
	}
}

func TestRendererValidUnknown(t *testing.T) {
	r := RenderCmd{Kind: RenderKind(255)}
	if !rendererValidForDraw(r) {
		t.Error("unknown kind should pass (default case returns true)")
	}
}

func TestRendererValidInfCoord(t *testing.T) {
	inf := float32(math.Inf(1))
	r := RenderCmd{Kind: RenderRect, X: inf, W: 10, H: 10}
	if rendererValidForDraw(r) {
		t.Error("+Inf coordinate should fail")
	}
}

func TestF32AllFinite(t *testing.T) {
	if !f32AllFinite([]float32{1, 2, 3}) {
		t.Error("finite values should pass")
	}
	if f32AllFinite([]float32{1, float32(math.NaN()), 3}) {
		t.Error("NaN should fail")
	}
	if !f32AllFinite(nil) {
		t.Error("nil slice should pass")
	}
	if !f32AllFinite([]float32{}) {
		t.Error("empty slice should pass")
	}
}
