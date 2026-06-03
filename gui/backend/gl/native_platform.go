//go:build !js

package gl

import (
	"fmt"
	"net/url"
	"os/exec"
	"runtime"
	"strings"

	"github.com/go-gui-org/go-gui/gui"
	"github.com/go-gui-org/go-gui/gui/backend/filedialog"
	"github.com/go-gui-org/go-gui/gui/backend/printdialog"
	"github.com/go-gui-org/go-gui/gui/backend/spellcheck"
	"github.com/veandco/go-sdl2/sdl"
)

// nativePlatform implements gui.NativePlatform for the GL backend.
type nativePlatform struct{}

func (n *nativePlatform) OpenURI(uri string) error {
	if err := validateOpenURI(uri); err != nil {
		return err
	}
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "darwin":
		cmd = exec.Command("open", uri)
	case "linux":
		cmd = exec.Command("xdg-open", uri)
	case "windows":
		cmd = exec.Command("rundll32", "url.dll,FileProtocolHandler", uri)
	default:
		return fmt.Errorf("unsupported platform: %s", runtime.GOOS)
	}
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("OpenURI: %w", err)
	}
	return nil
}

func validateOpenURI(raw string) error {
	u, err := url.Parse(raw)
	if err != nil {
		return fmt.Errorf("invalid URI: %w", err)
	}
	scheme := strings.ToLower(u.Scheme)
	switch scheme {
	case "http", "https", "mailto":
		return nil
	default:
		return fmt.Errorf("unsupported URI scheme: %q", u.Scheme)
	}
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
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "darwin":
		cmd = exec.Command("osascript",
			"-e", "on run argv",
			"-e", "display notification (item 2 of argv) with title (item 1 of argv)",
			"-e", "end run",
			"--", title, body)
	case "linux":
		// Use "--" so attacker-controlled title/body never get
		// interpreted as flags.
		cmd = exec.Command("notify-send", "--", title, body)
	case "windows":
		// BalloonTip via PowerShell — works on Windows 7+.
		// Single-quoted strings prevent variable/escape injection;
		// only ' needs escaping (doubled).
		safeTitle := strings.ReplaceAll(title, "'", "''")
		safeBody := strings.ReplaceAll(body, "'", "''")
		cmd = exec.Command("powershell", "-NoProfile", "-NonInteractive",
			"-Command", fmt.Sprintf(
				`Add-Type -AssemblyName System.Windows.Forms;`+
					`$n=New-Object System.Windows.Forms.NotifyIcon;`+
					`$n.Icon=[System.Drawing.SystemIcons]::Information;`+
					`$n.BalloonTipTitle='%s';`+
					`$n.BalloonTipText='%s';`+
					`$n.Visible=$true;`+
					`$n.ShowBalloonTip(5000);`+
					`Start-Sleep -Seconds 6;`+
					`$n.Dispose()`,
				safeTitle, safeBody))
		// Fire-and-forget — notification outlives the call.
		if err := cmd.Start(); err != nil {
			return gui.NativeNotificationResult{
				Status:       gui.NotificationError,
				ErrorCode:    "exec_failed",
				ErrorMessage: err.Error(),
			}
		}
		go cmd.Wait() //nolint:errcheck
		return gui.NativeNotificationResult{Status: gui.NotificationOK}
	default:
		return gui.NativeNotificationResult{
			Status:       gui.NotificationError,
			ErrorCode:    "unsupported",
			ErrorMessage: "unsupported platform: " + runtime.GOOS,
		}
	}
	if err := cmd.Run(); err != nil {
		return gui.NativeNotificationResult{
			Status:       gui.NotificationError,
			ErrorCode:    "exec_failed",
			ErrorMessage: err.Error(),
		}
	}
	return gui.NativeNotificationResult{Status: gui.NotificationOK}
}

func (n *nativePlatform) ShowPrintDialog(cfg gui.NativePrintParams) gui.PrintRunResult {
	return printdialog.ShowPrintDialog(cfg)
}

func (n *nativePlatform) BookmarkLoadAll(_ string) []gui.BookmarkEntry { return nil }
func (n *nativePlatform) BookmarkPersist(_, _ string, _ []byte)        {}
func (n *nativePlatform) BookmarkStopAccess(_ []byte)                  {}

func (n *nativePlatform) IMEStart() { sdl.StartTextInput() }
func (n *nativePlatform) IMEStop()  { sdl.StopTextInput() }
func (n *nativePlatform) IMESetRect(x, y, w, h int32) {
	sdl.SetTextInputRect(&sdl.Rect{X: x, Y: y, W: w, H: h})
}
func (n *nativePlatform) TitlebarDark(_ bool) {}

func (n *nativePlatform) SpellCheck(text string) []gui.SpellRange {
	return spellcheck.Check(text)
}

func (n *nativePlatform) SpellSuggest(text string, startByte, lenBytes int) []string {
	return spellcheck.Suggest(text, startByte, lenBytes)
}

func (n *nativePlatform) SpellLearn(word string) {
	spellcheck.Learn(word)
}

// Native menubar — no-op on GL.
func (n *nativePlatform) SetNativeMenubar(_ gui.NativeMenubarCfg, _ func(string)) {}
func (n *nativePlatform) ClearNativeMenubar()                                     {}
