//go:build darwin && arm64 && !ios

package metal

// Test hooks for the window, event and app code. They were C functions
// in metal_window_darwin.m, because they reached file-static C state.
// That state is Go now, so the hooks are Go and live in a test file.
//
// The hooks that touch a window or NSApp are main-thread only: they run
// from runMainThreadTests (TestMain). The text-queue and event hooks
// touch only Go state and run from any test.

import (
	"sync"
	"time"
	"unsafe"

	"github.com/ebitengine/purego/objc"

	"github.com/go-gui-org/go-gui/gui/internal/darwinffi"
)

func testActivationPolicyRegular() bool {
	return objc.Send[int](objs.app, darwinffi.Sel("activationPolicy")) ==
		nsApplicationActivationPolicyRegular
}

// testSizeLimits reads back the content size limits, so a test can see
// the WindowCfg limits reached AppKit. An unset maximum comes back as 0,
// not CGFLOAT_MAX.
func testSizeLimits(w *macWindow) (minW, minH, maxW, maxH int) {
	mn := objc.Send[nsSize](w.ns, darwinffi.Sel("contentMinSize"))
	mx := objc.Send[nsSize](w.ns, darwinffi.Sel("contentMaxSize"))
	unset := func(v float64) int {
		if v >= cgFloatMax {
			return 0
		}
		return int(v)
	}
	return int(mn.W), int(mn.H), unset(mx.W), unset(mx.H)
}

func testDelegateSet() bool {
	return objs.app.Send(darwinffi.Sel("delegate")) != 0
}

// testDelegateOpenURLs sends the delegate one file URL and one URL in a
// custom scheme, as AppKit does for odoc and GURL. It returns false when
// the delegate does not have the method.
func testDelegateOpenURLs() bool {
	pool := darwinffi.PoolPush()
	defer darwinffi.PoolPop(pool)
	d := objs.app.Send(darwinffi.Sel("delegate"))
	sel := darwinffi.Sel("application:openURLs:")
	if !objc.Send[bool](d, darwinffi.Sel("respondsToSelector:"), sel) {
		return false
	}
	nsURL := darwinffi.Class("NSURL")
	file := nsURL.Send(darwinffi.Sel("fileURLWithPath:"), darwinffi.NSString("/tmp/go gui.txt"))
	link := nsURL.Send(darwinffi.Sel("URLWithString:"), darwinffi.NSString("gogui-test://open?x=1"))
	urls := darwinffi.Class("NSArray").Send(darwinffi.Sel("arrayWithObject:"), file).
		Send(darwinffi.Sel("arrayByAddingObject:"), link)
	d.Send(sel, objs.app, urls)
	return true
}

func testMenuExists() bool {
	m := objs.app.Send(darwinffi.Sel("mainMenu"))
	return m != 0 && objc.Send[int](m, darwinffi.Sel("numberOfItems")) > 0
}

// testAppMenu returns the first submenu of the main menu, or 0.
func testAppMenu() objc.ID {
	if !testMenuExists() {
		return 0
	}
	m := objs.app.Send(darwinffi.Sel("mainMenu"))
	return m.Send(darwinffi.Sel("itemAtIndex:"), 0).Send(darwinffi.Sel("submenu"))
}

// testAppMenuHas reports whether the app menu has an item with action
// sel, and target target when target is not 0.
func testAppMenuHas(sel objc.SEL, target objc.ID) bool {
	am := testAppMenu()
	if am == 0 {
		return false
	}
	n := objc.Send[int](am, darwinffi.Sel("numberOfItems"))
	for i := range n {
		item := am.Send(darwinffi.Sel("itemAtIndex:"), i)
		if objc.Send[objc.SEL](item, darwinffi.Sel("action")) != sel {
			continue
		}
		if target == 0 || item.Send(darwinffi.Sel("target")) == target {
			return true
		}
	}
	return false
}

func testMenuQuitWired() bool {
	d := objs.app.Send(darwinffi.Sel("delegate"))
	return d != 0 && testAppMenuHas(darwinffi.Sel("quit:"), d)
}

func testMenuAboutExists() bool {
	return testAppMenuHas(darwinffi.Sel("orderFrontStandardAboutPanel:"), 0)
}

func testWindowsMenuExists() bool {
	m := objs.app.Send(darwinffi.Sel("windowsMenu"))
	return m != 0 && objc.Send[int](m, darwinffi.Sel("numberOfItems")) > 0
}

