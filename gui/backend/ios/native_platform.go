//go:build ios

package ios

import (
	"fmt"
	"net/url"
	"strings"

	"github.com/go-gui-org/go-gui/gui"
	"github.com/go-gui-org/go-gui/gui/backend/filedialog"
	"github.com/go-gui-org/go-gui/gui/backend/printdialog"
	"github.com/go-gui-org/go-gui/gui/backend/spellcheck"
)

// nativePlatform implements gui.NativePlatform for iOS.
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
	return fmt.Errorf("OpenURI not implemented on iOS")
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

func (n *nativePlatform) SendNotification(_, _ string) gui.NativeNotificationResult {
	return gui.NativeNotificationResult{
		Status:       gui.NotificationError,
		ErrorCode:    "unsupported",
		ErrorMessage: "notifications not available on iOS",
	}
}

func (n *nativePlatform) ShowPrintDialog(cfg gui.NativePrintParams) gui.PrintRunResult {
	return printdialog.ShowPrintDialog(cfg)
}

func (n *nativePlatform) BookmarkLoadAll(_ string) []gui.BookmarkEntry { return nil }
func (n *nativePlatform) BookmarkPersist(_, _ string, _ []byte)        {}
func (n *nativePlatform) BookmarkStopAccess(_ []byte)                  {}

func (n *nativePlatform) A11yInit(_ func(action, index int))      {}
func (n *nativePlatform) A11ySync(_ []gui.A11yNode, _, _ int)     {}
func (n *nativePlatform) A11yDestroy()                            {}
func (n *nativePlatform) A11yAnnounce(_ string)                   {}
func (n *nativePlatform) IMEStart()                               {}
func (n *nativePlatform) IMEStop()                                {}
func (n *nativePlatform) IMESetRect(_, _, _, _ int32)             {}
func (n *nativePlatform) TitlebarDark(_ bool)                     {}
func (n *nativePlatform) SpellCheck(text string) []gui.SpellRange { return spellcheck.Check(text) }
func (n *nativePlatform) SpellSuggest(text string, s, l int) []string {
	return spellcheck.Suggest(text, s, l)
}
func (n *nativePlatform) SpellLearn(word string) { spellcheck.Learn(word) }

// Native menubar — no-op on iOS.
func (n *nativePlatform) SetNativeMenubar(_ gui.NativeMenubarCfg, _ func(string)) {}
func (n *nativePlatform) ClearNativeMenubar()                                     {}

// System tray — no-op on iOS.
func (n *nativePlatform) CreateSystemTray(_ gui.SystemTrayCfg, _ func(string)) (int, error) {
	return 0, nil
}
func (n *nativePlatform) UpdateSystemTray(_ int, _ gui.SystemTrayCfg) {}
func (n *nativePlatform) RemoveSystemTray(_ int)                      {}
