//go:build !js

// Package gl provides an OpenGL 3.3 backend for go-gui.
package gl

import (
	"fmt"
	"log"
	"os"
	"runtime"
	"sync"

	"github.com/go-gl/gl/v3.3-core/gl"
	"github.com/go-gui-org/go-glyph"
	"github.com/veandco/go-sdl2/sdl"

	"github.com/go-gui-org/go-gui/gui"
	"github.com/go-gui-org/go-gui/gui/backend/internal/imgpath"
	"github.com/go-gui-org/go-gui/gui/backend/internal/tempfont"
	"github.com/go-gui-org/go-gui/gui/backend/internal/texcache"
	"github.com/go-gui-org/go-gui/gui/svg"
)

// Backend is the OpenGL 3.3 backend for go-gui.
type Backend struct {
	pipelines pipelineSet
	cursors   [11]*sdl.Cursor

	textures          texcache.Cache[string, glTexture]
	imagePathCache    texcache.Cache[string, string]
	window            *sdl.Window
	glCtx             sdl.GLContext
	textSys           *glyph.TextSystem
	filterColorMatrix *[16]float32

	glyphBack    *glyphBackend
	iconFontPath string

	mvpStack [][16]float32

	textPathPlacements []glyph.GlyphPlacement
	svgVerts           []vertex
	normBuf            []gui.GradientStop
	sampledBuf         []gui.GradientStop

	allowedImageRoots []string
	svgCap            int
	filterLayer       int
	maxImageBytes     int64
	maxImagePixels    int64
	mvp               [16]float32

	customOnce sync.Once
	dpiScale   float32
	physW      int32
	physH      int32
	quadVAO    uint32
	quadVBO    uint32
	quadIBO    uint32

	// Reusable buffers.
	svgVAO        uint32
	svgVBO        uint32
	filterFBO     uint32
	filterStencil uint32
	filterTexA    uint32
	filterTexB    uint32
	filterW       int32
	filterH       int32
	filterBlur    float32
}

