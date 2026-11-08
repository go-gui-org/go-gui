//go:build js && wasm

package web

import (
	"syscall/js"

	"github.com/go-gui-org/go-gui/gui/backend/internal/memtex"
	"github.com/go-gui-org/go-gui/gui/backend/internal/texcache"
)

// memCanvas is an offscreen canvas painted from a mem: image, with the
// registry serial of the pixels it holds.
type memCanvas struct {
	canvas js.Value
	serial uint64
}

// newMemCanvasCache bounds the mem: canvases like the Image elements. A
// canvas the cache lets go is shrunk to 0×0, which releases its bitmap at
// once instead of when both Go and the browser have collected it.
func newMemCanvasCache() texcache.Cache[string, memCanvas] {
	return texcache.New[string, memCanvas](maxImageCacheSize, releaseMemCanvas)
}

func releaseMemCanvas(c memCanvas) {
	c.canvas.Set("width", 0)
	c.canvas.Set("height", 0)
}

// keepMemCanvas is the canvas cache's sweep predicate. A plain function,
// not a closure, so the sweep allocates nothing.
func keepMemCanvas(key string, c memCanvas) bool {
	return memtex.Current(key, c.serial)
}

// sweepMemCanvases releases the canvases of mem: images that left gui's
// registry — dropped, evicted or registered again — since the last sweep.
// A frame with no removal costs one atomic load (#1230).
func (b *Backend) sweepMemCanvases() {
	if b.memSweep.Due() {
		b.memCanvases.Sweep(keepMemCanvas)
	}
}

// memImageCanvas paints an NRGBA8 buffer into an offscreen canvas.
// ImageData is non-premultiplied RGBA, the same layout the registry
// stores, so the bytes transfer with no conversion.
func (b *Backend) memImageCanvas(
	w, h int, pix []byte,
) js.Value {
	canvas := js.Global().Get("document").
		Call("createElement", "canvas")
	canvas.Set("width", w)
	canvas.Set("height", h)
	ctx := canvas.Call("getContext", "2d")

	buf := js.Global().Get("Uint8Array").New(len(pix))
	js.CopyBytesToJS(buf, pix)
	data := ctx.Call("createImageData", w, h)
	data.Get("data").Call("set", buf)
	ctx.Call("putImageData", data, 0, 0)
	return canvas
}
