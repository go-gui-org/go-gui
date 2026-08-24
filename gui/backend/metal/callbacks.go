//go:build darwin && !ios

package metal

/*
#include <stdlib.h>
#include "metal_window.h"
// For metalCompileShadersProbe (test helper, defined in metal_darwin.m).
#include "metal_darwin.h"

// Test helpers — defined in metal_window.m.
int metalTestActivationPolicyIsRegular(void);
int metalTestDelegateIsSet(void);
int metalTestMainMenuExists(void);
int metalTestMenuQuitWired(void);
int metalTestWindowDelegateExists(void *windowHandle);
void metalTestInjectKeyDown(unsigned short keyCode, unsigned int modifiers);
void metalTestInjectQuitEvent(void);
int metalTestQuitActionSetsQuitEvent(void);
int metalTestAppShouldTerminateCorrect(void);
int metalTestPollReturnsOnQuitRequested(void);
int metalTestPollIdleWake(void);
int metalTestCursorBoundsCheck(float mouseX, float mouseY,
                               float width, float height);
int metalTestMenuAboutExists(void);
int metalTestWindowsMenuExists(void);
int metalTestFocusedGUIWindowMatches(void *windowHandle);
int metalTestApplicationDidBecomeActive(void);
void metalTestRunModalMode(int ms);
void metalTestRunDefaultMode(int ms);
int metalTestFramePumpActive(void);
int metalTestPressAndHoldRegistered(void);
void metalTestPushIMECommit(const char *utf8);
void metalTestPushIMEComposition(const char *utf8, int start, int len);
int metalTestIMEQueueDepth(void);
void metalTestResetIMEQueue(void);
int metalTestPopTextEvent(void);
int metalTestIMEClientConformance(void *windowHandle);
int metalTestIMEKeySuppressedWhileComposing(void *windowHandle);
int metalTestIMEDeadKeyDeliveredWithoutTextContext(void *windowHandle);
int metalTestContentViewIsFirstResponder(void *windowHandle);
void metalTestStartLiveResize(void *windowHandle);
void metalTestEndLiveResize(void *windowHandle);
int metalTestLayerPresentsWithTransaction(void *windowHandle);
*/
import "C"
import (
	"sync"
	"sync/atomic"
	"unsafe"

	"github.com/go-gui-org/go-gui/gui"
	"github.com/go-gui-org/go-gui/gui/backend/internal/msl"
)

// windowRegistry maps window IDs to windowState pointers.
// All mutations and reads happen on the main thread (the event
// loop goroutine). The mutex guards against the window-open path
// which writes from the calling goroutine before the main loop
// picks up the pending window.
var (
	windowRegistry   = make(map[uint32]*windowState)
	windowRegistryMu sync.Mutex
)

func registerWindow(id uint32, ws *windowState) {
	windowRegistryMu.Lock()
	windowRegistry[id] = ws
	windowRegistryMu.Unlock()
}

func unregisterWindow(id uint32) {
	windowRegistryMu.Lock()
	delete(windowRegistry, id)
	windowRegistryMu.Unlock()
}

func lookupWindow(id uint32) *windowState {
	windowRegistryMu.Lock()
	ws := windowRegistry[id]
	windowRegistryMu.Unlock()
	return ws
}

// pumpScratch is the reusable snapshot buffer for goMetalPumpFrames, so a
// 60 Hz pump allocates nothing. Main-thread only, like the pump itself.
var pumpScratch []*windowState

// framePumpCount counts goMetalPumpFrames invocations that actually ran
// (i.e. reached Go from a nested runloop). Tests read it to prove the
// timer fires in nested modes and stays quiet in the default mode.
var framePumpCount atomic.Uint64

//export goMetalPumpFrames
func goMetalPumpFrames() {
	framePumpCount.Add(1)

	// Snapshot under the lock: PumpFrame runs app code, which may open or
	// close windows and so mutate the registry.
	windowRegistryMu.Lock()
	pumpScratch = pumpScratch[:0]
	for _, ws := range windowRegistry {
		pumpScratch = append(pumpScratch, ws)
	}
	windowRegistryMu.Unlock()

	for _, ws := range pumpScratch {
		w := ws.attachedWindow
		if w == nil || ws.ctx == nil {
			continue
		}
		// PumpFrame, not FrameFn: it declines the frame instead of
		// deadlocking when the stack that entered the nested runloop
		// already holds the window lock.
		if w.PumpFrame() {
			ws.renderFrame(w)
		}
	}
	clear(pumpScratch) // drop references to destroyed windows
}

// ─── Test helpers ──────────────────────────────────────────────

func testActivationPolicyRegular() bool {
	return C.metalTestActivationPolicyIsRegular() != 0
}

func testDelegateSet() bool {
	return C.metalTestDelegateIsSet() != 0
}

// testCompileShaders compiles the built-in MSL library standalone at
// the pinned language version. 0 = compiled, -1 = compile failed,
// 1 = no Metal device. Lives here because cgo is not permitted in
// _test.go files. See TestBuiltinShadersCompile.
func testCompileShaders() int {
	cMSL := C.CString(msl.Source)
	defer C.free(unsafe.Pointer(cMSL))
	return int(C.metalCompileShadersProbe(cMSL))
}