// New creates an OpenGL 3.3 backend and initializes the window.
func New(w *gui.Window) (*Backend, error) {
	runtime.LockOSThread()

	if err := sdl.Init(sdl.INIT_VIDEO | sdl.INIT_EVENTS); err != nil {
		return nil, fmt.Errorf("gl: Init: %w", err)
	}

	// Request OpenGL 3.3 core profile.
	_ = sdl.GLSetAttribute(sdl.GL_CONTEXT_MAJOR_VERSION, 3)
	_ = sdl.GLSetAttribute(sdl.GL_CONTEXT_MINOR_VERSION, 3)
	_ = sdl.GLSetAttribute(sdl.GL_CONTEXT_PROFILE_MASK,
		sdl.GL_CONTEXT_PROFILE_CORE)
	_ = sdl.GLSetAttribute(sdl.GL_CONTEXT_FLAGS,
		sdl.GL_CONTEXT_FORWARD_COMPATIBLE_FLAG)
	_ = sdl.GLSetAttribute(sdl.GL_DOUBLEBUFFER, 1)
	_ = sdl.GLSetAttribute(sdl.GL_STENCIL_SIZE, 8)

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
	flags := uint32(sdl.WINDOW_SHOWN | sdl.WINDOW_ALLOW_HIGHDPI | sdl.WINDOW_OPENGL)
	if !cfg.FixedSize {
		flags |= sdl.WINDOW_RESIZABLE
	}

	win, err := sdl.CreateWindow(
		title,
		sdl.WINDOWPOS_CENTERED, sdl.WINDOWPOS_CENTERED,
		width, height,
		flags,
	)
	if err != nil {
		sdl.Quit()
		return nil, fmt.Errorf("gl: CreateWindow: %w", err)
	}

	glCtx, err := win.GLCreateContext()
	if err != nil {
		_ = win.Destroy()
		sdl.Quit()
		return nil, fmt.Errorf(
			"gl: GLCreateContext: %w — OpenGL 3.3 required. "+
				"Install GPU drivers with OpenGL 3.3+ support, or "+
				"drop the -tags gl build tag to use the default SDL2 renderer", err)
	}

	if err := gl.Init(); err != nil {
		sdl.GLDeleteContext(glCtx)
		_ = win.Destroy()
		sdl.Quit()
		return nil, fmt.Errorf("gl: gl.Init: %w", err)
	}

	// Enable vsync.
	_ = sdl.GLSetSwapInterval(1)

	// Compute DPI scale.
	glW, glH := win.GLGetDrawableSize()
	winW, _ := win.GetSize()
	dpiScale := float32(1.0)
	if winW > 0 {
		dpiScale = float32(glW) / float32(winW)
	}

	b := &Backend{
		window:         win,
		glCtx:          glCtx,
		dpiScale:       dpiScale,
		physW:          glW,
		physH:          glH,
		textures:       newGLTexCacheLRU(128),
		imagePathCache: texcache.New[string, string](1024, nil),
		maxImageBytes:  cfg.MaxImageBytes,
		maxImagePixels: cfg.MaxImagePixels,
	}
	b.allowedImageRoots = imgpath.NormalizeRoots(cfg.AllowedImageRoots)

	// Initialize GL state.
	gl.Enable(gl.BLEND)
	gl.BlendFunc(gl.SRC_ALPHA, gl.ONE_MINUS_SRC_ALPHA)
	gl.Disable(gl.DEPTH_TEST)
	gl.Disable(gl.CULL_FACE)
	gl.Viewport(0, 0, glW, glH)

	// Compile shader pipelines.
	if err := b.initPipelines(); err != nil {
		sdl.GLDeleteContext(glCtx)
		_ = win.Destroy()
		sdl.Quit()
		return nil, fmt.Errorf("gl: initPipelines: %w", err)
	}

	// Initialize quad buffers.
	b.initQuadBuffers()
	b.initSvgBuffers()

	// Build ortho projection.
	b.updateProjection()

	// Initialize glyph text system with GL backend.
	b.glyphBack = newGlyphBackend(dpiScale)
	textSys, err := glyph.NewTextSystem(b.glyphBack)
	if err != nil {
		b.Destroy()
		return nil, fmt.Errorf("gl: NewTextSystem: %w", err)
	}
	b.textSys = textSys

	// Load embedded icon font. File must persist because
	// FontConfig registers the path; FreeType reads it lazily.
	if data := gui.IconFontData; len(data) > 0 {
		tmp, err := tempfont.Write("go_gui_feathericon", data)
		if err != nil {
			log.Printf("gl: write icon font: %v", err)
		} else if err := textSys.AddFontFile(tmp); err != nil {
			log.Printf("gl: load icon font: %v", err)
			_ = os.Remove(tmp)
		} else {
			b.iconFontPath = tmp
		}
	}

	for _, p := range gui.AppFontPaths {
		if err := textSys.AddFontFile(p); err != nil {
			log.Printf("gl: load app font %q: %v", p, err)
		}
	}

	// Create system cursors.
	b.cursors[gui.CursorDefault] = sdl.CreateSystemCursor(sdl.SYSTEM_CURSOR_ARROW)
	b.cursors[gui.CursorArrow] = sdl.CreateSystemCursor(sdl.SYSTEM_CURSOR_ARROW)
	b.cursors[gui.CursorIBeam] = sdl.CreateSystemCursor(sdl.SYSTEM_CURSOR_IBEAM)
	b.cursors[gui.CursorCrosshair] = sdl.CreateSystemCursor(sdl.SYSTEM_CURSOR_CROSSHAIR)
	b.cursors[gui.CursorPointingHand] = sdl.CreateSystemCursor(sdl.SYSTEM_CURSOR_HAND)
	b.cursors[gui.CursorResizeEW] = sdl.CreateSystemCursor(sdl.SYSTEM_CURSOR_SIZEWE)
	b.cursors[gui.CursorResizeNS] = sdl.CreateSystemCursor(sdl.SYSTEM_CURSOR_SIZENS)
	b.cursors[gui.CursorResizeNWSE] = sdl.CreateSystemCursor(sdl.SYSTEM_CURSOR_SIZENWSE)
	b.cursors[gui.CursorResizeNESW] = sdl.CreateSystemCursor(sdl.SYSTEM_CURSOR_SIZENESW)
	b.cursors[gui.CursorResizeAll] = sdl.CreateSystemCursor(sdl.SYSTEM_CURSOR_SIZEALL)
	b.cursors[gui.CursorNotAllowed] = sdl.CreateSystemCursor(sdl.SYSTEM_CURSOR_NO)

	// Set injected interfaces on gui Window.
	w.SetTextMeasurer(&textMeasurer{textSys: textSys})
	w.SetSvgParser(svg.New())
	w.SetClipboardFn(func(text string) {
		if err := sdl.SetClipboardText(text); err != nil {
			log.Printf("gl: set clipboard: %v", err)
		}
	})
	w.SetClipboardGetFn(func() string {
		text, _ := sdl.GetClipboardText()
		return text
	})
	w.SetTitleFn(func(t string) {
		b.window.SetTitle(t)
	})
	w.SetNativePlatform(&nativePlatform{})

	return b, nil
}

