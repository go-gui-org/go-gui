//go:build js && wasm

package web

import (
	"io"
	"log"
	"os"
	"syscall/js"
	"testing"
)

// needsPaint paints on a gui change or a standing request, and not
// otherwise: an idle frame must cost nothing.
func TestNeedsPaint(t *testing.T) {
	var b Backend
	if b.needsPaint(false) {
		t.Fatal("idle frame painted")
	}
	if !b.needsPaint(true) {
		t.Fatal("changed frame not painted")
	}
	b.requestRedraw()
	if !b.needsPaint(false) {
		t.Fatal("pending request not painted")
	}
}

// Each browser event the gui package cannot see must leave a repaint
// request, or the canvas stays stale until the next input.
func TestRedrawSources(t *testing.T) {
	g := js.Global()
	fonts := g.Get("EventTarget").New()
	doc := g.Get("Object").New()
	doc.Set("fonts", fonts)
	canvas := g.Get("EventTarget").New()

	var b Backend
	b.watchRedrawSources(doc, canvas)
	t.Cleanup(func() {
		for _, f := range b.callbacks {
			f.Release()
		}
	})

	fire := func(name string, target js.Value, event string) {
		t.Helper()
		b.redrawPending = false
		target.Call("dispatchEvent", g.Get("Event").New(event))
		if !b.redrawPending {
			t.Errorf("%s did not request a repaint", name)
		}
	}
	fire("font load", fonts, "loadingdone")
	fire("canvas restore", canvas, "contextrestored")

	b.redrawPending = false
	b.imgSettled.Invoke()
	if !b.redrawPending {
		t.Error("image load did not request a repaint")
	}
}

// A document without document.fonts (old browsers) must not break
// startup.
func TestRedrawSourcesNoFonts(t *testing.T) {
	var b Backend
	b.watchRedrawSources(js.Global().Get("Object").New(),
		js.Global().Get("EventTarget").New())
	for _, f := range b.callbacks {
		f.Release()
	}
}

// Setting the canvas size clears it, and the resize event that
// follows can be dropped, so resizeCanvas must request the repaint.
func TestResizeCanvasRequestsRedraw(t *testing.T) {
	g := js.Global()
	canvas := g.Get("Object").New()
	canvas.Set("style", g.Get("Object").New())
	rect := js.FuncOf(func(js.Value, []js.Value) any {
		return map[string]any{"left": 0, "top": 0}
	})
	t.Cleanup(rect.Release)
	canvas.Set("getBoundingClientRect", rect)

	b := Backend{canvas: canvas, dpiScale: 1}
	b.resizeCanvas(640, 480)
	if !b.redrawPending {
		t.Fatal("resize did not request a repaint")
	}
	if got := canvas.Get("width").Int(); got != 640 {
		t.Fatalf("canvas width = %d, want 640", got)
	}
}

// While the WebGL context is lost the shader area draws the solid
// fallback. Its return must request a repaint, or an idle page keeps
// the fallback on screen.
func TestShaderContextRestoredRequestsRedraw(t *testing.T) {
	g := js.Global()
	// Stand-in GL whose context is still lost, so the restore fails
	// the way a real failed restore does. The repaint is needed
	// either way — a failed restore still has to draw the fallback.
	lostFn := js.FuncOf(func(js.Value, []js.Value) any { return true })
	t.Cleanup(lostFn.Release)
	gl := g.Get("Object").New()
	gl.Set("isContextLost", lostFn)

	// The failed restore logs. Under node, a stderr write from inside
	// a JS event callback waits on the event loop that dispatchEvent
	// is holding, and the test hangs. Browsers write synchronously.
	log.SetOutput(io.Discard)
	t.Cleanup(func() { log.SetOutput(os.Stderr) })

	var b Backend
	r := &customShaderRenderer{
		canvas: g.Get("EventTarget").New(),
		gl:     gl,
		lost:   true,
	}
	r.registerContextCallbacks(&b.callbacks, b.requestRedraw)
	t.Cleanup(func() {
		for _, f := range b.callbacks {
			f.Release()
		}
	})

	r.canvas.Call("dispatchEvent", g.Get("Event").New("webglcontextrestored"))
	if !b.redrawPending {
		t.Fatal("WebGL context restore did not request a repaint")
	}
}

// A repaint asked for while a paint runs must survive it. Clearing the
// request after the paint returned wiped any request made during it.
func TestRedrawRequestedDuringPaintSurvives(t *testing.T) {
	b := &Backend{}
	painted := b.paintIfNeeded(true, b.requestRedraw)
	if !painted {
		t.Fatal("changed frame did not paint")
	}
	if !b.redrawPending {
		t.Fatal("repaint requested during the paint was lost")
	}
	// The standing request paints the next idle frame, then clears.
	calls := 0
	b.paintIfNeeded(false, func() { calls++ })
	if calls != 1 || b.redrawPending {
		t.Fatalf("idle frame: calls=%d pending=%v, want 1, false",
			calls, b.redrawPending)
	}
	if b.paintIfNeeded(false, func() { calls++ }); calls != 1 {
		t.Fatal("idle frame with no request painted")
	}
}
