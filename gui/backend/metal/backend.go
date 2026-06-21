//go:build darwin && !ios

// Package metal provides a native Metal backend for go-gui on macOS.
// Uses NSWindow + CAMetalLayer for windowing and Metal for GPU rendering.
// Zero SDL dependency — system frameworks only.
package metal

/*
#cgo CFLAGS: -fobjc-arc
#cgo LDFLAGS: -framework Metal -framework QuartzCore -framework AppKit -framework Foundation

#include <stdlib.h>
#include "metal_darwin.h"
#include "metal_window.h"
*/
import "C"
import (
	"errors"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"runtime"
	"unsafe"

	"github.com/go-gui-org/go-glyph"

	"github.com/go-gui-org/go-gui/gui"
	"github.com/go-gui-org/go-gui/gui/backend/internal/gpu"
	"github.com/go-gui-org/go-gui/gui/backend/internal/imgpath"
	"github.com/go-gui-org/go-gui/gui/backend/internal/tempfont"
	"github.com/go-gui-org/go-gui/gui/backend/internal/texcache"
	"github.com/go-gui-org/go-gui/gui/svg"
)

// Pipeline IDs matching the C enum.
const (
	pipeSolid       = C.PIPE_SOLID
	pipeShadow      = C.PIPE_SHADOW
	pipeBlur        = C.PIPE_BLUR
	pipeGradient    = C.PIPE_GRADIENT
	pipeImageClip   = C.PIPE_IMAGE_CLIP
	pipeFilterBlurH = C.PIPE_FILTER_BLUR_H
	pipeFilterBlurV = C.PIPE_FILTER_BLUR_V
	pipeFilterTex   = C.PIPE_FILTER_TEX
	pipeFilterColor = C.PIPE_FILTER_COLOR
	pipeGlyphTex    = C.PIPE_GLYPH_TEX
	pipeGlyphColor  = C.PIPE_GLYPH_COLOR
	pipeStencil     = C.PIPE_STENCIL
)

const maxCustomPipelines = 32

// Backend is the Metal backend for go-gui (single-window mode).
// Embeds windowState so all draw methods are shared with
// multi-window mode.
type Backend struct {
	windowState
}

// New creates a Metal backend and initializes the window.
func New(w *gui.Window) (*Backend, error) {
	runtime.LockOSThread()

	// Activate the app before creating any windows so the
	// window is visible on macOS for CLI-launched binaries.
	C.metalActivateApp()

	ws, err := createWindowState(w)
	if err != nil {
		return nil, fmt.Errorf("metal: %w", err)
	}

	b := &Backend{windowState: *ws}
	ws.setAttachedWindow(w)
	injectInterfaces(w, &b.windowState)
	return b, nil
}

// Run starts the event loop. Blocks until quit.
func (b *Backend) Run(w *gui.Window) {
	defer w.WindowCleanup()
	if w.Config.OnInit != nil {
		w.Config.OnInit(w)
	}

	// Set dock icon once, after first event poll so AppKit
	// initialization is complete.
	iconSet := false

	// Activate now that windows exist on screen.
	C.metalActivateNow()

	running := true
	rendered := true
	evt := new(gui.Event)
	for running {
		waitMs := 0
		if !rendered {
			waitMs = 100
		}
		ev := C.metalPollEvent(C.int(waitMs))
		for ev != 0 {
			mapped, cont := mapMetalEvent()
			*evt = mapped
			if !cont {
				running = false
				break
			}
			if evt.Type == gui.EventQuitRequested {
				gui.DispatchCloseRequest(w)
				ev = C.metalPollEvent(0)
				continue
			}
			if evt.Type != gui.EventInvalid {
				w.EventFn(evt)
			}
			ev = C.metalPollEvent(0) // drain remaining
		}
		if !running {
			break
		}

		// Check close request (set by windowShouldClose
		// callback during event polling).
		if w.CloseRequested() {
			break
		}

		if !iconSet {
			iconSet = true
			if len(b.appIconPNG) > 0 {
				setAppIcon(b.appIconPNG)
				b.appIconPNG = nil
			}
		}

		rendered = w.FrameFn()
		if rendered {
			b.renderFrame(w)
		}

		// Update cursor.
		mc := w.MouseCursorState()
		if cs := cursorSelector(mc); cs != "" {
			cstr := C.CString(cs)
			C.metalWindowSetCursor(b.window, cstr,
				C.metalEventMouseX(),
				C.metalEventMouseY())
			C.free(unsafe.Pointer(cstr))
		}
	}
}

