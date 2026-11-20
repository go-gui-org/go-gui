//go:build darwin && cgo && !ios

package metal

// The app: NSApplication, its delegate, the launch handshake, the frame
// pump for nested run loops, clipboard, cursors and the dock icon.
// Main-thread only. See window_darwin.go.
//
// Launch and activation, three steps in order:
//
//  1. metalAppInit, before any window exists: the NSApplication, the
//     activation policy, the menu bar and the delegate. It activates
//     early, so Launch Services registers the Dock tile and the Cmd+Tab
//     entry before it times out.
//  2. metalWindowCreate, for each window: it orders the window front and
//     activates again.
//  3. metalAppFinishLaunch, after the first window exists. finishLaunching
//     waits until here: a .app bundle shows windows on the active Space
//     only if the notification fires with a window already on screen. The
//     first call also does the Launch Services handshake (below). Each
//     call activates, so windows opened later come forward.
//
// The Launch Services handshake: a .app that Launch Services starts gets
// applicationWillFinishLaunching: but not applicationDidFinishLaunching:
// until the run loop handles the kAEOpenApplication event. Until then the
// app is in launch limbo (no Cmd+Tab, gray title bar, two clicks to
// close), even though isActive is YES. metalAppFinishLaunch runs
// [NSApp run] once to handle that event. applicationDidFinishLaunching:
// then calls [NSApp stop:] and posts a wake event (stop: takes effect only
// after the next dequeue), so the Go event loop can take over. A bare CLI
// exec has no handshake. run still calls finishLaunching and returns
// through the same stop.

import (
	"math"
	"sync"
	"sync/atomic"

	"github.com/ebitengine/purego"
	"github.com/ebitengine/purego/objc"

	"github.com/go-gui-org/go-gui/gui/internal/darwinffi"
)

// appDelegate is the GoGuiAppDelegate, +1 for the life of the process:
// NSApp.delegate and NSMenuItem.target are weak.
var appDelegate objc.ID

func appDelegateMethods() []objc.MethodDef {
	quit := func() {
		// Both are needed. ev.typ gives delivery in the same poll when
		// the quit fires inside that poll's sendEvent:. quitRequested is
		// the only signal that survives when the quit comes outside the
		// default mode (a menu-bar click during tracking, a system
		// termination).
		pollState.ev.typ = metalEventQuit
		pollState.quitRequested = true
	}
	return []objc.MethodDef{
		{Cmd: darwinffi.Sel("quit:"), Fn: func(objc.ID, objc.SEL, objc.ID) { quit() }},
		// System termination (logout, shutdown, [NSApp terminate:]) comes
		// here, so the Go event loop runs its own teardown and the process
		// is not killed.
		{Cmd: darwinffi.Sel("applicationShouldTerminate:"), Fn: func(objc.ID, objc.SEL, objc.ID) uint {
			quit()
			return nsTerminateCancel
		}},
		// After a switch back from another app (or a system dialog such
		// as a TCC prompt), applicationDidBecomeActive: fires but
		// windowDidBecomeKey: may not. Go's focused flag then stays false
		// and eventAllowed blocks keys and left clicks. Restore the focus
		// of the frontmost window.
		{Cmd: darwinffi.Sel("applicationDidBecomeActive:"), Fn: func(objc.ID, objc.SEL, objc.ID) {
			w := focusedWindow()
			if w == nil {
				return
			}
			if objs.app.Send(darwinffi.Sel("keyWindow")) != w.ns {
				// makeKeyAndOrderFront: fires windowDidBecomeKey:, which
				// sends EventFocused. Sending it here too would give
				// OnEvent a duplicate.
				w.ns.Send(darwinffi.Sel("makeKeyAndOrderFront:"), objc.ID(0))
				return
			}
			// The window is key, but Go's focus can have drifted to false
			// (windowDidBecomeKey: does not fire again on reactivation).
			// This is the only path that sets the focus again.
			goMetalWindowFocusChanged(w.id, true)
		}},
		// Files opened from Finder or the Dock, `open -a`, and links in a
		// URL scheme the bundle registers (CFBundleURLTypes). Since macOS
		// 10.13 AppKit sends the odoc and GURL Apple Events here once the
		// delegate has this method. A launch with documents delivers them
		// during the [NSApp run] of metalAppFinishLaunch, before the Go
		// loop starts. gui.App holds them until OnOpen can run.
		{Cmd: darwinffi.Sel("application:openURLs:"), Fn: func(_ objc.ID, _ objc.SEL, _, urls objc.ID) {
			n := objc.Send[uint](urls, darwinffi.Sel("count"))
			if n == 0 {
				return
			}
			items := make([]string, 0, n)
			isFile := make([]bool, 0, n)
			for i := range n {
				u := urls.Send(darwinffi.Sel("objectAtIndex:"), i)
				file := objc.Send[bool](u, darwinffi.Sel("isFileURL"))
				sel := "absoluteString"
				if file {
					sel = "path"
				}
				s := u.Send(darwinffi.Sel(sel))
				if s == 0 {
					continue
				}
				items = append(items, goStr(s))
				isFile = append(isFile, file)
			}
			if len(items) > 0 {
				deliverOpen(openApp.Load(), items, isFile)
			}
		}},
		// Stops the [NSApp run] that metalAppFinishLaunch started, so the
		// Go event loop can take over. stop: takes effect only after the
		// next event is dequeued, so post a wake event to force a return.
		{Cmd: darwinffi.Sel("applicationDidFinishLaunching:"), Fn: func(objc.ID, objc.SEL, objc.ID) {
			objs.app.Send(darwinffi.Sel("stop:"), objc.ID(0))
			objs.app.Send(sels.postEvent, msgWakeEvent(), true)
		}},
	}
}

