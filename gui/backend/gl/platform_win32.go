//go:build windows && !js

package gl

import (
	"fmt"
	"log"
	"runtime"
	"sync"
	"syscall"
	"unsafe"

	"github.com/go-gl/gl/v3.3-core/gl"
	"golang.org/x/sys/windows"

	"github.com/go-gui-org/go-gui/gui"
)

// --- Win32 API bindings ---

var (
	kernel32 = windows.NewLazySystemDLL("kernel32.dll")
	user32   = windows.NewLazySystemDLL("user32.dll")

	pGetModuleHandleW = kernel32.NewProc("GetModuleHandleW")
	pGlobalAlloc      = kernel32.NewProc("GlobalAlloc")
	pGlobalLock       = kernel32.NewProc("GlobalLock")
	pGlobalUnlock     = kernel32.NewProc("GlobalUnlock")
	pRtlMoveMemory    = kernel32.NewProc("RtlMoveMemory")
	pLstrlenW         = kernel32.NewProc("lstrlenW")

	pRegisterClassExW = user32.NewProc("RegisterClassExW")
	pCreateWindowExW  = user32.NewProc("CreateWindowExW")
	pDestroyWindow    = user32.NewProc("DestroyWindow")
	pDefWindowProcW   = user32.NewProc("DefWindowProcW")
	pShowWindow       = user32.NewProc("ShowWindow")
	pUpdateWindow     = user32.NewProc("UpdateWindow")
	pPeekMessageW     = user32.NewProc("PeekMessageW")
	pTranslateMessage = user32.NewProc("TranslateMessage")
	pDispatchMessageW = user32.NewProc("DispatchMessageW")
	pPostMessageW     = user32.NewProc("PostMessageW")
	pGetDC            = user32.NewProc("GetDC")
	pReleaseDC        = user32.NewProc("ReleaseDC")
	pMsgWaitForMulti  = user32.NewProc("MsgWaitForMultipleObjectsEx")
	pLoadCursorW      = user32.NewProc("LoadCursorW")
	pSetCursor        = user32.NewProc("SetCursor")
	pGetClientRect    = user32.NewProc("GetClientRect")
	pSetWindowTextW   = user32.NewProc("SetWindowTextW")
	pScreenToClient   = user32.NewProc("ScreenToClient")
	pSetCapture       = user32.NewProc("SetCapture")
	pReleaseCapture   = user32.NewProc("ReleaseCapture")
	pValidateRect     = user32.NewProc("ValidateRect")
	pGetDpiForWindow  = user32.NewProc("GetDpiForWindow")
	pGetDpiForSystem  = user32.NewProc("GetDpiForSystem")
	pSetDpiAware      = user32.NewProc("SetProcessDpiAwarenessContext")
	pAdjustRectDpi    = user32.NewProc("AdjustWindowRectExForDpi")
	pOpenClipboard    = user32.NewProc("OpenClipboard")
	pCloseClipboard   = user32.NewProc("CloseClipboard")
	pEmptyClipboard   = user32.NewProc("EmptyClipboard")
	pGetClipboardData = user32.NewProc("GetClipboardData")
	pSetClipboardData = user32.NewProc("SetClipboardData")
)

const (
	csVRedraw = 0x0001
	csHRedraw = 0x0002
	csOwnDC   = 0x0020

	wsOverlappedWindow = 0x00CF0000
	wsFixed            = 0x00CA0000 // caption|sysmenu|minimizebox
	wsClipSiblings     = 0x04000000
	wsClipChildren     = 0x02000000

	swShow       = 5
	pmRemove     = 0x0001
	cwUseDefault = 0x80000000

	qsAllInput         = 0x04FF
	mwmoInputAvailable = 0x0004

	gmemMoveable  = 0x0002
	cfUnicodeText = 13

	idcArrow    = 32512
	idcIBeam    = 32513
	idcCross    = 32515
	idcSizeNWSE = 32642
	idcSizeNESW = 32643
	idcSizeWE   = 32644
	idcSizeNS   = 32645
	idcSizeAll  = 32646
	idcNo       = 32648
	idcHand     = 32649

	// DPI_AWARENESS_CONTEXT_PER_MONITOR_AWARE_V2 == (HANDLE)-4.
	dpiPerMonitorV2 = ^uintptr(3)

	// maxClipboardChars bounds a clipboard read. Any process can place
	// arbitrarily large text on the clipboard; cap the allocation to
	// avoid an out-of-memory DoS (32 MiB of UTF-16).
	maxClipboardChars = 16 << 20
)

