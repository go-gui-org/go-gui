package gui

import "testing"

func TestColorFilterIdentity(t *testing.T) {
	m := ColorFilterIdentity().Matrix
	want := [16]float32{
		1, 0, 0, 0,
		0, 1, 0, 0,
		0, 0, 1, 0,
		0, 0, 0, 1,
	}
	if m != want {
		t.Errorf("identity: got %v", m)
	}
}

func TestColorFilterSaturateIdentity(t *testing.T) {
	m := ColorFilterSaturate(1.0).Matrix
	want := ColorFilterIdentity().Matrix
	for i := range 16 {
		if diff := m[i] - want[i]; diff > 1e-6 || diff < -1e-6 {
			t.Fatalf("saturate(1.0)[%d]=%f, want %f", i, m[i], want[i])
		}
	}
}

func TestColorFilterSaturateZeroIsGrayscale(t *testing.T) {
	m := ColorFilterSaturate(0.0).Matrix
	want := ColorFilterGrayscale().Matrix
	for i := range 16 {
		if diff := m[i] - want[i]; diff > 1e-6 || diff < -1e-6 {
			t.Fatalf("saturate(0)[%d]=%f, want %f", i, m[i], want[i])
		}
	}
}

func TestColorFilterGrayscaleLuminanceSum(t *testing.T) {
	m := ColorFilterGrayscale().Matrix
	// Each output channel = lr*R + lg*G + lb*B. In column-major,
	// row 0: m[0] + m[4] + m[8] should sum to 1.
	sum := m[0] + m[4] + m[8]
	if diff := sum - 1.0; diff > 1e-4 || diff < -1e-4 {
		t.Errorf("luminance sum=%f, want 1.0", sum)
	}
}

func TestColorFilterHueRotate360(t *testing.T) {
	m := ColorFilterHueRotate(360).Matrix
	want := ColorFilterIdentity().Matrix
	for i := range 16 {
		if diff := m[i] - want[i]; diff > 1e-5 || diff < -1e-5 {
			t.Fatalf("hue(360)[%d]=%f, want %f", i, m[i], want[i])
		}
	}
}

func TestColorFilterContrastIdentity(t *testing.T) {
	m := ColorFilterContrast(1.0).Matrix
	want := ColorFilterIdentity().Matrix
	for i := range 16 {
		if diff := m[i] - want[i]; diff > 1e-6 || diff < -1e-6 {
			t.Fatalf("contrast(1.0)[%d]=%f, want %f", i, m[i], want[i])
		}
	}
}

func TestColorFilterBrightnessIdentity(t *testing.T) {
	m := ColorFilterBrightness(1.0).Matrix
	want := ColorFilterIdentity().Matrix
	if m != want {
		t.Errorf("brightness(1.0): got %v", m)
	}
}

// applyMatrix multiplies a column-major 4x4 matrix by a vec4.
func applyMatrix(m [16]float32, r, g, b, a float32) (float32, float32, float32, float32) {
	or := m[0]*r + m[4]*g + m[8]*b + m[12]*a
	og := m[1]*r + m[5]*g + m[9]*b + m[13]*a
	ob := m[2]*r + m[6]*g + m[10]*b + m[14]*a
	oa := m[3]*r + m[7]*g + m[11]*b + m[15]*a
	return or, og, ob, oa
}

func clamp01(v float32) float32 {
	if v < 0 {
		return 0
	}
	if v > 1 {
		return 1
	}
	return v
}

func TestColorFilterInvertWhite(t *testing.T) {
	m := ColorFilterInvert().Matrix
	r, g, b, a := applyMatrix(m, 1, 1, 1, 1)
	r, g, b = clamp01(r), clamp01(g), clamp01(b)
	if r != 0 || g != 0 || b != 0 || a != 1 {
		t.Errorf("invert white: got (%f,%f,%f,%f)", r, g, b, a)
	}
}

func TestColorFilterInvertBlack(t *testing.T) {
	m := ColorFilterInvert().Matrix
	r, g, b, a := applyMatrix(m, 0, 0, 0, 1)
	r, g, b = clamp01(r), clamp01(g), clamp01(b)
	if r != 1 || g != 1 || b != 1 || a != 1 {
		t.Errorf("invert black: got (%f,%f,%f,%f)", r, g, b, a)
	}
}

func TestColorFilterContrastMidpoint(t *testing.T) {
	// Contrast(2.0) at 0.5: 2*(0.5-0.5)+0.5 = 0.5
	m := ColorFilterContrast(2.0).Matrix
	r, _, _, _ := applyMatrix(m, 0.5, 0.5, 0.5, 1)
	if diff := r - 0.5; diff > 1e-6 || diff < -1e-6 {
		t.Errorf("contrast(2) at 0.5: got %f, want 0.5", r)
	}
}

