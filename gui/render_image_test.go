package gui

import (
	"os"
	"path/filepath"
	"testing"
)

func TestRenderImageEmitsCommand(t *testing.T) {
	w := &Window{}
	w.clipRadius = 5
	shape := &Shape{
		shapeType: shapeImage,
		X:         10,
		Y:         20,
		Width:     100,
		Height:    80,
		Resource:  "test.png",
	}
	clip := drawClip{X: 0, Y: 0, Width: 500, Height: 500}
	renderImage(shape, clip, w)

	found := false
	for _, r := range w.renderers {
		if r.Kind == RenderImage {
			found = true
			if r.X != 10 || r.Y != 20 {
				t.Fatalf("expected pos (10,20), got (%f,%f)",
					r.X, r.Y)
			}
			if r.W != 100 || r.H != 80 {
				t.Fatalf("expected size (100,80), got (%f,%f)",
					r.W, r.H)
			}
			if r.Resource != "test.png" {
				t.Fatalf("expected resource test.png, got %s",
					r.Resource)
			}
			if r.ClipRadius != 5 {
				t.Fatalf("expected clip radius 5, got %f",
					r.ClipRadius)
			}
		}
	}
	if !found {
		t.Fatal("no RenderImage emitted")
	}
}

func TestRenderImageOutOfClip(t *testing.T) {
	w := &Window{}
	shape := &Shape{
		shapeType: shapeImage,
		X:         1000,
		Y:         1000,
		Width:     50,
		Height:    50,
	}
	clip := drawClip{X: 0, Y: 0, Width: 100, Height: 100}
	renderImage(shape, clip, w)
	if len(w.renderers) != 0 {
		t.Fatal("expected no render commands when out of clip")
	}
	for _, r := range w.renderers {
		if r.Kind == RenderImage {
			t.Fatal("should not emit RenderImage when clipped")
		}
	}
}

func TestRenderImageBgColorPassedThrough(t *testing.T) {
	w := &Window{}
	shape := &Shape{
		shapeType: shapeImage,
		X:         10,
		Y:         20,
		Width:     100,
		Height:    80,
		Resource:  "test.png",
		Color:     White,
		Opacity:   1.0,
	}
	clip := drawClip{X: 0, Y: 0, Width: 500, Height: 500}
	renderImage(shape, clip, w)

	for _, r := range w.renderers {
		if r.Kind == RenderImage {
			if r.Color != White {
				t.Fatalf("expected Color White, got %v", r.Color)
			}
			return
		}
	}
	t.Fatal("no RenderImage emitted")
}

func TestRenderImageBgColorNoContainerRect(t *testing.T) {
	w := &Window{}
	shape := &Shape{
		shapeType: shapeImage,
		X:         10,
		Y:         20,
		Width:     100,
		Height:    80,
		Resource:  "test.png",
		Color:     White,
		Opacity:   1.0,
	}
	clip := drawClip{X: 0, Y: 0, Width: 500, Height: 500}
	renderImage(shape, clip, w)

	for _, r := range w.renderers {
		if r.Kind == RenderRect {
			t.Fatal("renderContainer should not emit RenderRect for image bg")
		}
	}
}

func TestRenderImageIntegration(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "test.png")
	os.WriteFile(path, []byte("fake"), 0o644)

	w := &Window{
		windowWidth:  800,
		windowHeight: 600,
	}
	v := Image(ImageCfg{
		ID:     "img1",
		Src:    path,
		Width:  200,
		Height: 150,
	})
	layout := v.GenerateLayout(w)
	if layout.Shape.shapeType != shapeImage {
		t.Fatalf("expected shapeImage, got %d",
			layout.Shape.shapeType)
	}

	// Set positions for rendering.
	layout.Shape.X = 10
	layout.Shape.Y = 20
	layout.Shape.shapeClip = drawClip{
		X: 10, Y: 20, Width: 200, Height: 150,
	}

	clip := w.windowRect()
	renderImage(layout.Shape, clip, w)

	hasImage := false
	for _, r := range w.renderers {
		if r.Kind == RenderImage {
			hasImage = true
			if r.Resource != path {
				t.Fatalf("expected resource %s, got %s",
					path, r.Resource)
			}
		}
	}
	if !hasImage {
		t.Fatal("no RenderImage emitted")
	}
}
