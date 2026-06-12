//go:build !js

// Package sdl2 provides an SDL2-based backend for go-gui.
package sdl2

import (
	"fmt"
	"log"
	"os"
	"runtime"

	"github.com/go-gui-org/go-glyph"
	glyphsdl "github.com/go-gui-org/go-glyph/backend/sdl2"
	"github.com/veandco/go-sdl2/sdl"

	"github.com/go-gui-org/go-gui/gui"
	"github.com/go-gui-org/go-gui/gui/backend/internal/imgpath"
	"github.com/go-gui-org/go-gui/gui/backend/internal/tempfont"
	"github.com/go-gui-org/go-gui/gui/backend/internal/texcache"
	"github.com/go-gui-org/go-gui/gui/svg"
)

// Backend is the SDL2 backend for go-gui.
type Backend struct {
	cursors            [11]*sdl.Cursor
	texCache           texcache.Cache[string, *sdl.Texture]
	imagePathCache     texcache.Cache[string, string]
	window             *sdl.Window
	renderer           *sdl.Renderer
	textSys            *glyph.TextSystem
	filterTex          *sdl.Texture // temporary render target for filter groups
	filterPrevTarget   *sdl.Texture
	filterPool         *sdl.Texture // reusable filter render target
	filterColorMatrix  *[16]float32 // color transform matrix
	iconFontPath       string
	filterPixels       []uint32     // reusable pixel buffer for color matrix
	svgVerts           []sdl.Vertex // reusable vertex buffer for SVG geometry
	textPathPlacements []glyph.GlyphPlacement
	allowedImageRoots  []string
	roundedClipStack   []roundedClipState
	normBuf            []gui.GradientStop // reusable buffer for gradient normalization
	sampledBuf         []gui.GradientStop // reusable buffer for downsampled stops
	appIconPNG         []byte             // deferred dock/taskbar icon
	filterLayers       int                // number of blur layers
	maxImageBytes      int64
	maxImagePixels     int64
	dpiScale           float32
	filterPoolW        int32
	filterPoolH        int32
	filterBlur         float32 // blur radius in pixels
}

