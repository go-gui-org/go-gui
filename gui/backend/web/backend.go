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
	"github.com/go-gui-org/go-gui/gui/backend/internal/devscale"
	"github.com/go-gui-org/go-gui/gui/svg"
)

// Backend is the Canvas2D/WASM backend for go-gui.
type Backend struct {
	// win is the one window this backend serves — the browser window
	// IS the application window here. Held so draw paths that resolve
	// a theme default (custom-shader fallback) can name the window
	// instead of reading the frame-scoped theme cache.
	win       *gui.Window
	canvas    js.Value
	ctx2d     js.Value
	glyphBack *glyphweb.Backend
	textSys   *glyph.TextSystem
	shaders   *customShaderRenderer
	dpiScale  float32
	width     int

	// textErrLogged warns once for a persistent DrawText failure
	// instead of spamming stderr every frame.
	textErrLogged bool
	height        int

	normBuf      []gui.GradientStop
	imgCache     map[string]js.Value
	failedImages map[string]struct{}
	clipDepth    int
	clipStack    []clipRegion
	lastCursor   gui.MouseCursor

	textPathPlacements []glyph.GlyphPlacement

	// tris is the shared vertex buffer and JS path helper for
	// triangle meshes (draw_tris.go).
	tris triMesh

	// redrawPending asks the next animation frame to paint even when
	// FrameFn reports no change. The loop skips idle frames, so any
	// change the gui package cannot see — an image that finished
	// loading, a font arriving, a restored canvas — must call
	// requestRedraw or it stays off screen until the next input.
	//
	// Animated images (GIF, APNG, SMIL SVG in an <img>) show one
	// frame and do not animate: go-gui does not support them on any
	// backend, and repainting every frame just in case would bring
	// back the idle cost this gate removes.
	redrawPending bool
	// imgSettled is the shared onload/onerror handler for Image
	// elements; it sets redrawPending.
	imgSettled js.Func

	canvasLeft float64 // cached getBoundingClientRect().left
	canvasTop  float64 // cached getBoundingClientRect().top

	lastPasteText string
	colorBuf      []byte
	colorCache    [colorCacheSize]colorCacheEntry
	colorCacheLen int
	colorCacheIdx int
	callbacks     []js.Func // prevent GC of registered callbacks

	hasRoundRect bool // Canvas2D roundRect support (missing on Safari <15.4)
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
		return nil, fmt.Errorf("%s", msg)
	}

	canvas.Set("tabIndex", 0)

	// Compute DPI scale.
	dpr := js.Global().Get("devicePixelRatio")
	dpiScale := float32(1.0)
	if !dpr.IsUndefined() && !dpr.IsNull() {
		dpiScale = float32(dpr.Float())
	}
	// GOGUI_DEVICE_SCALE replaces devicePixelRatio (#971). The canvas
	// keeps its CSS size, so the browser resamples it like a display of
	// that density.
	dpiScale = devscale.Apply(dpiScale)

	// Size canvas to fill the browser viewport. Config Width/Height
	// are ignored — the browser window IS the application window.
	cssW := js.Global().Get("innerWidth").Int()
	cssH := js.Global().Get("innerHeight").Int()
	// Guard against iPadOS Safari returning 0 during split-screen
	// transitions or on-screen keyboard appearance.
	if cssW <= 0 {
		cssW = 800
	}
	if cssH <= 0 {
		cssH = 600
	}

	canvas.Get("style").Set("width", itoa(cssW)+"px")
	canvas.Get("style").Set("height", itoa(cssH)+"px")
	canvas.Set("width", int(float32(cssW)*dpiScale))
	canvas.Set("height", int(float32(cssH)*dpiScale))

	ctx2d := canvas.Call("getContext", "2d")

	// Detect Canvas2D roundRect support. Safari added it in 15.4
	// (March 2022); older versions need an arcTo fallback.
	hasRR := ctx2d.Get("roundRect").Type() == js.TypeFunction

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
		return nil, fmt.Errorf("%s", msg)
	}

	b := &Backend{
		win:          w,
		canvas:       canvas,
		ctx2d:        ctx2d,
		glyphBack:    glyphBack,
		textSys:      textSys,
		dpiScale:     dpiScale,
		width:        cssW,
		height:       cssH,
		imgCache:     make(map[string]js.Value),
		failedImages: make(map[string]struct{}),
		hasRoundRect: hasRR,
	}
	b.shaders = newCustomShaderRenderer(doc, &b.callbacks, b.requestRedraw)
	b.tris.fill = newTriPathFn()
	b.watchRedrawSources(doc, canvas)

	// Load embedded icon font via JS FontFace API. Text drawn before
	// it arrives used a fallback face, so repaint once it lands.
	loadIconFont(gui.IconFontData, b.requestRedraw)

	b.updateCanvasRect()

	// Cmd or Ctrl shortcuts, from the OS the browser runs on (#969).
	setKeyBinding()

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

	// requestAnimationFrame runs at the display rate, and the browser
	// composites the canvas no faster, so VSyncOff cannot be honored.
	if w.Config.VSyncOff {
		w.DebugWindowVSync("the web backend paces frames with requestAnimationFrame")
	}

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
		defer func() {
			if r := recover(); r != nil {
				log.Printf("web: render panic: %v", r)
				// The canvas may be half drawn. Paint it again on
				// the next frame instead of leaving it on screen.
				b.requestRedraw()
				// Keep the loop alive so a transient error
				// doesn't kill the entire WASM instance.
				js.Global().Call("requestAnimationFrame",
					renderFunc)
			}
		}()
		// Paint only when something changed. The canvas keeps its
		// pixels between frames, so an idle frame costs nothing
		// instead of a full Canvas2D redraw. Native backends gate
		// renderFrame on FrameFn the same way.
		b.paintIfNeeded(w.FrameFn(), func() { b.renderFrame(w) })

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
	bg := w.FrameBackground()
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

