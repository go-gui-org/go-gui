//go:build windows && !js

package gl

// CPU renderer present path for Windows (#1310): GOGUI_RENDERER=soft since
// phase 2, and since phase 3 the fallback of a first window whose GL context
// or binding fails with the variable unset (renderer_pick_win32.go).
//
// A window on this path has no pixel format, no WGL context, no kept window
// DC and no GL function table. With GOGUI_RENDERER=soft the process never
// loads opengl32.dll; after a fallback, the DLL the GL attempt loaded stays
// mapped.
// Each frame is drawn on the CPU by soft.Renderer, swizzled into a top-down
// 32 bpp DIB section that stays selected into a memory DC, and copied to the
// window with BitBlt. WM_PAINT copies the same DIB again. Everything else —
// events, IME, clipboard, DPI, cursors, window state, UI Automation — is the
// ordinary Win32 platform code, shared with GL.
//
// A WindowCfg.Transparent window without a system frame (DecorationNone) is
// a layered window instead (phase 4): WS_EX_LAYERED, and each frame goes to
// the system with UpdateLayeredWindow, which takes the DIB's premultiplied
// alpha, so the desktop shows through where the frame's alpha is below 255.
// A transparent window with a system frame stays opaque, with a report.
//
// The window class has no CS_OWNDC: window DCs are taken per use (GetDC
// around a present, BeginPaint in WM_PAINT), and the memory DC is kept.

