//go:build windows && !js

package gl

// GOGUI_RENDERER=soft present path for Windows (#1310, phase 2).
//
// A window on this path has no pixel format, no WGL context, no kept window
// DC and no GL function table, and the process never loads opengl32.dll.
// Each frame is drawn on the CPU by soft.Renderer, swizzled into a top-down
// 32 bpp DIB section that stays selected into a memory DC, and copied to the
// window with BitBlt. WM_PAINT copies the same DIB again. Everything else —
// events, IME, clipboard, DPI, cursors, window state, UI Automation — is the
// ordinary Win32 platform code, shared with GL.
//
// The window class has no CS_OWNDC: window DCs are taken per use (GetDC
// around a present, BeginPaint in WM_PAINT), and the memory DC is kept.

import (
	"errors"
	"fmt"
	"log"
	"sync"
	"syscall"
	"time"
	"unsafe"

	"golang.org/x/sys/windows"

	"github.com/go-gui-org/go-gui/gui"
	"github.com/go-gui-org/go-gui/gui/backend/soft"
)

var (
	// gdi32 is declared with the WGL bindings (wgl_windows.go), and
	// pDeleteObject with the transparency code (transparency_win32.go).
	pCreateCompatibleDC = gdi32.NewProc("CreateCompatibleDC")
	pDeleteDC           = gdi32.NewProc("DeleteDC")
	pCreateDIBSection   = gdi32.NewProc("CreateDIBSection")
	pSelectObject       = gdi32.NewProc("SelectObject")
	pBitBlt             = gdi32.NewProc("BitBlt")
	pGdiFlush           = gdi32.NewProc("GdiFlush")

	pBeginPaint           = user32.NewProc("BeginPaint")
	pEndPaint             = user32.NewProc("EndPaint")
	pEnumDisplaySettingsW = user32.NewProc("EnumDisplaySettingsW")
)

const (
	biRGB        = 0
	dibRGBColors = 0
	srcCopy      = 0x00CC0020
	// hgdiError is HGDI_ERROR, (HGDIOBJ)-1: SelectObject's failure value
	// besides NULL.
	hgdiError = ^uintptr(0)
	// enumCurrentSettings is ENUM_CURRENT_SETTINGS, (DWORD)-1.
	enumCurrentSettings = 0xFFFFFFFF
)

// bitmapInfoHeader mirrors BITMAPINFOHEADER.
type bitmapInfoHeader struct {
	biSize          uint32
	biWidth         int32
	biHeight        int32
	biPlanes        uint16
	biBitCount      uint16
	biCompression   uint32
	biSizeImage     uint32
	biXPelsPerMeter int32
	biYPelsPerMeter int32
	biClrUsed       uint32
	biClrImportant  uint32
}

// bitmapInfo mirrors BITMAPINFO with its one-entry color table, which a
// 32 bpp BI_RGB bitmap does not read.
type bitmapInfo struct {
	header bitmapInfoHeader
	colors [1]uint32
}

// paintStruct mirrors PAINTSTRUCT.
type paintStruct struct {
	hdc         uintptr
	fErase      int32
	rcPaint     rectW
	fRestore    int32
	fIncUpdate  int32
	rgbReserved [32]byte
}

// monitorInfoExW mirrors MONITORINFOEXW: MONITORINFO and the device name
// EnumDisplaySettingsW takes.
type monitorInfoExW struct {
	monitorInfo
	szDevice [32]uint16
}

// devModeW mirrors DEVMODEW. Only dmSize goes in and only
// dmDisplayFrequency is read; the rest is there for the layout.
type devModeW struct {
	dmDeviceName       [32]uint16
	dmSpecVersion      uint16
	dmDriverVersion    uint16
	dmSize             uint16
	dmDriverExtra      uint16
	dmFields           uint32
	dmPosition         [16]byte // a union: printer or display fields
	dmColor            int16
	dmDuplex           int16
	dmYResolution      int16
	dmTTOption         int16
	dmCollate          int16
	dmFormName         [32]uint16
	dmLogPixels        uint16
	dmBitsPerPel       uint32
	dmPelsWidth        uint32
	dmPelsHeight       uint32
	dmDisplayFlags     uint32
	dmDisplayFrequency uint32
	dmTail             [8]uint32 // ICM and panning fields
}

