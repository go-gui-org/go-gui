package imgdecode

import (
	"bytes"
	"image"
	"image/color"
	"image/png"
	"os"
	"path/filepath"
	"testing"
)

func encodedPNG(t *testing.T, width, height int) []byte {
	t.Helper()
	img := image.NewNRGBA(image.Rect(0, 0, width, height))
	for i := range img.Pix {
		img.Pix[i] = byte(i)
	}
	var data bytes.Buffer
	if err := png.Encode(&data, img); err != nil {
		t.Fatal(err)
	}
	return data.Bytes()
}

func TestDecodeNRGBABytes(t *testing.T) {
	t.Parallel()
	decoded, err := DecodeNRGBABytes("memory.png", encodedPNG(t, 2, 3), Limits{})
	if err != nil {
		t.Fatal(err)
	}
	if decoded.Bounds() != image.Rect(0, 0, 2, 3) || decoded.Stride != 2*4 ||
		len(decoded.Pix) != 2*3*4 {
		t.Fatalf("decoded image is not tightly packed: %+v", decoded)
	}
}

func TestDecodeNRGBABytesLimits(t *testing.T) {
	t.Parallel()
	data := encodedPNG(t, 2, 3)
	tests := []struct {
		name   string
		limits Limits
	}{
		{"encoded bytes", Limits{MaxBytes: int64(len(data) - 1)}},
		{"pixels", Limits{MaxPixels: 5}},
		{"width", Limits{MaxWidth: 1}},
		{"height", Limits{MaxHeight: 2}},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			if _, err := DecodeNRGBABytes("memory.png", data, test.limits); err == nil {
				t.Fatal("DecodeNRGBABytes returned nil error")
			}
		})
	}
}

// animatedGIF is a 2x2 GIF with two frames: solid red, then solid blue
// (gif.EncodeAll output). It is a literal, not encoded in the test,
// because importing image/gif here would register the GIF decoder itself
// and hide a missing registration in imgdecode.
var animatedGIF = []byte{
	0x47, 0x49, 0x46, 0x38, 0x39, 0x61, 0x02, 0x00, 0x02, 0x00, 0x00, 0x00,
	0x00, 0x21, 0xff, 0x0b, 0x4e, 0x45, 0x54, 0x53, 0x43, 0x41, 0x50, 0x45,
	0x32, 0x2e, 0x30, 0x03, 0x01, 0x00, 0x00, 0x00, 0x21, 0xf9, 0x04, 0x00,
	0x0a, 0x00, 0x00, 0x00, 0x2c, 0x00, 0x00, 0x00, 0x00, 0x02, 0x00, 0x02,
	0x00, 0x80, 0xff, 0x00, 0x00, 0x00, 0x00, 0xff, 0x02, 0x02, 0x84, 0x51,
	0x00, 0x21, 0xf9, 0x04, 0x00, 0x0a, 0x00, 0x00, 0x00, 0x2c, 0x00, 0x00,
	0x00, 0x00, 0x02, 0x00, 0x02, 0x00, 0x80, 0xff, 0x00, 0x00, 0x00, 0x00,
	0xff, 0x02, 0x02, 0x8c, 0x53, 0x00, 0x3b,
}

// GIF is a supported format in its own right, not through another
// package's decoder import. An animated GIF decodes to its first frame.
func TestDecodeNRGBABytesGIFFirstFrame(t *testing.T) {
	t.Parallel()
	decoded, err := DecodeNRGBABytes("anim.gif", animatedGIF, Limits{})
	if err != nil {
		t.Fatal(err)
	}
	if decoded.Bounds() != image.Rect(0, 0, 2, 2) {
		t.Fatalf("bounds = %v, want 2x2", decoded.Bounds())
	}
	if got := decoded.NRGBAAt(0, 0); got != (color.NRGBA{255, 0, 0, 255}) {
		t.Errorf("pixel = %v, want the first frame's red", got)
	}
}

// offsetFrameGIF is a 4x3 GIF canvas whose only frame is a 2x1 red strip
// at (1,1) (gif.EncodeAll output). A literal for the same reason as
// animatedGIF.
var offsetFrameGIF = []byte{
	0x47, 0x49, 0x46, 0x38, 0x39, 0x61, 0x04, 0x00, 0x03, 0x00, 0x80, 0x00,
	0x00, 0xff, 0x00, 0x00, 0x00, 0x00, 0xff, 0x2c, 0x01, 0x00, 0x01, 0x00,
	0x02, 0x00, 0x01, 0x00, 0x00, 0x02, 0x02, 0x04, 0x0a, 0x00, 0x3b,
}

// checkOffsetFrame asserts the frame sits at its offset on the full,
// otherwise transparent canvas.
func checkOffsetFrame(t *testing.T, decoded *image.NRGBA) {
	t.Helper()
	if decoded.Bounds() != image.Rect(0, 0, 4, 3) {
		t.Fatalf("bounds = %v, want the 4x3 canvas", decoded.Bounds())
	}
	red := color.NRGBA{255, 0, 0, 255}
	for y := range 3 {
		for x := range 4 {
			want := color.NRGBA{}
			if y == 1 && (x == 1 || x == 2) {
				want = red
			}
			if got := decoded.NRGBAAt(x, y); got != want {
				t.Errorf("pixel (%d,%d) = %v, want %v", x, y, got, want)
			}
		}
	}
}

// A GIF frame that covers part of its canvas decodes to the full canvas,
// not to the frame moved to the origin, from bytes and from a file.
func TestDecodeGIFFrameOnCanvas(t *testing.T) {
	t.Parallel()
	decoded, err := DecodeNRGBABytes("offset.gif", offsetFrameGIF, Limits{})
	if err != nil {
		t.Fatal(err)
	}
	checkOffsetFrame(t, decoded)

	path := filepath.Join(t.TempDir(), "offset.gif")
	if writeErr := os.WriteFile(path, offsetFrameGIF, 0o600); writeErr != nil {
		t.Fatal(writeErr)
	}
	f, err := os.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = f.Close() }()
	fromFile, err := DecodeNRGBA(path, f, 0, 0)
	if err != nil {
		t.Fatal(err)
	}
	checkOffsetFrame(t, fromFile)
}

func TestDecodeNRGBABytesRejectsInvalidData(t *testing.T) {
	t.Parallel()
	if _, err := DecodeNRGBABytes("bad.png", []byte("not an image"), Limits{}); err == nil {
		t.Fatal("DecodeNRGBABytes returned nil error")
	}
}
