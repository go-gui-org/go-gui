//go:build ios

// Package ios provides an iOS backend for go-gui using Metal
// rendering and UIKit for windowing/events.
package ios

/*
#cgo CFLAGS: -fobjc-arc
#cgo LDFLAGS: -framework Metal -framework QuartzCore -framework Foundation -framework UIKit
#include <stdlib.h>
#include "metal_darwin.h"
#include "ios_app.h"
*/
import "C"

import (
	"fmt"
	"log"
	"os"
	"runtime"
	"unsafe"

	"github.com/go-gui-org/go-glyph"

	"github.com/go-gui-org/go-gui/gui"
	"github.com/go-gui-org/go-gui/gui/backend/internal/framestate"
	"github.com/go-gui-org/go-gui/gui/backend/internal/imgpath"
	"github.com/go-gui-org/go-gui/gui/backend/internal/msl"
	"github.com/go-gui-org/go-gui/gui/backend/internal/tempfont"
	"github.com/go-gui-org/go-gui/gui/backend/internal/texcache"
	"github.com/go-gui-org/go-gui/gui/svg"
)

// Pipeline IDs matching the C enum.
const (
	pipeSolid      = C.PIPE_SOLID
	pipeShadow     = C.PIPE_SHADOW
	pipeBlur       = C.PIPE_BLUR
	pipeGradient   = C.PIPE_GRADIENT
	pipeImageClip  = C.PIPE_IMAGE_CLIP
	pipeGlyphTex   = C.PIPE_GLYPH_TEX
	pipeGlyphColor = C.PIPE_GLYPH_COLOR
)

const maxCustomPipelines = 32

// Package-level singleton (iOS has exactly one window).
var (
	iosBackend *Backend
	iosWindow  *gui.Window
)

// metalBridge implements framestate.Bridge with the Metal C calls.
type metalBridge struct{}

func (metalBridge) SetPipeline(pipe int, mvp *[16]float32) {
	C.metalSetPipeline(C.int(pipe))
	C.metalSetMVP((*C.float)(unsafe.Pointer(mvp)))
}

func (metalBridge) Resize(physW, physH int32) {
	C.metalResize(C.int(physW), C.int(physH))
}

// Backend is the iOS Metal backend for go-gui.
// Embeds framestate.FrameState for all shared render-frame
// state; platform-specific resources stay on the Backend.
type Backend struct {
	framestate.FrameState

	textSys      *glyph.TextSystem
	textures     texcache.Cache[string, metalTexture]
	glyphBack    *metalGlyphBackend
	customCache  texcache.Cache[uint64, C.int]
	iconFontPath string

	maxImageBytes  int64
	maxImagePixels int64
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

func initBackend(layerPtr unsafe.Pointer,
	w, h int32, scale float32) {

	// Shader source is owned by Go so the build cache tracks edits to
	// it; C copies it during the call. See gui/backend/internal/msl.
	cMSL := C.CString(msl.Source)
	rc := C.metalInit(layerPtr, cMSL)
	C.free(unsafe.Pointer(cMSL))
	if rc != 0 {
		panic(fmt.Sprintf("ios: metalInit failed: %d", rc))
	}

	physW := int32(float32(w) * scale)
	physH := int32(float32(h) * scale)
	C.metalResize(C.int(physW), C.int(physH))

	cfg := iosWindow.Config
	b := &Backend{
		FrameState: *framestate.New(
			int(pipeSolid), int(pipeGlyphTex), metalBridge{}),
		textures: newMetalTexCacheLRU(128),
		customCache: texcache.New[uint64, C.int](
			maxCustomPipelines,
			func(idx C.int) { C.metalDeleteCustomPipeline(idx) },
		),
		maxImageBytes:  cfg.MaxImageBytes,
		maxImagePixels: cfg.MaxImagePixels,
	}
	b.AllowedImageRoots = imgpath.NormalizeRoots(
		cfg.AllowedImageRoots)
	b.HandleResize(int32(w), int32(h), scale)

	// Initialize glyph text system with Metal backend.
	b.glyphBack = newMetalGlyphBackend(scale)
	textSys, err := glyph.NewTextSystem(b.glyphBack)
	if err != nil {
		panic(fmt.Sprintf("ios: NewTextSystem: %v", err))
	}
	b.textSys = textSys

	// Load embedded icon font.
	if data := gui.IconFontData; len(data) > 0 {
		tmp, err := tempfont.Write("go_gui_feathericon", data)
		if err != nil {
			log.Printf("ios: write icon font: %v", err)
		} else if err = textSys.AddFontFile(tmp); err != nil {
			log.Printf("ios: load icon font: %v", err)
			_ = os.Remove(tmp)
		} else {
			b.iconFontPath = tmp
		}
	}
	gui.LoadAppFonts(textSys, "ios")

	// Set injected interfaces on gui Window.
	iosWindow.SetTextMeasurer(&textMeasurer{textSys: textSys})
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

	if iosWindow.Config.OnInit != nil {
		iosWindow.Config.OnInit(iosWindow)
	}
}

// renderFrame clears the screen, draws the current layout, and
// presents the Metal drawable.
func (b *Backend) renderFrame(w *gui.Window) {
	r, g, bl, a := b.FrameBg(w)

	rc := C.metalBeginFrame(
		C.float(r), C.float(g), C.float(bl), C.float(a),
	)
	if rc != 0 {
		return
	}

	b.InvalidatePipelineState()
	b.SetPipeline(b.SolidPipe)

	w.Lock()
	b.renderersDraw(w)
	w.Unlock()

	// Flush queued text.
	b.FlushText(b.textSys)

	C.metalEndFrame()
}

func (b *Backend) handleResize(w, h int32, scale float32) {
	b.HandleResize(w, h, scale)
}

// Destroy releases all backend resources.
func (b *Backend) Destroy() {
	b.textures.DestroyAll()
	b.customCache.DestroyAll()
	if b.glyphBack != nil {
		b.glyphBack.destroy()
	}
	if b.textSys != nil {
		b.textSys.Free()
	}
	if b.iconFontPath != "" {
		_ = os.Remove(b.iconFontPath)
		b.iconFontPath = ""
	}
	C.metalDestroy()
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