// dibHeader describes the frame store: top-down (negative height), so row
// 0 is the top row as in soft's RGBA buffer, 32 bpp BI_RGB, whose bytes
// are B, G, R and an unused fourth, the order swizzleBGRA writes.
func dibHeader(width, height int32) bitmapInfoHeader {
	h := bitmapInfoHeader{
		biWidth:       width,
		biHeight:      -height,
		biPlanes:      1,
		biBitCount:    32,
		biCompression: biRGB,
	}
	h.biSize = uint32(unsafe.Sizeof(h))
	return h
}

// displayFrameInterval is one frame at a display frequency of hz, as
// EnumDisplaySettingsW reports it. 0 and 1 mean the hardware default, and
// a rate outside 20–1000 Hz is a misreport; both pace at 60 Hz.
func displayFrameInterval(hz uint32) time.Duration {
	if hz < 20 || hz > 1000 {
		return defaultFrameInterval
	}
	return time.Second / time.Duration(hz)
}

// paceDeadline is softPresent.pace's arithmetic: given the deadline the
// last frame set and the time this frame was presented, it returns how
// long to sleep and the deadline for the next frame. A frame ahead of its
// deadline sleeps until it, and the next deadline is one interval later. A
// late frame does not sleep, and the next deadline counts from now, so a
// slow frame does not bank time for a burst after it.
func paceDeadline(next, now time.Time, interval time.Duration) (time.Duration, time.Time) {
	if next.After(now) {
		return next.Sub(now), next.Add(interval)
	}
	return 0, now.Add(interval)
}

// monitorFrequency reads the display frequency of mon's current mode, in
// Hz. 0 when it cannot be read.
func monitorFrequency(mon uintptr) uint32 {
	if mon == 0 {
		return 0
	}
	var mi monitorInfoExW
	mi.cbSize = uint32(unsafe.Sizeof(mi))
	if r, _, _ := pGetMonitorInfoW.Call(mon, uintptr(unsafe.Pointer(&mi))); r == 0 {
		return 0
	}
	var dm devModeW
	dm.dmSize = uint16(unsafe.Sizeof(dm))
	r, _, _ := pEnumDisplaySettingsW.Call(uintptr(unsafe.Pointer(&mi.szDevice[0])),
		enumCurrentSettings, uintptr(unsafe.Pointer(&dm)))
	if r == 0 {
		return 0
	}
	return dm.dmDisplayFrequency
}

// --- window class ---

var (
	softClassOnce sync.Once
	softClassName = windows.StringToUTF16Ptr("GoGuiSoftWindow")
)

// registerSoftClass registers the soft window class once and returns its
// name. It is the GL class without CS_OWNDC: a soft window keeps no window
// DC, and CS_OWNDC excludes WS_EX_LAYERED, which phase 4 needs.
func registerSoftClass(hInstance uintptr) *uint16 {
	softClassOnce.Do(func() {
		wc := wndClassExW{
			style:         csHRedraw | csVRedraw,
			lpfnWndProc:   wndProcCallback,
			hInstance:     hInstance,
			lpszClassName: softClassName,
		}
		hArrow, _, _ := pLoadCursorW.Call(0, idcArrow)
		wc.hCursor = hArrow
		wc.cbSize = uint32(unsafe.Sizeof(wc))
		pRegisterClassExW.Call(uintptr(unsafe.Pointer(&wc)))
	})
	return softClassName
}

// reportSoftTransparency reports a WindowCfg.Transparent window on soft,
// which stays opaque and opens with no error (#1310, decision 3).
func reportSoftTransparency(w *gui.Window) {
	w.DebugWindowTransparency("the CPU renderer (GOGUI_RENDERER=soft) " +
		"draws it opaque on Windows")
}

// --- present ---

