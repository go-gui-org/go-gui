//go:build windows

// Package sni provides system tray support. On Windows this uses
// Shell_NotifyIconW with a message-only window for callbacks.
package sni

import (
	"bytes"
	"crypto/rand"
	"fmt"
	"image/png"
	"sync"
	"syscall"
	"unsafe"

	"github.com/go-gui-org/go-gui/gui"
)

// Win32 DLLs.
var (
	shell32 = syscall.NewLazyDLL("shell32.dll")
	user32  = syscall.NewLazyDLL("user32.dll")
	gdi32   = syscall.NewLazyDLL("gdi32.dll")

	procShellNotifyIconW    = shell32.NewProc("Shell_NotifyIconW")
	procCreateWindowExW     = user32.NewProc("CreateWindowExW")
	procDefWindowProcW      = user32.NewProc("DefWindowProcW")
	procRegisterClassExW    = user32.NewProc("RegisterClassExW")
	procUnregisterClassW    = user32.NewProc("UnregisterClassW")
	procGetMessageW         = user32.NewProc("GetMessageW")
	procTranslateMessage    = user32.NewProc("TranslateMessage")
	procDispatchMessageW    = user32.NewProc("DispatchMessageW")
	procPostMessageW        = user32.NewProc("PostMessageW")
	procDestroyWindow       = user32.NewProc("DestroyWindow")
	procCreatePopupMenu     = user32.NewProc("CreatePopupMenu")
	procAppendMenuW         = user32.NewProc("AppendMenuW")
	procTrackPopupMenu      = user32.NewProc("TrackPopupMenu")
	procDestroyMenu         = user32.NewProc("DestroyMenu")
	procDestroyIcon         = user32.NewProc("DestroyIcon")
	procGetDC               = user32.NewProc("GetDC")
	procReleaseDC           = user32.NewProc("ReleaseDC")
	procCreateDIBSection    = gdi32.NewProc("CreateDIBSection")
	procDeleteObject        = gdi32.NewProc("DeleteObject")
	procCreateIconIndirect  = user32.NewProc("CreateIconIndirect")
	procGetCursorPos        = user32.NewProc("GetCursorPos")
	procSetForegroundWindow = user32.NewProc("SetForegroundWindow")
)

// Shell_NotifyIcon constants.
const (
	nimAdd        = 0x00000000
	nimModify     = 0x00000001
	nimDelete     = 0x00000002
	nimSetVersion = 0x00000004
	nifMessage    = 0x00000001
	nifIcon       = 0x00000002
	nifTip        = 0x00000004
	nifGuid       = 0x00000020
	nisHidden     = 0x00000001
	notifyIconV4  = 4
)

// Window message constants.
const (
	wmAppTray   = 0x8000 + 100
	wmRButtonUp = 0x0205
	wmCommand   = 0x0111
	wmDestroy   = 0x0002
	wmClose     = 0x0010
)

// Window class / style constants.
const (
	hwndMessage = ^uintptr(2) // HWND_MESSAGE = -3
	classStyle  = 0
)

// Menu flags.
const (
	mfString    = 0x00000000
	mfSeparator = 0x00000800
	mfDisabled  = 0x00000002
	mfChecked   = 0x00000008
	mfPopup     = 0x00000010
)

// TrackPopupMenu flags.
const (
	tpmRightButton = 0x0002
	tpmBottomAlign = 0x0020
)

// DIB constants.
const (
	biRGB        = 0
	dibRGBColors = 0
)

// notifyIconDataW mirrors the Win32 NOTIFYICONDATAW structure.
type notifyIconDataW struct {
	cbSize           uint32
	hWnd             uintptr
	uID              uint32
	uFlags           uint32
	uCallbackMessage uint32
	hIcon            uintptr
	szTip            [128]uint16
	dwState          uint32
	dwStateMask      uint32
	szInfo           [256]uint16
	uVersion         uint32
	szInfoTitle      [64]uint16
	dwInfoFlags      uint32
	guidItem         [16]byte
	hBalloonIcon     uintptr
}

type point struct {
	x int32
	y int32
}

type msg struct {
	hwnd    uintptr
	message uint32
	wParam  uintptr
	lParam  uintptr
	time    uint32
	pt      point
}

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

