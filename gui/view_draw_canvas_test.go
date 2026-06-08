package gui

import "testing"

func TestDrawCanvasGenerateLayout(t *testing.T) {
	w := &Window{}
	v := DrawCanvas(DrawCanvasCfg{
		ID:      "dc1",
		Version: 1,
		Width:   200,
		Height:  100,
		OnDraw: func(dc *DrawContext) {
			dc.FilledRect(0, 0, dc.Width, dc.Height, Red)
		},
	})
	layout := generateViewLayout(v, w)
	if layout.Shape.shapeType != shapeDrawCanvas {
		t.Errorf("shape type = %d, want shapeDrawCanvas", layout.Shape.shapeType)
	}
	if layout.Shape.Width != 200 {
		t.Errorf("width = %f", layout.Shape.Width)
	}
	if layout.Shape.events.OnDraw == nil {
		t.Error("OnDraw not set on shape")
	}
}

func TestDrawCanvasCaching(t *testing.T) {
	w := &Window{}
	callCount := 0
	cfg := DrawCanvasCfg{
		ID:      "dc-cache",
		Version: 1,
		Width:   50,
		Height:  50,
		OnDraw: func(dc *DrawContext) {
			callCount++
			dc.FilledRect(0, 0, 10, 10, Blue)
		},
	}
	clip := drawClip{Width: 100, Height: 100}

	// First call: draws.
	v := DrawCanvas(cfg)
	layout := generateViewLayout(v, w)
	renderDrawCanvas(layout.Shape, clip, w)
	if callCount != 1 {
		t.Fatalf("first call: count = %d", callCount)
	}

	// Same version: cache hit.
	v = DrawCanvas(cfg)
	layout = generateViewLayout(v, w)
	renderDrawCanvas(layout.Shape, clip, w)
	if callCount != 1 {
		t.Errorf("second call with same version: count = %d", callCount)
	}

	// Bump version: redraws.
	cfg.Version = 2
	v = DrawCanvas(cfg)
	layout = generateViewLayout(v, w)
	renderDrawCanvas(layout.Shape, clip, w)
	if callCount != 2 {
		t.Errorf("after version bump: count = %d", callCount)
	}
}

func TestDrawCanvasResizeRedraw(t *testing.T) {
	w := &Window{}
	callCount := 0
	lastWidth := float32(0)
	cfg := DrawCanvasCfg{
		ID:      "dc-resize",
		Version: 1,
		Width:   50,
		Height:  50,
		OnDraw: func(dc *DrawContext) {
			callCount++
			lastWidth = dc.Width
		},
	}
	clip := drawClip{Width: 100, Height: 100}

	// First draw.
	v := DrawCanvas(cfg)
	layout := generateViewLayout(v, w)
	renderDrawCanvas(layout.Shape, clip, w)
	if callCount != 1 || lastWidth != 50 {
		t.Fatalf("first draw: count=%d, width=%f", callCount, lastWidth)
	}

	// Change width (simulate layout engine change).
	layout.Shape.Width = 80
	renderDrawCanvas(layout.Shape, clip, w)
	if callCount != 2 || lastWidth != 80 {
		t.Errorf("after resize: count=%d, width=%f", callCount, lastWidth)
	}
}

func TestDrawCanvasDefaultSizing(t *testing.T) {
	v := DrawCanvas(DrawCanvasCfg{ID: "dc-def"})
	dv := v.(*drawCanvasView)
	if dv.cfg.Sizing != FixedFixed {
		t.Errorf("default sizing = %v, want FixedFixed", dv.cfg.Sizing)
	}
}

func TestDrawCanvasNoOnDraw(t *testing.T) {
	w := &Window{}
	v := DrawCanvas(DrawCanvasCfg{
		ID:     "dc-nil",
		Width:  10,
		Height: 10,
	})
	layout := generateViewLayout(v, w)
	if layout.Shape.shapeType != shapeDrawCanvas {
		t.Error("expected shapeDrawCanvas")
	}
}

func TestRenderDrawCanvas(t *testing.T) {
	w := &Window{
		windowWidth: 800, windowHeight: 600,
	}
	v := DrawCanvas(DrawCanvasCfg{
		ID:      "dc-render",
		Version: 1,
		Width:   100,
		Height:  80,
		Clip:    true,
		OnDraw: func(dc *DrawContext) {
			dc.FilledRect(0, 0, 50, 50, Green)
		},
	})
	layout := generateViewLayout(v, w)

	clip := drawClip{X: 0, Y: 0, Width: 800, Height: 600}
	w.renderers = w.renderers[:0]
	renderDrawCanvas(layout.Shape, clip, w)
	// Should have: container + clip + svg + restore clip.
	if len(w.renderers) < 3 {
		t.Fatalf("renderers = %d, want >= 3", len(w.renderers))
	}

	// Assert clip restore values.
	var clips []RenderCmd
	for _, r := range w.renderers {
		if r.Kind == RenderClip {
			clips = append(clips, r)
		}
	}
	if len(clips) != 2 {
		t.Fatalf("clip commands = %d, want 2", len(clips))
	}
	// First clip: content area.
	if clips[0].W != 100 || clips[0].H != 80 {
		t.Errorf("content clip = (%g, %g), want (100, 80)",
			clips[0].W, clips[0].H)
	}
	// Restore clip: parent clip.
	if clips[1].X != 0 || clips[1].Y != 0 ||
		clips[1].W != 800 || clips[1].H != 600 {
		t.Errorf("restore clip = (%g, %g, %g, %g), want (0, 0, 800, 600)",
			clips[1].X, clips[1].Y, clips[1].W, clips[1].H)
	}
}