// Run initializes the Metal backend, runs the event loop, and
// cleans up on exit. Panics on error; call RunE for error-returning
// variant.
func Run(w *gui.Window) {
	if err := RunE(w); err != nil {
		panic(fmt.Sprintf("metal: %v", err))
	}
}

// RunE initializes the Metal backend, runs the event loop, and
// cleans up on exit. Returns an error instead of panicking so
// embedders and tests can handle backend init failures.
func RunE(w *gui.Window) error {
	b, err := New(w)
	if err != nil {
		return fmt.Errorf("metal: %w", err)
	}
	defer b.Destroy()
	b.Run(w)
	return nil
}

// RunApp starts a multi-window event loop. Panics on error;
// call RunAppE for error-returning variant.
func RunApp(app *gui.App, initialWindows ...*gui.Window) {
	if err := RunAppE(app, initialWindows...); err != nil {
		panic(fmt.Sprintf("metal: %v", err))
	}
}

// RunAppE starts a multi-window event loop. Each window in
// initialWindows is created and registered with app. Blocks
// until the app signals exit. Returns an error instead of
// panicking so embedders and tests can handle init failures.
//
//nolint:gocyclo // backend event loop
func RunAppE(app *gui.App, initialWindows ...*gui.Window) error {
	runtime.LockOSThread()

	// Activate the app before creating any windows so
	// windows are visible on macOS for CLI-launched binaries.
	C.metalActivateApp()

	states := make(map[uint32]*windowState)

	// Create initial windows.
	for _, w := range initialWindows {
		ws, err := createWindowState(w)
		if err != nil {
			return fmt.Errorf("metal: create window: %w", err)
		}
		winID := uint32(C.metalWindowGetID(ws.window))
		ws.setAttachedWindow(w)
		states[winID] = ws
		app.Register(winID, w)
		injectInterfaces(w, ws)
		if w.Config.OnInit != nil {
			w.Config.OnInit(w)
		}
	}

	defer func() {
		for _, ws := range states {
			ws.destroy()
		}
	}()

	wakeUp := func() {
		C.metalPostEmptyEvent()
	}
	setWakeFn := func(w *gui.Window) {
		w.SetWakeMainFn(wakeUp)
	}
	for _, w := range initialWindows {
		setWakeFn(w)
	}

	// Activate now that windows exist on screen.
	C.metalActivateNow()

	running := true
	rendered := true
	evt := new(gui.Event)
	appIconSet := false

	for running {
		// Drain pending window opens.
	drain:
		for {
			select {
			case cfg := <-app.PendingOpen():
				w := gui.NewWindow(cfg)
				ws, err := createWindowState(w)
				if err != nil {
					log.Printf("metal: open window: %v", err)
					continue
				}
				winID := uint32(C.metalWindowGetID(ws.window))
				ws.setAttachedWindow(w)
				states[winID] = ws
				app.Register(winID, w)
				injectInterfaces(w, ws)
				setWakeFn(w)
				if cfg.OnInit != nil {
					cfg.OnInit(w)
				}
			default:
				break drain
			}
		}

		// Poll events. When idle, wait up to 100ms.
		waitMs := 0
		if !rendered {
			waitMs = 100
		}
		ev := C.metalPollEvent(C.int(waitMs))
		for ev != 0 {
			wid := uint32(C.metalEventWindowID())
			mapped, cont := mapMetalEvent()
			*evt = mapped
			evt.WindowID = wid
			if !cont {
				// App quit — dispatch to per-window hooks.
				if !gui.DispatchQuitRequest(app) {
					running = false
				}
				break
			}
			if evt.Type == gui.EventInvalid {
				ev = C.metalPollEvent(0)
				continue
			}

			// wid=0 is global quit (Cmd+Q or system
			// termination) — broadcast close to every
			// window so per-window OnCloseRequest
			// hooks can veto or save state.
			if evt.Type == gui.EventQuitRequested {
				if wid == 0 {
					app.Broadcast(func(w *gui.Window) {
						gui.DispatchCloseRequest(w)
					})
				} else {
					gui.DispatchCloseRequest(app.Window(wid))
				}
				ev = C.metalPollEvent(0)
				continue
			}

			if w := app.Window(wid); w != nil {
				w.EventFn(evt)
			}
			ev = C.metalPollEvent(0) // drain remaining
		}
		if !running {
			break
		}

		// Set dock icon once, after first event poll.
		if !appIconSet {
			appIconSet = true
			for _, ws := range states {
				if len(ws.appIconPNG) > 0 {
					setAppIcon(ws.appIconPNG)
					ws.appIconPNG = nil
					break
				}
			}
		}

		// Handle close requests.
		for wid, ws := range states {
			w := app.Window(wid)
			if w == nil || !w.CloseRequested() {
				continue
			}
			w.WindowCleanup()
			ws.destroy()
			delete(states, wid)
			if app.Unregister(wid) {
				running = false
				break
			}
		}
		if !running {
			break
		}

		// Frame + render each window.
		rendered = false
		for wid, ws := range states {
			w := app.Window(wid)
			if w == nil {
				continue
			}
			if w.FrameFn() {
				ws.renderFrame(w)
				rendered = true
			}
		}

		// Cursor for each window.
		for wid, ws := range states {
			w := app.Window(wid)
			if w == nil {
				continue
			}
			mc := w.MouseCursorState()
			if cs := cursorSelector(mc); cs != "" {
				cstr := C.CString(cs)
				C.metalWindowSetCursor(ws.window, cstr,
					C.metalEventMouseX(),
					C.metalEventMouseY())
				C.free(unsafe.Pointer(cstr))
			}
		}
	}

	// Cleanup remaining windows.
	for wid, ws := range states {
		if w := app.Window(wid); w != nil {
			w.WindowCleanup()
		}
		ws.destroy()
	}
	return nil
}

