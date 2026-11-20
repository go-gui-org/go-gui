//go:build darwin && arm64 && !ios

package metal

// The GUIWindow class: the window is its own delegate, and these are its
// methods, plus the fullscreen and zoom bookkeeping they share.
// Main-thread only. See window_darwin.go.

import (
	"github.com/ebitengine/purego/objc"

	"github.com/go-gui-org/go-gui/gui/internal/darwinffi"
)

// ─── GUIWindow ─────────────────────────────────────────────────

func guiWindowMethods() []objc.MethodDef {
	note := func(fn func(w *macWindow)) func(objc.ID, objc.SEL, objc.ID) {
		return func(self objc.ID, _ objc.SEL, _ objc.ID) {
			if w := windowOf(self); w != nil {
				fn(w)
			}
		}
	}
	return []objc.MethodDef{
		{Cmd: darwinffi.Sel("canBecomeKeyWindow"), Fn: yes},
		{Cmd: darwinffi.Sel("canBecomeMainWindow"), Fn: yes},

		// A resize drag begun on the window border. The press that
		// started it belongs to AppKit, not to the app: see the
		// liveResizeStarted consumer in metalPollEvent.
		{Cmd: darwinffi.Sel("windowWillStartLiveResize:"), Fn: note(func(w *macWindow) {
			pollState.liveResizeStarted = true
			w.setPresentsWithTransaction(true)
		})},
		// The other half of the presentation-mode switch. Without it the
		// window would stay in transaction mode after its first resize
		// and keep the per-frame main-thread block for the session.
		{Cmd: darwinffi.Sel("windowDidEndLiveResize:"), Fn: note(func(w *macWindow) {
			w.setPresentsWithTransaction(false)
		})},
		{Cmd: darwinffi.Sel("windowDidResize:"), Fn: note(func(w *macWindow) {
			// Content-bounds size, not frame size. The frame includes
			// the title bar and borders (~28pt more height), which would
			// make the Go layout give space to widgets that cannot draw.
			b := msgRect(w.view, sels.bounds)
			goMetalWindowResized(w.id, int(b.W), int(b.H))
			if !w.fsTransitioning {
				w.reportWindowState()
			}
			w.recordNormalFrameWhenSettled()
		})},
		{Cmd: darwinffi.Sel("windowDidMove:"), Fn: note(func(w *macWindow) {
			w.recordNormalFrameWhenSettled()
		})},
		{Cmd: sels.recordNormalFrame, Fn: func(self objc.ID, _ objc.SEL) {
			if w := windowOf(self); w != nil {
				w.recordNormalFrame()
			}
		}},
		// Called before any zoom:, so an app that zooms right after a
		// resize does not depend on the settle delay. The frame
		// argument is not read.
		{Cmd: darwinffi.Sel("windowShouldZoom:toFrame:"), Fn: func(self objc.ID, _ objc.SEL, _ objc.ID, _ nsRect) bool {
			if w := windowOf(self); w != nil {
				w.recordNormalFrame()
			}
			return true
		}},
		{Cmd: darwinffi.Sel("windowWillEnterFullScreen:"), Fn: note(func(w *macWindow) {
			w.fsTransitioning = true
			w.fsWanted = true
			// The green button comes here without lifting anything.
			if !w.fsLifted {
				w.fsZoomedBefore = w.isZoomed()
			}
		})},
		{Cmd: darwinffi.Sel("windowDidEnterFullScreen:"), Fn: note(func(w *macWindow) {
			w.fsTransitioning = false
			w.reportWindowState()
			w.moveToWantedFullscreen()
		})},
		{Cmd: darwinffi.Sel("windowDidFailToEnterFullScreen:"), Fn: note(func(w *macWindow) {
			w.fsTransitioning = false
			w.fsWanted = false
			w.restoreFullscreenLimits()
			w.reportWindowState()
		})},
		{Cmd: darwinffi.Sel("windowWillExitFullScreen:"), Fn: note(func(w *macWindow) {
			w.fsTransitioning = true
			w.fsWanted = false
		})},
		{Cmd: darwinffi.Sel("windowDidExitFullScreen:"), Fn: note(func(w *macWindow) {
			w.fsTransitioning = false
			w.restoreFullscreenLimits()
			w.reportWindowState()
			w.moveToWantedFullscreen()
		})},
		{Cmd: darwinffi.Sel("windowDidFailToExitFullScreen:"), Fn: note(func(w *macWindow) {
			w.fsTransitioning = false
			w.fsWanted = true
			w.reportWindowState()
		})},
		// A window dragged between a Retina and a non-Retina display
		// changes backing scale without changing its content size, so
		// windowDidResize: never fires and the text would keep its old
		// density. Route it through the same path: the resize recomputes
		// the drawable from the new backingScaleFactor. A color-space
		// change also comes here; the scale is then the same and the Go
		// side does nothing.
		{Cmd: darwinffi.Sel("windowDidChangeBackingProperties:"), Fn: note(func(w *macWindow) {
			b := msgRect(w.view, sels.bounds)
			goMetalWindowResized(w.id, int(b.W), int(b.H))
		})},
		// Go decides when to destroy the window, so AppKit gets NO.
		{Cmd: darwinffi.Sel("windowShouldClose:"), Fn: func(self objc.ID, _ objc.SEL, _ objc.ID) bool {
			if w := windowOf(self); w != nil {
				goMetalWindowShouldClose(w.id)
			}
			return false
		}},
		{Cmd: darwinffi.Sel("windowDidBecomeKey:"), Fn: note(func(w *macWindow) {
			// The first frame moves nothing, and Fill may be the first
			// thing that happens to a new window.
			if rectEmpty(w.fsNormalFrame) {
				w.recordNormalFrame()
			}
			goMetalWindowFocusChanged(w.id, true)
		})},
		{Cmd: darwinffi.Sel("windowDidResignKey:"), Fn: note(func(w *macWindow) {
			goMetalWindowFocusChanged(w.id, false)
		})},
		// Minimized, fully covered, on another Space, or the screen is
		// locked: AppKit clears NSWindowOcclusionStateVisible. Go stops
		// asking for animation frames while it is clear (issue #943).
		{Cmd: darwinffi.Sel("windowDidChangeOcclusionState:"), Fn: note(func(w *macWindow) {
			state := objc.Send[uint](w.ns, darwinffi.Sel("occlusionState"))
			goMetalWindowOccluded(w.id, state&nsWindowOcclusionStateVisible == 0)
		})},

		// NSDraggingDestination: a file drop.
		{Cmd: darwinffi.Sel("draggingEntered:"), Fn: func(_ objc.ID, _ objc.SEL, info objc.ID) uint {
			pb := info.Send(darwinffi.Sel("draggingPasteboard"))
			if objc.Send[bool](pb, darwinffi.Sel("canReadObjectForClasses:options:"),
				urlClasses(), fileURLsOnly()) {
				return nsDragOperationCopy
			}
			return nsDragOperationNone
		}},
		{Cmd: darwinffi.Sel("prepareForDragOperation:"), Fn: func(objc.ID, objc.SEL, objc.ID) bool {
			return true
		}},
		{Cmd: darwinffi.Sel("performDragOperation:"), Fn: func(self objc.ID, _ objc.SEL, info objc.ID) bool {
			w := windowOf(self)
			if w == nil {
				return false
			}
			pb := info.Send(darwinffi.Sel("draggingPasteboard"))
			urls := pb.Send(darwinffi.Sel("readObjectsForClasses:options:"),
				urlClasses(), fileURLsOnly())
			n := objc.Send[uint](urls, darwinffi.Sel("count"))
			if n == 0 {
				return false
			}
			for i := range n {
				u := urls.Send(darwinffi.Sel("objectAtIndex:"), i)
				if !objc.Send[bool](u, darwinffi.Sel("isFileURL")) {
					continue
				}
				if path := goStr(u.Send(darwinffi.Sel("path"))); path != "" {
					goMetalFileDrop(w.id, path)
				}
			}
			return true
		}},
	}
}

