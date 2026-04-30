//go:build darwin && !ios

// Package metal provides a Metal backend for go-gui on macOS.
// Uses SDL2 for window creation and event handling, and Metal
// for GPU rendering. Eliminates the macOS compositor content
// shift during window resize via CAMetalLayer's
// presentsWithTransaction.
package metal

/*
#cgo CFLAGS: -fobjc-arc
#cgo darwin,arm64 CFLAGS: -I/opt/homebrew/include/SDL2
#cgo darwin,amd64 CFLAGS: -I/usr/local/include/SDL2
#cgo LDFLAGS: -framework Metal -framework QuartzCore -framework Foundation
#include "metal_darwin.h"
#include <SDL.h>
#include <SDL_metal.h>
*/
import "C"

import (
	"fmt"
	"log"
	"os"
	"runtime"
	"unsafe"

	"github.com/mike-ward/go-glyph"
	"github.com/veandco/go-sdl2/sdl"

	"github.com/mike-ward/go-gui/gui"
	"github.com/mike-ward/go-gui/gui/backend/internal/imgpath"
	"github.com/mike-ward/go-gui/gui/backend/internal/tempfont"
	"github.com/mike-ward/go-gui/gui/backend/internal/texcache"
	"github.com/mike-ward/go-gui/gui/svg"
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
	cursors [11]*sdl.Cursor
}

// New creates a Metal backend and initializes the window.
func New(w *gui.Window) (*Backend, error) {
	runtime.LockOSThread()

	if err := sdl.Init(sdl.INIT_VIDEO | sdl.INIT_EVENTS); err != nil {
		return nil, fmt.Errorf("metal: Init: %w", err)
	}

	ws, err := createWindowState(w, nil)
	if err != nil {
		sdl.Quit()
		return nil, fmt.Errorf("metal: %w", err)
	}

	b := &Backend{windowState: *ws}
	b.cursors = createCursors()

	injectInterfaces(w, &b.windowState)
	return b, nil
}

// Run starts the event loop. Blocks until quit.
func (b *Backend) Run(w *gui.Window) {
	defer w.WindowCleanup()
	if w.Config.OnInit != nil {
		w.Config.OnInit(w)
	}

	// Register event watcher for live resize on macOS.
	// During window drag-resize, macOS enters a modal loop that
	// blocks PollEvent. This callback fires from within that
	// loop, allowing re-layout and re-render at the new size.
	resizeEvent := &gui.Event{Type: gui.EventResized}
	watchHandle := sdl.AddEventWatchFunc(
		func(ev sdl.Event, _ any) bool {
			we, ok := ev.(*sdl.WindowEvent)
			if !ok ||
				we.Event != sdl.WINDOWEVENT_SIZE_CHANGED {
				return true
			}
			b.handleResize()
			resizeEvent.WindowWidth = int(we.Data1)
			resizeEvent.WindowHeight = int(we.Data2)
			w.EventFn(resizeEvent)
			w.FrameFn()
			b.renderFrame(w)
			return true
		}, nil)
	defer sdl.DelEventWatch(watchHandle)

	// Register native Cocoa file drop handler. This bypasses
	// SDL2's DropEvent which crashes in go-sdl2 on macOS (the
	// binding calls SDL_free on a Cocoa-allocated string).
	fileDropInitBridge(b.window, w)
	defer fileDropDestroyBridge(b.window)

	wakeType := sdl.RegisterEvents(1)
	w.SetWakeMainFn(func() {
		_, _ = sdl.PushEvent(&sdl.UserEvent{Type: wakeType})
	})

	running := true
	rendered := true
	evt := new(gui.Event)
	for running {
		waitMs := 0
		if !rendered {
			waitMs = 100
		}
		for ev := sdl.WaitEventTimeout(waitMs); ev != nil; ev = sdl.PollEvent() {
			mapped, cont := mapEvent(ev, b)
			*evt = mapped
			if !cont {
				running = false
				break
			}
			if evt.Type != gui.EventInvalid {
				w.EventFn(evt)
			}
		}
		if !running {
			break
		}

		// Set dock icon once, after the first event poll so
		// SDL's Cocoa initialization is complete.
		if len(b.appIconPNG) > 0 {
			setAppIcon(b.appIconPNG)
			b.appIconPNG = nil
		}

		rendered = w.FrameFn()
		if rendered {
			b.renderFrame(w)
		}

		mc := w.MouseCursorState()
		if int(mc) < len(b.cursors) && b.cursors[mc] != nil {
			sdl.SetCursor(b.cursors[mc])
		}
	}
}