// cursorSelector returns the NSCursor class method name for a
// gui.MouseCursor.
func cursorSelector(mc gui.MouseCursor) string {
	switch mc {
	case gui.CursorDefault, gui.CursorArrow:
		return "arrowCursor"
	case gui.CursorIBeam:
		return "IBeamCursor"
	case gui.CursorCrosshair:
		return "crosshairCursor"
	case gui.CursorPointingHand:
		return "pointingHandCursor"
	case gui.CursorResizeEW:
		return "resizeLeftRightCursor"
	case gui.CursorResizeNS:
		return "resizeUpDownCursor"
	case gui.CursorResizeNWSE:
		return "_windowResizeNorthWestSouthEastCursor"
	case gui.CursorResizeNESW:
		return "_windowResizeNorthEastSouthWestCursor"
	case gui.CursorResizeAll:
		return "closedHandCursor"
	case gui.CursorNotAllowed:
		return "operationNotAllowedCursor"
	default:
		return ""
	}
}

// windowState holds per-window backend resources for
// multi-window mode.
type windowState struct {
	ctx      C.MetalCtx
	window   C.GoGuiNSWindow
	textSys  *glyph.TextSystem
	dpiScale float32
	physW    int32
	physH    int32
	mvp      [16]float32

	mvpStack [][16]float32

	svgVerts           []gpu.Vertex
	textPathPlacements []glyph.GlyphPlacement
	normBuf            []gui.GradientStop
	sampledBuf         []gui.GradientStop

	textures          texcache.Cache[string, metalTexture]
	glyphBack         *metalGlyphBackend
	filterBlur        float32
	filterLayer       int
	filterColorMatrix *[16]float32
	customCache       texcache.Cache[uint64, C.int]
	iconFontPath      string

	allowedImageRoots []string
	imagePathCache    texcache.Cache[string, string]
	maxImageBytes     int64
	maxImagePixels    int64
	appIconPNG        []byte
	attachedWindow    *gui.Window // for file-drop callbacks
}

