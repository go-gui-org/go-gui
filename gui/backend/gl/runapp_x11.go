//go:build linux && !js && !android

package gl

import (
	"fmt"
	"log"
	"os"
	"runtime"
	"sync"

	"github.com/jezek/xgb"

	"github.com/go-gui-org/go-gui/gui"
)

// Destroy releases all backend resources.
func (b *Backend) Destroy() {
	// A soft window (#1265) never loaded EGL or the GL function table, so
	// the GL teardown would call through nil function pointers.
	if b.plat.softMode {
		b.plat.soft.destroy()
		b.plat.soft = nil
	} else {
		b.destroyGLResources()
	}
	b.plat.destroy()
	b.plat.releaseThread()
}

// Run starts the event loop. Blocks until the window is closed.
func (b *Backend) Run(w *gui.Window) {
	if b.plat.wl != nil {
		b.runWayland(w)
		return
	}
	defer w.WindowCleanup()
	b.plat.w = w
	if w.Config.OnInit != nil {
		w.Config.OnInit(w)
	}
	w.SetWakeMainFn(b.plat.wake)

	events := make(chan xgb.Event, 64)
	go pumpXEvents(b.plat.conn, events)

	// drain processes every currently-queued event without blocking.
	// Returns false when the connection has closed.
	drain := func() bool {
		for {
			select {
			case ev, ok := <-events:
				if !ok {
					return false
				}
				b.handleXEvent(ev)
			default:
				b.drainIME()
				return true
			}
		}
	}

	var rendered bool
	running := true
	for running {
		if !drain() || w.CloseRequested() {
			break
		}
		rendered = w.FrameFn()
		if rendered {
			b.presentX11(w)
		}
		b.plat.setCursor(w.MouseCursorState())
		if !rendered {
			// Idle: block until an event or a wake arrives instead of
			// polling (issue #405). wake() posts a ClientMessage that
			// pumpXEvents forwards onto this channel.
			ev, ok := <-events
			if !ok {
				running = false
			} else {
				b.handleXEvent(ev)
			}
		}
	}
}

// Run initializes the backend, runs the event loop, and cleans up on
// exit. A GPU-context failure prints advice and exits non-zero; any
// other error panics. Call runE for the error-returning variant.
func Run(w *gui.Window) {
	if err := runE(w); err != nil {
		exitNoGPUOrPanic(err)
	}
}

// RunE initializes the backend, runs the event loop, and cleans up on
// exit. Returns an error instead of panicking.
func runE(w *gui.Window) error {
	b, err := New(w)
	if err != nil {
		return fmt.Errorf("gl: %w", err)
	}
	defer b.Destroy()
	b.Run(w)
	return nil
}

// RunApp starts a multi-window event loop. A GPU-context failure
// prints advice and exits non-zero; any other error panics. Call
// runAppE for the error-returning variant.
func RunApp(app *gui.App, initialWindows ...*gui.Window) {
	if err := runAppE(app, initialWindows...); err != nil {
		exitNoGPUOrPanic(err)
	}
}

// waylandRequested reports whether to try the native Wayland backend
// (#919). Wayland is the default on a Wayland session (#1113), which WAYLAND_DISPLAY
// marks, as GTK, Qt and SDL also read it. GOGUI_WAYLAND=0 keeps X11, and
// GOGUI_WAYLAND=1 tries Wayland even with no WAYLAND_DISPLAY. Any other value
// is the same as none. Without WAYLAND_DISPLAY, libwayland would connect to
// "wayland-0", which can be a stray socket in an X11 session, so the default
// does not try.
//
// GOGUI_RENDERER=soft declines Wayland outright (#1265): the CPU renderer
// presents through X11 PutImage only, so until the wl_shm path lands it runs
// on XWayland. The decision is logged once, since a user who set both
// GOGUI_RENDERER=soft and GOGUI_WAYLAND=1 gets a backend they did not name.
func waylandRequested() bool {
	if softRequested() {
		softWaylandOnce.Do(func() {
			if os.Getenv("WAYLAND_DISPLAY") != "" || waylandForced() {
				log.Printf("gl: GOGUI_RENDERER=soft presents through X11; using XWayland")
			}
		})
		return false
	}
	switch os.Getenv("GOGUI_WAYLAND") {
	case "0":
		return false
	case "1":
		return true
	}
	return os.Getenv("WAYLAND_DISPLAY") != ""
}

// waylandForced reports whether the user set GOGUI_WAYLAND=1. Only then is a
// Wayland backend that cannot be used on this CPU worth a warning: on the
// default path, X11 is the expected result there.
func waylandForced() bool { return os.Getenv("GOGUI_WAYLAND") == "1" }

// softWaylandOnce keeps waylandRequested's XWayland notice to one line per
// process: New and RunApp both ask, and RunApp may open many windows.
var softWaylandOnce sync.Once

