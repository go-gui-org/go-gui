package gui

import (
	"bytes"
	"image"
	"image/color"
	"image/gif"
	"image/jpeg"
	"image/png"
	"testing"
)

func encodeTestImage(t *testing.T, format string, width, height int) []byte {
	t.Helper()
	img := image.NewNRGBA(image.Rect(0, 0, width, height))
	for y := range height {
		for x := range width {
			i := y*img.Stride + x*4
			img.Pix[i] = byte(20 + x*30)
			img.Pix[i+1] = byte(40 + y*40)
			img.Pix[i+2] = 120
			img.Pix[i+3] = 255
		}
	}
	var data bytes.Buffer
	var err error
	switch format {
	case "png":
		err = png.Encode(&data, img)
	case "gif":
		err = gif.Encode(&data, img, nil)
	default:
		err = jpeg.Encode(&data, img, nil)
	}
	if err != nil {
		t.Fatal(err)
	}
	return data.Bytes()
}

// wantEncodedPixels is the NRGBA the registry must hold for data. PNG is
// lossless, so it is encodeTestImage's source. JPEG and GIF encoding
// changes colors, so it is the standard decoder's own result for the same
// bytes: the registry must keep exactly what the decoder produced.
func wantEncodedPixels(t *testing.T, format string, data []byte, width, height int) []byte {
	t.Helper()
	want := make([]byte, 0, width*height*4)
	if format == "png" {
		for y := range height {
			for x := range width {
				want = append(want, byte(20+x*30), byte(40+y*40), 120, 255)
			}
		}
		return want
	}
	decoded, _, err := image.Decode(bytes.NewReader(data))
	if err != nil {
		t.Fatal(err)
	}
	for y := range height {
		for x := range width {
			c := color.NRGBAModel.Convert(decoded.At(x, y)).(color.NRGBA)
			want = append(want, c.R, c.G, c.B, c.A)
		}
	}
	return want
}

func TestRegisterEncodedImageFormats(t *testing.T) {
	resetMemImages(t)
	for _, format := range []string{"png", "jpeg", "gif"} {
		t.Run(format, func(t *testing.T) {
			key := "encoded/" + format
			data := encodeTestImage(t, format, 2, 3)
			src, err := RegisterEncodedImage(key, data)
			if err != nil {
				t.Fatal(err)
			}
			if src != "mem:"+key {
				t.Fatalf("src = %q", src)
			}
			mi, ok := LookupImage(src)
			pix := mi.Pix
			if !ok || mi.W != 2 || mi.H != 3 || len(pix) != 2*3*4 {
				t.Fatalf("registered image = %dx%d len=%d ok=%t", mi.W, mi.H, len(pix), ok)
			}
			if want := wantEncodedPixels(t, format, data, 2, 3); !bytes.Equal(pix, want) {
				t.Fatalf("registered pixels = %v, want %v", pix, want)
			}
		})
	}
}

func TestRegisterEncodedImageCopiesEncodedInput(t *testing.T) {
	resetMemImages(t)
	data := encodeTestImage(t, "png", 1, 1)
	src, err := RegisterEncodedImage("encoded/ownership", data)
	if err != nil {
		t.Fatal(err)
	}
	before, ok := LookupImage(src)
	if !ok {
		t.Fatal("registered image missing")
	}
	want := append([]byte(nil), before.Pix...)
	clear(data)
	after, ok := LookupImage(src)
	if !ok || !bytes.Equal(after.Pix, want) {
		t.Fatal("registry retained or reused the encoded input")
	}
}

func TestRegisterEncodedImageRejectsInvalidInput(t *testing.T) {
	resetMemImages(t)
	if src, err := RegisterEncodedImage("", encodeTestImage(t, "png", 1, 1)); err == nil || src != "" {
		t.Fatalf("empty key: src=%q err=%v", src, err)
	}
	src, err := RegisterEncodedImage("encoded/bad", []byte("not an image"))
	if err == nil || src != "" {
		t.Fatalf("bad data: src=%q err=%v", src, err)
	}
	if HasImage("encoded/bad") {
		t.Fatal("bad image was registered")
	}
}

func TestRegisterEncodedImageRejectsRegistryOversize(t *testing.T) {
	resetMemImages(t)
	data := encodeTestImage(t, "png", maxMemImageDim+1, 1)
	if src, err := RegisterEncodedImage("encoded/wide", data); err == nil || src != "" {
		t.Fatalf("wide image: src=%q err=%v", src, err)
	}
}

