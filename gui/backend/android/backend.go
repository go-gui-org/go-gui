//go:build android

// Package android provides an Android backend for go-gui using
// OpenGL ES 3.0 rendering with Kotlin/GLSurfaceView for
// windowing and events.
package android

/*
#cgo LDFLAGS: -lGLESv3 -lEGL -landroid -llog
#include "gles_android.h"
*/
import "C"

import (
	"fmt"
	"log"
	"os"
	"sync"
	"time"

	"github.com/go-gui-org/go-glyph"

	"github.com/go-gui-org/go-gui/gui"
	"github.com/go-gui-org/go-gui/gui/backend/internal/gpu"
	"github.com/go-gui-org/go-gui/gui/backend/internal/imgpath"
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

// Package-level singleton (Android has exactly one window).
var (
	androidBackend *Backend
	androidWindow  *gui.Window
)

// Backend is the Android GLES3 backend for go-gui.
type Backend struct {
	textSys  *glyph.TextSystem
	dpiScale float32
	physW    int32
	physH    int32
	mvp      [16]float32

	mvpStack [][16]float32

	// Reusable buffers.
	svgVerts           []gpu.Vertex
	textPathPlacements []glyph.GlyphPlacement
	normBuf            []gui.GradientStop
	sampledBuf         []gui.GradientStop

	textures          texcache.Cache[string, glesTexture]
	glyphBack         *glesGlyphBackend
	filterBlur        float32
	filterLayer       int
	filterColorMatrix *[16]float32
	customCache       texcache.Cache[uint64, C.int]
	iconFontPath      string

	allowedImageRoots []string
	imagePathCache    texcache.Cache[string, string]
	maxImageBytes     int64
	maxImagePixels    int64

	// Pipeline state tracking to skip redundant CGo calls.
	lastPipe   int
	mvpDirty   bool
	textQueued bool
}

// --- Pattern B only (no Pattern A / Run) ---

// SetWindow sets the gui.Window for the Android backend.
// Must be called before Start.
func SetWindow(w *gui.Window) { androidWindow = w }

// Start initializes the backend. If already initialized,
// handles resize (idempotent for onSurfaceChanged).
func Start(w, h int, scale float32) {
	if androidWindow == nil {
		androidWindow = gui.NewWindow(gui.WindowCfg{})
	}
	if androidBackend == nil {
		initBackend(int32(w), int32(h), scale)
	} else {
		androidBackend.handleResize(int32(w), int32(h), scale)
	}
}

// Render runs one frame: layout + draw + present.
func Render() {
	if androidBackend == nil || androidWindow == nil {
		return
	}
	androidWindow.FrameFn()
	androidBackend.renderFrame(androidWindow)
}

// TouchInput dispatches a touch event with a unique finger
// identifier for multi-touch support. Phase constants:
// 0=began, 1=moved, 2=ended, 3=cancelled.
// identifier is int64 (not uint64) for gomobile compatibility.
func TouchInput(phase int, identifier int64, x, y float32) {
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
	touchEvent(typ, uint64(identifier), x, y)
}

// TouchBegan dispatches a single-touch began event with id 0.
// Deprecated: use TouchInput for multi-touch support.
func TouchBegan(x, y float32) {
	touchEvent(gui.EventTouchesBegan, 0, x, y)
}

// TouchMoved dispatches a single-touch moved event with id 0.
// Deprecated: use TouchInput for multi-touch support.
func TouchMoved(x, y float32) {
	touchEvent(gui.EventTouchesMoved, 0, x, y)
}

// TouchEnded dispatches a single-touch ended event with id 0.
// Deprecated: use TouchInput for multi-touch support.
func TouchEnded(x, y float32) {
	touchEvent(gui.EventTouchesEnded, 0, x, y)
}

// Resize updates the viewport after a layout change.
func Resize(w, h int, scale float32) {
	if androidBackend == nil {
		return
	}
	androidBackend.handleResize(int32(w), int32(h), scale)
	if androidWindow != nil {
		evt := gui.Event{
			Type:         gui.EventResized,
			WindowWidth:  w,
			WindowHeight: h,
		}
		androidWindow.EventFn(&evt)
	}
}

// CleanUp releases all backend resources.
func CleanUp() {
	if androidBackend != nil {
		androidBackend.Destroy()
		androidBackend = nil
	}
	if androidWindow != nil {
		androidWindow.WindowCleanup()
		androidWindow = nil
	}
}

// --- Shared initialization ---

func initBackend(w, h int32, scale float32) {
	rc := C.glesInit()
	if rc != 0 {
		panic(fmt.Sprintf("android: glesInit failed: %d", rc))
	}

	physW := int32(float32(w) * scale)
	physH := int32(float32(h) * scale)
	C.glesResize(C.int(physW), C.int(physH))

	cfg := androidWindow.Config
	b := &Backend{
		dpiScale: scale,
		physW:    physW,
		physH:    physH,
		textures: newGLESTexCacheLRU(128),
		customCache: texcache.New[uint64, C.int](
			maxCustomPipelines,
			func(idx C.int) { C.glesDeleteCustomPipeline(idx) },
		),
		imagePathCache: texcache.New[string, string](1024, nil),
		maxImageBytes:  cfg.MaxImageBytes,
		maxImagePixels: cfg.MaxImagePixels,
		lastPipe:       -1,
	}
	b.allowedImageRoots = imgpath.NormalizeRoots(
		cfg.AllowedImageRoots)
	b.updateProjection()

	// Initialize glyph text system with GLES backend.
	b.glyphBack = newGLESGlyphBackend(scale)
	textSys, err := glyph.NewTextSystem(b.glyphBack)
	if err != nil {
		panic(fmt.Sprintf("android: NewTextSystem: %v", err))
	}
	b.textSys = textSys

	// Load embedded icon font.
	if data := gui.IconFontData; len(data) > 0 {
		tmp, err := tempfont.Write("go_gui_feathericon", data)
		if err != nil {
			log.Printf("android: write icon font: %v", err)
		} else if err = textSys.AddFontFile(tmp); err != nil {
			log.Printf("android: load icon font: %v", err)
			_ = os.Remove(tmp)
		} else {
			b.iconFontPath = tmp
		}
	}

	// Set injected interfaces on gui Window.
	androidWindow.SetTextMeasurer(
		&textMeasurer{textSys: textSys})
	androidWindow.SetSvgParser(svg.New())
	androidWindow.SetClipboardFn(func(_ string) {})
	androidWindow.SetClipboardGetFn(func() string { return "" })
	androidWindow.SetNativePlatform(&nativePlatform{})

	androidBackend = b

	// Fire initial resize so w.WindowSize() returns the
	// correct dimensions when the view function runs.
	evt := gui.Event{
		Type:         gui.EventResized,
		WindowWidth:  int(w),
		WindowHeight: int(h),
	}
	androidWindow.EventFn(&evt)

	if androidWindow.Config.OnInit != nil {
		androidWindow.Config.OnInit(androidWindow)
	}
}

// renderFrame clears the screen, draws the current layout, and
// flushes the GL pipeline.
func (b *Backend) renderFrame(w *gui.Window) {
	bg := w.Config.BgColor
	if bg == (gui.Color{}) {
		t := gui.CurrentTheme()
		bg = t.ColorBackground
	}

	C.glesBeginFrame(
		C.float(float32(bg.R)/255.0),
		C.float(float32(bg.G)/255.0),
		C.float(float32(bg.B)/255.0),
		C.float(float32(bg.A)/255.0),
	)

	b.invalidatePipelineState()
	b.setPipeline(pipeSolid)

	w.Lock()
	b.renderersDraw(w)
	w.Unlock()

	// Flush queued text.
	if b.textQueued {
		b.useGlyphPipeline()
		b.textSys.Commit()
		b.textQueued = false
	}

	C.glesEndFrame()
}

func (b *Backend) handleResize(w, h int32, scale float32) {
	b.dpiScale = scale
	b.physW = int32(float32(w) * scale)
	b.physH = int32(float32(h) * scale)
	C.glesResize(C.int(b.physW), C.int(b.physH))
	b.updateProjection()
}

func (b *Backend) updateProjection() {
	gpu.Ortho(&b.mvp,
		0, float32(b.physW),
		float32(b.physH), 0,
		-1, 1)
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
	C.glesDestroy()
}

// setPipeline sets GLES pipeline and MVP, skipping redundant
// CGo calls when unchanged.
func (b *Backend) setPipeline(pipe int) {
	if pipe == b.lastPipe && !b.mvpDirty {
		return
	}
	C.glesSetPipeline(C.int(pipe))
	C.glesSetMVP((*C.float)(&b.mvp[0]))
	b.lastPipe = pipe
	b.mvpDirty = false
}

// invalidatePipelineState forces the next setPipeline to issue
// CGo calls.
func (b *Backend) invalidatePipelineState() {
	b.lastPipe = -1
}

// useGlyphPipeline sets up GLES state for glyph text rendering.
func (b *Backend) useGlyphPipeline() {
	b.setPipeline(pipeGlyphTex)
}

// --- OpenURI bridge ---

var (
	pendingURIMu sync.Mutex
	pendingURI   string
)

// setPendingURI stores a URI for Kotlin to pick up.
func setPendingURI(uri string) {
	pendingURIMu.Lock()
	pendingURI = uri
	pendingURIMu.Unlock()
}

// PendingURI returns and clears the pending URI. Kotlin polls
// this each frame and opens via Intent.ACTION_VIEW.
func PendingURI() string {
	pendingURIMu.Lock()
	uri := pendingURI
	pendingURI = ""
	pendingURIMu.Unlock()
	return uri
}

// --- IME bridge ---

var (
	pendingIMEMu     sync.Mutex
	pendingIMEAction int32 // 0=none, 1=show, 2=hide
	pendingIMERectX  int32
	pendingIMERectY  int32
	pendingIMERectW  int32
	pendingIMERectH  int32
)

// setPendingIMEAction sets the IME action for Kotlin to pick up.
func setPendingIMEAction(action int32) {
	pendingIMEMu.Lock()
	pendingIMEAction = action
	pendingIMEMu.Unlock()
}

// setPendingIMERect sets the cursor rect for Kotlin.
func setPendingIMERect(x, y, w, h int32) {
	pendingIMEMu.Lock()
	pendingIMERectX = x
	pendingIMERectY = y
	pendingIMERectW = w
	pendingIMERectH = h
	pendingIMEMu.Unlock()
}

// PendingIMEAction returns and clears the pending IME action.
// 0=none, 1=show keyboard, 2=hide keyboard.
func PendingIMEAction() int32 {
	pendingIMEMu.Lock()
	a := pendingIMEAction
	pendingIMEAction = 0
	pendingIMEMu.Unlock()
	return a
}

// PendingIMERectX returns the IME cursor rect X coordinate.
func PendingIMERectX() int32 {
	pendingIMEMu.Lock()
	v := pendingIMERectX
	pendingIMEMu.Unlock()
	return v
}

// PendingIMERectY returns the IME cursor rect Y coordinate.
func PendingIMERectY() int32 {
	pendingIMEMu.Lock()
	v := pendingIMERectY
	pendingIMEMu.Unlock()
	return v
}

// PendingIMERectW returns the IME cursor rect width.
func PendingIMERectW() int32 {
	pendingIMEMu.Lock()
	v := pendingIMERectW
	pendingIMEMu.Unlock()
	return v
}

// PendingIMERectH returns the IME cursor rect height.
func PendingIMERectH() int32 {
	pendingIMEMu.Lock()
	v := pendingIMERectH
	pendingIMEMu.Unlock()
	return v
}

// IMEComposition is called from Kotlin with preedit text.
func IMEComposition(text string, cursor, selLen int64) {
	imeComposition(text, int32(cursor), int32(selLen))
}

// IMECommit is called from Kotlin when text is committed.
func IMECommit(text string) {
	imeCommit(text)
}

// --- Notification bridge ---

var (
	pendingNotifMu    sync.Mutex
	pendingNotifTitle string
	pendingNotifBody  string
	notifResultCh     = make(chan gui.NativeNotificationResult, 1)
)

// setPendingNotification queues a notification for Kotlin and
// blocks until the result is reported back or timeout expires.
func setPendingNotification(title, body string) gui.NativeNotificationResult {
	pendingNotifMu.Lock()
	pendingNotifTitle = title
	pendingNotifBody = body
	pendingNotifMu.Unlock()

	select {
	case r := <-notifResultCh:
		return r
	case <-time.After(10 * time.Second):
		return gui.NativeNotificationResult{
			Status:       gui.NotificationError,
			ErrorCode:    "timeout",
			ErrorMessage: "notification result not received",
		}
	}
}

// PendingNotificationTitle returns and clears the pending
// notification title. Empty string means no pending notification.
func PendingNotificationTitle() string {
	pendingNotifMu.Lock()
	t := pendingNotifTitle
	pendingNotifTitle = ""
	pendingNotifMu.Unlock()
	return t
}

// PendingNotificationBody returns the pending notification body.
func PendingNotificationBody() string {
	pendingNotifMu.Lock()
	b := pendingNotifBody
	pendingNotifBody = ""
	pendingNotifMu.Unlock()
	return b
}

// NotificationResult is called from Kotlin to report the
// notification outcome. Status: 0=OK, 1=denied, 2=error.
func NotificationResult(status int64, errCode, errMsg string) {
	r := gui.NativeNotificationResult{
		Status:       gui.NativeNotificationStatus(status),
		ErrorCode:    errCode,
		ErrorMessage: errMsg,
	}
	select {
	case notifResultCh <- r:
	default:
	}
}
