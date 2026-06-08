package gui

import (
	"math"
	"testing"
)

func makeWindow() *Window {
	return &Window{}
}

func makeWindowWithScratch() *Window {
	return &Window{scratch: newScratchPools()}
}

func makeClip(x, y, w, h float32) drawClip {
	return drawClip{X: x, Y: y, Width: w, Height: h}
}

// --- rectsOverlap ---

func TestRectsOverlap(t *testing.T) {
	a := makeClip(0, 0, 10, 10)
	b := makeClip(5, 5, 10, 10)
	c := makeClip(10, 0, 5, 5) // touches edge at x=10

	if !rectsOverlap(a, b) {
		t.Error("a and b should overlap")
	}
	if rectsOverlap(a, c) {
		t.Error("touching edge should not overlap (strict <)")
	}
}

// --- roundedImageClipParams ---

func TestRoundedImageClipParamsIntersectsAndMapsUV(t *testing.T) {
	clip := makeClip(4, 2, 116, 116)
	params, ok := roundedImageClipParams(2, 2, 120, 120, clip)
	if !ok {
		t.Fatal("expected rounded image clip params")
	}
	if !f32AreClose(params.X, 4) {
		t.Errorf("X: got %f, want 4", params.X)
	}
	if !f32AreClose(params.Y, 2) {
		t.Errorf("Y: got %f, want 2", params.Y)
	}
	if !f32AreClose(params.W, 116) {
		t.Errorf("W: got %f, want 116", params.W)
	}
	if !f32AreClose(params.H, 116) {
		t.Errorf("H: got %f, want 116", params.H)
	}
	if !f32AreClose(params.U0, -1+float32(2.0*2.0/120.0)) {
		t.Errorf("U0: got %f", params.U0)
	}
	if !f32AreClose(params.V0, -1) {
		t.Errorf("V0: got %f", params.V0)
	}
	if !f32AreClose(params.U1, -1+float32(2.0*118.0/120.0)) {
		t.Errorf("U1: got %f", params.U1)
	}
	if !f32AreClose(params.V1, -1+float32(2.0*116.0/120.0)) {
		t.Errorf("V1: got %f", params.V1)
	}
}

func TestRoundedImageClipParamsReturnsNoneWhenNoOverlap(t *testing.T) {
	_, ok := roundedImageClipParams(10, 10, 20, 20, makeClip(0, 0, 5, 5))
	if ok {
		t.Error("expected no overlap")
	}
}

func TestRoundedImageClipParamsShrinksWhenTopLeftAnchoredInnerClip(t *testing.T) {
	clip := makeClip(2, 2, 116, 116)
	params, ok := roundedImageClipParams(2, 2, 120, 120, clip)
	if !ok {
		t.Fatal("expected rounded image clip params")
	}
	if !f32AreClose(params.X, 2) {
		t.Errorf("X: got %f", params.X)
	}
	if !f32AreClose(params.Y, 2) {
		t.Errorf("Y: got %f", params.Y)
	}
	if !f32AreClose(params.W, 116) {
		t.Errorf("W: got %f", params.W)
	}
	if !f32AreClose(params.H, 116) {
		t.Errorf("H: got %f", params.H)
	}
	if !f32AreClose(params.U0, -1) {
		t.Errorf("U0: got %f", params.U0)
	}
	if !f32AreClose(params.V0, -1) {
		t.Errorf("V0: got %f", params.V0)
	}
	if !f32AreClose(params.U1, 1) {
		t.Errorf("U1: got %f", params.U1)
	}
	if !f32AreClose(params.V1, 1) {
		t.Errorf("V1: got %f", params.V1)
	}
}

// --- quantizedScissorClip ---

func TestQuantizedScissorClipMatchesSokolIntTruncation(t *testing.T) {
	clip := makeClip(10.9, 20.9, 30.9, 40.9)
	q := quantizedScissorClip(clip, 1.0)
	if !f32AreClose(q.X, 10) {
		t.Errorf("X: got %f", q.X)
	}
	if !f32AreClose(q.Y, 20) {
		t.Errorf("Y: got %f", q.Y)
	}
	if !f32AreClose(q.Width, 30) {
		t.Errorf("W: got %f", q.Width)
	}
	if !f32AreClose(q.Height, 40) {
		t.Errorf("H: got %f", q.Height)
	}
}

