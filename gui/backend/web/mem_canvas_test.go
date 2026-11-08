//go:build js && wasm

package web

import (
	"syscall/js"
	"testing"

	"github.com/go-gui-org/go-gui/gui"
)

// canvasFakes stands in for the DOM the mem: canvas path touches: a
// document whose canvases record the first pixel they were painted with,
// and a 2D context that records which canvas each drawImage drew.
type canvasFakes struct {
	ctx   js.Value
	drawn []js.Value // drawImage sources, in order
	funcs []js.Func
}

func (f *canvasFakes) fn(do func(a []js.Value) any) js.Func {
	jf := js.FuncOf(func(_ js.Value, a []js.Value) any { return do(a) })
	f.funcs = append(f.funcs, jf)
	return jf
}

func newCanvasFakes(t *testing.T) *canvasFakes {
	t.Helper()
	f := &canvasFakes{}
	obj := func() js.Value { return js.Global().Get("Object").New() }
	doc := obj()
	doc.Set("createElement", f.fn(func([]js.Value) any {
		canvas, ctx := obj(), obj()
		ctx.Set("createImageData", f.fn(func(a []js.Value) any {
			data := obj()
			data.Set("data", js.Global().Get("Uint8Array").New(a[0].Int()*a[1].Int()*4))
			return data
		}))
		ctx.Set("putImageData", f.fn(func(a []js.Value) any {
			canvas.Set("red", a[0].Get("data").Index(0))
			return nil
		}))
		canvas.Set("getContext", f.fn(func([]js.Value) any { return ctx }))
		return canvas
	}))
	f.ctx = obj()
	f.ctx.Set("drawImage", f.fn(func(a []js.Value) any {
		f.drawn = append(f.drawn, a[0])
		return nil
	}))
	prev := js.Global().Get("document")
	js.Global().Set("document", doc)
	t.Cleanup(func() {
		js.Global().Set("document", prev)
		for _, jf := range f.funcs {
			jf.Release()
		}
	})
	return f
}

// solidRed returns a w×h NRGBA buffer whose red channel is r.
func solidRed(w, h int, r byte) []byte {
	pix := make([]byte, w*h*4)
	for i := 0; i < len(pix); i += 4 {
		pix[i], pix[i+3] = r, 255
	}
	return pix
}

// A mem: canvas follows the registry (#1230): a drop releases it and draws
// nothing — where the canvas used to be looked up as an Image element and
// panic — and a key registered again paints a new canvas.
func TestMemCanvasFollowsRegistry(t *testing.T) {
	f := newCanvasFakes(t)
	b := &Backend{ctx2d: f.ctx, dpiScale: 1,
		imgCache: map[string]js.Value{}, failedImages: map[string]struct{}{},
		memCanvases: newMemCanvasCache()}
	const key = "webtest/1230"
	t.Cleanup(func() { gui.DropImage(key) })

	src := gui.UseImage(key, 2, 2, solidRed(2, 2, 10))
	cmds := []gui.RenderCmd{{Kind: gui.RenderImage, Resource: src,
		W: 10, H: 10, Opacity: 1}}
	// frame draws cmds and returns the canvas drawn, if any.
	frame := func() (js.Value, bool) {
		f.drawn = f.drawn[:0]
		b.drawCmds(cmds)
		if len(f.drawn) == 0 {
			return js.Value{}, false
		}
		return f.drawn[0], true
	}

	first, ok := frame()
	if !ok || first.Get("red").Int() != 10 {
		t.Fatal("first draw did not paint the registered pixels")
	}

	gui.DropImage(key)
	if _, ok = frame(); ok {
		t.Error("a dropped image was drawn")
	}
	if first.Get("width").Int() != 0 {
		t.Error("the dropped image's canvas was not released")
	}
	if b.memCanvases.Len() != 0 || len(b.imgCache) != 0 {
		t.Errorf("caches hold %d canvases, %d images after the drop, want 0, 0",
			b.memCanvases.Len(), len(b.imgCache))
	}

	gui.UseImage(key, 2, 2, solidRed(2, 2, 20))
	again, ok := frame()
	if !ok || again.Get("red").Int() != 20 {
		t.Error("the key registered again did not draw its new pixels")
	}

	// A re-registration after this frame's sweep: the draw itself sees
	// the new serial and paints a new canvas.
	b.sweepMemCanvases()
	gui.DropImage(key)
	gui.UseImage(key, 2, 2, solidRed(2, 2, 30))
	f.drawn = f.drawn[:0]
	b.drawImage(&cmds[0])
	if len(f.drawn) != 1 || f.drawn[0].Get("red").Int() != 30 {
		t.Error("re-registration between sweep and draw did not draw its new pixels")
	}
	if again.Get("width").Int() != 0 {
		t.Error("the replaced canvas was not released")
	}
}