// An image registered once at startup has no generator to rebuild it, so
// the budget must neither evict it nor count it against generated imagery.
func TestRegisterEncodedImageSurvivesBudgetEviction(t *testing.T) {
	resetMemImages(t)
	defer SetMemImageBudget(0)

	const px = 16 // 2x2 NRGBA
	SetMemImageBudget(2 * px)
	src, err := RegisterEncodedImage("encoded/pinned", encodeTestImage(t, "png", 2, 2))
	if err != nil {
		t.Fatal(err)
	}
	// Three generated entries against a two-entry budget: the oldest
	// generated one goes, the older pinned one stays.
	UseImage("filler/a", 2, 2, pixels(2, 2, 0xaa))
	UseImage("filler/b", 2, 2, pixels(2, 2, 0xbb))
	UseImage("filler/c", 2, 2, pixels(2, 2, 0xcc))

	if _, ok := LookupImage(src); !ok {
		t.Fatal("the budget evicted an encoded image")
	}
	if HasImage("filler/a") {
		t.Error("filler/a survived; want the least-recently-used generated entry evicted")
	}
	if !HasImage("filler/b") || !HasImage("filler/c") {
		t.Error("the encoded image counted toward the budget")
	}
}

func TestRegisterEncodedImageDropReleases(t *testing.T) {
	resetMemImages(t)
	src, err := RegisterEncodedImage("encoded/drop", encodeTestImage(t, "png", 2, 2))
	if err != nil {
		t.Fatal(err)
	}
	DropImage("encoded/drop")
	if _, ok := LookupImage(src); ok {
		t.Error("LookupImage found a dropped encoded image")
	}
	memImages.mu.Lock()
	defer memImages.mu.Unlock()
	if memImages.bytes != 0 {
		t.Errorf("budget bytes = %d after register and drop, want 0", memImages.bytes)
	}
}

// A repeat call for a registered key is the cheap path: a key names
// immutable content, so there is nothing to decode.
func TestRegisterEncodedImageRepeatSkipsDecode(t *testing.T) {
	resetMemImages(t)
	data := encodeTestImage(t, "png", 64, 64)
	src, err := RegisterEncodedImage("encoded/repeat", data)
	if err != nil {
		t.Fatal(err)
	}
	got := testing.AllocsPerRun(20, func() {
		again, againErr := RegisterEncodedImage("encoded/repeat", data)
		if againErr != nil || again != src {
			t.Fatalf("repeat: src=%q err=%v", again, againErr)
		}
	})
	if got != 0 {
		t.Errorf("repeat call allocs = %v, want 0", got)
	}
}

// A pinned entry adds nothing to the budget, so neither registering one nor
// it being the most recently used entry may evict the newest generated
// buffer, even one larger than the whole budget.
func TestRegisterEncodedImageKeepsNewestGenerated(t *testing.T) {
	resetMemImages(t)
	defer SetMemImageBudget(0)

	SetMemImageBudget(4)
	UseImage("huge", 8, 8, pixels(8, 8, 0xff))
	src, err := RegisterEncodedImage("encoded/after-huge", encodeTestImage(t, "png", 1, 1))
	if err != nil {
		t.Fatal(err)
	}
	if !HasImage("huge") {
		t.Fatal("registering a pinned image evicted the newest generated buffer")
	}
	LookupImage(src) // the pinned entry is now the most recently used
	SetMemImageBudget(4)
	if !HasImage("huge") {
		t.Error("SetMemImageBudget evicted the newest generated buffer behind a pinned one")
	}
}

// A key first registered by UseImage is pinned when RegisterEncodedImage
// claims it, and its bytes leave the budget.
func TestRegisterEncodedImagePinsExistingKey(t *testing.T) {
	resetMemImages(t)
	UseImage("encoded/shared", 2, 2, pixels(2, 2, 0x11))
	if _, err := RegisterEncodedImage("encoded/shared", encodeTestImage(t, "png", 2, 2)); err != nil {
		t.Fatal(err)
	}
	memImages.mu.Lock()
	defer memImages.mu.Unlock()
	if mi := memImages.data["encoded/shared"]; mi == nil || !mi.pinned {
		t.Fatal("existing key was not pinned")
	}
	if memImages.bytes != 0 {
		t.Errorf("budget bytes = %d, want 0 once the only entry is pinned", memImages.bytes)
	}
}
