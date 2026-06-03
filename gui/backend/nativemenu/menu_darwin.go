//go:build darwin && !ios

// Package nativemenu provides native macOS menubar and system tray.
package nativemenu

/*
#cgo CFLAGS: -fobjc-arc
#cgo LDFLAGS: -framework AppKit
#include "menu_darwin.h"
#include <stdlib.h>
*/
import "C"

import (
	"sync"
	"unsafe"

	"github.com/go-gui-org/go-gui/gui"
)

// Callback registries — set by Go, invoked from ObjC.
var (
	mu              sync.Mutex
	menubarActionCb func(string)
	trayActionCbs   = map[int]func(string){}
)

//export goNativeMenuAction
func goNativeMenuAction(cID *C.char) {
	id := C.GoString(cID)
	mu.Lock()
	cb := menubarActionCb
	mu.Unlock()
	if cb != nil {
		cb(id)
	}
}

//export goNativeTrayAction
func goNativeTrayAction(trayID C.int, cID *C.char) {
	id := C.GoString(cID)
	mu.Lock()
	cb := trayActionCbs[int(trayID)]
	mu.Unlock()
	if cb != nil {
		cb(id)
	}
}

// flatten converts a tree of NativeMenuItemCfg into a flat
// contiguous slice. Returns the per-menu descriptors and the
// full flat array.
func flatten(
	menus []gui.NativeMenuCfg,
) (menuDescs []C.NativeMenuItemC, allItems []C.NativeMenuItemC, cleanup func()) {
	var freeList []*C.char

	cstr := func(s string) *C.char {
		if s == "" {
			return nil
		}
		p := C.CString(s)
		freeList = append(freeList, p)
		return p
	}

	// First pass: count total items.
	var totalItems int
	for i := range menus {
		totalItems += countItems(menus[i].Items)
	}

	allItems = make([]C.NativeMenuItemC, 0, totalItems)
	menuDescs = make([]C.NativeMenuItemC, len(menus))

	for i := range menus {
		start := len(allItems)
		flattenItems(menus[i].Items, &allItems, cstr)
		topCount := len(menus[i].Items)
		menuDescs[i] = C.NativeMenuItemC{
			text:       cstr(menus[i].Title),
			childStart: C.int(start),
			childCount: C.int(topCount),
		}
	}

	cleanup = func() {
		for _, p := range freeList {
			C.free(unsafe.Pointer(p))
		}
	}
	return
}

func countItems(items []gui.NativeMenuItemCfg) int {
	n := len(items)
	for i := range items {
		n += countItems(items[i].Submenu)
	}
	return n
}

func flattenItems(
	items []gui.NativeMenuItemCfg,
	out *[]C.NativeMenuItemC,
	cstr func(string) *C.char,
) {
	baseIdx := len(*out)
	// Reserve slots for this level.
	for range items {
		*out = append(*out, C.NativeMenuItemC{})
	}

	for i, item := range items {
		ci := &(*out)[baseIdx+i]
		ci.id = cstr(item.ID)
		ci.text = cstr(item.Text)
		if item.Separator {
			ci.separator = 1
		}
		if item.Disabled {
			ci.disabled = 1
		}
		if item.Checked {
			ci.checked = 1
		}
		ci.shortcutChar, ci.shortcutMods = encodeShortcut(
			item.Shortcut)

		if len(item.Submenu) > 0 {
			childStart := len(*out)
			flattenItems(item.Submenu, out, cstr)
			ci = &(*out)[baseIdx+i] // re-derive after append
			ci.childStart = C.int(childStart)
			ci.childCount = C.int(len(item.Submenu))
		}
	}
}

func encodeShortcut(s gui.Shortcut) (C.char, C.int) {
	if !s.IsSet() {
		return 0, 0
	}
	var ch byte
	k := s.Key
	switch {
	case k >= gui.KeyA && k <= gui.KeyZ:
		ch = byte('A' + (k - gui.KeyA))
	case k >= gui.Key0 && k <= gui.Key9:
		ch = byte('0' + (k - gui.Key0))
	default:
		return 0, 0
	}
	var mods int
	if s.Modifiers.Has(gui.ModSuper) {
		mods |= 1
	}
	if s.Modifiers.Has(gui.ModShift) {
		mods |= 2
	}
	if s.Modifiers.Has(gui.ModAlt) {
		mods |= 4
	}
	if s.Modifiers.Has(gui.ModCtrl) {
		mods |= 8
	}
	return C.char(ch), C.int(mods)
}

