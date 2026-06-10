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

func TestRendererValidGradientBorder(t *testing.T) {
	gd := &GradientDef{}
	r := RenderCmd{Kind: RenderGradientBorder, W: 10, H: 10, Thickness: 2,
		Gradient: gd}
	if !rendererValidForDraw(r) {
		t.Error("valid gradient border should pass")
	}
	r.Gradient = nil
	if rendererValidForDraw(r) {
		t.Error("nil gradient should fail")
	}
	r.Gradient = gd
	r.Thickness = 0
	if rendererValidForDraw(r) {
		t.Error("zero thickness should fail")
	}
}

func TestRendererValidShadow(t *testing.T) {
	r := RenderCmd{Kind: RenderShadow, W: 10, H: 10,
		BlurRadius: 4, Radius: 3, OffsetX: 2, OffsetY: 2}
	if !rendererValidForDraw(r) {
		t.Error("valid shadow should pass")
	}
	r.W = -1
	if rendererValidForDraw(r) {
		t.Error("negative width should fail")
	}
	r.W = 10
	r.OffsetX = float32(math.NaN())
	if rendererValidForDraw(r) {
		t.Error("NaN offset should fail")
	}
}

func TestRendererValidBlur(t *testing.T) {
	r := RenderCmd{Kind: RenderBlur, W: 10, H: 10, BlurRadius: 4}
	if !rendererValidForDraw(r) {
		t.Error("valid blur should pass")
	}
	r.H = -1
	if rendererValidForDraw(r) {
		t.Error("negative height should fail")
	}
	r.H = 10
	r.BlurRadius = float32(math.Inf(1))
	if rendererValidForDraw(r) {
		t.Error("Inf blur radius should fail")
	}
}

func TestRendererValidCustomShader(t *testing.T) {
	sh := &Shader{Metal: "..."}
	r := RenderCmd{Kind: RenderCustomShader, W: 10, H: 10, Shader: sh}
	if !rendererValidForDraw(r) {
		t.Error("valid custom shader should pass")
	}
	r.Shader = nil
	if rendererValidForDraw(r) {
		t.Error("nil shader should fail")
	}
	r.Shader = sh
	r.W = 0
	if rendererValidForDraw(r) {
		t.Error("zero width should fail (W > 0 required)")
	}
}

func TestRendererValidRotateBegin(t *testing.T) {
	r := RenderCmd{Kind: RenderRotateBegin,
		RotAngle: 0.5, RotCX: 100, RotCY: 200}
	if !rendererValidForDraw(r) {
		t.Error("valid rotate begin should pass")
	}
	r.RotAngle = float32(math.NaN())
	if rendererValidForDraw(r) {
		t.Error("NaN angle should fail")
	}
	r.RotAngle = 0.5
	r.RotCX = float32(math.Inf(1))
	if rendererValidForDraw(r) {
		t.Error("Inf center X should fail")
	}
}

func TestRendererValidSvg_ZeroScale(t *testing.T) {
	tris := make([]float32, 6)
	r := RenderCmd{Kind: RenderSvg, Scale: 0, Triangles: tris}
	if rendererValidForDraw(r) {
		t.Error("zero scale should fail")
	}
	r.Scale = -1
	if rendererValidForDraw(r) {
		t.Error("negative scale should fail")
	}
}

func TestRendererValidSvg_NaNTriangles(t *testing.T) {
	tris := []float32{1, 2, 3, 4, 5, float32(math.NaN())}
	r := RenderCmd{Kind: RenderSvg, Scale: 1, Triangles: tris}
	if rendererValidForDraw(r) {
		t.Error("NaN in triangles should fail")
	}
}

func TestRendererValidImage_NaNClipRadius(t *testing.T) {
	r := RenderCmd{Kind: RenderImage, W: 10, H: 10,
		ClipRadius: float32(math.NaN())}
	if rendererValidForDraw(r) {
		t.Error("NaN clip radius should fail")
	}
}

func TestRendererValidClip_ZeroSize(t *testing.T) {
	r := RenderCmd{Kind: RenderClip, W: 0, H: 0}
	if !rendererValidForDraw(r) {
		t.Error("zero-size clip should pass (W >= 0, H >= 0)")
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
