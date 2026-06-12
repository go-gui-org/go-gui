//go:build js && wasm

// Package web provides a Canvas2D-based backend for go-gui
// running in the browser via WebAssembly.
package web

import (
	"encoding/base64"
	"fmt"
	"log"
	"syscall/js"

	"github.com/go-gui-org/go-glyph"
	glyphweb "github.com/go-gui-org/go-glyph/backend/web"

	"github.com/go-gui-org/go-gui/gui"
	"github.com/go-gui-org/go-gui/gui/svg"
)

// Backend is the Canvas2D/WASM backend for go-gui.
type Backend struct {
	canvas    js.Value
	ctx2d     js.Value
	glyphBack *glyphweb.Backend
	textSys   *glyph.TextSystem
	shaders   *customShaderRenderer
	dpiScale  float32
	width     int
	height    int

	normBuf      []gui.GradientStop
	sampledBuf   []gui.GradientStop
	imgCache     map[string]js.Value
	failedImages map[string]struct{}
	clipDepth    int
	clipStack    []clipRegion
	lastCursor   gui.MouseCursor

	textPathPlacements []glyph.GlyphPlacement

	canvasLeft float64 // cached getBoundingClientRect().left
	canvasTop  float64 // cached getBoundingClientRect().top

	lastPasteText string
	colorBuf      []byte
	colorCache    [colorCacheSize]colorCacheEntry
	colorCacheLen int
	colorCacheIdx int
	callbacks     []js.Func // prevent GC of registered callbacks
}

type clipKind uint8

const (
	clipKindRect clipKind = iota
	clipKindStencil
)

type clipRegion struct {
	kind   clipKind
	x, y   float32
	w, h   float32
	radius float32
}

// Run initializes the web backend and runs the event/render
// loop. Blocks forever. Panics on error; call RunE for
// error-returning variant.
func Run(w *gui.Window) {
	if err := RunE(w); err != nil {
		panic(fmt.Sprintf("web: %v", err))
	}
}

// RunE initializes the web backend and runs the event/render
// loop. Blocks forever. Returns an error instead of panicking
// so embedders and tests can handle init failures.
func RunE(w *gui.Window) error {
	b, err := newBackend(w)
	if err != nil {
		return fmt.Errorf("web: %w", err)
	}
	b.run(w)
	return nil
}

func newBackend(w *gui.Window) (*Backend, error) {
	doc := js.Global().Get("document")
	canvas := doc.Call("getElementById", "go-gui-canvas")
	if canvas.IsNull() || canvas.IsUndefined() {
		const msg = "go-gui: canvas #go-gui-canvas not found"
		js.Global().Call("alert", msg)
		return nil, fmt.Errorf(msg)
	}

	canvas.Set("tabIndex", 0)

	// Compute DPI scale.
	dpr := js.Global().Get("devicePixelRatio")
	dpiScale := float32(1.0)
	if !dpr.IsUndefined() && !dpr.IsNull() {
		dpiScale = float32(dpr.Float())
	}

	// Size canvas to fill the browser viewport. Config Width/Height
	// are ignored — the browser window IS the application window.
	cssW := js.Global().Get("innerWidth").Int()
	cssH := js.Global().Get("innerHeight").Int()

	canvas.Get("style").Set("width", itoa(cssW)+"px")
	canvas.Get("style").Set("height", itoa(cssH)+"px")
	canvas.Set("width", int(float32(cssW)*dpiScale))
	canvas.Set("height", int(float32(cssH)*dpiScale))

	ctx2d := canvas.Call("getContext", "2d")

	// Scale context for HiDPI.
	if dpiScale != 1.0 {
		ctx2d.Call("scale", float64(dpiScale), float64(dpiScale))
	}

	// Initialize glyph web backend. Pass dpiScale=1 so glyph
	// works in logical (CSS) pixels. The canvas transform handles
	// scaling to physical pixels for HiDPI sharpness.
	glyphBack := glyphweb.New(canvas, 1.0)
	textSys, err := glyph.NewTextSystem(glyphBack)
	if err != nil {
		msg := "go-gui: text system init failed: " + err.Error()
		js.Global().Call("alert", msg)
		return nil, fmt.Errorf(msg)
	}

	// Load embedded icon font via JS FontFace API.
	loadIconFont(gui.IconFontData)

	b := &Backend{
		canvas:       canvas,
		ctx2d:        ctx2d,
		glyphBack:    glyphBack,
		textSys:      textSys,
		dpiScale:     dpiScale,
		width:        cssW,
		height:       cssH,
		imgCache:     make(map[string]js.Value),
		failedImages: make(map[string]struct{}),
	}
	b.shaders = newCustomShaderRenderer(doc, &b.callbacks)

	b.updateCanvasRect()

	// Inject interfaces into Window.
	w.SetTextMeasurer(&textMeasurer{textSys: textSys})
	w.SetSvgParser(svg.New())
	clipCatchFn := js.FuncOf(
		func(_ js.Value, _ []js.Value) any {
			return nil
		})
	b.callbacks = append(b.callbacks, clipCatchFn)
	w.SetClipboardFn(func(text string) {
		nav := js.Global().Get("navigator")
		cb := nav.Get("clipboard")
		if !cb.IsUndefined() && !cb.IsNull() {
			cb.Call("writeText", text).
				Call("catch", clipCatchFn)
		}
	})
	w.SetClipboardGetFn(func() string {
		return b.lastPasteText
	})
	w.SetNativePlatform(&nativePlatform{
		doc:    doc,
		canvas: canvas,
	})

	return b, nil
}

