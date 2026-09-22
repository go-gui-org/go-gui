//go:build darwin && cgo && !ios

package metal

import (
	"testing"

	"github.com/go-gui-org/go-gui/gui"
)

// goMetalAppearanceChanged fans out to every registered window's
// callback without touching Cocoa, so this runs without the main
// thread or a display. Callback registration itself (which starts
// the KVO observer) stays main-thread-only and is covered by the
// opt-in main-thread suite.
func TestAppearanceChangedFansOut(t *testing.T) {
	appearanceMu.Lock()
	appearanceCallbacks = make(map[*nativePlatform]func(gui.Appearance))
	appearanceMu.Unlock()
	defer func() {
		appearanceMu.Lock()
		appearanceCallbacks = make(map[*nativePlatform]func(gui.Appearance))
		appearanceMu.Unlock()
	}()

	n1, n2 := &nativePlatform{}, &nativePlatform{}
	var got1, got2 []gui.Appearance
	appearanceMu.Lock()
	appearanceCallbacks[n1] = func(a gui.Appearance) { got1 = append(got1, a) }
	appearanceCallbacks[n2] = func(a gui.Appearance) { got2 = append(got2, a) }
	appearanceMu.Unlock()

	goMetalAppearanceChanged(1)
	goMetalAppearanceChanged(0)

	if len(got1) != 2 || got1[0] != gui.AppearanceDark || got1[1] != gui.AppearanceLight {
		t.Errorf("window 1: got %v, want [dark light]", got1)
	}
	if len(got2) != 2 || got2[0] != gui.AppearanceDark || got2[1] != gui.AppearanceLight {
		t.Errorf("window 2: got %v, want [dark light]", got2)
	}
}
