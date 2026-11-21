//go:build darwin && arm64 && !ios

package metal

// The window, its content view, the event poll, IME, the app delegate
// and the frame pump go through purego (#1321). This replaces
// metal_window_darwin.m.
//
// The ObjC classes keep their names: GUIWindow, MetalContentView,
// GUIApplication and GoGuiAppDelegate. Their state is in Go. A
// GUIWindow is its own delegate, as before, and its methods find their
// window through macWindows.
//
// All of this is main-thread only, as the cgo code was. AppKit calls the
// class methods on the main thread, and the backend calls the window
// functions from the main loop. The exceptions say so where they are:
// wakeUp and windowLater can run on any goroutine.

import (
	"log"
	"math"
	"strings"
	"sync"

	"github.com/ebitengine/purego/objc"

	"github.com/go-gui-org/go-gui/gui/internal/darwinffi"
)

// nsPoint, nsSize and nsRange are NSPoint, NSSize and NSRange. NSRect is
// a11yRect, named nsRect here.
type (
	nsPoint struct{ X, Y float64 }
	nsSize  struct{ W, H float64 }
	nsRange struct{ Loc, Len uint }
	nsRect  = a11yRect
)

// nsNotFound is NSNotFound, NSIntegerMax, as an NSRange location.
const nsNotFound = uint(math.MaxInt64)

// NSWindowStyleMask bits.
const (
	nsWindowStyleMaskBorderless             = 0
	nsWindowStyleMaskTitled                 = 1 << 0
	nsWindowStyleMaskClosable               = 1 << 1
	nsWindowStyleMaskMiniaturizable         = 1 << 2
	nsWindowStyleMaskResizable              = 1 << 3
	nsWindowStyleMaskFullScreen             = 1 << 14
	nsWindowStyleMaskFullSizeContentView    = 1 << 15
	nsBackingStoreBuffered                  = 2
	nsWindowCollectionBehaviorMoveToActive  = 1 << 1
	nsWindowCollectionBehaviorFullPrimary   = 1 << 7
	nsWindowTitleHidden                     = 1
	nsWindowOcclusionStateVisible           = 1 << 1
	nsViewWidthSizable                      = 1 << 1
	nsViewHeightSizable                     = 1 << 4
	nsWindowBelow                           = -1
	nsVisualEffectBlendingBehindWindow      = 0
	nsVisualEffectStateActive               = 1
	nsTrackingMouseEnteredAndExited         = 0x01
	nsTrackingActiveAlways                  = 0x80
	nsTrackingInVisibleRect                 = 0x200
	nsDragOperationNone                     = 0
	nsDragOperationCopy                     = 1
	mtlPixelFormatBGRA8Unorm                = 80
	nsEventTypeApplicationDefined           = 15
	nsApplicationActivationPolicyRegular    = 0
	nsTerminateCancel                       = 0
	nsEventModifierFlagCommand              = 1 << 20
	nsVisualEffectMaterialSidebar           = 7
	nsVisualEffectMaterialMenu              = 5
	nsVisualEffectMaterialHUDWindow         = 13
	nsVisualEffectMaterialUnderWindowBackgr = 21
)

// gui.WindowDecoration values.
const (
	decorationDefault        = 0
	decorationHiddenTitlebar = 1
	decorationNone           = 2
)

// cgFloatMax is CGFLOAT_MAX, the "no maximum" of a content size.
const cgFloatMax = math.MaxFloat64