// New creates an SDL2 backend and initializes the window.
func New(w *gui.Window) (*Backend, error) {
	runtime.LockOSThread()

	if err := sdl.Init(sdl.INIT_VIDEO | sdl.INIT_EVENTS); err != nil {
		return nil, fmt.Errorf("sdl2: Init: %w", err)
	}

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
	flags := uint32(sdl.WINDOW_SHOWN | sdl.WINDOW_ALLOW_HIGHDPI)
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
		return nil, fmt.Errorf("sdl2: CreateWindow: %w", err)
	}

	iconPNG := cfg.IconPNG
	if len(iconPNG) == 0 {
		iconPNG = gui.DefaultIconPNG
	}
	setWindowIcon(win, iconPNG)

	ren, err := sdl.CreateRenderer(win, -1,
		sdl.RENDERER_ACCELERATED|sdl.RENDERER_PRESENTVSYNC)
	if err != nil {
		_ = win.Destroy()
		sdl.Quit()
		return nil, fmt.Errorf("sdl2: CreateRenderer: %w", err)
	}
	_ = ren.SetDrawBlendMode(sdl.BLENDMODE_BLEND)

	// Compute DPI scale.
	outW, _, _ := ren.GetOutputSize()
	winW, _ := win.GetSize()
	dpiScale := float32(1.0)
	if winW > 0 {
		dpiScale = float32(outW) / float32(winW)
	}

	// Initialize glyph text system.
	glyphBack := glyphsdl.New(ren, dpiScale)
	textSys, err := glyph.NewTextSystem(glyphBack)
	if err != nil {
		_ = ren.Destroy()
		_ = win.Destroy()
		sdl.Quit()
		return nil, fmt.Errorf("sdl2: NewTextSystem: %w", err)
	}

	// Load embedded icon font into glyph via temp file.
	var iconFontPath string
	if data := gui.IconFontData; len(data) > 0 {
		tmp, err := tempfont.Write("go_gui_feathericon", data)
		if err != nil {
			log.Printf("sdl2: write icon font: %v", err)
		} else if err := textSys.AddFontFile(tmp); err != nil {
			log.Printf("sdl2: load icon font: %v", err)
			_ = os.Remove(tmp)
		} else {
			iconFontPath = tmp
		}
	}

	for _, p := range gui.AppFontPaths {
		if err := textSys.AddFontFile(p); err != nil {
			log.Printf("sdl2: load app font %q: %v", p, err)
		}
	}

	b := &Backend{
		window:            win,
		renderer:          ren,
		textSys:           textSys,
		dpiScale:          dpiScale,
		texCache:          newSDLTexCache(128),
		iconFontPath:      iconFontPath,
		allowedImageRoots: imgpath.NormalizeRoots(cfg.AllowedImageRoots),
		imagePathCache:    texcache.New[string, string](1024, nil),
		maxImageBytes:     cfg.MaxImageBytes,
		maxImagePixels:    cfg.MaxImagePixels,
		appIconPNG:        iconPNG,
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

	// Set text measurer on gui Window.
	w.SetTextMeasurer(&textMeasurer{textSys: textSys})

	// Set SVG parser on gui Window.
	w.SetSvgParser(svg.New())

	// Set clipboard functions.
	w.SetClipboardFn(func(text string) {
		if err := sdl.SetClipboardText(text); err != nil {
			log.Printf("sdl2: set clipboard: %v", err)
		}
	})
	w.SetClipboardGetFn(func() string {
		text, _ := sdl.GetClipboardText()
		return text
	})

	w.SetTitleFn(func(t string) {
		win.SetTitle(t)
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

	// Register event watcher for live resize rendering on macOS and Windows.
	// During window drag-resize, the OS enters a modal loop that
	// blocks PollEvent. This callback fires from within that loop,
	// allowing re-layout and re-render at the new size.
	var watchHandle sdl.EventWatchHandle
	if runtime.GOOS == "darwin" || runtime.GOOS == "windows" {
		watchHandle = sdl.AddEventWatchFunc(
			func(ev sdl.Event, _ any) bool {
				we, ok := ev.(*sdl.WindowEvent)
				if !ok || we.Event != sdl.WINDOWEVENT_SIZE_CHANGED {
					return true
				}
				resizeEvent := &gui.Event{
					Type:         gui.EventResized,
					WindowWidth:  int(we.Data1),
					WindowHeight: int(we.Data2),
				}
				w.EventFn(resizeEvent)
				w.FrameFn()
				b.handleResize()
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

		// Set dock/taskbar icon once, after the first event poll
		// so SDL's Cocoa initialization is complete.
		if len(b.appIconPNG) > 0 {
			setAppIcon(b.appIconPNG)
			b.appIconPNG = nil
		}

		rendered = w.FrameFn()
		if rendered {
			b.renderFrame(w)
		}

		// Update cursor.
		mc := w.MouseCursorState()
		if int(mc) < len(b.cursors) && b.cursors[mc] != nil {
			sdl.SetCursor(b.cursors[mc])
		}
	}
}

// renderFrame clears the screen, draws the current layout, and presents.
func (b *Backend) renderFrame(w *gui.Window) {
	bg := w.Config.BgColor
	if bg == (gui.Color{}) {
		t := gui.CurrentTheme()
		bg = t.ColorBackground
	}
	_ = b.renderer.SetDrawColor(bg.R, bg.G, bg.B, bg.A)
	_ = b.renderer.Clear()
	_ = b.renderer.SetClipRect(nil)

	w.Lock()
	w.BackingScale = b.dpiScale
	b.renderersDraw(w)
	w.Unlock()

	b.textSys.Commit()
	b.renderer.Present()
}

func (b *Backend) handleResize() {
	outW, _, _ := b.renderer.GetOutputSize()
	winW, _ := b.window.GetSize()
	if winW > 0 {
		b.dpiScale = float32(outW) / float32(winW)
	}
}

// Run initializes the SDL2 backend, runs the event loop,
// and cleans up on exit. Panics on error; call RunE for
// error-returning variant.
func Run(w *gui.Window) {
	if err := RunE(w); err != nil {
		panic(fmt.Sprintf("sdl2: %v", err))
	}
}

// RunE initializes the SDL2 backend, runs the event loop,
// and cleans up on exit. Returns an error instead of panicking
// so embedders and tests can handle backend init failures.
func RunE(w *gui.Window) error {
	b, err := New(w)
	if err != nil {
		return fmt.Errorf("sdl2: %w", err)
	}
	defer b.Destroy()
	b.Run(w)
	return nil
}

// RunApp starts a multi-window event loop. Panics on error;
// call RunAppE for error-returning variant.
func RunApp(app *gui.App, initialWindows ...*gui.Window) {
	if err := RunAppE(app, initialWindows...); err != nil {
		panic(fmt.Sprintf("sdl2: %v", err))
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
		return fmt.Errorf("sdl2: Init: %w", err)
	}
	defer sdl.Quit()

	backends := make(map[uint32]*Backend)

	// Create initial windows.
	for _, w := range initialWindows {
		b, err := New(w)
		if err != nil {
			return fmt.Errorf("sdl2: create window: %w", err)
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
				resizeEvent.WindowID = wid
				resizeEvent.WindowWidth = int(we.Data1)
				resizeEvent.WindowHeight = int(we.Data2)
				w.EventFn(resizeEvent)
				w.FrameFn()
				b.handleResize()
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
	appIconSet := false

	for running {
		// Drain pending window opens.
	drain:
		for {
			select {
			case cfg := <-app.PendingOpen():
				w := gui.NewWindow(cfg)
				b, err := New(w)
				if err != nil {
					log.Printf("sdl2: open window: %v", err)
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

		// Set dock/taskbar icon once, after the first event poll
		// so SDL's Cocoa initialization is complete.
		if !appIconSet {
			appIconSet = true
			for _, b := range backends {
				if len(b.appIconPNG) > 0 {
					setAppIcon(b.appIconPNG)
					b.appIconPNG = nil
					break
				}
			}
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

// Destroy releases all backend resources.
func (b *Backend) Destroy() {
	b.texCache.DestroyAll()
	if b.filterPool != nil {
		_ = b.filterPool.Destroy()
		b.filterPool = nil
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
	if b.renderer != nil {
		_ = b.renderer.Destroy()
	}
	if b.window != nil {
		_ = b.window.Destroy()
	}
	sdl.Quit()
}
