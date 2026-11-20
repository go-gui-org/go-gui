//go:build darwin && cgo && !ios

package metal

// The event poll: one NSEvent at a time from the main run loop, copied
// into pollState.ev for mapMetalEvent, plus the text events of the IME.
// Main-thread only, except wakeUp and mainQueueAsync, which any goroutine
// can call. See window_darwin.go.

import (
	"sync"

	"github.com/ebitengine/purego"
	"github.com/ebitengine/purego/objc"

	"github.com/go-gui-org/go-gui/gui/internal/darwinffi"
)

// Event types of pollState.ev, read by mapMetalEvent.
const (
	metalEventNone = iota
	metalEventQuit
	metalEventMouseDown
	metalEventMouseUp
	metalEventMouseMove
	metalEventScrollWheel
	metalEventKeyDown
	metalEventKeyUp
	metalEventFlagsChanged
	metalEventChar       // IME committed text
	metalEventIMEComp    // IME composition in progress
	metalEventMouseLeave // pointer left the content view
)

// NSEventType values.
const (
	nsEventLeftMouseDown     = 1
	nsEventLeftMouseUp       = 2
	nsEventRightMouseDown    = 3
	nsEventRightMouseUp      = 4
	nsEventMouseMoved        = 5
	nsEventLeftMouseDragged  = 6
	nsEventRightMouseDragged = 7
	nsEventMouseExited       = 9
	nsEventKeyDown           = 10
	nsEventKeyUp             = 11
	nsEventFlagsChanged      = 12
	nsEventScrollWheel       = 22
	nsEventOtherMouseDown    = 25
	nsEventOtherMouseUp      = 26
	nsEventOtherMouseDragged = 27
)

// NSEventPhase values.
const (
	nsEventPhaseBegan     = 1
	nsEventPhaseEnded     = 8
	nsEventPhaseCancelled = 16
	nsEventPhaseMayBegin  = 32
)

// nsEventMaskAny is NSEventMaskAny, the mask of both polls. It must be the
// mask [NSApp run] uses: AppKit dispatches Apple Events from
// nextEventMatchingMask: only under it. With an explicit list (even one
// that names every NSEventMask bit) a document opened from Finder while
// the app runs never reaches application:openURLs: (issue #1106; checked
// with a 40-line native loop). Types storeEvent does not know take its
// default branch and still go to sendEvent:, as under [NSApp run].
const nsEventMaskAny = ^uintptr(0)

// Scale for AppKit precise (trackpad, high-res) scroll deltas, before
// they reach gui's ScrollMultiplier (20). AppKit reports precise deltas
// in points; with the gui multiplier this lands trackpad motion at about
// 1.5x raw points, close to 1:1. This value is coupled to gui's
// ScrollMultiplier: it divides most of that gain back out for the
// precise path, which (unlike the wheel) gui does not smooth. See #22.
const preciseScrollScale = 0.075

// Scale for AppKit line-based (mouse wheel) scroll deltas, to gui.Event's
// line unit. AppKit reports about 1 line per notch (more with
// acceleration); every other backend reports three lines per notch, so
// that is the factor. With gui's ScrollMultiplier (20) one notch is about
// 60px, which gui eases into place. It was 2.5 (~50px per notch) when the
// unit was not defined and each backend chose its own: Win32 and X11
// reported 1.0, so macOS scrolled 2.5x faster. It was 10.0 before that.
const wheelScrollScale = 3.0

// metalEvent is the event mapMetalEvent reads.
type metalEvent struct {
	typ                              int
	windowID                         uint32
	mouseX, mouseY, mouseDX, mouseDY float32
	mouseButton                      int // 0 left, 1 right, 2 and up other
	clickCount                       int
	scrollX, scrollY                 float32
	scrollPhase                      int // 0 normal, 1 may begin, 2 began, 3 ended
	scrollPrecise                    bool
	keyCode                          uint16 // macOS virtual key code
	modifiers                        uint32 // NSEventModifierFlags
	keyRepeat                        bool
	text                             string // UTF-8
	imeStart, imeLength              int    // composition, in code points
}