// flattenTrayItems converts tray menu items into a flat C array.
func flattenTrayItems(
	items []gui.NativeMenuItemCfg,
) (cItems []C.NativeMenuItemC, cleanup func()) {
	var freeList []*C.char

	cstr := func(s string) *C.char {
		if s == "" {
			return nil
		}
		p := C.CString(s)
		freeList = append(freeList, p)
		return p
	}

	cItems = make([]C.NativeMenuItemC, 0,
		countItems(items))
	flattenItems(items, &cItems, cstr)

	cleanup = func() {
		for _, p := range freeList {
			C.free(unsafe.Pointer(p))
		}
	}
	return
}

// SetMenubar installs a native macOS menubar.
func SetMenubar(
	cfg gui.NativeMenubarCfg, actionCb func(string),
) {
	mu.Lock()
	menubarActionCb = actionCb
	mu.Unlock()

	menuDescs, allItems, cleanup := flatten(cfg.Menus)
	defer cleanup()

	cAppName := C.CString(cfg.AppName)
	defer C.free(unsafe.Pointer(cAppName))

	var cAboutID *C.char
	if cfg.AboutActionID != "" {
		cAboutID = C.CString(cfg.AboutActionID)
		defer C.free(unsafe.Pointer(cAboutID))
	}

	var menusPtr *C.NativeMenuItemC
	if len(menuDescs) > 0 {
		menusPtr = &menuDescs[0]
	}
	var itemsPtr *C.NativeMenuItemC
	if len(allItems) > 0 {
		itemsPtr = &allItems[0]
	}

	inclEdit := C.int(0)
	if cfg.IncludeEditMenu {
		inclEdit = 1
	}

	suppSys := C.int(0)
	if cfg.SuppressSystemEditItems {
		suppSys = 1
	}

	C.nativemenuSetMenubar(cAppName,
		menusPtr, C.int(len(menuDescs)),
		itemsPtr, C.int(len(allItems)),
		inclEdit, suppSys, cAboutID)
}

// ClearMenubar removes the native menubar.
func ClearMenubar() {
	mu.Lock()
	menubarActionCb = nil
	mu.Unlock()
	C.nativemenuClearMenubar()
}

// CreateSystemTray creates a tray icon with menu.
func CreateSystemTray(
	cfg gui.SystemTrayCfg, actionCb func(string),
) (int, error) {
	cItems, cleanup := flattenTrayItems(cfg.Menu)
	defer cleanup()

	var iconPtr unsafe.Pointer
	iconLen := C.int(0)
	if len(cfg.IconPNG) > 0 {
		iconPtr = unsafe.Pointer(&cfg.IconPNG[0])
		iconLen = C.int(len(cfg.IconPNG))
	}

	var cTooltip *C.char
	if cfg.Tooltip != "" {
		cTooltip = C.CString(cfg.Tooltip)
		defer C.free(unsafe.Pointer(cTooltip))
	}

	var itemsPtr *C.NativeMenuItemC
	if len(cItems) > 0 {
		itemsPtr = &cItems[0]
	}

	trayID := int(C.nativemenuCreateTray(
		iconPtr, iconLen,
		cTooltip,
		itemsPtr, C.int(len(cItems))))

	mu.Lock()
	trayActionCbs[trayID] = actionCb
	mu.Unlock()

	return trayID, nil
}

// UpdateSystemTray updates an existing tray entry.
func UpdateSystemTray(id int, cfg gui.SystemTrayCfg) {
	cItems, cleanup := flattenTrayItems(cfg.Menu)
	defer cleanup()

	var iconPtr unsafe.Pointer
	iconLen := C.int(0)
	if len(cfg.IconPNG) > 0 {
		iconPtr = unsafe.Pointer(&cfg.IconPNG[0])
		iconLen = C.int(len(cfg.IconPNG))
	}

	var cTooltip *C.char
	if cfg.Tooltip != "" {
		cTooltip = C.CString(cfg.Tooltip)
		defer C.free(unsafe.Pointer(cTooltip))
	}

	var itemsPtr *C.NativeMenuItemC
	if len(cItems) > 0 {
		itemsPtr = &cItems[0]
	}

	C.nativemenuUpdateTray(C.int(id),
		iconPtr, iconLen,
		cTooltip,
		itemsPtr, C.int(len(cItems)))
}

// RemoveSystemTray removes a tray icon.
func RemoveSystemTray(id int) {
	C.nativemenuRemoveTray(C.int(id))
	mu.Lock()
	delete(trayActionCbs, id)
	mu.Unlock()
}