// softPresentWin32 is one window's CPU render-and-present state. Non-nil
// platformState.soft selects this path.
type softPresentWin32 struct {
	hwnd uintptr
	r    *soft.Renderer

	// memDC holds the DIB section for the window's whole life. stock is
	// the bitmap it was created with, put back before the DC is deleted.
	memDC uintptr
	stock uintptr
	// bmp is the DIB section selected into memDC, dibW x dibH pixels, and
	// pix its bits. The three change together, and only once a new DIB is
	// selected, so WM_PAINT always has the last frame that was stored.
	bmp        uintptr
	dibW, dibH int32
	pix        []byte

	// ps is BeginPaint's output, kept so a WM_PAINT allocates nothing.
	ps paintStruct

	// Frame pacing stands in for wglSwapIntervalEXT(1): without it, a view
	// that invalidates every frame would redraw as fast as the CPU allows.
	// interval is the display frequency of pacedFor, the monitor the
	// window was last presented on; paced is false until it is read, and
	// again after WM_DISPLAYCHANGE.
	vsyncOff bool
	interval time.Duration
	pacedFor uintptr
	paced    bool
	next     time.Time

	// skipLogged keeps a run of skipped frames to one log line.
	skipLogged bool
}

// createDIBSectionFunc and selectObjectFunc are the seams the resize test
// replaces to make an allocation fail.
var (
	createDIBSectionFunc = createDIBSection
	selectObjectFunc     = selectObject
)

// createDIBSection creates a DIB section described by bmi and stores its
// bits pointer in *bits.
func createDIBSection(hdc uintptr, bmi *bitmapInfo, bits *unsafe.Pointer) (uintptr, error) {
	bmp, _, err := pCreateDIBSection.Call(hdc, uintptr(unsafe.Pointer(bmi)),
		dibRGBColors, uintptr(unsafe.Pointer(bits)), 0, 0)
	if bmp == 0 {
		return 0, fmt.Errorf("CreateDIBSection: %w", err)
	}
	return bmp, nil
}

// selectObject selects obj into hdc and returns the object it replaced.
func selectObject(hdc, obj uintptr) (uintptr, error) {
	old, _, err := pSelectObject.Call(hdc, obj)
	if old == 0 || old == hgdiError {
		return 0, fmt.Errorf("SelectObject: %w", err)
	}
	return old, nil
}

// newSoftPresentWin32 creates the memory DC and the renderer for hwnd. The
// DIB comes with the first frame, at the size that frame is drawn at.
func newSoftPresentWin32(hwnd uintptr, w *gui.Window, scale float32) (*softPresentWin32, error) {
	memDC, _, err := pCreateCompatibleDC.Call(0)
	if memDC == 0 {
		return nil, fmt.Errorf("gl: soft: CreateCompatibleDC: %w", err)
	}
	r, rerr := soft.NewRenderer(w, scale)
	if rerr != nil {
		pDeleteDC.Call(memDC)
		return nil, fmt.Errorf("gl: soft: %w", rerr)
	}
	return &softPresentWin32{
		hwnd:     hwnd,
		r:        r,
		memDC:    memDC,
		vsyncOff: w.Config.VSyncOff,
		interval: defaultFrameInterval,
	}, nil
}

// present draws w at physW x physH, stores it in the DIB, copies the DIB
// to the window and paces to the monitor. It reports whether the frame
// reached the window.
//
// Draw reads the window's render commands, so it runs under the window
// lock, as the GL drawFrame does; the swizzle and the blit run after the
// unlock. A frame Draw refuses, or one whose DIB cannot be allocated, is
// skipped, logged once and still paced: an animating view asks for a frame
// every loop, and a frame that fails every time would otherwise spin a
// core doing nothing. The DIB's size is compared with the frame's at every
// present, so the frame after a failed allocation tries again.
func (sp *softPresentWin32) present(w *gui.Window, physW, physH int32) bool {
	sp.checkMonitor()
	w.Lock()
	img, err := sp.r.Draw(w, int(physW), int(physH))
	w.Unlock()
	if err != nil {
		sp.skip(err)
		return false
	}
	if aerr := sp.ensureDIB(physW, physH); aerr != nil {
		sp.skip(aerr)
		return false
	}
	swizzleBGRA(sp.pix, img.Pix)
	if !sp.blit() {
		// The frame is in the DIB, so a WM_PAINT can still show it.
		sp.skip(errSoftBlit)
		return false
	}
	sp.skipLogged = false
	sp.pace()
	return true
}

// errSoftBlit: GetDC, BitBlt or GdiFlush failed, so the frame did not
// reach the window.
var errSoftBlit = errors.New("the copy to the window failed")