// watchRedrawSources hooks the browser events that change what the
// canvas should show without the gui package knowing, so the idle
// frame gate still repaints for them.
func (b *Backend) watchRedrawSources(doc, canvas js.Value) {
	// Shared onload/onerror handler for Image elements.
	b.imgSettled = js.FuncOf(func(_ js.Value, _ []js.Value) any {
		b.requestRedraw()
		return nil
	})
	b.callbacks = append(b.callbacks, b.imgSettled)

	// A face the page declares with CSS @font-face can arrive after
	// the first paint. Repaint when any font load finishes.
	b.onEvent(doc.Get("fonts"), "loadingdone", b.requestRedraw)

	// Chrome may drop a 2D canvas's backing store while the tab is
	// in the background and restore it blank. Repaint when it does.
	b.onEvent(canvas, "contextrestored", b.requestRedraw)
}

// requestRedraw makes the next animation frame paint even if
// FrameFn reports no change.
func (b *Backend) requestRedraw() { b.redrawPending = true }

// needsPaint reports whether this animation frame must paint: the
// gui package changed the frame, or something outside it asked.
func (b *Backend) needsPaint(changed bool) bool {
	return changed || b.redrawPending
}

// paintIfNeeded runs paint when needsPaint says so, and reports whether
// it did. The request is cleared BEFORE the paint: a requestRedraw made
// while paint runs is for a later frame and must stand. A panic inside
// paint loses nothing, because the render loop's recover asks again.
func (b *Backend) paintIfNeeded(changed bool, paint func()) bool {
	if !b.needsPaint(changed) {
		return false
	}
	b.redrawPending = false
	paint()
	return true
}

// onEvent registers fn for a DOM event on target and keeps the
// callback alive for the life of the backend. A missing target (no
// document.fonts on an old browser) is skipped.
func (b *Backend) onEvent(target js.Value, name string, fn func()) {
	if target.Type() != js.TypeObject {
		return
	}
	cb := js.FuncOf(func(_ js.Value, _ []js.Value) any {
		fn()
		return nil
	})
	b.callbacks = append(b.callbacks, cb)
	target.Call("addEventListener", name, cb)
}

// updateCanvasRect caches the canvas bounding rect to avoid
// a DOM layout query on every touch event.
func (b *Backend) updateCanvasRect() {
	rect := b.canvas.Call("getBoundingClientRect")
	b.canvasLeft = rect.Get("left").Float()
	b.canvasTop = rect.Get("top").Float()
}

func (b *Backend) resizeCanvas(cssW, cssH int) {
	// Guard against transient 0-dimension reports on iPadOS
	// Safari (split-screen transitions, keyboard appearance).
	if cssW <= 0 || cssH <= 0 {
		return
	}
	// Re-read devicePixelRatio — it may change when the window
	// moves between displays with different DPI.
	dpr := js.Global().Get("devicePixelRatio")
	if !dpr.IsUndefined() && !dpr.IsNull() {
		b.dpiScale = float32(dpr.Float())
	}
	b.dpiScale = devscale.Apply(b.dpiScale)
	b.width = cssW
	b.height = cssH
	b.canvas.Get("style").Set("width", itoa(cssW)+"px")
	b.canvas.Get("style").Set("height", itoa(cssH)+"px")
	physW := int(float32(cssW) * b.dpiScale)
	physH := int(float32(cssH) * b.dpiScale)
	b.canvas.Set("width", physW)
	b.canvas.Set("height", physH)
	// Setting the size clears the canvas. The resize event that
	// follows can be dropped (unfocused or frozen window), so ask
	// for the repaint here.
	b.requestRedraw()
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
func loadIconFont(data []byte, onLoaded func()) {
	if len(data) == 0 {
		return
	}
	b64 := base64.StdEncoding.EncodeToString(data)
	src := "url(data:font/truetype;base64," + b64 + ")"

	ff := js.Global().Get("FontFace").New(gui.IconFontName, src)
	promise := ff.Call("load")
	var thenFn, catchFn js.Func
	thenFn = js.FuncOf(func(_ js.Value, _ []js.Value) any {
		js.Global().Get("document").Get("fonts").Call("add", ff)
		onLoaded()
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