func TestQuantizedScissorClipRespectsScale(t *testing.T) {
	clip := makeClip(1.26, 2.26, 3.26, 4.26)
	q := quantizedScissorClip(clip, 2.0)
	if !f32AreClose(q.X, 1.0) {
		t.Errorf("X: got %f", q.X)
	}
	if !f32AreClose(q.Y, 2.0) {
		t.Errorf("Y: got %f", q.Y)
	}
	if !f32AreClose(q.Width, 3.0) {
		t.Errorf("W: got %f", q.Width)
	}
	if !f32AreClose(q.Height, 4.0) {
		t.Errorf("H: got %f", q.Height)
	}
}

// --- dimAlpha ---

func TestDimAlpha(t *testing.T) {
	c := RGBA(10, 20, 30, 201)
	d := dimAlpha(c)
	if d.R != c.R || d.G != c.G || d.B != c.B {
		t.Error("RGB should be unchanged")
	}
	if d.A != 201/2 {
		t.Errorf("A: got %d, want %d", d.A, 201/2)
	}
}

// --- renderRectangle ---

func TestRenderRectangleInsideClip(t *testing.T) {
	w := makeWindow()
	s := &Shape{
		shapeType:  shapeRectangle,
		X:          10,
		Y:          20,
		Width:      30,
		Height:     40,
		Color:      RGB(100, 150, 200),
		Radius:     5,
		SizeBorder: 0,
	}
	clip := makeClip(0, 0, 200, 200)
	renderRectangle(s, clip, w)

	if len(w.renderers) != 1 {
		t.Fatalf("renderers: got %d, want 1", len(w.renderers))
	}
	r := w.renderers[0]
	if r.Kind != RenderRect {
		t.Fatalf("kind: got %d, want RenderRect", r.Kind)
	}
	if r.X != s.X || r.Y != s.Y {
		t.Errorf("pos: got (%f,%f), want (%f,%f)", r.X, r.Y, s.X, s.Y)
	}
	if r.W != s.Width || r.H != s.Height {
		t.Errorf("size: got (%f,%f)", r.W, r.H)
	}
	if r.Fill != true {
		t.Error("expected fill")
	}
	if r.Radius != s.Radius {
		t.Errorf("radius: got %f, want %f", r.Radius, s.Radius)
	}
	if r.Color != s.Color {
		t.Errorf("color mismatch")
	}
}

func TestRenderRectangleOutsideClipSkipsDraw(t *testing.T) {
	w := makeWindow()
	s := &Shape{
		shapeType: shapeRectangle,
		X:         100,
		Y:         100,
		Width:     20,
		Height:    20,
		Color:     RGB(10, 10, 10),
	}
	clip := makeClip(0, 0, 50, 50)
	renderRectangle(s, clip, w)

	if len(w.renderers) != 0 {
		t.Errorf("renderers: got %d, want 0", len(w.renderers))
	}
}

func TestRenderContainerShadowUsesBasePositionAndSeparateOffset(t *testing.T) {
	w := makeWindow()
	s := &Shape{
		shapeType: shapeRectangle,
		X:         100,
		Y:         200,
		Width:     80,
		Height:    60,
		Color:     ColorTransparent,
		Radius:    12,
		fx: &shapeEffects{
			Shadow: &BoxShadow{
				Color:      RGBA(0, 0, 0, 80),
				OffsetX:    3,
				OffsetY:    5,
				BlurRadius: 20,
			},
		},
	}
	clip := makeClip(0, 0, 500, 500)
	renderContainer(s, ColorTransparent, clip, w)

	if len(w.renderers) != 1 {
		t.Fatalf("renderers: got %d, want 1", len(w.renderers))
	}
	r := w.renderers[0]
	if r.Kind != RenderShadow {
		t.Fatalf("kind: got %v, want RenderShadow", r.Kind)
	}
	if r.X != s.X || r.Y != s.Y {
		t.Errorf("pos: got (%f,%f), want base (%f,%f)", r.X, r.Y, s.X, s.Y)
	}
	if r.OffsetX != s.fx.Shadow.OffsetX || r.OffsetY != s.fx.Shadow.OffsetY {
		t.Errorf("offset: got (%f,%f), want (%f,%f)",
			r.OffsetX, r.OffsetY, s.fx.Shadow.OffsetX, s.fx.Shadow.OffsetY)
	}
}

