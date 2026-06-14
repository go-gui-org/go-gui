// Package nativehost provides shared native-platform helpers
// used by desktop backends (SDL2, GL, Metal). It consolidates
// URI validation, notification dispatch, and thin forwarders to
// sub-packages so each backend doesn't duplicate the same glue.
package nativehost

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
)

// maxURILen caps the raw URI length to prevent OOM from maliciously
// large input. 8 KB exceeds any practical URL (browsers typically
// cap at ~2 KB; IE's historical limit was 2083).
const maxURILen = 8192

// maxNotifyTitleLen caps notification title length to match
// platform limits (macOS NSUserNotification ≈ 256, GTK ≈ 128).
const maxNotifyTitleLen = 256

// maxNotifyBodyLen caps notification body length. Platform limits
// vary but 1 KB is safe across all three desktop targets.
const maxNotifyBodyLen = 1024

// maxSpellTextLen caps the text passed to native spell-check APIs.
// 64 KB covers any realistic paragraph without risking Hunspell
// allocation blowup.
const maxSpellTextLen = 64 << 10

// maxSpellWordLen caps a single word for spell-check learning.
const maxSpellWordLen = 256

// ValidateOpenURI checks that raw is a valid absolute URI whose scheme
// is in the allowlist (http, https, mailto).
func ValidateOpenURI(raw string) error {
	if len(raw) > maxURILen {
		return fmt.Errorf("URI too long: %d bytes (max %d)",
			len(raw), maxURILen)
	}
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

// OpenURI validates uri and opens it in the default OS handler.
func OpenURI(uri string) error {
	if err := ValidateOpenURI(uri); err != nil {
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

// SendNotification dispatches a desktop notification using the best
// available OS mechanism. Returns NotificationOK on success or a
// result with Status=NotificationError on failure.
//
// title is capped at maxNotifyTitleLen; body is capped at
// maxNotifyBodyLen to prevent argument-list overflow in the
// underlying shell commands.
func SendNotification(title, body string) gui.NativeNotificationResult {
	if len(title) > maxNotifyTitleLen {
		title = title[:maxNotifyTitleLen]
	}
	if len(body) > maxNotifyBodyLen {
		body = body[:maxNotifyBodyLen]
	}
	var cmd *exec.Cmd
	var fireAndForget bool

	switch runtime.GOOS {
	case "darwin":
		cmd = exec.Command("osascript",
			"-e", "on run argv",
			"-e", "display notification (item 2 of argv) with title (item 1 of argv)",
			"-e", "end run",
			"--", title, body)
	case "linux":
		// "--" so attacker-controlled title/body never get
		// interpreted as flags.
		cmd = exec.Command("notify-send", "--", title, body)
	case "windows":
		// BalloonTip via PowerShell — works on Windows 7+.
		// Single-quoted strings prevent variable/escape
		// injection; only ' needs escaping (doubled).
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
		fireAndForget = true
	default:
		return gui.NativeNotificationResult{
			Status:       gui.NotificationError,
			ErrorCode:    "unsupported",
			ErrorMessage: "unsupported platform: " + runtime.GOOS,
		}
	}

	if fireAndForget {
		if err := cmd.Start(); err != nil {
			return gui.NativeNotificationResult{
				Status:       gui.NotificationError,
				ErrorCode:    "exec_failed",
				ErrorMessage: err.Error(),
			}
		}
		go cmd.Wait() //nolint:errcheck
		return gui.NativeNotificationResult{Status: gui.NotificationOK}
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

// --- Dialog forwarders ---

// ShowOpenDialog forwards to filedialog.ShowOpenDialog.
func ShowOpenDialog(title, startDir string, extensions []string, allowMultiple bool) gui.PlatformDialogResult {
	return filedialog.ShowOpenDialog(title, startDir, extensions, allowMultiple)
}

// ShowSaveDialog forwards to filedialog.ShowSaveDialog.
func ShowSaveDialog(title, startDir, defaultName, defaultExt string, extensions []string, confirmOverwrite bool) gui.PlatformDialogResult {
	return filedialog.ShowSaveDialog(title, startDir, defaultName, defaultExt, extensions, confirmOverwrite)
}

// ShowFolderDialog forwards to filedialog.ShowFolderDialog.
func ShowFolderDialog(title, startDir string) gui.PlatformDialogResult {
	return filedialog.ShowFolderDialog(title, startDir)
}

// ShowMessageDialog forwards to filedialog.ShowMessageDialog.
func ShowMessageDialog(title, body string, level gui.NativeAlertLevel) gui.NativeAlertResult {
	return filedialog.ShowMessageDialog(title, body, level)
}

// ShowConfirmDialog forwards to filedialog.ShowConfirmDialog.
func ShowConfirmDialog(title, body string, level gui.NativeAlertLevel) gui.NativeAlertResult {
	return filedialog.ShowConfirmDialog(title, body, level)
}

// ShowSaveDiscardDialog forwards to filedialog.ShowSaveDiscardDialog.
func ShowSaveDiscardDialog(title, body string, level gui.NativeAlertLevel) gui.NativeAlertResult {
	return filedialog.ShowSaveDiscardDialog(title, body, level)
}

// --- Print forwarder ---

// ShowPrintDialog forwards to printdialog.ShowPrintDialog.
func ShowPrintDialog(cfg gui.NativePrintParams) gui.PrintRunResult {
	return printdialog.ShowPrintDialog(cfg)
}

// --- Spell-check forwarders ---

// SpellCheck forwards to spellcheck.Check. Caps text at
// maxSpellTextLen to prevent Hunspell allocation blowup on
// pathological input.
func SpellCheck(text string) []gui.SpellRange {
	if len(text) > maxSpellTextLen {
		text = text[:maxSpellTextLen]
	}
	return spellcheck.Check(text)
}

// SpellSuggest forwards to spellcheck.Suggest. Caps text at
// maxSpellTextLen and clamps startByte/lenBytes to valid ranges
// to prevent out-of-bounds access in the native spell engine.
func SpellSuggest(text string, startByte, lenBytes int) []string {
	if len(text) > maxSpellTextLen {
		text = text[:maxSpellTextLen]
	}
	if startByte < 0 {
		startByte = 0
	}
	if startByte >= len(text) {
		return nil
	}
	if lenBytes <= 0 || startByte+lenBytes > len(text) {
		lenBytes = len(text) - startByte
	}
	return spellcheck.Suggest(text, startByte, lenBytes)
}

// SpellLearn forwards to spellcheck.Learn. Caps word at
// maxSpellWordLen — words longer than this are not realistic
// dictionary entries.
func SpellLearn(word string) {
	if len(word) > maxSpellWordLen {
		word = word[:maxSpellWordLen]
	}
	spellcheck.Learn(word)
}
