//go:build darwin && !ios

package metal

/*
#include <stdlib.h>
#include "metal_window.h"
#include "a11y_darwin.h"
*/
import "C"
import (
	"unsafe"

	"github.com/go-gui-org/go-gui/gui"
	"github.com/go-gui-org/go-gui/gui/backend/internal/nativehost"
	"github.com/go-gui-org/go-gui/gui/backend/nativemenu"
)

// nativePlatform implements gui.NativePlatform for the Metal backend.
type nativePlatform struct {
	window C.GoGuiNSWindow
}

// --- URI ---

func (n *nativePlatform) OpenURI(uri string) error {
	return nativehost.OpenURI(uri)
}

// --- Dialogs ---

func (n *nativePlatform) ShowOpenDialog(title, startDir string, extensions []string, allowMultiple bool) gui.PlatformDialogResult {
	return nativehost.ShowOpenDialog(title, startDir, extensions, allowMultiple)
}

func (n *nativePlatform) ShowSaveDialog(title, startDir, defaultName, defaultExt string, extensions []string, confirmOverwrite bool) gui.PlatformDialogResult {
	return nativehost.ShowSaveDialog(title, startDir, defaultName, defaultExt, extensions, confirmOverwrite)
}

func (n *nativePlatform) ShowFolderDialog(title, startDir string) gui.PlatformDialogResult {
	return nativehost.ShowFolderDialog(title, startDir)
}

func (n *nativePlatform) ShowMessageDialog(title, body string, level gui.NativeAlertLevel) gui.NativeAlertResult {
	return nativehost.ShowMessageDialog(title, body, level)
}

func (n *nativePlatform) ShowConfirmDialog(title, body string, level gui.NativeAlertLevel) gui.NativeAlertResult {
	return nativehost.ShowConfirmDialog(title, body, level)
}

func (n *nativePlatform) ShowSaveDiscardDialog(title, body string, level gui.NativeAlertLevel) gui.NativeAlertResult {
	return nativehost.ShowSaveDiscardDialog(title, body, level)
}

// --- Notification ---

func (n *nativePlatform) SendNotification(title, body string) gui.NativeNotificationResult {
	return nativehost.SendNotification(title, body)
}

// --- Print ---

func (n *nativePlatform) ShowPrintDialog(cfg gui.NativePrintParams) gui.PrintRunResult {
	return nativehost.ShowPrintDialog(cfg)
}

// --- Bookmarks ---

func (n *nativePlatform) BookmarkLoadAll(_ string) []gui.BookmarkEntry { return nil }
func (n *nativePlatform) BookmarkPersist(_, _ string, _ []byte)        {}
func (n *nativePlatform) BookmarkStopAccess(_ []byte)                  {}

// --- Accessibility ---

func (n *nativePlatform) A11yInit(cb func(action, index int)) {
	setA11yCallback(cb)
	C.a11yInit(n.window)
}

func (n *nativePlatform) A11ySync(nodes []gui.A11yNode, count, focusedIdx int) {
	var logW, logH C.int
	C.metalWindowGetSize(n.window, &logW, &logH)
	a11ySyncBridge(nodes, count, focusedIdx, float32(logH))
}

func (n *nativePlatform) A11yDestroy() {
	C.a11yDestroy()
}

func (n *nativePlatform) A11yAnnounce(text string) {
	cstr := C.CString(text)
	defer C.free(unsafe.Pointer(cstr))
	C.a11yAnnounce(cstr)
}

// --- IME ---

func (n *nativePlatform) IMEStart() {
	C.metalWindowIMESetActive(n.window, 1)
}

func (n *nativePlatform) IMEStop() {
	C.metalWindowIMESetActive(n.window, 0)
}

func (n *nativePlatform) IMESetRect(x, y, w, h int32) {
	C.metalWindowIMESetCursorRect(n.window,
		C.float(x), C.float(y), C.float(w), C.float(h))
}

// --- Appearance ---

func (n *nativePlatform) TitlebarDark(_ bool) {}

func (n *nativePlatform) SetWindowVibrancy(m gui.VibrancyMaterial) {
	C.metalWindowSetVibrancy(n.window, C.int(m))
}

// --- Spell check ---

func (n *nativePlatform) SpellCheck(text string) []gui.SpellRange {
	return nativehost.SpellCheck(text)
}

func (n *nativePlatform) SpellSuggest(text string, startByte, lenBytes int) []string {
	return nativehost.SpellSuggest(text, startByte, lenBytes)
}

func (n *nativePlatform) SpellLearn(word string) {
	nativehost.SpellLearn(word)
}

// --- Menubar ---

func (n *nativePlatform) SetNativeMenubar(
	cfg gui.NativeMenubarCfg, actionCb func(string),
) {
	nativemenu.SetMenubar(cfg, actionCb)
}

func (n *nativePlatform) ClearNativeMenubar() {
	nativemenu.ClearMenubar()
}

// --- System tray ---

func (n *nativePlatform) CreateSystemTray(
	cfg gui.SystemTrayCfg, actionCb func(string),
) (int, error) {
	return nativemenu.CreateSystemTray(cfg, actionCb)
}

func (n *nativePlatform) UpdateSystemTray(
	id int, cfg gui.SystemTrayCfg,
) {
	nativemenu.UpdateSystemTray(id, cfg)
}

func (n *nativePlatform) RemoveSystemTray(id int) {
	nativemenu.RemoveSystemTray(id)
}
