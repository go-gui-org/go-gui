package gui

import (
	"math"
	"testing"

	"github.com/go-gui-org/go-glyph"
)

func TestDimColorOrdersOpacityBeforeDisabled(t *testing.T) {
	c := dimColor(RGBA(10, 20, 30, 200), 0.5, true)
	// 200*0.5=100, then halved to 50. RGB untouched.
	if c != RGBA(10, 20, 30, 50) {
		t.Fatalf("got %v, want alpha 50", c)
	}
}

func TestDimColorFastPathIsIdentity(t *testing.T) {
	c := RGBA(10, 20, 30, 200)
	if got := dimColor(c, 1.0, false); got != c {
		t.Fatalf("enabled full-opacity must not touch color: %v", got)
	}
	if got := dimColor(c, float32(math.NaN()), false); got != c {
		t.Fatalf("NaN opacity must apply nothing, like renderShape: %v",
			got)
	}
}

func TestDimmedGradientFastPathKeepsPointer(t *testing.T) {
	def := &GradientDef{Stops: []GradientStop{
		{Color: RGBA(255, 0, 0, 255), Pos: 0},
		{Color: RGBA(0, 0, 255, 255), Pos: 1},
	}}
	if dimmedGradient(nil, 0.5, true) != nil {
		t.Fatal("nil gradient must stay nil")
	}
	if got := dimmedGradient(def, 1.0, false); got != def {
		t.Fatal("enabled gradient must keep its pointer (no copy)")
	}
}

func TestDimmedGradientDimsStopsWithoutMutatingSource(t *testing.T) {
	def := &GradientDef{Stops: []GradientStop{
		{Color: RGBA(255, 0, 0, 200), Pos: 0},
	}}
	got := dimmedGradient(def, 0.5, true)
	if got == def {
		t.Fatal("dimmed gradient must be a copy, not the source")
	}
	// 200*0.5=100, halved to 50.
	if got.Stops[0].Color.A != 50 {
		t.Fatalf("got alpha %d, want 50", got.Stops[0].Color.A)
	}
	if def.Stops[0].Color.A != 200 {
		t.Fatalf("source stops must not mutate: %d",
			def.Stops[0].Color.A)
	}
}

func TestDimmedVColorsFastPathKeepsSlice(t *testing.T) {
	w := &Window{}
	vcols := []Color{RGBA(1, 2, 3, 200)}
	if got := dimmedVColors(vcols, 1.0, false, w); &got[0] != &vcols[0] {
		t.Fatal("enabled batches must keep their backing array")
	}
}

func TestDimmedTextGradientScalesGlyphAlpha(t *testing.T) {
	cfg := &glyph.GradientConfig{Stops: []glyph.GradientStop{
		{Color: glyph.Color{R: 255, A: 200}},
	}}
	if dimmedTextGradient(nil, 0.5, true) != nil {
		t.Fatal("nil text gradient must stay nil")
	}
	if got := dimmedTextGradient(cfg, 1.0, false); got != cfg {
		t.Fatal("enabled text gradient must keep its pointer")
	}
	got := dimmedTextGradient(cfg, 0.5, true)
	if got.Stops[0].Color.A != 50 {
		t.Fatalf("got alpha %d, want 50", got.Stops[0].Color.A)
	}
	if cfg.Stops[0].Color.A != 200 {
		t.Fatal("source glyph stops must not mutate")
	}
}

func TestRenderShapeDisabledDimsShadow(t *testing.T) {
	w := &Window{}
	shape := &Shape{
		shapeType: shapeRectangle,
		X:         0, Y: 0, Width: 80, Height: 60,
		Color:   RGBA(200, 200, 200, 255),
		Opacity: 1.0, Disabled: true,
		fx: &shapeEffects{
			Shadow: &BoxShadow{
				Color:   RGBA(0, 0, 0, 80),
				OffsetX: 3, OffsetY: 5, BlurRadius: 20,
			},
		},
	}
	renderShape(shape, ColorTransparent, makeClip(0, 0, 500, 500), w)
	for _, r := range w.renderers {
		if r.Kind == RenderShadow && r.Color.A != 40 {
			t.Fatalf("disabled shadow must halve: got %d", r.Color.A)
		}
		if r.Kind == RenderRect && r.Color.A != 127 {
			t.Fatalf("disabled rect must halve: got %d", r.Color.A)
		}
	}
}

func TestRenderShapeOpacityReachesShadow(t *testing.T) {
	w := &Window{}
	shape := &Shape{
		shapeType: shapeRectangle,
		X:         0, Y: 0, Width: 80, Height: 60,
		Color:   RGBA(200, 200, 200, 255),
		Opacity: 0.5,
		fx: &shapeEffects{
			Shadow: &BoxShadow{
				Color:   RGBA(0, 0, 0, 80),
				OffsetX: 3, OffsetY: 5, BlurRadius: 20,
			},
		},
	}
	renderShape(shape, ColorTransparent, makeClip(0, 0, 500, 500), w)
	for _, r := range w.renderers {
		if r.Kind == RenderShadow && r.Color.A != 40 {
			t.Fatalf("faded shadow must scale: got %d", r.Color.A)
		}
	}
}

