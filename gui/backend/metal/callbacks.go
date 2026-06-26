//go:build darwin && !ios

package metal

/*
#include <stdlib.h>
#include "metal_window.h"

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
int metalTestCursorBoundsCheck(float mouseX, float mouseY,
                               float width, float height);
int metalTestMenuAboutExists(void);
int metalTestWindowsMenuExists(void);
int metalTestFocusedGUIWindowMatches(void *windowHandle);
int metalTestApplicationDidBecomeActive(void);
*/
import "C"
import (
	"sync"
	"unsafe"

	"github.com/go-gui-org/go-gui/gui"
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

// ─── Test helpers ──────────────────────────────────────────────

func testActivationPolicyRegular() bool {
	return C.metalTestActivationPolicyIsRegular() != 0
}

func testDelegateSet() bool {
	return C.metalTestDelegateIsSet() != 0
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

func testWindowID(handle C.GoGuiNSWindow) uint32 {
	return uint32(C.metalWindowGetID(handle))
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
}
