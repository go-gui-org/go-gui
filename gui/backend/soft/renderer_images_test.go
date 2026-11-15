//go:build !wasm

package soft

import (
	"fmt"
	"testing"

	"github.com/go-gui-org/go-gui/gui"
)

// Image cache tests for the live Renderer (#1265). RenderToImage builds a
// renderer per capture, so its decoded-image cache died with the call; a
// Renderer keeps one for the window's life, which makes the cache's bounds
// and its staleness rules matter.

// solidPix is a w x h NRGBA buffer of one opaque color.
func solidPix(w, h int, r, g, b uint8) []byte {
	pix := make([]byte, w*h*4)
	for i := 0; i < len(pix); i += 4 {
		pix[i], pix[i+1], pix[i+2], pix[i+3] = r, g, b, 255
	}
	return pix
}

// centeredImage is a window-filling column with one 40x40 image centred in
// it, so the window's centre pixel is the image's.
func centeredImage(src string) gui.View {
	return gui.Column(gui.ContainerCfg{
		Sizing:     gui.FillFill,
		SizeBorder: gui.NoBorder,
		HAlign:     gui.HAlignCenter,
		VAlign:     gui.VAlignMiddle,
		Content: []gui.View{
			gui.Image(gui.ImageCfg{ID: "img", Src: src, Width: 40, Height: 40}),
		},
	})
}

// drawFrame settles w and draws one frame with lr, failing the test on error.
func drawFrame(t *testing.T, lr *Renderer, w *gui.Window, ww, wh int) []byte {
	t.Helper()
	settle(w)
	img, err := lr.Draw(w, ww, wh)
	if err != nil {
		t.Fatalf("Draw: %v", err)
	}
	return img.Pix
}

// TestRendererImageCacheBounded: a live window that shows a new image every
// frame (a slideshow, generated thumbnails) must not keep every decoded
// image until Destroy. The cache follows WindowCfg.MaxTextureBytes as the GL
// texture cache does (#1239): entries not drawn for two frames go once the
// total is over the cap.
func TestRendererImageCacheBounded(t *testing.T) {
	const ww, wh = 80, 80
	const side = 32
	const imgBytes = side * side * 4 // 4 KiB
	const budget = 4 * imgBytes
	const frames = 40

	src := ""
	w := gui.NewWindow(gui.WindowCfg{Width: ww, Height: wh,
		MaxTextureBytes: budget,
		OnInit: func(w *gui.Window) {
			w.SetView(func(*gui.Window) gui.View { return centeredImage(src) })
		}})
	w.Config.OnInit(w)
	lr, err := NewRenderer(w, 1)
	if err != nil {
		t.Fatalf("NewRenderer: %v", err)
	}
	t.Cleanup(lr.Release)

	for i := range frames {
		key := fmt.Sprintf("soft-cache-bound-%d", i)
		src = gui.UseImage(key, side, side, solidPix(side, side, uint8(i), 0, 0))
		t.Cleanup(func() { gui.DropImage(key) })
		drawFrame(t, lr, w, ww, wh)
	}
	// Two frames' images stay protected past the cap, so the bound is the
	// budget plus those two.
	const maxBytes = budget + 2*imgBytes
	if got := lr.r.imageCache.Bytes(); got > maxBytes {
		t.Errorf("image cache holds %d B after %d distinct images, want <= %d",
			got, frames, maxBytes)
	}
	if got := lr.r.imageCache.Len(); got >= frames {
		t.Errorf("image cache holds %d entries, every image drawn: unbounded", got)
	}
}

// TestRendererMemImageReregistered: a mem: key dropped and registered again
// with new pixels is a new image. RenderToImage never saw this (one capture,
// one registry state); a live window caching by the Src string alone would
// draw the old pixels forever. The cache keys mem: entries on the
// registration serial, as the GL backend does (#1230).
func TestRendererMemImageReregistered(t *testing.T) {
	const ww, wh = 80, 80
	const side = 8
	const key = "soft-mem-reregister"
	t.Cleanup(func() { gui.DropImage(key) })

	src := gui.UseImage(key, side, side, solidPix(side, side, 255, 0, 0))
	w := gui.NewWindow(gui.WindowCfg{Width: ww, Height: wh,
		OnInit: func(w *gui.Window) {
			w.SetView(func(*gui.Window) gui.View { return centeredImage(src) })
		}})
	w.Config.OnInit(w)
	lr, err := NewRenderer(w, 1)
	if err != nil {
		t.Fatalf("NewRenderer: %v", err)
	}
	t.Cleanup(lr.Release)

	centre := (wh/2*ww + ww/2) * 4
	pix := drawFrame(t, lr, w, ww, wh)
	if r, b := pix[centre], pix[centre+2]; r != 255 || b != 0 {
		t.Fatalf("first frame centre = %v, want red", pix[centre:centre+4])
	}

	gui.DropImage(key)
	src = gui.UseImage(key, side, side, solidPix(side, side, 0, 0, 255))
	pix = drawFrame(t, lr, w, ww, wh)
	if r, b := pix[centre], pix[centre+2]; r != 0 || b != 255 {
		t.Errorf("after re-registering %q centre = %v, want blue (stale cache)",
			key, pix[centre:centre+4])
	}
}