func TestRenderShapeDisabledDimsImageFill(t *testing.T) {
	w := &Window{}
	shape := &Shape{
		shapeType: shapeImage,
		X:         10, Y: 20, Width: 100, Height: 80,
		Color:    RGBA(255, 255, 255, 200),
		Opacity:  1.0,
		Disabled: true,
		Resource: "test.png",
	}
	renderShape(shape, ColorTransparent, makeClip(0, 0, 500, 500), w)
	found := false
	for _, r := range w.renderers {
		if r.Kind == RenderImage {
			found = true
			if r.Color.A != 100 {
				t.Fatalf("disabled image fill must halve: got %d",
					r.Color.A)
			}
		}
	}
	if !found {
		t.Fatal("no RenderImage emitted")
	}
}

// A disabled image must dim its emitted fill without writing the
// dimmed color back onto the shape: renderShape only restores
// shape.Color when Opacity < 1, so a write-back would halve again
// on every later frame the shape survives.
func TestRenderShapeDisabledImageKeepsShapeColor(t *testing.T) {
	w := &Window{}
	orig := RGBA(255, 255, 255, 200)
	shape := &Shape{
		shapeType: shapeImage,
		X:         10, Y: 20, Width: 100, Height: 80,
		Color:    orig,
		Opacity:  1.0,
		Disabled: true,
		Resource: "test.png",
	}
	for frame := range 3 {
		w.renderers = w.renderers[:0]
		renderShape(shape, ColorTransparent, makeClip(0, 0, 500, 500), w)
		if shape.Color != orig {
			t.Fatalf("frame %d: shape.Color = %v, want %v",
				frame, shape.Color, orig)
		}
		for _, r := range w.renderers {
			if r.Kind == RenderImage && r.Color.A != 100 {
				t.Fatalf("frame %d: fill alpha = %d, want 100",
					frame, r.Color.A)
			}
		}
	}
}

func TestRenderShapeDisabledDimsGradientFill(t *testing.T) {
	w := &Window{}
	shape := &Shape{
		shapeType: shapeRectangle,
		X:         0, Y: 0, Width: 80, Height: 60,
		Color:   RGBA(200, 200, 200, 255),
		Opacity: 1.0, Disabled: true,
		fx: &shapeEffects{
			Gradient: &GradientDef{Stops: []GradientStop{
				{Color: RGBA(255, 0, 0, 200), Pos: 0},
			}},
		},
	}
	renderShape(shape, ColorTransparent, makeClip(0, 0, 500, 500), w)
	found := false
	for _, r := range w.renderers {
		if r.Kind == RenderGradient {
			found = true
			if r.Gradient.Stops[0].Color.A != 100 {
				t.Fatalf("disabled gradient must halve: got %d",
					r.Gradient.Stops[0].Color.A)
			}
		}
	}
	if !found {
		t.Fatal("no RenderGradient emitted")
	}
	if shape.fx.Gradient.Stops[0].Color.A != 200 {
		t.Fatal("source gradient def must not mutate")
	}
}

func TestEmitSvgPathUntintedOpacityScalesAlphaOnly(t *testing.T) {
	w := &Window{}
	path := cachedSvgPath{
		Triangles: []float32{0, 0, 1, 0, 0, 1},
		Color:     RGBA(10, 20, 30, 200),
		VertexColors: []Color{
			{R: 100, G: 110, B: 120, A: 200},
			{R: 100, G: 110, B: 120, A: 200},
			{R: 100, G: 110, B: 120, A: 200},
		},
	}
	shape := &Shape{Opacity: 0.5}
	emitSvgPathRenderer(path, Color{}, 0, 0, 1, 0, 0,
		false, nil, shape, w)
	if len(w.renderers) != 1 {
		t.Fatalf("expected 1 renderer, got %d", len(w.renderers))
	}
	rc := w.renderers[0]
	// Untinted + faded: flat color scales, vertex RGB preserved.
	if rc.Color.A != 100 {
		t.Fatalf("flat path alpha must scale: got %d", rc.Color.A)
	}
	if len(rc.VertexColors) != 3 {
		t.Fatalf("expected 3 vertex colors, got %d",
			len(rc.VertexColors))
	}
	vc := rc.VertexColors[0]
	if vc.R != 100 || vc.G != 110 || vc.B != 120 || vc.A != 100 {
		t.Fatalf("vertex RGB must survive, alpha scales: got %v", vc)
	}
	if path.VertexColors[0].A != 200 {
		t.Fatal("cached vertex colors must not mutate")
	}
}

func TestEmitDrawCanvasGeometryDisabledDimsBatches(t *testing.T) {
	w := &Window{}
	cached := &drawCanvasCache{
		Batches: []DrawCanvasTriBatch{{
			Triangles: []float32{0, 0, 1, 0, 0, 1},
			Color:     RGBA(10, 20, 30, 200),
			VertexColors: []Color{
				{R: 1, G: 2, B: 3, A: 200},
				{R: 1, G: 2, B: 3, A: 200},
				{R: 1, G: 2, B: 3, A: 200},
			},
		}},
	}
	shape := &Shape{Opacity: 1.0, Disabled: true}
	emitDrawCanvasGeometry(cached, 0, 0, shape, w)
	if len(w.renderers) != 1 {
		t.Fatalf("expected 1 renderer, got %d", len(w.renderers))
	}
	rc := w.renderers[0]
	if rc.Color.A != 100 || rc.VertexColors[0].A != 100 {
		t.Fatalf("disabled canvas batch must halve: %v %v",
			rc.Color, rc.VertexColors[0])
	}
	if cached.Batches[0].Color.A != 200 ||
		cached.Batches[0].VertexColors[0].A != 200 {
		t.Fatal("cache entry must not mutate")
	}
}
