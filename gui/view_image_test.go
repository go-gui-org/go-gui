package gui

import (
	"os"
	"path/filepath"
	"testing"
)

func TestImageFactory(t *testing.T) {
	v := Image(ImageCfg{ID: "img1", Src: "test.png"})
	if v == nil {
		t.Fatal("Image factory returned nil")
	}
	if _, ok := v.(*imageView); !ok {
		t.Fatal("expected *imageView")
	}
}

func TestImageInvisible(t *testing.T) {
	v := Image(ImageCfg{ID: "img1", Invisible: true})
	if _, ok := v.(*imageView); ok {
		t.Fatal("invisible should not return *imageView")
	}
}

func TestImageContent(t *testing.T) {
	v := Image(ImageCfg{ID: "img1", Src: "test.png"})
	if v.Content() != nil {
		t.Fatal("expected nil Content")
	}
}

func TestImageGenerateLayoutLocalMissing(t *testing.T) {
	w := &Window{}
	v := Image(ImageCfg{
		ID:  "img1",
		Src: "/nonexistent/photo.png",
	})
	layout := v.GenerateLayout(w)
	// Missing file → error text with magenta color.
	if layout.Shape == nil {
		t.Fatal("expected shape")
	}
	if layout.Shape.shapeType != shapeText {
		t.Fatalf("expected shapeText for missing, got %d",
			layout.Shape.shapeType)
	}
}

func TestImageGenerateLayoutLocalExists(t *testing.T) {
	// Create a temp file to simulate an image.
	dir := t.TempDir()
	path := filepath.Join(dir, "test.png")
	if err := os.WriteFile(path, []byte("fake"), 0o644); err != nil {
		t.Fatal(err)
	}
	w := &Window{}
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
	if layout.Shape.Resource != path {
		t.Fatalf("expected resource %s, got %s",
			path, layout.Shape.Resource)
	}
	if layout.Shape.Width != 200 {
		t.Fatalf("expected width 200, got %f",
			layout.Shape.Width)
	}
}

func TestImageOpacityDefault(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "test.png")
	os.WriteFile(path, []byte("fake"), 0o644)
	w := &Window{}
	v := Image(ImageCfg{Src: path, Width: 100, Height: 100})
	layout := v.GenerateLayout(w)
	if layout.Shape.Opacity != 1.0 {
		t.Fatalf("expected Opacity 1.0, got %f",
			layout.Shape.Opacity)
	}
}

func TestImageBgColor(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "test.png")
	os.WriteFile(path, []byte("fake"), 0o644)
	w := &Window{}
	v := Image(ImageCfg{
		Src: path, Width: 100, Height: 100,
		BgColor: White,
	})
	layout := v.GenerateLayout(w)
	if layout.Shape.Color != White {
		t.Fatalf("expected Color White, got %v",
			layout.Shape.Color)
	}
}

func TestImageDefaultDimensions(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "test.png")
	os.WriteFile(path, []byte("fake"), 0o644)
	w := &Window{}
	v := Image(ImageCfg{ID: "img1", Src: path})
	layout := v.GenerateLayout(w)
	if layout.Shape.Width != 100 || layout.Shape.Height != 100 {
		t.Fatalf("expected default 100x100, got %fx%f",
			layout.Shape.Width, layout.Shape.Height)
	}
}

func TestImageWithEvents(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "test.png")
	os.WriteFile(path, []byte("fake"), 0o644)
	w := &Window{}
	clicked := false
	v := Image(ImageCfg{
		ID:    "img1",
		Src:   path,
		Width: 50, Height: 50,
		OnClick: func(_ *Layout, _ *Event, _ *Window) {
			clicked = true
		},
	})
	layout := v.GenerateLayout(w)
	if layout.Shape.events == nil {
		t.Fatal("expected events")
	}
	// Simulate left click.
	layout.Shape.events.OnClick(&layout, &Event{
		MouseButton: MouseLeft,
	}, w)
	if !clicked {
		t.Fatal("click handler not called")
	}
	_ = clicked
}

func TestImageA11Y(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "test.png")
	os.WriteFile(path, []byte("fake"), 0o644)
	w := &Window{}
	v := Image(ImageCfg{
		ID:              "img1",
		Src:             path,
		Width:           50,
		Height:          50,
		A11YLabel:       "test image",
		A11YDescription: "a test",
	})
	layout := v.GenerateLayout(w)
	if layout.Shape.A11YRole != AccessRoleImage {
		t.Fatal("expected AccessRoleImage")
	}
	if layout.Shape.A11Y == nil {
		t.Fatal("expected A11Y info")
	}
	if layout.Shape.A11Y.Label != "test image" {
		t.Fatalf("expected label 'test image', got %q",
			layout.Shape.A11Y.Label)
	}
}
