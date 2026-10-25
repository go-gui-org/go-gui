//go:build js && wasm

package web

import "syscall/js"

// surfaceCanvas is a surface's offscreen canvas. Browser images draw through
// Canvas2D, so the ImageData is kept with the canvas and a new frame copies
// bytes without creating either object again.
type surfaceCanvas struct {
	canvas, ctx, data js.Value
	w, h              int
}

// uploadSurfaceCanvas puts a surface frame into current, reusing its canvas
// at a fixed size and creating a new one on a resize.
func uploadSurfaceCanvas(current surfaceCanvas, w, h int, pix []byte) surfaceCanvas {
	if current.w != w || current.h != h {
		canvas := js.Global().Get("document").Call("createElement", "canvas")
		canvas.Set("width", w)
		canvas.Set("height", h)
		ctx := canvas.Call("getContext", "2d")
		current = surfaceCanvas{canvas: canvas, ctx: ctx,
			data: ctx.Call("createImageData", w, h), w: w, h: h}
	}
	// CopyBytesToJS accepts a Uint8ClampedArray (ImageData.data).
	js.CopyBytesToJS(current.data.Get("data"), pix)
	current.ctx.Call("putImageData", current.data, 0, 0)
	return current
}
