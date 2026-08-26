package gui

import (
	"fmt"
	"math"
	"testing"
)

func TestRenderDrawCanvasOutsideClipSkips(t *testing.T) {
	w := makeWindowWithScratch()
	shape := &Shape{
		shapeType: shapeDrawCanvas,
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
		shapeType: shapeDrawCanvas,
		Width:     100, Height: 100,
		Color: RGB(100, 100, 100),
		events: &eventHandlers{
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
		shapeType: shapeDrawCanvas,
		Width:     100, Height: 100,
		Color: ColorTransparent,
		events: &eventHandlers{
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
		shapeType: shapeDrawCanvas,
		Width:     100, Height: 100,
		Color: RGB(100, 100, 100),
		Clip:  true,
		events: &eventHandlers{
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
		shapeType: shapeDrawCanvas,
		ID:        "test-canvas",
		Width:     100, Height: 100,
		Version: 1,
		Color:   RGB(100, 100, 100),
		events: &eventHandlers{
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
		shapeType: shapeDrawCanvas,
		Width:     100, Height: 100,
		Color: ColorTransparent,
		events: &eventHandlers{
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
		shapeType: shapeDrawCanvas,
		X:         10, Y: 20,
		Width: 100, Height: 100,
		Color:   ColorTransparent,
		Padding: NewPadding(5, 5, 5, 5),
		events: &eventHandlers{
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
				shapeType: shapeDrawCanvas,
				Width:     50, Height: 50,
				Color: ColorTransparent,
				events: &eventHandlers{
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
		shapeType: shapeDrawCanvas,
		Width:     50, Height: 50,
		Color: ColorTransparent,
		events: &eventHandlers{
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
		shapeType: shapeDrawCanvas,
		Width:     100, Height: 100,
		Color: ColorTransparent,
		events: &eventHandlers{
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
		shapeType: shapeDrawCanvas,
		Width:     100, Height: 100,
		Color: ColorTransparent,
		events: &eventHandlers{
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
		shapeType: shapeDrawCanvas,
		Width:     100, Height: 100,
		Color: ColorTransparent,
		events: &eventHandlers{
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
		shapeType: shapeDrawCanvas,
		Width:     100, Height: 100,
		Color: ColorTransparent,
		events: &eventHandlers{
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
			shapeType: shapeDrawCanvas,
			Width:     100, Height: 100,
			Color: ColorTransparent,
			events: &eventHandlers{
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
		shapeType: shapeDrawCanvas,
		ID:        "", // empty ID
		Width:     100, Height: 100,
		Version: 1,
		Color:   RGB(100, 100, 100),
		events: &eventHandlers{
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

// TestRenderDrawCanvasClipStaysInsideViewport pins the scroll bleed: a
// clipping canvas scrolled so its top is above the viewport must emit
// a scissor bounded by what it inherits, not its own content box. The
// broken form emitted the raw content box and painted the canvas over
// whatever sat above the scroll panel.
func TestRenderDrawCanvasClipStaysInsideViewport(t *testing.T) {
	w := makeWindowWithScratch()
	shape := &Shape{
		shapeType: shapeDrawCanvas,
		// Scrolled 40px above the viewport, which starts at y=20.
		X: 20, Y: -40,
		Width: 200, Height: 150,
		Padding: PadAll(10),
		Clip:    true,
		Color:   RGB(100, 100, 100),
		events: &eventHandlers{
			OnDraw: func(dc *DrawContext) {
				dc.batches = append(dc.batches, DrawCanvasTriBatch{
					Triangles: []float32{0, 0, 1, 0, 0, 1},
					Color:     RGB(255, 0, 0),
				})
			},
		},
	}
	viewport := makeClip(20, 20, 360, 260)

	renderDrawCanvas(shape, viewport, w)

	var clips []RenderCmd
	for _, r := range w.renderers {
		if r.Kind == RenderClip {
			clips = append(clips, r)
		}
	}
	if len(clips) == 0 {
		t.Fatal("no clip emitted for a clipping canvas")
	}
	got := clips[0]
	if got.Y < viewport.Y {
		t.Errorf("clip escapes above the viewport: Y=%v, want >= %v",
			got.Y, viewport.Y)
	}
	if bottom := got.Y + got.H; bottom > viewport.Y+viewport.Height {
		t.Errorf("clip spills below the viewport: bottom=%v, want <= %v",
			bottom, viewport.Y+viewport.Height)
	}
	// Content box spans y=-30..100; the viewport starts at 20, so the
	// intersection is 80 tall.
	want := drawClip{X: 30, Y: 20, Width: 180, Height: 80}
	if got.X != want.X || got.Y != want.Y ||
		got.W != want.Width || got.H != want.Height {
		t.Errorf("clip: got x=%v y=%v w=%v h=%v, want %+v",
			got.X, got.Y, got.W, got.H, want)
	}
}

// TestRenderDrawCanvasEmitsVertexColors covers the whole seam a canvas
// gradient rides on: the batch's per-vertex colors must reach the
// RenderSvg command, and must survive validSvgCmd, which rejects a
// command whose color count does not match its vertex count.
func TestRenderDrawCanvasEmitsVertexColors(t *testing.T) {
	w := makeWindowWithScratch()
	shape := &Shape{
		shapeType: shapeDrawCanvas,
		Width:     100, Height: 100,
		Color: RGB(100, 100, 100),
		events: &eventHandlers{
			OnDraw: func(dc *DrawContext) {
				dc.FilledRectGradient(0, 0, 50, 50, &CanvasGradient{
					Stops: []GradientStop{
						{Color: RGB(255, 0, 0), Pos: 0},
						{Color: RGB(0, 0, 255), Pos: 1},
					},
				})
			},
		},
	}

	renderDrawCanvas(shape, makeClip(0, 0, 200, 200), w)

	var found bool
	for i := range w.renderers {
		r := &w.renderers[i]
		if r.Kind != RenderSvg {
			continue
		}
		found = true
		if len(r.VertexColors) == 0 {
			t.Fatal("RenderSvg carries no VertexColors")
		}
		if len(r.VertexColors)*2 != len(r.Triangles) {
			t.Fatalf("VertexColors=%d, Triangles=%d",
				len(r.VertexColors), len(r.Triangles))
		}
		if r.VertexColors[0] == r.VertexColors[len(r.VertexColors)-1] {
			t.Error("first and last vertex share a color; " +
				"the ramp was not applied")
		}
	}
	if !found {
		t.Fatal("expected a RenderSvg command for the gradient batch")
	}
}

// TestRenderDrawCanvasFlatBatchHasNoVertexColors guards the other half
// of the invariant: an ordinary flat fill must not start carrying a
// colors slice, or every existing canvas would change how it renders.
func TestRenderDrawCanvasFlatBatchHasNoVertexColors(t *testing.T) {
	w := makeWindowWithScratch()
	shape := &Shape{
		shapeType: shapeDrawCanvas,
		Width:     100, Height: 100,
		events: &eventHandlers{
			OnDraw: func(dc *DrawContext) {
				dc.FilledRect(0, 0, 50, 50, RGB(255, 0, 0))
			},
		},
	}

	renderDrawCanvas(shape, makeClip(0, 0, 200, 200), w)

	for i := range w.renderers {
		r := &w.renderers[i]
		if r.Kind == RenderSvg && r.VertexColors != nil {
			t.Error("flat canvas batch emitted VertexColors")
		}
	}
}

// benchCanvasDraw is a stand-in for an animated canvas: a flat fan, a
// stroked ellipse and a radial gradient fill, the three shapes whose
// tessellation buffers dominate a real drawing app's frame.
func benchCanvasDraw(dc *DrawContext, t float32, circles int) {
	dc.FilledRect(0, 0, dc.Width, dc.Height, RGB(6, 8, 18))
	for i := range circles {
		x := 20 + float32(i%40)*6 + t
		dc.FilledCircle(x, 60+t, 9, RGBA(255, 250, 240, 40))
	}
	dc.Arc(150, 150, 120, 40, 0, 2*math.Pi, RGBA(150, 170, 210, 46), 1)
	dc.FilledCircleGradient(150, 150, 90, &benchCanvasGradient)
}

// Hoisted so the draw itself allocates nothing: the measurement is
// about the tessellation buffers, not the caller's literals.
var benchCanvasGradient = CanvasGradient{
	Radial: true,
	Stops: []GradientStop{
		{Color: RGBA(255, 186, 78, 200), Pos: 0},
		{Color: RGBA(255, 186, 78, 90), Pos: 0.4},
		{Color: RGBA(255, 186, 78, 0), Pos: 1},
	},
}

func benchCanvasShape(t *float32) *Shape {
	return benchCanvasShapeN(t, 40)
}

func benchCanvasShapeN(t *float32, circles int) *Shape {
	return &Shape{
		shapeType: shapeDrawCanvas,
		ID:        "bench-canvas",
		Width:     300, Height: 300,
		Color: ColorTransparent,
		events: &eventHandlers{
			OnDraw: func(dc *DrawContext) { benchCanvasDraw(dc, *t, circles) },
		},
	}
}

// BenchmarkDrawCanvasRedraw drives the path an animated canvas takes:
// a fresh Version every frame, so the cache always misses and OnDraw
// re-tessellates. Steady state must not allocate — the buffers come
// from the entry being replaced.
func BenchmarkDrawCanvasRedraw(b *testing.B) {
	w := makeWindowWithScratch()
	var t float32
	shape := benchCanvasShape(&t)
	clip := makeClip(0, 0, 300, 300)
	var version uint64

	// Two warm-up frames: the first allocates everything, the second
	// proves the pool is in place before the timer starts.
	for range 2 {
		version++
		shape.Version = version
		// Mirror buildRenderers: a new command list is a new pass.
		w.renderers = w.renderers[:0]
		w.renderPass++
		renderDrawCanvas(shape, clip, w)
	}

	b.ReportAllocs()
	b.ResetTimer()
	for b.Loop() {
		version++
		shape.Version = version
		t = float32(version%17) * 0.5
		// Mirror buildRenderers: a new command list is a new pass.
		w.renderers = w.renderers[:0]
		w.renderPass++
		renderDrawCanvas(shape, clip, w)
	}
}

// TestDrawCanvasReuseMatchesFresh is the correctness half of the
// pooling: a redraw that recycles the previous entry's buffers must
// emit exactly what a context starting from nil emits. A stale byte
// left in a reused buffer would show up here as a length or value
// mismatch.
func TestDrawCanvasReuseMatchesFresh(t *testing.T) {
	w := makeWindowWithScratch()
	var phase float32
	shape := benchCanvasShape(&phase)
	clip := makeClip(0, 0, 300, 300)

	// Frames with different geometry first, so the pooled buffers are
	// sized and dirtied by a draw that is not the one being compared.
	for i, p := range []float32{0, 7, 3} {
		phase = p
		shape.Version = uint64(i + 1)
		// Mirror buildRenderers: a new command list is a new pass.
		w.renderers = w.renderers[:0]
		w.renderPass++
		renderDrawCanvas(shape, clip, w)
	}

	sm := StateMap[string, drawCanvasCache](w, nsDrawCanvas, capModerate)
	pooled, ok := sm.Get("bench-canvas")
	if !ok {
		t.Fatal("no cache entry after redraw")
	}

	fresh := DrawContext{Width: 300, Height: 300, Scale: 1}
	benchCanvasDraw(&fresh, 3, 40)

	if len(pooled.Batches) != len(fresh.batches) {
		t.Fatalf("pooled %d batches, fresh %d",
			len(pooled.Batches), len(fresh.batches))
	}
	for i := range fresh.batches {
		want, got := &fresh.batches[i], &pooled.Batches[i]
		if got.Color != want.Color {
			t.Errorf("batch %d color = %v, want %v", i, got.Color, want.Color)
		}
		if len(got.Triangles) != len(want.Triangles) {
			t.Fatalf("batch %d: %d triangle floats, want %d",
				i, len(got.Triangles), len(want.Triangles))
		}
		for j := range want.Triangles {
			if got.Triangles[j] != want.Triangles[j] {
				t.Fatalf("batch %d tri[%d] = %v, want %v",
					i, j, got.Triangles[j], want.Triangles[j])
			}
		}
		if len(got.VertexColors) != len(want.VertexColors) {
			t.Fatalf("batch %d: %d vertex colors, want %d",
				i, len(got.VertexColors), len(want.VertexColors))
		}
		for j := range want.VertexColors {
			if got.VertexColors[j] != want.VertexColors[j] {
				t.Fatalf("batch %d col[%d] = %v, want %v",
					i, j, got.VertexColors[j], want.VertexColors[j])
			}
		}
	}
}

// TestDrawCanvasRedrawReusesBuffers pins the property the pooling
// exists for, by identity rather than by counting allocations: after
// the first couple of redraws, every batch must land in the same
// backing array it landed in last time.
//
// Counting was the obvious way to write this and does not work here.
// testing.AllocsPerRun reads process-wide malloc counters, so anything
// else running in the package is attributed to the measurement, and the
// error grows with how long the measured function takes — a whole
// canvas redraw is long enough that CI reported six allocations for a
// draw that allocates none. A pointer is exact and takes no wall time
// to observe.
//
// The heavy case is the one that matters. An earlier version of the
// pooling refused to recycle a buffer past a size cap, which left every
// canvas above it allocating its whole tessellation on every frame —
// precisely the ones the pooling is for.
func TestDrawCanvasRedrawReusesBuffers(t *testing.T) {
	for _, circles := range []int{40, 400} {
		t.Run(fmt.Sprintf("circles=%d", circles), func(t *testing.T) {
			w := makeWindowWithScratch()
			var phase float32
			shape := benchCanvasShapeN(&phase, circles)
			clip := makeClip(0, 0, 300, 300)
			var version uint64

			frame := func() {
				version++
				shape.Version = version
				phase = float32(version%17) * 0.5
				w.renderers = w.renderers[:0]
				w.renderPass++
				renderDrawCanvas(shape, clip, w)
			}
			// Three frames to settle: the two batch arrays alternate,
			// so each is grown on a different frame.
			for range 3 {
				frame()
			}

			sm := StateMap[string, drawCanvasCache](w, nsDrawCanvas,
				capModerate)
			before, ok := sm.Get("bench-canvas")
			if !ok {
				t.Fatal("no cache entry after redraw")
			}
			if len(before.Batches) < 3 {
				t.Fatalf("got %d batches, want the full set",
					len(before.Batches))
			}
			ptrs := batchDataPtrs(before.Batches)

			frame()

			after, _ := sm.Get("bench-canvas")
			if len(after.Batches) != len(before.Batches) {
				t.Fatalf("batch count moved from %d to %d",
					len(before.Batches), len(after.Batches))
			}
			for i, got := range batchDataPtrs(after.Batches) {
				if got != ptrs[i] {
					t.Errorf("batch %d re-allocated its triangles; "+
						"the redraw is not recycling", i)
				}
			}
		})
	}
}

// batchDataPtrs identifies each batch's triangle storage by the address
// of its first element, which is stable exactly when the buffer was
// reused rather than re-allocated.
func batchDataPtrs(bs []DrawCanvasTriBatch) []*float32 {
	out := make([]*float32, len(bs))
	for i := range bs {
		if len(bs[i].Triangles) > 0 {
			out[i] = &bs[i].Triangles[0]
		}
	}
	return out
}

// TestDrawCanvasSamePassNoReuse covers the guard against recycling
// buffers that a command already emitted in this list points at. Two
// canvases sharing an ID within one render pass is an invariant
// violation the debug gate reports, but it must degrade to allocating,
// not to overwriting geometry that is about to be drawn.
func TestDrawCanvasSamePassNoReuse(t *testing.T) {
	w := makeWindowWithScratch()
	var phase float32
	shape := benchCanvasShape(&phase)
	clip := makeClip(0, 0, 300, 300)

	w.renderPass++
	shape.Version = 1
	renderDrawCanvas(shape, clip, w)

	// Snapshot every emitted batch as the backend would see it. The
	// background rect is identical between draws, so the check has to
	// cover the batches that move with phase, not just the first one.
	type emitted struct{ live, want []float32 }
	var cmds []emitted
	for i := range w.renderers {
		if w.renderers[i].Kind != RenderSvg ||
			len(w.renderers[i].Triangles) == 0 {
			continue
		}
		tris := w.renderers[i].Triangles
		cmds = append(cmds, emitted{
			live: tris,
			want: append([]float32(nil), tris...),
		})
	}
	if len(cmds) < 2 {
		t.Fatalf("got %d canvas batches, want the full set", len(cmds))
	}

	// Same pass, same ID, different content.
	phase = 9
	shape.Version = 2
	renderDrawCanvas(shape, clip, w)

	for c := range cmds {
		for i := range cmds[c].want {
			if cmds[c].live[i] != cmds[c].want[i] {
				t.Fatalf("second draw overwrote emitted batch %d at %d: "+
					"%v, was %v", c, i, cmds[c].live[i], cmds[c].want[i])
			}
		}
	}
}