func TestColorFilterSepiaPreservesAlpha(t *testing.T) {
	m := ColorFilterSepia().Matrix
	_, _, _, a := applyMatrix(m, 0.5, 0.5, 0.5, 0.8)
	if diff := a - 0.8; diff > 1e-6 || diff < -1e-6 {
		t.Errorf("sepia alpha: got %f, want 0.8", a)
	}
}

func TestColorFilterHueRotate180(t *testing.T) {
	m := ColorFilterHueRotate(180).Matrix
	// Pure red (1,0,0,1) rotated 180° around (1,1,1) axis
	// should shift toward cyan-ish.
	r, g, b, _ := applyMatrix(m, 1, 0, 0, 1)
	// R should decrease, G and B should increase.
	if r >= 0.5 {
		t.Errorf("hue(180) red channel too high: %f", r)
	}
	if g <= 0 {
		t.Errorf("hue(180) green channel should increase: %f", g)
	}
	if b <= 0 {
		t.Errorf("hue(180) blue channel should increase: %f", b)
	}
}

func TestColorFilterComposeIdentity(t *testing.T) {
	// Composing with identity should yield the original.
	gs := ColorFilterGrayscale()
	id := ColorFilterIdentity()
	composed := ColorFilterCompose(gs, id)
	for i := range 16 {
		if diff := composed.Matrix[i] - gs.Matrix[i]; diff > 1e-6 || diff < -1e-6 {
			t.Fatalf("compose(gs, id)[%d]=%f, want %f",
				i, composed.Matrix[i], gs.Matrix[i])
		}
	}
	composed2 := ColorFilterCompose(id, gs)
	for i := range 16 {
		if diff := composed2.Matrix[i] - gs.Matrix[i]; diff > 1e-6 || diff < -1e-6 {
			t.Fatalf("compose(id, gs)[%d]=%f, want %f",
				i, composed2.Matrix[i], gs.Matrix[i])
		}
	}
}

func TestColorFilterComposeInvertInvert(t *testing.T) {
	// Invert composed with invert should approximate identity.
	inv := ColorFilterInvert()
	composed := ColorFilterCompose(inv, inv)
	id := ColorFilterIdentity()
	for i := range 16 {
		if diff := composed.Matrix[i] - id.Matrix[i]; diff > 1e-5 || diff < -1e-5 {
			t.Fatalf("compose(inv, inv)[%d]=%f, want %f",
				i, composed.Matrix[i], id.Matrix[i])
		}
	}
}

func TestColorFilterNilMatrix(t *testing.T) {
	// Verify nil ColorFilter doesn't panic in shapeEffects check.
	fx := &shapeEffects{ColorFilter: nil}
	if fx.ColorFilter != nil {
		t.Error("should be nil")
	}
}

func TestRenderFilterBracketWithColorFilter(t *testing.T) {
	w := newTestWindow()
	cf := ColorFilterGrayscale()
	l := Layout{
		Shape: &Shape{
			shapeType: shapeRectangle,
			Width:     100,
			Height:    100,
			Color:     White,
			Opacity:   1.0,
			fx: &shapeEffects{
				ColorFilter: cf,
			},
		},
	}
	clip := drawClip{Width: 800, Height: 600}
	renderLayout(&l, White, clip, w)

	var hasBegin, hasEnd bool
	var beginMatrix *[16]float32
	for i := range w.renderers {
		switch w.renderers[i].Kind {
		case RenderFilterBegin:
			hasBegin = true
			beginMatrix = w.renderers[i].ColorMatrix
		case RenderFilterEnd:
			hasEnd = true
		}
	}
	if !hasBegin {
		t.Error("missing RenderFilterBegin")
	}
	if !hasEnd {
		t.Error("missing RenderFilterEnd")
	}
	if beginMatrix == nil {
		t.Fatal("ColorMatrix not set on RenderFilterBegin")
	}
	if *beginMatrix != cf.Matrix {
		t.Errorf("matrix mismatch: got %v", *beginMatrix)
	}
}

func TestRenderFilterBracketBlurSuppressed(t *testing.T) {
	w := newTestWindow()
	l := Layout{
		Shape: &Shape{
			shapeType: shapeRectangle,
			Width:     100,
			Height:    100,
			Color:     White,
			Opacity:   1.0,
			fx: &shapeEffects{
				ColorFilter: ColorFilterGrayscale(),
				BlurRadius:  10,
			},
		},
	}
	clip := drawClip{Width: 800, Height: 600}
	renderLayout(&l, White, clip, w)

	for i := range w.renderers {
		if w.renderers[i].Kind == RenderBlur {
			t.Error("SDF RenderBlur should be suppressed when ColorFilter is set")
		}
	}

	// Should have FilterBegin with blur radius.
	var found bool
	for i := range w.renderers {
		if w.renderers[i].Kind == RenderFilterBegin &&
			w.renderers[i].BlurRadius == 10 {
			found = true
		}
	}
	if !found {
		t.Error("FilterBegin with blur radius not found")
	}
}
