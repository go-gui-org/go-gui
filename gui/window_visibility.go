package gui

// window_visibility.go — OS-level window show/hide (issue #779).
//
// Hide removes the window from the screen without destroying it: the
// Window stays registered with its App, keeps framing, and keeps its
// state, so a system-tray menu can bring it back with Show. Close, by
// contrast, marks the window for destruction on the next frame. Pair
// Hide with an OnCloseRequest hook that hides instead of closing to
// keep a tray app alive with no window on screen.

// IsVisible reports whether the window is on screen. True until Hide,
// false after Hide until Show, and false once the backend destroys
// the window. A lock-free atomic read, safe from any goroutine.
func (w *Window) IsVisible() bool {
	if w == nil || w.destroyed.Load() {
		return false
	}
	return !w.hidden.Load()
}

// Hide removes the window from the screen without destroying it. The
// window stays registered, so App.Windows still lists it and, with
// ExitOnTrayRemoved, the app stays alive with no window showing. No-op
// on a destroyed window, and safe on a nil window (also a no-op).
// Call from the UI thread, or from a Window.QueueCommand like the
// other window setters — the tray OnAction already runs there when a
// window is alive (see App.SetSystemTray).
func (w *Window) Hide() {
	if w == nil || w.destroyed.Load() {
		return
	}
	w.hidden.Store(true)
	if w.nativePlatform != nil {
		w.nativePlatform.HideWindow()
	}
}

// Show brings the window back on screen after Hide, raising it above
// its siblings and activating it. Calling Show on an already visible
// window still raises it, so a tray "Show Window" item doubles as
// bring-to-front. No-op on a destroyed window — a closed window is
// gone and cannot come back (issue #779); open a new one with
// App.OpenWindow instead. Nil-safe like Hide.
//
// Same thread rule as Hide: UI thread or Window.QueueCommand.
func (w *Window) Show() {
	if w == nil || w.destroyed.Load() {
		return
	}
	w.hidden.Store(false)
	if w.nativePlatform != nil {
		w.nativePlatform.ShowWindow()
	}
}
