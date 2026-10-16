package gui

import (
	"os"
	"path/filepath"
	"testing"
)

// The zero value of a DrawContext samples images linearly, so code that
// never touches the filter draws exactly as it did before #1065.
func TestImageFilterDefaultsToLinear(t *testing.T) {
	var dc DrawContext
	if got := dc.ImageFilter(); got != ImageFilterLinear {
		t.Fatalf("ImageFilter() = %v, want ImageFilterLinear", got)
	}
	dc.Image(0, 0, 10, 10, "a.png", Opt[float32]{}, Color{})
	if got := dc.images[0].filter; got != ImageFilterLinear {
		t.Fatalf("entry filter = %v, want ImageFilterLinear", got)
	}
}

// Every image entry point stamps the filter in force when it is called.
func TestImageFilterStampsEveryEntryPoint(t *testing.T) {
	var dc DrawContext
	dc.SetImageFilter(ImageFilterNearest)
	dc.Image(0, 0, 10, 10, "a.png", Opt[float32]{}, Color{})
	dc.ImageWithFetcher(0, 0, 10, 10, "b.png", Opt[float32]{}, Color{}, nil)
	dc.ImageClipped(0, 0, 10, 10, "c.png", Opt[float32]{}, Color{}, 0, 0, 5, 5)
	// A rotated image takes the bake path, which builds its own entry.
	dc.Rotate(0.5)
	dc.Image(0, 0, 10, 10, "d.png", Opt[float32]{}, Color{})
	if len(dc.images) != 4 {
		t.Fatalf("got %d entries, want 4", len(dc.images))
	}
	for i, e := range dc.images {
		if e.filter != ImageFilterNearest {
			t.Errorf("entry %d (%s) filter = %v, want ImageFilterNearest",
				i, e.Src, e.filter)
		}
	}
}

// Save/Restore scopes the filter the same way it scopes the transform.
func TestImageFilterSaveRestore(t *testing.T) {
	var dc DrawContext
	dc.Save()
	dc.SetImageFilter(ImageFilterNearest)
	dc.Save()
	dc.SetImageFilter(ImageFilterLinear)
	dc.Restore()
	if got := dc.ImageFilter(); got != ImageFilterNearest {
		t.Fatalf("after inner Restore = %v, want ImageFilterNearest", got)
	}
	dc.Restore()
	if got := dc.ImageFilter(); got != ImageFilterLinear {
		t.Fatalf("after outer Restore = %v, want ImageFilterLinear", got)
	}
}

// Restores that match pushes dropped by the depth cap must not change
// the filter: the same balance rule the transform follows.
func TestImageFilterRestorePastDepthCap(t *testing.T) {
	var dc DrawContext
	for range maxXformDepth - 1 {
		dc.Save()
	}
	// The last stored push saves Nearest, so the stored Restore below
	// must change the filter: a Restore that skips it would fail.
	dc.SetImageFilter(ImageFilterNearest)
	dc.Save() // stored: fills the stack to the cap
	dc.Save() // dropped: over the cap
	dc.SetImageFilter(ImageFilterLinear)
	dc.Restore() // matches the dropped push: no-op
	if got := dc.ImageFilter(); got != ImageFilterLinear {
		t.Fatalf("no-op Restore changed filter to %v", got)
	}
	dc.Restore() // pops the last stored push, saved while Nearest
	if got := dc.ImageFilter(); got != ImageFilterNearest {
		t.Fatalf("stored Restore = %v, want ImageFilterNearest", got)
	}
}

// The window shares one DrawContext between canvases. A filter left set
// by one OnDraw must not leak into the next.
func TestImageFilterResetFor(t *testing.T) {
	var dc DrawContext
	dc.SetImageFilter(ImageFilterNearest)
	dc.Save()
	dc.resetFor(10, 10, 1, nil, drawCanvasCache{})
	if got := dc.ImageFilter(); got != ImageFilterLinear {
		t.Fatalf("resetFor left filter %v", got)
	}
}

// An unknown value reads back as given but draws as linear, so a bad
// cast cannot reach a backend as an undefined mode.
func TestImageFilterUnknownValueDrawsLinear(t *testing.T) {
	var dc DrawContext
	dc.SetImageFilter(ImageFilter(200))
	dc.Image(0, 0, 10, 10, "a.png", Opt[float32]{}, Color{})
	if got := dc.images[0].filter; got != ImageFilterLinear {
		t.Fatalf("entry filter = %v, want ImageFilterLinear", got)
	}
}

// The emitted RenderImage carries the entry's filter to the backend.
func TestEmitDrawCanvasImagesCarriesFilter(t *testing.T) {
	w := makeWindowWithScratch()
	dir := t.TempDir()
	path := filepath.Join(dir, "x.png")
	if err := os.WriteFile(path, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	entries := []DrawCanvasImageEntry{
		{X: 0, Y: 0, W: 10, H: 10, Src: path, filter: ImageFilterNearest},
		{X: 0, Y: 0, W: 10, H: 10, Src: path},
	}
	emitDrawCanvasImages(entries, 0, 0, testFullClip, &Shape{Opacity: 1}, w)
	var got []ImageFilter
	for _, r := range w.renderers {
		if r.Kind == RenderImage {
			got = append(got, r.ImageFilter)
		}
	}
	if len(got) != 2 || got[0] != ImageFilterNearest || got[1] != ImageFilterLinear {
		t.Fatalf("emitted filters = %v, want [Nearest Linear]", got)
	}
}
