//go:build !windows && !js

package gl

import (
	"fmt"
	"log"
	"runtime"

	"github.com/go-gl/gl/v3.3-core/gl"
	"github.com/veandco/go-sdl2/sdl"

	"github.com/go-gui-org/go-gui/gui"
)

// platformState holds the SDL2 windowing state for the GL backend.
type platformState struct {
	window  *sdl.Window
	glCtx   sdl.GLContext
	cursors [11]*sdl.Cursor
}

func (p *platformState) makeCurrent() { _ = p.window.GLMakeCurrent(p.glCtx) }
func (p *platformState) swap()        { p.window.GLSwap() }

func (p *platformState) drawableSize() (int32, int32) {
	return p.window.GLGetDrawableSize()
}

func (p *platformState) dpiScale() float32 {
	glW, _ := p.window.GLGetDrawableSize()
	winW, _ := p.window.GetSize()
	if winW <= 0 {
		return 1.0
	}
	return float32(glW) / float32(winW)
}

func (p *platformState) setCursor(mc gui.MouseCursor) {
	if int(mc) < len(p.cursors) && p.cursors[mc] != nil {
		sdl.SetCursor(p.cursors[mc])
	}
}

func (p *platformState) destroy() {
	for i, c := range p.cursors {
		if c != nil {
			sdl.FreeCursor(c)
			p.cursors[i] = nil
		}
	}
	if p.glCtx != nil {
		sdl.GLDeleteContext(p.glCtx)
	}
	if p.window != nil {
		_ = p.window.Destroy()
	}
	sdl.Quit()
}

// New creates an OpenGL 3.3 backend and initializes the SDL2 window.
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

	b := &Backend{}
	b.plat.window = win
	b.plat.glCtx = glCtx
	b.dpiScale = dpiScale
	b.physW = glW
	b.physH = glH
	b.initCaches(cfg)

	if err := b.initGLResources(w); err != nil {
		b.Destroy()
		return nil, fmt.Errorf("gl: initGLResources: %w", err)
	}

	// Create system cursors.
	b.plat.cursors[gui.CursorDefault] = sdl.CreateSystemCursor(sdl.SYSTEM_CURSOR_ARROW)
	b.plat.cursors[gui.CursorArrow] = sdl.CreateSystemCursor(sdl.SYSTEM_CURSOR_ARROW)
	b.plat.cursors[gui.CursorIBeam] = sdl.CreateSystemCursor(sdl.SYSTEM_CURSOR_IBEAM)
	b.plat.cursors[gui.CursorCrosshair] = sdl.CreateSystemCursor(sdl.SYSTEM_CURSOR_CROSSHAIR)
	b.plat.cursors[gui.CursorPointingHand] = sdl.CreateSystemCursor(sdl.SYSTEM_CURSOR_HAND)
	b.plat.cursors[gui.CursorResizeEW] = sdl.CreateSystemCursor(sdl.SYSTEM_CURSOR_SIZEWE)
	b.plat.cursors[gui.CursorResizeNS] = sdl.CreateSystemCursor(sdl.SYSTEM_CURSOR_SIZENS)
	b.plat.cursors[gui.CursorResizeNWSE] = sdl.CreateSystemCursor(sdl.SYSTEM_CURSOR_SIZENWSE)
	b.plat.cursors[gui.CursorResizeNESW] = sdl.CreateSystemCursor(sdl.SYSTEM_CURSOR_SIZENESW)
	b.plat.cursors[gui.CursorResizeAll] = sdl.CreateSystemCursor(sdl.SYSTEM_CURSOR_SIZEALL)
	b.plat.cursors[gui.CursorNotAllowed] = sdl.CreateSystemCursor(sdl.SYSTEM_CURSOR_NO)

	// Set platform-specific injected interfaces on gui Window.
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
		b.plat.window.SetTitle(t)
	})

	return b, nil
}

// Destroy releases all backend resources.
func (b *Backend) Destroy() {
	b.destroyGLResources()
	b.plat.destroy()
}

// Run starts the event loop. Blocks until quit.
func (b *Backend) Run(w *gui.Window) {
	defer w.WindowCleanup()
	if w.Config.OnInit != nil {
		w.Config.OnInit(w)
	}

	// Register event watcher for live resize on macOS.
	// During window drag-resize, the OS enters a modal loop that
	// blocks PollEvent. This callback fires from within that
	// loop, allowing re-layout and re-render at the new size.
	resizeEvent := &gui.Event{Type: gui.EventResized}
	var watchHandle sdl.EventWatchHandle
	if runtime.GOOS == "darwin" {
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

		b.plat.setCursor(w.MouseCursorState())
	}
}

// Run initializes the GL backend, runs the event loop, and cleans
// up on exit. Panics on error; call RunE for error-returning variant.
func Run(w *gui.Window) {
	if err := RunE(w); err != nil {
		panic(fmt.Sprintf("gl: %v", err))
	}
}

// RunE initializes the GL backend, runs the event loop, and cleans
// up on exit. Returns an error instead of panicking so embedders
// and tests can handle backend init failures gracefully.
func RunE(w *gui.Window) error {
	b, err := New(w)
	if err != nil {
		return fmt.Errorf("gl: %w", err)
	}
	defer b.Destroy()
	b.Run(w)
	return nil
}

// RunApp starts a multi-window event loop. Panics on error; call
// RunAppE for error-returning variant.
func RunApp(app *gui.App, initialWindows ...*gui.Window) {
	if err := RunAppE(app, initialWindows...); err != nil {
		panic(fmt.Sprintf("gl: %v", err))
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

	if err := sdl.Init(sdl.INIT_VIDEO | sdl.INIT_EVENTS); err != nil {
		return fmt.Errorf("gl: Init: %w", err)
	}
	defer sdl.Quit()

	backends := make(map[uint32]*Backend)

	// Create initial windows.
	for _, w := range initialWindows {
		b, err := New(w)
		if err != nil {
			return fmt.Errorf("gl: create window: %w", err)
		}
		sdlID, _ := b.plat.window.GetID()
		backends[sdlID] = b
		app.Register(sdlID, w)
		if w.Config.OnInit != nil {
			w.Config.OnInit(w)
		}
	}

	// Event watcher for live resize on macOS.
	resizeEvent := &gui.Event{Type: gui.EventResized}
	var watchHandle sdl.EventWatchHandle
	if runtime.GOOS == "darwin" {
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
				sdlID, _ := b.plat.window.GetID()
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
					b.plat.setCursor(w.MouseCursorState())
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
	return nil
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

// --- IME (SDL2) ---

func (n *nativePlatform) IMEStart() { sdl.StartTextInput() }
func (n *nativePlatform) IMEStop()  { sdl.StopTextInput() }
func (n *nativePlatform) IMESetRect(x, y, w, h int32) {
	sdl.SetTextInputRect(&sdl.Rect{X: x, Y: y, W: w, H: h})
}
