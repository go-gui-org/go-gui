package gui

// System light/dark appearance (issue #752).
//
// The OS owns the setting; the app only reads it. A window follows
// the setting with FollowSystemAppearance, or reads it manually
// with SystemAppearance and OnSystemAppearance for custom logic.

// Appearance is the OS light/dark setting.
type Appearance int

const (
	// AppearanceLight is the OS light mode.
	AppearanceLight Appearance = iota
	// AppearanceDark is the OS dark mode.
	AppearanceDark
)

// SystemAppearance queries the OS light/dark setting. The second
// result is false when the OS reports no setting (nil platform in
// tests, a backend whose OS has none, a missing Linux schema), in
// which case the app keeps its own theme.
func (w *Window) SystemAppearance() (Appearance, bool) {
	np := w.NativePlatformBackend()
	if np == nil {
		return AppearanceLight, false
	}
	return np.SystemAppearance()
}

// OnSystemAppearance registers cb for OS appearance changes. The
// callback runs on the frame thread with the new setting. Replaces
// any previous callback; nil unregisters. Independent of
// FollowSystemAppearance: a window can follow the setting and still
// get the callback (applied first, callback second).
//
// cb also runs with the current setting, not only on a change, each
// time a following window applies it: at FollowSystemAppearance and
// when a backend attaches. Do not treat a call as proof of a flip.
// exportaudit:keep — documented app API (issue #752); siblings adopt post-release
func (w *Window) OnSystemAppearance(cb func(Appearance)) {
	w.appearanceMu.Lock()
	w.appearanceHook = cb
	w.appearanceMu.Unlock()
	w.refreshAppearanceSubscription()
}

// FollowSystemAppearance pins light or dark as this window's theme
// from the current OS setting and re-pins on every OS change, until
// an explicit SetTheme ends following. Each apply also calls
// TitlebarDark, which no backend implements yet, so the titlebar
// does not follow today.
//
// Frame-thread only, like SetTheme: call from main before Run or
// from an event handler. With no OS setting (nil platform, unknown
// desktop) the window keeps its theme but stays following, so a
// later SetNativePlatform applies the setting on attach.
// exportaudit:keep — documented app API (issue #752); siblings adopt post-release
func (w *Window) FollowSystemAppearance(light, dark Theme) {
	w.appearanceMu.Lock()
	// Publish fresh values rather than aliasing the caller's: a
	// reader may still hold the previous pair.
	l, d := light, dark
	w.appearanceLight = &l
	w.appearanceDark = &d
	w.appearanceFollowing = true
	w.appearanceMu.Unlock()
	w.refreshAppearanceSubscription()
	if a, ok := w.SystemAppearance(); ok {
		w.applySystemAppearance(a)
	}
}

// StopFollowSystemAppearance ends OS following without changing the
// current theme. A later FollowSystemAppearance starts it again.
// exportaudit:keep — documented app API (issue #752); siblings adopt post-release
func (w *Window) StopFollowSystemAppearance() {
	w.appearanceMu.Lock()
	w.appearanceFollowing = false
	w.appearanceMu.Unlock()
	w.refreshAppearanceSubscription()
}

// applySystemAppearance pins the theme for setting a and runs the
// user hook. Frame-thread only: backends invoke their watcher
// callback off-thread, and dispatchSystemAppearance marshals it
// here through the command queue.
func (w *Window) applySystemAppearance(a Appearance) {
	w.appearanceMu.RLock()
	following := w.appearanceFollowing
	light, dark := w.appearanceLight, w.appearanceDark
	hook := w.appearanceHook
	w.appearanceMu.RUnlock()
	if following && light != nil && dark != nil {
		if a == AppearanceDark {
			w.pinTheme(*dark)
		} else {
			w.pinTheme(*light)
		}
		w.TitlebarDark(a == AppearanceDark)
	}
	if hook != nil {
		hook(a)
	}
}

// dispatchSystemAppearance is the backend watcher callback. It runs
// on the watcher's thread, so it only queues; the frame thread does
// the work in applySystemAppearance.
func (w *Window) dispatchSystemAppearance(a Appearance) {
	w.QueueCommand(func(win *Window) { win.applySystemAppearance(a) })
}

// refreshAppearanceSubscription registers the watcher callback when
// the window needs OS change events (following, a user hook, or
// both) and unregisters otherwise. Backends start and stop their
// watcher on this call. Nil-safe: before a backend attaches there
// is nothing to register with; SetNativePlatform replays.
func (w *Window) refreshAppearanceSubscription() {
	np := w.NativePlatformBackend()
	if np == nil {
		return
	}
	w.appearanceMu.RLock()
	active := w.appearanceFollowing || w.appearanceHook != nil
	w.appearanceMu.RUnlock()
	if active {
		np.SetSystemAppearanceCallback(w.dispatchSystemAppearance)
	} else {
		np.SetSystemAppearanceCallback(nil)
	}
}
