package gui

import "sync"

// Window-scoped theme installation.
//
// A window owns its theme; the frame pass installs that theme into the
// package-level style state (guiTheme + the default*Style mirrors) before
// the view function runs. Widget factories resolve their defaults at
// construction time, and construction happens inside the frame pass of the
// window being generated, so a factory-time read always sees the right
// window's theme.
//
// The invariant this rests on: every window's frame pass runs on the one
// main OS thread. Both desktop backends lock the thread in package init
// and drive all windows from a single sequential loop, so two windows can
// never generate layouts concurrently.

// liveWindows tracks windows that follow the app-default theme so
// SetTheme can request a repaint for them. Registration happens in
// NewWindow; removal in WindowCleanup.
var (
	liveWindowsMu sync.Mutex
	liveWindows   []*Window
)

// registerWindow adds w to the live-window set.
func registerWindow(w *Window) {
	liveWindowsMu.Lock()
	liveWindows = append(liveWindows, w)
	liveWindowsMu.Unlock()
}

// unregisterWindow drops w from the live-window set.
func unregisterWindow(w *Window) {
	liveWindowsMu.Lock()
	for i := range liveWindows {
		if liveWindows[i] == w {
			liveWindows = append(liveWindows[:i], liveWindows[i+1:]...)
			break
		}
	}
	liveWindowsMu.Unlock()
}

// appUpdateWindows requests a rebuild on every window that follows the
// app-default theme. Called after the default changes. The refresh flag
// is frame-thread state, so the request goes through the command queue
// rather than being written from the caller's goroutine.
func appUpdateWindows() {
	liveWindowsMu.Lock()
	ws := make([]*Window, len(liveWindows))
	copy(ws, liveWindows)
	liveWindowsMu.Unlock()

	for _, w := range ws {
		if w.themePinned() {
			continue
		}
		w.QueueCommand(func(win *Window) { win.markLayoutRefresh() })
	}
}

// themePinned reports whether the window set its own theme.
func (w *Window) themePinned() bool {
	w.themeMu.RLock()
	defer w.themeMu.RUnlock()
	return w.themeSet
}

// Theme returns the window's active theme: the one set with SetTheme, or
// the app default when the window has not pinned one.
func (w *Window) Theme() Theme {
	if w == nil {
		return currentDefaultTheme()
	}
	w.themeMu.RLock()
	pinned, t := w.themeSet, w.theme
	w.themeMu.RUnlock()
	if pinned {
		return t
	}
	return currentDefaultTheme()
}

// SetTheme pins t as this window's theme and requests a rebuild. Other
// windows keep theirs. The install happens at the start of the next
// frame, which the requested rebuild guarantees.
func (w *Window) SetTheme(t Theme) {
	w.themeMu.Lock()
	w.theme = t
	w.themeSet = true
	w.themeMu.Unlock()
	// Installed eagerly for the same reason package SetTheme does it:
	// the common caller is an event handler on the frame thread, and
	// code running after it in that frame should already see t.
	applyTheme(t)
	w.UpdateWindow()
}

// needsInstall reports whether t must be (re-)installed: its id is
// unset (built outside ThemeMaker, so it can never match the fast
// path) or it is not the theme currently installed.
func needsInstall(t Theme) bool {
	return t.id == 0 || t.id != installedThemeID
}

// installTheme makes this window's theme the active one for the frame.
// Frame-thread only; a no-op when the same theme is already installed,
// which is the steady state for a single-window app.
func (w *Window) installTheme() {
	t := w.Theme()
	if !needsInstall(t) {
		return
	}
	applyTheme(t)
}

// pushTheme installs t and returns the theme it displaced, for a later
// popTheme. Frame-thread only; used to scope a theme to a subtree.
func pushTheme(t Theme) Theme {
	prev := CurrentTheme()
	if needsInstall(t) {
		applyTheme(t)
	}
	return prev
}

// popTheme restores the theme a matching pushTheme displaced.
func popTheme(prev Theme) {
	if !needsInstall(prev) {
		return
	}
	applyTheme(prev)
}
