//go:build android

package android

import (
	"sync"

	"github.com/go-gui-org/go-gui/gui"
)

// OS light/dark setting on Android (issue #752). The Kotlin host
// owns the configuration: it pushes the current uiMode at startup
// and every onConfigurationChanged through AppearanceChanged (see
// the android_demo host for the wiring). Go keeps the last pushed
// value; queries before the first push report no setting, and the
// app keeps its own theme.

var (
	appearanceMu        sync.Mutex
	appearanceCallbacks = make(map[*nativePlatform]func(gui.Appearance))
	appearanceLast      = gui.AppearanceLight
	appearanceKnown     = false
)

// SystemAppearance returns the last uiMode Kotlin pushed. The
// second result is false until the first push.
func (n *nativePlatform) SystemAppearance() (gui.Appearance, bool) {
	appearanceMu.Lock()
	defer appearanceMu.Unlock()
	return appearanceLast, appearanceKnown
}

// SetSystemAppearanceCallback registers cb for uiMode pushes. A nil
// cb unregisters. No OS watcher to manage: Kotlin drives.
func (n *nativePlatform) SetSystemAppearanceCallback(cb func(gui.Appearance)) {
	appearanceMu.Lock()
	defer appearanceMu.Unlock()
	if cb == nil {
		delete(appearanceCallbacks, n)
	} else {
		appearanceCallbacks[n] = cb
	}
}

// AppearanceChanged records a uiMode push from Kotlin and fans out
// to subscribers. Called once at startup with the current mode and
// again on every change.
func AppearanceChanged(dark bool) {
	a := gui.AppearanceLight
	if dark {
		a = gui.AppearanceDark
	}
	appearanceMu.Lock()
	appearanceLast, appearanceKnown = a, true
	cbs := make([]func(gui.Appearance), 0, len(appearanceCallbacks))
	for _, cb := range appearanceCallbacks {
		cbs = append(cbs, cb)
	}
	appearanceMu.Unlock()
	for _, cb := range cbs {
		cb(a)
	}
}
