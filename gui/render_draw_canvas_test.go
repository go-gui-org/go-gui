package gui

import (
	"math"
	"testing"
)

func TestRenderDrawCanvasOutsideClipSkips(t *testing.T) {
	w := makeWindowWithScratch()
	shape := &Shape{
		ShapeType: ShapeDrawCanvas,
		X:         500, Y: 500,
		Width: 50, Height: 50,
		Color: RGB(100, 100, 100),
	}
	clip := makeClip(0, 0, 100, 100)

	renderDrawCanvas(shape, clip, w)

	if len(w.renderers) != 0 {
		t.Errorf("got %d renderers, want 0 for out-of-clip canvas",
			len(w.renderers))
	}
}

func TestRenderDrawCanvasCallsOnDraw(t *testing.T) {
	w := makeWindowWithScratch()
	called := false
	shape := &Shape{
		ShapeType: ShapeDrawCanvas,
		Width:     100, Height: 100,
		Color: RGB(100, 100, 100),
		Events: &EventHandlers{
			OnDraw: func(dc *DrawContext) {
				called = true
				dc.batches = append(dc.batches, DrawCanvasTriBatch{
					Triangles: []float32{0, 0, 1, 0, 0, 1},
					Color:     RGB(255, 0, 0),
				})
			},
		},
	}
	clip := makeClip(0, 0, 200, 200)

	renderDrawCanvas(shape, clip, w)

	if !called {
		t.Error("OnDraw not called")
	}
	found := false
	for _, r := range w.renderers {
		if r.Kind == RenderSvg {
			found = true
		}
	}
	if !found {
		t.Error("expected RenderSvg for canvas batch")
	}
}

func TestRenderDrawCanvasEmptyBatchesNoOutput(t *testing.T) {
	w := makeWindowWithScratch()
	shape := &Shape{
		ShapeType: ShapeDrawCanvas,
		Width:     100, Height: 100,
		Color: ColorTransparent,
		Events: &EventHandlers{
			OnDraw: func(_ *DrawContext) {
				// produce no batches
			},
		},
	}
	clip := makeClip(0, 0, 200, 200)

	renderDrawCanvas(shape, clip, w)

	for _, r := range w.renderers {
		if r.Kind == RenderSvg {
			t.Error("should not emit RenderSvg for empty batches")
		}
	}
}

func TestRenderDrawCanvasClipBrackets(t *testing.T) {
	w := makeWindowWithScratch()
	shape := &Shape{
		ShapeType: ShapeDrawCanvas,
		Width:     100, Height: 100,
		Color: RGB(100, 100, 100),
		Clip:  true,
		Events: &EventHandlers{
			OnDraw: func(dc *DrawContext) {
				dc.batches = append(dc.batches, DrawCanvasTriBatch{
					Triangles: []float32{0, 0, 1, 0, 0, 1},
					Color:     RGB(255, 0, 0),
				})
			},
		},
	}
	clip := makeClip(0, 0, 200, 200)

	renderDrawCanvas(shape, clip, w)

	clipCount := 0
	for _, r := range w.renderers {
		if r.Kind == RenderClip {
			clipCount++
		}
	}
	if clipCount < 2 {
		t.Errorf("got %d RenderClip, want >= 2 (push + pop)", clipCount)
	}
}

func TestRenderDrawCanvasCachedSkipsOnDraw(t *testing.T) {
	w := makeWindowWithScratch()
	callCount := 0
	shape := &Shape{
		ShapeType: ShapeDrawCanvas,
		ID:        "test-canvas",
		Width:     100, Height: 100,
		Version: 1,
		Color:   RGB(100, 100, 100),
		Events: &EventHandlers{
			OnDraw: func(dc *DrawContext) {
				callCount++
				dc.batches = append(dc.batches, DrawCanvasTriBatch{
					Triangles: []float32{0, 0, 1, 0, 0, 1},
					Color:     RGB(255, 0, 0),
				})
			},
		},
	}
	clip := makeClip(0, 0, 200, 200)

	// First render — should call OnDraw.
	renderDrawCanvas(shape, clip, w)
	if callCount != 1 {
		t.Fatalf("first render: callCount = %d, want 1", callCount)
	}

	// Second render with same version/dimensions — should skip.
	w.renderers = w.renderers[:0]
	renderDrawCanvas(shape, clip, w)
	if callCount != 1 {
		t.Errorf("cached render: callCount = %d, want 1", callCount)
	}
}

