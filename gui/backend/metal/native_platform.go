//go:build darwin && !ios

package metal

import (
	"github.com/go-gui-org/go-gui/gui"
	"github.com/go-gui-org/go-gui/gui/backend/internal/nativehost"
	"github.com/go-gui-org/go-gui/gui/backend/nativemenu"
	"github.com/veandco/go-sdl2/sdl"
)

// nativePlatform implements gui.NativePlatform for the Metal
// backend.
type nativePlatform struct {
	window *sdl.Window
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
	a11yActionCallback = cb
	a11yInitBridge(n.window)
}

func (n *nativePlatform) A11ySync(nodes []gui.A11yNode, count, focusedIdx int) {
	_, h := n.window.GetSize()
	a11ySyncBridge(nodes, count, focusedIdx, float32(h))
}

func (n *nativePlatform) A11yDestroy() {
	a11yDestroyBridge()
}

func (n *nativePlatform) A11yAnnounce(text string) {
	a11yAnnounceBridge(text)
}

// --- IME ---

func (n *nativePlatform) IMEStart() { sdl.StartTextInput() }
func (n *nativePlatform) IMEStop()  { sdl.StopTextInput() }
func (n *nativePlatform) IMESetRect(x, y, w, h int32) {
	sdl.SetTextInputRect(&sdl.Rect{X: x, Y: y, W: w, H: h})
}

// --- Appearance ---

func (n *nativePlatform) TitlebarDark(_ bool) {}

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