func TestRenderContainerShadowZeroAlphaSkipped(t *testing.T) {
	w := makeWindow()
	s := &Shape{
		shapeType: shapeRectangle,
		X:         10, Y: 10,
		Width: 50, Height: 50,
		Color: RGB(200, 200, 200),
		fx: &shapeEffects{
			Shadow: &BoxShadow{
				Color:      RGBA(0, 0, 0, 0),
				OffsetY:    5,
				BlurRadius: 10,
			},
		},
	}
	renderContainer(s, ColorTransparent, makeClip(0, 0, 500, 500), w)
	for _, r := range w.renderers {
		if r.Kind == RenderShadow {
			t.Error("zero-alpha shadow should be skipped")
		}
	}
}

func TestRenderContainerShadowNegativeBlurSkipped(t *testing.T) {
	w := makeWindow()
	s := &Shape{
		shapeType: shapeRectangle,
		X:         10, Y: 10,
		Width: 50, Height: 50,
		Color: RGB(200, 200, 200),
		fx: &shapeEffects{
			Shadow: &BoxShadow{
				Color:      RGBA(0, 0, 0, 80),
				BlurRadius: -5,
			},
		},
	}
	renderContainer(s, ColorTransparent, makeClip(0, 0, 500, 500), w)
	for _, r := range w.renderers {
		if r.Kind == RenderShadow {
			t.Error("negative-blur shadow should be skipped")
		}
	}
}

func TestRenderContainerShadowHardShadow(t *testing.T) {
	w := makeWindow()
	s := &Shape{
		shapeType: shapeRectangle,
		X:         10, Y: 10,
		Width: 50, Height: 50,
		Color: ColorTransparent,
		fx: &shapeEffects{
			Shadow: &BoxShadow{
				Color:   RGBA(0, 0, 0, 80),
				OffsetY: 5,
			},
		},
	}
	renderContainer(s, ColorTransparent, makeClip(0, 0, 500, 500), w)
	found := false
	for _, r := range w.renderers {
		if r.Kind == RenderShadow {
			found = true
			if r.BlurRadius != 0 {
				t.Errorf("blur: got %f, want 0", r.BlurRadius)
			}
			if r.OffsetY != 5 {
				t.Errorf("offsetY: got %f, want 5", r.OffsetY)
			}
		}
	}
	if !found {
		t.Error("hard shadow (blur=0, offset!=0) should emit RenderShadow")
	}
}

func TestRenderContainerNoShadowWhenAllZero(t *testing.T) {
	w := makeWindow()
	s := &Shape{
		shapeType: shapeRectangle,
		X:         10, Y: 10,
		Width: 50, Height: 50,
		Color: RGB(200, 200, 200),
		fx: &shapeEffects{
			Shadow: &BoxShadow{
				Color: RGBA(0, 0, 0, 80),
			},
		},
	}
	renderContainer(s, ColorTransparent, makeClip(0, 0, 500, 500), w)
	for _, r := range w.renderers {
		if r.Kind == RenderShadow {
			t.Error("all-zero shadow should be skipped")
		}
	}
}

// --- renderCircle ---

func TestRenderCircleInsideClip(t *testing.T) {
	w := makeWindow()
	s := &Shape{
		shapeType:  shapeCircle,
		X:          0,
		Y:          0,
		Width:      40,
		Height:     20,
		Color:      RGB(1, 2, 3),
		SizeBorder: 0,
	}
	clip := makeClip(-10, -10, 100, 100)
	renderCircle(s, clip, w)

	if len(w.renderers) != 1 {
		t.Fatalf("renderers: got %d, want 1", len(w.renderers))
	}
	r := w.renderers[0]
	if r.Kind != RenderCircle {
		t.Fatalf("kind: got %d, want RenderCircle", r.Kind)
	}
	if !f32AreClose(r.X, s.X+s.Width/2) {
		t.Errorf("center X: got %f", r.X)
	}
	if !f32AreClose(r.Y, s.Y+s.Height/2) {
		t.Errorf("center Y: got %f", r.Y)
	}
	if !f32AreClose(r.Radius, f32Min(s.Width, s.Height)/2) {
		t.Errorf("radius: got %f", r.Radius)
	}
	if r.Color != s.Color {
		t.Error("color mismatch")
	}
}

