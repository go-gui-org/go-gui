//go:build linux && !js && !android

package gl

import (
	"bytes"
	"image"
	"image/color"
	"image/png"
	"testing"
)

func TestDecodeIconARGB(t *testing.T) {
	// 3x1 PNG: opaque red, green, blue. Opaque pixels roundtrip
	// losslessly (the encoder premultiplies translucent NRGBA
	// sources, which would corrupt the byte-exact assertions) and
	// exercise every byte of the AARRGGBB layout.
	img := image.NewNRGBA(image.Rect(0, 0, 3, 1))
	img.Set(0, 0, color.NRGBA{R: 0xFF, G: 0x00, B: 0x00, A: 0xFF})
	img.Set(1, 0, color.NRGBA{R: 0x00, G: 0xFF, B: 0x00, A: 0xFF})
	img.Set(2, 0, color.NRGBA{R: 0x00, G: 0x00, B: 0xFF, A: 0xFF})
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		t.Fatalf("png encode: %v", err)
	}

	w, h, argb, err := decodeIconARGB(buf.Bytes(), 1024)
	if err != nil {
		t.Fatalf("decodeIconARGB: %v", err)
	}
	if w != 3 || h != 1 {
		t.Errorf("dims = %dx%d, want 3x1", w, h)
	}
	want := []uint32{
		0xFFFF0000, // AARRGGBB: alpha sits in the high byte
		0xFF00FF00,
		0xFF0000FF,
	}
	for i, v := range want {
		if argb[i] != v {
			t.Errorf("pixel %d = %08x, want %08x", i, argb[i], v)
		}
	}
}

func TestDecodeIconARGBRejects(t *testing.T) {
	if _, _, _, err := decodeIconARGB([]byte("not a png"), 1024); err == nil {
		t.Error("bad PNG accepted")
	}

	img := image.NewNRGBA(image.Rect(0, 0, 32, 32))
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		t.Fatalf("png encode: %v", err)
	}
	if _, _, _, err := decodeIconARGB(buf.Bytes(), 16); err == nil {
		t.Error("oversized icon accepted")
	}
}

func TestScaleARGB(t *testing.T) {
	// 2x1 → 1x1 keeps the sampled pixel.
	src := []uint32{0xFF0000FF, 0xFF00FF00}
	dst := scaleARGB(src, 2, 1, 1, 1)
	if len(dst) != 1 || dst[0] != 0xFF0000FF {
		t.Errorf("scale 2x1→1x1 = %x, want ff0000ff", dst[0])
	}
	// 4x4 → 2x2 samples the four corners.
	src = make([]uint32, 16)
	for i := range src {
		src[i] = uint32(i)
	}
	dst = scaleARGB(src, 4, 4, 2, 2)
	want := []uint32{0, 2, 8, 10}
	for i, v := range want {
		if dst[i] != v {
			t.Errorf("scaled pixel %d = %d, want %d", i, dst[i], v)
		}
	}
}
