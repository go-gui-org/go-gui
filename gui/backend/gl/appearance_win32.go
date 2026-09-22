//go:build windows && !js

package gl

import (
	"sync"

	"golang.org/x/sys/windows/registry"

	"github.com/go-gui-org/go-gui/gui"
)

// OS light/dark setting on Windows (issue #752). The value lives in
// the registry; flips arrive as WM_SETTINGCHANGE, hooked in
// handleMessage (events_win32.go). The watcher is free: Windows
// delivers the broadcast to windows that already exist.

var (
	winAppearanceMu    sync.Mutex
	winAppearanceCBs   = make(map[*nativePlatform]func(gui.Appearance))
	winAppearanceLast  = gui.AppearanceLight
	winAppearanceKnown = false
)

// personalizeKey is the theme key holding AppsUseLightTheme.
const personalizeKey = `Software\Microsoft\Windows\CurrentVersion\Themes\Personalize`

// querySystemAppearance reads AppsUseLightTheme: 0 is dark, anything
// else is light. A missing key or value reads as no setting, so the
// app keeps its own theme.
func querySystemAppearance() (gui.Appearance, bool) {
	k, err := registry.OpenKey(registry.CURRENT_USER, personalizeKey, registry.QUERY_VALUE)
	if err != nil {
		return gui.AppearanceLight, false
	}
	defer func() { _ = k.Close() }()
	v, _, err := k.GetIntegerValue("AppsUseLightTheme")
	if err != nil {
		return gui.AppearanceLight, false
	}
	if v == 0 {
		return gui.AppearanceDark, true
	}
	return gui.AppearanceLight, true
}

// SystemAppearance implements gui.NativePlatform.
func (n *nativePlatform) SystemAppearance() (gui.Appearance, bool) {
	return querySystemAppearance()
}

// winAppearanceQuery reads the OS setting. A var so tests can feed
// a reading without touching the registry.
var winAppearanceQuery = querySystemAppearance

// SetSystemAppearanceCallback implements gui.NativePlatform. A nil cb
// unregisters.
func (n *nativePlatform) SetSystemAppearanceCallback(cb func(gui.Appearance)) {
	if cb != nil {
		// Seed the last-known value before the first broadcast.
		// Without it, the first WM_SETTINGCHANGE of any kind (a
		// wallpaper or locale change) reads as a flip and re-pins
		// the theme and runs the user hook with no real change.
		// The registry read runs outside the lock.
		seedSystemAppearance(winAppearanceQuery())
	}
	winAppearanceMu.Lock()
	defer winAppearanceMu.Unlock()
	if cb == nil {
		delete(winAppearanceCBs, n)
	} else {
		winAppearanceCBs[n] = cb
	}
}

// systemAppearanceChanged runs on the WndProc thread after
// WM_SETTINGCHANGE. It re-queries and fans out only on a real flip:
// the broadcast fires for every setting change, and re-pinning on
// each would rebuild layouts for nothing.
func systemAppearanceChanged() {
	a, ok := winAppearanceQuery()
	noteSystemAppearance(a, ok)
}

// seedSystemAppearance records a reading as the last-known value
// when none is known yet. It never fans out: a seed is not a change.
func seedSystemAppearance(a gui.Appearance, ok bool) {
	if !ok {
		return
	}
	winAppearanceMu.Lock()
	if !winAppearanceKnown {
		winAppearanceKnown, winAppearanceLast = true, a
	}
	winAppearanceMu.Unlock()
}

// noteSystemAppearance records a fresh reading and fans out to
// subscribers only when it flips. Split from systemAppearanceChanged
// so tests can feed readings without touching the registry.
func noteSystemAppearance(a gui.Appearance, ok bool) {
	if !ok {
		return
	}
	winAppearanceMu.Lock()
	if winAppearanceKnown && a == winAppearanceLast {
		winAppearanceMu.Unlock()
		return
	}
	winAppearanceKnown, winAppearanceLast = true, a
	cbs := make([]func(gui.Appearance), 0, len(winAppearanceCBs))
	for _, cb := range winAppearanceCBs {
		cbs = append(cbs, cb)
	}
	winAppearanceMu.Unlock()
	for _, cb := range cbs {
		cb(a)
	}
}
