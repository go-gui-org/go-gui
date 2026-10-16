package gui

import (
	"bytes"
	"fmt"
	"image"
	"image/color"
	"image/gif"
	"image/jpeg"
	"image/png"
	"math"
	"os"
	"path/filepath"
	"testing"
)

// pdfNearestWant is the embedded width of a 2-pixel-wide image drawn
// 64 points wide at PrintScaleActualSize with ImageFilterNearest:
// 64pt is 0.889in, 266.7 pixels at 300 dpi, so each source pixel
// repeats ceil(133.3) = 134 times.
const pdfNearestWant = 2 * 134

// renderPDFBytes renders cmds at actual size and returns the file.
func renderPDFBytes(t *testing.T, cmds []RenderCmd) []byte {
	t.Helper()
	j := testPrintJob(t)
	j.ScaleMode = PrintScaleActualSize
	if err := renderToPDF(cmds, j, 800, 600); err != nil {
		t.Fatal(err)
	}
	out, err := os.ReadFile(j.OutputPath)
	if err != nil {
		t.Fatal(err)
	}
	return out
}

// hasImageWidth reports whether the PDF holds an image XObject of the
// given pixel width. fpdf writes the XObject dictionary uncompressed.
func hasImageWidth(pdf []byte, w int) bool {
	return bytes.Contains(pdf, fmt.Appendf(nil, "/Width %d\n", w))
}

// A nearest image is embedded with each pixel repeated, so a viewer
// that smooths the scaled image still shows hard edges (#1069). A
// linear image keeps its own size.
func TestRenderToPDF_MemImageNearest(t *testing.T) {
	resetMemImages(t)
	src := UseImage("pdf/nearest", 2, 2, pixels(2, 2, 0x7f))
	tests := []struct {
		name   string
		filter ImageFilter
		want   int
	}{
		{"linear keeps native size", ImageFilterLinear, 2},
		{"nearest repeats pixels", ImageFilterNearest, pdfNearestWant},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			out := renderPDFBytes(t, []RenderCmd{{
				Kind: RenderImage, W: 64, H: 64, Resource: src,
				Opacity: 1, ImageFilter: tt.filter,
			}})
			if !hasImageWidth(out, tt.want) {
				t.Fatalf("no embedded image of width %d", tt.want)
			}
		})
	}
}

// The same nearest image drawn twice at one size is embedded once.
// Opaque pixels, so fpdf writes no second XObject for an alpha mask.
func TestRenderToPDF_MemImageNearestDedup(t *testing.T) {
	resetMemImages(t)
	src := UseImage("pdf/nearest-dedup", 2, 2, pixels(2, 2, 0xff))
	cmd := RenderCmd{
		Kind: RenderImage, W: 64, H: 64, Resource: src,
		Opacity: 1, ImageFilter: ImageFilterNearest,
	}
	out := renderPDFBytes(t, []RenderCmd{cmd, cmd})
	if n := bytes.Count(out, []byte("/Subtype /Image")); n != 1 {
		t.Fatalf("embedded %d images, want 1", n)
	}
}

// A file image is decoded, repeated and embedded as PNG for every
// format fpdf reads.
func TestRenderToPDF_FileImageNearest(t *testing.T) {
	img := image.NewNRGBA(image.Rect(0, 0, 2, 2))
	img.Set(0, 0, color.NRGBA{255, 0, 0, 255})
	img.Set(1, 1, color.NRGBA{0, 0, 255, 255})
	encoders := map[string]func(*bytes.Buffer) error{
		"png":  func(b *bytes.Buffer) error { return png.Encode(b, img) },
		"jpg":  func(b *bytes.Buffer) error { return jpeg.Encode(b, img, nil) },
		"gif":  func(b *bytes.Buffer) error { return gif.Encode(b, img, nil) },
		"jpeg": func(b *bytes.Buffer) error { return jpeg.Encode(b, img, nil) },
	}
	for ext, enc := range encoders {
		t.Run(ext, func(t *testing.T) {
			var buf bytes.Buffer
			if err := enc(&buf); err != nil {
				t.Fatal(err)
			}
			path := filepath.Join(t.TempDir(), "sprite."+ext)
			if err := os.WriteFile(path, buf.Bytes(), 0o600); err != nil {
				t.Fatal(err)
			}
			out := renderPDFBytes(t, []RenderCmd{{
				Kind: RenderImage, W: 64, H: 64, Resource: path,
				Opacity: 1, ImageFilter: ImageFilterNearest,
			}})
			if !hasImageWidth(out, pdfNearestWant) {
				t.Fatalf("no embedded image of width %d", pdfNearestWant)
			}
		})
	}
}