// TestRenderDrawCanvas_ImagesEmitBeforeBatchesAndText: images are
// the back layer so triangles and text drawn in the same OnDraw paint
// over them. Tile-map consumers depend on this to place marker discs
// and HUD chips over OSM tile images rendered by the same DrawCanvas.
func TestRenderDrawCanvas_ImagesEmitBeforeBatchesAndText(t *testing.T) {
	w := makeWindowWithScratch()
	shape := &Shape{
		ShapeType: ShapeDrawCanvas,
		Width:     100, Height: 100,
		Color: ColorTransparent,
		Events: &EventHandlers{
			OnDraw: func(dc *DrawContext) {
				dc.Image(0, 0, 10, 10, "bg.png", Opt[float32]{}, ColorTransparent)
				dc.FilledRect(0, 0, 5, 5, Blue)
				dc.Text(2, 2, "hi", TextStyle{Size: 10, Color: Blue})
			},
		},
	}
	renderDrawCanvas(shape, makeClip(0, 0, 200, 200), w)

	var imageIdx, svgIdx, textIdx = -1, -1, -1
	for i := range w.renderers {
		switch w.renderers[i].Kind {
		case RenderImage:
			if imageIdx < 0 {
				imageIdx = i
			}
		case RenderSvg:
			if svgIdx < 0 {
				svgIdx = i
			}
		case RenderText:
			if textIdx < 0 {
				textIdx = i
			}
		}
	}
	if imageIdx < 0 || svgIdx < 0 || textIdx < 0 {
		t.Fatalf("missing emission: image=%d svg=%d text=%d",
			imageIdx, svgIdx, textIdx)
	}
	if imageIdx >= svgIdx || svgIdx >= textIdx {
		t.Errorf("order image=%d svg=%d text=%d; want image<svg<text",
			imageIdx, svgIdx, textIdx)
	}
}

func TestRenderDrawCanvasEmitsImage(t *testing.T) {
	w := makeWindowWithScratch()
	shape := &Shape{
		ShapeType: ShapeDrawCanvas,
		X:         10, Y: 20,
		Width: 100, Height: 100,
		Color: ColorTransparent,
		Padding: Padding{
			Top: 5, Left: 5, Right: 5, Bottom: 5,
		},
		Events: &EventHandlers{
			OnDraw: func(dc *DrawContext) {
				dc.Image(3, 4, 16, 16, "tile.png",
					SomeF(0.5), Blue)
			},
		},
	}
	clip := makeClip(0, 0, 200, 200)

	renderDrawCanvas(shape, clip, w)

	var img *RenderCmd
	for i := range w.renderers {
		if w.renderers[i].Kind == RenderImage {
			img = &w.renderers[i]
			break
		}
	}
	if img == nil {
		t.Fatal("no RenderImage emitted")
	}
	// Origin = shape pos + padding + entry pos = 10+5+3, 20+5+4.
	if img.X != 18 || img.Y != 29 {
		t.Errorf("pos = (%v,%v), want (18,29)", img.X, img.Y)
	}
	if img.W != 16 || img.H != 16 {
		t.Errorf("size = (%v,%v), want (16,16)", img.W, img.H)
	}
	if img.Resource != "tile.png" {
		t.Errorf("resource = %q, want %q", img.Resource, "tile.png")
	}
	// Blue bg with 0.5 opacity -> alpha halved.
	if img.Color.A == 255 || img.Color.A == 0 {
		t.Errorf("color alpha = %d, want opacity-folded", img.Color.A)
	}
}

func TestRenderDrawCanvasImageOpacityClamped(t *testing.T) {
	nan := float32(math.NaN())
	cases := []struct {
		name     string
		opacity  Opt[float32]
		wantA    uint8 // expected Color.A on emitted cmd
		wantNote string
	}{
		{"above-1 clamps to 1", SomeF(1.5), 255, "full alpha"},
		{"below-0 clamps to 0", SomeF(-0.25), 0, "zero alpha"},
		{"NaN falls back to 1", SomeF(nan), 255, "full alpha"},
		{"unset defaults to 1", Opt[float32]{}, 255, "full alpha"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			w := makeWindowWithScratch()
			op := tc.opacity
			shape := &Shape{
				ShapeType: ShapeDrawCanvas,
				Width:     50, Height: 50,
				Color: ColorTransparent,
				Events: &EventHandlers{
					OnDraw: func(dc *DrawContext) {
						dc.Image(0, 0, 10, 10, "x.png", op, Blue)
					},
				},
			}
			renderDrawCanvas(shape, makeClip(0, 0, 200, 200), w)

			var img *RenderCmd
			for i := range w.renderers {
				if w.renderers[i].Kind == RenderImage {
					img = &w.renderers[i]
					break
				}
			}
			if img == nil {
				t.Fatal("no RenderImage emitted")
			}
			if img.Color.A != tc.wantA {
				t.Errorf("%s: Color.A = %d, want %d (%s)",
					tc.name, img.Color.A, tc.wantA, tc.wantNote)
			}
		})
	}
}