// skip logs the first of a run of skipped frames and paces.
func (sp *softPresentWin32) skip(err error) {
	if !sp.skipLogged {
		log.Printf("gl: soft: frame skipped: %v", err)
		sp.skipLogged = true
	}
	sp.pace()
}

// ensureDIB makes the DIB width x height. The new DIB is created, then
// selected into the memory DC; only when the selection succeeds do the
// bitmap, its size and its bits change, and the old bitmap is deleted.
// When either step fails the old three stay and the new bitmap, if any, is
// deleted. A failed CreateDIBSection means memory or GDI handles ran out,
// so there is no retry timer: the next present compares the sizes again.
func (sp *softPresentWin32) ensureDIB(width, height int32) error {
	if sp.bmp != 0 && sp.dibW == width && sp.dibH == height {
		return nil
	}
	bmi := bitmapInfo{header: dibHeader(width, height)}
	var bits unsafe.Pointer
	bmp, err := createDIBSectionFunc(sp.memDC, &bmi, &bits)
	if err != nil {
		return fmt.Errorf("%dx%d frame: %w", width, height, err)
	}
	old, err := selectObjectFunc(sp.memDC, bmp)
	if err != nil {
		pDeleteObject.Call(bmp)
		return fmt.Errorf("%dx%d frame: %w", width, height, err)
	}
	if sp.bmp == 0 {
		sp.stock = old
	} else {
		// Every blit is flushed before present returns, so GDI has
		// finished reading the old bitmap.
		pDeleteObject.Call(sp.bmp)
	}
	sp.bmp, sp.dibW, sp.dibH = bmp, width, height
	sp.pix = unsafe.Slice((*byte)(bits), int(width)*int(height)*4)
	return nil
}

// The per-frame GDI and user32 calls below go through syscall.SyscallN, not
// LazyProc.Call: Call is go:uintptrescapes, so its variadic arguments move
// to the heap, one allocation per call and four per frame.

// blit copies the DIB to the window through a window DC taken for the
// call.
func (sp *softPresentWin32) blit() bool {
	hdc, _, _ := syscall.SyscallN(pGetDC.Addr(), sp.hwnd)
	if hdc == 0 {
		return false
	}
	ok := sp.blitTo(hdc)
	syscall.SyscallN(pReleaseDC.Addr(), sp.hwnd, hdc)
	return ok
}

// blitTo copies the DIB to hdc and flushes GDI's batch at once, while hdc
// is still held. A DIB section's bits may only be written, or the bitmap
// deleted, after GDI has finished reading them (CreateDIBSection), so the
// flush runs with VSyncOff too. The copy counts only when both succeed.
func (sp *softPresentWin32) blitTo(hdc uintptr) bool {
	r, _, _ := syscall.SyscallN(pBitBlt.Addr(), hdc, 0, 0,
		uintptr(sp.dibW), uintptr(sp.dibH), sp.memDC, 0, 0, srcCopy)
	f, _, _ := syscall.SyscallN(pGdiFlush.Addr())
	return r != 0 && f != 0
}

// paint answers WM_PAINT: it copies the last frame to the invalid area and
// validates it. It never draws a new frame and never sleeps. Before the
// first frame it only validates; that frame covers the window.
func (sp *softPresentWin32) paint() {
	hdc, _, _ := syscall.SyscallN(pBeginPaint.Addr(), sp.hwnd,
		uintptr(unsafe.Pointer(&sp.ps)))
	if hdc != 0 && sp.bmp != 0 {
		sp.blitTo(hdc)
	}
	syscall.SyscallN(pEndPaint.Addr(), sp.hwnd, uintptr(unsafe.Pointer(&sp.ps)))
}

// pace sleeps out the rest of the monitor's frame interval. Sleep is a
// best-effort limiter: it can come back milliseconds late.
func (sp *softPresentWin32) pace() {
	if sp.vsyncOff {
		return
	}
	wait, next := paceDeadline(sp.next, time.Now(), sp.interval)
	if wait > 0 {
		time.Sleep(wait)
	}
	sp.next = next
}

