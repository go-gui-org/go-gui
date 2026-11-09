package gui

import "sync/atomic"

// window_placement.go — OS window state and placement (issues #1171,
// #1220, docs/specs/window-state.md).
//
// Every call here has the thread rule of Hide and Show: UI thread or
// Window.QueueCommand. The native side applies a request later, so the
// getters change once the OS confirms it. A view reading them redraws
// then, with no event to subscribe to.

// windowPlacement is the window's on-screen state, embedded in Window.
type windowPlacement struct {
	// hidden is true after Hide until Show. Atomic: IsVisible reads
	// lock-free from any goroutine while Hide/Show write on the
	// frame thread. The zero value reads as visible.
	hidden atomic.Bool
	// maximized and fullscreen are what the OS last confirmed, through
	// DispatchWindowState. Atomic for IsMaximized and IsFullscreen, as
	// hidden is for IsVisible.
	maximized  atomic.Bool
	fullscreen atomic.Bool
	// safeArea is the space the OS keeps along each edge, as the
	// backend last reported it through DispatchSafeArea; nil until a
	// report. Atomic because backends report from their own thread.
	safeArea atomic.Pointer[Padding]
	// pendingPlacement holds placement calls made before the backend
	// attached. Frame thread only.
	pendingPlacement placementRequest
}

// placementRequest records placement calls made before the backend
// attached; SetNativePlatform replays them. The last call of each kind
// wins, so it stays one fixed-size value however often an app calls.
type placementRequest struct {
	width, height int
	x, y          int
	moved         bool
	minimize      bool
	maximize      bool
	fullscreen    bool
}

// Minimize asks the OS to minimize the window. No-op on a hidden or
// destroyed window, and on a platform with no window manager to ask
// (web, iOS, Android). Nil-safe.
func (w *Window) Minimize() {
	if !w.placeable() {
		return
	}
	if w.nativePlatform == nil {
		w.pendingPlacement.minimize = true
		return
	}
	w.nativePlatform.MinimizeWindow()
}

// Maximize asks the OS to maximize the window. No-op on a FixedSize
// window, and where Minimize is. On macOS this zooms the window. A
// MaxWidth or MaxHeight caps the result on Windows and macOS.
func (w *Window) Maximize() {
	if !w.placeable() || w.Config.FixedSize {
		return
	}
	if w.nativePlatform == nil {
		w.pendingPlacement.maximize = true
		return
	}
	w.nativePlatform.MaximizeWindow()
}

// Restore undoes Minimize, then Maximize: a minimized window returns to
// the state it was minimized from, and a maximized one to its normal
// size. It leaves fullscreen alone; SetFullscreen(false) ends that.
// No-op where Minimize is. Wayland has no request to bring back a
// minimized window, so there it only undoes Maximize.
func (w *Window) Restore() {
	if !w.placeable() {
		return
	}
	if w.nativePlatform == nil {
		w.pendingPlacement.minimize = false
		w.pendingPlacement.maximize = false
		return
	}
	w.nativePlatform.RestoreWindow()
}

// IsMaximized reports whether the OS last confirmed the window
// maximized. It follows the user's maximize button too. It stays true
// through fullscreen for a window that was maximized before it, and
// stays false until the OS answers a Maximize. A lock-free atomic read,
// safe from any goroutine.
func (w *Window) IsMaximized() bool {
	return w != nil && w.maximized.Load()
}

// SetFullscreen enters or leaves fullscreen. MaxWidth and MaxHeight do
// not apply to a fullscreen window. No-op on a hidden or destroyed
// window, and on Android today. Nil-safe.
//
// On the web the whole page goes fullscreen. A browser allows that
// only right after the user's click, tap or key press, so call it from
// that handler: elsewhere the browser refuses, and IsFullscreen stays
// false. iPhone Safari has no fullscreen for a page.
//
// On iOS it hides the status bar and lets the home indicator fade out;
// the view keeps its size, so no EventResized follows. IsFullscreen is
// true once the window fills the screen, which a windowed iPad does
// not. It does nothing for an app whose Swift host owns the view
// controller.
func (w *Window) SetFullscreen(fullscreen bool) {
	if !w.placeable() {
		return
	}
	if w.nativePlatform == nil {
		w.pendingPlacement.fullscreen = fullscreen
		return
	}
	w.nativePlatform.SetWindowFullscreen(fullscreen)
}

