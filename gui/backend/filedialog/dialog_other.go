//go:build !darwin && !linux && !windows

package filedialog

import "github.com/go-gui-org/go-gui/gui"

// ShowOpenDialog is unsupported on non-macOS platforms.
func ShowOpenDialog(_, _ string, _ []string,
	_ bool) gui.PlatformDialogResult {
	return unsupported()
}

// ShowSaveDialog is unsupported on non-macOS platforms.
func ShowSaveDialog(_, _, _, _ string, _ []string,
	_ bool) gui.PlatformDialogResult {
	return unsupported()
}

// ShowFolderDialog is unsupported on non-macOS platforms.
func ShowFolderDialog(_, _ string) gui.PlatformDialogResult {
	return unsupported()
}

// ShowMessageDialog is unsupported on non-macOS platforms.
func ShowMessageDialog(_, _ string,
	_ gui.NativeAlertLevel) gui.NativeAlertResult {
	return gui.NativeAlertResult{
		Status:       gui.DialogError,
		ErrorCode:    "unsupported",
		ErrorMessage: "native dialogs not available",
	}
}

// ShowConfirmDialog is unsupported on non-macOS platforms.
func ShowConfirmDialog(_, _ string,
	_ gui.NativeAlertLevel) gui.NativeAlertResult {
	return gui.NativeAlertResult{
		Status:       gui.DialogError,
		ErrorCode:    "unsupported",
		ErrorMessage: "native dialogs not available",
	}
}

// ShowSaveDiscardDialog is unsupported on non-macOS platforms.
func ShowSaveDiscardDialog(_, _ string,
	_ gui.NativeAlertLevel) gui.NativeAlertResult {
	return gui.NativeAlertResult{
		Status:       gui.DialogError,
		ErrorCode:    "unsupported",
		ErrorMessage: "3-button save dialog not available",
	}
}

func unsupported() gui.PlatformDialogResult {
	return gui.PlatformDialogResult{
		Status:       gui.DialogError,
		ErrorCode:    "unsupported",
		ErrorMessage: "native dialogs not available",
	}
}
