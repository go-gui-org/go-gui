package gui

import (
	"errors"
	"log"
	"sync"
	"sync/atomic"
	"time"
)

// pendingCap bounds queued window-creation requests. OpenWindow
// never blocks; requests past this are dropped with a warning.
const pendingCap = 16

// dropLogInterval throttles the buffer-full warning so a burst
// of OpenWindow calls logs once, not once per dropped request.
const dropLogInterval = int64(time.Second)

// ExitMode controls when the application exits. Use the
// ExitOnMainClose / ExitOnTrayRemoved constants; the zero value
// is ExitOnMainClose. The type is unexported to restrict values
// to those constants — infer it with := rather than naming it.
type exitMode int

const (
	// ExitOnMainClose exits when the main (first) window is closed.
	ExitOnMainClose exitMode = iota
	// ExitOnTrayRemoved keeps the app alive while a system tray
	// icon exists, even if all windows are closed.
	ExitOnTrayRemoved
)

// App manages multiple windows in a single application.
//
// The zero value is not usable; construct with NewApp.
// Register/Unregister are called by backends on the main thread.
// OpenWindow, SetWakeMainFn, Window, Windows, and Broadcast are
// safe to call from any goroutine.
type App struct {
	windows  map[uint32]*Window
	pending  chan WindowCfg
	trays    map[int]*SystemTrayHandle
	order    []uint32
	ExitMode exitMode
	mu       sync.Mutex
	mainID   uint32
	// wakeMu guards wakeMainFn separately from mu so OpenWindow
	// (any goroutine) never blocks on map operations, and so
	// Register can queue the debug window without lock ordering
	// issues.
	wakeMu sync.RWMutex
	// wakeMainFn unblocks the backend's idle event loop when OpenWindow
	// queues a window from another goroutine. Set by backends whose
	// idle wait cannot select on pending itself (metal, win32); nil
	// elsewhere (x11 selects on the pending channel directly).
	wakeMainFn func()
	// dropped counts OpenWindow requests lost to a full buffer.
	// lastDropLog is the UnixNano of the last buffer-full warning.
	dropped     atomic.Uint64
	lastDropLog atomic.Int64
}

// NewApp creates an App with an empty window registry.
// The default exit mode is ExitOnMainClose.
func NewApp() *App {
	return &App{
		windows: make(map[uint32]*Window),
		pending: make(chan WindowCfg, pendingCap),
		trays:   make(map[int]*SystemTrayHandle),
	}
}

// Register associates a platform window ID with a Window.
// Duplicate IDs are ignored with a warning. Nil app or window
// is ignored with a warning. Called by backends on the main
// thread. The window linkage (App, PlatformID) is atomic, so
// readers on other goroutines observe a consistent value.
func (a *App) Register(id uint32, w *Window) {
	if a == nil || w == nil {
		log.Printf("gui: App.Register: nil app or window "+
			"(id %d) ignored", id)
		return
	}
	a.mu.Lock()
	if a.windows == nil {
		a.windows = make(map[uint32]*Window)
	}
	if _, exists := a.windows[id]; exists {
		a.mu.Unlock()
		log.Printf("gui: App.Register: duplicate window ID %d ignored", id)
		return
	}
	a.windows[id] = w
	a.order = append(a.order, id)
	if len(a.order) == 1 {
		a.mainID = id
	}
	// Config is immutable after NewWindow, so this read is safe.
	needDebug := w.Config.DebugTimeTravel
	a.mu.Unlock()
	w.app.Store(a)
	w.platformID.Store(id)
	if needDebug {
		// Auto-spawn the scrubber for the newly registered
		// window. Runs outside a.mu: OpenWindow takes wakeMu
		// and must never nest inside the map lock.
		w.openDebugWindow()
	}
}