// focusedWindow is the window that gets focus events: the key window,
// then the main window, then any visible window of this backend.
func focusedWindow() *macWindow {
	if w := windowOf(objs.app.Send(darwinffi.Sel("keyWindow"))); w != nil {
		return w
	}
	if w := windowOf(objs.app.Send(darwinffi.Sel("mainWindow"))); w != nil {
		return w
	}
	wins := objs.app.Send(darwinffi.Sel("windows"))
	n := objc.Send[uint](wins, darwinffi.Sel("count"))
	for i := range n {
		ns := wins.Send(darwinffi.Sel("objectAtIndex:"), i)
		if w := windowOf(ns); w != nil && w.ns == ns &&
			objc.Send[bool](ns, darwinffi.Sel("isVisible")) {
			return w
		}
	}
	return nil
}

// metalForceActivate brings the app to the foreground. [NSApp activate]
// (macOS 14+) cooperates: it does not take focus from the active app (the
// terminal or Finder that launched it), so a new window comes up inactive:
// gray traffic lights, two clicks to close, and no Cmd+Tab entry until the
// user clicks the window. activateIgnoringOtherApps: always takes the
// foreground. It is deprecated on 14+ but is the only reliable path at
// launch.
func metalForceActivate() {
	objs.app.Send(darwinffi.Sel("activateIgnoringOtherApps:"), true)
}

var appInitDone bool