// Run initializes the Metal backend, runs the event loop, and
// cleans up on exit.
func Run(w *gui.Window) {
	b, err := New(w)
	if err != nil {
		panic(fmt.Sprintf("metal: %v", err))
	}
	defer b.Destroy()
	b.Run(w)
}

// RunApp starts a multi-window event loop. Each window in
// initialWindows is created and registered with app. Blocks
// until the app signals exit.
//
//nolint:gocyclo // backend event loop
func RunApp(app *gui.App, initialWindows ...*gui.Window) {
	runtime.LockOSThread()

	if err := sdl.Init(sdl.INIT_VIDEO | sdl.INIT_EVENTS); err != nil {
		panic(fmt.Sprintf("metal: Init: %v", err))
	}
	defer sdl.Quit()

	// Shared resources.
	cursors := createCursors()
	defer freeCursors(&cursors)

	states := make(map[uint32]*windowState)

	// Create initial windows.
	for _, w := range initialWindows {
		ws, err := createWindowState(w, &cursors)
		if err != nil {
			panic(fmt.Sprintf("metal: create window: %v", err))
		}
		sdlID, _ := ws.window.GetID()
		states[sdlID] = ws
		app.Register(sdlID, w)
		injectInterfaces(w, ws)
		fileDropInitBridge(ws.window, w)
		if w.Config.OnInit != nil {
			w.Config.OnInit(w)
		}
	}
	defer func() {
		for _, ws := range states {
			fileDropDestroyBridge(ws.window)
		}
	}()

	// Event watcher for live resize on macOS.
	resizeEvent := &gui.Event{Type: gui.EventResized}
	watchHandle := sdl.AddEventWatchFunc(
		func(ev sdl.Event, _ any) bool {
			we, ok := ev.(*sdl.WindowEvent)
			if !ok ||
				we.Event != sdl.WINDOWEVENT_SIZE_CHANGED {
				return true
			}
			wid := we.WindowID
			ws := states[wid]
			w := app.Window(wid)
			if ws == nil || w == nil {
				return true
			}
			ws.handleResize()
			resizeEvent.WindowID = wid
			resizeEvent.WindowWidth = int(we.Data1)
			resizeEvent.WindowHeight = int(we.Data2)
			w.EventFn(resizeEvent)
			w.FrameFn()
			ws.renderFrame(w)
			return true
		}, nil)
	defer sdl.DelEventWatch(watchHandle)

	wakeType := sdl.RegisterEvents(1)
	setWakeFn := func(w *gui.Window) {
		w.SetWakeMainFn(func() {
			_, _ = sdl.PushEvent(&sdl.UserEvent{Type: wakeType})
		})
	}
	for _, w := range initialWindows {
		setWakeFn(w)
	}

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
				ws, err := createWindowState(w, &cursors)
				if err != nil {
					log.Printf("metal: open window: %v", err)
					continue
				}
				sdlID, _ := ws.window.GetID()
				states[sdlID] = ws
				app.Register(sdlID, w)
				injectInterfaces(w, ws)
				setWakeFn(w)
				fileDropInitBridge(ws.window, w)
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
		for ev := sdl.WaitEventTimeout(waitMs); ev != nil; ev = sdl.PollEvent() {
			wid := sdlEventWindowID(ev)
			mapped, cont := mapEventMulti(ev, states[wid])
			*evt = mapped
			evt.WindowID = wid
			if !cont {
				// QuitEvent — dispatch to per-window hooks.
				if !gui.DispatchQuitRequest(app) {
					running = false
				}
				break
			}
			if evt.Type == gui.EventInvalid {
				continue
			}

			// Window close event — dispatch to hook or closeReq.
			if isWindowClose(ev) {
				gui.DispatchCloseRequest(app.Window(wid))
				continue
			}

			if w := app.Window(wid); w != nil {
				w.EventFn(evt)
			}
		}
		if !running {
			break
		}

		// Set dock icon once, after the first event poll.
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
			fileDropDestroyBridge(ws.window)
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

		// Cursor for focused window.
		if focused := sdl.GetKeyboardFocus(); focused != nil {
			fid, _ := focused.GetID()
			if w := app.Window(fid); w != nil {
				mc := w.MouseCursorState()
				if int(mc) < len(cursors) && cursors[mc] != nil {
					sdl.SetCursor(cursors[mc])
				}
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
}

// windowState holds per-window backend resources for
// multi-window mode.
type windowState struct {
	ctx       C.MetalCtx
	window    *sdl.Window
	metalView unsafe.Pointer
	textSys   *glyph.TextSystem
	dpiScale  float32
	physW     int32
	physH     int32
	mvp       [16]float32

	mvpStack [][16]float32

	svgVerts           []vertex
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
}

func createWindowState(w *gui.Window,
	_ *[11]*sdl.Cursor) (*windowState, error) {

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

	const windowMetal = 0x20000000
	flags := uint32(sdl.WINDOW_SHOWN |
		sdl.WINDOW_ALLOW_HIGHDPI | windowMetal)
	if !cfg.FixedSize {
		flags |= sdl.WINDOW_RESIZABLE
	}
	win, err := sdl.CreateWindow(
		title,
		sdl.WINDOWPOS_CENTERED, sdl.WINDOWPOS_CENTERED,
		width, height, flags,
	)
	if err != nil {
		return nil, fmt.Errorf("CreateWindow: %w", err)
	}

	iconPNG := cfg.IconPNG
	if len(iconPNG) == 0 {
		iconPNG = gui.DefaultIconPNG
	}
	setWindowIcon(win, iconPNG)

	cWin := (*C.SDL_Window)(unsafe.Pointer(win))
	metalView := unsafe.Pointer(
		C.SDL_Metal_CreateView(cWin))
	if metalView == nil {
		_ = win.Destroy()
		return nil, fmt.Errorf("MetalCreateView failed")
	}
	layer := C.SDL_Metal_GetLayer(
		C.SDL_MetalView(metalView))
	if layer == nil {
		C.SDL_Metal_DestroyView(C.SDL_MetalView(metalView))
		_ = win.Destroy()
		return nil, fmt.Errorf("MetalGetLayer failed")
	}

	ctx := C.metalCtxCreate(layer)
	if ctx == nil {
		C.SDL_Metal_DestroyView(C.SDL_MetalView(metalView))
		_ = win.Destroy()
		return nil, fmt.Errorf("metalCtxCreate failed")
	}

	var dw, dh C.int
	C.SDL_Metal_GetDrawableSize(cWin, &dw, &dh)
	winW, _ := win.GetSize()
	dpiScale := float32(1.0)
	if winW > 0 {
		dpiScale = float32(dw) / float32(winW)
	}
	C.metalResize(ctx, dw, dh)

	ws := &windowState{
		ctx:       ctx,
		window:    win,
		metalView: metalView,
		dpiScale:  dpiScale,
		physW:     int32(dw),
		physH:     int32(dh),
		textures:  newMetalTexCacheLRU(ctx, 128),
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
			log.Printf("metal: load app font %q: %v", p, err)
		}
	}

	return ws, nil
}

func injectInterfaces(w *gui.Window, ws *windowState) {
	w.SetTextMeasurer(&textMeasurer{textSys: ws.textSys})
	w.SetSvgParser(svg.New())
	w.SetClipboardFn(func(text string) {
		if err := sdl.SetClipboardText(text); err != nil {
			log.Printf("metal: set clipboard: %v", err)
		}
	})
	w.SetClipboardGetFn(func() string {
		text, _ := sdl.GetClipboardText()
		return text
	})
	w.SetTitleFn(func(t string) {
		ws.window.SetTitle(t)
	})
	w.SetNativePlatform(&nativePlatform{window: ws.window})
}

func (ws *windowState) destroy() {
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
	if ws.metalView != nil {
		C.SDL_Metal_DestroyView(
			C.SDL_MetalView(ws.metalView))
	}
	if ws.window != nil {
		_ = ws.window.Destroy()
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
	ws.renderersDraw(w)
	w.Unlock()

	ws.useGlyphPipeline()
	ws.textSys.Commit()
	C.metalEndFrame(ws.ctx)
}

func (ws *windowState) handleResize() {
	var dw, dh C.int
	cWin := (*C.SDL_Window)(unsafe.Pointer(ws.window))
	C.SDL_Metal_GetDrawableSize(cWin, &dw, &dh)
	ws.physW = int32(dw)
	ws.physH = int32(dh)
	C.metalResize(ws.ctx, dw, dh)
	ws.updateProjection()
}

func (ws *windowState) updateProjection() {
	ortho(&ws.mvp,
		0, float32(ws.physW),
		float32(ws.physH), 0,
		-1, 1)
}

func (ws *windowState) useGlyphPipeline() {
	C.metalSetPipeline(ws.ctx, C.int(pipeGlyphTex))
	C.metalSetMVP(ws.ctx, (*C.float)(&ws.mvp[0]))
}

func createCursors() [11]*sdl.Cursor {
	var c [11]*sdl.Cursor
	c[gui.CursorDefault] = sdl.CreateSystemCursor(sdl.SYSTEM_CURSOR_ARROW)
	c[gui.CursorArrow] = sdl.CreateSystemCursor(sdl.SYSTEM_CURSOR_ARROW)
	c[gui.CursorIBeam] = sdl.CreateSystemCursor(sdl.SYSTEM_CURSOR_IBEAM)
	c[gui.CursorCrosshair] = sdl.CreateSystemCursor(sdl.SYSTEM_CURSOR_CROSSHAIR)
	c[gui.CursorPointingHand] = sdl.CreateSystemCursor(sdl.SYSTEM_CURSOR_HAND)
	c[gui.CursorResizeEW] = sdl.CreateSystemCursor(sdl.SYSTEM_CURSOR_SIZEWE)
	c[gui.CursorResizeNS] = sdl.CreateSystemCursor(sdl.SYSTEM_CURSOR_SIZENS)
	c[gui.CursorResizeNWSE] = sdl.CreateSystemCursor(sdl.SYSTEM_CURSOR_SIZENWSE)
	c[gui.CursorResizeNESW] = sdl.CreateSystemCursor(sdl.SYSTEM_CURSOR_SIZENESW)
	c[gui.CursorResizeAll] = sdl.CreateSystemCursor(sdl.SYSTEM_CURSOR_SIZEALL)
	c[gui.CursorNotAllowed] = sdl.CreateSystemCursor(sdl.SYSTEM_CURSOR_NO)
	return c
}

func freeCursors(c *[11]*sdl.Cursor) {
	for i, cur := range c {
		if cur != nil {
			sdl.FreeCursor(cur)
			c[i] = nil
		}
	}
}

// sdlEventWindowID extracts the SDL window ID from any event.
func sdlEventWindowID(ev sdl.Event) uint32 {
	switch e := ev.(type) {
	case *sdl.WindowEvent:
		return e.WindowID
	case *sdl.MouseButtonEvent:
		return e.WindowID
	case *sdl.MouseMotionEvent:
		return e.WindowID
	case *sdl.MouseWheelEvent:
		return e.WindowID
	case *sdl.KeyboardEvent:
		return e.WindowID
	case *sdl.TextInputEvent:
		return e.WindowID
	case *sdl.TextEditingEvent:
		return e.WindowID
	}
	return 0
}

// isWindowClose returns true if the event is a window close.
func isWindowClose(ev sdl.Event) bool {
	we, ok := ev.(*sdl.WindowEvent)
	return ok && we.Event == sdl.WINDOWEVENT_CLOSE
}

// mapEventMulti is like mapEvent but for multi-window mode.
// A QuitEvent returns false. Window close events are handled
// by the caller.
func mapEventMulti(ev sdl.Event,
	ws *windowState) (gui.Event, bool) {

	switch e := ev.(type) {
	case *sdl.QuitEvent:
		return gui.Event{}, false

	case *sdl.WindowEvent:
		if e.Event == sdl.WINDOWEVENT_CLOSE {
			return gui.Event{Type: gui.EventQuitRequested}, true
		}
	}
	// Delegate to normal mapping (ws may be nil for
	// unowned events).
	if ws != nil {
		return mapEventWS(ev, ws)
	}
	return gui.Event{}, true
}

// Destroy releases all backend resources.
func (b *Backend) Destroy() {
	freeCursors(&b.cursors)
	b.destroy()
	sdl.Quit()
}