func (ws *windowState) setAttachedWindow(w *gui.Window) {
	ws.attachedWindow = w
	id := uint32(C.metalWindowGetID(ws.window))
	registerWindow(id, ws)
}

func createWindowState(w *gui.Window) (*windowState, error) {
	cfg := w.Config
	title := cfg.Title
	if title == "" {
		title = "go-gui"
	}
	width := int32(cfg.Width)
	if width <= 0 {
		width = 640
	}
	height := int32(cfg.Height)
	if height <= 0 {
		height = 480
	}

	fixed := 0
	if cfg.FixedSize {
		fixed = 1
	}

	cTitle := C.CString(title)
	defer C.free(unsafe.Pointer(cTitle))
	win := C.metalWindowCreate(cTitle, C.int(width), C.int(height),
		C.int(fixed))
	if win == nil {
		return nil, errors.New("metalWindowCreate failed")
	}

	iconPNG := cfg.IconPNG
	if len(iconPNG) == 0 {
		iconPNG = gui.DefaultIconPNG
	}

	layer := C.metalWindowGetLayer(win)
	if layer == nil {
		C.metalWindowDestroy(win)
		return nil, errors.New("metalWindowGetLayer failed")
	}

	ctx := C.metalCtxCreate(layer)
	if ctx == nil {
		C.metalWindowDestroy(win)
		return nil, errors.New("metalCtxCreate failed")
	}

	// Compute DPI scale from framebuffer vs logical size.
	var fbW, fbH C.int
	C.metalWindowGetFramebufferSize(win, &fbW, &fbH)
	var logW, logH C.int
	C.metalWindowGetSize(win, &logW, &logH)
	dpiScale := float32(1.0)
	if logW > 0 {
		dpiScale = float32(fbW) / float32(logW)
	}
	C.metalResize(ctx, fbW, fbH)

	ws := &windowState{
		ctx:      ctx,
		window:   win,
		dpiScale: dpiScale,
		physW:    int32(fbW),
		physH:    int32(fbH),
		textures: newMetalTexCacheLRU(ctx, 128),
		customCache: texcache.New[uint64, C.int](
			maxCustomPipelines,
			func(idx C.int) {
				C.metalDeleteCustomPipeline(ctx, idx)
			},
		),
		imagePathCache: texcache.New[string, string](1024, nil),
		maxImageBytes:  cfg.MaxImageBytes,
		maxImagePixels: cfg.MaxImagePixels,
		appIconPNG:     iconPNG,
	}
	ws.allowedImageRoots = imgpath.NormalizeRoots(
		cfg.AllowedImageRoots)
	ws.updateProjection()

	ws.glyphBack = newMetalGlyphBackend(ctx, dpiScale)
	textSys, err := glyph.NewTextSystem(ws.glyphBack)
	if err != nil {
		ws.destroy()
		return nil, fmt.Errorf("NewTextSystem: %w", err)
	}
	ws.textSys = textSys

	// Load embedded icon font.
	if data := gui.IconFontData; len(data) > 0 {
		tmp, err := tempfont.Write("go_gui_feathericon", data)
		if err != nil {
			log.Printf("metal: write icon font: %v", err)
		} else if err := textSys.AddFontFile(tmp); err != nil {
			log.Printf("metal: load icon font: %v", err)
			_ = os.Remove(tmp)
		} else {
			ws.iconFontPath = tmp
		}
	}

	for _, p := range gui.AppFontPaths {
		if err := textSys.AddFontFile(p); err != nil {
			log.Printf("metal: load app font %q: %v",
				filepath.Base(p), err)
		}
	}

	return ws, nil
}

