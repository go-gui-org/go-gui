//go:build !js

package gl

import (
	"github.com/go-gui-org/go-gui/gui"
	"github.com/go-gui-org/go-gui/gui/backend/internal/nativehost"
)

// nativePlatform implements gui.NativePlatform for the GL backend.
type nativePlatform struct{}

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

// IME methods (IMEStart/IMEStop/IMESetRect) are platform-specific
// and defined in platform_sdl2.go / platform_win32.go.

// --- Appearance ---

func (n *nativePlatform) TitlebarDark(_ bool) {}

func (n *nativePlatform) SetWindowVibrancy(_ gui.VibrancyMaterial) {}

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

// --- Menubar (no-op on GL) ---

func (n *nativePlatform) SetNativeMenubar(_ gui.NativeMenubarCfg, _ func(string)) {}
func (n *nativePlatform) ClearNativeMenubar()                                     {}