func testMenuExists() bool {
	return C.metalTestMainMenuExists() != 0
}

func testMenuQuitWired() bool {
	return C.metalTestMenuQuitWired() != 0
}

func testWindowDelegateExists(handle C.GoGuiNSWindow) bool {
	return C.metalTestWindowDelegateExists(unsafe.Pointer(handle)) != 0
}

// testIMEClientConformance checks the NSTextInputClient answers a
// Japanese IME needs to commit converted text. Needs a real window, so
// it runs from runMainThreadTests.
func testIMEClientConformance(handle C.GoGuiNSWindow) bool {
	return C.metalTestIMEClientConformance(unsafe.Pointer(handle)) != 0
}

// testIMEKeySuppressedWhileComposing checks that a key the input
// method owns does not also reach the widget.
func testIMEKeySuppressedWhileComposing(handle C.GoGuiNSWindow) bool {
	return C.metalTestIMEKeySuppressedWhileComposing(
		unsafe.Pointer(handle)) != 0
}

// testIMEDeadKeyDeliveredWithoutTextContext checks that an Option key
// that produces a dead-key preedit still reaches the app as a key-down
// when no editable text widget is focused (issue #393).
func testIMEDeadKeyDeliveredWithoutTextContext(handle C.GoGuiNSWindow) bool {
	return C.metalTestIMEDeadKeyDeliveredWithoutTextContext(
		unsafe.Pointer(handle)) != 0
}

// testContentViewIsFirstResponder checks keys reach the view without
// the input method having been activated.
func testContentViewIsFirstResponder(handle C.GoGuiNSWindow) bool {
	return C.metalTestContentViewIsFirstResponder(
		unsafe.Pointer(handle)) != 0
}

func testWindowID(handle C.GoGuiNSWindow) uint32 {
	return uint32(C.metalWindowGetID(handle))
}

// testStartLiveResize / testEndLiveResize fire the live-resize
// delegate methods the way AppKit's tracking loop does. testLayer-
// PresentsWithTransaction reports the layer's presentation mode:
// 1 = transaction, 0 = async, -1 = no CAMetalLayer. Together they
// verify the async-presentation switch without a real drag, which
// AppKit does not expose programmatically.
func testStartLiveResize(handle C.GoGuiNSWindow) {
	C.metalTestStartLiveResize(unsafe.Pointer(handle))
}

func testEndLiveResize(handle C.GoGuiNSWindow) {
	C.metalTestEndLiveResize(unsafe.Pointer(handle))
}

func testLayerPresentsWithTransaction(handle C.GoGuiNSWindow) int {
	return int(C.metalTestLayerPresentsWithTransaction(
		unsafe.Pointer(handle)))
}

// testActivateNow calls C.metalAppFinishLaunch directly to verify
// the activation function exists, links, and runs without crashing.
func testActivateNow() {
	C.metalAppFinishLaunch()
}

// testInjectKeyDown sets up the C event globals to simulate a
// key-down event, so mapMetalEvent can be tested without a
// running event loop.
func testInjectKeyDown(keyCode uint16, modifiers uint32) {
	C.metalTestInjectKeyDown(C.ushort(keyCode), C.uint(modifiers))
}

// testInjectQuitEvent sets up the C event globals to simulate a
// quit event so mapMetalEvent's METAL_EVENT_QUIT branch can be
// tested without a running event loop.
func testInjectQuitEvent() {
	C.metalTestInjectQuitEvent()
}

// testPushIMECommit queues a committed-text event exactly as
// insertText: does, so tests can simulate several IME callbacks
// landing inside one sendEvent:.
func testPushIMECommit(text string) {
	ctext := C.CString(text)
	defer C.free(unsafe.Pointer(ctext))
	C.metalTestPushIMECommit(ctext)
}

// testPushIMEComposition queues a preedit-update event as
// setMarkedText: does. An empty text is the composition-end signal.
func testPushIMEComposition(text string, start, length int) {
	ctext := C.CString(text)
	defer C.free(unsafe.Pointer(ctext))
	C.metalTestPushIMEComposition(ctext, C.int(start), C.int(length))
}

// testIMEQueueDepth reports how many text events are still queued.
func testIMEQueueDepth() int {
	return int(C.metalTestIMEQueueDepth())
}

// testResetIMEQueue drops queued text events so one test cannot
// leak state into the next.
func testResetIMEQueue() {
	C.metalTestResetIMEQueue()
}

// testPopTextEvent drains one queued text event into the C event
// globals, bypassing metalPollEvent. Used where the queue may be
// empty: metalPollEvent would then fall through to NSApp, which is
// main-thread-only.
func testPopTextEvent() bool {
	return C.metalTestPopTextEvent() != 0
}

// testPollEvent runs the real metalPollEvent drain path. Only safe to
// call with a non-empty text queue — the drain returns before any
// NSApp access.
func testPollEvent() bool {
	return C.metalPollEvent(0) != 0
}