type wndClassExW struct {
	cbSize        uint32
	style         uint32
	lpfnWndProc   uintptr
	cbClsExtra    int32
	cbWndExtra    int32
	hInstance     uintptr
	hIcon         uintptr
	hCursor       uintptr
	hbrBackground uintptr
	lpszMenuName  *uint16
	lpszClassName *uint16
	hIconSm       uintptr
}

type pointW struct{ x, y int32 }

type rectW struct{ left, top, right, bottom int32 }

type msgW struct {
	hwnd     uintptr
	message  uint32
	wParam   uintptr
	lParam   uintptr
	time     uint32
	pt       pointW
	lPrivate uint32
}

// --- window registry (hwnd → Backend) ---

var (
	winMu  sync.Mutex
	winReg = map[uintptr]*Backend{}

	wndProcCallback = syscall.NewCallback(wndProc)
	classOnce       sync.Once
	className       = windows.StringToUTF16Ptr("GoGuiGLWindow")
)

func registerWindow(hwnd uintptr, b *Backend) {
	winMu.Lock()
	winReg[hwnd] = b
	winMu.Unlock()
}

func unregisterWindow(hwnd uintptr) {
	winMu.Lock()
	delete(winReg, hwnd)
	winMu.Unlock()
}

func lookupWindow(hwnd uintptr) *Backend {
	winMu.Lock()
	b := winReg[hwnd]
	winMu.Unlock()
	return b
}

// wndProc is the window procedure. It routes messages to the owning
// Backend, falling back to DefWindowProc for unhandled messages and
// for messages that arrive before the window is fully wired.
func wndProc(hwnd, msg, wparam, lparam uintptr) uintptr {
	if b := lookupWindow(hwnd); b != nil && b.plat.w != nil {
		if res, handled := b.handleMessage(msg, wparam, lparam); handled {
			return res
		}
	}
	r, _, _ := pDefWindowProcW.Call(hwnd, msg, wparam, lparam)
	return r
}

// --- platformState (Win32 + WGL) ---

type platformState struct {
	hwnd      uintptr
	hdc       uintptr
	hglrc     uintptr
	cursors   [11]uintptr
	curCursor uintptr
	w         *gui.Window
	evt       gui.Event // reused per message to avoid per-event allocation
	highSurr  uint16    // pending UTF-16 high surrogate for WM_CHAR
}

func (p *platformState) wake() {
	pPostMessageW.Call(p.hwnd, wmApp, 0, 0)
}

// pumpMessages drains all pending window messages, dispatching each
// through wndProc (which delivers gui events synchronously).
func pumpMessages(msg *msgW) {
	for {
		r, _, _ := pPeekMessageW.Call(
			uintptr(unsafe.Pointer(msg)), 0, 0, 0, pmRemove)
		if r == 0 {
			return
		}
		pTranslateMessage.Call(uintptr(unsafe.Pointer(msg)))
		pDispatchMessageW.Call(uintptr(unsafe.Pointer(msg)))
	}
}

// waitMessage blocks until a message arrives or the timeout elapses.
func waitMessage(ms uintptr) {
	pMsgWaitForMulti.Call(0, 0, ms, qsAllInput, mwmoInputAvailable)
}

func (p *platformState) makeCurrent() { wglMakeCurrent(p.hdc, p.hglrc) }
func (p *platformState) swap()        { swapBuffers(p.hdc) }

func (p *platformState) drawableSize() (int32, int32) {
	return clientSize(p.hwnd)
}

func (p *platformState) dpiScale() float32 {
	return float32(dpiForWindow(p.hwnd)) / 96.0
}