// macWindow is one window: the GUIWindow, its MetalContentView and the
// state the ObjC classes kept in their properties.
type macWindow struct {
	id     uint32
	ns     objc.ID // GUIWindow, +1, released in metalWindowDestroy
	view   objc.ID // MetalContentView, +1
	layer  objc.ID // CAMetalLayer, owned by view
	effect objc.ID // NSVisualEffectView, +1, or 0 when opaque

	// transparent is WindowCfg.Transparent. Turning vibrancy off
	// restores it, not plain opacity.
	transparent bool

	// closed is set by metalWindowDestroy. A state change still queued
	// by windowLater then finds the window gone.
	closed bool

	// Fullscreen bookkeeping (#1171). fsWanted is the state to end up in;
	// a request during the animation waits in it. fsTransitioning holds
	// off state reports until the animation ends. fsZoomedBefore is the
	// answer for IsMaximized while fullscreen. fsLifted says the max size
	// and style were changed for the transition and need restoring.
	fsWanted, fsTransitioning, fsZoomedBefore, fsLifted bool
	fsSavedMaxSize                                      nsSize
	fsSavedStyle                                        uint
	// fsNormalFrame is the frame Restore returns a zoomed window to.
	fsNormalFrame nsRect
	// fsHidden is the app's Hide, which isVisible cannot tell from
	// minimized.
	fsHidden bool

	// IME state of the content view. markedText is an
	// NSAttributedString, +1, or 0 when nothing is composed. The two
	// ranges are in UTF-16 units, as AppKit reads them.
	imeActive     bool
	imeCursorRect nsRect
	markedText    objc.ID
	markedRange   nsRange
	selRange      nsRange
}

var (
	// macWindows finds a window from its GUIWindow or its
	// MetalContentView. Main-thread only.
	macWindows = map[objc.ID]*macWindow{}

	// nextWindowID counts window IDs from 1. Main-thread only.
	nextWindowID uint32 = 1
)

// windowOf returns the window of a GUIWindow or MetalContentView, or nil.
func windowOf(obj objc.ID) *macWindow { return macWindows[obj] }

// sels holds the selectors of the event path. objc.RegisterName takes
// the runtime lock, so the hot path reads these instead.
var sels struct {
	alloc, init, new, retain, release, autorelease  objc.SEL
	window, contentView, bounds, frame              objc.SEL
	eventType, modifierFlags, clickCount, buttonNum objc.SEL
	locationInWindow, deltaX, deltaY, phase         objc.SEL
	hasPrecise, scrollingDeltaX, scrollingDeltaY    objc.SEL
	keyCode, isARepeat                              objc.SEL
	nextEvent, sendEvent, postEvent, distantFuture  objc.SEL
	dateWithTimeIntervalSinceNow                    objc.SEL
	otherEventWithType                              objc.SEL
	recordNormalFrame, pumpFrames                   objc.SEL
}

// objs holds the classes and constants the event path uses.
var objs struct {
	nsDate, nsEvent, nsString objc.ID
	// app is NSApp, set by metalAppInit.
	app objc.ID
	// defaultMode is NSDefaultRunLoopMode.
	defaultMode objc.ID
	// distantFuture is +[NSDate distantFuture], +1.
	distantFuture objc.ID
}

// loadObjc fills sels and objs and registers the classes. It runs
// once, from metalAppInit and from any test that needs a class.
var loadObjc = sync.OnceFunc(func() {
	appkit := darwinffi.Open(darwinffi.PathAppKit)
	darwinffi.Open(darwinffi.PathQuartzCore)
	darwinffi.Open(darwinffi.PathMetal)

	s := darwinffi.Sel
	sels.alloc, sels.init, sels.new = s("alloc"), s("init"), s("new")
	sels.retain, sels.release, sels.autorelease = s("retain"), s("release"), s("autorelease")
	sels.window, sels.contentView = s("window"), s("contentView")
	sels.bounds, sels.frame = s("bounds"), s("frame")
	sels.eventType, sels.modifierFlags = s("type"), s("modifierFlags")
	sels.clickCount, sels.buttonNum = s("clickCount"), s("buttonNumber")
	sels.locationInWindow = s("locationInWindow")
	sels.deltaX, sels.deltaY, sels.phase = s("deltaX"), s("deltaY"), s("phase")
	sels.hasPrecise = s("hasPreciseScrollingDeltas")
	sels.scrollingDeltaX, sels.scrollingDeltaY = s("scrollingDeltaX"), s("scrollingDeltaY")
	sels.keyCode, sels.isARepeat = s("keyCode"), s("isARepeat")
	sels.nextEvent = s("nextEventMatchingMask:untilDate:inMode:dequeue:")
	sels.sendEvent = s("sendEvent:")
	sels.postEvent = s("postEvent:atStart:")
	sels.distantFuture = s("distantFuture")
	sels.dateWithTimeIntervalSinceNow = s("dateWithTimeIntervalSinceNow:")
	sels.otherEventWithType = s("otherEventWithType:location:modifierFlags:" +
		"timestamp:windowNumber:context:subtype:data1:data2:")
	sels.recordNormalFrame = s("recordNormalFrame")
	sels.pumpFrames = s("pumpFrames:")

	objs.nsDate = darwinffi.Class("NSDate")
	objs.nsEvent = darwinffi.Class("NSEvent")
	objs.nsString = darwinffi.Class("NSString")
	objs.defaultMode = objc.ID(darwinffi.Data(appkit, "NSDefaultRunLoopMode"))
	objs.distantFuture = objs.nsDate.Send(sels.distantFuture).Send(sels.retain)

	registerWindowClasses()
})