// textEvent is one queued NSTextInputClient callback.
type textEvent struct {
	typ        int // metalEventChar or metalEventIMEComp
	windowID   uint32
	text       string
	start, len int
	// modifiers is a snapshot: ev.modifiers moves on before the pop.
	modifiers uint32
}

// textQueueCap bounds the text queue. One sendEvent: gives a handful of
// callbacks, and the queue drains on each poll.
const textQueueCap = 32

// pollState is the state of the poll. Main-thread only.
var pollState struct {
	ev metalEvent

	// quitRequested is set by quit: and applicationShouldTerminate:.
	quitRequested bool

	// liveResizeStarted is set by windowWillStartLiveResize: and read
	// after sendEvent:. AppKit's resize tracking loop runs inside
	// sendEvent:, so the flag says the mouse-down already stored was
	// used to drive a window resize.
	liveResizeStarted bool

	// imeConsumedKey is set by keyDown: when an input method owned the
	// keystroke. While a composition is live the IME owns the keyboard
	// (arrows move between conversion clauses, Enter commits, Escape
	// reverts), so the raw key must not also reach the widget, or the
	// caret of the field moves under the preedit.
	imeConsumedKey bool

	// imeDeclinedKey is set by doCommandBySelector: during
	// interpretKeyEvents:, and cleared by keyDown: before it. The input
	// source calls it to give a key back: it looked at the key, declined
	// it as text input, and returned it as a command for the widget. The
	// key is then the widget's, even during a composition (issue #393).
	imeDeclinedKey bool

	// deferred is a key-down held back for one poll. When the input
	// source declines a key that also committed a composition (Tab on a
	// pending dead key commits the accent through insertText: and
	// returns insertTab:), the commit is already queued and must reach
	// Go first. Tab moves focus, so the key first would put the
	// committed text in the widget that just took focus.
	deferred    bool
	deferredKey metalEvent

	// lastMouseDown is the last mouse-down NSEvent, +1, for
	// metalWindowStartDrag: performWindowDragWithEvent: wants the press
	// that started the gesture, and Go only has the coordinates.
	lastMouseDown objc.ID

	// The text queue. A single sendEvent: can fire several
	// NSTextInputClient callbacks: for CJK, insertText: (the commit) and
	// then setMarkedText: (a remaining composition). ev is one slot, so
	// the earlier callback would be lost. The queue gives them to Go one
	// per poll.
	textQ     [textQueueCap]textEvent
	textHead  int // index of the oldest entry
	textCount int
}

// pushTextEvent queues a text event. An empty text is how the IME reports
// "preedit ended".
func pushTextEvent(typ int, text string, wid uint32, start, length int) {
	ps := &pollState
	// A full queue drops the oldest entry, not the new one, so the most
	// recent input state stays.
	if ps.textCount == textQueueCap {
		ps.textQ[ps.textHead] = textEvent{}
		ps.textHead = (ps.textHead + 1) % textQueueCap
		ps.textCount--
	}
	ps.textQ[(ps.textHead+ps.textCount)%textQueueCap] = textEvent{
		typ: typ, windowID: wid, text: text, start: start, len: length,
		modifiers: ps.ev.modifiers,
	}
	ps.textCount++
}

// popTextEvent moves the oldest queued text event into ev. It reports
// whether there was one.
func popTextEvent() bool {
	ps := &pollState
	if ps.textCount == 0 {
		return false
	}
	e := ps.textQ[ps.textHead]
	ps.textQ[ps.textHead] = textEvent{}
	ps.textHead = (ps.textHead + 1) % textQueueCap
	ps.textCount--
	ps.ev.typ = e.typ
	ps.ev.windowID = e.windowID
	ps.ev.text = e.text
	ps.ev.imeStart = e.start
	ps.ev.imeLength = e.len
	ps.ev.modifiers = e.modifiers
	return true
}

// resetTextQueue drops every queued text event.
func resetTextQueue() {
	ps := &pollState
	ps.textQ = [textQueueCap]textEvent{}
	ps.textHead, ps.textCount = 0, 0
	ps.ev.typ = metalEventNone
}

