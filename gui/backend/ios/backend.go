//go:build ios

// Package ios provides an iOS backend for go-gui using Metal
// rendering and UIKit for windowing/events.
//
// The UIKit shell (ios_app.m) stays cgo. The Metal renderer is the one
// the macOS backend uses, gui/backend/internal/mtldraw, which calls
// Metal through purego (#1342).
package ios

/*
#cgo CFLAGS: -fobjc-arc
#cgo LDFLAGS: -framework Metal -framework QuartzCore -framework Foundation -framework UIKit -framework CoreGraphics
#include <stdlib.h>
#include "ios_app.h"
*/
import "C"

import (
	"fmt"
	"runtime"
	"unsafe"

	"github.com/ebitengine/purego/objc"

	"github.com/go-gui-org/go-gui/gui"
	"github.com/go-gui-org/go-gui/gui/backend/internal/mtldraw"
	"github.com/go-gui-org/go-gui/gui/svg"
)

// Package-level singleton (iOS has exactly one window).
var (
	iosBackend *Backend
	iosWindow  *gui.Window
)

// Backend is the iOS Metal backend for go-gui. The shared renderer
// holds all GPU and text state; the backend only feeds it sizes and
// frames from UIKit.
type Backend struct {
	r     *mtldraw.Renderer
	layer objc.ID // retained in initBackend, released in Destroy
}

// --- Pattern A: Go-driven (backend.Run) ---

// Run creates the UIKit app from Go. Calls UIApplicationMain
// which never returns.
func Run(w *gui.Window) {
	runtime.LockOSThread()
	iosWindow = w
	C.iosStartApp()
}

// --- Pattern B: Swift-driven (c-archive) ---

// SetWindow sets the gui.Window for the c-archive pattern.
// Must be called from an init() function before GoGuiStart.
func SetWindow(w *gui.Window) { iosWindow = w }

// (Pattern B initialization is via the Start function below.)

// --- Shared initialization ---

// physSize converts a size in points to drawable pixels.
func physSize(w, h int32, scale float32) (int, int) {
	return int(float32(w) * scale), int(float32(h) * scale)
}

func initBackend(layerPtr unsafe.Pointer,
	w, h int32, scale float32) {

	// layerPtr is a CAMetalLayer: the view's backing layer (Pattern A)
	// or a layer the host passes to Start (Pattern B). The renderer
	// keeps it unretained and asks it for a drawable every frame, so
	// the backend retains it for its lifetime. The cgo renderer held
	// it in a strong ARC reference; this keeps that guarantee for a
	// Pattern B host that drops its own reference.
	layer := objc.ID(uintptr(layerPtr))
	layer.Send(objc.RegisterName("retain"))
	physW, physH := physSize(w, h, scale)
	r, err := mtldraw.New(layer, physW, physH,
		scale, iosWindow.Config, "ios")
	if err != nil {
		layer.Send(objc.RegisterName("release"))
		panic(fmt.Sprintf("ios: %v", err))
	}
	b := &Backend{r: r, layer: layer}
	gui.SetWindowRendersShaders(iosWindow, true)

	// Set injected interfaces on gui Window.
	iosWindow.SetTextMeasurer(r.TextMeasurer())
	iosWindow.SetSvgParser(svg.New())
	iosWindow.SetClipboardFn(func(_ string) {})
	iosWindow.SetClipboardGetFn(func() string { return "" })
	iosWindow.SetNativePlatform(&nativePlatform{})

	iosBackend = b

	// Fire initial resize so w.WindowSize() returns the
	// correct dimensions when the view function runs.
	evt := gui.Event{
		Type:         gui.EventResized,
		WindowWidth:  int(w),
		WindowHeight: int(h),
	}
	iosWindow.EventFn(&evt)

	// CADisplayLink paces every frame to the display, and iOS has no
	// CAMetalLayer.displaySyncEnabled, so VSyncOff cannot be honored.
	if iosWindow.Config.VSyncOff {
		iosWindow.DebugWindowVSync("VSyncOff requested but iOS paces frames with CADisplayLink")
	}

	if iosWindow.Config.OnInit != nil {
		iosWindow.Config.OnInit(iosWindow)
	}
}

// renderFrame clears the screen, draws the current layout, and
// presents the Metal drawable.
func (b *Backend) renderFrame(w *gui.Window) {
	b.r.RenderFrame(w)
}

// handleResize adopts a new view size in points and a new screen scale.
// The scale goes first: Resize recomputes the projection, and text
// must shape at the new density before the next frame.
func (b *Backend) handleResize(w, h int32, scale float32) {
	b.r.SetDPIScale(scale)
	// Size the drawable from the scale the renderer kept: SetDPIScale
	// rejects an implausible value, and the drawable must match the
	// projection and text density.
	b.r.Resize(physSize(w, h, b.r.DPIScale()))
}