// Classes, registered once by loadObjc. A class can be registered only
// once per process, so tests and a second window share them.
var (
	guiWindowClass   objc.Class
	contentViewClass objc.Class
	appClass         objc.Class
	delegateClass    objc.Class
	pumpTargetClass  objc.Class
)

func mustClass(name string, super string, protocols []string, methods []objc.MethodDef) objc.Class {
	var ps []*objc.Protocol
	for _, p := range protocols {
		proto := objc.GetProtocol(p)
		if proto == nil {
			panic("metal: no protocol " + p)
		}
		ps = append(ps, proto)
	}
	cls, err := objc.RegisterClass(name, objc.GetClass(super), ps, nil, methods)
	if err != nil {
		panic("metal: " + err.Error())
	}
	return cls
}

func registerWindowClasses() {
	guiWindowClass = mustClass("GUIWindow", "NSWindow", nil, guiWindowMethods())
	contentViewClass = mustClass("MetalContentView", "NSView",
		[]string{"NSTextInputClient"}, contentViewMethods())
	// GUIApplication has no methods of its own. It exists so that
	// [GUIApplication sharedApplication] makes NSApp of this class, as the
	// cgo code did: code that checks isKindOfClass: on NSApp sees it.
	appClass = mustClass("GUIApplication", "NSApplication", nil, nil)
	delegateClass = mustClass("GoGuiAppDelegate", "NSObject",
		[]string{"NSApplicationDelegate"}, appDelegateMethods())
	pumpTargetClass = mustClass("GoGuiFramePumpTarget", "NSObject", nil,
		framePumpMethods())
}

// nsStr returns an autoreleased NSString for s, with each byte that is
// not valid UTF-8 replaced by U+FFFD. stringWithUTF8String: returns nil
// for such text, and nil passed to setTitle: and similar setters throws.
// As with C.CString, s ends at its first NUL byte.
func nsStr(s string) objc.ID {
	return darwinffi.NSString(strings.ToValidUTF8(s, "\uFFFD"))
}

// goStr copies an NSString into Go. It returns "" for nil.
func goStr(ns objc.ID) string {
	if ns == 0 {
		return ""
	}
	return darwinffi.GoString(uintptr(ns))
}

// ─── Lifecycle ─────────────────────────────────────────────────