func testQuitActionSetsQuitEvent() bool {
	return C.metalTestQuitActionSetsQuitEvent() != 0
}

func testAppShouldTerminateCorrect() bool {
	return C.metalTestAppShouldTerminateCorrect() != 0
}

// testPollReturnsOnQuitRequested verifies metalPollEvent returns
// immediately when _quitRequested is set, before any event dequeue.
func testPollReturnsOnQuitRequested() bool {
	return C.metalTestPollReturnsOnQuitRequested() != 0
}

// testPollIdleWake verifies metalPollEvent(-1) blocks until a wake
// event arrives (issue #405). 0 on success, non-zero on failure.
// Main-thread only: it runs the real blocking poll.
func testPollIdleWake() int {
	return int(C.metalTestPollIdleWake())
}

func testCursorBoundsCheck(mouseX, mouseY, width, height float32) bool {
	return C.metalTestCursorBoundsCheck(C.float(mouseX), C.float(mouseY),
		C.float(width), C.float(height)) != 0
}

func testMenuAboutExists() bool {
	return C.metalTestMenuAboutExists() != 0
}

func testWindowsMenuExists() bool {
	return C.metalTestWindowsMenuExists() != 0
}

func testFocusedGUIWindowMatches(handle C.GoGuiNSWindow) bool {
	return C.metalTestFocusedGUIWindowMatches(unsafe.Pointer(handle)) != 0
}

func testApplicationDidBecomeActive() bool {
	return C.metalTestApplicationDidBecomeActive() != 0
}

// testFramePumpActive reports whether the nested-runloop frame-pump
// timer is currently installed.
func testFramePumpActive() bool { return C.metalTestFramePumpActive() != 0 }

// testPressAndHoldRegistered reports ApplePressAndHoldEnabled as
// registered by metalAppInit: 0 off, 1 on, -1 never registered.
// Press-and-hold suppresses the insertText: calls that back key repeat,
// so the fix registers it off.
func testPressAndHoldRegistered() int {
	return int(C.metalTestPressAndHoldRegistered())
}

// testRunModalMode spins the main runloop in NSModalPanelRunLoopMode —
// what AppKit does inside runModal — so the pump timer can fire there.
// testRunDefaultMode is the negative control.
func testRunModalMode(ms int)   { C.metalTestRunModalMode(C.int(ms)) }
func testRunDefaultMode(ms int) { C.metalTestRunDefaultMode(C.int(ms)) }

// ─── C callbacks (weak in metal_window.m, strong here) ───────

//export goMetalWindowResized
func goMetalWindowResized(wid C.uint, width, height C.int) {
	ws := lookupWindow(uint32(wid))
	if ws == nil {
		return
	}
	ws.handleResize()

	// Dispatch resize event to the attached window.
	if ws.attachedWindow != nil {
		w := ws.attachedWindow
		w32, h32 := int(width), int(height)
		if w32 < 0 {
			w32 = 0
		}
		if h32 < 0 {
			h32 = 0
		}
		w.EventFn(&gui.Event{
			Type:         gui.EventResized,
			WindowWidth:  w32,
			WindowHeight: h32,
		})
		w.FrameFn()
		ws.renderFrame(w)
	}
}

//export goMetalWindowShouldClose
func goMetalWindowShouldClose(wid C.uint) {
	ws := lookupWindow(uint32(wid))
	if ws == nil || ws.attachedWindow == nil {
		return
	}
	gui.DispatchCloseRequest(ws.attachedWindow)
	// windowShouldClose: returns NO to AppKit so the window
	// stays open until the Go event loop destroys it.
}

//export goMetalWindowFocusChanged
func goMetalWindowFocusChanged(wid C.uint, focused C.int) {
	ws := lookupWindow(uint32(wid))
	if ws == nil || ws.attachedWindow == nil {
		return
	}
	et := gui.EventUnfocused
	if focused != 0 {
		et = gui.EventFocused
	}
	ws.attachedWindow.EventFn(&gui.Event{Type: et})
	// windowDidBecomeKey:/windowDidResignKey: are notifications, not
	// NSEvents — the idle loop blocks in metalPollEvent(-1) on their
	// own, so the frame reflecting the focus change would wait for
	// the next real event. Post an empty event to bring the loop
	// round for the repaint (issue #405).
	C.metalPostEmptyEvent()
}

//export goMetalFileDrop
func goMetalFileDrop(wid C.uint, cpath *C.char) {
	// cpath is [NSString UTF8String] — internal autoreleased
	// buffer. Do NOT free.
	path := C.GoString(cpath)
	ws := lookupWindow(uint32(wid))
	if ws == nil || ws.attachedWindow == nil {
		return
	}
	ws.attachedWindow.EventFn(&gui.Event{
		Type:     gui.EventFileDropped,
		FilePath: path,
	})
	// performDragOperation: is delivered by the drag session, not an
	// NSEvent — wake the idle loop so the drop's repaint happens
	// promptly (issue #405).
	C.metalPostEmptyEvent()
}