func (p *platformState) setCursor(mc gui.MouseCursor) {
	if int(mc) >= len(p.cursors) {
		return
	}
	c := p.cursors[mc]
	if c == 0 {
		return
	}
	p.curCursor = c
	pSetCursor.Call(c)
}

func (p *platformState) destroy() {
	if p.hglrc != 0 {
		wglMakeCurrent(0, 0)
		wglDeleteContext(p.hglrc)
		p.hglrc = 0
	}
	if p.hdc != 0 && p.hwnd != 0 {
		pReleaseDC.Call(p.hwnd, p.hdc)
		p.hdc = 0
	}
	if p.hwnd != 0 {
		unregisterWindow(p.hwnd)
		pDestroyWindow.Call(p.hwnd)
		p.hwnd = 0
	}
}

// --- helpers ---

func clientSize(hwnd uintptr) (int32, int32) {
	var r rectW
	pGetClientRect.Call(hwnd, uintptr(unsafe.Pointer(&r)))
	return r.right - r.left, r.bottom - r.top
}

func dpiForWindow(hwnd uintptr) uint32 {
	r, _, _ := pGetDpiForWindow.Call(hwnd)
	if r == 0 {
		return 96
	}
	return uint32(r)
}

func dpiForSystem() uint32 {
	r, _, _ := pGetDpiForSystem.Call()
	if r == 0 {
		return 96
	}
	return uint32(r)
}

func registerClass(hInstance uintptr) {
	classOnce.Do(func() {
		wc := wndClassExW{
			style:         csHRedraw | csVRedraw | csOwnDC,
			lpfnWndProc:   wndProcCallback,
			hInstance:     hInstance,
			lpszClassName: className,
		}
		hArrow, _, _ := pLoadCursorW.Call(0, idcArrow)
		wc.hCursor = hArrow
		wc.cbSize = uint32(unsafe.Sizeof(wc))
		pRegisterClassExW.Call(uintptr(unsafe.Pointer(&wc)))
	})
}

func loadCursors(p *platformState) {
	ld := func(id uintptr) uintptr {
		h, _, _ := pLoadCursorW.Call(0, id)
		return h
	}
	p.cursors[gui.CursorDefault] = ld(idcArrow)
	p.cursors[gui.CursorArrow] = ld(idcArrow)
	p.cursors[gui.CursorIBeam] = ld(idcIBeam)
	p.cursors[gui.CursorCrosshair] = ld(idcCross)
	p.cursors[gui.CursorPointingHand] = ld(idcHand)
	p.cursors[gui.CursorResizeEW] = ld(idcSizeWE)
	p.cursors[gui.CursorResizeNS] = ld(idcSizeNS)
	p.cursors[gui.CursorResizeNWSE] = ld(idcSizeNWSE)
	p.cursors[gui.CursorResizeNESW] = ld(idcSizeNESW)
	p.cursors[gui.CursorResizeAll] = ld(idcSizeAll)
	p.cursors[gui.CursorNotAllowed] = ld(idcNo)
}

func setClipboard(hwnd uintptr, s string) {
	u := windows.StringToUTF16(s) // NUL-terminated
	if r, _, _ := pOpenClipboard.Call(hwnd); r == 0 {
		return
	}
	defer pCloseClipboard.Call()
	pEmptyClipboard.Call()
	n := uintptr(len(u) * 2)
	h, _, _ := pGlobalAlloc.Call(gmemMoveable, n)
	if h == 0 {
		return
	}
	dst, _, _ := pGlobalLock.Call(h)
	if dst == 0 {
		return
	}
	// Copy via RtlMoveMemory to avoid a uintptr→unsafe.Pointer
	// conversion on the locked handle (go vet unsafeptr).
	pRtlMoveMemory.Call(dst, uintptr(unsafe.Pointer(&u[0])), n)
	pGlobalUnlock.Call(h)
	pSetClipboardData.Call(cfUnicodeText, h)
}

