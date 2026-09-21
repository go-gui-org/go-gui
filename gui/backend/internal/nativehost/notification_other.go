//go:build !windows

package nativehost

import (
	"os/exec"
	"runtime"

	"github.com/go-gui-org/go-gui/gui"
)

// #nosec G204 -- length-capped; -- separates Linux arguments from flags.
func sendNotification(title, body string) gui.NativeNotificationResult {
	var cmd *exec.Cmd

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