// Destroy releases all backend resources.
// The layer is released last, after the renderer stops using it.
func (b *Backend) Destroy() {
	b.r.Destroy()
	if b.layer != 0 {
		b.layer.Send(objc.RegisterName("release"))
		b.layer = 0
	}
}

// --- Exported callbacks for ios_app.m (Pattern A) ---

//export goIOSInit
func goIOSInit(layerPtr unsafe.Pointer,
	w, h C.int, scale C.float) {
	initBackend(layerPtr, int32(w), int32(h), float32(scale))
}

//export goIOSRender
func goIOSRender() {
	if iosBackend == nil || iosWindow == nil {
		return
	}
	iosWindow.FrameFn()
	iosBackend.renderFrame(iosWindow)
}

//export goIOSResize
func goIOSResize(w, h C.int, scale C.float) {
	if iosBackend == nil {
		return
	}
	iosBackend.handleResize(int32(w), int32(h), float32(scale))
	if iosWindow != nil {
		evt := gui.Event{
			Type:         gui.EventResized,
			WindowWidth:  int(w),
			WindowHeight: int(h),
		}
		iosWindow.EventFn(&evt)
	}
}

// goIOSSafeArea receives the view's safeAreaInsets, in points, which
// are logical pixels. The gui side clamps them and ignores a repeat.
//
//export goIOSSafeArea
func goIOSSafeArea(top, right, bottom, left C.float) {
	gui.DispatchSafeArea(iosWindow, float32(top), float32(right),
		float32(bottom), float32(left))
}

//export goIOSTouchEvent
func goIOSTouchEvent(phase C.int, identifier C.uintptr_t,
	x, y C.float) {
	TouchInput(int(phase), uint64(identifier),
		float32(x), float32(y))
}

// --- Public API for Swift host (Pattern B) ---
// These are regular Go functions that c-archive apps can call
// or re-export under their own names.

// Start initializes the backend with a pre-existing Metal
// layer. SetWindow should be called before this.
func Start(layerPtr unsafe.Pointer, w, h int, scale float32) {
	if iosWindow == nil {
		iosWindow = gui.NewWindow(gui.WindowCfg{})
	}
	initBackend(layerPtr, int32(w), int32(h), scale)
}

// Render runs one frame: layout + draw + present.
func Render() {
	if iosBackend == nil || iosWindow == nil {
		return
	}
	iosWindow.FrameFn()
	iosBackend.renderFrame(iosWindow)
}

// TouchBegan dispatches a single-touch began event with id 0.
//
// Deprecated: use TouchInput for multi-touch support.
func TouchBegan(x, y float32) {
	touchEvent(gui.EventTouchesBegan, 0, x, y)
}

// TouchMoved dispatches a single-touch moved event with id 0.
//
// Deprecated: use TouchInput for multi-touch support.
func TouchMoved(x, y float32) {
	touchEvent(gui.EventTouchesMoved, 0, x, y)
}

// TouchEnded dispatches a single-touch ended event with id 0.
//
// Deprecated: use TouchInput for multi-touch support.
func TouchEnded(x, y float32) {
	touchEvent(gui.EventTouchesEnded, 0, x, y)
}

// TouchInput dispatches a touch event with a unique finger
// identifier for multi-touch support. Phase constants:
// 0=began, 1=moved, 2=ended, 3=cancelled.
func TouchInput(phase int, identifier uint64, x, y float32) {
	var typ gui.EventType
	switch phase {
	case 0:
		typ = gui.EventTouchesBegan
	case 1:
		typ = gui.EventTouchesMoved
	case 2:
		typ = gui.EventTouchesEnded
	case 3:
		typ = gui.EventTouchesCancelled
	default:
		return
	}
	touchEvent(typ, identifier, x, y)
}

// Resize updates the viewport after a layout change.
func Resize(w, h int, scale float32) {
	if iosBackend == nil {
		return
	}
	iosBackend.handleResize(int32(w), int32(h), scale)
	if iosWindow != nil {
		evt := gui.Event{
			Type:         gui.EventResized,
			WindowWidth:  w,
			WindowHeight: h,
		}
		iosWindow.EventFn(&evt)
	}
}

// CleanUp releases all backend resources.
func CleanUp() {
	if iosBackend != nil {
		iosBackend.Destroy()
		iosBackend = nil
	}
	if iosWindow != nil {
		iosWindow.WindowCleanup()
		iosWindow = nil
	}
}