// --- renderLayout clip push/pop ---

func TestRenderLayoutClipPushPop(t *testing.T) {
	w := makeWindow()
	root := &Layout{
		Shape: &Shape{
			Color: ColorTransparent,
			Clip:  true,
			Padding: Padding{
				Left: 2, Right: 3, Top: 4, Bottom: 5,
			},
			SizeBorder: 0,
			shapeClip:  makeClip(10, 20, 100, 50),
		},
	}

	initialClip := makeClip(0, 0, 400, 400)
	bg := RGB(0, 0, 0)
	renderLayout(root, bg, initialClip, w)

	if len(w.renderers) != 2 {
		t.Fatalf("renderers: got %d, want 2", len(w.renderers))
	}

	push := w.renderers[0]
	pop := w.renderers[1]

	if push.Kind != RenderClip {
		t.Fatal("first renderer should be RenderClip (push)")
	}
	if !f32AreClose(push.X, 10+2) {
		t.Errorf("push X: got %f, want 12", push.X)
	}
	if !f32AreClose(push.Y, 20+4) {
		t.Errorf("push Y: got %f, want 24", push.Y)
	}
	if !f32AreClose(push.W, 100-(2+3)) {
		t.Errorf("push W: got %f, want 95", push.W)
	}
	if !f32AreClose(push.H, 50-(4+5)) {
		t.Errorf("push H: got %f, want 41", push.H)
	}

	if pop.Kind != RenderClip {
		t.Fatal("second renderer should be RenderClip (pop)")
	}
	if !f32AreClose(pop.X, initialClip.X) {
		t.Errorf("pop X: got %f", pop.X)
	}
	if !f32AreClose(pop.Y, initialClip.Y) {
		t.Errorf("pop Y: got %f", pop.Y)
	}
	if !f32AreClose(pop.W, initialClip.Width) {
		t.Errorf("pop W: got %f", pop.W)
	}
	if !f32AreClose(pop.H, initialClip.Height) {
		t.Errorf("pop H: got %f", pop.H)
	}
}

// --- resolveClipRadius ---

func TestResolveClipRadiusKeepsParentWhenChildNotRounded(t *testing.T) {
	shape := &Shape{Clip: true, Width: 60, Height: 40, Radius: 0}
	if !f32AreClose(resolveClipRadius(12, shape), 12) {
		t.Error("should keep parent radius")
	}
}

func TestResolveClipRadiusUsesMinForNestedRounded(t *testing.T) {
	shape := &Shape{Clip: true, Width: 60, Height: 40, Radius: 8}
	if !f32AreClose(resolveClipRadius(12, shape), 8) {
		t.Error("should use min radius")
	}
}

func TestResolveClipRadiusSubtractsBorderAndPadding(t *testing.T) {
	shape := &Shape{
		Clip: true, Width: 80, Height: 60, Radius: 12,
		SizeBorder: 1,
		Padding:    Padding{Left: 2, Right: 2, Top: 2, Bottom: 2},
	}
	// inset = 3, 12 - 3 = 9
	if !f32AreClose(resolveClipRadius(0, shape), 9) {
		t.Errorf("got %f, want 9", resolveClipRadius(0, shape))
	}
}

func TestResolveClipRadiusUsesMaxInsetForAsymmetricPadding(t *testing.T) {
	shape := &Shape{
		Clip: true, Width: 80, Height: 60, Radius: 20,
		SizeBorder: 2,
		Padding:    Padding{Left: 1, Right: 7, Top: 3, Bottom: 0},
	}
	// max inset = 9, 20 - 9 = 11
	if !f32AreClose(resolveClipRadius(0, shape), 11) {
		t.Errorf("got %f, want 11", resolveClipRadius(0, shape))
	}
}

