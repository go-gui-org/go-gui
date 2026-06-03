//go:build android

package android

import (
	"fmt"
	"net/url"
	"strings"

	"github.com/go-gui-org/go-gui/gui"
	"github.com/go-gui-org/go-gui/gui/backend/filedialog"
	"github.com/go-gui-org/go-gui/gui/backend/printdialog"
	"github.com/go-gui-org/go-gui/gui/backend/spellcheck"
)

// nativePlatform implements gui.NativePlatform for Android.
type nativePlatform struct{}

func (n *nativePlatform) OpenURI(uri string) error {
	u, err := url.Parse(uri)
	if err != nil {
		return fmt.Errorf("invalid URI: %w", err)
	}
	scheme := strings.ToLower(u.Scheme)
	switch scheme {
	case "http", "https", "mailto":
	default:
		return fmt.Errorf("unsupported URI scheme: %q",
			u.Scheme)
	}
	setPendingURI(uri)
	return nil
}

func (n *nativePlatform) ShowOpenDialog(title, startDir string, extensions []string, allowMultiple bool) gui.PlatformDialogResult {
	return filedialog.ShowOpenDialog(title, startDir, extensions, allowMultiple)
}

func (n *nativePlatform) ShowSaveDialog(title, startDir, defaultName, defaultExt string, extensions []string, confirmOverwrite bool) gui.PlatformDialogResult {
	return filedialog.ShowSaveDialog(title, startDir, defaultName, defaultExt, extensions, confirmOverwrite)
}

func (n *nativePlatform) ShowFolderDialog(title, startDir string) gui.PlatformDialogResult {
	return filedialog.ShowFolderDialog(title, startDir)
}

func (n *nativePlatform) ShowMessageDialog(title, body string, level gui.NativeAlertLevel) gui.NativeAlertResult {
	return filedialog.ShowMessageDialog(title, body, level)
}

func (n *nativePlatform) ShowConfirmDialog(title, body string, level gui.NativeAlertLevel) gui.NativeAlertResult {
	return filedialog.ShowConfirmDialog(title, body, level)
}

func (n *nativePlatform) ShowSaveDiscardDialog(title, body string, level gui.NativeAlertLevel) gui.NativeAlertResult {
	return filedialog.ShowSaveDiscardDialog(title, body, level)
}

func (n *nativePlatform) SendNotification(title, body string) gui.NativeNotificationResult {
	return setPendingNotification(title, body)
}

func (n *nativePlatform) ShowPrintDialog(cfg gui.NativePrintParams) gui.PrintRunResult {
	return printdialog.ShowPrintDialog(cfg)
}

func (n *nativePlatform) BookmarkLoadAll(_ string) []gui.BookmarkEntry { return nil }
func (n *nativePlatform) BookmarkPersist(_, _ string, _ []byte)        {}
func (n *nativePlatform) BookmarkStopAccess(_ []byte)                  {}

func (n *nativePlatform) A11yInit(cb func(action, index int)) { initA11y(cb) }
func (n *nativePlatform) A11ySync(nodes []gui.A11yNode, count, focusedIdx int) {
	syncA11y(nodes, count, focusedIdx)
}
func (n *nativePlatform) A11yDestroy()             { destroyA11y() }
func (n *nativePlatform) A11yAnnounce(text string) { setA11yAnnounce(text) }
func (n *nativePlatform) IMEStart()                { setPendingIMEAction(1) }
func (n *nativePlatform) IMEStop()                 { setPendingIMEAction(2) }
func (n *nativePlatform) IMESetRect(x, y, w, h int32) {
	setPendingIMERect(x, y, w, h)
}
func (n *nativePlatform) TitlebarDark(_ bool)                     {}
func (n *nativePlatform) SpellCheck(text string) []gui.SpellRange { return spellcheck.Check(text) }
func (n *nativePlatform) SpellSuggest(text string, s, l int) []string {
	return spellcheck.Suggest(text, s, l)
}
func (n *nativePlatform) SpellLearn(word string) { spellcheck.Learn(word) }

// Native menubar — no-op on Android.
func (n *nativePlatform) SetNativeMenubar(_ gui.NativeMenubarCfg, _ func(string)) {}
func (n *nativePlatform) ClearNativeMenubar()                                     {}

// System tray — no-op on Android.
func (n *nativePlatform) CreateSystemTray(_ gui.SystemTrayCfg, _ func(string)) (int, error) {
	return 0, nil
}
func (n *nativePlatform) UpdateSystemTray(_ int, _ gui.SystemTrayCfg) {}
func (n *nativePlatform) RemoveSystemTray(_ int)                      {}