// storeEvent copies what Go reads from event into pollState.ev. It runs
// for every event and must not allocate (TestStoreEventDoesNotAllocate).
func storeEvent(event objc.ID, wid uint32) {
	ev := &pollState.ev
	ev.typ = metalEventNone
	ev.windowID = wid
	ev.text = ""
	if event == 0 {
		return
	}

	// locationInWindow has its origin at the bottom left, and gui uses
	// the top left. Flip once here.
	winH := 0.0
	if win := objc.ID(msg(event, sels.window)); win != 0 {
		if cv := objc.ID(msg(win, sels.contentView)); cv != 0 {
			winH = msgRect(cv, sels.bounds).H
		}
	}
	loc := msgPoint(event, sels.locationInWindow)
	x, y := float32(loc.X), float32(winH-loc.Y)

	switch typ := msg(event, sels.eventType); typ {
	case nsEventLeftMouseDown, nsEventRightMouseDown, nsEventOtherMouseDown:
		// Kept for metalWindowStartDrag.
		msg(event, sels.retain)
		if pollState.lastMouseDown != 0 {
			msg(pollState.lastMouseDown, sels.release)
		}
		pollState.lastMouseDown = event
		ev.typ = metalEventMouseDown
		ev.mouseX, ev.mouseY = x, y
		ev.clickCount = int(msg(event, sels.clickCount))
		ev.modifiers = uint32(msg(event, sels.modifierFlags))
		ev.mouseButton = mouseButtonOf(event, typ)

	case nsEventLeftMouseUp, nsEventRightMouseUp, nsEventOtherMouseUp:
		ev.typ = metalEventMouseUp
		ev.mouseX, ev.mouseY = x, y
		ev.modifiers = uint32(msg(event, sels.modifierFlags))
		ev.mouseButton = mouseButtonOf(event, typ)

	case nsEventMouseMoved, nsEventLeftMouseDragged, nsEventRightMouseDragged,
		nsEventOtherMouseDragged:
		ev.typ = metalEventMouseMove
		ev.mouseX, ev.mouseY = x, y
		ev.mouseDX = float32(msgF64(event, sels.deltaX))
		ev.mouseDY = float32(msgF64(event, sels.deltaY))
		ev.modifiers = uint32(msg(event, sels.modifierFlags))

	case nsEventMouseExited:
		// Posted by the tracking area of the content view when the
		// pointer leaves it, so nothing stays hovered (issue #587).
		ev.typ = metalEventMouseLeave

	case nsEventScrollWheel:
		ev.typ = metalEventScrollWheel
		ev.mouseX, ev.mouseY = x, y
		ev.modifiers = uint32(msg(event, sels.modifierFlags))
		switch msg(event, sels.phase) {
		case nsEventPhaseMayBegin:
			ev.scrollPhase = 1
		case nsEventPhaseBegan:
			ev.scrollPhase = 2
		case nsEventPhaseEnded, nsEventPhaseCancelled:
			ev.scrollPhase = 3
		default:
			ev.scrollPhase = 0
		}
		// A precise device reports points. A mouse wheel reports lines,
		// scaled toward pixels.
		scale := float32(wheelScrollScale)
		ev.scrollPrecise = msg(event, sels.hasPrecise)&0xff != 0
		if ev.scrollPrecise {
			scale = preciseScrollScale
		}
		ev.scrollX = float32(msgF64(event, sels.scrollingDeltaX)) * scale
		ev.scrollY = float32(msgF64(event, sels.scrollingDeltaY)) * scale

	case nsEventKeyDown:
		ev.typ = metalEventKeyDown
		ev.keyCode = uint16(msg(event, sels.keyCode))
		ev.modifiers = uint32(msg(event, sels.modifierFlags))
		ev.keyRepeat = msg(event, sels.isARepeat)&0xff != 0

	case nsEventKeyUp:
		ev.typ = metalEventKeyUp
		ev.keyCode = uint16(msg(event, sels.keyCode))
		ev.modifiers = uint32(msg(event, sels.modifierFlags))

	case nsEventFlagsChanged:
		ev.typ = metalEventFlagsChanged
		ev.keyCode = uint16(msg(event, sels.keyCode))
		ev.modifiers = uint32(msg(event, sels.modifierFlags))
	}
}