func TestResolveClipRadiusReturnsParentWhenInsetConsumesRadius(t *testing.T) {
	shape := &Shape{
		Clip: true, Width: 60, Height: 40, Radius: 6,
		SizeBorder: 2,
		Padding:    Padding{Left: 4, Right: 4, Top: 4, Bottom: 4},
	}
	if !f32AreClose(resolveClipRadius(10, shape), 10) {
		t.Error("should return parent")
	}
}

func TestResolveClipRadiusIgnoresNonFiniteChildRadius(t *testing.T) {
	shape := &Shape{
		Clip: true, Width: 60, Height: 40,
		Radius: float32(math.Inf(1)),
	}
	if !f32AreClose(resolveClipRadius(12, shape), 12) {
		t.Error("should ignore non-finite")
	}
}

// --- renderShape opacity ---

func TestRenderShapeOpacityNonTextIsSafe(t *testing.T) {
	w := makeWindow()
	s := &Shape{
		shapeType: shapeRectangle,
		X:         0, Y: 0,
		Width: 20, Height: 10,
		Color:   RGB(100, 120, 140),
		Opacity: 0.5,
	}
	clip := makeClip(0, 0, 200, 200)
	renderShape(s, ColorTransparent, clip, w)

	if len(w.renderers) != 1 {
		t.Errorf("renderers: got %d, want 1", len(w.renderers))
	}
}

func TestRenderShapeTextWithoutTextConfigDegradesSafe(t *testing.T) {
	w := makeWindow()
	s := &Shape{
		shapeType: shapeText,
		X:         0, Y: 0,
		Width: 50, Height: 20,
		Color:   Black,
		Opacity: 0.5,
	}
	clip := makeClip(0, 0, 200, 200)
	renderShape(s, ColorTransparent, clip, w)

	if len(w.renderers) != 0 {
		t.Errorf("renderers: got %d, want 0", len(w.renderers))
	}
}

// --- renderer validation ---

func TestRendererGuardValidDrawRect(t *testing.T) {
	r := RenderCmd{
		Kind: RenderRect,
		X:    1, Y: 2,
		W: 20, H: 10,
		Color: White,
		Fill:  true,
	}
	if !rendererValidForDraw(r) {
		t.Error("valid DrawRect should pass")
	}
}

func TestRendererGuardDrawGradientAllowsZeroSizeNoop(t *testing.T) {
	r := RenderCmd{
		Kind: RenderGradient,
		X:    21, Y: 66,
		W: 0, H: 0,
		Radius:   5.5,
		Gradient: &GradientDef{},
	}
	if !rendererValidForDraw(r) {
		t.Error("zero-size gradient should be valid noop")
	}
}

func TestRendererGuardDrawStrokeRectAllowsZeroSizeNoop(t *testing.T) {
	r := RenderCmd{
		Kind: RenderStrokeRect,
		X:    106, Y: 29,
		W: 241.5, H: 0,
		Radius:    5.5,
		Color:     White,
		Thickness: 1.5,
	}
	if !rendererValidForDraw(r) {
		t.Error("zero-height stroke rect should be valid noop")
	}
}

func TestRendererGuardDrawRectAllowsZeroSizeNoop(t *testing.T) {
	r := RenderCmd{
		Kind: RenderRect,
		X:    244.75, Y: 312.1875,
		W: 0, H: 29.09375,
		Radius: 5.5,
		Color:  White,
		Fill:   true,
	}
	if !rendererValidForDraw(r) {
		t.Error("zero-width rect should be valid noop")
	}
}

func TestRendererGuardInvalidDrawSvgOddTriangleCount(t *testing.T) {
	r := RenderCmd{
		Kind:      RenderSvg,
		Triangles: []float32{0, 0, 10, 0, 0, 10, 5},
		Color:     RGB(255, 0, 0),
		Scale:     1,
	}
	if rendererValidForDraw(r) {
		t.Error("odd triangle count should be invalid")
	}
}

