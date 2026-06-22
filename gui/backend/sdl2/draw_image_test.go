//go:build !js && !darwin

package sdl2

import (
	"image"
	"image/color"
	"image/png"
	"os"
	"path/filepath"
	"testing"

	"github.com/go-gui-org/go-gui/gui/backend/internal/imgload"
)

func TestResolveValidatedPathAllowedRoots(t *testing.T) {
	root := t.TempDir()
	imgPath := filepath.Join(root, "img.png")
	if err := os.WriteFile(imgPath, []byte("x"), 0o644); err != nil {
		t.Fatalf("write image: %v", err)
	}

	got, err := imgload.ResolveValidatedPath(imgPath, []string{root})
	if err != nil {
		t.Fatalf("resolve path: %v", err)
	}
	if got == "" {
		t.Fatal("expected non-empty resolved path")
	}
}

func TestResolveValidatedPathRejectsOutsideRoots(t *testing.T) {
	root := t.TempDir()
	outsideDir := t.TempDir()
	imgPath := filepath.Join(outsideDir, "img.png")
	if err := os.WriteFile(imgPath, []byte("x"), 0o644); err != nil {
		t.Fatalf("write image: %v", err)
	}

	if _, err := imgload.ResolveValidatedPath(
		imgPath, []string{root}); err == nil {
		t.Fatal("expected disallowed image path")
	}
}

func TestDecodeNRGBARejectsByBytes(t *testing.T) {
	path := filepath.Join(t.TempDir(), "big.bin")
	if err := os.WriteFile(path, make([]byte, 1024), 0o644); err != nil {
		t.Fatalf("write file: %v", err)
	}
	f, err := os.Open(path)
	if err != nil {
		t.Fatalf("open file: %v", err)
	}
	defer func() { _ = f.Close() }()

	if _, err := imgload.DecodeNRGBA(path, f, 32, 0); err == nil {
		t.Fatal("expected image byte limit error")
	}
}

func TestDecodeNRGBARejectsByPixels(t *testing.T) {
	path := filepath.Join(t.TempDir(), "img.png")
	if err := writePNG(path, 100, 100); err != nil {
		t.Fatalf("write png: %v", err)
	}
	f, err := os.Open(path)
	if err != nil {
		t.Fatalf("open file: %v", err)
	}
	defer func() { _ = f.Close() }()

	if _, err := imgload.DecodeNRGBA(
		path, f, 1<<20, 1000); err == nil {
		t.Fatal("expected image pixel limit error")
	}
}

func writePNG(path string, w, h int) error {
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	for y := range h {
		for x := range w {
			img.Set(x, y, color.RGBA{R: 200, G: 100, B: 50, A: 255})
		}
	}
	f, err := os.Create(path)
	if err != nil {
		return err
	}
	defer func() { _ = f.Close() }()
	return png.Encode(f, img)
}
