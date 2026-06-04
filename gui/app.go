package gui

import (
	"errors"
	"log"
	"sync"
)

// ExitMode controls when the application exits.
type ExitMode int

const (
	// ExitOnLastClose exits when the last window is closed.
	ExitOnLastClose ExitMode = iota
	// ExitOnMainClose exits when the main (first) window is closed.
	ExitOnMainClose
	// ExitOnTrayRemoved keeps the app alive while a system tray
	// icon exists, even if all windows are closed.
	ExitOnTrayRemoved
)

// App manages multiple windows in a single application.
type App struct {
	windows  map[uint32]*Window
	pending  chan WindowCfg
	trays    map[int]*SystemTrayHandle
	order    []uint32
	ExitMode ExitMode
	mu       sync.Mutex
	mainID   uint32
}

// NewApp creates an App with an empty window registry.
func NewApp() *App {
	return &App{
		windows: make(map[uint32]*Window),
		pending: make(chan WindowCfg, 16),
	}
}

// Register associates a platform window ID with a Window.
// Duplicate IDs are ignored with a warning.
func (a *App) Register(id uint32, w *Window) {
	a.mu.Lock()
	defer a.mu.Unlock()
	if _, exists := a.windows[id]; exists {
		log.Printf("gui: App.Register: duplicate window ID %d ignored", id)
		return
	}
	a.windows[id] = w
	a.order = append(a.order, id)
	w.app = a
	w.platformID = id
	if len(a.order) == 1 {
		a.mainID = id
	}
	if w.Config.DebugTimeTravel {
		// Auto-spawn the scrubber for the newly registered
		// window. OpenDebugWindow→App.OpenWindow is a
		// non-blocking channel send on a.pending, which is
		// independent of a.mu, so it's safe to call here.
		w.OpenDebugWindow()
	}
}

// Unregister removes a window. Returns true if the app should
// exit based on ExitMode.
//
// Clears w.app and w.platformID without holding Window.mu. This is
// safe because Unregister runs on the main thread, which is also the
// only thread that reads these fields.
func (a *App) Unregister(id uint32) bool {
	a.mu.Lock()
	defer a.mu.Unlock()
	w := a.windows[id]
	if w != nil {
		w.app = nil
		w.platformID = 0
	}
	delete(a.windows, id)
	for i, oid := range a.order {
		if oid == id {
			a.order = append(a.order[:i], a.order[i+1:]...)
			break
		}
	}
	switch a.ExitMode {
	case ExitOnMainClose:
		return id == a.mainID
	case ExitOnTrayRemoved:
		return len(a.windows) == 0 && len(a.trays) == 0
	default:
		return len(a.windows) == 0
	}
}

// Window returns the Window for the given platform ID, or nil.
func (a *App) Window(id uint32) *Window {
	a.mu.Lock()
	defer a.mu.Unlock()
	return a.windows[id]
}

// Windows returns all registered windows in creation order.
func (a *App) Windows() []*Window {
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
// The internal buffer holds up to 16 pending requests. Requests
// beyond that are dropped with a log warning.
func (a *App) OpenWindow(cfg WindowCfg) {
	select {
	case a.pending <- cfg:
	default:
		log.Println("gui: App.OpenWindow: pending buffer full, " +
			"window request dropped")
	}
}

// PendingOpen returns the channel of window configs to create.
func (a *App) PendingOpen() <-chan WindowCfg {
	return a.pending
}

// Broadcast calls fn for every registered window. Snapshots the
// window list under lock, then iterates without holding the lock
// so fn may safely call other App methods.
func (a *App) Broadcast(fn func(*Window)) {
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
// and routes actions through QueueCommand.
func (a *App) SetNativeMenubar(cfg NativeMenubarCfg) {
	a.mu.Lock()
	mainW := a.windows[a.mainID]
	a.mu.Unlock()
	if mainW == nil {
		return
	}
	np := mainW.NativePlatformBackend()
	if np == nil {
		return
	}
	actionCb := func(id string) {
		mainW.QueueCommand(func(w *Window) {
			if cmd, ok := w.CommandByID(id); ok {
				cmd.Execute(nil, w)
			} else if cfg.OnAction != nil {
				cfg.OnAction(id)
			}
		})
	}
	np.SetNativeMenubar(cfg, actionCb)
}

// ClearNativeMenubar removes the native OS menubar.
func (a *App) ClearNativeMenubar() {
	a.mu.Lock()
	mainW := a.windows[a.mainID]
	a.mu.Unlock()
	if mainW == nil {
		return
	}
	np := mainW.NativePlatformBackend()
	if np == nil {
		return
	}
	np.ClearNativeMenubar()
}

// SetSystemTray creates a system tray icon with menu.
func (a *App) SetSystemTray(
	cfg SystemTrayCfg,
) (*SystemTrayHandle, error) {
	a.mu.Lock()
	mainW := a.windows[a.mainID]
	a.mu.Unlock()
	if mainW == nil {
		return nil, errors.New("gui: no main window")
	}
	np := mainW.NativePlatformBackend()
	if np == nil {
		return nil, errors.New("gui: no native platform")
	}
	actionCb := func(id string) {
		if cfg.OnAction != nil {
			mainW.QueueCommand(func(_ *Window) {
				cfg.OnAction(id)
			})
		}
	}
	trayID, err := np.CreateSystemTray(cfg, actionCb)
	if err != nil {
		return nil, err
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
func (a *App) UpdateSystemTray(
	h *SystemTrayHandle, cfg SystemTrayCfg,
) {
	if h == nil {
		return
	}
	a.mu.Lock()
	mainW := a.windows[a.mainID]
	a.mu.Unlock()
	if mainW == nil {
		return
	}
	np := mainW.NativePlatformBackend()
	if np == nil {
		return
	}
	np.UpdateSystemTray(h.id, cfg)
}

// RemoveSystemTray removes a system tray icon.
func (a *App) RemoveSystemTray(h *SystemTrayHandle) {
	if h == nil {
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
