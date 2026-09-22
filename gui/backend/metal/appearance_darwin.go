//go:build darwin && !ios

package metal

/*
#include <stdlib.h>
#include "appearance_darwin.h"
*/
import "C"

import (
	"sync"

	"github.com/go-gui-org/go-gui/gui"
)

// appearanceCallbacks routes OS appearance changes to the windows
// that registered: the setting is app-wide (NSApp), but following
// is per window, so one observer fans out to many callbacks.
// Guarded by appearanceMu; the KVO notification runs on the main
// thread and each callback marshals to the frame thread itself.
var (
	appearanceMu        sync.Mutex
	appearanceCallbacks = make(map[*nativePlatform]func(gui.Appearance))
)

// SystemAppearance reads NSApp.effectiveAppearance (issue #752).
func (n *nativePlatform) SystemAppearance() (gui.Appearance, bool) {
	if C.metalSystemAppearanceDark() != 0 {
		return gui.AppearanceDark, true
	}
	return gui.AppearanceLight, true
}

// SetSystemAppearanceCallback registers cb for OS appearance
// changes. The KVO observer runs while at least one window is
// registered; the last unregister stops it. A nil cb unregisters.
//
// The register-then-start decision is one critical section: the C
// call runs under the mutex so a concurrent unregister cannot stop
// an observer a new subscriber needs. The C side never re-enters Go
// synchronously (KVO delivers later, on the main thread), so this
// cannot deadlock.
func (n *nativePlatform) SetSystemAppearanceCallback(cb func(gui.Appearance)) {
	appearanceMu.Lock()
	defer appearanceMu.Unlock()
	if cb == nil {
		delete(appearanceCallbacks, n)
	} else {
		appearanceCallbacks[n] = cb
	}
	// Main-thread only, like every NSApp call: SetTheme pins,
	// FollowSystemAppearance and event handlers run on the frame
	// thread, and WindowCleanup runs on the main loop.
	if len(appearanceCallbacks) > 0 {
		C.metalAppearanceWatchStart()
	} else {
		C.metalAppearanceWatchStop()
	}
}

//export goMetalAppearanceChanged
func goMetalAppearanceChanged(dark C.int) {
	appearanceMu.Lock()
	cbs := make([]func(gui.Appearance), 0, len(appearanceCallbacks))
	for _, cb := range appearanceCallbacks {
		cbs = append(cbs, cb)
	}
	appearanceMu.Unlock()
	a := gui.AppearanceLight
	if dark != 0 {
		a = gui.AppearanceDark
	}
	for _, cb := range cbs {
		cb(a)
	}
}
