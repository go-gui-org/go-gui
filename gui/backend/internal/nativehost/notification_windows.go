//go:build windows

package nativehost

import (
	"strings"

	"github.com/go-gui-org/go-gui/gui"
	"golang.org/x/sys/windows"
)

func sendNotification(title, body string) gui.NativeNotificationResult {
	if strings.ContainsRune(title, 0) || strings.ContainsRune(body, 0) {
		return gui.NativeNotificationResult{Status: gui.NotificationError, ErrorCode: "invalid_cfg", ErrorMessage: "notification text contains a NUL character"}
	}
	if windows.RtlGetVersion().MajorVersion >= 10 {
		return sendWindowsToast(title, body)
	}
	return gui.NativeNotificationResult{Status: gui.NotificationError, ErrorCode: "unsupported", ErrorMessage: "Windows notifications require Windows 10 or later"}
}
