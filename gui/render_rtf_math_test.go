package gui

import (
	"testing"

	"github.com/go-gui-org/go-glyph"
)

func TestRenderRtfEmitsInlineMathImages(t *testing.T) {
	cache := NewBoundedDiagramCache(10)
	mathID := "a+b"
	hash := diagramCacheHash(mathID)
	cache.Set(hash, DiagramCacheEntry{
		State:   DiagramReady,
		PNGPath: "/tmp/math.png",
		Width:   80,
		Height:  30,
		DPI:     150,
	})

	layout := &glyph.Layout{
		Items: []glyph.Item{
			{RunText: "text", Width: 40, X: 0, Y: 12},
			{
				IsObject: true,
				ObjectID: mathID,
				Width:    50,
				X:        40,
				Y:        12,
				Ascent:   10,
				Descent:  3,
			},
		},
	}

	shape := &Shape{
		X: 10, Y: 20, Width: 300, Height: 100,
		TC: &ShapeTextConfig{
			RtfLayout:     layout,
			rtfMathHashes: []int64{hash},
		},
	}
	w := &Window{}
	w.viewState.diagramCache = cache
	clip := drawClip{X: 0, Y: 0, Width: 500, Height: 500}

	renderRtf(shape, clip, w)

	hasRTF := false
	hasImage := false
	for _, r := range w.renderers {
		if r.Kind == RenderRTF {
			hasRTF = true
		}
		if r.Kind == RenderImage {
			hasImage = true
			if r.Resource != "/tmp/math.png" {
				t.Fatalf("resource: got %q", r.Resource)
			}
			// X = 10 (shape.X) + 0 (pad) + 40 (item.X)
			if r.X != 50 {
				t.Fatalf("X: got %f, want 50", r.X)
			}
			// Y = 20 (shape.Y) + 0 (pad) + 12 (item.Y) - 10 (ascent)
			if r.Y != 22 {
				t.Fatalf("Y: got %f, want 22", r.Y)
			}
			if r.W != 50 {
				t.Fatalf("W: got %f, want 50", r.W)
			}
			// H = ascent + descent = 13
			if r.H != 13 {
				t.Fatalf("H: got %f, want 13", r.H)
			}
		}
	}
	if !hasRTF {
		t.Fatal("no RenderRTF emitted")
	}
	if !hasImage {
		t.Fatal("no RenderImage emitted for inline math")
	}
}

func TestRenderRtfNoImagesWithoutCache(t *testing.T) {
	layout := &glyph.Layout{
		Items: []glyph.Item{{
			IsObject: true,
			ObjectID: "x",
			Width:    50,
			X:        0,
			Y:        12,
			Ascent:   10,
			Descent:  3,
		}},
	}
	shape := &Shape{
		X: 0, Y: 0, Width: 200, Height: 100,
		TC: &ShapeTextConfig{RtfLayout: layout},
	}
	w := &Window{}
	clip := drawClip{X: 0, Y: 0, Width: 500, Height: 500}

	renderRtf(shape, clip, w)

	for _, r := range w.renderers {
		if r.Kind == RenderImage {
			t.Fatal("should not emit RenderImage without cache")
		}
	}
}

func TestRenderRtfSkipsLoadingEntry(t *testing.T) {
	cache := NewBoundedDiagramCache(10)
	hash := diagramCacheHash("q")
	cache.Set(hash, DiagramCacheEntry{
		State: DiagramLoading,
	})

	layout := &glyph.Layout{
		Items: []glyph.Item{{
			IsObject: true,
			ObjectID: "q",
			Width:    50,
			X:        0,
			Y:        12,
			Ascent:   10,
			Descent:  3,
		}},
	}
	shape := &Shape{
		X: 0, Y: 0, Width: 200, Height: 100,
		TC: &ShapeTextConfig{RtfLayout: layout},
	}
	w := &Window{}
	w.viewState.diagramCache = cache
	clip := drawClip{X: 0, Y: 0, Width: 500, Height: 500}

	renderRtf(shape, clip, w)

	for _, r := range w.renderers {
		if r.Kind == RenderImage {
			t.Fatal("should not emit RenderImage for loading entry")
		}
	}
}