func testWindowDelegateExists(w *macWindow) bool {
	return w != nil && w.ns != 0 && w.ns.Send(darwinffi.Sel("delegate")) != 0
}

func testWindowID(w *macWindow) uint32 { return metalWindowGetID(w) }

// testSetMarked sends setMarkedText:selectedRange:replacementRange: to
// the content view, as an input method does.
func testSetMarked(w *macWindow, text string, sel nsRange) {
	w.view.Send(darwinffi.Sel("setMarkedText:selectedRange:replacementRange:"),
		darwinffi.NSString(text), sel, nsRange{nsNotFound, 0})
}

func testHasMarked(w *macWindow) bool {
	return objc.Send[bool](w.view, darwinffi.Sel("hasMarkedText"))
}

// testSubstring sends attributedSubstringForProposedRange:actualRange:
// and returns the text, and ok false for nil.
func testSubstring(w *macWindow, r nsRange, actual *nsRange) (string, bool) {
	sub := w.view.Send(darwinffi.Sel("attributedSubstringForProposedRange:actualRange:"),
		r, unsafe.Pointer(actual))
	if sub == 0 {
		return "", false
	}
	return goStr(sub.Send(darwinffi.Sel("string"))), true
}

// testIMEClientConformance checks the NSTextInputClient answers that an
// input method needs to commit converted text. NSNotFound from
// selectedRange, or nil from attributedSubstringForProposedRange:, makes
// the Japanese IME drop the commit after conversion: the composition
// stays marked and nothing reaches the app. Reproduced in a plain AppKit
// program with no go-gui code, so the contract is AppKit's. An empty
// validAttributesForMarkedText tells the IME the client cannot draw
// clause segments.
//
// The checks send through the Objective-C runtime, as AppKit does, so
// they test the method types as well as the Go code.
func testIMEClientConformance(w *macWindow) bool {
	pool := darwinffi.PoolPush()
	defer darwinffi.PoolPop(pool)
	ok := true

	// Compose "あい" with the whole clause selected, as conversion does.
	testSetMarked(w, "あい", nsRange{0, 2})
	sel := objc.Send[nsRange](w.view, darwinffi.Sel("selectedRange"))
	if sel.Loc == nsNotFound || sel.Loc+sel.Len > 2 {
		ok = false
	}
	actual := new(nsRange)
	*actual = nsRange{nsNotFound, 0}
	if s, got := testSubstring(w, nsRange{0, 2}, actual); !got || s != "あい" || *actual != (nsRange{0, 2}) {
		ok = false
	}
	// A request out of range must give nil, not crash or lie.
	if _, got := testSubstring(w, nsRange{99, 2}, nil); got {
		ok = false
	}
	// A length near NSUIntegerMax must clamp, not wrap past the guard
	// into substringWithRange:. IMK does ask for these.
	if s, got := testSubstring(w, nsRange{1, ^uint(0)}, nil); !got || s != "い" {
		ok = false
	}
	attrs := w.view.Send(darwinffi.Sel("validAttributesForMarkedText"))
	if objc.Send[uint](attrs, darwinffi.Sel("count")) == 0 {
		ok = false
	}

	// The queued composition carries the clamped selection, not the raw
	// one: NSNotFound would reach the renderer as a negative offset.
	// Surrogate pairs: the IME reports selectedRange in UTF-16 units, but
	// the event is in code points, so a pair before the clause must not
	// move the underline (issue #169). The clamp runs in UTF-16 space and
	// converts after it. A boundary between the halves of a pair rounds
	// to the end of the pair.
	for _, tc := range []struct {
		text       string
		sel        nsRange
		start, len int
	}{
		{"あい", nsRange{nsNotFound, 0}, 2, 0},
		{"あ😀", nsRange{1, 2}, 1, 1},
		{"あ😀", nsRange{nsNotFound, 0}, 2, 0},
		{"𠮟", nsRange{0, 2}, 0, 1},
		{"あ😀", nsRange{2, 0}, 2, 0},
	} {
		resetTextQueue()
		testSetMarked(w, tc.text, tc.sel)
		if !popTextEvent() || pollState.ev.imeStart != tc.start || pollState.ev.imeLength != tc.len {
			ok = false
		}
	}

	w.view.Send(darwinffi.Sel("unmarkText"))
	if objc.Send[nsRange](w.view, darwinffi.Sel("selectedRange")).Loc == nsNotFound {
		ok = false
	}
	resetTextQueue()
	return ok
}