func TestRenderDrawCanvasImageOnlyNotSkipped(t *testing.T) {
	w := makeWindowWithScratch()
	shape := &Shape{
		ShapeType: ShapeDrawCanvas,
		Width:     50, Height: 50,
		Color: ColorTransparent,
		Events: &EventHandlers{
			OnDraw: func(dc *DrawContext) {
				dc.Image(0, 0, 10, 10, "x.png",
					Opt[float32]{}, Color{})
			},
		},
	}
	clip := makeClip(0, 0, 200, 200)

	renderDrawCanvas(shape, clip, w)

	var n int
	for _, r := range w.renderers {
		if r.Kind == RenderImage {
			n++
		}
	}
	if n != 1 {
		t.Errorf("RenderImage count = %d, want 1", n)
	}
}

func TestRenderDrawCanvas_ValidBackingScalePassedToDrawContext(t *testing.T) {
	w := makeWindowWithScratch()
	w.BackingScale = 2.0
	var got float32
	shape := &Shape{
		ShapeType: ShapeDrawCanvas,
		Width:     100, Height: 100,
		Color: ColorTransparent,
		Events: &EventHandlers{
			OnDraw: func(dc *DrawContext) { got = dc.Scale },
		},
	}
	renderDrawCanvas(shape, makeClip(0, 0, 200, 200), w)
	if got != 2.0 {
		t.Errorf("dc.Scale = %v, want 2.0", got)
	}
}

func TestRenderDrawCanvas_ZeroBackingScaleDefaultsToOne(t *testing.T) {
	w := makeWindowWithScratch()
	// BackingScale zero-value: window before first backend frame.
	var got float32
	shape := &Shape{
		ShapeType: ShapeDrawCanvas,
		Width:     100, Height: 100,
		Color: ColorTransparent,
		Events: &EventHandlers{
			OnDraw: func(dc *DrawContext) { got = dc.Scale },
		},
	}
	renderDrawCanvas(shape, makeClip(0, 0, 200, 200), w)
	if got != 1.0 {
		t.Errorf("dc.Scale = %v, want 1.0 for zero BackingScale", got)
	}
}

func TestRenderDrawCanvas_NegativeBackingScaleDefaultsToOne(t *testing.T) {
	w := makeWindowWithScratch()
	w.BackingScale = -2.0
	var got float32
	shape := &Shape{
		ShapeType: ShapeDrawCanvas,
		Width:     100, Height: 100,
		Color: ColorTransparent,
		Events: &EventHandlers{
			OnDraw: func(dc *DrawContext) { got = dc.Scale },
		},
	}
	renderDrawCanvas(shape, makeClip(0, 0, 200, 200), w)
	if got != 1.0 {
		t.Errorf("dc.Scale = %v, want 1.0 for negative BackingScale", got)
	}
}

func TestRenderDrawCanvas_NaNBackingScaleDefaultsToOne(t *testing.T) {
	w := makeWindowWithScratch()
	w.BackingScale = float32(math.NaN())
	var got float32
	shape := &Shape{
		ShapeType: ShapeDrawCanvas,
		Width:     100, Height: 100,
		Color: ColorTransparent,
		Events: &EventHandlers{
			OnDraw: func(dc *DrawContext) { got = dc.Scale },
		},
	}
	renderDrawCanvas(shape, makeClip(0, 0, 200, 200), w)
	if got != 1.0 {
		t.Errorf("dc.Scale = %v, want 1.0 for NaN BackingScale", got)
	}
}

func TestRenderDrawCanvas_InfBackingScaleDefaultsToOne(t *testing.T) {
	w := makeWindowWithScratch()
	for _, scale := range []float32{
		float32(math.Inf(1)),
		float32(math.Inf(-1)),
	} {
		w.BackingScale = scale
		var got float32
		shape := &Shape{
			ShapeType: ShapeDrawCanvas,
			Width:     100, Height: 100,
			Color: ColorTransparent,
			Events: &EventHandlers{
				OnDraw: func(dc *DrawContext) { got = dc.Scale },
			},
		}
		w.renderers = w.renderers[:0]
		renderDrawCanvas(shape, makeClip(0, 0, 200, 200), w)
		if got != 1.0 {
			t.Errorf("BackingScale=%v: dc.Scale = %v, want 1.0", scale, got)
		}
	}
}

func TestRenderDrawCanvasEmptyIDAlwaysRedraws(t *testing.T) {
	w := makeWindowWithScratch()
	callCount := 0
	shape := &Shape{
		ShapeType: ShapeDrawCanvas,
		ID:        "", // empty ID
		Width:     100, Height: 100,
		Version: 1,
		Color:   RGB(100, 100, 100),
		Events: &EventHandlers{
			OnDraw: func(dc *DrawContext) {
				callCount++
				dc.batches = append(dc.batches, DrawCanvasTriBatch{
					Triangles: []float32{0, 0, 1, 0, 0, 1},
					Color:     RGB(255, 0, 0),
				})
			},
		},
	}
	clip := makeClip(0, 0, 200, 200)

	renderDrawCanvas(shape, clip, w)
	renderDrawCanvas(shape, clip, w)

	if callCount != 2 {
		t.Errorf("callCount = %d, want 2 (empty ID always redraws)",
			callCount)
	}
}