// Run starts the event loop. Blocks until quit.
func (b *Backend) Run(w *gui.Window) {
	defer w.WindowCleanup()
	if w.Config.OnInit != nil {
		w.Config.OnInit(w)
	}

	// Register event watcher for live resize on macOS and Windows.
	// During window drag-resize, the OS enters a modal loop that
	// blocks PollEvent. This callback fires from within that
	// loop, allowing re-layout and re-render at the new size.
	resizeEvent := &gui.Event{Type: gui.EventResized}
	var watchHandle sdl.EventWatchHandle
	if runtime.GOOS == "darwin" || runtime.GOOS == "windows" {
		watchHandle = sdl.AddEventWatchFunc(
			func(ev sdl.Event, _ any) bool {
				we, ok := ev.(*sdl.WindowEvent)
				if !ok || we.Event != sdl.WINDOWEVENT_SIZE_CHANGED {
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
	}

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

// renderFrame clears the screen, draws the current layout, and
// swaps buffers. Makes this window's GL context current first.
func (b *Backend) renderFrame(w *gui.Window) {
	_ = b.window.GLMakeCurrent(b.glCtx)
	bg := w.Config.BgColor
	if bg == (gui.Color{}) {
		t := gui.CurrentTheme()
		bg = t.ColorBackground
	}
	gl.ClearColor(
		float32(bg.R)/255.0,
		float32(bg.G)/255.0,
		float32(bg.B)/255.0,
		float32(bg.A)/255.0,
	)
	gl.Disable(gl.SCISSOR_TEST)
	gl.Clear(gl.COLOR_BUFFER_BIT | gl.STENCIL_BUFFER_BIT)

	w.Lock()
	w.BackingScale = b.dpiScale
	b.renderersDraw(w)
	w.Unlock()

	b.textSys.Commit()
	b.window.GLSwap()
}

func (b *Backend) handleResize() {
	glW, glH := b.window.GLGetDrawableSize()
	b.physW = glW
	b.physH = glH
	winW, _ := b.window.GetSize()
	if winW > 0 {
		b.dpiScale = float32(glW) / float32(winW)
	}
	gl.Viewport(0, 0, glW, glH)
	b.updateProjection()
}

func (b *Backend) updateProjection() {
	ortho(&b.mvp,
		0, float32(b.physW),
		float32(b.physH), 0,
		-1, 1)
}

// Run initializes the GL backend, runs the event loop, and cleans
// up on exit.
func Run(w *gui.Window) {
	b, err := New(w)
	if err != nil {
		panic(fmt.Sprintf("gl: %v", err))
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
		panic(fmt.Sprintf("gl: Init: %v", err))
	}
	defer sdl.Quit()

	backends := make(map[uint32]*Backend)

	// Create initial windows.
	for _, w := range initialWindows {
		b, err := New(w)
		if err != nil {
			panic(fmt.Sprintf("gl: create window: %v", err))
		}
		sdlID, _ := b.window.GetID()
		backends[sdlID] = b
		app.Register(sdlID, w)
		if w.Config.OnInit != nil {
			w.Config.OnInit(w)
		}
	}

	// Event watcher for live resize on macOS and Windows.
	resizeEvent := &gui.Event{Type: gui.EventResized}
	var watchHandle sdl.EventWatchHandle
	if runtime.GOOS == "darwin" || runtime.GOOS == "windows" {
		watchHandle = sdl.AddEventWatchFunc(
			func(ev sdl.Event, _ any) bool {
				we, ok := ev.(*sdl.WindowEvent)
				if !ok ||
					we.Event != sdl.WINDOWEVENT_SIZE_CHANGED {
					return true
				}
				wid := we.WindowID
				b := backends[wid]
				w := app.Window(wid)
				if b == nil || w == nil {
					return true
				}
				b.handleResize()
				resizeEvent.WindowID = wid
				resizeEvent.WindowWidth = int(we.Data1)
				resizeEvent.WindowHeight = int(we.Data2)
				w.EventFn(resizeEvent)
				w.FrameFn()
				b.renderFrame(w)
				return true
			}, nil)
		defer sdl.DelEventWatch(watchHandle)
	}

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

	for running {
		// Drain pending window opens.
	drain:
		for {
			select {
			case cfg := <-app.PendingOpen():
				w := gui.NewWindow(cfg)
				b, err := New(w)
				if err != nil {
					log.Printf("gl: open window: %v", err)
					continue
				}
				sdlID, _ := b.window.GetID()
				backends[sdlID] = b
				app.Register(sdlID, w)
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
		for ev := sdl.WaitEventTimeout(waitMs); ev != nil; ev = sdl.PollEvent() {
			wid := sdlEventWindowID(ev)
			mapped, cont := mapEventMulti(ev, backends[wid])
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

		// Handle close requests.
		for wid, b := range backends {
			w := app.Window(wid)
			if w == nil || !w.CloseRequested() {
				continue
			}
			w.WindowCleanup()
			b.Destroy()
			delete(backends, wid)
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
		for wid, b := range backends {
			w := app.Window(wid)
			if w == nil {
				continue
			}
			if w.FrameFn() {
				b.renderFrame(w)
				rendered = true
			}
		}

		// Cursor for focused window.
		if focused := sdl.GetKeyboardFocus(); focused != nil {
			fid, _ := focused.GetID()
			if w := app.Window(fid); w != nil {
				if b := backends[fid]; b != nil {
					mc := w.MouseCursorState()
					if int(mc) < len(b.cursors) &&
						b.cursors[mc] != nil {
						sdl.SetCursor(b.cursors[mc])
					}
				}
			}
		}
	}

	// Cleanup remaining windows.
	for wid, b := range backends {
		if w := app.Window(wid); w != nil {
			w.WindowCleanup()
		}
		b.Destroy()
		delete(backends, wid)
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

// Destroy releases all backend resources.
func (b *Backend) Destroy() {
	b.textures.DestroyAll()
	b.destroyPipelines()
	if b.quadVAO != 0 {
		gl.DeleteVertexArrays(1, &b.quadVAO)
	}
	if b.quadVBO != 0 {
		gl.DeleteBuffers(1, &b.quadVBO)
	}
	if b.quadIBO != 0 {
		gl.DeleteBuffers(1, &b.quadIBO)
	}
	if b.svgVAO != 0 {
		gl.DeleteVertexArrays(1, &b.svgVAO)
	}
	if b.svgVBO != 0 {
		gl.DeleteBuffers(1, &b.svgVBO)
	}
	b.destroyFilterFBO()
	if b.glyphBack != nil {
		b.glyphBack.destroy()
	}
	if b.textSys != nil {
		b.textSys.Free()
	}
	if b.iconFontPath != "" {
		_ = os.Remove(b.iconFontPath)
		b.iconFontPath = ""
	}
	for i, c := range b.cursors {
		if c != nil {
			sdl.FreeCursor(c)
			b.cursors[i] = nil
		}
	}
	if b.glCtx != nil {
		sdl.GLDeleteContext(b.glCtx)
	}
	if b.window != nil {
		_ = b.window.Destroy()
	}
	sdl.Quit()
}

// ortho builds an orthographic projection matrix (column-major).
func ortho(m *[16]float32, l, r, b, t, n, f float32) {
	m[0] = 2 / (r - l)
	m[1] = 0
	m[2] = 0
	m[3] = 0

	m[4] = 0
	m[5] = 2 / (t - b)
	m[6] = 0
	m[7] = 0

	m[8] = 0
	m[9] = 0
	m[10] = -2 / (f - n)
	m[11] = 0

	m[12] = -(r + l) / (r - l)
	m[13] = -(t + b) / (t - b)
	m[14] = -(f + n) / (f - n)
	m[15] = 1
}
