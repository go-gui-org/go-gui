//go:build darwin && arm64 && !ios

// Package metal provides a native Metal backend for go-gui on macOS.
// Uses NSWindow + CAMetalLayer for windowing and Metal for GPU rendering.
// System frameworks only — no external library dependencies.
package metal

import (
	"errors"
	"fmt"
	"log"
	"runtime"

	"github.com/ebitengine/purego/objc"

	"github.com/go-gui-org/go-gui/gui"
	"github.com/go-gui-org/go-gui/gui/backend/internal/mtldraw"
	"github.com/go-gui-org/go-gui/gui/svg"
)

// idleWaitMs returns the poll timeout for one event-loop iteration.
// -1 blocks until an event ([NSDate distantFuture]); 0 drains without
// waiting. The loop blocks when the previous frame rendered nothing
// (idle) — every path that dirties state wakes it: QueueCommand and
// the animation loop via wakeUp, App.OpenWindow via the
// app wake fn, and the focus/file-drop callbacks in callbacks.go
// (issue #405).
func idleWaitMs(rendered bool) int {
	if !rendered {
		return -1
	}
	return 0
}

// Backend is the Metal backend for go-gui (single-window mode).
// Embeds windowState so all draw methods are shared with
// multi-window mode. A pointer: windowState owns texcache.Cache
// values (container/list internals), which must not be copied
// after first use.
type Backend struct {
	*windowState
}

// New creates a Metal backend and initializes the window.
// exportaudit:keep — lowercase new shadows the Go builtin
func New(w *gui.Window) (*Backend, error) {
	runtime.LockOSThread()

	// Activate the app before creating any windows so the
	// window is visible on macOS for CLI-launched binaries.
	metalAppInit()

	ws, err := createWindowState(w)
	if err != nil {
		return nil, fmt.Errorf("metal: %w", err)
	}

	b := &Backend{windowState: ws}
	b.setAttachedWindow(w)
	injectInterfaces(w, b.windowState)
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
	metalAppFinishLaunch()

	w.SetWakeMainFn(wakeUp)

	running := true
	rendered := true
	evt := new(gui.Event)
	for running {
		// Idle: block until an event wakes us instead of polling.
		// Every path that dirties state wakes the loop (issue #405).
		ev := metalPollEvent(idleWaitMs(rendered))
		for ev {
			mapped, cont := mapMetalEvent()
			*evt = mapped
			if !cont {
				// Single-window Run does not support quit
				// veto — running=false unconditionally.
				// OnCloseRequest fires so hooks can save
				// state, but the window exits regardless.
				gui.DispatchCloseRequest(w)
				running = false
				break
			}
			if evt.Type != gui.EventInvalid {
				if evt.Type == gui.EventMouseLeave || evt.Type == gui.EventFocused {
					b.lastCursorInBounds = false
				}
				w.EventFn(evt)
			}
			ev = metalPollEvent(0) // drain remaining
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
		b.updateCursor(w.MouseCursorState())
	}
}

// Run initializes the Metal backend, runs the event loop, and
// cleans up on exit. Panics on error; call RunE for error-returning
// variant.
func Run(w *gui.Window) {
	if err := runE(w); err != nil {
		panic(fmt.Sprintf("metal: %v", err))
	}
}

// RunE initializes the Metal backend, runs the event loop, and
// cleans up on exit. Returns an error instead of panicking so
// embedders and tests can handle backend init failures.
func runE(w *gui.Window) error {
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
	if err := runAppE(app, initialWindows...); err != nil {
		panic(fmt.Sprintf("metal: %v", err))
	}
}

// RunAppE starts a multi-window event loop. Each window in
// initialWindows is created and registered with app. Blocks
// until the app signals exit. Returns an error instead of
// panicking so embedders and tests can handle init failures.
//
//nolint:gocyclo // backend event loop
func runAppE(app *gui.App, initialWindows ...*gui.Window) error {
	runtime.LockOSThread()

	// Before metalAppInit: a launch with documents delivers them in
	// the first [NSApp run] (metalAppFinishLaunch), which must find
	// the App already set.
	openApp.Store(app)
	defer openApp.CompareAndSwap(app, nil)

	// Activate the app before creating any windows so
	// windows are visible on macOS for CLI-launched binaries.
	metalAppInit()

	states := make(map[uint32]*windowState)

	// Create initial windows.
	for _, w := range initialWindows {
		ws, err := createWindowState(w)
		if err != nil {
			return fmt.Errorf("metal: create window: %w", err)
		}
		winID := metalWindowGetID(ws.window)
		ws.setAttachedWindow(w)
		states[winID] = ws
		app.Register(winID, w)
		injectInterfaces(w, ws)
		if w.Config.OnInit != nil {
			w.Config.OnInit(w)
		}
	}

	defer func() {
		metalStopFramePump()
		for _, ws := range states {
			ws.destroy()
		}
	}()

	setWakeFn := func(w *gui.Window) {
		w.SetWakeMainFn(wakeUp)
		// App-level wake: OpenWindow from another goroutine must
		// unblock the idle loop even though it cannot select on the
		// pending channel (issue #405).
		app.SetWakeMainFn(wakeUp)
	}
	for _, w := range initialWindows {
		setWakeFn(w)
	}

	// Activate now that windows exist on screen.
	metalAppFinishLaunch()

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
				winID := metalWindowGetID(ws.window)
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

		// Poll events. Idle: block until an event wakes us instead of
		// polling (issue #405).
		ev := metalPollEvent(idleWaitMs(rendered))
		for ev {
			wid := pollState.ev.windowID
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
				ev = metalPollEvent(0)
				continue
			}

			if w := app.Window(wid); w != nil {
				if evt.Type == gui.EventMouseLeave || evt.Type == gui.EventFocused {
					if ws := states[wid]; ws != nil {
						ws.lastCursorInBounds = false
					}
				}
				w.EventFn(evt)
			}
			ev = metalPollEvent(0) // drain remaining
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
			ws.updateCursor(w.MouseCursorState())
		}
	}

	// Cleanup remaining windows. Delete each entry so the deferred
	// cleanup above does not destroy it a second time.
	for wid, ws := range states {
		if w := app.Window(wid); w != nil {
			w.WindowCleanup()
		}
		ws.destroy()
		delete(states, wid)
	}
	return nil
}

