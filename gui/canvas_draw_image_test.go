package gui

import (
	"context"
	"math"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"testing"
)

// Local image via DrawContext.Image must emit a RenderImage cmd.
func TestEmitDrawCanvasImagesLocalPath(t *testing.T) {
	w := makeWindowWithScratch()

	dir := t.TempDir()
	path := filepath.Join(dir, "x.png")
	if err := os.WriteFile(path, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}

	entries := []DrawCanvasImageEntry{{
		X: 0, Y: 0, W: 10, H: 10,
		Src: path,
	}}
	emitDrawCanvasImages(entries, 0, 0, testFullClip, w)

	found := false
	for _, r := range w.renderers {
		if r.Kind == RenderImage && r.Resource == path {
			found = true
		}
	}
	if !found {
		t.Fatal("expected RenderImage for local path")
	}
}

// In-flight http download must skip emit this frame.
func TestEmitDrawCanvasImagesHTTPInFlight(t *testing.T) {
	resetDownloadSem()
	url := "http://example.invalid/" + t.Name() + ".png"
	t.Cleanup(func() { removeCachedFor(url) })

	blocked := make(chan struct{})
	t.Cleanup(func() { close(blocked) })

	w := makeWindowWithScratch()
	w.Config = WindowCfg{
		ImageFetcher: func(
			ctx context.Context, _ string,
		) (*http.Response, error) {
			select {
			case <-blocked:
			case <-ctx.Done():
			}
			return nil, context.Canceled
		},
	}

	entries := []DrawCanvasImageEntry{{
		X: 0, Y: 0, W: 10, H: 10,
		Src: url,
	}}
	emitDrawCanvasImages(entries, 0, 0, testFullClip, w)

	for _, r := range w.renderers {
		if r.Kind == RenderImage {
			t.Fatalf("should not emit RenderImage while download "+
				"in flight; got %+v", r)
		}
	}
}

// Cached http URL resolves to the cached path and emits.
func TestEmitDrawCanvasImagesHTTPCached(t *testing.T) {
	url := "http://example.test/" + t.Name() + ".png"
	t.Cleanup(func() { removeCachedFor(url) })

	hash := hashString(url)
	cacheDir := filepath.Join(
		os.TempDir(), "gui_cache", "images")
	if err := os.MkdirAll(cacheDir, 0o755); err != nil {
		t.Fatal(err)
	}
	cached := filepath.Join(
		cacheDir, strconv.FormatUint(hash, 16)) + ".png"
	if err := os.WriteFile(cached, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}

	w := makeWindowWithScratch()
	entries := []DrawCanvasImageEntry{{
		X: 0, Y: 0, W: 10, H: 10,
		Src: url,
	}}
	emitDrawCanvasImages(entries, 0, 0, testFullClip, w)

	found := false
	for _, r := range w.renderers {
		if r.Kind == RenderImage && r.Resource == cached {
			found = true
		}
	}
	if !found {
		t.Fatalf("expected RenderImage for cached http URL; "+
			"renderers=%v", w.renderers)
	}
}

// Data URL passes through unchanged.
func TestEmitDrawCanvasImagesDataURL(t *testing.T) {
	w := makeWindowWithScratch()
	src := "data:image/png;base64,AAAA"
	entries := []DrawCanvasImageEntry{{
		X: 0, Y: 0, W: 10, H: 10,
		Src: src,
	}}
	emitDrawCanvasImages(entries, 0, 0, testFullClip, w)

	found := false
	for _, r := range w.renderers {
		if r.Kind == RenderImage && r.Resource == src {
			found = true
		}
	}
	if !found {
		t.Fatal("expected RenderImage for data URL")
	}
}

// testFullClip is a clip rect large enough to contain any test geometry, so
// emitDrawCanvasImages behaves as if the canvas did not clip.
var testFullClip = drawClip{X: -1e6, Y: -1e6, Width: 2e6, Height: 2e6}

// A clipped entry must narrow the scissor around its own image and restore
// the canvas clip afterwards, so the following entry is not clipped by it.
func TestEmitDrawCanvasImagesClipped(t *testing.T) {
	w := makeWindowWithScratch()

	dir := t.TempDir()
	path := filepath.Join(dir, "x.png")
	if err := os.WriteFile(path, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}

	entries := []DrawCanvasImageEntry{
		{
			X: 0, Y: 0, W: 100, H: 100, Src: path,
			ClipX: 10, ClipY: 20, ClipW: 30, ClipH: 40, Clipped: true,
		},
		{X: 0, Y: 0, W: 100, H: 100, Src: path},
	}
	emitDrawCanvasImages(entries, 5, 5, testFullClip, w)

	var kinds []renderKind
	var clips []RenderCmd
	for _, r := range w.renderers {
		kinds = append(kinds, r.Kind)
		if r.Kind == RenderClip {
			clips = append(clips, r)
		}
	}
	want := []renderKind{RenderClip, RenderImage, RenderClip, RenderImage}
	if len(kinds) != len(want) {
		t.Fatalf("cmd kinds = %v, want %v", kinds, want)
	}
	for i := range want {
		if kinds[i] != want[i] {
			t.Fatalf("cmd kinds = %v, want %v", kinds, want)
		}
	}
	// Origin offset applies to the clip rect exactly as it does to the image.
	if clips[0].X != 15 || clips[0].Y != 25 ||
		clips[0].W != 30 || clips[0].H != 40 {
		t.Errorf("narrowed clip = %v,%v %vx%v, want 15,25 30x40",
			clips[0].X, clips[0].Y, clips[0].W, clips[0].H)
	}
	if clips[1].W != testFullClip.Width {
		t.Errorf("restored clip width = %v, want %v",
			clips[1].W, testFullClip.Width)
	}
}