// testIMEKeySuppressedWhileComposing reports whether a key during a
// composition goes to the input method or to the widget. A key that
// reaches the widget during a composition moves the caret of the field
// under the preedit and can fire Enter or Escape actions. But a key the
// input source declines must still reach the widget, or Tab cannot
// leave the field.
//
// The decision is driven directly, not through a synthetic NSEvent: the
// real input source has no composition here, so it declines every key
// and could test only the declined branch.
func testIMEKeySuppressedWhileComposing(w *macWindow) bool {
	pool := darwinffi.PoolPush()
	defer darwinffi.PoolPop(pool)
	ps := &pollState

	// A composition exists only over an editable widget.
	w.imeActive = true
	testSetMarked(w, "あい", nsRange{0, 2})

	// Used by the input method: an arrow that walks its clauses.
	ps.imeDeclinedKey, ps.imeConsumedKey = false, false
	w.imeSettle(true)
	suppressed := ps.imeConsumedKey

	// Declined: the input method gave the key back through
	// doCommandBySelector:, so it is the widget's. This is Tab on a
	// pending dead key: taking it left focus stuck in the field (#393).
	w.view.Send(darwinffi.Sel("doCommandBySelector:"), darwinffi.Sel("insertTab:"))
	ps.imeConsumedKey = false
	w.imeSettle(true)
	declinedReachesWidget := !ps.imeConsumedKey

	// Not composing: the key is the widget's.
	w.imeActive = false
	w.view.Send(darwinffi.Sel("unmarkText"))
	ps.imeDeclinedKey, ps.imeConsumedKey = false, false
	w.imeSettle(false)
	deliveredWhenIdle := !ps.imeConsumedKey

	ps.imeDeclinedKey, ps.imeConsumedKey = false, false
	resetTextQueue()
	return suppressed && declinedReachesWidget && deliveredWhenIdle
}

// testIMEDeadKeyDeliveredWithoutTextContext drives the preedit decision
// an Option+printable key makes, and reports whether the key reaches the
// app when no editable text widget has focus. On the US layout Option+I
// is the circumflex dead key: it starts a composition, which used to take
// the key and leave the app shortcut dead (issue #393), and left the
// accent pending so the next keystroke composed against it.
func testIMEDeadKeyDeliveredWithoutTextContext(w *macWindow) bool {
	pool := darwinffi.PoolPush()
	defer darwinffi.PoolPop(pool)
	ps := &pollState

	// No text context: the dead key is dropped and the key-down goes to
	// the app as the shortcut it is.
	w.imeActive = false
	ps.imeDeclinedKey, ps.imeConsumedKey = false, false
	resetTextQueue()
	testSetMarked(w, "ˆ", nsRange{0, 1})
	w.imeSettle(false)
	deliveredToApp := !ps.imeConsumedKey
	preeditDropped := !testHasMarked(w)
	// The staged preedit, and the empty one that clears it in Go.
	queuedClear := ps.textCount == 2

	// Editable text has focus: the same dead key is the first half of an
	// accent (Option+e then e gives é) and stays with the input method.
	w.imeActive = true
	ps.imeDeclinedKey, ps.imeConsumedKey = false, false
	resetTextQueue()
	testSetMarked(w, "ˆ", nsRange{0, 1})
	w.imeSettle(false)
	claimedWhenEditing := ps.imeConsumedKey && testHasMarked(w)

	// A live composition wins over both: it owns the keyboard whatever
	// has focus.
	ps.imeConsumedKey = false
	w.imeSettle(true)
	claimedWhileComposing := ps.imeConsumedKey

	w.imeActive = false
	w.view.Send(darwinffi.Sel("unmarkText"))
	ps.imeConsumedKey = false
	resetTextQueue()
	return deliveredToApp && preeditDropped && queuedClear &&
		claimedWhenEditing && claimedWhileComposing
}

// testContentViewIsFirstResponder checks keys reach the view without
// the input method having been activated (issue #393).
func testContentViewIsFirstResponder(w *macWindow) bool {
	return w.ns.Send(darwinffi.Sel("firstResponder")) == w.view
}