func injectInterfaces(w *gui.Window, ws *windowState) {
	w.SetTextMeasurer(&textMeasurer{textSys: ws.textSys})
	w.SetSvgParser(svg.New())
	w.SetClipboardFn(func(text string) {
		cstr := C.CString(text)
		defer C.free(unsafe.Pointer(cstr))
		C.metalClipboardSet(cstr)
	})
	w.SetClipboardGetFn(func() string {
		cstr := C.metalClipboardGet()
		if cstr == nil {
			return ""
		}
		defer C.free(unsafe.Pointer(cstr))
		return C.GoString(cstr)
	})
	w.SetTitleFn(func(t string) {
		cstr := C.CString(t)
		defer C.free(unsafe.Pointer(cstr))
		C.metalWindowSetTitle(ws.window, cstr)
	})
	w.SetNativePlatform(&nativePlatform{window: ws.window})
}

func (ws *windowState) destroy() {
	if ws.window != nil {
		unregisterWindow(uint32(C.metalWindowGetID(ws.window)))
	}
	ws.textures.DestroyAll()
	ws.customCache.DestroyAll()
	if ws.glyphBack != nil {
		ws.glyphBack.destroy()
	}
	if ws.textSys != nil {
		ws.textSys.Free()
	}
	if ws.iconFontPath != "" {
		_ = os.Remove(ws.iconFontPath)
	}
	if ws.ctx != nil {
		C.metalCtxDestroy(ws.ctx)
		ws.ctx = nil
	}
	if ws.window != nil {
		C.metalWindowDestroy(ws.window)
		ws.window = nil
	}
}

func (ws *windowState) renderFrame(w *gui.Window) {
	bg := w.Config.BgColor
	if bg == (gui.Color{}) {
		t := gui.CurrentTheme()
		bg = t.ColorBackground
	}
	rc := C.metalBeginFrame(ws.ctx,
		C.float(float32(bg.R)/255.0),
		C.float(float32(bg.G)/255.0),
		C.float(float32(bg.B)/255.0),
		C.float(float32(bg.A)/255.0),
	)
	if rc != 0 {
		return
	}
	C.metalSetPipeline(ws.ctx, C.int(pipeSolid))
	C.metalSetMVP(ws.ctx, (*C.float)(&ws.mvp[0]))

	w.Lock()
	w.BackingScale = ws.dpiScale
	ws.renderersDraw(w)
	w.Unlock()

	ws.useGlyphPipeline()
	ws.textSys.Commit()
	C.metalEndFrame(ws.ctx)
}

func (ws *windowState) handleResize() {
	var fbW, fbH C.int
	C.metalWindowGetFramebufferSize(ws.window, &fbW, &fbH)
	var logW, logH C.int
	C.metalWindowGetSize(ws.window, &logW, &logH)

	// Guard against zero dimensions — can occur during
	// teardown or if the window is occluded before its first
	// draw. Passing 0 to Metal resize/projection produces NaN.
	if fbW <= 0 || fbH <= 0 {
		return
	}

	ws.physW = int32(fbW)
	ws.physH = int32(fbH)
	if logW > 0 {
		ws.dpiScale = float32(fbW) / float32(logW)
	}
	C.metalResize(ws.ctx, fbW, fbH)
	ws.updateProjection()
}

func (ws *windowState) updateProjection() {
	gpu.Ortho(&ws.mvp,
		0, float32(ws.physW),
		float32(ws.physH), 0,
		-1, 1)
}

func (ws *windowState) useGlyphPipeline() {
	C.metalSetPipeline(ws.ctx, C.int(pipeGlyphTex))
	C.metalSetMVP(ws.ctx, (*C.float)(&ws.mvp[0]))
}

// Destroy releases all backend resources.
func (b *Backend) Destroy() {
	b.destroy()
}