type bitmapV5Header struct {
	bV5Size          uint32
	bV5Width         int32
	bV5Height        int32
	bV5Planes        uint16
	bV5BitCount      uint16
	bV5Compression   uint32
	bV5SizeImage     uint32
	bV5XPelsPerMeter int32
	bV5YPelsPerMeter int32
	bV5ClrUsed       uint32
	bV5ClrImportant  uint32
	bV5RedMask       uint32
	bV5GreenMask     uint32
	bV5BlueMask      uint32
	bV5AlphaMask     uint32
	bV5CSType        uint32
	bV5Endpoints     [36]byte
	bV5GammaRed      uint32
	bV5GammaGreen    uint32
	bV5GammaBlue     uint32
	bV5Intent        uint32
	bV5ProfileData   uint32
	bV5ProfileSize   uint32
	bV5Reserved      uint32
}

type iconInfo struct {
	fIcon    int32
	xHotspot uint32
	yHotspot uint32
	hbmMask  uintptr
	hbmColor uintptr
}

// --- Tray implementation ---

// Tray manages Windows system tray entries.
type Tray struct {
	mu      sync.Mutex
	entries map[int]*entry
	nextID  int
	hwnd    uintptr
	done    chan struct{}
	once    sync.Once
}

type entry struct {
	id        int
	tooltip   string
	iconPNG   []byte
	menuNodes []menuNode
	actionCb  func(string)
	hIcon     uintptr
	hMenu     uintptr
}

// menuNode mirrors the flat-menu structure from sni_linux.go.
type menuNode struct {
	actionID   string
	label      string
	separator  bool
	disabled   bool
	checked    bool
	childStart int
	childCount int
}

// ensureWindow creates the message-only window and starts the
// message pump. Safe to call multiple times.
func (t *Tray) ensureWindow() error {
	var err error
	t.once.Do(func() {
		err = t.initWindow()
	})
	return err
}

func (t *Tray) initWindow() error {
	className := "go-gui-tray-window"
	classNameW, _ := syscall.UTF16PtrFromString(className)

	// Register window class. Use a callback via syscall for the
	// window procedure — we need to route messages to the Tray.
	wndProcCB := syscall.NewCallback(t.wndProc)

	wc := wndClassExW{
		cbSize:        uint32(unsafe.Sizeof(wndClassExW{})),
		style:         classStyle,
		lpfnWndProc:   wndProcCB,
		hInstance:     0,
		lpszClassName: classNameW,
	}

	atom, _, _ := procRegisterClassExW.Call(
		uintptr(unsafe.Pointer(&wc)))
	if atom == 0 {
		return fmt.Errorf("sni: RegisterClassExW failed")
	}

	hwnd, _, _ := procCreateWindowExW.Call(
		0,
		uintptr(unsafe.Pointer(classNameW)),
		0,          // window name
		0,          // style
		0, 0, 0, 0, // x, y, w, h
		hwndMessage, // parent = HWND_MESSAGE
		0,           // menu
		0,           // hInstance
		0,           // lpParam
	)
	if hwnd == 0 {
		return fmt.Errorf("sni: CreateWindowExW failed")
	}
	t.hwnd = hwnd
	t.done = make(chan struct{})

	// Start message pump.
	go t.messageLoop()

	return nil
}

func (t *Tray) messageLoop() {
	var m msg
	for {
		r, _, _ := procGetMessageW.Call(
			uintptr(unsafe.Pointer(&m)),
			0, 0, 0)
		if r == 0 {
			break // WM_QUIT
		}
		if r == ^uintptr(0) { //nolint:staticcheck
			break // error
		}
		procTranslateMessage.Call(uintptr(unsafe.Pointer(&m)))
		procDispatchMessageW.Call(uintptr(unsafe.Pointer(&m)))
	}
	close(t.done)
}

func (t *Tray) wndProc(hwnd uintptr, msg uint32, wParam, lParam uintptr) uintptr {
	switch msg {
	case wmAppTray:
		// lParam holds the event type from Shell_NotifyIcon.
		switch lParam {
		case wmRButtonUp:
			t.showContextMenu(wParam) // wParam = icon ID
		default:
			// Left-click — fire action callback with empty
			// ID (default action).
			t.fireAction(wParam, "")
		}
		return 0

	case wmCommand:
		// Menu item clicked. Low word of wParam = menu item ID.
		menuID := uint32(wParam & 0xFFFF)
		t.fireMenuAction(menuID)
		return 0

	case wmDestroy, wmClose:
		procPostMessageW.Call(hwnd, 0x0012, 0, 0) // WM_QUIT
		return 0
	}

	r, _, _ := procDefWindowProcW.Call(hwnd, uintptr(msg), wParam, lParam)
	return r
}

