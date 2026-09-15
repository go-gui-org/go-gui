//go:build windows

// Package sni provides system tray support. On Windows this uses
// Shell_NotifyIconW with a message-only window for callbacks. The
// window and its message loop live on one thread owned by the tray, so
// a tray can be made from any goroutine.
package sni

import (
	"crypto/rand"
	"errors"
	"fmt"
	"runtime"
	"sync"
	"sync/atomic"
	"syscall"
	"unsafe"

	"github.com/go-gui-org/go-gui/gui"
	"github.com/go-gui-org/go-gui/gui/backend/internal/hicon"
)

// Win32 DLLs.
var (
	shell32 = syscall.NewLazyDLL("shell32.dll")
	user32  = syscall.NewLazyDLL("user32.dll")

	procShellNotifyIconW    = shell32.NewProc("Shell_NotifyIconW")
	procCreateWindowExW     = user32.NewProc("CreateWindowExW")
	procDefWindowProcW      = user32.NewProc("DefWindowProcW")
	procRegisterClassExW    = user32.NewProc("RegisterClassExW")
	procGetMessageW         = user32.NewProc("GetMessageW")
	procTranslateMessage    = user32.NewProc("TranslateMessage")
	procDispatchMessageW    = user32.NewProc("DispatchMessageW")
	procPostMessageW        = user32.NewProc("PostMessageW")
	procCreatePopupMenu     = user32.NewProc("CreatePopupMenu")
	procAppendMenuW         = user32.NewProc("AppendMenuW")
	procTrackPopupMenu      = user32.NewProc("TrackPopupMenu")
	procDestroyMenu         = user32.NewProc("DestroyMenu")
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

// --- Tray implementation ---

// maxTrayIconDim caps tray icon size: tray icons render small and
// oversized PNGs waste GDI memory.
const maxTrayIconDim = 256

// Tray manages Windows system tray entries.
type Tray struct {
	mu      sync.Mutex
	entries map[int]*entry
	nextID  int
	hwnd    uintptr
	once    sync.Once
	// initErr keeps the result of the one window init. once never runs
	// the init again, so later ensureWindow calls must read the error
	// from here, or they report success with hwnd still 0.
	initErr error
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

// ensureWindow starts the tray thread, which creates the message-only
// window and runs its message loop. Safe to call multiple times. A failed init is not
// retried: every call returns that same error. A retry would call
// RegisterClassExW again, which fails once the class exists.
func (t *Tray) ensureWindow() error {
	t.once.Do(func() {
		t.initErr = trayInitWindow(t)
	})
	return t.initErr
}

// trayInitWindow is the init ensureWindow runs. Tests replace it to
// force a failure without a real Win32 call.
var trayInitWindow = (*Tray).initWindow

// trayClassSeq numbers the window classes, so each Tray registers its
// own. A class names one window procedure, and that procedure is bound
// to one Tray. If two trays shared a class, the second tray's clicks
// would go to the first tray, and RegisterClassExW would also fail with
// ERROR_CLASS_ALREADY_EXISTS for the second tray.
var trayClassSeq atomic.Uint64

// initWindow starts the tray thread and waits until the window exists
// or the thread reports why it could not make one.
//
// Win32 gives a window's messages only to the thread that created the
// window. So the window and the loop that reads its messages must run
// on one thread. The caller's thread cannot be that thread: nothing
// says the caller pumps messages, and a goroutine that is not locked
// can move to another thread (#616).
func (t *Tray) initWindow() error {
	ready := make(chan error, 1)
	go t.messageLoop(ready)
	return <-ready
}

// messageLoop owns the tray window. It creates the window, sends the
// result to ready, then reads the window's messages until WM_QUIT.
// t.hwnd is written before the send, so the caller reads it after the
// receive without a race.
func (t *Tray) messageLoop(ready chan<- error) {
	// Not unlocked on purpose. When this goroutine returns while it is
	// still locked, Go ends the OS thread, and the window goes with it.
	runtime.LockOSThread()

	hwnd, err := t.createWindow()
	if err != nil {
		ready <- err
		return
	}
	t.hwnd = hwnd
	ready <- nil

	var m msg
	for {
		r, _, _ := procGetMessageW.Call(
			uintptr(unsafe.Pointer(&m)),
			0, 0, 0)
		if r == 0 {
			return // WM_QUIT
		}
		if r == ^uintptr(0) { //nolint:staticcheck
			return // error
		}
		procTranslateMessage.Call(uintptr(unsafe.Pointer(&m)))
		procDispatchMessageW.Call(uintptr(unsafe.Pointer(&m)))
	}
}

// createWindow registers this tray's window class and creates the
// message-only window. It must run on the thread that reads the
// window's messages.
func (t *Tray) createWindow() (uintptr, error) {
	className := fmt.Sprintf("go-gui-tray-window-%d", trayClassSeq.Add(1))
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
		return 0, errors.New("sni: RegisterClassExW failed")
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
		return 0, errors.New("sni: CreateWindowExW failed")
	}
	return hwnd, nil
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
		hIcon, iconErr = hicon.FromPNG(cfg.IconPNG, maxTrayIconDim)
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
			hicon.Destroy(hIcon)
		}
		if hMenu != 0 {
			procDestroyMenu.Call(hMenu)
		}
		return 0, errors.New("sni: Shell_NotifyIcon(NIM_ADD) failed")
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
		if newIcon, err := hicon.FromPNG(cfg.IconPNG, maxTrayIconDim); err == nil {
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
		hicon.Destroy(oldIcon)
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
		hicon.Destroy(e.hIcon)
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
