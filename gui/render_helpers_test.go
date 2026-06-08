package gui

import (
	"math"
	"testing"
)

func TestRectsOverlapBasic(t *testing.T) {
	a := drawClip{X: 0, Y: 0, Width: 10, Height: 10}
	b := drawClip{X: 5, Y: 5, Width: 10, Height: 10}
	if !rectsOverlap(a, b) {
		t.Error("overlapping rects should return true")
	}
}

func TestRectsOverlapDisjoint(t *testing.T) {
	a := drawClip{X: 0, Y: 0, Width: 10, Height: 10}
	b := drawClip{X: 20, Y: 20, Width: 10, Height: 10}
	if rectsOverlap(a, b) {
		t.Error("disjoint rects should return false")
	}
}

func TestRectsOverlapAdjacent(t *testing.T) {
	a := drawClip{X: 0, Y: 0, Width: 10, Height: 10}
	b := drawClip{X: 10, Y: 0, Width: 10, Height: 10}
	if rectsOverlap(a, b) {
		t.Error("adjacent rects (shared edge) should return false (strict <)")
	}
}

func TestRectsOverlapContained(t *testing.T) {
	outer := drawClip{X: 0, Y: 0, Width: 100, Height: 100}
	inner := drawClip{X: 10, Y: 10, Width: 5, Height: 5}
	if !rectsOverlap(outer, inner) {
		t.Error("contained rect should overlap")
	}
	if !rectsOverlap(inner, outer) {
		t.Error("containing rect should overlap (commutative)")
	}
}

func TestRectsOverlapZeroSizePoint(t *testing.T) {
	// A zero-size "point" at (5,5) inside a 10x10 rect: the strict-<
	// comparisons still hold (5 < 10 && 0 < 5), so overlap is true.
	a := drawClip{X: 5, Y: 5, Width: 0, Height: 0}
	b := drawClip{X: 0, Y: 0, Width: 10, Height: 10}
	if !rectsOverlap(a, b) {
		t.Error("zero-size point inside rect overlaps with strict <")
	}
}

func TestRectsOverlapZeroSizeOutside(t *testing.T) {
	a := drawClip{X: 15, Y: 15, Width: 0, Height: 0}
	b := drawClip{X: 0, Y: 0, Width: 10, Height: 10}
	if rectsOverlap(a, b) {
		t.Error("zero-size point outside rect should not overlap")
	}
}

func TestRectsOverlapNegativeCoords(t *testing.T) {
	a := drawClip{X: -5, Y: -5, Width: 10, Height: 10}
	b := drawClip{X: 0, Y: 0, Width: 10, Height: 10}
	if !rectsOverlap(a, b) {
		t.Error("negative-coord rects should overlap when intersecting")
	}
}

func TestDimAlphaPreservesRGB(t *testing.T) {
	c := Color{R: 200, G: 100, B: 50, A: 80, set: true}
	d := dimAlpha(c)
	if d.R != 200 || d.G != 100 || d.B != 50 {
		t.Error("RGB should be unchanged")
	}
	if d.A != 40 {
		t.Errorf("alpha should be halved: got %d, want 40", d.A)
	}
	if !d.set {
		t.Error("set flag should be preserved")
	}
}

func TestDimAlphaZeroAlpha(t *testing.T) {
	c := Color{R: 255, G: 255, B: 255, A: 0}
	d := dimAlpha(c)
	if d.A != 0 {
		t.Errorf("zero alpha halved should be 0: got %d", d.A)
	}
}

func TestDimAlphaOdd(t *testing.T) {
	c := Color{A: 255}
	d := dimAlpha(c)
	if d.A != 127 {
		t.Errorf("255/2 should truncate to 127: got %d", d.A)
	}
}

func TestResolveClipRadiusNoClip(t *testing.T) {
	s := &Shape{Clip: false, Radius: 10}
	got := resolveClipRadius(5, s)
	if got != 5 {
		t.Errorf("non-clip shape should return parent radius: got %f", got)
	}
}

func TestResolveClipRadiusCircle(t *testing.T) {
	s := &Shape{
		Clip:      true,
		shapeType: shapeCircle,
		Width:     20,
		Height:    20,
	}
	got := resolveClipRadius(0, s)
	if got != 10 {
		t.Errorf("circle radius should be min(W,H)/2: got %f", got)
	}
}

