//go:build linux && !js && !android

package gl

import (
	"fmt"
	"log"
	"runtime"
	"time"

	"github.com/jezek/xgb"

	"github.com/go-gui-org/go-gui/gui"
)

// Destroy releases all backend resources.
func (b *Backend) Destroy() {
	b.destroyGLResources()
	b.plat.destroy()
}

// Run starts the event loop. Blocks until the window is closed.
func (b *Backend) Run(w *gui.Window) {
	defer w.WindowCleanup()
	b.plat.w = w
	if w.Config.OnInit != nil {
		w.Config.OnInit(w)
	}
	w.SetWakeMainFn(b.plat.wake)

	events := make(chan xgb.Event, 64)
	go b.plat.pumpEvents(events)

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
			b.renderFrame(w)
		}
		b.plat.setCursor(w.MouseCursorState())
		if !rendered {
			select {
			case ev, ok := <-events:
				if !ok {
					running = false
				} else {
					b.handleXEvent(ev)
				}
			case <-time.After(100 * time.Millisecond):
			}
		}
	}
}

// Run initializes the backend, runs the event loop, and cleans up on
// exit. Panics on error; call RunE for the error-returning variant.
func Run(w *gui.Window) {
	if err := runE(w); err != nil {
		panic(fmt.Sprintf("gl: %v", err))
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

// RunApp starts a multi-window event loop. Panics on error; call
// RunAppE for the error-returning variant.
func RunApp(app *gui.App, initialWindows ...*gui.Window) {
	if err := runAppE(app, initialWindows...); err != nil {
		panic(fmt.Sprintf("gl: %v", err))
	}
}

// taggedEvent carries an X event alongside the backend it belongs to so
// a single channel can multiplex several windows' event pumps.
type taggedEvent struct {
	b      *Backend
	ev     xgb.Event
	closed bool
}

// RunAppE starts a multi-window event loop. Each window is created and
// registered with app, keyed by its X window id. Blocks until the last
// window closes.
//
//nolint:gocyclo // backend event loop
func runAppE(app *gui.App, initialWindows ...*gui.Window) error {
	runtime.LockOSThread()

	backends := make(map[uint32]*Backend) // window XID → backend
	events := make(chan taggedEvent, 128)

	open := func(w *gui.Window) error {
		b, err := New(w)
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
		go func(bk *Backend) {
			ch := make(chan xgb.Event, 64)
			go bk.plat.pumpEvents(ch)
			for ev := range ch {
				events <- taggedEvent{b: bk, ev: ev}
			}
			events <- taggedEvent{b: bk, closed: true}
		}(b)
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

	var rendered bool
	for len(backends) > 0 {
		// Drain queued events + window opens.
		drained := false
		for !drained {
			select {
			case te := <-events:
				if te.closed {
					continue
				}
				te.b.handleXEvent(te.ev)
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
				b.renderFrame(w)
				rendered = true
			}
			b.plat.setCursor(w.MouseCursorState())
		}

		if !rendered {
			select {
			case te := <-events:
				if !te.closed {
					te.b.handleXEvent(te.ev)
				}
			case cfg := <-app.PendingOpen():
				if err := open(gui.NewWindow(cfg)); err != nil {
					log.Printf("gl: open window: %v", err)
				}
			case <-time.After(100 * time.Millisecond):
			}
		}
	}
	return nil
}
