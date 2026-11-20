//go:build darwin && cgo && !ios

package metal

import (
	"fmt"
	"testing"

	"github.com/ebitengine/purego/objc"

	"github.com/go-gui-org/go-gui/gui"
	"github.com/go-gui-org/go-gui/gui/internal/darwinffi"
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

// testSetAppAppearance pins NSApp.appearance to the named appearance
// (an NSAppearanceName* global in AppKit), or clears the pin when name
// is "". Main thread only.
func testSetAppAppearance(name string) {
	pool := darwinffi.PoolPush()
	defer darwinffi.PoolPop(pool)
	appkit := darwinffi.Open(darwinffi.PathAppKit)
	var appearance objc.ID
	if name != "" {
		appearance = darwinffi.Class("NSAppearance").Send(darwinffi.Sel("appearanceNamed:"),
			objc.ID(darwinffi.Data(appkit, name)))
	}
	objc.ID(darwinffi.Data(appkit, "NSApp")).Send(darwinffi.Sel("setAppearance:"), appearance)
}

// testAppearanceObserver is step 8c2 of runMainThreadTests: a change
// of NSApp's effective appearance reaches the registered callback, and
// SystemAppearance reads the new value. Panics on failure.
func testAppearanceObserver() {
	n := &nativePlatform{}
	var got []gui.Appearance
	n.SetSystemAppearanceCallback(func(a gui.Appearance) { got = append(got, a) })
	defer func() {
		n.SetSystemAppearanceCallback(nil)
		testSetAppAppearance("")
	}()
	for _, step := range []struct {
		name string
		want gui.Appearance
	}{
		{"NSAppearanceNameDarkAqua", gui.AppearanceDark},
		{"NSAppearanceNameAqua", gui.AppearanceLight},
	} {
		testSetAppAppearance(step.name)
		if a, _ := n.SystemAppearance(); a != step.want {
			panic(fmt.Sprintf("SystemAppearance after %s = %v, want %v", step.name, a, step.want))
		}
		if len(got) == 0 || got[len(got)-1] != step.want {
			panic(fmt.Sprintf("appearance observer after %s: callbacks %v, want last %v",
				step.name, got, step.want))
		}
	}
	if appearanceObserver == 0 {
		panic("appearance observer not installed while a callback is registered")
	}
	n.SetSystemAppearanceCallback(nil)
	if appearanceObserver != 0 {
		panic("appearance observer still installed after the last unregister")
	}
}