// testWindowAppearance reports the appearance pinned on the window: -1
// none (follows the system), 0 Aqua, 1 DarkAqua, 2 anything else.
func testWindowAppearance(w *macWindow) int {
	a := w.ns.Send(darwinffi.Sel("appearance"))
	if a == 0 {
		return -1
	}
	name := goStr(a.Send(darwinffi.Sel("name")))
	switch name {
	case "NSAppearanceNameAqua":
		return 0
	case "NSAppearanceNameDarkAqua":
		return 1
	}
	return 2
}

func testWindowVisible(w *macWindow) bool {
	return objc.Send[bool](w.ns, darwinffi.Sel("isVisible"))
}

// testStartLiveResize and testEndLiveResize call the live-resize
// delegate methods as AppKit's resize loop does. The window is its own
// delegate, so a notification posted to NSNotificationCenter would not
// reach it. AppKit has no API to start a real drag, so this is the only
// way to test the presentation switch.
func testStartLiveResize(w *macWindow) {
	w.ns.Send(darwinffi.Sel("windowWillStartLiveResize:"), objc.ID(0))
}

func testEndLiveResize(w *macWindow) {
	w.ns.Send(darwinffi.Sel("windowDidEndLiveResize:"), objc.ID(0))
}

// testLayerPresentsWithTransaction reports the presentation mode of the
// content layer: 1 transaction mode, 0 async, -1 no CAMetalLayer.
func testLayerPresentsWithTransaction(w *macWindow) int {
	layer := w.ns.Send(sels.contentView).Send(darwinffi.Sel("layer"))
	if !objc.Send[bool](layer, darwinffi.Sel("isKindOfClass:"), darwinffi.Class("CAMetalLayer")) {
		return -1
	}
	if objc.Send[bool](layer, darwinffi.Sel("presentsWithTransaction")) {
		return 1
	}
	return 0
}

// testActivateNow runs metalAppFinishLaunch, to check it runs without a
// crash.
func testActivateNow() { metalAppFinishLaunch() }

// testInjectKeyDown sets the event state to a key-down, so mapMetalEvent
// can be tested without an event loop.
func testInjectKeyDown(keyCode uint16, modifiers uint32) {
	ev := &pollState.ev
	ev.typ = metalEventKeyDown
	ev.windowID = 0
	ev.keyCode = keyCode
	ev.modifiers = modifiers
	ev.keyRepeat = false
}

// testInjectQuitEvent sets the event state to a quit.
func testInjectQuitEvent() {
	pollState.ev.typ = metalEventQuit
	pollState.ev.windowID = 0
}

// testPushIMECommit and testPushIMEComposition queue text events as
// insertText: and setMarkedText: do, so tests can drive the IME path (and
// several events in one sendEvent:, issue #149) with no event loop and no
// real input method. An empty composition is the end signal.
func testPushIMECommit(text string) { pushTextEvent(metalEventChar, text, 0, 0, 0) }

func testPushIMEComposition(text string, start, length int) {
	pushTextEvent(metalEventIMEComp, text, 0, start, length)
}

func testIMEQueueDepth() int { return pollState.textCount }

func testResetIMEQueue() { resetTextQueue() }

// testPopTextEvent takes one queued text event without metalPollEvent,
// for the empty-queue checks: metalPollEvent would go on to NSApp, which
// is main-thread only.
func testPopTextEvent() bool { return popTextEvent() }

// testPollEvent runs the real poll. Only safe with a non-empty text
// queue: the drain returns before any NSApp access. The autorelease pool
// it pushes is a plain C call, safe on any thread.
func testPollEvent() bool { return metalPollEvent(0) }

// testQuitActionSetsQuitEvent sends quit: to the delegate and checks it
// sets both ev.typ and quitRequested. quitRequested is how the poll sees
// a quit whose event is not in the default mode (a menu-bar click, a
// system termination). Without it the quit is lost.
func testQuitActionSetsQuitEvent() bool {
	d := objs.app.Send(darwinffi.Sel("delegate"))
	if d == 0 {
		return false
	}
	pollState.ev.typ = metalEventNone
	pollState.quitRequested = false
	d.Send(darwinffi.Sel("quit:"), objc.ID(0))
	return pollState.ev.typ == metalEventQuit && pollState.quitRequested
}