func TestIntersectClipsContained(t *testing.T) {
	a := drawClip{X: 0, Y: 0, Width: 100, Height: 100}
	b := drawClip{X: 20, Y: 30, Width: 40, Height: 50}
	got, ok := intersectClips(a, b)
	if !ok {
		t.Fatal("expect overlap")
	}
	if got != b {
		t.Errorf("intersection = %+v, want %+v (fully contained)", got, b)
	}
}

func TestIntersectClipsPartialOverlap(t *testing.T) {
	a := drawClip{X: 0, Y: 0, Width: 50, Height: 50}
	b := drawClip{X: 30, Y: 20, Width: 50, Height: 50}
	want := drawClip{X: 30, Y: 20, Width: 20, Height: 30}
	got, ok := intersectClips(a, b)
	if !ok {
		t.Fatal("expect overlap")
	}
	if got != want {
		t.Errorf("intersection = %+v, want %+v", got, want)
	}
}

func TestIntersectClipsTouchingEdgesNoOverlap(t *testing.T) {
	a := drawClip{X: 0, Y: 0, Width: 10, Height: 10}
	b := drawClip{X: 10, Y: 0, Width: 10, Height: 10}
	_, ok := intersectClips(a, b)
	if ok {
		t.Fatal("touching edges should not overlap")
	}
}

func TestIntersectClipsDegenerateB(t *testing.T) {
	a := drawClip{X: 0, Y: 0, Width: 100, Height: 100}
	tests := []struct {
		name string
		b    drawClip
	}{
		{"zero width", drawClip{X: 0, Y: 0, Width: 0, Height: 10}},
		{"zero height", drawClip{X: 0, Y: 0, Width: 10, Height: 0}},
		{"neg width", drawClip{X: 0, Y: 0, Width: -1, Height: 10}},
		{"nan x", drawClip{X: float32(math.NaN()), Y: 0, Width: 10, Height: 10}},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			_, ok := intersectClips(a, tc.b)
			if ok {
				t.Fatal("degenerate b must not overlap")
			}
		})
	}
}

func TestEmitDrawCanvasImagesConsecutiveClipped(t *testing.T) {
	w := makeWindowWithScratch()

	dir := t.TempDir()
	path := filepath.Join(dir, "x.png")
	if err := os.WriteFile(path, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}

	entries := []DrawCanvasImageEntry{
		{
			X: 0, Y: 0, W: 100, H: 100, Src: path,
			ClipX: 10, ClipY: 10, ClipW: 50, ClipH: 50, Clipped: true,
		},
		{
			X: 0, Y: 0, W: 100, H: 100, Src: path,
			ClipX: 20, ClipY: 20, ClipW: 30, ClipH: 30, Clipped: true,
		},
	}
	emitDrawCanvasImages(entries, 0, 0, testFullClip, w)

	var kinds []renderKind
	for _, r := range w.renderers {
		kinds = append(kinds, r.Kind)
	}
	// Two clipped entries, each with its own clip: [Clip, Image, Clip, Image, Clip(restore)]
	want := []renderKind{RenderClip, RenderImage, RenderClip, RenderImage, RenderClip}
	if len(kinds) != len(want) {
		t.Fatalf("cmd kinds = %v, want %v", kinds, want)
	}
	for i := range want {
		if kinds[i] != want[i] {
			t.Fatalf("cmd kinds[%d] = %v, want %v", i, kinds[i], want[i])
		}
	}
}

func TestEmitDrawCanvasImagesAllClippedRestoreAtEnd(t *testing.T) {
	w := makeWindowWithScratch()

	dir := t.TempDir()
	path := filepath.Join(dir, "x.png")
	if err := os.WriteFile(path, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}

	entries := []DrawCanvasImageEntry{
		{
			X: 0, Y: 0, W: 100, H: 100, Src: path,
			ClipX: 0, ClipY: 0, ClipW: 40, ClipH: 40, Clipped: true,
		},
	}
	emitDrawCanvasImages(entries, 0, 0, testFullClip, w)

	nClips := 0
	for _, r := range w.renderers {
		if r.Kind == RenderClip {
			nClips++
		}
	}
	if nClips != 2 {
		t.Fatalf("expected 2 clips (narrow + restore), got %d", nClips)
	}
}

// A clip rect that does not overlap the canvas clip drops the entry entirely
// rather than emitting a degenerate scissor.
func TestEmitDrawCanvasImagesClippedAway(t *testing.T) {
	w := makeWindowWithScratch()

	dir := t.TempDir()
	path := filepath.Join(dir, "x.png")
	if err := os.WriteFile(path, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}

	entries := []DrawCanvasImageEntry{{
		X: 0, Y: 0, W: 100, H: 100, Src: path,
		ClipX: 500, ClipY: 500, ClipW: 10, ClipH: 10, Clipped: true,
	}}
	emitDrawCanvasImages(entries, 0, 0,
		drawClip{X: 0, Y: 0, Width: 100, Height: 100}, w)

	for _, r := range w.renderers {
		if r.Kind == RenderImage {
			t.Fatal("image emitted despite non-overlapping clip")
		}
	}
}