import (
	"encoding/binary"
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
	pUpdateLayeredWindow  = user32.NewProc("UpdateLayeredWindow")
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

	// UpdateLayeredWindow's flag and BLENDFUNCTION values: blend the
	// bitmap over what is behind the window by its own premultiplied
	// alpha.
	ulwAlpha   = 0x00000002
	acSrcOver  = 0x00
	acSrcAlpha = 0x01

	// swpNoSendChanging is SWP_NOSENDCHANGING: SetWindowPos sends no
	// WM_WINDOWPOSCHANGING, where DefWindowProc applies the minimum
	// track size.
	swpNoSendChanging = 0x0400
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

// sizeW mirrors SIZE.
type sizeW struct{ cx, cy int32 }

// blendFunction mirrors BLENDFUNCTION.
type blendFunction struct {
	blendOp, blendFlags, sourceConstantAlpha, alphaFormat byte
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
// are B, G, R and A, the order swizzleBGRA writes. BitBlt to an opaque
// window ignores the fourth byte; UpdateLayeredWindow reads it as
// premultiplied alpha.
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

// swizzleBGRALayered is swizzleBGRA for a layered window, with the alpha
// floor of #1310's decision 4 in the same pass: a pixel whose alpha is 0
// gets alpha 1, and every other alpha is unchanged. A layered window passes
// the mouse through where alpha is 0 and takes it where alpha is 1 or more,
// while a transparent GL window takes it everywhere. A premultiplied pixel
// of alpha 0 has no color, so it becomes (0, 0, 0, 1): 1/255 of black over
// the desktop.
//
// The floor is branch-free on the swizzled word v, two pixels at once.
// Adding 0x7f to an alpha's low seven bits carries into its top bit exactly
// when they are not all 0, and cannot carry past the byte, so after OR-ing
// in v itself and every other bit, t holds bit 31 or 63 where that pixel's
// alpha is 0; shifted down by seven it is the alpha's low bit. The opaque
// path keeps swizzleBGRA, without this step.
func swizzleBGRALayered(dst, src []byte) {
	const (
		greenAlpha = 0xff00ff00_ff00ff00
		red        = 0x000000ff_000000ff
		blue       = 0x00ff0000_00ff0000
		alphaLow7  = 0x7f000000_7f000000
		notAlphaHi = 0x7fffffff_7fffffff
	)
	n := min(len(dst), len(src)) &^ 3
	dst, src = dst[:n], src[:n]
	for len(src) >= 8 {
		v := binary.LittleEndian.Uint64(src)
		v = v&greenAlpha | (v&red)<<16 | (v&blue)>>16
		t := ^(((v & alphaLow7) + alphaLow7) | v | notAlphaHi)
		binary.LittleEndian.PutUint64(dst, v|t>>7)
		dst, src = dst[8:], src[8:]
	}
	if len(src) == 4 {
		// The odd pixel; read before written, as in swizzleBGRA.
		dst[0], dst[1], dst[2], dst[3] = src[2], src[1], src[0], max(src[3], 1)
	}
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
// DC, and CS_OWNDC excludes WS_EX_LAYERED, which a transparent frameless
// soft window has.
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

// softExStyle is the extended style of a new window: WS_EX_LAYERED for a
// Transparent frameless soft window, which is presented with
// UpdateLayeredWindow, and none otherwise. UpdateLayeredWindow's bitmap is
// the whole window, so a window with a system frame is not layered: it
// stays opaque (reportSoftTransparency).
func softExStyle(soft bool, cfg gui.WindowCfg) uintptr {
	if soft && cfg.Transparent && cfg.Decorations == gui.DecorationNone {
		return wsExLayered
	}
	return 0
}

// reportSoftTransparency reports a WindowCfg.Transparent window with a
// system frame on soft, which stays opaque and opens with no error (#1310,
// decision 3). The text does not name GOGUI_RENDERER=soft: a fallback from
// GL lands here too.
func reportSoftTransparency(report func(string)) {
	report("the CPU renderer on Windows draws a window with a system frame " +
		"opaque; a frameless one (DecorationNone) is transparent")
}

// fitLayeredFrame makes a layered window's client area the whole window,
// at the client size it was created with. UpdateLayeredWindow's bitmap
// covers the whole window and the frame is drawn at the client size, so
// the two must be equal, or the frame would be offset from the mouse
// coordinates and could not be sent without resizing the window. A
// frameless window answers WM_NCCALCSIZE with no non-client area, but the
// first one comes inside CreateWindowExW, before the window is registered,
// and DefWindowProc gives a WS_THICKFRAME window its resize border there
// (a 320x200 client in a 334x214 window at 100 %, measured). The frame
// change sends it again, and routeMessage answers it for a layered window
// before Run.
//
// The size change skips WM_WINDOWPOSCHANGING: through it, DefWindowProc
// holds a WS_THICKFRAME window to its minimum track size, 14x14 at 100 %,
// so an 8x80 client would become 14x80 (measured). Creating the window at
// the client size does not avoid that: CreateWindowExW applies the same
// minimum (measured). The size limits (trackSizeFor) are only answered
// once Run has started, so they did not apply here before either.
func fitLayeredFrame(hwnd uintptr) {
	cw, ch := clientSize(hwnd)
	pSetWindowPos.Call(hwnd, 0, 0, 0, uintptr(cw), uintptr(ch),
		swpNoMove|swpNoZOrder|swpNoActivate|swpFrameChanged|swpNoSendChanging)
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

	// layered: the window is WS_EX_LAYERED and each frame goes to it with
	// UpdateLayeredWindow. The call's arguments are kept here, as ps is,
	// so a present allocates nothing: wr is the window rect read before
	// the call, size the size passed, src the DIB's origin.
	layered bool
	wr      rectW
	size    sizeW
	src     pointW
	blend   blendFunction

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
// replaces to make an allocation fail; newSoftPresentWin32Func the one the
// fallback test replaces to make the soft setup fail.
var (
	createDIBSectionFunc    = createDIBSection
	selectObjectFunc        = selectObject
	newSoftPresentWin32Func = newSoftPresentWin32
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
	if physW <= 0 || physH <= 0 {
		// A client area 0 wide or high has nothing to draw: GL draws an
		// empty viewport here and logs nothing. Draw would refuse the
		// size and skip would log it as a fault, so return first. Still
		// pace, so an animating view does not spin a core on empty frames.
		sp.pace()
		return false
	}
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
	if sp.layered {
		swizzleBGRALayered(sp.pix, img.Pix)
		if lerr := sp.updateLayered(); lerr != nil {
			// The frame is in the DIB; the next present sends it again.
			if errors.Is(lerr, errLayeredMinimized) {
				sp.pace()
			} else {
				sp.skip(lerr)
			}
			return false
		}
	} else {
		swizzleBGRA(sp.pix, img.Pix)
		if !sp.blit() {
			// The frame is in the DIB, so a WM_PAINT can still show it.
			sp.skip(errSoftBlit)
			return false
		}
	}
	sp.skipLogged = false
	sp.pace()
	return true
}

// errSoftBlit: GetDC, BitBlt or GdiFlush failed, so the frame did not
// reach the window.
var errSoftBlit = errors.New("the copy to the window failed")

// errLayeredMinimized: a minimized layered window whose parked size is not
// the frame's is not updated, and that is not a fault to log. Its window
// rect is the parked icon's, which the frame's size would resize; the
// WM_SIZE of the restore presents again.
var errLayeredMinimized = errors.New("the window is minimized")

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

// updateLayered sends the DIB to the layered window: UpdateLayeredWindow
// from the memory DC, blended by the bitmap's premultiplied alpha, then a
// flush, as blitTo flushes. The position is left as it is (no pptDst).
// The size cannot be left out: without psize the call succeeds and shows
// nothing new, and a psize unlike the window's resizes the window, a
// minimized one too (both measured). So the window rect is read first, and
// the frame is sent only when its size is the window's, which
// fitLayeredFrame, WM_NCCALCSIZE and the frameless maximize clamp keep
// true: a frame never resizes the window. When the sizes differ, a
// minimized window is skipped quietly (errLayeredMinimized) and any other
// difference is a skipped frame. IsIconic is asked only then: a minimized
// window whose parked size happens to equal the frame's is updated, which
// resizes nothing and is harmless.
func (sp *softPresentWin32) updateLayered() error {
	if r, _, e := syscall.SyscallN(pGetWindowRect.Addr(), sp.hwnd,
		uintptr(unsafe.Pointer(&sp.wr))); r == 0 {
		return fmt.Errorf("GetWindowRect: %w", e)
	}
	ww, wh := sp.wr.right-sp.wr.left, sp.wr.bottom-sp.wr.top
	if ww != sp.dibW || wh != sp.dibH {
		if r, _, _ := syscall.SyscallN(pIsIconic.Addr(), sp.hwnd); r != 0 {
			return errLayeredMinimized
		}
		return fmt.Errorf("the window is %dx%d and the frame %dx%d; "+
			"UpdateLayeredWindow would resize the window", ww, wh, sp.dibW, sp.dibH)
	}
	sp.size = sizeW{cx: sp.dibW, cy: sp.dibH}
	r, _, e := syscall.SyscallN(pUpdateLayeredWindow.Addr(), sp.hwnd, 0, 0,
		uintptr(unsafe.Pointer(&sp.size)), sp.memDC, uintptr(unsafe.Pointer(&sp.src)),
		0, uintptr(unsafe.Pointer(&sp.blend)), ulwAlpha)
	f, _, _ := syscall.SyscallN(pGdiFlush.Addr())
	if r == 0 {
		return fmt.Errorf("UpdateLayeredWindow: %w", e)
	}
	if f == 0 {
		return errors.New("GdiFlush failed after UpdateLayeredWindow")
	}
	return nil
}

// paint answers WM_PAINT: it copies the last frame to the invalid area and
// validates it. It never draws a new frame and never sleeps. Before the
// first frame it only validates; that frame covers the window. A layered
// window only validates: the system composes it from the bitmap the last
// UpdateLayeredWindow gave it, which no window DC paints. It is sent no
// WM_PAINT after a present, a resize or a minimize and restore, but an
// InvalidateRect still sends one, with an empty paint rect (measured), and
// an unvalidated window would be sent it again and again.
func (sp *softPresentWin32) paint() {
	hdc, _, _ := syscall.SyscallN(pBeginPaint.Addr(), sp.hwnd,
		uintptr(unsafe.Pointer(&sp.ps)))
	if hdc != 0 && sp.bmp != 0 && !sp.layered {
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
// the CPU renderer in place of WGL and the GL resources. The window setup's
// degradation reports go to rep. A failure of the renderer's own setup is a
// softInitError, which a fallback from GL reports after the GPU error.
// commit is as for newGLWin32: when not nil, it runs once nothing can fail,
// before application code can run.
func newSoftWin32(w *gui.Window, rep *windowReports, commit func()) (*Backend, error) {
	b, err := openWin32Window(w, true, rep)
	if err != nil {
		return nil, err
	}
	if serr := b.initSoftWin32(w, commit); serr != nil {
		b.plat.destroy()
		return nil, &softInitError{err: serr}
	}
	finishWin32Window(w, b)
	return b, nil
}

// initSoftWin32 stands in for initGLResources on a soft window: the
// renderer installs the text measurer and SVG parser, and the window is
// told it cannot run custom shaders (soft has no GLSL; a shader container
// draws its children and no fill). commit runs before the native platform
// attaches, the first point where application code can run.
func (b *Backend) initSoftWin32(w *gui.Window, commit func()) error {
	hwnd := b.plat.hwnd
	if b.plat.layered {
		fitLayeredFrame(hwnd)
	}
	b.dpiScale = windowScale(hwnd)
	b.physW, b.physH = clientSize(hwnd)
	sp, err := newSoftPresentWin32Func(hwnd, w, b.dpiScale)
	if err != nil {
		return err
	}
	if b.plat.layered {
		sp.layered = true
		sp.blend = blendFunction{blendOp: acSrcOver, sourceConstantAlpha: 255,
			alphaFormat: acSrcAlpha}
	}
	b.plat.softMode = true
	b.plat.soft = sp
	// Nothing below, or in finishWin32Window, fails.
	if commit != nil {
		commit()
	}
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
// and swaps, soft draws on the CPU and blits it or updates the layered
// window.
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
// repaints. A soft window copies its last frame back, unless it is layered.
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