func TestRendererGuardInvalidDrawSvgVertexColorsCountMismatch(t *testing.T) {
	r := RenderCmd{
		Kind:      RenderSvg,
		Triangles: []float32{0, 0, 10, 0, 0, 10},
		Color:     White,
		VertexColors: []Color{
			RGB(255, 0, 0), RGB(0, 255, 0),
			RGB(0, 0, 255), RGB(255, 255, 0),
		},
		Scale: 1,
	}
	if rendererValidForDraw(r) {
		t.Error("vertex color count mismatch should be invalid")
	}
}

func TestRendererGuardInvalidDrawSvgVertexAlphaScale(t *testing.T) {
	r := RenderCmd{
		Kind:             RenderSvg,
		Triangles:        []float32{0, 0, 10, 0, 0, 10},
		Color:            White,
		Scale:            1,
		HasVertexAlpha:   true,
		VertexAlphaScale: 1.5,
	}
	if rendererValidForDraw(r) {
		t.Error("vertex alpha scale > 1 should be invalid")
	}
}

func TestRendererGuardInvalidDrawClipNegativeSize(t *testing.T) {
	r := RenderCmd{
		Kind: RenderClip,
		X:    10, Y: 20,
		W: -1, H: 5,
	}
	if rendererValidForDraw(r) {
		t.Error("negative clip size should be invalid")
	}
}

func TestRendererGuardDrawTextRequiresNonEmptyText(t *testing.T) {
	valid := RenderCmd{
		Kind: RenderText,
		X:    1, Y: 2,
		Text: "ok",
	}
	if !rendererValidForDraw(valid) {
		t.Error("non-empty text should be valid")
	}
	invalid := RenderCmd{
		Kind: RenderText,
		X:    1, Y: 2,
		Text: "",
	}
	if rendererValidForDraw(invalid) {
		t.Error("empty text should be invalid")
	}
}

func TestRendererGuardValidDrawClipZeroSize(t *testing.T) {
	r := RenderCmd{
		Kind: RenderClip,
		X:    10, Y: 20,
		W: 0, H: 0,
	}
	if !rendererValidForDraw(r) {
		t.Error("zero-size clip should be valid")
	}
}

func TestInvalidClipIsSkippedAndNextDrawKept(t *testing.T) {
	invalidClip := RenderCmd{
		Kind: RenderClip,
		X:    10, Y: 20,
		W: -5, H: 10,
	}
	validRect := RenderCmd{
		Kind: RenderRect,
		X:    1, Y: 2,
		W: 20, H: 10,
		Color: White,
		Fill:  true,
	}

	w := makeWindow()
	if emitRendererIfValid(invalidClip, w) {
		t.Error("invalid clip should not emit")
	}
	if !emitRendererIfValid(validRect, w) {
		t.Error("valid rect should emit")
	}
	if len(w.renderers) != 1 {
		t.Fatalf("renderers: got %d, want 1", len(w.renderers))
	}
	if w.renderers[0].Kind != RenderRect {
		t.Error("expected RenderRect after invalid clip skip")
	}
}

// --- ClipContents stencil emission ---

func TestClipContentsEmitsStencilBracket(t *testing.T) {
	w := makeWindow()
	child := Layout{
		Shape: &Shape{
			shapeType: shapeRectangle,
			X:         5, Y: 5,
			Width: 10, Height: 10,
			Color: RGB(1, 2, 3),
		},
	}
	root := &Layout{
		Shape: &Shape{
			ClipContents: true,
			Radius:       12,
			Width:        100,
			Height:       80,
			shapeClip:    makeClip(0, 0, 100, 80),
		},
		Children: []Layout{child},
	}

	clip := makeClip(0, 0, 400, 400)
	renderLayout(root, RGB(0, 0, 0), clip, w)

	// Expect: StencilBegin, Clip(push), [child], Clip(pop),
	//         StencilEnd
	foundBegin := false
	foundEnd := false
	for _, r := range w.renderers {
		if r.Kind == RenderStencilBegin {
			foundBegin = true
			if r.StencilDepth != 1 {
				t.Errorf("begin depth: got %d, want 1",
					r.StencilDepth)
			}
			if r.Radius != 12 {
				t.Errorf("begin radius: got %f, want 12",
					r.Radius)
			}
		}
		if r.Kind == RenderStencilEnd {
			foundEnd = true
			if r.StencilDepth != 1 {
				t.Errorf("end depth: got %d, want 1",
					r.StencilDepth)
			}
		}
	}
	if !foundBegin {
		t.Error("expected RenderStencilBegin")
	}
	if !foundEnd {
		t.Error("expected RenderStencilEnd")
	}
}