func TestResolveClipRadiusWithInset(t *testing.T) {
	s := &Shape{
		Clip:       true,
		Radius:     10,
		SizeBorder: 2,
		Padding:    Padding{Left: 1, Right: 1, Top: 1, Bottom: 1},
	}
	// inset = max(1+2, 1+2, 1+2, 1+2) = 3
	// localRadius = max(0, 10-3) = 7
	// parentRadius <= 0, return localRadius
	got := resolveClipRadius(0, s)
	if got != 7 {
		t.Errorf("expected 7, got %f", got)
	}
}

func TestResolveClipRadiusMinOfParentAndLocal(t *testing.T) {
	s := &Shape{Clip: true, Radius: 20}
	got := resolveClipRadius(5, s)
	if got != 5 {
		t.Errorf("should return min(parent, local): got %f, want 5", got)
	}
}

func TestResolveClipRadiusZeroRadius(t *testing.T) {
	s := &Shape{Clip: true, Radius: 0}
	got := resolveClipRadius(5, s)
	if got != 5 {
		t.Errorf("zero shape radius should return parent: got %f", got)
	}
}

func TestQuantizedScissorClipScale1(t *testing.T) {
	clip := drawClip{X: 1.7, Y: 2.3, Width: 10.9, Height: 5.1}
	got := quantizedScissorClip(clip, 1)
	if got.X != 1 || got.Y != 2 || got.Width != 10 || got.Height != 5 {
		t.Errorf("scale=1 should truncate to int: got %+v", got)
	}
}

func TestQuantizedScissorClipScale2(t *testing.T) {
	clip := drawClip{X: 1.3, Y: 2.7, Width: 10, Height: 5}
	got := quantizedScissorClip(clip, 2)
	// 1.3*2=2.6→2, 2/2=1.0
	// 2.7*2=5.4→5, 5/2=2.5
	if got.X != 1.0 || got.Y != 2.5 {
		t.Errorf("scale=2 X,Y: got %f,%f want 1.0,2.5", got.X, got.Y)
	}
}

func TestQuantizedScissorClipZeroScale(t *testing.T) {
	clip := drawClip{X: 1.5, Y: 2.5, Width: 10, Height: 5}
	got := quantizedScissorClip(clip, 0)
	if got != clip {
		t.Error("zero scale should return clip unchanged")
	}
}

func TestRoundedImageClipParamsNoOverlap(t *testing.T) {
	clip := drawClip{X: 100, Y: 100, Width: 10, Height: 10}
	_, ok := roundedImageClipParams(0, 0, 10, 10, clip)
	if ok {
		t.Error("non-overlapping image and clip should return false")
	}
}

func TestRoundedImageClipParamsZeroSize(t *testing.T) {
	clip := drawClip{X: 0, Y: 0, Width: 10, Height: 10}
	_, ok := roundedImageClipParams(0, 0, 0, 10, clip)
	if ok {
		t.Error("zero-width image should return false")
	}
}

func TestRoundedImageClipParamsFullOverlap(t *testing.T) {
	clip := drawClip{X: 0, Y: 0, Width: 100, Height: 100}
	r, ok := roundedImageClipParams(10, 10, 20, 20, clip)
	if !ok {
		t.Fatal("fully contained image should return true")
	}
	if r.X != 10 || r.Y != 10 || r.W != 20 || r.H != 20 {
		t.Errorf("unexpected clip params: %+v", r)
	}
}

func TestRoundedImageClipParamsPartialClip(t *testing.T) {
	clip := drawClip{X: 5, Y: 5, Width: 10, Height: 10}
	r, ok := roundedImageClipParams(0, 0, 20, 20, clip)
	if !ok {
		t.Fatal("overlapping image should return true")
	}
	if r.W != 10 || r.H != 10 {
		t.Errorf("clipped size should be 10x10: got %fx%f", r.W, r.H)
	}
	// UV coordinates should map the visible portion.
	if r.U0 >= r.U1 || r.V0 >= r.V1 {
		t.Errorf("UV should be ordered: U0=%f U1=%f V0=%f V1=%f",
			r.U0, r.U1, r.V0, r.V1)
	}
}

func TestRoundedImageClipParamsNaN(t *testing.T) {
	// NaN propagates through arithmetic; the function does not
	// reject it because NaN comparisons return false. Callers
	// guard via rendererValidForDraw which checks f32AllFinite.
	clip := drawClip{X: 0, Y: 0, Width: 10, Height: 10}
	nan := float32(math.NaN())
	r, ok := roundedImageClipParams(nan, 0, 10, 10, clip)
	if ok && r.W <= 0 {
		t.Error("if ok, width should be positive")
	}
}