// mouseButtonOf is 0 for left, 1 for right, and the button number for
// any other button: 2 is middle, and Go maps 3 and up (back, forward) to
// MouseInvalid (#1098).
func mouseButtonOf(event objc.ID, typ uintptr) int {
	switch typ {
	case nsEventLeftMouseDown, nsEventLeftMouseUp:
		return 0
	case nsEventRightMouseDown, nsEventRightMouseUp:
		return 1
	}
	return int(msg(event, sels.buttonNum))
}

// metalPollEvent takes one event. It reports whether pollState.ev holds
// one. timeoutMs 0 does not wait, -1 waits until an event comes, and a
// positive value waits up to that many milliseconds.
//
// The poll runs in an autorelease pool. The Go event loop has no Cocoa
// run loop, so nothing else drains a pool on this thread.
// nextEventMatchingMask: returns an autoreleased NSEvent, and the wake
// that wakeUp schedules (it runs inside the dequeue) autoreleases the
// event it builds. Without the pool each one lives for ever: under a
// repeating animation that is one wake event per frame. storeEvent
// copies all that Go reads, so no event needs to outlive the pool.
func metalPollEvent(timeoutMs int) bool {
	pool := msgCall(darwinffi.PoolPushAddr())
	got := pollEvent(timeoutMs)
	msgCall(darwinffi.PoolPopAddr(), pool)
	return got
}

func pollEvent(timeoutMs int) bool {
	ps := &pollState

	// A quit request from the app delegate (quit:,
	// applicationShouldTerminate:). It can fire with no event in the
	// default mode (menu-bar quit, logout), so the dequeue below would not
	// see it. The flag is taken here, so a vetoed quit does not fire
	// again on the next poll.
	if ps.quitRequested {
		ps.quitRequested = false
		ps.ev.typ = metalEventQuit
		ps.ev.windowID = 0
		return true
	}

	// Text events queued by a previous sendEvent: go before the NSEvent
	// queue. This keeps a keystroke with several callbacks (a CJK commit
	// and a remaining composition) in order.
	if popTextEvent() {
		return true
	}

	// The text queue is empty: release a key-down held back so the
	// composition it committed reached Go first.
	if ps.deferred {
		ps.deferred = false
		ps.ev = ps.deferredKey
		return true
	}

	event := objc.ID(msg(objs.app, sels.nextEvent, nsEventMaskAny, 0,
		uintptr(objs.defaultMode), 1))
	if event == 0 && timeoutMs != 0 {
		until := objs.distantFuture
		if timeoutMs > 0 {
			until = msgDateSinceNow(float64(timeoutMs) / 1000)
		}
		event = objc.ID(msg(objs.app, sels.nextEvent, nsEventMaskAny,
			uintptr(until), uintptr(objs.defaultMode), 1))
	}
	if event == 0 {
		// IME callbacks can fire during the wait (the candidate window
		// runs the run loop), so look at the queue again.
		return popTextEvent()
	}

	var wid uint32
	if w := windowOf(objc.ID(msg(event, sels.window))); w != nil {
		wid = w.id
	}
	storeEvent(event, wid)

	ps.liveResizeStarted = false
	ps.imeConsumedKey = false

	// AppKit gets every event, for window management and text input.
	// keyDown: → interpretKeyEvents: → insertText: for printable keys, or
	// doCommandBySelector: for others. Mouse and scroll events take the
	// usual AppKit route (resize, close, focus).
	msg(objs.app, sels.sendEvent, uintptr(event))

	// Drop a mouse-down that AppKit used to resize the window. The resize
	// grab area overlaps the content view by a few points, so storeEvent
	// (which runs before sendEvent:, with no way to know where AppKit
	// will route the press) gives Go a plain click. A widget that fills
	// the window then treats it as a real press: a text input starts a
	// drag-selection and calls MouseLock. The mouse-up never arrives,
	// because AppKit's resize loop inside sendEvent: takes it, so the
	// drag stays locked for ever.
	if ps.liveResizeStarted && ps.ev.typ == metalEventMouseDown {
		ps.ev.typ = metalEventNone
	}
	ps.liveResizeStarted = false

	// Drop a key the input method took. Its visible effect comes as the
	// queued composition or commit.
	switch {
	case ps.imeConsumedKey && ps.ev.typ == metalEventKeyDown:
		ps.ev.typ = metalEventNone
	case ps.imeDeclinedKey && ps.textCount > 0 && ps.ev.typ == metalEventKeyDown:
		// The input source declined this key but committed text on the
		// way: Tab on a pending dead key commits the accent, then returns
		// insertTab:. Hold the key until the commit is delivered: Tab
		// moves focus, and the accent belongs to the field being left.
		ps.deferred = true
		ps.deferredKey = ps.ev
		ps.deferredKey.text = ""
		ps.ev.typ = metalEventNone
	}
	ps.imeConsumedKey = false

	// sendEvent: can have queued text events. Return the NSEvent's own
	// event now, and let the drain at the top of the next poll deliver
	// them, so a printable key gives KEY_DOWN then CHAR, the order X11,
	// win32 and web use. If the NSEvent gave nothing of its own, deliver
	// a queued event at once rather than an empty poll.
	if ps.ev.typ == metalEventNone {
		popTextEvent()
	}
	return true
}

