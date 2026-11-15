package gui

// window_visibility.go — OS-level window show/hide (issue #779).
//
// Hide removes the window from the screen without destroying it: the
// Window stays registered with its App, keeps framing, and keeps its
// state, so a system-tray menu can bring it back with Show. Close, by
// contrast, marks the window for destruction on the next frame. Pair
// Hide with an OnCloseRequest hook that hides instead of closing to
// keep a tray app alive with no window on screen.

// IsVisible reports whether the app shows the window. True until Hide,
// false after Hide until Show, and false once the backend destroys
// the window. It does not know what the OS does: a minimized window,
// or an app in the background, is still visible here. IsOccluded
// reports that part. A lock-free atomic read, safe from any goroutine.
func (w *Window) IsVisible() bool {
	if w == nil || w.destroyed.Load() {
		return false
	}
	return !w.hidden.Load()
}

// IsOccluded reports whether the OS shows the window nowhere: it is
// minimized, or the app is in the background (issue #1170). The user
// sees the window when IsVisible() && !IsOccluded(), and
// EventVisibilityChanged reports each change of that. A lock-free
// atomic read, safe from any goroutine. Nil-safe: false.
//
// What counts differs by platform:
//
//   - Windows: minimized. A covered window, the lock screen and
//     another virtual desktop are not reported.
//   - macOS: minimized, fully covered, or on another space.
//   - X11: minimized. A covered window is not reported.
//   - Wayland: minimized, on another workspace, or behind a locked
//     screen. A fully covered window is reported about a second late.
//   - Android: the activity is stopped. In multi-window mode a paused
//     activity is still on screen, so it is not occluded.
//   - iOS: the app is in the background. Control Center and a system
//     alert do not count.
//   - Web: the page is hidden (another tab, or a minimized browser).
//     Whether a covered browser window counts depends on the browser.
func (w *Window) IsOccluded() bool {
	if w == nil {
		return false
	}
	return w.occluded.Load()
}

// reportVisibility sends EventVisibilityChanged when the combined
// state, IsVisible() && !IsOccluded(), differs from the one last
// reported. Hide, Show and DispatchWindowOccluded all end here, so a
// Hide that the OS also reports as an occlusion (X11, macOS) sends one
// event, like a Hide that it does not (Win32).
//
// The event goes out now, on the caller's thread, not through the
// command queue: on Android, iOS and web the frame loop stops in the
// background, and a queued command would run when the app comes back.
// It is a new Event, not a backend's shared one: Hide inside a click
// handler can get here while that event is still being handled
// (the #1127 hazard). UI thread only.
//
// While the time-travel debugger has the window frozen, EventFn drops
// every event. The change is then not recorded either, so the report
// that resume queues still finds it and sends it.
//
// It reports whether the window became shown, for Show to ask for the
// frame that draws what the handler changed.
func (w *Window) reportVisibility() (shown bool) {
	if w.destroyed.Load() || w.frozen.Load() {
		return false
	}
	notShown := w.hidden.Load() || w.occluded.Load()
	if w.notShown.Swap(notShown) == notShown {
		return false
	}
	e := Event{Type: EventVisibilityChanged}
	w.EventFn(&e)
	return !notShown
}

// Hide removes the window from the screen without destroying it. The
// window stays registered, so App.Windows still lists it and, with
// ExitOnTrayRemoved, the app stays alive with no window showing. Sends
// EventVisibilityChanged when the window was shown before.
//
// Hide may be called before the backend starts, for an app that begins
// with no window on screen, such as a tray app: the window is created
// but not shown, and Show brings it up (issue #1270). In OnInit the
// window is already on screen, so a Hide there takes it away again.
//
// Where no window can be hidden (web, iOS, Android, native Wayland),
// Hide does nothing and IsVisible stays true: the user still
// sees the app. DebugWindowDegraded reports the call. No-op
// on a destroyed window, and safe on a nil window (also a no-op).
// Call from the UI thread, or from a Window.QueueCommand like the
// other window setters — the tray OnAction already runs there when a
// window is alive (see App.SetSystemTray).
func (w *Window) Hide() {
	if w == nil || w.destroyed.Load() {
		return
	}
	if !w.canHide() {
		w.debugWarn(debugCheckWindowHide, "hide",
			"window: Hide requested but the platform cannot hide a window")
		return
	}
	w.hidden.Store(true)
	if w.nativePlatform != nil {
		w.nativePlatform.HideWindow()
	}
	w.reportVisibility()
}

// canHide reports whether the platform can take the window off the
// screen. A platform that cannot says so with CanHideWindow; one that
// does not have the method can (every desktop backend, and a nil
// platform in tests).
func (w *Window) canHide() bool {
	h, ok := w.nativePlatform.(interface{ CanHideWindow() bool })
	return !ok || h.CanHideWindow()
}

// reconcileHidden runs when the backend attaches. A Hide made before
// that found no platform to ask, so it marked the window hidden. On a
// platform that cannot hide a window the mark is wrong: the user sees
// the app. It is cleared, so IsVisible is true again, and the app is
// told through EventVisibilityChanged.
//
// Only the mark is put right. A Minimize, Maximize or SetFullscreen
// made while the window was marked hidden was a no-op, as it is on any
// hidden window, and stays one. On a platform that can hide, the mark
// stands: the backend reads it when it creates the window and does not
// show it (issue #1270).
func (w *Window) reconcileHidden() {
	if w.nativePlatform == nil || !w.hidden.Load() || w.canHide() {
		return
	}
	w.debugWarn(debugCheckWindowHide, "hide",
		"window: Hide requested but the platform cannot hide a window")
	w.hidden.Store(false)
	w.reportVisibility()
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
	// The handler may have changed what the window shows, and so may
	// the one that ran on Hide: draw it now that someone can see it.
	if w.reportVisibility() {
		w.markLayoutRefresh(refreshShown)
		w.wakeMain()
	}
}