// metalWindowCreate makes a window with a CAMetalLayer-backed content
// view. width and height are logical pixels. decorations is a
// gui.WindowDecoration: 0 standard frame, 1 hidden title bar (window
// controls stay, content runs full height), 2 borderless. A hidden
// window is made but not ordered front: the app hid it before the
// backend started (issue #1270), and metalWindowShow shows it later. It
// returns nil when there is no Metal device.
func metalWindowCreate(title string, width, height int, fixedSize bool, decorations int, hidden bool) *macWindow {
	loadObjc()
	pool := darwinffi.PoolPush()
	defer darwinffi.PoolPop(pool)

	device := mtlCreateSystemDefaultDevice()
	if device == 0 {
		return nil
	}
	defer device.Send(sels.release)

	// AppKit can refuse a zero or negative content rect, or make an
	// invisible window.
	width = max(width, 1)
	height = max(height, 1)

	// A borderless window keeps Resizable, so AppKit still resizes it
	// from the edges: macOS needs no grip drawn by the client.
	var style uint
	if decorations == decorationNone {
		// Miniaturizable draws nothing here. Without it a borderless
		// window ignores Window.Minimize.
		style = nsWindowStyleMaskBorderless | nsWindowStyleMaskMiniaturizable
	} else {
		style = nsWindowStyleMaskTitled | nsWindowStyleMaskClosable |
			nsWindowStyleMaskMiniaturizable
		if decorations == decorationHiddenTitlebar {
			style |= nsWindowStyleMaskFullSizeContentView
		}
	}
	if !fixedSize {
		style |= nsWindowStyleMaskResizable
	}

	w := &macWindow{id: nextWindowID}
	nextWindowID++
	w.markedRange = nsRange{nsNotFound, 0}

	content := nsRect{0, 0, float64(width), float64(height)}
	w.ns = objc.ID(guiWindowClass).Send(sels.alloc).Send(
		darwinffi.Sel("initWithContentRect:styleMask:backing:defer:"),
		content, style, uint(nsBackingStoreBuffered), false)
	if w.ns == 0 {
		return nil
	}
	// metalWindowDestroy releases the window itself. A window that also
	// released itself on close would be released twice.
	w.ns.Send(darwinffi.Sel("setReleasedWhenClosed:"), false)
	macWindows[w.ns] = w
	w.ns.Send(darwinffi.Sel("setDelegate:"), w.ns)
	// The setter, not a getter override: acceptsMouseMovedEvents is a
	// stored ivar, and the setter makes AppKit post
	// NSEventTypeMouseMoved even with no button down.
	w.ns.Send(darwinffi.Sel("setAcceptsMouseMovedEvents:"), true)
	fileURL := objc.ID(darwinffi.Data(darwinffi.Open(darwinffi.PathAppKit),
		"NSPasteboardTypeFileURL"))
	w.ns.Send(darwinffi.Sel("registerForDraggedTypes:"),
		darwinffi.Class("NSArray").Send(darwinffi.Sel("arrayWithObject:"), fileURL))

	if title != "" {
		w.ns.Send(darwinffi.Sel("setTitle:"), nsStr(title))
	}
	if decorations == decorationHiddenTitlebar {
		// Content runs the full height, with the traffic lights over
		// whatever the app draws there.
		w.ns.Send(darwinffi.Sel("setTitlebarAppearsTransparent:"), true)
		w.ns.Send(darwinffi.Sel("setTitleVisibility:"), int(nsWindowTitleHidden))
	}
	w.ns.Send(darwinffi.Sel("center"))

	w.view = newContentView(w, content, device)
	macWindows[w.view] = w
	w.view.Send(darwinffi.Sel("setAutoresizingMask:"),
		uint(nsViewWidthSizable|nsViewHeightSizable))
	w.ns.Send(darwinffi.Sel("setContentView:"), w.view)
	// Key delivery must not depend on the input method being active:
	// IMEStart also calls makeFirstResponder:, and since issue #393 it
	// runs only for editable text widgets. The content view as the
	// first responder from the start keeps every other window receiving
	// keys.
	w.ns.Send(darwinffi.Sel("setInitialFirstResponder:"), w.view)

	w.ns.Send(darwinffi.Sel("setCollectionBehavior:"),
		uint(nsWindowCollectionBehaviorMoveToActive))
	// A window the app hid before the backend started stays off the
	// screen (issue #1270). The flag is set as metalWindowHide sets it,
	// so a state change asked for while hidden waits the same way.
	if hidden {
		w.fsHidden = true
	} else {
		w.ns.Send(darwinffi.Sel("makeKeyAndOrderFront:"), objc.ID(0))
	}

	// Tell AppKit to update window state: cursor tracking and frame
	// management need it for windows made before the run loop runs.
	objs.app.Send(darwinffi.Sel("setWindowsNeedUpdate:"), true)

	// Bring the app forward for a window opened later. For the first
	// windows, Go also calls metalAppFinishLaunch after all are on
	// screen.
	metalAppFinishLaunch()
	return w
}