// metalAppInit makes NSApp, the menu bar and the delegate. A second call
// does nothing.
func metalAppInit() {
	if appInitDone {
		return
	}
	appInitDone = true
	loadObjc()
	pool := darwinffi.PoolPush()
	defer darwinffi.PoolPop(pool)

	// Key repeat. macOS press-and-hold (on by default) gives a held key
	// to the accent palette after the first insertText:, and stops
	// calling insertText: for the repeat keyDown: events after it. Each
	// repeat is then a bare EventKeyDown with no EventChar: holding a
	// letter types one character, holding Backspace deletes one, and a
	// terminal pane echoes nothing while the key is down.
	//
	// Registered, not written: the registration domain is at the bottom
	// of the defaults search order, so a user who wants the accent
	// palette keeps the way out
	//
	//	defaults write -g ApplePressAndHoldEnabled -bool true
	//
	// which is in NSGlobalDomain and wins over this. It must run before
	// sharedApplication, so AppKit reads it during its own setup.
	no := darwinffi.Class("NSNumber").Send(darwinffi.Sel("numberWithBool:"), false)
	defaults := darwinffi.Class("NSDictionary").Send(darwinffi.Sel("dictionaryWithObject:forKey:"),
		no, darwinffi.NSString("ApplePressAndHoldEnabled"))
	darwinffi.Class("NSUserDefaults").Send(darwinffi.Sel("standardUserDefaults")).
		Send(darwinffi.Sel("registerDefaults:"), defaults)

	objs.app = objc.ID(appClass).Send(darwinffi.Sel("sharedApplication"))
	objs.app.Send(darwinffi.Sel("setActivationPolicy:"), nsApplicationActivationPolicyRegular)

	appName := goStr(darwinffi.Class("NSProcessInfo").Send(darwinffi.Sel("processInfo")).
		Send(darwinffi.Sel("processName")))
	if appName == "" {
		appName = "go-gui"
	}

	if appDelegate == 0 {
		appDelegate = objc.ID(delegateClass).Send(sels.new)
	}

	bar := darwinffi.Class("NSMenu").Send(sels.new).Send(sels.autorelease)
	appMenu := darwinffi.Class("NSMenu").Send(sels.new).Send(sels.autorelease)
	addItem := func(menu objc.ID, title, action, key string) objc.ID {
		var sel objc.SEL
		if action != "" {
			sel = darwinffi.Sel(action)
		}
		return menu.Send(darwinffi.Sel("addItemWithTitle:action:keyEquivalent:"),
			nsStr(title), sel, darwinffi.NSString(key))
	}
	separator := func(menu objc.ID) {
		menu.Send(darwinffi.Sel("addItem:"),
			darwinffi.Class("NSMenuItem").Send(darwinffi.Sel("separatorItem")))
	}
	submenuItem := func(title string, sub objc.ID) objc.ID {
		item := darwinffi.Class("NSMenuItem").Send(sels.alloc).Send(
			darwinffi.Sel("initWithTitle:action:keyEquivalent:"),
			nsStr(title), objc.SEL(0), darwinffi.NSString("")).Send(sels.autorelease)
		item.Send(darwinffi.Sel("setSubmenu:"), sub)
		return item
	}

	addItem(appMenu, "About "+appName, "orderFrontStandardAboutPanel:", "")
	separator(appMenu)
	quitItem := addItem(appMenu, "Quit "+appName, "quit:", "q")
	quitItem.Send(darwinffi.Sel("setKeyEquivalentModifierMask:"), uint(nsEventModifierFlagCommand))
	quitItem.Send(darwinffi.Sel("setTarget:"), appDelegate)
	bar.Send(darwinffi.Sel("addItem:"), submenuItem(appName, appMenu))

	windowMenu := darwinffi.Class("NSMenu").Send(sels.new).Send(sels.autorelease)
	addItem(windowMenu, "Close", "performClose:", "w")
	addItem(windowMenu, "Minimize", "performMiniaturize:", "m")
	addItem(windowMenu, "Zoom", "performZoom:", "")
	separator(windowMenu)
	addItem(windowMenu, "Bring All to Front", "arrangeInFront:", "")
	bar.Send(darwinffi.Sel("addItem:"), submenuItem("Window", windowMenu))
	objs.app.Send(darwinffi.Sel("setWindowsMenu:"), windowMenu)
	objs.app.Send(darwinffi.Sel("setMainMenu:"), bar)

	objs.app.Send(darwinffi.Sel("setDelegate:"), appDelegate)
	// finishLaunching waits for metalAppFinishLaunch. Activate early so
	// Launch Services registers the app before it times out.
	metalForceActivate()
}

var finishLaunchOnce sync.Once

// metalAppFinishLaunch runs the Launch Services handshake once and
// activates the app on each call.
func metalAppFinishLaunch() {
	pool := darwinffi.PoolPush()
	defer darwinffi.PoolPop(pool)
	// A bare CLI exec has already finished launching, so it skips run.
	finishLaunchOnce.Do(func() {
		cur := darwinffi.Class("NSRunningApplication").Send(darwinffi.Sel("currentApplication"))
		if !objc.Send[bool](cur, darwinffi.Sel("isFinishedLaunching")) {
			objs.app.Send(darwinffi.Sel("run"))
		}
	})
	// Not in the Once: each call must activate, so new windows come to
	// the foreground.
	metalForceActivate()

	// Windows exist now, so frames mean something: arm the nested run
	// loop frame pump. A second call does nothing.
	metalStartFramePump()
}

// ─── Frame pump (nested run loops) ─────────────────────────────
//
// The Go event loop pumps only NSDefaultRunLoopMode, so a nested AppKit
// run loop (a modal dialog, menu tracking, live resize) gives no window a
// frame. A repeating timer is armed only while such a nested mode runs
// (issue #406). In the default mode no timer exists, so an idle app costs
// the run loop nothing and can take App Nap.

// framePumpInterval is 60 Hz. A frame with nothing dirty is cheap:
// Window.PumpFrame flushes commands and returns false without rendering
// unless a refresh is pending.
const framePumpInterval = 1.0 / 60.0

var framePump struct {
	target   objc.ID // GoGuiFramePumpTarget, +1
	timer    objc.ID // NSTimer, +1, or 0 when not armed
	observer uintptr // CFRunLoopObserverRef, +1, or 0
	// depth is how many nested (non-default) modes run. The timer is
	// armed while it is more than 0 (a menu over a modal dialog is 2).
	depth int

	callback uintptr // the observer's C callback
	cf       struct {
		create, addObserver, removeObserver, release, getMain uintptr
		commonModes                                           uintptr
	}
}