// testAppShouldTerminateCorrect sends applicationShouldTerminate: and
// checks it returns NSTerminateCancel and sets the quit, as quit: does.
func testAppShouldTerminateCorrect() bool {
	d := objs.app.Send(darwinffi.Sel("delegate"))
	if d == 0 {
		return false
	}
	pollState.ev.typ = metalEventNone
	pollState.quitRequested = false
	reply := objc.Send[uint](d, darwinffi.Sel("applicationShouldTerminate:"), objs.app)
	return reply == nsTerminateCancel && pollState.ev.typ == metalEventQuit &&
		pollState.quitRequested
}

// testPollReturnsOnQuitRequested checks metalPollEvent returns at once
// when quitRequested is set, before any dequeue, and takes the flag. The
// regression test for the out-of-band quit path.
func testPollReturnsOnQuitRequested() bool {
	pollState.ev.typ = metalEventNone
	pollState.quitRequested = true
	got := metalPollEvent(0)
	return got && pollState.ev.typ == metalEventQuit && !pollState.quitRequested
}

// testPollIdleWake checks the endless idle wait blocks and wakes: another
// goroutine calls wakeUp after 100 ms, as the animation loop and
// QueueCommand do, and metalPollEvent(-1) must sleep until then. Issue
// #405: the idle loop stopped polling at 100 ms, so every path that
// dirties state must wake it. 0 is success, 1 the poll returned no event,
// 2 the poll did not block.
func testPollIdleWake() int {
	for metalPollEvent(0) {
	}
	go func() {
		time.Sleep(100 * time.Millisecond)
		wakeUp()
	}()
	t0 := time.Now()
	got := metalPollEvent(-1)
	elapsed := time.Since(t0)
	switch {
	case !got:
		return 1
	case elapsed < 50*time.Millisecond:
		return 2
	}
	return 0
}

// mallocStats is malloc_statistics_t.
type mallocStats struct {
	blocksInUse  uint32
	sizeInUse    uintptr
	maxSizeInUse uintptr
	allocated    uintptr
}

// testPollWakeGrowth checks metalPollEvent drains the autorelease pool
// its dequeue fills. The Go loop has no Cocoa run loop to drain a pool,
// so an object autoreleased in the poll (the dequeued NSEvent, the wake
// event) lives for ever without one: about 60 NSEvents a second under a
// repeating animation (solar_system example). It runs the real wake path
// cycles times and returns the growth of the malloc zones in bytes. -1
// means a wake did not arrive within 1 s: a dead wake path would allocate
// nothing and pass.
func testPollWakeGrowth(cycles int) int {
	for metalPollEvent(0) {
	}
	wakeUp()
	if !metalPollEvent(1000) {
		return -1
	}
	for metalPollEvent(0) {
	}

	stats := darwinffi.Func(darwinffi.Open("/usr/lib/libSystem.B.dylib"), "malloc_zone_statistics")
	before, after := new(mallocStats), new(mallocStats)
	msgCall(stats, 0, uintptr(unsafe.Pointer(before)))
	for range cycles {
		wakeUp()
		if !metalPollEvent(1000) {
			return -1
		}
		for metalPollEvent(0) {
		}
	}
	msgCall(stats, 0, uintptr(unsafe.Pointer(after)))
	return int(after.sizeInUse) - int(before.sizeInUse)
}

// testCursorBoundsCheck reports whether metalWindowSetCursor would
// reject the point.
func testCursorBoundsCheck(mouseX, mouseY, width, height float32) bool {
	return !cursorInContentBounds(mouseX, mouseY, width, height)
}

func testFocusedGUIWindowMatches(w *macWindow) bool {
	return focusedWindow() == w
}

// testApplicationDidBecomeActive sends applicationDidBecomeActive: to the
// delegate. The regression test for focus after an app switch when
// keyWindow is nil.
func testApplicationDidBecomeActive() bool {
	d := objs.app.Send(darwinffi.Sel("delegate"))
	if d == 0 {
		return false
	}
	d.Send(darwinffi.Sel("applicationDidBecomeActive:"), objc.ID(0))
	return true
}

// testFramePumpActive reports whether the frame-pump timer is armed.
func testFramePumpActive() bool {
	return framePump.timer != 0 && objc.Send[bool](framePump.timer, darwinffi.Sel("isValid"))
}

