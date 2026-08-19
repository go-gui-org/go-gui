package gui

// Test helpers for driving the theme values that post-generation code
// reads through w.Theme() (issue #301). Before that migration these
// tests wrote guiTheme directly; that no longer reaches the code under
// test, and it leaked across tests because guiTheme is process-wide.

// pinTheme pins a mutated copy of w's active theme on w. Window-scoped,
// so no restore is needed. Deliberately does not go through
// (*Window).SetTheme: that also calls applyTheme and UpdateWindow, which
// a bare test Window does not need.
//
// The copy keeps its Theme.id, so needsInstall treats it as already
// installed and the default*Style mirrors are NOT refreshed from it. That
// is correct for code reading w.Theme(); do not use this helper to drive
// a test that asserts on a default*Style var.
func pinTheme(w *Window, mutate func(*Theme)) {
	th := w.Theme()
	mutate(&th)
	w.themeMu.Lock()
	w.theme = &th
	w.themeSet = true
	w.themeMu.Unlock()
}

// pinScrollMultiplier pins mult as w's scroll multiplier, the knob the
// scroll paths read per event. Tests use 1 so a delta maps 1:1 to an
// offset.
func pinScrollMultiplier(w *Window, mult float32) {
	pinTheme(w, func(th *Theme) { th.ScrollMultiplier = mult })
}