// yes is a method that returns YES.
func yes(objc.ID, objc.SEL) bool { return true }

// urlClasses returns @[[NSURL class]], autoreleased.
func urlClasses() objc.ID {
	return darwinffi.Class("NSArray").Send(darwinffi.Sel("arrayWithObject:"),
		darwinffi.Class("NSURL"))
}

// fileURLsOnly returns @{NSPasteboardURLReadingFileURLsOnlyKey: @YES},
// autoreleased.
func fileURLsOnly() objc.ID {
	key := objc.ID(darwinffi.Data(darwinffi.Open(darwinffi.PathAppKit),
		"NSPasteboardURLReadingFileURLsOnlyKey"))
	yesNum := darwinffi.Class("NSNumber").Send(darwinffi.Sel("numberWithBool:"), true)
	return darwinffi.Class("NSDictionary").Send(
		darwinffi.Sel("dictionaryWithObject:forKey:"), yesNum, key)
}

// rectEmpty is NSIsEmptyRect.
func rectEmpty(r nsRect) bool { return r.W <= 0 || r.H <= 0 }

func (w *macWindow) styleMask() uint {
	return objc.Send[uint](w.ns, darwinffi.Sel("styleMask"))
}

func (w *macWindow) isMiniaturized() bool {
	return objc.Send[bool](w.ns, darwinffi.Sel("isMiniaturized"))
}