// ─── Main queue and wake ───────────────────────────────────────

// mainQueue sends work to the main dispatch queue, which the run loop
// serves inside metalPollEvent and inside nested run loops (modal
// dialogs, menu tracking, live resize).
var mainQueue struct {
	once sync.Once

	asyncF uintptr // dispatch_async_f
	queue  uintptr // &_dispatch_main_q, what dispatch_get_main_queue returns
	wakeCB uintptr // posts the wake event
	runCB  uintptr // runs the oldest fns entry

	mu     sync.Mutex
	caller darwinffi.Caller // guarded by mu
	fns    []func()         // guarded by mu
}

func loadMainQueue() {
	mainQueue.once.Do(func() {
		libSystem := darwinffi.Open("/usr/lib/libSystem.B.dylib")
		mainQueue.asyncF = darwinffi.Func(libSystem, "dispatch_async_f")
		mainQueue.queue = darwinffi.Func(libSystem, "_dispatch_main_q")
		mainQueue.wakeCB = purego.NewCallback(func(uintptr) { postWakeEvent() })
		mainQueue.runCB = purego.NewCallback(func(uintptr) { runMainQueueFn() })
	})
}

// wakeUp posts an empty event, so the idle poll returns and the loop can
// repaint. Any goroutine can call it: the animation loop does, once per
// frame. It does not allocate (TestWakeUpDoesNotAllocate). It sends no
// Objective-C message on the calling thread: dispatch_async_f is a C call
// that is safe from any thread, and the event is built on the main
// thread.
func wakeUp() {
	loadMainQueue()
	mainQueue.mu.Lock()
	mainQueue.caller.Call(mainQueue.asyncF, mainQueue.queue, 0, mainQueue.wakeCB)
	mainQueue.mu.Unlock()
}

// postWakeEvent posts the wake event. Main thread, from the main queue.
func postWakeEvent() {
	if objs.app == 0 {
		return
	}
	msg(objs.app, sels.postEvent, uintptr(msgWakeEvent()), 0)
}

// mainQueueAsync runs fn on the main thread, from the main queue, after
// the work queued before it. Any goroutine can call it.
func mainQueueAsync(fn func()) {
	loadMainQueue()
	mainQueue.mu.Lock()
	mainQueue.fns = append(mainQueue.fns, fn)
	mainQueue.caller.Call(mainQueue.asyncF, mainQueue.queue, 0, mainQueue.runCB)
	mainQueue.mu.Unlock()
}

// runMainQueueFn runs the oldest queued fn. Each mainQueueAsync makes one
// dispatch, so each call runs one fn, in order.
func runMainQueueFn() {
	mainQueue.mu.Lock()
	if len(mainQueue.fns) == 0 {
		mainQueue.mu.Unlock()
		return
	}
	fn := mainQueue.fns[0]
	mainQueue.fns[0] = nil
	mainQueue.fns = mainQueue.fns[1:]
	mainQueue.mu.Unlock()

	pool := darwinffi.PoolPush()
	defer darwinffi.PoolPop(pool)
	fn()
}