// IsFullscreen reports whether the OS last confirmed the window
// fullscreen, including a change the user made through the OS. False
// until the OS answers a SetFullscreen, which on macOS is after the
// animation. A lock-free atomic read, safe from any goroutine.
func (w *Window) IsFullscreen() bool {
	return w != nil && w.fullscreen.Load()
}

// SetWindowSize resizes the window's content area, in the logical
// pixels WindowSize reports, kept within the window's size limits.
// EventResized follows once the OS has applied it. No-op for a size
// below one pixel, on a FixedSize window, while maximized or
// fullscreen, on a destroyed window, and on web, iOS and Android.
// Nil-safe.
func (w *Window) SetWindowSize(width, height int) {
	if w == nil || w.destroyed.Load() || w.Config.FixedSize ||
		width < 1 || height < 1 ||
		w.maximized.Load() || w.fullscreen.Load() {
		return
	}
	width, height = WindowSizeLimits(w.Config).clamp(width, height)
	if w.nativePlatform == nil {
		w.pendingPlacement.width = width
		w.pendingPlacement.height = height
		return
	}
	w.nativePlatform.ResizeWindow(width, height)
}

// Position returns the top-left corner of the window's content area in
// OS desktop units: physical pixels on Windows and X11, points on
// macOS. These are not the logical pixels WindowSize reports, because
// no single scale holds across monitors of different DPI. ok is false
// where the OS does not say: on Wayland, web, iOS and Android, on a
// minimized window, and before the backend attaches. Nil-safe.
func (w *Window) Position() (x, y int, ok bool) {
	if w == nil || w.destroyed.Load() || w.nativePlatform == nil {
		return 0, 0, false
	}
	return w.nativePlatform.WindowPosition()
}

// SetPosition moves the window so the content area's top-left corner
// lands at x, y, in the units Position returns, so
// SetPosition(Position()) moves nothing. No-op while maximized or
// fullscreen, on a destroyed window, and where Position has no answer:
// a Wayland client cannot place its window. Nil-safe.
func (w *Window) SetPosition(x, y int) {
	if w == nil || w.destroyed.Load() ||
		w.maximized.Load() || w.fullscreen.Load() {
		return
	}
	if w.nativePlatform == nil {
		p := &w.pendingPlacement
		p.x, p.y, p.moved = x, y, true
		return
	}
	w.nativePlatform.MoveWindow(x, y)
}

// DispatchWindowState reports what the OS says the window is now,
// whoever changed it. On a change the window asks for a full frame, so
// a view reading the getters updates even when no EventResized comes.
// Repeated reports of the same state are no-ops.
//
// Intended for backend use. Safe for a nil window (no-op).
func DispatchWindowState(w *Window, maximized, fullscreen bool) {
	if w == nil {
		return
	}
	changed := w.maximized.Swap(maximized) != maximized
	if w.fullscreen.Swap(fullscreen) != fullscreen {
		changed = true
	}
	if changed {
		w.markLayoutRefresh(refreshWindowState)
		w.wakeMain()
	}
}

// placeable reports whether a state request can go ahead. A hidden
// window refuses: Windows would show it behind the app's Hide.
func (w *Window) placeable() bool {
	return w != nil && !w.destroyed.Load() && !w.hidden.Load()
}

// replayPlacement applies what was asked for before the backend
// attached. Size and position come first, so they are the normal rect
// that maximize and fullscreen return to.
func (w *Window) replayPlacement() {
	p := w.pendingPlacement
	np := w.nativePlatform
	if np == nil || p == (placementRequest{}) {
		return
	}
	w.pendingPlacement = placementRequest{}
	if p.width > 0 {
		np.ResizeWindow(p.width, p.height)
	}
	if p.moved {
		np.MoveWindow(p.x, p.y)
	}
	if p.maximize {
		np.MaximizeWindow()
	}
	if p.fullscreen {
		np.SetWindowFullscreen(true)
	}
	if p.minimize {
		np.MinimizeWindow()
	}
}