// cursorSelector returns the NSCursor class method name for a
// gui.MouseCursor.
//
// The diagonal-resize entries name private AppKit selectors
// (underscore prefix): AppKit exposes no public diagonal-resize
// cursor. If a future macOS removes them, the ObjC side
// (metalWindowSetCursor) degrades to a no-op via its
// respondsToSelector check, keeping the previous cursor rather
// than crashing. Do not add more private selectors without a
// public fallback.
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
	case gui.CursorGrab:
		return "openHandCursor"
	case gui.CursorGrabbing:
		return "closedHandCursor"
	default:
		return ""
	}
}

// cursorSels caches the NSCursor selector of each cursor, made once at
// startup, so the per-frame cursor update does not take the runtime lock
// of objc.RegisterName.
var cursorSelectors = func() map[gui.MouseCursor]objc.SEL {
	cursors := []gui.MouseCursor{
		gui.CursorDefault, gui.CursorArrow, gui.CursorIBeam,
		gui.CursorCrosshair, gui.CursorPointingHand,
		gui.CursorResizeEW, gui.CursorResizeNS,
		gui.CursorResizeNWSE, gui.CursorResizeNESW,
		gui.CursorResizeAll, gui.CursorNotAllowed,
		gui.CursorGrab, gui.CursorGrabbing,
	}
	m := make(map[gui.MouseCursor]objc.SEL, len(cursors))
	for _, c := range cursors {
		if sel := cursorSelector(c); sel != "" {
			m[c] = objc.RegisterName(sel)
		}
	}
	return m
}()

// updateCursor sets the native cursor for the window from a cached
// selector, skipping redundant AppKit calls when both the cursor shape
// and pointer-in-bounds state are unchanged.
func (ws *windowState) updateCursor(mc gui.MouseCursor) {
	sel := cursorSelectors[mc]
	if sel == 0 {
		return
	}
	if ws.logW > 0 && ws.logH > 0 {
		inBounds := cursorInContentBounds(pollState.ev.mouseX, pollState.ev.mouseY,
			float32(ws.logW), float32(ws.logH))
		if !inBounds {
			ws.lastCursorInBounds = false
			return
		}
		if ws.lastCursorInBounds && mc == ws.lastCursor {
			return
		}
		ws.lastCursor = mc
		ws.lastCursorInBounds = true
	}
	metalWindowSetCursor(ws.window, sel, pollState.ev.mouseX, pollState.ev.mouseY)
}

// windowState holds per-window backend resources for
// multi-window mode: the AppKit window, and the shared Metal renderer
// (gui/backend/internal/mtldraw) that draws into its layer.
type windowState struct {
	r                  *mtldraw.Renderer
	window             *macWindow
	appIconPNG         []byte
	attachedWindow     *gui.Window // for file-drop callbacks
	lastCursor         gui.MouseCursor
	lastCursorInBounds bool
	logW, logH         int
}