// Unregister removes a window. Returns true if the app should
// exit based on ExitMode. Unknown IDs still evaluate the exit
// rule against the remaining set.
//
// The window linkage clears atomically, so App() and PlatformID()
// are safe from any goroutine. When the main window leaves while
// siblings remain, mainID fails over to the oldest survivor (or
// zero when none remain) so menubar and tray calls keep working
// instead of stranding on a dead ID.
func (a *App) Unregister(id uint32) bool {
	if a == nil {
		return false
	}
	a.mu.Lock()
	defer a.mu.Unlock()
	if w := a.windows[id]; w != nil {
		w.app.Store(nil)
		w.platformID.Store(0)
	}
	delete(a.windows, id)
	for i, oid := range a.order {
		if oid == id {
			copy(a.order[i:], a.order[i+1:])
			a.order[len(a.order)-1] = 0
			a.order = a.order[:len(a.order)-1]
			break
		}
	}
	wasMain := id == a.mainID
	if wasMain {
		a.mainID = 0
		if len(a.order) > 0 {
			a.mainID = a.order[0]
		}
	}
	switch a.ExitMode {
	case ExitOnMainClose:
		return wasMain
	case ExitOnTrayRemoved:
		return len(a.windows) == 0 && len(a.trays) == 0
	default:
		return len(a.windows) == 0
	}
}

// Window returns the Window for the given platform ID, or nil.
// Safe to call from any goroutine.
func (a *App) Window(id uint32) *Window {
	if a == nil {
		return nil
	}
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.windows[id]
}

// mainWindow returns the current main window, or nil. Callers
// must not hold a.mu; the lookup is self-contained.
func (a *App) mainWindow() *Window {
	if a == nil {
		return nil
	}
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.windows[a.mainID]
}

// Windows returns all registered windows in creation order.
// exportaudit:keep — collides with the windows map field
func (a *App) Windows() []*Window {
	if a == nil {
		return nil
	}
	a.mu.Lock()
	defer a.mu.Unlock()
	ws := make([]*Window, 0, len(a.order))
	for _, id := range a.order {
		if w, ok := a.windows[id]; ok {
			ws = append(ws, w)
		}
	}
	return ws
}

// OpenWindow queues a new window for creation on the next frame.
// The internal buffer holds up to pendingCap pending requests.
// Requests beyond that are dropped; the warning logs at most
// once per second with a running total, so a burst cannot spam
// stderr. Safe to call from any goroutine.
func (a *App) OpenWindow(cfg WindowCfg) {
	if a == nil {
		return
	}
	select {
	case a.pending <- cfg:
	default:
		n := a.dropped.Add(1)
		now := time.Now().UnixNano()
		last := a.lastDropLog.Load()
		if now-last > dropLogInterval &&
			a.lastDropLog.CompareAndSwap(last, now) {
			log.Printf("gui: App.OpenWindow: pending buffer full, "+
				"window request dropped (%d total)", n)
		}
		return
	}
	// The backend event loop may be blocked in an idle wait that
	// cannot observe the pending channel (issue #405) — wake it so
	// the window is created promptly instead of at the next event.
	a.wakeMu.RLock()
	fn := a.wakeMainFn
	a.wakeMu.RUnlock()
	if fn != nil {
		fn()
	}
}

// SetWakeMainFn sets the function called to wake the main event loop
// when OpenWindow queues a window from another goroutine. The backend
// sets this at init time. Safe to call from any goroutine.
func (a *App) SetWakeMainFn(fn func()) {
	if a == nil {
		return
	}
	a.wakeMu.Lock()
	a.wakeMainFn = fn
	a.wakeMu.Unlock()
}

// PendingOpen returns the channel of window configs to create.
func (a *App) PendingOpen() <-chan WindowCfg {
	if a == nil {
		return nil
	}
	return a.pending
}

// Broadcast calls fn for every registered window. Snapshots the
// window list under lock, then iterates without holding the lock
// so fn may safely call other App methods. A nil fn is a no-op.
// Safe to call from any goroutine, but fn itself runs on the
// caller's goroutine; queue window work with QueueCommand when
// off the main thread.
func (a *App) Broadcast(fn func(*Window)) {
	if a == nil || fn == nil {
		return
	}
	a.mu.Lock()
	windows := make([]*Window, 0, len(a.order))
	for _, id := range a.order {
		if w, ok := a.windows[id]; ok {
			windows = append(windows, w)
		}
	}
	a.mu.Unlock()
	for _, w := range windows {
		fn(w)
	}
}