func TestClipContentsNestedIncrementsDepth(t *testing.T) {
	w := makeWindow()
	inner := Layout{
		Shape: &Shape{
			ClipContents: true,
			Radius:       6,
			Width:        40,
			Height:       30,
			shapeClip:    makeClip(10, 10, 40, 30),
		},
	}
	outer := &Layout{
		Shape: &Shape{
			ClipContents: true,
			Radius:       12,
			Width:        100,
			Height:       80,
			shapeClip:    makeClip(0, 0, 100, 80),
		},
		Children: []Layout{inner},
	}

	clip := makeClip(0, 0, 400, 400)
	renderLayout(outer, RGB(0, 0, 0), clip, w)

	depths := []uint8{}
	for _, r := range w.renderers {
		if r.Kind == RenderStencilBegin {
			depths = append(depths, r.StencilDepth)
		}
	}
	if len(depths) != 2 {
		t.Fatalf("expected 2 StencilBegin, got %d", len(depths))
	}
	if depths[0] != 1 {
		t.Errorf("outer depth: got %d, want 1", depths[0])
	}
	if depths[1] != 2 {
		t.Errorf("inner depth: got %d, want 2", depths[1])
	}
}

func TestClipContentsCoexistsWithClip(t *testing.T) {
	w := makeWindow()
	root := &Layout{
		Shape: &Shape{
			Clip:         true,
			ClipContents: true,
			Radius:       8,
			Width:        60,
			Height:       40,
			shapeClip:    makeClip(5, 5, 60, 40),
			Padding:      Padding{Left: 2, Right: 2, Top: 2, Bottom: 2},
		},
	}

	clip := makeClip(0, 0, 400, 400)
	renderLayout(root, RGB(0, 0, 0), clip, w)

	hasClip := false
	hasStencilBegin := false
	hasStencilEnd := false
	for _, r := range w.renderers {
		switch r.Kind {
		case RenderClip:
			hasClip = true
		case RenderStencilBegin:
			hasStencilBegin = true
		case RenderStencilEnd:
			hasStencilEnd = true
		}
	}
	if !hasClip {
		t.Error("expected RenderClip from Clip=true")
	}
	if !hasStencilBegin {
		t.Error("expected RenderStencilBegin")
	}
	if !hasStencilEnd {
		t.Error("expected RenderStencilEnd")
	}
}

// --- findFilterBracketRange ---

func TestFindFilterBracketRangeMatchedBeginEnd(t *testing.T) {
	renderers := []RenderCmd{
		{Kind: RenderNone},
		{Kind: RenderSvg, Triangles: []float32{0, 0, 10, 0, 0, 10}, Color: White, Scale: 1},
		{Kind: RenderFilterEnd},
		{Kind: RenderNone},
	}
	bracket := findFilterBracketRange(renderers, 0)
	if !bracket.FoundEnd {
		t.Error("expected found end")
	}
	if bracket.StartIdx != 0 {
		t.Errorf("start: got %d", bracket.StartIdx)
	}
	if bracket.EndIdx != 2 {
		t.Errorf("end: got %d", bracket.EndIdx)
	}
	if bracket.NextIdx != 3 {
		t.Errorf("next: got %d", bracket.NextIdx)
	}
}

func TestFindFilterBracketRangeUnmatchedBeginEnd(t *testing.T) {
	renderers := []RenderCmd{
		{Kind: RenderNone},
		{Kind: RenderNone},
	}
	bracket := findFilterBracketRange(renderers, 0)
	if bracket.FoundEnd {
		t.Error("expected not found")
	}
	if bracket.StartIdx != 0 {
		t.Errorf("start: got %d", bracket.StartIdx)
	}
	if bracket.EndIdx != 2 {
		t.Errorf("end: got %d", bracket.EndIdx)
	}
	if bracket.NextIdx != 2 {
		t.Errorf("next: got %d", bracket.NextIdx)
	}
}