func (ws *windowState) setAttachedWindow(w *gui.Window) {
	ws.attachedWindow = w
	gui.SetWindowRendersShaders(w, true)
	id := metalWindowGetID(ws.window)
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

	win := metalWindowCreate(title, int(width), int(height),
		cfg.FixedSize, int(cfg.Decorations), startHidden(w))
	if win == nil {
		return nil, errors.New("metalWindowCreate failed")
	}

	// Before the first frame, so no opaque frame is composited first.
	if cfg.Transparent {
		metalWindowSetTransparent(win, true)
	}

	// CAMetalLayer.displaySyncEnabled = NO was tried and measured (issue
	// #907): nextDrawable still blocks until the window compositor
	// returns a drawable, once per display refresh, in a window and in
	// full screen alike. FPS stayed at 240 on a 240 Hz display, so the
	// flag is reported as not honored rather than set to no effect.
	if cfg.VSyncOff {
		w.DebugWindowVSync("VSyncOff requested but macOS paces frames with the window compositor")
	}

	// Replay a SetWindowOpacity made before the native platform was
	// attached (in OnInit, or before backend.Run). Same reason as
	// above: apply it before the first frame is composited.
	if o := w.WindowOpacity(); o < 1 {
		metalWindowSetAlpha(win, o)
	}

	// Cocoa content sizes are points, the same unit WindowCfg uses, so
	// the limits go across unscaled. Skipped entirely when nothing is
	// constrained, leaving the window's AppKit defaults untouched.
	if limits := gui.WindowSizeLimits(cfg); !limits.None() {
		metalWindowSetSizeLimits(win, limits.MinW, limits.MinH,
			limits.MaxW, limits.MaxH)
	}

	iconPNG := cfg.IconPNG
	if len(iconPNG) == 0 {
		iconPNG = gui.DefaultIconPNG
	}

	if win.layer == 0 {
		metalWindowDestroy(win)
		return nil, errors.New("metal: window has no CAMetalLayer")
	}

	// Compute DPI scale from framebuffer vs logical size.
	fbW, fbH := metalWindowGetFramebufferSize(win, scaleOverride())
	logW, logH := metalWindowGetSize(win)
	dpiScale := drawableScale(int32(fbW), int32(logW), 1)

	r, err := mtldraw.New(win.layer, fbW, fbH, dpiScale, cfg, "metal")
	if err != nil {
		metalWindowDestroy(win)
		return nil, err
	}
	return &windowState{r: r, window: win, appIconPNG: iconPNG, logW: logW, logH: logH}, nil
}

func injectInterfaces(w *gui.Window, ws *windowState) {
	w.SetTextMeasurer(ws.r.TextMeasurer())
	w.SetSvgParser(svg.New())
	w.SetClipboardFn(metalClipboardSet)
	w.SetClipboardGetFn(func() string {
		text, _ := metalClipboardGet()
		return text
	})
	w.SetTitleFn(func(t string) {
		metalWindowSetTitle(ws.window, t)
	})
	w.SetNativePlatform(&nativePlatform{window: ws.window})
}

func (ws *windowState) destroy() {
	if ws.window != nil {
		unregisterWindow(metalWindowGetID(ws.window))
	}
	if ws.r != nil {
		ws.r.Destroy()
		ws.r = nil
	}
	if ws.window != nil {
		metalWindowDestroy(ws.window)
		ws.window = nil
	}
}

func (ws *windowState) renderFrame(w *gui.Window) {
	ws.r.RenderFrame(w)
}

func (ws *windowState) handleResize() {
	fbW, fbH := metalWindowGetFramebufferSize(ws.window, scaleOverride())
	logW, logH := metalWindowGetSize(ws.window)
	ws.logW, ws.logH = logW, logH
	ws.lastCursorInBounds = false

	// Guard against zero dimensions — can occur during
	// teardown or if the window is occluded before its first
	// draw. Passing 0 to Metal resize/projection produces NaN.
	if fbW <= 0 || fbH <= 0 {
		return
	}
	ws.r.SetDPIScale(drawableScale(int32(fbW), int32(logW), ws.r.DPIScale()))
	ws.r.Resize(fbW, fbH)
}

// Destroy releases all backend resources.
func (b *Backend) Destroy() {
	// App-global, so it belongs here rather than in windowState.destroy:
	// leaving a live timer behind would keep pumping frames for windows
	// that no longer exist (and, in tests, across packages' runs).
	metalStopFramePump()
	b.destroy()
}
