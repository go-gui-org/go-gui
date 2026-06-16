package gui

import (
	"context"
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
	emitDrawCanvasImages(entries, 0, 0, w)

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
	emitDrawCanvasImages(entries, 0, 0, w)

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
	emitDrawCanvasImages(entries, 0, 0, w)

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
	emitDrawCanvasImages(entries, 0, 0, w)

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