func TestDrawCanvasOffScreenSkip(t *testing.T) {
	w := &Window{}
	callCount := 0
	v := DrawCanvas(DrawCanvasCfg{
		Width: 50, Height: 50,
		OnDraw: func(dc *DrawContext) {
			callCount++
			dc.FilledRect(0, 0, 10, 10, Red)
		},
	})
	layout := generateViewLayout(v, w)
	// Place canvas at (200, 200); clip region doesn't overlap.
	layout.Shape.X = 200
	layout.Shape.Y = 200
	clip := drawClip{X: 0, Y: 0, Width: 100, Height: 100}

	w.renderers = w.renderers[:0]
	renderDrawCanvas(layout.Shape, clip, w)

	if callCount != 0 {
		t.Errorf("off-screen canvas called OnDraw %d times", callCount)
	}
	// No render commands should be emitted (not even container).
	for _, r := range w.renderers {
		if r.Kind == RenderSvg {
			t.Error("off-screen canvas emitted RenderSvg")
		}
	}
}

func TestDrawCanvasFocusWiring(t *testing.T) {
	w := &Window{}
	called := false
	v := DrawCanvas(DrawCanvasCfg{
		ID:      "dc-focus",
		Width:   50,
		Height:  50,
		IDFocus: 42,
		OnKeyDown: func(_ *Layout, _ *Event, _ *Window) {
			called = true
		},
	})
	layout := generateViewLayout(v, w)

	if layout.Shape.IDFocus != 42 {
		t.Errorf("Shape.IDFocus = %d, want 42", layout.Shape.IDFocus)
	}
	if layout.Shape.A11YRole != AccessRoleButton {
		t.Errorf("A11YRole = %v, want AccessRoleButton when focusable",
			layout.Shape.A11YRole)
	}
	if layout.Shape.events == nil || layout.Shape.events.OnKeyDown == nil {
		t.Fatal("OnKeyDown not wired")
	}
	layout.Shape.events.OnKeyDown(&layout, &Event{}, w)
	if !called {
		t.Error("OnKeyDown did not fire")
	}
}

func TestDrawCanvasNonFocusableA11Role(t *testing.T) {
	w := &Window{}
	v := DrawCanvas(DrawCanvasCfg{
		ID: "dc-nofocus", Width: 10, Height: 10,
	})
	layout := generateViewLayout(v, w)
	if layout.Shape.A11YRole != AccessRoleImage {
		t.Errorf("A11YRole = %v, want AccessRoleImage when IDFocus=0",
			layout.Shape.A11YRole)
	}
	if layout.Shape.IDFocus != 0 {
		t.Errorf("Shape.IDFocus = %d, want 0", layout.Shape.IDFocus)
	}
}

func TestDrawCanvasFocusedReceivesKeydown(t *testing.T) {
	w := &Window{}
	var gotKey KeyCode
	v := DrawCanvas(DrawCanvasCfg{
		ID:      "dc-key",
		Width:   50,
		Height:  50,
		IDFocus: 7,
		OnKeyDown: func(_ *Layout, e *Event, _ *Window) {
			gotKey = e.KeyCode
			e.IsHandled = true
		},
	})
	layout := generateViewLayout(v, w)
	root := Layout{Shape: &Shape{}, Children: []Layout{layout}}

	w.SetIDFocus(7)
	e := &Event{KeyCode: KeyRight}
	keydownHandler(&root, e, w)

	if gotKey != KeyRight {
		t.Errorf("KeyCode = %v, want KeyRight", gotKey)
	}
	if !e.IsHandled {
		t.Error("event not marked handled")
	}
}

func TestDrawCanvasEmptyIDNoCollision(t *testing.T) {
	w := &Window{}
	clip := drawClip{Width: 200, Height: 200}

	v1 := DrawCanvas(DrawCanvasCfg{
		Width: 50, Height: 50,
		OnDraw: func(dc *DrawContext) {
			dc.FilledRect(0, 0, 10, 10, Red)
		},
	})
	v2 := DrawCanvas(DrawCanvasCfg{
		Width: 50, Height: 50,
		OnDraw: func(dc *DrawContext) {
			dc.FilledRect(0, 0, 20, 20, Blue)
			dc.FilledRect(20, 20, 10, 10, Green)
		},
	})

	l1 := generateViewLayout(v1, w)
	l2 := generateViewLayout(v2, w)

	w.renderers = w.renderers[:0]
	renderDrawCanvas(l1.Shape, clip, w)
	var svg1 int
	for _, r := range w.renderers {
		if r.Kind == RenderSvg {
			svg1++
		}
	}

	w.renderers = w.renderers[:0]
	renderDrawCanvas(l2.Shape, clip, w)
	var svg2 int
	for _, r := range w.renderers {
		if r.Kind == RenderSvg {
			svg2++
		}
	}

	if svg1 != 1 {
		t.Errorf("canvas 1: RenderSvg count = %d, want 1", svg1)
	}
	if svg2 != 2 {
		t.Errorf("canvas 2: RenderSvg count = %d, want 2", svg2)
	}
}