// checkMonitor re-reads the frame interval when the window is on another
// monitor than the one it was read for, or after WM_DISPLAYCHANGE. It runs
// at every present, so a move or a resize to another monitor is caught by
// the next frame; the check is one MonitorFromWindow call.
func (sp *softPresentWin32) checkMonitor() {
	mon, _, _ := syscall.SyscallN(pMonitorFromWindow.Addr(), sp.hwnd,
		monitorDefaultToNearest)
	if sp.paced && mon == sp.pacedFor {
		return
	}
	sp.paced, sp.pacedFor = true, mon
	sp.interval = displayFrameInterval(monitorFrequency(mon))
}

// destroy flushes GDI, then frees the DIB, the memory DC and the renderer.
// The window itself is the platform's to destroy.
func (sp *softPresentWin32) destroy() {
	if sp == nil {
		return
	}
	pGdiFlush.Call()
	if sp.memDC != 0 {
		if sp.bmp != 0 {
			pSelectObject.Call(sp.memDC, sp.stock)
			pDeleteObject.Call(sp.bmp)
			sp.bmp, sp.pix = 0, nil
		}
		pDeleteDC.Call(sp.memDC)
		sp.memDC = 0
	}
	sp.r.Release()
}

// --- Backend glue: the sites where the Win32 path branches on soft ---

// newSoftWin32 is New's soft attempt: the window of the soft class, then
// the CPU renderer in place of WGL and the GL resources.
func newSoftWin32(w *gui.Window) (*Backend, error) {
	b, err := openWin32Window(w, true)
	if err != nil {
		return nil, err
	}
	if serr := b.initSoftWin32(w); serr != nil {
		b.plat.destroy()
		return nil, serr
	}
	finishWin32Window(w, b)
	return b, nil
}

// initSoftWin32 stands in for initGLResources on a soft window: the
// renderer installs the text measurer and SVG parser, and the window is
// told it cannot run custom shaders (soft has no GLSL; a shader container
// draws its children and no fill).
func (b *Backend) initSoftWin32(w *gui.Window) error {
	hwnd := b.plat.hwnd
	b.dpiScale = windowScale(hwnd)
	b.physW, b.physH = clientSize(hwnd)
	sp, err := newSoftPresentWin32(hwnd, w, b.dpiScale)
	if err != nil {
		return err
	}
	b.plat.softMode = true
	b.plat.soft = sp
	w.SetNativePlatform(&nativePlatform{b: b})
	gui.SetWindowRendersShaders(w, false)
	return nil
}

// destroySoftWin32 stands in for destroyGLResources, which would make a
// GL context current.
func (b *Backend) destroySoftWin32() {
	b.plat.soft.destroy()
	b.plat.soft = nil
}

// presentWin32 is the frame call of both loops and of WM_SIZE: GL draws
// and swaps, soft draws on the CPU and blits.
func (b *Backend) presentWin32(w *gui.Window) {
	if sp := b.plat.soft; sp != nil {
		sp.present(w, b.physW, b.physH)
		return
	}
	b.renderFrame(w)
}

// resizeWin32 is handleResize for WM_SIZE. handleResize sets the GL
// viewport and projection, which a soft window has no context for; soft
// needs only the new size and scale.
func (b *Backend) resizeWin32() {
	sp := b.plat.soft
	if sp == nil {
		b.handleResize()
		return
	}
	b.physW, b.physH = b.plat.drawableSize()
	// applyDPIScale validates the scale and is nil-safe for a window with
	// no GL glyph backend; the renderer then moves its text system to the
	// scale applyDPIScale kept.
	b.applyDPIScale(b.plat.dpiScale())
	if err := sp.r.SetScale(b.dpiScale); err != nil {
		log.Printf("gl: soft: rescale: %v", err)
	}
}

// paintWin32 answers WM_PAINT. A GL window only validates: its next swap
// repaints. A soft window copies its last frame back.
func (b *Backend) paintWin32() {
	if sp := b.plat.soft; sp != nil {
		sp.paint()
		return
	}
	pValidateRect.Call(b.plat.hwnd, 0)
}

// displayChangedWin32 tells the soft pacer that display modes changed, so
// the next present re-reads the frequency.
func (b *Backend) displayChangedWin32() {
	if sp := b.plat.soft; sp != nil {
		sp.paced = false
	}
}