// mtlCreateSystemDefaultDevice calls MTLCreateSystemDefaultDevice. The
// device is +1 (the Create rule), or 0 when there is none.
func mtlCreateSystemDefaultDevice() objc.ID {
	fn := darwinffi.Func(darwinffi.Open(darwinffi.PathMetal), "MTLCreateSystemDefaultDevice")
	return objc.ID(msgCall(fn))
}

func metalWindowDestroy(w *macWindow) {
	if w == nil || w.ns == 0 {
		return
	}
	pool := darwinffi.PoolPush()
	defer darwinffi.PoolPop(pool)
	// A state change still on the main queue must find the window gone,
	// and the settle timer must not outlive it.
	w.closed = true
	darwinffi.Class("NSObject").Send(
		darwinffi.Sel("cancelPreviousPerformRequestsWithTarget:"), w.ns)
	w.ns.Send(darwinffi.Sel("setDelegate:"), objc.ID(0))
	w.ns.Send(darwinffi.Sel("unregisterDraggedTypes"))
	w.ns.Send(darwinffi.Sel("close"))
	w.unmark()
	if w.effect != 0 {
		w.effect.Send(sels.release)
		w.effect = 0
	}
	delete(macWindows, w.ns)
	delete(macWindows, w.view)
	w.view.Send(sels.release)
	w.ns.Send(sels.release)
	w.ns, w.view, w.layer = 0, 0, 0
}

func metalWindowHide(w *macWindow) {
	if w == nil || w.ns == 0 {
		return
	}
	w.fsHidden = true
	w.ns.Send(darwinffi.Sel("orderOut:"), objc.ID(0))
}

func metalWindowShow(w *macWindow) {
	if w == nil || w.ns == 0 {
		return
	}
	w.fsHidden = false
	w.ns.Send(darwinffi.Sel("makeKeyAndOrderFront:"), objc.ID(0))
	metalForceActivate()
}

// ─── State and placement (#1171, #1220) ────────────────────────

// windowLater runs change on the main queue, which the run loop serves
// inside metalPollEvent. AppKit answers these calls with windowDidResize:
// at once, which reaches EventFn. gui makes them from event handlers, and
// a nested EventFn would overwrite the event in flight.
func windowLater(w *macWindow, change func(w *macWindow)) {
	if w == nil {
		return
	}
	mainQueueAsync(func() {
		if !w.closed {
			change(w)
		}
	})
}

// windowStateLater is windowLater for a change that can show the
// window. A change queued just before Hide runs after it, and must not
// bring the window back. Resize and move show nothing, so they use
// windowLater.
func windowStateLater(w *macWindow, change func(w *macWindow)) {
	windowLater(w, func(w *macWindow) {
		if !w.fsHidden {
			change(w)
		}
	})
}

func metalWindowMinimize(w *macWindow) {
	windowStateLater(w, func(w *macWindow) {
		w.ns.Send(darwinffi.Sel("miniaturize:"), objc.ID(0))
	})
}

func metalWindowMaximize(w *macWindow) {
	windowStateLater(w, func(w *macWindow) {
		if !w.isZoomed() && !w.isFullscreen() {
			w.ns.Send(darwinffi.Sel("zoom:"), objc.ID(0))
		}
	})
}

func metalWindowRestore(w *macWindow) {
	windowStateLater(w, func(w *macWindow) {
		switch {
		case w.isMiniaturized():
			w.ns.Send(darwinffi.Sel("deminiaturize:"), objc.ID(0))
		case w.isZoomed() && !w.isFullscreen():
			if rectEmpty(w.fsNormalFrame) {
				w.ns.Send(darwinffi.Sel("zoom:"), objc.ID(0))
			} else {
				w.ns.Send(darwinffi.Sel("setFrame:display:animate:"),
					w.fsNormalFrame, true, true)
			}
		}
	})
}

