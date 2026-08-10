//go:build windows

package hicon

import (
	"bytes"
	"image"
	"image/color"
	"image/png"
	"testing"
)

// smallIconPNG builds an 16x16 RGBA PNG with an opaque center and a
// fully transparent corner, the shape a real app icon has.
func smallIconPNG(t *testing.T) []byte {
	t.Helper()
	img := image.NewNRGBA(image.Rect(0, 0, 16, 16))
	for y := range 16 {
		for x := range 16 {
			a := uint8(255)
			if x == 0 && y == 0 {
				a = 0 // fully transparent corner pixel
			}
			img.SetNRGBA(x, y, color.NRGBA{R: 200, G: 100, B: 50, A: a})
		}
	}
	return encodePNG(t, img)
}

func encodePNG(t *testing.T, img image.Image) []byte {
	t.Helper()
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		t.Fatalf("png encode: %v", err)
	}
	return buf.Bytes()
}

// TestFromPNG exercises the only path through this package: a valid
// PNG must yield a real HICON (issue #235: a NULL mask made
// CreateIconIndirect fail silently, returning an error here).
func TestFromPNG(t *testing.T) {
	h, err := FromPNG(smallIconPNG(t), 32)
	if err != nil {
		t.Fatalf("FromPNG: %v", err)
	}
	if h == 0 {
		t.Fatal("FromPNG returned a zero HICON")
	}
	Destroy(h)
}

// TestFromPNGOddWidth covers an odd-width icon, which exercises the
// mask stride rounding (1bpp rows are word-aligned).
func TestFromPNGOddWidth(t *testing.T) {
	img := image.NewNRGBA(image.Rect(0, 0, 17, 5))
	for y := range 5 {
		for x := range 17 {
			img.SetNRGBA(x, y, color.NRGBA{R: 10, G: 20, B: 30, A: 255})
		}
	}
	h, err := FromPNG(encodePNG(t, img), 32)
	if err != nil {
		t.Fatalf("FromPNG: %v", err)
	}
	if h == 0 {
		t.Fatal("FromPNG returned a zero HICON")
	}
	Destroy(h)
}

func TestFromPNGBadData(t *testing.T) {
	if h, err := FromPNG([]byte("not a png"), 32); err == nil {
		Destroy(h)
		t.Fatal("FromPNG accepted garbage bytes")
	}
}

func TestFromPNGTooLarge(t *testing.T) {
	h, err := FromPNG(smallIconPNG(t), 8)
	if err == nil {
		Destroy(h)
		t.Fatal("FromPNG accepted an icon larger than maxDim")
	}
}