func (t *Tray) fireAction(iconID uintptr, actionID string) {
	t.mu.Lock()
	var cb func(string)
	// Icon IDs are 1-based in the Win32 impl (uID starts at 1).
	for _, e := range t.entries {
		if uintptr(e.id) == iconID {
			cb = e.actionCb
			break
		}
	}
	t.mu.Unlock()
	if cb != nil {
		cb(actionID)
	}
}

func (t *Tray) fireMenuAction(menuID uint32) {
	t.mu.Lock()
	var cb func(string)
	var actionID string
	for _, e := range t.entries {
		// Menu IDs are offset by (entryID * 1000) to avoid
		// collisions across tray entries.
		base := uint32(e.id * 1000)
		if menuID >= base && menuID < base+1000 {
			idx := int(menuID - base)
			if idx >= 0 && idx < len(e.menuNodes) {
				actionID = e.menuNodes[idx].actionID
				cb = e.actionCb
			}
			break
		}
	}
	t.mu.Unlock()
	if cb != nil && actionID != "" {
		cb(actionID)
	}
}

func (t *Tray) showContextMenu(iconID uintptr) {
	t.mu.Lock()
	var e *entry
	for _, ent := range t.entries {
		if uintptr(ent.id) == iconID {
			e = ent
			break
		}
	}
	if e == nil || e.hMenu == 0 {
		t.mu.Unlock()
		return
	}
	hMenu := e.hMenu
	t.mu.Unlock()

	// Get cursor position.
	var pt point
	procGetCursorPos.Call(uintptr(unsafe.Pointer(&pt)))

	// Must set foreground window before TrackPopupMenu so
	// the menu dismisses properly when clicking elsewhere.
	procSetForegroundWindow.Call(t.hwnd)

	procTrackPopupMenu.Call(
		hMenu,
		uintptr(tpmRightButton|tpmBottomAlign),
		uintptr(pt.x), uintptr(pt.y),
		0, t.hwnd, 0)
}

// Create registers a new system tray icon with an optional
// context menu.
func (t *Tray) Create(
	cfg gui.SystemTrayCfg, actionCb func(string),
) (int, error) {
	if err := t.ensureWindow(); err != nil {
		return 0, err
	}

	t.mu.Lock()
	if t.entries == nil {
		t.entries = make(map[int]*entry)
	}
	t.nextID++
	id := t.nextID
	t.mu.Unlock()

	// Convert PNG to HICON.
	var hIcon uintptr
	if len(cfg.IconPNG) > 0 {
		var iconErr error
		hIcon, iconErr = pngToHICON(cfg.IconPNG)
		if iconErr != nil {
			return 0, fmt.Errorf("sni: icon: %w", iconErr)
		}
	}

	// Build popup menu.
	hMenu := buildPopupMenu(cfg.Menu, id)

	// Create unique GUID for this icon (prevents shell from
	// reusing old icons across app restarts).
	var guid [16]byte
	_, _ = rand.Read(guid[:])

	// Build tooltip as UTF-16.
	tip, _ := syscall.UTF16FromString(cfg.Tooltip)

	nid := notifyIconDataW{
		cbSize:           uint32(unsafe.Sizeof(notifyIconDataW{})),
		hWnd:             t.hwnd,
		uID:              uint32(id),
		uFlags:           nifMessage | nifGuid | nifTip,
		uCallbackMessage: wmAppTray,
		hIcon:            hIcon,
		guidItem:         guid,
	}
	copy(nid.szTip[:], tip)

	if hIcon != 0 {
		nid.uFlags |= nifIcon
	}

	// Set version to NOTIFYICON_VERSION_4 (Vista+) for GUID
	// support and better behavior.
	nidVersion := nid
	nidVersion.uFlags = 0
	nidVersion.uVersion = notifyIconV4
	procShellNotifyIconW.Call(nimSetVersion,
		uintptr(unsafe.Pointer(&nidVersion)))

	r, _, _ := procShellNotifyIconW.Call(nimAdd,
		uintptr(unsafe.Pointer(&nid)))
	if r == 0 {
		if hIcon != 0 {
			procDestroyIcon.Call(hIcon)
		}
		if hMenu != 0 {
			procDestroyMenu.Call(hMenu)
		}
		return 0, fmt.Errorf("sni: Shell_NotifyIcon(NIM_ADD) failed")
	}

	t.mu.Lock()
	t.entries[id] = &entry{
		id:        id,
		tooltip:   cfg.Tooltip,
		iconPNG:   cfg.IconPNG,
		menuNodes: buildMenuNodes(cfg.Menu),
		actionCb:  actionCb,
		hIcon:     hIcon,
		hMenu:     hMenu,
	}
	t.mu.Unlock()

	return id, nil
}

