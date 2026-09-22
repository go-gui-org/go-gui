//go:build ios

package ios

/*
#include <stdlib.h>
#include "ios_app.h"
*/
import "C"

import (
	"sync"

	"github.com/go-gui-org/go-gui/gui"
)

// OS light/dark setting on iOS (issue #752). The view controller
// owns the UITraitCollection: it seeds the C-held current value at
// first layout and reports changes through goIOSAppearanceChanged.
// Singleton window, but callbacks stay per-registration like every
// other backend.

var (
	appearanceMu        sync.Mutex
	appearanceCallbacks = make(map[*nativePlatform]func(gui.Appearance))
)

// SystemAppearance reads the last trait collection the view
// controller saw. Always known: the view controller seeds the value
// before goIOSInit attaches this platform, so no query can run
// ahead of the seed.
func (n *nativePlatform) SystemAppearance() (gui.Appearance, bool) {
	if C.iosAppearanceDark() != 0 {
		return gui.AppearanceDark, true
	}
	return gui.AppearanceLight, true
}

// SetSystemAppearanceCallback registers cb for trait changes. A nil
// cb unregisters. No OS watcher to manage: UIKit drives the VC,
// which always reports.
func (n *nativePlatform) SetSystemAppearanceCallback(cb func(gui.Appearance)) {
	appearanceMu.Lock()
	defer appearanceMu.Unlock()
	if cb == nil {
		delete(appearanceCallbacks, n)
	} else {
		appearanceCallbacks[n] = cb
	}
}

//export goIOSAppearanceChanged
func goIOSAppearanceChanged(dark C.int) {
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
