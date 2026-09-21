// Package nativehost provides shared native-platform helpers
// used by desktop backends (GL, Metal). It consolidates
// URI validation, notification dispatch, and thin forwarders to
// sub-packages so each backend doesn't duplicate the same glue.
package nativehost

import (
	"fmt"
	"net/url"
	"os/exec"
	"runtime"
	"strings"
	"unicode/utf8"

	"github.com/go-gui-org/go-gui/gui"
	"github.com/go-gui-org/go-gui/gui/backend/filedialog"
	"github.com/go-gui-org/go-gui/gui/backend/printdialog"
	"github.com/go-gui-org/go-gui/gui/backend/spellcheck"
	"github.com/go-gui-org/go-gui/gui/backend/sysbeep"
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
//
// #nosec G204 — uri validated via ValidateOpenURI (scheme allowlist)
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

// truncateBytes caps s at max bytes on a rune boundary, so the
// result stays valid UTF-8. A naive s[:max] can split a multi-byte
// rune and hand broken UTF-8 to exec or the spell engine.
func truncateBytes(s string, max int) string {
	if len(s) <= max {
		return s
	}
	end := max
	for end > 0 && !utf8.RuneStart(s[end]) {
		end--
	}
	return s[:end]
}

// SendNotification dispatches a desktop notification. NotificationOK
// means the platform accepted the request, not that the user saw it.
func SendNotification(title, body string) gui.NativeNotificationResult {
	return notificationSender(truncateBytes(title, maxNotifyTitleLen),
		truncateBytes(body, maxNotifyBodyLen))
}

// Tests replace the synchronous dispatch boundary, so testing argument
// validation never creates a desktop notification or outliving subprocess.
var notificationSender = sendNotification

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
	return spellcheck.Check(truncateBytes(text, maxSpellTextLen))
}

// SpellSuggest forwards to spellcheck.Suggest. Caps text at
// maxSpellTextLen and clamps startByte/lenBytes to valid ranges
// to prevent out-of-bounds access in the native spell engine.
func SpellSuggest(text string, startByte, lenBytes int) []string {
	text = truncateBytes(text, maxSpellTextLen)
	if startByte < 0 {
		startByte = 0
	}
	if startByte >= len(text) {
		return nil
	}
	// Written as a subtraction, not startByte+lenBytes: the sum
	// overflows for a hostile lenBytes near math.MaxInt and the
	// bound check would silently pass.
	if lenBytes <= 0 || lenBytes > len(text)-startByte {
		lenBytes = len(text) - startByte
	}
	return spellcheck.Suggest(text, startByte, lenBytes)
}

// SpellLearn forwards to spellcheck.Learn. Caps word at
// maxSpellWordLen — words longer than this are not realistic
// dictionary entries.
func SpellLearn(word string) {
	spellcheck.Learn(truncateBytes(word, maxSpellWordLen))
}

// Beep plays the system alert sound. Thin forwarder to the sysbeep
// sub-package; no-op where the platform has no such sound.
func Beep() { sysbeep.Play() }

// BeepAvailable reports whether Beep is audible on this platform.
func BeepAvailable() bool { return sysbeep.Available() }

// soundEvents maps gui.SoundCue onto sysbeep.Event. Not a switch on
// the cue value with a default: gui adds cues over time, and a cue this
// table does not name is silent rather than wrong. The two enums are
// deliberately separate — sysbeep knows nothing about widgets, and gui
// knows nothing about system sounds (issue #469).
var soundEvents = map[gui.SoundCue]sysbeep.Event{
	gui.SoundClick:     sysbeep.EventClick,
	gui.SoundToggleOn:  sysbeep.EventToggleOn,
	gui.SoundToggleOff: sysbeep.EventToggleOff,
	gui.SoundSelection: sysbeep.EventSelection,
	gui.SoundError:     sysbeep.EventError,
	gui.SoundNotify:    sysbeep.EventNotify,
	gui.SoundOpen:      sysbeep.EventOpen,
	gui.SoundSuccess:   sysbeep.EventSuccess,
}

// PlaySystemSound plays the platform's system sound for cue. An
// unmapped cue is silent, not an error. Thin forwarder to sysbeep,
// like Beep above.
func PlaySystemSound(cue gui.SoundCue) {
	event, ok := soundEvents[cue]
	if !ok {
		return
	}
	sysbeep.PlayEvent(event)
}

// SystemSoundAvailable reports whether PlaySystemSound is audible on
// this platform.
func SystemSoundAvailable() bool { return sysbeep.EventAvailable() }
