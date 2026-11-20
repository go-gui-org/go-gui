//go:build darwin && !ios

package metal

/*
#include <stdlib.h>
// For metalCompileShadersProbe (test helper, defined in metal_darwin.m).
#include "metal_darwin.h"
*/
import "C"
import (
	"log"
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
// pumpWindows holds it out of the global while in use, so a re-entrant pump
// cannot clobber it.
var pumpScratch []*windowState

// goMetalPumpFrames is the frame-pump timer action (app_darwin.go).
func goMetalPumpFrames() {
	framePumpCount.Add(1)
	pumpWindows(pumpWindow)
}

// pumpWindow renders one frame for ws if its window accepts one. A package
// func, not a closure, so passing it to pumpWindows allocates nothing.
func pumpWindow(ws *windowState) {
	w := ws.attachedWindow
	if w == nil || ws.ctx == nil {
		return
	}
	// PumpFrame, not FrameFn: it declines the frame instead of
	// deadlocking when the stack that entered the nested runloop
	// already holds the window lock.
	if w.PumpFrame() {
		ws.renderFrame(w)
	}
}

// pumpWindows calls pump for a snapshot of every registered window. The
// pump is a parameter so tests can re-enter it without Cocoa.
func pumpWindows(pump func(*windowState)) {
	// Take the shared buffer out of the global for this call. pump runs app
	// code, which can enter a deeper nested runloop whose frame timer calls
	// back in here on the same thread. That inner call must not truncate and
	// clear the slice this call is still walking, so it finds pumpScratch nil
	// and allocates its own buffer. Only real re-entry allocates; a plain
	// 60 Hz tick reuses the one buffer.
	buf := pumpScratch[:0]
	pumpScratch = nil

	// Snapshot under the lock: PumpFrame runs app code, which may open or
	// close windows and so mutate the registry.
	windowRegistryMu.Lock()
	for _, ws := range windowRegistry {
		buf = append(buf, ws)
	}
	windowRegistryMu.Unlock()

	for _, ws := range buf {
		pump(ws)
	}
	clear(buf) // drop references to destroyed windows

	// Give the buffer back. A re-entrant call may already have returned its
	// own; keep the larger so steady state stays allocation-free.
	if cap(buf) >= cap(pumpScratch) {
		pumpScratch = buf[:0]
	}
}

// ─── Test helpers ──────────────────────────────────────────────
//
// The renderer probes stay here: they call metal_darwin.m, and a _test.go
// file cannot import "C". The window test hooks are Go now and live in
// window_hooks_darwin_test.go.

// testCompileShaders compiles the built-in MSL library standalone at
// the pinned language version. 0 = compiled, -1 = compile failed,
// 1 = no Metal device. Lives here because cgo is not permitted in
// _test.go files. See TestBuiltinShadersCompile.
func testCompileShaders() int {
	return testCompileMSL(msl.Source)
}

// testCompileMSL compiles src through the same probe; return codes
// as testCompileShaders. See TestCustomWrapperCompiles.
func testCompileMSL(src string) int {
	cMSL := C.CString(src)
	defer C.free(unsafe.Pointer(cMSL))
	return int(C.metalCompileShadersProbe(cMSL))
}

// testEdgeCoverage draws a diagonal triangle edge through the main
// render pass and returns how many edge pixels are partly covered.
// -1 = no Metal device, -2 = setup failed. See
// TestTriangleEdgesAntialiased.
func testEdgeCoverage() int {
	cMSL := C.CString(msl.Source)
	defer C.free(unsafe.Pointer(cMSL))
	return int(C.metalEdgeCoverageProbe(cMSL))
}

// ─── Window callbacks (from the GUIWindow methods) ────────────

func goMetalWindowResized(wid uint32, width, height int) {
	ws := lookupWindow(wid)
	if ws == nil {
		return
	}
	ws.handleResize()

	// Dispatch resize event to the attached window.
	if ws.attachedWindow != nil {
		w := ws.attachedWindow
		w32, h32 := width, height
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

func goMetalWindowShouldClose(wid uint32) {
	ws := lookupWindow(wid)
	if ws == nil || ws.attachedWindow == nil {
		return
	}
	gui.DispatchCloseRequest(ws.attachedWindow)
	// windowShouldClose: returns NO to AppKit so the window
	// stays open until the Go event loop destroys it.
}

func goMetalWindowFocusChanged(wid uint32, focused bool) {
	ws := lookupWindow(wid)
	if ws == nil || ws.attachedWindow == nil {
		return
	}
	et := gui.EventUnfocused
	if focused {
		et = gui.EventFocused
	}
	ws.attachedWindow.EventFn(&gui.Event{Type: et})
	// windowDidBecomeKey:/windowDidResignKey: are notifications, not
	// NSEvents — the idle loop blocks in metalPollEvent(-1) on their
	// own, so the frame reflecting the focus change would wait for
	// the next real event. Post an empty event to bring the loop
	// round for the repaint (issue #405).
	wakeUp()
}

func goMetalWindowOccluded(wid uint32, occluded bool) {
	ws := lookupWindow(wid)
	if ws == nil || ws.attachedWindow == nil {
		return
	}
	// On show, DispatchWindowOccluded marks a full refresh and wakes the
	// idle loop through the window's wake function, so the first visible
	// frame does not wait for the next real event.
	gui.DispatchWindowOccluded(ws.attachedWindow, occluded)
}

func goMetalWindowState(wid uint32, zoomed, fullscreen bool) {
	ws := lookupWindow(wid)
	if ws == nil || ws.attachedWindow == nil {
		return
	}
	gui.DispatchWindowState(ws.attachedWindow, zoomed, fullscreen)
}

func goMetalFileDrop(wid uint32, path string) {
	ws := lookupWindow(wid)
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
	wakeUp()
}

// openApp is the App that receives OS open requests. RunApp sets it
// for its run; it is nil under single-window Run, which has no App to
// deliver to.
var openApp atomic.Pointer[gui.App]

// deliverOpen sorts what AppKit sent into an OpenEvent and hands it
// to app. Split from the delegate method so tests can drive it.
func deliverOpen(app *gui.App, items []string, isFile []bool) {
	var ev gui.OpenEvent
	for i, s := range items {
		if isFile[i] {
			ev.Files = append(ev.Files, s)
		} else {
			ev.URLs = append(ev.URLs, s)
		}
	}
	if app == nil {
		if gui.DebugEnabled() {
			log.Printf("metal: open request dropped (%d files, %d URLs): "+
				"open events need gui.App and backend.RunApp, not Run",
				len(ev.Files), len(ev.URLs))
		}
		return
	}
	app.DispatchOpen(ev)
}

// testNullBackends returns a windowState and a glyph backend over a
// zeroed C context (see metalTestNullCtx). Every C draw call returns
// at its encoder guard, so a test sees only the Go side of a draw
// path — what TestDrawPathsDoNotAllocate needs to count heap
// allocations without a GPU. The texture registered under glyph id 1
// lets the textured-quad paths get past their lookup. Call the
// returned func to free the context.
func testNullBackends() (*windowState, *metalGlyphBackend, func()) {
	ctx := C.metalTestNullCtx()
	ws := &windowState{ctx: ctx, dpiScale: 2}
	gb := newMetalGlyphBackend(ctx, 2)
	gb.textures[1] = metalTexInfo{cID: 1, w: 256, h: 256}
	return ws, gb, func() { C.metalCtxDestroy(ctx) }
}
