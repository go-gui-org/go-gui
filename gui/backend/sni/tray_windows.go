//go:build windows

// Package sni provides system tray support. On Windows this uses
// Shell_NotifyIconW with a message-only window for callbacks. The
// window and its message loop live on one thread owned by the tray, so
// a tray can be made from any goroutine.
package sni

import (
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
	nisHidden     = 0x00000001
	notifyIconV4  = 4
)

// Window message constants.
const (
	wmAppTray = 0x8000 + 100
	wmCommand = 0x0111
	wmDestroy = 0x0002
	wmClose   = 0x0010
)

// Tray callback events sent with NOTIFYICON_VERSION_4. Version 4 also
// sends raw mouse messages (WM_MOUSEMOVE, WM_LBUTTONUP, ...); the tray
// ignores them, because these three already cover click, keyboard and
// menu input.
const (
	ninSelect     = 0x0400 // NIN_SELECT: the icon was selected
	ninKeySelect  = 0x0401 // NIN_KEYSELECT: selected with the keyboard
	wmContextMenu = 0x007B // menu asked for, by mouse or keyboard
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
	// trackMenu, when set, replaces the TrackPopupMenu call so a test
	// can see the menu request without a modal menu loop blocking the
	// tray thread. Read and written under mu.
	trackMenu func(hMenu uintptr, x, y int32)
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

// shellNotifyIcon sends one Shell_NotifyIconW message and reports
// whether the shell accepted it. Tests replace it to see every call
// without a notification area.
var shellNotifyIcon = func(message uint32, nid *notifyIconDataW) bool {
	r, _, _ := procShellNotifyIconW.Call(uintptr(message),
		uintptr(unsafe.Pointer(nid)))
	return r != 0
}

// iconFromPNG and destroyIcon make and free icon handles. Tests replace
// them with fake handles to check which handle is freed, and when.
var (
	iconFromPNG = hicon.FromPNG
	destroyIcon = hicon.Destroy
)

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
		// Create sets NOTIFYICON_VERSION_4, so the message uses that
		// layout. The old layout (icon ID in wParam, message in lParam)
		// never arrives (#617).
		event, iconID, x, y := decodeTrayCallback(wParam, lParam)
		switch event {
		case ninSelect, ninKeySelect:
			// Default action: empty action ID.
			t.fireAction(uintptr(iconID), "")
		case wmContextMenu:
			t.showContextMenu(uintptr(iconID), x, y)
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

// decodeTrayCallback splits a NOTIFYICON_VERSION_4 tray callback.
// lParam holds the event in its low word and the icon ID (uID) in its
// high word. wParam holds the anchor position: X in the low word, Y in
// the high word. The anchor is the click point, or the icon for
// keyboard input. X and Y are signed, as GET_X_LPARAM and GET_Y_LPARAM
// read them, because a monitor left of or above the primary one has
// negative coordinates. Only the low 32 bits carry data.
func decodeTrayCallback(
	wParam, lParam uintptr,
) (event, iconID uint16, x, y int32) {
	event = uint16(lParam)
	iconID = uint16(lParam >> 16)
	x = int32(int16(uint16(wParam)))
	y = int32(int16(uint16(wParam >> 16)))
	return event, iconID, x, y
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

// showContextMenu opens the entry's menu at (x, y), the anchor the shell
// sent. The mouse cursor is not used: when the menu was opened from the
// keyboard, the cursor can be anywhere on screen.
func (t *Tray) showContextMenu(iconID uintptr, x, y int32) {
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
	trackMenu := t.trackMenu
	t.mu.Unlock()

	if trackMenu != nil {
		trackMenu(hMenu, x, y)
		return
	}

	// Must set foreground window before TrackPopupMenu so
	// the menu dismisses properly when clicking elsewhere.
	procSetForegroundWindow.Call(t.hwnd)

	procTrackPopupMenu.Call(
		hMenu,
		uintptr(tpmRightButton|tpmBottomAlign),
		uintptr(x), uintptr(y),
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
		hIcon, iconErr = iconFromPNG(cfg.IconPNG, maxTrayIconDim)
		if iconErr != nil {
			return 0, fmt.Errorf("sni: icon: %w", iconErr)
		}
	}

	// Build popup menu.
	hMenu := buildPopupMenu(cfg.Menu, id)

	// Build tooltip as UTF-16.
	tip, _ := syscall.UTF16FromString(cfg.Tooltip)

	// The icon is named by hWnd + uID in every call, never by GUID. The
	// shell matches an icon added with NIF_GUID by its GUID only, so
	// NIM_MODIFY and NIM_DELETE by uID would miss it (#619). A GUID
	// would also tie the icon to the path of the executable: NIM_ADD
	// fails after the executable moves, and a second instance of the
	// same executable cannot add its icon.
	nid := notifyIconDataW{
		cbSize:           uint32(unsafe.Sizeof(notifyIconDataW{})),
		hWnd:             t.hwnd,
		uID:              uint32(id),
		uFlags:           nifMessage | nifTip,
		uCallbackMessage: wmAppTray,
		hIcon:            hIcon,
	}
	copy(nid.szTip[:], tip)

	if hIcon != 0 {
		nid.uFlags |= nifIcon
	}

	// freeHandles releases the icon and menu when Create fails.
	freeHandles := func() {
		if hIcon != 0 {
			destroyIcon(hIcon)
		}
		if hMenu != 0 {
			procDestroyMenu.Call(hMenu)
		}
	}

	if !shellNotifyIcon(nimAdd, &nid) {
		freeHandles()
		return 0, errors.New("sni: Shell_NotifyIcon(NIM_ADD) failed")
	}

	// Set NOTIFYICON_VERSION_4. It applies to an icon that exists, so it
	// must come after NIM_ADD; sent before, it had no icon to act on and
	// its failure went unseen (#617). wndProc decodes only the version 4
	// layout, so if the shell refuses, the icon's clicks would be decoded
	// wrong: remove the icon and fail instead.
	nidVersion := nid
	nidVersion.uFlags = 0
	nidVersion.uVersion = notifyIconV4
	if !shellNotifyIcon(nimSetVersion, &nidVersion) {
		nidDelete := nid
		nidDelete.uFlags = 0
		shellNotifyIcon(nimDelete, &nidDelete)
		freeHandles()
		return 0, errors.New("sni: Shell_NotifyIcon(NIM_SETVERSION) failed")
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
		if newIcon, err := iconFromPNG(cfg.IconPNG, maxTrayIconDim); err == nil {
			hIcon = newIcon
		}
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

	// mu is not held across the shell call: the tray thread takes mu to
	// handle clicks, and it must not wait on a call that waits on the
	// shell.
	accepted := shellNotifyIcon(nimModify, &nid)

	// The shell keeps drawing the icon it last accepted, so the entry
	// swaps to the new handle, and frees the old one, only once the
	// shell has accepted it. If the shell refused, or Remove took the
	// entry during the call (Remove frees the icon the entry held), the
	// new handle is the one nothing uses. With no new icon, NIF_ICON was
	// not sent and the shell keeps the entry's icon, so nothing is freed.
	var freeIcon uintptr
	if hIcon != 0 {
		t.mu.Lock()
		if accepted && t.entries[id] == e {
			freeIcon, e.hIcon = e.hIcon, hIcon
		} else {
			freeIcon = hIcon
		}
		t.mu.Unlock()
	}
	if freeIcon != 0 {
		destroyIcon(freeIcon)
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

	// Remove from shell. The result is not checked: the entry is already
	// gone and nothing later would free its handles, so they are freed
	// either way. An icon the shell kept goes when the tray window does.
	nid := notifyIconDataW{
		cbSize: uint32(unsafe.Sizeof(notifyIconDataW{})),
		hWnd:   t.hwnd,
		uID:    uint32(id),
	}
	shellNotifyIcon(nimDelete, &nid)

	if e.hIcon != 0 {
		destroyIcon(e.hIcon)
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
