package imgdecode

import (
	"image"
	"image/color"
	"io"
	"os"
	"path/filepath"
	"testing"
)

// The tests read encoded fixtures instead of encoding images, and import
// no image format package: importing image/png, image/jpeg or image/gif
// registers that decoder for the whole test binary and would hide a
// registration missing from imgdecode. testdata/ramp-2x3.png is a 2x3
// NRGBA image whose bytes count up from 0 (png.Encode output);
// testdata/solid-2x2.jpg is a 2x2 image of (200, 100, 50) (jpeg.Encode,
// quality 100).
func readFixture(t *testing.T, name string) []byte {
	t.Helper()
	data, err := os.ReadFile(filepath.Join("testdata", name))
	if err != nil {
		t.Fatal(err)
	}
	return data
}

func TestDecodeNRGBABytes(t *testing.T) {
	t.Parallel()
	decoded, err := DecodeNRGBABytes("memory.png", readFixture(t, "ramp-2x3.png"), Limits{})
	if err != nil {
		t.Fatal(err)
	}
	if decoded.Bounds() != image.Rect(0, 0, 2, 3) || decoded.Stride != 2*4 ||
		len(decoded.Pix) != 2*3*4 {
		t.Fatalf("decoded image is not tightly packed: %+v", decoded)
	}
	for i, got := range decoded.Pix {
		if got != byte(i) {
			t.Fatalf("Pix[%d] = %d, want %d", i, got, i)
		}
	}
}

func TestDecodeNRGBABytesJPEG(t *testing.T) {
	t.Parallel()
	decoded, err := DecodeNRGBABytes("solid.jpg", readFixture(t, "solid-2x2.jpg"), Limits{})
	if err != nil {
		t.Fatal(err)
	}
	if decoded.Bounds() != image.Rect(0, 0, 2, 2) {
		t.Fatalf("bounds = %v, want 2x2", decoded.Bounds())
	}
	if got := decoded.NRGBAAt(1, 1); got != (color.NRGBA{200, 100, 50, 255}) {
		t.Errorf("pixel = %v, want {200 100 50 255}", got)
	}
}

// paddedMagic starts the encoding of the test-only "padded" format,
// whose decoder returns an NRGBA that is a sub-image of a wider buffer:
// its rows are further apart than 4·width bytes and it does not start
// at the origin. A decoder an app imports may do this.
const paddedMagic = "\x00go-gui-padded-test"

func init() {
	image.RegisterFormat("go-gui-padded-test", paddedMagic,
		func(r io.Reader) (image.Image, error) {
			if _, err := io.ReadAll(r); err != nil {
				return nil, err
			}
			wide := image.NewNRGBA(image.Rect(0, 0, 5, 4))
			for i := range wide.Pix {
				wide.Pix[i] = byte(i)
			}
			return wide.SubImage(image.Rect(1, 1, 3, 3)), nil
		},
		func(r io.Reader) (image.Config, error) {
			if _, err := io.ReadAll(r); err != nil {
				return image.Config{}, err
			}
			return image.Config{ColorModel: color.NRGBAModel, Width: 2, Height: 2}, nil
		})
}

// checkPacked asserts a tightly packed, zero-origin copy of the padded
// decoder's 2x2 sub-image.
func checkPacked(t *testing.T, decoded *image.NRGBA) {
	t.Helper()
	if decoded.Bounds() != image.Rect(0, 0, 2, 2) || decoded.Stride != 2*4 ||
		len(decoded.Pix) != 2*2*4 {
		t.Fatalf("not tightly packed: bounds %v stride %d len %d",
			decoded.Bounds(), decoded.Stride, len(decoded.Pix))
	}
	wideStride := 5 * 4
	for y := range 2 {
		for x := range 2 {
			src := (y+1)*wideStride + (x+1)*4
			if got := decoded.Pix[y*8+x*4]; got != byte(src) {
				t.Errorf("pixel (%d,%d) R = %d, want %d", x, y, got, byte(src))
			}
		}
	}
}

// The backends upload Pix as rows of exactly 4·width bytes, so both the
// file and the bytes path must pack what a decoder returns.
func TestDecodePacksPaddedNRGBA(t *testing.T) {
	t.Parallel()
	data := []byte(paddedMagic)
	fromBytes, err := DecodeNRGBABytes("padded", data, Limits{})
	if err != nil {
		t.Fatal(err)
	}
	checkPacked(t, fromBytes)

	path := filepath.Join(t.TempDir(), "padded.bin")
	if writeErr := os.WriteFile(path, data, 0o600); writeErr != nil {
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
	checkPacked(t, fromFile)
}

func TestDecodeNRGBABytesLimits(t *testing.T) {
	t.Parallel()
	data := readFixture(t, "ramp-2x3.png")
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