func metalWindowSetFullscreen(w *macWindow, on bool) {
	windowStateLater(w, func(w *macWindow) {
		// Only a real change brings a minimized window back. Asking a
		// minimized window to stay out of fullscreen must not show it.
		if on != w.isFullscreen() && w.isMiniaturized() {
			w.ns.Send(darwinffi.Sel("deminiaturize:"), objc.ID(0))
		}
		w.fsWanted = on
		w.moveToWantedFullscreen()
	})
}

func metalWindowResize(w *macWindow, width, height int) {
	windowLater(w, func(w *macWindow) {
		if w.isMiniaturized() || w.isFullscreen() || w.isZoomed() {
			return
		}
		content := objc.Send[nsRect](w.ns, darwinffi.Sel("contentRectForFrameRect:"),
			objc.Send[nsRect](w.ns, sels.frame))
		// AppKit's origin is the bottom-left corner, so keeping the top
		// of the window where it is moves the origin.
		content.Y += content.H - float64(height)
		content.W, content.H = float64(width), float64(height)
		frame := objc.Send[nsRect](w.ns, darwinffi.Sel("frameRectForContentRect:"), content)
		w.ns.Send(darwinffi.Sel("setFrame:display:"), frame, true)
	})
}

// desktopHeight is the height of the primary screen, the one AppKit's
// desktop coordinates are measured from. It turns AppKit's y-up points
// into the y-down desktop coordinates that Window.Position reports.
func desktopHeight() float64 {
	screens := darwinffi.Class("NSScreen").Send(darwinffi.Sel("screens"))
	first := screens.Send(darwinffi.Sel("firstObject"))
	return objc.Send[nsRect](first, sels.frame).H
}

func metalWindowMove(w *macWindow, x, y int) {
	windowLater(w, func(w *macWindow) {
		if w.isMiniaturized() || w.isFullscreen() || w.isZoomed() {
			return
		}
		frame := objc.Send[nsRect](w.ns, sels.frame)
		content := objc.Send[nsRect](w.ns, darwinffi.Sel("contentRectForFrameRect:"), frame)
		titlebar := (frame.Y + frame.H) - (content.Y + content.H)
		w.ns.Send(darwinffi.Sel("setFrameTopLeftPoint:"),
			nsPoint{float64(x), desktopHeight() - float64(y) + titlebar})
	})
}

// metalWindowGetPosition reports the top-left of the content area in
// desktop coordinates. ok is false for a minimized window, which has no
// position on the desktop.
func metalWindowGetPosition(w *macWindow) (x, y int, ok bool) {
	if w == nil || w.ns == 0 || w.isMiniaturized() {
		return 0, 0, false
	}
	pool := darwinffi.PoolPush()
	defer darwinffi.PoolPop(pool)
	content := objc.Send[nsRect](w.ns, darwinffi.Sel("contentRectForFrameRect:"),
		objc.Send[nsRect](w.ns, sels.frame))
	return int(content.X), int(desktopHeight() - (content.Y + content.H)), true
}

// ─── Properties ────────────────────────────────────────────────

func metalWindowSetTitle(w *macWindow, title string) {
	if w == nil || w.ns == 0 {
		return
	}
	pool := darwinffi.PoolPush()
	defer darwinffi.PoolPop(pool)
	w.ns.Send(darwinffi.Sel("setTitle:"), nsStr(title))
}

// metalWindowSetSizeLimits limits interactive resizing to the given
// content size, in points: AppKit's content size is in points, so no DPI
// scaling applies. A zero minimum means no floor, and a zero maximum no
// ceiling. setContentMaxSize: also caps the zoom button, not only the
// drag.
func metalWindowSetSizeLimits(w *macWindow, minW, minH, maxW, maxH int) {
	if w == nil || w.ns == 0 {
		return
	}
	// Each axis on its own: a zero on one axis leaves only that axis
	// free.
	limit := func(v int, none float64) float64 {
		if v > 0 {
			return float64(v)
		}
		return none
	}
	w.ns.Send(darwinffi.Sel("setContentMinSize:"), nsSize{limit(minW, 0), limit(minH, 0)})
	// AppKit has no "no maximum" value. CGFLOAT_MAX is the idiom, and it
	// is what a window has before anyone sets a ceiling.
	w.ns.Send(darwinffi.Sel("setContentMaxSize:"),
		nsSize{limit(maxW, cgFloatMax), limit(maxH, cgFloatMax)})
}