// CFRunLoopActivity values.
const (
	cfRunLoopEntry = 1 << 0
	cfRunLoopExit  = 1 << 7
)

func framePumpMethods() []objc.MethodDef {
	return []objc.MethodDef{{
		Cmd: sels.pumpFrames,
		// Reached only while a nested mode is current: the run-loop
		// observer arms the timer, so no timer wakes in the default mode.
		Fn: func(objc.ID, objc.SEL, objc.ID) { goMetalPumpFrames() },
	}}
}

var loadFramePump = sync.OnceFunc(func() {
	cfLib := darwinffi.Open(darwinffi.PathCoreFoundation)
	cf := &framePump.cf
	cf.create = darwinffi.Func(cfLib, "CFRunLoopObserverCreate")
	cf.addObserver = darwinffi.Func(cfLib, "CFRunLoopAddObserver")
	cf.removeObserver = darwinffi.Func(cfLib, "CFRunLoopRemoveObserver")
	cf.release = darwinffi.Func(cfLib, "CFRelease")
	cf.getMain = darwinffi.Func(cfLib, "CFRunLoopGetMain")
	cf.commonModes = darwinffi.Data(cfLib, "kCFRunLoopCommonModes")
	framePump.callback = purego.NewCallback(framePumpModeObserver)
	framePump.target = objc.ID(pumpTargetClass).Send(sels.new)
})

// framePumpArm schedules the timer in NSRunLoopCommonModes, which holds
// NSEventTrackingRunLoopMode and NSModalPanelRunLoopMode, so it fires
// while the current mode is a nested one.
func framePumpArm() {
	if framePump.timer != 0 {
		return
	}
	t := darwinffi.Class("NSTimer").Send(
		darwinffi.Sel("timerWithTimeInterval:target:selector:userInfo:repeats:"),
		framePumpInterval, framePump.target, sels.pumpFrames, objc.ID(0), true)
	framePump.timer = t.Send(sels.retain)
	common := objc.ID(darwinffi.Data(darwinffi.Open(darwinffi.PathFoundation), "NSRunLoopCommonModes"))
	darwinffi.Class("NSRunLoop").Send(darwinffi.Sel("mainRunLoop")).
		Send(darwinffi.Sel("addTimer:forMode:"), t, common)
}

func framePumpDisarm() {
	if framePump.timer == 0 {
		return
	}
	framePump.timer.Send(darwinffi.Sel("invalidate"))
	framePump.timer.Send(sels.release)
	framePump.timer = 0
}

// framePumpModeObserver arms the pump when a nested mode begins and
// disarms it when the last one ends. It runs on the main thread inside
// the run loop, so the timer can change here.
func framePumpModeObserver(_, activity, _ uintptr) {
	mode := darwinffi.Class("NSRunLoop").Send(darwinffi.Sel("currentRunLoop")).
		Send(darwinffi.Sel("currentMode"))
	nested := mode != 0 && !objc.Send[bool](mode, darwinffi.Sel("isEqualToString:"), objs.defaultMode)
	switch activity {
	case cfRunLoopEntry:
		if nested {
			framePump.depth++
			if framePump.depth == 1 {
				framePumpArm()
			}
		}
	case cfRunLoopExit:
		// An exit can come for a mode entered before the observer was
		// added, so the depth never goes below zero.
		if nested && framePump.depth > 0 {
			framePump.depth--
			if framePump.depth == 0 {
				framePumpDisarm()
			}
		}
	}
}

func metalStartFramePump() {
	if framePump.observer != 0 {
		return
	}
	loadObjc()
	loadFramePump()
	cf := &framePump.cf
	// CFRunLoopObserverCreate(allocator, activities, repeats, order,
	// callout, context).
	framePump.observer = msgCall(cf.create, 0, cfRunLoopEntry|cfRunLoopExit, 1, 0,
		framePump.callback, 0)
	msgCall(cf.addObserver, msgCall(cf.getMain), framePump.observer, cf.commonModes)
}

func metalStopFramePump() {
	if framePump.observer != 0 {
		cf := &framePump.cf
		msgCall(cf.removeObserver, msgCall(cf.getMain), framePump.observer, cf.commonModes)
		msgCall(cf.release, framePump.observer)
		framePump.observer = 0
	}
	framePumpDisarm()
	framePump.depth = 0
}

