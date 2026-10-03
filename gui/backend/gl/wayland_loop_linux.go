//go:build linux && !js && !android && (amd64 || arm64)

package gl

import (
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/go-gui-org/go-gui/gui"
)

var wlFallbackOnce sync.Once

// newWaylandBackendFunc is the seam a test counts Wayland window attempts
// through.
var newWaylandBackendFunc = newWaylandBackend

// warnWaylandFallback says once why the Wayland backend was not used.
func warnWaylandFallback(err error) {
	wlFallbackOnce.Do(func() {
		log.Printf("gl: GOGUI_WAYLAND=1 but Wayland setup failed, using X11: %v", err)
	})
}

// tryWayland is New's Wayland branch. ok is false when Wayland was not
// asked for, or could not be set up; New then continues with X11.
func tryWayland(w *gui.Window) (*Backend, bool) {
	if !waylandRequested() {
		return nil, false
	}
	b, err := newWaylandBackendFunc(w)
	if err != nil {
		warnWaylandFallback(err)
		return nil, false
	}
	return b, true
}

// runWayland is Run for a window on Wayland.
func (b *Backend) runWayland(w *gui.Window) {
	defer w.WindowCleanup()
	if w.Config.OnInit != nil {
		w.Config.OnInit(w)
	}
	d := b.plat.wl.d
	w.SetWakeMainFn(d.conn.Wake)
	l := wlLoop{d: d, wins: map[uint32]*Backend{b.plat.wl.id(): b}}
	if err := l.run(); err != nil {
		log.Printf("gl: wayland: %v", err)
	}
}

// runAppWayland is RunApp on Wayland. handled is false when the display
// could not be opened; RunApp then falls back to X11.
func runAppWayland(app *gui.App, initial []*gui.Window) (handled bool, err error) {
	d, err := acquireWaylandDisplay()
	if err != nil {
		warnWaylandFallback(err)
		return false, nil
	}
	// The loop holds its own reference, so the connection outlives the
	// moment between the last window closing and a new one opening.
	defer d.release()
	app.SetWakeMainFn(d.conn.Wake)
	l := wlLoop{d: d, app: app, wins: map[uint32]*Backend{}}
	for _, w := range initial {
		if err := l.open(w); err != nil {
			l.closeAll()
			return true, fmt.Errorf("gl: create window: %w", err)
		}
	}
	return true, l.run()
}

// wlLoop is the Wayland event loop. One loop drives every window, because
// all of them share the one connection. app is nil for Run: one window,
// owned and destroyed by the caller.
type wlLoop struct {
	d    *wlDisplay
	app  *gui.App
	wins map[uint32]*Backend // surface id → backend
}

// open creates a window and registers it with the app.
func (l *wlLoop) open(w *gui.Window) error {
	b, err := newWaylandBackend(w)
	if err != nil {
		return err
	}
	id := b.plat.wl.id()
	l.wins[id] = b
	l.app.Register(id, w)
	w.SetWakeMainFn(l.d.conn.Wake)
	if w.Config.OnInit != nil {
		w.Config.OnInit(w)
	}
	return nil
}

// close destroys one window. It reports whether it was the app's last.
func (l *wlLoop) close(id uint32, b *Backend) bool {
	b.plat.w.WindowCleanup()
	b.Destroy()
	delete(l.wins, id)
	return l.app.Unregister(id)
}

func (l *wlLoop) closeAll() {
	for id, b := range l.wins {
		l.close(id, b)
	}
}

// run loops until the last window closes or the connection fails.
//
// Each pass: take the events already waiting, open queued windows, close
// windows that asked to, then give every window not waiting on a frame
// callback a frame. When nothing rendered, it blocks in Dispatch until an
// event, a Wake (gui redraw requests, OpenWindow, titles), the nearest
// frame timeout or the next key repeat. A window that rendered waits for its frame callback, so
// the loop runs at the compositor's pace, not flat out.
//
//nolint:gocyclo // event loop
func (l *wlLoop) run() error {
	for len(l.wins) > 0 {
		if err := l.d.conn.Dispatch(0); err != nil {
			l.closeOnError()
			return err
		}
		if l.app != nil {
			l.openPending()
		}
		for id, b := range l.wins {
			if !b.plat.w.CloseRequested() {
				continue
			}
			if l.app == nil {
				return nil // Run: the caller destroys the backend
			}
			if l.close(id, b) {
				// The app exits: with ExitOnMainClose (the default),
				// other windows may still be open. Each holds a
				// reference to the display, so all are destroyed here.
				l.closeAll()
				return nil
			}
		}

		now := time.Now()
		wait := time.Duration(-1)
		if l.d.seat != nil {
			// Key repeats fire here; the wait below ends in time for
			// the next one.
			wait = l.d.seat.tickRepeat(now)
		}
		rendered := false
		for _, b := range l.wins {
			ww := b.plat.wl
			ww.flushTitle()
			ww.syncFocus()
			if pending, left := ww.framePending(now); pending {
				if wait < 0 || left < wait {
					wait = left
				}
				continue
			}
			w := b.plat.w
			// FrameFn runs every pass, as on X11: it also drains queued
			// commands. A configure needs a frame even when it has none.
			if w.FrameFn() || ww.dirty {
				ww.requestFrame(now)
				b.renderFrame(w)
				ww.dirty = false
				rendered = true
			}
		}
		if rendered {
			continue
		}
		if err := l.d.conn.Dispatch(wait); err != nil {
			l.closeOnError()
			return err
		}
	}
	return nil
}

// openPending creates the windows App.OpenWindow queued, without blocking.
func (l *wlLoop) openPending() {
	for {
		select {
		case cfg := <-l.app.PendingOpen():
			if err := l.open(gui.NewWindow(cfg)); err != nil {
				log.Printf("gl: open window: %v", err)
			}
		default:
			return
		}
	}
}

// closeOnError cleans up after the connection died. RunApp owns its
// windows; Run's caller destroys its one.
func (l *wlLoop) closeOnError() {
	if l.app != nil {
		l.closeAll()
	}
}