// Every output pixel is an exact copy of its source pixel: no
// blending at the block edges.
func TestPDFUpscaleNearest(t *testing.T) {
	src := image.NewNRGBA(image.Rect(0, 0, 2, 1))
	a := color.NRGBA{255, 0, 0, 255}
	b := color.NRGBA{0, 0, 255, 128}
	src.SetNRGBA(0, 0, a)
	src.SetNRGBA(1, 0, b)
	got := pdfUpscaleNearest(src, 3)
	if got.Rect.Dx() != 6 || got.Rect.Dy() != 3 {
		t.Fatalf("size = %v, want 6x3", got.Rect.Size())
	}
	for y := range 3 {
		for x := range 6 {
			want := a
			if x >= 3 {
				want = b
			}
			if c := got.NRGBAAt(x, y); c != want {
				t.Fatalf("pixel (%d,%d) = %v, want %v", x, y, c, want)
			}
		}
	}
}

func TestPDFNearestFactor(t *testing.T) {
	nan := math.NaN()
	tests := []struct {
		name       string
		srcW, srcH int
		wMM, hMM   float64
		want       int
	}{
		// 1in square at 300 dpi from 2 pixels: 150 per pixel.
		{"upscales to 300 dpi", 2, 2, 25.4, 25.4, 150},
		{"larger axis wins", 2, 2, 25.4, 50.8, 300},
		{"drawn at native size", 300, 300, 25.4, 25.4, 1},
		{"drawn smaller", 600, 600, 25.4, 25.4, 1},
		{"capped at 4096 per side", 1000, 10, 1000, 1000, 4},
		{"source past cap", 5000, 5000, 1000, 1000, 1},
		{"zero source", 0, 2, 25.4, 25.4, 1},
		{"zero drawn", 2, 2, 0, 0, 1},
		{"negative drawn", 2, 2, -25.4, -25.4, 1},
		{"NaN drawn", 2, 2, nan, nan, 1},
		{"Inf drawn", 2, 2, math.Inf(1), math.Inf(1), 1},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := pdfNearestFactor(tt.srcW, tt.srcH, tt.wMM, tt.hMM)
			if got != tt.want {
				t.Fatalf("factor = %d, want %d", got, tt.want)
			}
		})
	}
}

// A linear and a nearest draw of one source in one document embed two
// images: the repeated form lives under its own key and never replaces
// the original pixels.
func TestRenderToPDF_MemImageLinearAndNearest(t *testing.T) {
	resetMemImages(t)
	src := UseImage("pdf/both", 2, 2, pixels(2, 2, 0xff))
	cmd := RenderCmd{Kind: RenderImage, W: 64, H: 64, Resource: src, Opacity: 1}
	nearest := cmd
	nearest.ImageFilter = ImageFilterNearest
	out := renderPDFBytes(t, []RenderCmd{cmd, nearest})
	for _, w := range []int{2, pdfNearestWant} {
		if !hasImageWidth(out, w) {
			t.Fatalf("no embedded image of width %d", w)
		}
	}
}

// A nearest file image already at or past the target resolution is
// embedded as it is: no decode, no repeat.
func TestRenderToPDF_FileImageNearestAtTarget(t *testing.T) {
	const side = 300 // past the 266.7 pixels 64pt needs at 300 dpi
	var buf bytes.Buffer
	if err := png.Encode(&buf, image.NewNRGBA(image.Rect(0, 0, side, side))); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(t.TempDir(), "big.png")
	if err := os.WriteFile(path, buf.Bytes(), 0o600); err != nil {
		t.Fatal(err)
	}
	out := renderPDFBytes(t, []RenderCmd{{
		Kind: RenderImage, W: 64, H: 64, Resource: path,
		Opacity: 1, ImageFilter: ImageFilterNearest,
	}})
	if !hasImageWidth(out, side) {
		t.Fatalf("no embedded image of width %d", side)
	}
}