// SetNativeMenubar installs a native OS menubar. Resolves
// CommandID fields from the main window's command registry
// and routes actions through QueueCommand. Silently does nothing
// when there is no main window or no native platform, so backends
// without menubar support need no guard at the call site.
//
// The action callback resolves the current main window when it
// fires, not when the menubar installs, so a main-window change
// between install and click still reaches the live window. With
// no window alive, a fallback OnAction still runs directly.
func (a *App) SetNativeMenubar(cfg NativeMenubarCfg) {
	if a == nil {
		return
	}
	mainW := a.mainWindow()
	if mainW == nil {
		return
	}
	np := mainW.NativePlatformBackend()
	if np == nil {
		return
	}
	actionCb := func(id string) {
		cur := a.mainWindow()
		if cur == nil {
			if cfg.OnAction != nil {
				cfg.OnAction(id)
			}
			return
		}
		cur.QueueCommand(func(w *Window) {
			if cmd, ok := w.CommandByID(id); ok {
				cmd.Execute(nil, w)
			} else if cfg.OnAction != nil {
				cfg.OnAction(id)
			}
		})
	}
	np.SetNativeMenubar(cfg, actionCb)
}

// ClearNativeMenubar removes the native OS menubar. Silently
// does nothing when there is no main window or no native
// platform.
func (a *App) ClearNativeMenubar() {
	if a == nil {
		return
	}
	mainW := a.mainWindow()
	if mainW == nil {
		return
	}
	np := mainW.NativePlatformBackend()
	if np == nil {
		return
	}
	np.ClearNativeMenubar()
}

// SetSystemTray creates a system tray icon with menu. Unlike the
// silent menubar/tray updaters, creation reports failure: no main
// window, no native platform, a platform error, or a non-positive
// platform tray ID (platforms must return unique positive IDs).
func (a *App) SetSystemTray(
	cfg SystemTrayCfg,
) (*SystemTrayHandle, error) {
	if a == nil {
		return nil, errors.New("gui: no main window")
	}
	mainW := a.mainWindow()
	if mainW == nil {
		return nil, errors.New("gui: no main window")
	}
	np := mainW.NativePlatformBackend()
	if np == nil {
		return nil, errors.New("gui: no native platform")
	}
	actionCb := func(id string) {
		if cfg.OnAction == nil {
			return
		}
		if cur := a.mainWindow(); cur != nil {
			cur.QueueCommand(func(_ *Window) {
				cfg.OnAction(id)
			})
		} else {
			cfg.OnAction(id)
		}
	}
	trayID, err := np.CreateSystemTray(cfg, actionCb)
	if err != nil {
		return nil, err
	}
	if trayID <= 0 {
		return nil, errors.New("gui: invalid tray id")
	}
	h := &SystemTrayHandle{id: trayID}
	a.mu.Lock()
	if a.trays == nil {
		a.trays = make(map[int]*SystemTrayHandle)
	}
	a.trays[trayID] = h
	a.mu.Unlock()
	return h, nil
}

// UpdateSystemTray updates an existing system tray entry.
// Silently does nothing for a nil handle, a missing main window,
// or a missing native platform, so teardown paths need no guard.
// Unknown handle IDs forward to the platform, which ignores them.
func (a *App) UpdateSystemTray(
	h *SystemTrayHandle, cfg SystemTrayCfg,
) {
	if a == nil || h == nil {
		return
	}
	mainW := a.mainWindow()
	if mainW == nil {
		return
	}
	np := mainW.NativePlatformBackend()
	if np == nil {
		return
	}
	np.UpdateSystemTray(h.id, cfg)
}

// RemoveSystemTray removes a system tray icon. The handle leaves
// app tracking even when no window or platform remains to notify,
// so a late remove never leaks the entry; the platform call is
// then skipped. Silently does nothing for a nil handle.
func (a *App) RemoveSystemTray(h *SystemTrayHandle) {
	if a == nil || h == nil {
		return
	}
	a.mu.Lock()
	mainW := a.windows[a.mainID]
	delete(a.trays, h.id)
	a.mu.Unlock()
	if mainW == nil {
		return
	}
	np := mainW.NativePlatformBackend()
	if np == nil {
		return
	}
	np.RemoveSystemTray(h.id)
}