// framePumpCount counts goMetalPumpFrames calls that ran (reached Go from
// a nested run loop). Tests read it to prove the timer fires in nested
// modes and stays quiet in the default mode.
var framePumpCount atomic.Uint64

// ─── Cursors ───────────────────────────────────────────────────

// cursorInContentBounds says whether the pointer is in the content view.
// metalWindowSetCursor and its tests share it.
func cursorInContentBounds(mouseX, mouseY, width, height float32) bool {
	if math.IsInf(float64(mouseX), 0) || math.IsNaN(float64(mouseX)) ||
		math.IsInf(float64(mouseY), 0) || math.IsNaN(float64(mouseY)) {
		return false
	}
	return mouseX >= 0 && mouseX < width && mouseY >= 0 && mouseY < height
}

// metalWindowSetCursor sets the cursor named by sel, an NSCursor class
// method such as arrowCursor. It does nothing while the pointer is
// outside the content view: the window server manages the cursors of the
// frame (resize edges, title-bar buttons), and a cursor set there would
// stop the edge-resize cursors and the traffic-light hover effects.
func metalWindowSetCursor(w *macWindow, sel objc.SEL, mouseX, mouseY float32) {
	if w == nil || w.view == 0 || sel == 0 {
		return
	}
	b := msgRect(w.view, sels.bounds)
	if !cursorInContentBounds(mouseX, mouseY, float32(b.W), float32(b.H)) {
		return
	}
	cls := cursorClass()
	if msg(cls, cursorSels.respondsToSelector, uintptr(sel))&0xff == 0 {
		return
	}
	if cursor := objc.ID(msg(cls, sel)); cursor != 0 {
		msg(cursor, cursorSels.set)
	}
}

var cursorSels struct {
	respondsToSelector, set objc.SEL
}

var cursorClass = sync.OnceValue(func() objc.ID {
	cursorSels.respondsToSelector = darwinffi.Sel("respondsToSelector:")
	cursorSels.set = darwinffi.Sel("set")
	return darwinffi.Class("NSCursor")
})

// ─── Clipboard ─────────────────────────────────────────────────

// maxClipboardBytes caps clipboard text at 16 MB, against a hostile or
// broken pasteboard.
const maxClipboardBytes = 16 * 1024 * 1024

// nsUTF8StringEncoding is NSUTF8StringEncoding.
const nsUTF8StringEncoding = 4

func pasteboardString() objc.ID {
	return objc.ID(darwinffi.Data(darwinffi.Open(darwinffi.PathAppKit), "NSPasteboardTypeString"))
}

// metalClipboardGet returns the clipboard text, and ok false when there is
// none or it is over the cap.
func metalClipboardGet() (text string, ok bool) {
	pool := darwinffi.PoolPush()
	defer darwinffi.PoolPop(pool)
	pb := darwinffi.Class("NSPasteboard").Send(darwinffi.Sel("generalPasteboard"))
	s := pb.Send(darwinffi.Sel("stringForType:"), pasteboardString())
	if s == 0 {
		return "", false
	}
	if objc.Send[uint](s, darwinffi.Sel("lengthOfBytesUsingEncoding:"), uint(nsUTF8StringEncoding)) > maxClipboardBytes {
		return "", false
	}
	return goStr(s), true
}

// metalClipboardSet puts text on the clipboard. Text over the cap is
// refused, as on the read side.
func metalClipboardSet(text string) {
	if len(text) > maxClipboardBytes {
		return
	}
	pool := darwinffi.PoolPush()
	defer darwinffi.PoolPop(pool)
	pb := darwinffi.Class("NSPasteboard").Send(darwinffi.Sel("generalPasteboard"))
	pb.Send(darwinffi.Sel("clearContents"))
	pb.Send(darwinffi.Sel("setString:forType:"), nsStr(text), pasteboardString())
}

// ─── Dock icon ─────────────────────────────────────────────────

// metalSetDockIcon sets the application icon from image data (PNG).
func metalSetDockIcon(data []byte) {
	if len(data) == 0 {
		return
	}
	pool := darwinffi.PoolPush()
	defer darwinffi.PoolPop(pool)
	ns := darwinffi.Class("NSData").Send(darwinffi.Sel("dataWithBytes:length:"),
		&data[0], uint(len(data)))
	img := darwinffi.Class("NSImage").Send(sels.alloc).Send(darwinffi.Sel("initWithData:"), ns)
	if img == 0 {
		return
	}
	objs.app.Send(darwinffi.Sel("setApplicationIconImage:"), img)
	img.Send(sels.release)
}