func (b *Backend) run(w *gui.Window) {
	defer w.WindowCleanup()

	if w.Config.OnInit != nil {
		w.Config.OnInit(w)
	}

	b.registerEvents(w)
	b.canvas.Call("focus")

	// Sync Window dimensions with the actual canvas size.
	// NewWindow sets windowWidth/Height from Config, which may
	// differ from the browser viewport.
	w.EventFn(&gui.Event{
		Type:         gui.EventResized,
		WindowWidth:  b.width,
		WindowHeight: b.height,
	})

	// requestAnimationFrame render loop.
	var renderFunc js.Func
	renderFunc = js.FuncOf(func(_ js.Value, _ []js.Value) any {
		w.FrameFn()
		b.renderFrame(w)

		// Update cursor.
		mc := w.MouseCursorState()
		if mc != b.lastCursor {
			b.lastCursor = mc
			css, ok := cursorCSS[mc]
			if !ok {
				css = "default"
			}
			b.canvas.Get("style").Set("cursor", css)
		}

		js.Global().Call("requestAnimationFrame", renderFunc)
		return nil
	})
	b.callbacks = append(b.callbacks, renderFunc)
	js.Global().Call("requestAnimationFrame", renderFunc)

	// Block forever.
	select {}
}

func (b *Backend) renderFrame(w *gui.Window) {
	// Zero Color (transparent black) falls through to theme
	// default — not distinguishable from unset.
	bg := w.Config.BgColor
	if bg == (gui.Color{}) {
		t := gui.CurrentTheme()
		bg = t.ColorBackground
	}
	b.glyphBack.BeginFrame(
		float32(bg.R)/255, float32(bg.G)/255,
		float32(bg.B)/255, float32(bg.A)/255)

	// BeginFrame resets the canvas transform to identity.
	// Re-apply DPI scale so all drawing uses logical coordinates
	// that map to physical pixels via the transform.
	if b.dpiScale != 1.0 {
		b.ctx2d.Call("setTransform",
			float64(b.dpiScale), 0, 0,
			float64(b.dpiScale), 0, 0)
	}

	// Reset clip depth.
	for b.clipDepth > 0 {
		b.ctx2d.Call("restore")
		b.clipDepth--
	}
	b.clipStack = nil

	w.Lock()
	w.BackingScale = b.dpiScale
	b.renderersDraw(w)
	w.Unlock()

	b.textSys.Commit()
	b.glyphBack.EndFrame()
}

// updateCanvasRect caches the canvas bounding rect to avoid
// a DOM layout query on every touch event.
func (b *Backend) updateCanvasRect() {
	rect := b.canvas.Call("getBoundingClientRect")
	b.canvasLeft = rect.Get("left").Float()
	b.canvasTop = rect.Get("top").Float()
}

func (b *Backend) resizeCanvas(cssW, cssH int) {
	// Re-read devicePixelRatio — it may change when the window
	// moves between displays with different DPI.
	dpr := js.Global().Get("devicePixelRatio")
	if !dpr.IsUndefined() && !dpr.IsNull() {
		b.dpiScale = float32(dpr.Float())
	}
	b.width = cssW
	b.height = cssH
	b.canvas.Get("style").Set("width", itoa(cssW)+"px")
	b.canvas.Get("style").Set("height", itoa(cssH)+"px")
	physW := int(float32(cssW) * b.dpiScale)
	physH := int(float32(cssH) * b.dpiScale)
	b.canvas.Set("width", physW)
	b.canvas.Set("height", physH)
	// Re-apply DPI scale after resize resets transform.
	if b.dpiScale != 1.0 {
		b.ctx2d.Call("setTransform",
			float64(b.dpiScale), 0, 0,
			float64(b.dpiScale), 0, 0)
	}
	b.updateCanvasRect()
}

// loadIconFont loads the embedded icon font via the JS FontFace
// API. Converts TTF bytes to a base64 data URL.
func loadIconFont(data []byte) {
	if len(data) == 0 {
		return
	}
	b64 := base64.StdEncoding.EncodeToString(data)
	src := "url(data:font/truetype;base64," + b64 + ")"

	ff := js.Global().Get("FontFace").New("feathericon", src)
	promise := ff.Call("load")
	var thenFn, catchFn js.Func
	thenFn = js.FuncOf(func(_ js.Value, _ []js.Value) any {
		js.Global().Get("document").Get("fonts").Call("add", ff)
		thenFn.Release()
		catchFn.Release()
		return nil
	})
	catchFn = js.FuncOf(func(_ js.Value, args []js.Value) any {
		log.Printf("web: icon font load failed: %v",
			args[0].String())
		thenFn.Release()
		catchFn.Release()
		return nil
	})
	promise.Call("then", thenFn)
	promise.Call("catch", catchFn)
}