func getClipboard(hwnd uintptr) string {
	if r, _, _ := pOpenClipboard.Call(hwnd); r == 0 {
		return ""
	}
	defer pCloseClipboard.Call()
	h, _, _ := pGetClipboardData.Call(cfUnicodeText)
	if h == 0 {
		return ""
	}
	src, _, _ := pGlobalLock.Call(h)
	if src == 0 {
		return ""
	}
	defer pGlobalUnlock.Call(h)
	n, _, _ := pLstrlenW.Call(src) // uint16 count, excluding NUL
	if n == 0 {
		return ""
	}
	if n > maxClipboardChars {
		n = maxClipboardChars // cap oversized clipboard content
	}
	buf := make([]uint16, n)
	pRtlMoveMemory.Call(uintptr(unsafe.Pointer(&buf[0])), src, n*2)
	return windows.UTF16ToString(buf)
}

// --- lifecycle ---

// New creates an OpenGL 3.3 backend backed by a native Win32 window
// and a WGL context. No SDL2.
func New(w *gui.Window) (*Backend, error) {
	runtime.LockOSThread()

	pSetDpiAware.Call(dpiPerMonitorV2) // best-effort; ignore result
	hInst, _, _ := pGetModuleHandleW.Call(0)
	registerClass(hInst)

	cfg := w.Config
	title := cfg.Title
	if title == "" {
		title = "go-gui"
	}
	width := cfg.Width
	if width <= 0 {
		width = 640
	}
	height := cfg.Height
	if height <= 0 {
		height = 480
	}

	style := uintptr(wsClipSiblings | wsClipChildren)
	if cfg.FixedSize {
		style |= wsFixed
	} else {
		style |= wsOverlappedWindow
	}

	// Size the window so its client area matches the requested
	// logical size at the current system DPI.
	dpi := dpiForSystem()
	scale := float64(dpi) / 96.0
	rc := rectW{0, 0, int32(float64(width) * scale), int32(float64(height) * scale)}
	pAdjustRectDpi.Call(uintptr(unsafe.Pointer(&rc)), style, 0, 0, uintptr(dpi))
	winW := rc.right - rc.left
	winH := rc.bottom - rc.top

	hwnd, _, err := pCreateWindowExW.Call(
		0,
		uintptr(unsafe.Pointer(className)),
		uintptr(unsafe.Pointer(windows.StringToUTF16Ptr(title))),
		style,
		cwUseDefault, cwUseDefault,
		uintptr(winW), uintptr(winH),
		0, 0, hInst, 0,
	)
	if hwnd == 0 {
		return nil, fmt.Errorf("gl: CreateWindowExW: %w", err)
	}

	b := &Backend{}
	b.plat.hwnd = hwnd
	registerWindow(hwnd, b)

	hdc, _, _ := pGetDC.Call(hwnd)
	if hdc == 0 {
		b.plat.destroy()
		return nil, fmt.Errorf("gl: GetDC failed")
	}
	b.plat.hdc = hdc

	hglrc, err := createContext(hdc)
	if err != nil {
		b.plat.destroy()
		return nil, fmt.Errorf("gl: createContext: %w", err)
	}
	b.plat.hglrc = hglrc

	if err := gl.Init(); err != nil {
		b.plat.destroy()
		return nil, fmt.Errorf("gl: gl.Init: %w", err)
	}

	b.dpiScale = float32(dpiForWindow(hwnd)) / 96.0
	b.physW, b.physH = clientSize(hwnd)
	b.initCaches(cfg)

	if err := b.initGLResources(w); err != nil {
		b.Destroy()
		return nil, fmt.Errorf("gl: initGLResources: %w", err)
	}

	loadCursors(&b.plat)
	b.plat.curCursor = b.plat.cursors[gui.CursorDefault]

	w.SetClipboardFn(func(text string) { setClipboard(hwnd, text) })
	w.SetClipboardGetFn(func() string { return getClipboard(hwnd) })
	w.SetTitleFn(func(t string) {
		pSetWindowTextW.Call(hwnd, uintptr(unsafe.Pointer(windows.StringToUTF16Ptr(t))))
	})

	pShowWindow.Call(hwnd, swShow)
	pUpdateWindow.Call(hwnd)
	return b, nil
}