// testRunMode runs the main run loop in mode for ms milliseconds, as
// AppKit does inside [NSAlert runModal] or menu tracking.
// runMode:beforeDate: returns once it serves one source (and at once when
// the mode has none), so it loops until the deadline.
func testRunMode(mode objc.ID, ms int) {
	pool := darwinffi.PoolPush()
	defer darwinffi.PoolPop(pool)
	loop := darwinffi.Class("NSRunLoop").Send(darwinffi.Sel("mainRunLoop"))
	deadline := time.Now().Add(time.Duration(ms) * time.Millisecond)
	for time.Now().Before(deadline) {
		until := msgDateSinceNow(time.Until(deadline).Seconds())
		loop.Send(darwinffi.Sel("runMode:beforeDate:"), mode, until)
	}
}

func appKitString(name string) objc.ID {
	return objc.ID(darwinffi.Data(darwinffi.Open(darwinffi.PathAppKit), name))
}

// testRunModalMode runs NSModalPanelRunLoopMode, where the pump timer
// must fire. testRunDefaultMode is the negative control.
func testRunModalMode(ms int)   { testRunMode(appKitString("NSModalPanelRunLoopMode"), ms) }
func testRunDefaultMode(ms int) { testRunMode(objs.defaultMode, ms) }

// testTimer is the target of the one-shot timer of
// testNestedModesPumpStaysArmed.
var testTimer struct {
	once     sync.Once
	cls      objc.Class
	disarmed bool
}

// testNestedModesPumpStaysArmed tests the depth count: while a nested
// mode runs, a second nested mode (a menu over a modal dialog) must keep
// the pump armed. Only the exit of the last nested mode may disarm it. It
// runs the modal mode. 60 ms in, a one-shot timer enters the tracking
// mode for 30 ms and checks the timer on both sides of that change. It
// returns true if the timer was disarmed in the nesting.
func testNestedModesPumpStaysArmed() bool {
	testTimer.once.Do(func() {
		testTimer.cls = mustClass("GoGuiTestNestedModeTarget", "NSObject", nil, []objc.MethodDef{{
			Cmd: darwinffi.Sel("fire:"),
			Fn: func(objc.ID, objc.SEL, objc.ID) {
				if !testFramePumpActive() {
					testTimer.disarmed = true
				}
				// Enter the tracking mode while the modal mode is still
				// active (depth 1 to 2), then let it exit (2 to 1): the
				// pump must stay armed.
				until := msgDateSinceNow(0.03)
				darwinffi.Class("NSRunLoop").Send(darwinffi.Sel("mainRunLoop")).
					Send(darwinffi.Sel("runMode:beforeDate:"),
						appKitString("NSEventTrackingRunLoopMode"), until)
				if !testFramePumpActive() {
					testTimer.disarmed = true
				}
			},
		}})
	})
	testTimer.disarmed = false
	pool := darwinffi.PoolPush()
	target := objc.ID(testTimer.cls).Send(sels.new).Send(sels.autorelease)
	inner := darwinffi.Class("NSTimer").Send(
		darwinffi.Sel("timerWithTimeInterval:target:selector:userInfo:repeats:"),
		0.06, target, darwinffi.Sel("fire:"), objc.ID(0), false)
	// Fires only in the modal mode, the mode under test.
	darwinffi.Class("NSRunLoop").Send(darwinffi.Sel("mainRunLoop")).
		Send(darwinffi.Sel("addTimer:forMode:"), inner, appKitString("NSModalPanelRunLoopMode"))
	darwinffi.PoolPop(pool)
	testRunModalMode(150)
	return testTimer.disarmed
}

// testPressAndHoldRegistered reports ApplePressAndHoldEnabled in the
// registration domain: 0 registered off, 1 on, -1 not registered. Not
// the effective value on purpose: a developer who set NSGlobalDomain
// would otherwise fail this test for a preference the fix leaves them.
func testPressAndHoldRegistered() int {
	pool := darwinffi.PoolPush()
	defer darwinffi.PoolPop(pool)
	domain := objc.ID(darwinffi.Data(darwinffi.Open(darwinffi.PathFoundation), "NSRegistrationDomain"))
	reg := darwinffi.Class("NSUserDefaults").Send(darwinffi.Sel("standardUserDefaults")).
		Send(darwinffi.Sel("volatileDomainForName:"), domain)
	v := reg.Send(darwinffi.Sel("objectForKey:"), darwinffi.NSString("ApplePressAndHoldEnabled"))
	if v == 0 {
		return -1
	}
	if objc.Send[bool](v, darwinffi.Sel("boolValue")) {
		return 1
	}
	return 0
}