// metalWindowStartDrag moves the window with the pointer, from the last
// mouse-down. AppKit owns the pointer until the button comes up, so no
// move events reach Go during the drag.
func metalWindowStartDrag(w *macWindow) {
	if w == nil || w.ns == 0 || pollState.lastMouseDown == 0 {
		return
	}
	w.ns.Send(darwinffi.Sel("performWindowDragWithEvent:"), pollState.lastMouseDown)
}

// metalWindowGetSize reports the content bounds in points, not the frame:
// the space the Go side renders into and what goMetalWindowResized
// reports.
func metalWindowGetSize(w *macWindow) (width, height int) {
	if w == nil || w.view == 0 {
		return 0, 0
	}
	b := msgRect(w.view, sels.bounds)
	return int(b.W), int(b.H)
}

// metalWindowGetFramebufferSize sets the drawable size of the layer from
// the view bounds and the backing scale, and reports it in pixels. It
// must run after the window is shown, so the view has a frame and the
// window is on a screen. Setting the size here is on purpose: creation
// and resize then share one code path with the Go DPI scale.
//
// scaleOverride, when more than 0, sizes the drawable at that density.
// The layer keeps its bounds, so Core Animation resamples the drawable
// to the screen: the window keeps its size and renders like a display of
// that scale.
func metalWindowGetFramebufferSize(w *macWindow, scaleOverride float32) (width, height int) {
	if w == nil || w.view == 0 {
		return 0, 0
	}
	b := msgRect(w.view, sels.bounds)
	scale := objc.Send[float64](w.ns, darwinffi.Sel("backingScaleFactor"))
	if scaleOverride > 0 {
		scale = float64(scaleOverride)
	}
	if scale <= 0 {
		scale = 1
	}
	size := nsSize{b.W * scale, b.H * scale}
	w.layer.Send(darwinffi.Sel("setDrawableSize:"), size)
	return int(size.W), int(size.H)
}

// ─── Vibrancy and opacity ──────────────────────────────────────

// vibrancyMaterial maps gui.VibrancyMaterial 1..4 to an
// NSVisualEffectMaterial. Keep it in the order of the Go enum.
func vibrancyMaterial(material int) int {
	switch material {
	case 1:
		return nsVisualEffectMaterialSidebar
	case 2:
		return nsVisualEffectMaterialMenu
	case 3:
		return nsVisualEffectMaterialHUDWindow
	default:
		return nsVisualEffectMaterialUnderWindowBackgr
	}
}

// setOpaque switches the window and its Metal drawable between opaque
// and see-through. Vibrancy and plain transparency share it, so the two
// cannot drift; only vibrancy adds a backdrop view.
func (w *macWindow) setOpaque(opaque bool) {
	w.ns.Send(darwinffi.Sel("setOpaque:"), opaque)
	color := "clearColor"
	if opaque {
		color = "windowBackgroundColor"
	}
	w.ns.Send(darwinffi.Sel("setBackgroundColor:"),
		darwinffi.Class("NSColor").Send(darwinffi.Sel(color)))
	w.layer.Send(darwinffi.Sel("setOpaque:"), opaque)
}

// metalWindowSetTransparent is WindowCfg.Transparent: the desktop shows
// through wherever the content is not opaque, with no blur.
// metalWindowSetVibrancy is the blurred form.
func metalWindowSetTransparent(w *macWindow, on bool) {
	if w == nil || w.ns == 0 {
		return
	}
	pool := darwinffi.PoolPush()
	defer darwinffi.PoolPop(pool)
	w.transparent = on
	w.setOpaque(!on)
}