func (w *macWindow) isFullscreen() bool {
	return w.styleMask()&nsWindowStyleMaskFullScreen != 0
}

// isZoomed is a zoom the app can undo. isZoomed only compares the frame
// with the standard frame, which the content limits cap, so a FixedSize
// window always reads as zoomed. Its answer also changes with the limits.
func (w *macWindow) isZoomed() bool {
	return w.styleMask()&nsWindowStyleMaskResizable != 0 &&
		objc.Send[bool](w.ns, darwinffi.Sel("isZoomed"))
}

// setPresentsWithTransaction switches the content layer's presentation
// mode. Safe between frames only, which is where both callers are: the
// live-resize notifications arrive on the main thread, and metalEndFrame
// reads the property each frame.
func (w *macWindow) setPresentsWithTransaction(on bool) {
	w.layer.Send(darwinffi.Sel("setPresentsWithTransaction:"), on)
}

// Restore cannot use zoom:. It only undoes a zoom AppKit saw, and the
// system's Fill tiles the window without one, so zoom: finds no frame to
// return to and makes the window a little smaller. fsNormalFrame is that
// frame, kept here for every way of being zoomed (#1220).
func (w *macWindow) recordNormalFrame() {
	if w.fsTransitioning || w.isMiniaturized() || w.isFullscreen() || w.isZoomed() {
		return
	}
	w.fsNormalFrame = objc.Send[nsRect](w.ns, sels.frame)
}

// recordNormalFrameWhenSettled records the frame once it stops changing.
// Fill and zoom animate, and a frame in flight is not one to return to.
func (w *macWindow) recordNormalFrameWhenSettled() {
	nsObject := darwinffi.Class("NSObject")
	nsObject.Send(darwinffi.Sel("cancelPreviousPerformRequestsWithTarget:selector:object:"),
		w.ns, sels.recordNormalFrame, objc.ID(0))
	w.ns.Send(darwinffi.Sel("performSelector:withObject:afterDelay:"),
		sels.recordNormalFrame, objc.ID(0), 0.4)
}

func (w *macWindow) reportWindowState() {
	full := w.isFullscreen()
	zoomed := w.isZoomed()
	if full {
		zoomed = w.fsZoomedBefore
	}
	goMetalWindowState(w.id, zoomed, full)
}

// liftFullscreenLimits raises the content ceiling and sets Resizable for
// the time the window is fullscreen. A window with either cannot fill
// the screen.
func (w *macWindow) liftFullscreenLimits() {
	if w.fsLifted {
		return
	}
	// Before the ceiling goes, which would change the standard frame.
	w.fsZoomedBefore = w.isZoomed()
	w.fsLifted = true
	w.fsSavedMaxSize = objc.Send[nsSize](w.ns, darwinffi.Sel("contentMaxSize"))
	w.fsSavedStyle = w.styleMask()
	w.ns.Send(darwinffi.Sel("setContentMaxSize:"), nsSize{cgFloatMax, cgFloatMax})
	w.ns.Send(darwinffi.Sel("setStyleMask:"), w.styleMask()|nsWindowStyleMaskResizable)
	behavior := objc.Send[uint](w.ns, darwinffi.Sel("collectionBehavior"))
	w.ns.Send(darwinffi.Sel("setCollectionBehavior:"),
		behavior|nsWindowCollectionBehaviorFullPrimary)
}

func (w *macWindow) restoreFullscreenLimits() {
	if !w.fsLifted {
		return
	}
	w.fsLifted = false
	w.ns.Send(darwinffi.Sel("setContentMaxSize:"), w.fsSavedMaxSize)
	style := w.styleMask()&^nsWindowStyleMaskResizable |
		w.fsSavedStyle&nsWindowStyleMaskResizable
	w.ns.Send(darwinffi.Sel("setStyleMask:"), style)
}

func (w *macWindow) moveToWantedFullscreen() {
	if w.fsTransitioning || w.fsWanted == w.isFullscreen() {
		return
	}
	if w.fsWanted {
		w.liftFullscreenLimits()
	}
	w.ns.Send(darwinffi.Sel("toggleFullScreen:"), objc.ID(0))
}