// softRequested reports whether GOGUI_RENDERER=soft asks for the CPU renderer
// (gui/backend/soft) in place of EGL/GL (#1265). Only the exact value "soft"
// counts; "gl", empty and unknown values keep GL, and matching is
// case-sensitive like GOGUI_WAYLAND. Step 3 of #1265 adds auto-pick for the
// unset case.
func softRequested() bool { return os.Getenv("GOGUI_RENDERER") == "soft" }

// taggedEvent carries an X event alongside the backend it belongs to so
// a single channel can multiplex several windows' event pumps.
type taggedEvent struct {
	b      *Backend
	ev     xgb.Event
	closed bool
}

// liveBackend reports whether te should be handed to its backend. Events
// queued before closeWindow ran still sit in the channel, and a destroyed
// backend has a nil conn that handlers such as setCursor or a selection
// reply would dereference (issue #701). closeWindow deletes the map entry
// and destroy() zeroes plat.window, so a closed backend is never the map
// entry for its id.
func liveBackend(backends map[uint32]*Backend, te taggedEvent) bool {
	return !te.closed && backends[uint32(te.b.plat.window)] == te.b
}

// RunAppE starts a multi-window event loop. Each window is created and
// registered with app, keyed by its X window id. Blocks until the last
// window closes.
//
//nolint:gocyclo // backend event loop
func runAppE(app *gui.App, initialWindows ...*gui.Window) error {
	runtime.LockOSThread()
	// Balanced for the same reason as New's lock (#827): a caller whose
	// goroutine returns after RunApp must not exit with its thread locked.
	defer runtime.UnlockOSThread()

	if waylandRequested() {
		if handled, err := runAppWayland(app, initialWindows); handled {
			return err
		}
	}

	backends := make(map[uint32]*Backend) // window XID → backend
	events := make(chan taggedEvent, 128)

	open := func(w *gui.Window) error {
		b, err := newX11(w)
		if err != nil {
			return err
		}
		b.plat.w = w
		id := uint32(b.plat.window)
		backends[id] = b
		app.Register(id, w)
		w.SetWakeMainFn(b.plat.wake)
		if w.Config.OnInit != nil {
			w.Config.OnInit(w)
		}
		// The connection is read here, on the main goroutine, and handed
		// to the forwarder. Reading bk.plat.conn inside the goroutine
		// would race closeWindow's destroy(), which nils the field, and
		// could start the pump on a nil conn (issue #701).
		go func(bk *Backend, conn *xgb.Conn) {
			ch := make(chan xgb.Event, 64)
			go pumpXEvents(conn, ch)
			for ev := range ch {
				events <- taggedEvent{b: bk, ev: ev}
			}
			events <- taggedEvent{b: bk, closed: true}
		}(b, b.plat.conn)
		return nil
	}

	for _, w := range initialWindows {
		if err := open(w); err != nil {
			for _, b := range backends {
				b.Destroy()
			}
			return fmt.Errorf("gl: create window: %w", err)
		}
	}

	closeWindow := func(b *Backend) bool {
		id := uint32(b.plat.window)
		if w := app.Window(id); w != nil {
			w.WindowCleanup()
		}
		b.Destroy()
		delete(backends, id)
		return app.Unregister(id)
	}

	dispatch := func(te taggedEvent) {
		if liveBackend(backends, te) {
			te.b.handleXEvent(te.ev)
		}
	}

	var rendered bool
	for len(backends) > 0 {
		// Drain queued events + window opens.
		drained := false
		for !drained {
			select {
			case te := <-events:
				dispatch(te)
			case cfg := <-app.PendingOpen():
				if err := open(gui.NewWindow(cfg)); err != nil {
					log.Printf("gl: open window: %v", err)
				}
			default:
				for _, b := range backends {
					b.drainIME()
				}
				drained = true
			}
		}

		// Handle close requests.
		for _, b := range backends {
			w := app.Window(uint32(b.plat.window))
			if w == nil || !w.CloseRequested() {
				continue
			}
			if closeWindow(b) {
				return nil // last window closed
			}
		}
		if len(backends) == 0 {
			return nil
		}

		// Frame + render each window.
		rendered = false
		for _, b := range backends {
			w := app.Window(uint32(b.plat.window))
			if w == nil {
				continue
			}
			if w.FrameFn() {
				b.presentX11(w)
				rendered = true
			}
			b.plat.setCursor(w.MouseCursorState())
		}

		if !rendered {
			// Idle: block until an event, a wake, or a pending window
			// arrives instead of polling (issue #405).
			select {
			case te := <-events:
				dispatch(te)
			case cfg := <-app.PendingOpen():
				if err := open(gui.NewWindow(cfg)); err != nil {
					log.Printf("gl: open window: %v", err)
				}
			}
		}
	}
	return nil
}