// metalWindowSetAlpha fades the whole window. It is the compositor knob
// and does not depend on transparent: alphaValue scales everything AppKit
// composites for the window, the content too.
func metalWindowSetAlpha(w *macWindow, alpha float32) {
	if w == nil || w.ns == 0 {
		return
	}
	w.ns.Send(darwinffi.Sel("setAlphaValue:"), float64(alpha))
}

// metalWindowSetAppearanceDark pins the appearance of this window, not of
// NSApp: SystemAppearance and its watcher read NSApp.effectiveAppearance,
// which an app-wide pin would replace with the app's own choice. Light is
// Aqua, not nil, so a light pin holds on a dark desktop. A window never
// pinned keeps nil and follows the system.
func metalWindowSetAppearanceDark(w *macWindow, dark bool) {
	if w == nil || w.ns == 0 {
		return
	}
	pool := darwinffi.PoolPush()
	defer darwinffi.PoolPop(pool)
	key := "NSAppearanceNameAqua"
	if dark {
		key = "NSAppearanceNameDarkAqua"
	}
	name := objc.ID(darwinffi.Data(darwinffi.Open(darwinffi.PathAppKit), key))
	cur := w.ns.Send(darwinffi.Sel("appearance")).Send(darwinffi.Sel("name"))
	if cur != 0 && objc.Send[bool](cur, darwinffi.Sel("isEqualToString:"), name) {
		return
	}
	w.ns.Send(darwinffi.Sel("setAppearance:"),
		darwinffi.Class("NSAppearance").Send(darwinffi.Sel("appearanceNamed:"), name))
}

func metalWindowSetVibrancy(w *macWindow, material int) {
	if w == nil || w.ns == 0 {
		return
	}
	pool := darwinffi.PoolPush()
	defer darwinffi.PoolPop(pool)

	if material == 0 {
		// Off: remove the backdrop and restore the opacity the window
		// had. A Transparent window stays see-through.
		if w.effect != 0 {
			w.effect.Send(darwinffi.Sel("removeFromSuperview"))
			w.effect.Send(sels.release)
			w.effect = 0
		}
		w.setOpaque(!w.transparent)
		return
	}
	// Fail closed on a value out of range: leave the window as it is,
	// rather than make it translucent under a material nobody chose.
	if material < 1 || material > 4 {
		log.Printf("metal: ignoring unknown vibrancy material %d", material)
		return
	}
	// Make the effect view on first use and put it behind the content
	// view, as a sibling in the window frame. The content view stays the
	// first responder, so keys, resize and cursors do not change.
	if w.effect == 0 {
		frame := objc.Send[nsRect](w.view, sels.frame)
		fx := darwinffi.Class("NSVisualEffectView").Send(sels.alloc).
			Send(darwinffi.Sel("initWithFrame:"), frame)
		fx.Send(darwinffi.Sel("setAutoresizingMask:"),
			uint(nsViewWidthSizable|nsViewHeightSizable))
		fx.Send(darwinffi.Sel("setBlendingMode:"), int(nsVisualEffectBlendingBehindWindow))
		fx.Send(darwinffi.Sel("setState:"), int(nsVisualEffectStateActive))
		w.view.Send(darwinffi.Sel("superview")).Send(
			darwinffi.Sel("addSubview:positioned:relativeTo:"),
			fx, int(nsWindowBelow), w.view)
		w.effect = fx
	}
	w.effect.Send(darwinffi.Sel("setMaterial:"), vibrancyMaterial(material))
	// Non-opaque window and drawable, so the backdrop shows through
	// content cleared with a translucent color (see renderFrame).
	w.setOpaque(false)
}

// metalWindowNS returns the NSWindow, or 0.
func metalWindowNS(w *macWindow) objc.ID {
	if w == nil {
		return 0
	}
	return w.ns
}

// metalWindowGetID returns the window ID, or 0 for nil.
func metalWindowGetID(w *macWindow) uint32 {
	if w == nil {
		return 0
	}
	return w.id
}
