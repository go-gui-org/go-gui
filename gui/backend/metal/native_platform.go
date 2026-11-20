//go:build darwin && cgo && !ios

package metal

import (
	"unicode/utf8"

	"github.com/go-gui-org/go-gui/gui"
	"github.com/go-gui-org/go-gui/gui/backend/internal/keyring"
	"github.com/go-gui-org/go-gui/gui/backend/internal/nativehost"
	"github.com/go-gui-org/go-gui/gui/backend/nativemenu"
)

// nativePlatform implements gui.NativePlatform for the Metal backend.
type nativePlatform struct {
	// keyring.Platform supplies SecretLoad, SecretSave and SecretDelete,
	// gui's optional secret-store hook (issue #920).
	keyring.Platform
	window *macWindow
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
	setA11yCallback(a11yToken(n.window), cb)
	a11yInit(a11yToken(n.window), metalWindowNS(n.window))
}

func (n *nativePlatform) A11ySync(nodes []gui.A11yNode, count, focusedIdx int) {
	_, logH := metalWindowGetSize(n.window)
	a11ySyncBridge(a11yToken(n.window), nodes, count, focusedIdx, float32(logH))
}

func (n *nativePlatform) A11yDestroy() {
	clearA11yCallback(a11yToken(n.window))
	a11yDestroy(a11yToken(n.window))
}

func (n *nativePlatform) A11yAnnounce(text string) {
	// Cap the announcement text: VoiceOver has no use for more, and the
	// text is copied into a CFString.
	a11yAnnounce(truncateA11yText(text, maxA11yAnnounceLen))
}

// a11yToken names the tree of window w: its NSWindow, as a number. It
// is 0 for nil.
func a11yToken(w *macWindow) uintptr {
	return uintptr(metalWindowNS(w))
}

// maxA11yAnnounceLen caps spoken announcement text at 8 KB, well
// beyond any realistic announcement.
const maxA11yAnnounceLen = 8192

// truncateA11yText caps s at max bytes on a rune boundary, so the
// text stays valid UTF-8.
func truncateA11yText(s string, max int) string {
	if len(s) <= max {
		return s
	}
	end := max
	for end > 0 && !utf8.RuneStart(s[end]) {
		end--
	}
	return s[:end]
}

// --- IME ---

func (n *nativePlatform) IMEStart() {
	metalWindowIMESetActive(n.window, true)
}

func (n *nativePlatform) IMEStop() {
	metalWindowIMESetActive(n.window, false)
}

func (n *nativePlatform) IMESetRect(x, y, w, h int32) {
	metalWindowIMESetCursorRect(n.window, float32(x), float32(y), float32(w), float32(h))
}

// macOS has no soft keyboard to ask for (issue #770).
func (n *nativePlatform) ShowSoftKeyboard(_ gui.KeyboardKind, _ bool) {}
func (n *nativePlatform) HideSoftKeyboard()                           {}

// --- Appearance ---

// TitlebarDark pins this window's appearance to dark or light (#1207).
func (n *nativePlatform) TitlebarDark(dark bool) {
	metalWindowSetAppearanceDark(n.window, dark)
}

func (n *nativePlatform) SetWindowVibrancy(m gui.VibrancyMaterial) {
	metalWindowSetVibrancy(n.window, int(m))
}

func (n *nativePlatform) SetWindowOpacity(opacity float32) {
	metalWindowSetAlpha(n.window, opacity)
}

func (n *nativePlatform) StartWindowDrag() {
	metalWindowStartDrag(n.window)
}

// StartWindowResize is a no-op on macOS: a borderless window keeps
// NSWindowStyleMaskResizable, so AppKit still resizes it from the
// edges and an app-drawn grip has nothing to add.
func (n *nativePlatform) StartWindowResize(_ gui.WindowEdge) {}

// ShowWindow unhides a hidden window, keys it, and activates the app
// so a tray "Show Window" item brings the app forward (issue #779).
func (n *nativePlatform) ShowWindow() {
	metalWindowShow(n.window)
}

// HideWindow removes the window from the screen without destroying
// it, so a tray menu can bring it back with ShowWindow.
func (n *nativePlatform) HideWindow() {
	metalWindowHide(n.window)
}

// MinimizeWindow sends the window to the Dock.
func (n *nativePlatform) MinimizeWindow() { metalWindowMinimize(n.window) }

// MaximizeWindow zooms the window.
func (n *nativePlatform) MaximizeWindow() { metalWindowMaximize(n.window) }

// RestoreWindow undoes a minimize, then a zoom.
func (n *nativePlatform) RestoreWindow() { metalWindowRestore(n.window) }

// SetWindowFullscreen enters or leaves a fullscreen Space. It animates;
// the state arrives through goMetalWindowState when that ends.
func (n *nativePlatform) SetWindowFullscreen(fullscreen bool) {
	metalWindowSetFullscreen(n.window, fullscreen)
}

// ResizeWindow sets the content area, in points.
func (n *nativePlatform) ResizeWindow(width, height int) {
	metalWindowResize(n.window, width, height)
}

// MoveWindow puts the content area's corner at x, y.
func (n *nativePlatform) MoveWindow(x, y int) {
	metalWindowMove(n.window, x, y)
}

// WindowPosition returns the content area's corner in y-down points. A
// minimized window reports no position.
func (n *nativePlatform) WindowPosition() (x, y int, ok bool) {
	return metalWindowGetPosition(n.window)
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

// --- Sound ---

func (n *nativePlatform) Beep() { nativehost.Beep() }

func (n *nativePlatform) BeepAvailable() bool { return nativehost.BeepAvailable() }

// PlaySystemSound and SystemSoundAvailable satisfy gui's optional
// system-sound capability, which gui.NewSystemSoundPlayer type-asserts
// for (issue #469).
func (n *nativePlatform) PlaySystemSound(cue gui.SoundCue) { nativehost.PlaySystemSound(cue) }

func (n *nativePlatform) SystemSoundAvailable() bool { return nativehost.SystemSoundAvailable() }

// startHidden is the hidden argument of metalWindowCreate: true for a
// window the app hid before the backend started (issue #1270). Such a
// window is created but not ordered front, and Window.Show brings it up
// later.
func startHidden(w *gui.Window) bool {
	return !w.IsVisible()
}