// Destroy releases all backend resources.
func (b *Backend) Destroy() {
	b.destroyGLResources()
	b.plat.destroy()
}

// Run starts the event loop. Blocks until the window is closed.
func (b *Backend) Run(w *gui.Window) {
	defer w.WindowCleanup()
	b.plat.w = w
	if w.Config.OnInit != nil {
		w.Config.OnInit(w)
	}
	w.SetWakeMainFn(b.plat.wake)

	rendered := true
	var msg msgW
	for {
		pumpMessages(&msg)
		if w.CloseRequested() {
			break
		}
		rendered = w.FrameFn()
		if rendered {
			b.renderFrame(w)
		}
		b.plat.setCursor(w.MouseCursorState())
		if !rendered {
			waitMessage(100)
		}
	}
}

// Run initializes the backend, runs the event loop, and cleans up on
// exit. Panics on error; call RunE for the error-returning variant.
func Run(w *gui.Window) {
	if err := RunE(w); err != nil {
		panic(fmt.Sprintf("gl: %v", err))
	}
}

// RunE initializes the backend, runs the event loop, and cleans up on
// exit. Returns an error instead of panicking.
func RunE(w *gui.Window) error {
	b, err := New(w)
	if err != nil {
		return fmt.Errorf("gl: %w", err)
	}
	defer b.Destroy()
	b.Run(w)
	return nil
}

// RunApp starts a multi-window event loop. Panics on error; call
// RunAppE for the error-returning variant.
func RunApp(app *gui.App, initialWindows ...*gui.Window) {
	if err := RunAppE(app, initialWindows...); err != nil {
		panic(fmt.Sprintf("gl: %v", err))
	}
}

// RunAppE starts a multi-window event loop. Each window is created and
// registered with app. Blocks until the last window closes.
//
//nolint:gocyclo // backend event loop
func RunAppE(app *gui.App, initialWindows ...*gui.Window) error {
	runtime.LockOSThread()

	backends := make(map[uintptr]*Backend) // hwnd → backend
	ids := make(map[uintptr]uint32)        // hwnd → app id
	var nextID uint32

	register := func(b *Backend, w *gui.Window) {
		nextID++
		ids[b.plat.hwnd] = nextID
		backends[b.plat.hwnd] = b
		app.Register(nextID, w)
	}

	open := func(w *gui.Window) error {
		b, err := New(w)
		if err != nil {
			return err
		}
		b.plat.w = w
		register(b, w)
		w.SetWakeMainFn(b.plat.wake)
		if w.Config.OnInit != nil {
			w.Config.OnInit(w)
		}
		return nil
	}

	for _, w := range initialWindows {
		if err := open(w); err != nil {
			for _, b := range backends {
				b.Destroy()
			}
			return fmt.Errorf("gl: create window: %w", err)
		}
	}

	rendered := true
	var msg msgW
	for {
		// Drain pending window opens.
	drain:
		for {
			select {
			case cfg := <-app.PendingOpen():
				if err := open(gui.NewWindow(cfg)); err != nil {
					log.Printf("gl: open window: %v", err)
				}
			default:
				break drain
			}
		}

		pumpMessages(&msg)

		// Close windows whose close was requested.
		for hwnd, b := range backends {
			w := b.plat.w
			if w == nil || !w.CloseRequested() {
				continue
			}
			w.WindowCleanup()
			b.Destroy()
			delete(backends, hwnd)
			if app.Unregister(ids[hwnd]) {
				return nil // last window closed
			}
			delete(ids, hwnd)
		}
		if len(backends) == 0 {
			return nil
		}

		// Frame + render each window.
		rendered = false
		for _, b := range backends {
			w := b.plat.w
			if w.FrameFn() {
				b.renderFrame(w)
				rendered = true
			}
			b.plat.setCursor(w.MouseCursorState())
		}

		if !rendered {
			waitMessage(100)
		}
	}
}

// --- IME (stub; WM_CHAR still yields EventChar) ---

func (n *nativePlatform) IMEStart()                   {}
func (n *nativePlatform) IMEStop()                    {}
func (n *nativePlatform) IMESetRect(_, _, _, _ int32) {}
