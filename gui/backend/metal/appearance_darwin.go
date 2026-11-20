//go:build darwin && cgo && !ios

package metal

// The file has no cgo of its own since #1307: the sends go through
// purego. It keeps the cgo build term because nativePlatform and the
// rest of this package still need cgo, until the last port step.

import (
	"sync"

	"github.com/ebitengine/purego/objc"

	"github.com/go-gui-org/go-gui/gui"
	"github.com/go-gui-org/go-gui/gui/internal/darwinffi"
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
	if appearanceDark() {
		return gui.AppearanceDark, true
	}
	return gui.AppearanceLight, true
}

// PrefersReducedMotion reports the OS reduce-motion setting
// (issue #757). Queried per call: the theme fade and the SVG
// parser read it only when a theme changes or an SVG loads, so no
// watcher is needed.
func (n *nativePlatform) PrefersReducedMotion() bool {
	pool := darwinffi.PoolPush()
	defer darwinffi.PoolPop(pool)
	ws := darwinffi.Class("NSWorkspace").Send(darwinffi.Sel("sharedWorkspace"))
	return objc.Send[bool](ws, darwinffi.Sel("accessibilityDisplayShouldReduceMotion"))
}

// SetSystemAppearanceCallback registers cb for OS appearance
// changes. The KVO observer runs while at least one window is
// registered; the last unregister stops it. A nil cb unregisters.
//
// The register-then-start decision is one critical section: the
// start and stop sends run under the mutex so a concurrent unregister
// cannot stop an observer a new subscriber needs. AppKit never calls
// the observer synchronously from addObserver (KVO delivers later, on
// the main thread), so this cannot deadlock.
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
		appearanceWatchStart()
	} else {
		appearanceWatchStop()
	}
}

// goMetalAppearanceChanged fans one OS change out to every registered
// callback. dark is 1 for dark, 0 for light.
func goMetalAppearanceChanged(dark int) {
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

// appearanceDark reports the current OS setting. It reads
// NSApp.effectiveAppearance, so a per-app NSAppearance override is
// honored the way the titlebar is. Main-thread only, like every NSApp
// call. The sends go through purego (#1307); none is per frame.
func appearanceDark() bool {
	pool := darwinffi.PoolPush()
	defer darwinffi.PoolPop(pool)
	appkit := darwinffi.Open(darwinffi.PathAppKit)
	// NSApp is a global that +[NSApplication sharedApplication] sets.
	// Read it at each call: before the app exists it is nil, and a
	// send to nil returns 0, which reads as light, as in the cgo form.
	app := objc.ID(darwinffi.Data(appkit, "NSApp"))
	aqua := objc.ID(darwinffi.Data(appkit, "NSAppearanceNameAqua"))
	darkAqua := objc.ID(darwinffi.Data(appkit, "NSAppearanceNameDarkAqua"))
	names := darwinffi.Class("NSArray").Send(darwinffi.Sel("arrayWithObject:"), aqua).
		Send(darwinffi.Sel("arrayByAddingObject:"), darkAqua)
	match := app.Send(darwinffi.Sel("effectiveAppearance")).
		Send(darwinffi.Sel("bestMatchFromAppearancesWithNames:"), names)
	return objc.Send[bool](match, darwinffi.Sel("isEqualToString:"), darkAqua)
}

// appearanceObserver is the KVO observer on NSApp.effectiveAppearance,
// 0 while not installed. Main-thread only, and the start and stop calls
// also hold appearanceMu.
var appearanceObserver objc.ID

// appearanceObserverClass registers GoGuiAppearanceObserver once. Its
// one method forwards each KVO change to goMetalAppearanceChanged. A
// class can be registered only once per process, so tests and a second
// window share it.
var appearanceObserverClass = sync.OnceValue(func() objc.Class {
	cls, err := objc.RegisterClass("GoGuiAppearanceObserver",
		objc.GetClass("NSObject"), nil, nil, []objc.MethodDef{{
			Cmd: darwinffi.Sel("observeValueForKeyPath:ofObject:change:context:"),
			Fn: func(_ objc.ID, _ objc.SEL, _, _, _ objc.ID, _ uintptr) {
				dark := 0
				if appearanceDark() {
					dark = 1
				}
				goMetalAppearanceChanged(dark)
			},
		}})
	if err != nil {
		panic("metal: " + err.Error())
	}
	return cls
})

// appearanceWatchStart installs the observer; a no-op when installed.
func appearanceWatchStart() {
	if appearanceObserver != 0 {
		return
	}
	pool := darwinffi.PoolPush()
	defer darwinffi.PoolPop(pool)
	obs := objc.ID(appearanceObserverClass()).Send(darwinffi.Sel("new"))
	app := objc.ID(darwinffi.Data(darwinffi.Open(darwinffi.PathAppKit), "NSApp"))
	app.Send(darwinffi.Sel("addObserver:forKeyPath:options:context:"),
		obs, darwinffi.NSString("effectiveAppearance"), uint(0), uintptr(0))
	appearanceObserver = obs
}

// appearanceWatchStop removes the observer; a no-op when not installed.
func appearanceWatchStop() {
	if appearanceObserver == 0 {
		return
	}
	pool := darwinffi.PoolPush()
	defer darwinffi.PoolPop(pool)
	app := objc.ID(darwinffi.Data(darwinffi.Open(darwinffi.PathAppKit), "NSApp"))
	app.Send(darwinffi.Sel("removeObserver:forKeyPath:"),
		appearanceObserver, darwinffi.NSString("effectiveAppearance"))
	// +new gave +1. The cgo form let ARC release it on nil assignment.
	appearanceObserver.Send(darwinffi.Sel("release"))
	appearanceObserver = 0
}