// Update replaces the icon, tooltip, and menu for an existing
// tray entry.
func (t *Tray) Update(id int, cfg gui.SystemTrayCfg) {
	t.mu.Lock()
	e, ok := t.entries[id]
	if !ok {
		t.mu.Unlock()
		return
	}

	e.tooltip = cfg.Tooltip
	if cfg.OnAction != nil {
		e.actionCb = cfg.OnAction
	}

	var hIcon uintptr
	if len(cfg.IconPNG) > 0 {
		if newIcon, err := pngToHICON(cfg.IconPNG); err == nil {
			hIcon = newIcon
		}
	}

	oldIcon := e.hIcon
	if hIcon != 0 {
		e.hIcon = hIcon
	}
	e.iconPNG = cfg.IconPNG

	e.menuNodes = buildMenuNodes(cfg.Menu)
	oldMenu := e.hMenu
	e.hMenu = buildPopupMenu(cfg.Menu, id)

	tip, _ := syscall.UTF16FromString(cfg.Tooltip)

	nid := notifyIconDataW{
		cbSize: uint32(unsafe.Sizeof(notifyIconDataW{})),
		hWnd:   t.hwnd,
		uID:    uint32(id),
		uFlags: nifTip,
	}
	copy(nid.szTip[:], tip)

	if hIcon != 0 {
		nid.uFlags |= nifIcon
		nid.hIcon = hIcon
	}

	t.mu.Unlock()

	procShellNotifyIconW.Call(nimModify,
		uintptr(unsafe.Pointer(&nid)))

	// Clean up old GDI resources.
	if oldIcon != 0 {
		procDestroyIcon.Call(oldIcon)
	}
	if oldMenu != 0 {
		procDestroyMenu.Call(oldMenu)
	}
}

// Remove deletes a tray entry and cleans up resources.
func (t *Tray) Remove(id int) {
	t.mu.Lock()
	e, ok := t.entries[id]
	if !ok {
		t.mu.Unlock()
		return
	}
	delete(t.entries, id)
	t.mu.Unlock()

	// Remove from shell.
	nid := notifyIconDataW{
		cbSize: uint32(unsafe.Sizeof(notifyIconDataW{})),
		hWnd:   t.hwnd,
		uID:    uint32(id),
	}
	procShellNotifyIconW.Call(nimDelete,
		uintptr(unsafe.Pointer(&nid)))

	if e.hIcon != 0 {
		procDestroyIcon.Call(e.hIcon)
	}
	if e.hMenu != 0 {
		procDestroyMenu.Call(e.hMenu)
	}
}

// --- Popup menu construction ---

func buildPopupMenu(items []gui.NativeMenuItemCfg, entryID int) uintptr {
	if len(items) == 0 {
		return 0
	}
	hMenu, _, _ := procCreatePopupMenu.Call()
	if hMenu == 0 {
		return 0
	}
	base := uint32(entryID * 1000)
	appendMenuItems(hMenu, items, base, 0)
	return hMenu
}

func appendMenuItems(
	hMenu uintptr,
	items []gui.NativeMenuItemCfg,
	baseID uint32,
	idx int,
) int {
	for _, item := range items {
		id := baseID + uint32(idx)
		idx++

		if item.Separator {
			procAppendMenuW.Call(hMenu,
				uintptr(mfSeparator), 0, 0)
			continue
		}

		var flags uintptr = mfString
		if item.Disabled {
			flags |= mfDisabled
		}
		if item.Checked {
			flags |= mfChecked
		}

		labelW, _ := syscall.UTF16PtrFromString(item.Text)

		if len(item.Submenu) > 0 {
			subMenu, _, _ := procCreatePopupMenu.Call()
			if subMenu != 0 {
				appendMenuItems(subMenu, item.Submenu, baseID, idx)
				procAppendMenuW.Call(hMenu,
					uintptr(mfPopup|flags),
					subMenu,
					uintptr(unsafe.Pointer(labelW)))
			}
		} else {
			procAppendMenuW.Call(hMenu,
				flags,
				uintptr(id),
				uintptr(unsafe.Pointer(labelW)))
		}
	}
	return idx
}

