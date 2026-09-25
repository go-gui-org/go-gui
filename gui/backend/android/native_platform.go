//go:build android

package android

import (
	"fmt"
	"net/url"
	"strings"
	"sync/atomic"
	"unicode/utf8"

	"github.com/go-gui-org/go-gui/gui"
	"github.com/go-gui-org/go-gui/gui/backend/filedialog"
	"github.com/go-gui-org/go-gui/gui/backend/printdialog"
	"github.com/go-gui-org/go-gui/gui/backend/spellcheck"
)

// androidTrayIDs hands out unique positive tray IDs. Android has no
// tray; the no-op still reports success, so handles must stay distinct.
var androidTrayIDs atomic.Int64

// nativePlatform implements gui.NativePlatform for Android.
type nativePlatform struct{}

// maxOpenURILen caps the raw URI length, mirroring nativehost's
// limit for the desktop backends.
const maxOpenURILen = 8192

func (n *nativePlatform) OpenURI(uri string) error {
	if len(uri) > maxOpenURILen {
		return fmt.Errorf("URI too long: %d bytes (max %d)",
			len(uri), maxOpenURILen)
	}
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
func (n *nativePlatform) IMEStart()                { setPendingIMEAction(pendingIMEShow) }
func (n *nativePlatform) IMEStop()                 { setPendingIMEAction(pendingIMEHide) }
func (n *nativePlatform) IMESetRect(x, y, w, h int32) {
	setPendingIMERect(x, y, w, h)
}
func (n *nativePlatform) TitlebarDark(_ bool) {}
func (n *nativePlatform) SpellCheck(text string) []gui.SpellRange {
	// Cap mirrors nativehost: pathological input must not reach
	// the spell engine uncapped.
	return spellcheck.Check(truncateUTF8(text, maxSpellTextLen))
}

func (n *nativePlatform) SetWindowVibrancy(_ gui.VibrancyMaterial) {}

func (n *nativePlatform) SetWindowOpacity(_ float32) {}

// No window manager to hand a move or resize gesture to.
func (n *nativePlatform) StartWindowDrag()                   {}
func (n *nativePlatform) StartWindowResize(_ gui.WindowEdge) {}

// No window manager to show or hide a window with on Android.
func (n *nativePlatform) ShowWindow() {}
func (n *nativePlatform) HideWindow() {}

func (n *nativePlatform) SpellSuggest(text string, s, l int) []string {
	// Bounds handling mirrors nativehost.SpellSuggest so the spell
	// engine never sees out-of-range offsets.
	text = truncateUTF8(text, maxSpellTextLen)
	if s < 0 {
		s = 0
	}
	if s >= len(text) {
		return nil
	}
	// Written as a subtraction, not s+l: the sum overflows for a
	// hostile l near math.MaxInt and the bound check would pass.
	if l <= 0 || l > len(text)-s {
		l = len(text) - s
	}
	return spellcheck.Suggest(text, s, l)
}
func (n *nativePlatform) SpellLearn(word string) {
	spellcheck.Learn(truncateUTF8(word, maxSpellWordLen))
}

// maxSpellTextLen and maxSpellWordLen mirror nativehost's caps so
// pathological input cannot blow up the spell engine's allocations.
const (
	maxSpellTextLen = 64 << 10
	maxSpellWordLen = 256
)

// truncateUTF8 caps s at max bytes on a rune boundary, so the
// result stays valid UTF-8. A naive s[:max] can split a multi-byte
// rune and hand broken UTF-8 to the spell engine.
func truncateUTF8(s string, max int) string {
	if len(s) <= max {
		return s
	}
	end := max
	for end > 0 && !utf8.RuneStart(s[end]) {
		end--
	}
	return s[:end]
}

// Native menubar — no-op on Android.
func (n *nativePlatform) SetNativeMenubar(_ gui.NativeMenubarCfg, _ func(string)) {}
func (n *nativePlatform) ClearNativeMenubar()                                     {}

// System tray — no-op on Android. Reports success with a unique
// handle so App bookkeeping stays consistent.
func (n *nativePlatform) CreateSystemTray(_ gui.SystemTrayCfg, _ func(string)) (int, error) {
	return int(androidTrayIDs.Add(1)), nil
}
func (n *nativePlatform) UpdateSystemTray(_ int, _ gui.SystemTrayCfg) {}
func (n *nativePlatform) RemoveSystemTray(_ int)                      {}

// --- Sound ---

// Beep is a no-op: this platform routes alerts through its own
// notification framework rather than an app-triggered system sound.
func (n *nativePlatform) Beep() {}

func (n *nativePlatform) BeepAvailable() bool { return false }