// buildMenuNodes converts NativeMenuItemCfg to flat menuNode slice
// (same layout as sni_linux.go for consistency).
func buildMenuNodes(
	items []gui.NativeMenuItemCfg,
) []menuNode {
	nodes := []menuNode{{}} // root node 0
	appendMenuNodeItems(items, &nodes)
	nodes[0].childStart = 1
	nodes[0].childCount = len(items)
	return nodes
}

func appendMenuNodeItems(
	items []gui.NativeMenuItemCfg,
	nodes *[]menuNode,
) {
	baseIdx := len(*nodes)
	for range items {
		*nodes = append(*nodes, menuNode{})
	}

	for i, item := range items {
		idx := baseIdx + i
		n := &(*nodes)[idx]
		n.actionID = item.ID
		n.label = item.Text
		n.separator = item.Separator
		n.disabled = item.Disabled
		n.checked = item.Checked

		if len(item.Submenu) > 0 {
			childStart := len(*nodes)
			appendMenuNodeItems(item.Submenu, nodes)
			n = &(*nodes)[idx] // re-derive after realloc
			n.childStart = childStart
			n.childCount = len(item.Submenu)
		}
	}
}

// --- PNG → HICON conversion ---

// maxIconDim rejects oversized icons to prevent GDI memory blowup.
const maxIconDim = 256

func pngToHICON(pngData []byte) (uintptr, error) {
	img, err := png.Decode(bytes.NewReader(pngData))
	if err != nil {
		return 0, fmt.Errorf("png decode: %w", err)
	}
	bounds := img.Bounds()
	w := bounds.Dx()
	h := bounds.Dy()
	if w <= 0 || h <= 0 {
		return 0, fmt.Errorf("empty icon")
	}
	if w > maxIconDim || h > maxIconDim {
		return 0, fmt.Errorf("icon too large: %dx%d (max %d)",
			w, h, maxIconDim)
	}

	// Build 32-bit BGRA pixel buffer (GDI expects BGR order).
	stride := w * 4
	pixels := make([]byte, stride*h)
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			r, g, b, a := img.At(x+bounds.Min.X, y+bounds.Min.Y).RGBA()
			off := y*stride + x*4
			pixels[off+0] = byte(b >> 8)
			pixels[off+1] = byte(g >> 8)
			pixels[off+2] = byte(r >> 8)
			pixels[off+3] = byte(a >> 8)
		}
	}

	// Get screen DC for DIB creation.
	hdc, _, _ := procGetDC.Call(0)
	if hdc == 0 {
		return 0, fmt.Errorf("GetDC failed")
	}
	defer procReleaseDC.Call(0, hdc)

	// Create 32-bit DIB section.
	bmi := bitmapV5Header{
		bV5Size:        uint32(unsafe.Sizeof(bitmapV5Header{})),
		bV5Width:       int32(w),
		bV5Height:      int32(-h), // negative = top-down
		bV5Planes:      1,
		bV5BitCount:    32,
		bV5Compression: biRGB,
		bV5AlphaMask:   0xFF000000,
		bV5RedMask:     0x00FF0000,
		bV5GreenMask:   0x0000FF00,
		bV5BlueMask:    0x000000FF,
	}

	var bits unsafe.Pointer
	hbm, _, _ := procCreateDIBSection.Call(
		hdc,
		uintptr(unsafe.Pointer(&bmi)),
		dibRGBColors,
		uintptr(unsafe.Pointer(&bits)),
		0, 0)
	if hbm == 0 {
		return 0, fmt.Errorf("CreateDIBSection failed")
	}
	defer procDeleteObject.Call(hbm)

	// Copy pixels into the DIB section.
	dst := unsafe.Slice((*byte)(bits), len(pixels))
	copy(dst, pixels)

	// Create icon from bitmap (nil mask = use alpha channel).
	ii := iconInfo{
		fIcon:    1, // TRUE
		xHotspot: 0,
		yHotspot: 0,
		hbmColor: hbm,
		hbmMask:  0, // NULL — alpha in color bitmap
	}
	hIcon, _, _ := procCreateIconIndirect.Call(
		uintptr(unsafe.Pointer(&ii)))
	if hIcon == 0 {
		return 0, fmt.Errorf("CreateIconIndirect failed")
	}

	return hIcon, nil
}
