//go:build ios

package filedialog

import "github.com/go-gui-org/go-gui/gui"

// ShowOpenDialog is unsupported on iOS.
func ShowOpenDialog(_, _ string, _ []string,
	_ bool) gui.PlatformDialogResult {
	return unsupportedIOS()
}

// ShowSaveDialog is unsupported on iOS.
func ShowSaveDialog(_, _, _, _ string, _ []string,
	_ bool) gui.PlatformDialogResult {
	return unsupportedIOS()
}

// ShowFolderDialog is unsupported on iOS.
func ShowFolderDialog(_, _ string) gui.PlatformDialogResult {
	return unsupportedIOS()
}

// ShowMessageDialog is unsupported on iOS.
func ShowMessageDialog(_, _ string,
	_ gui.NativeAlertLevel) gui.NativeAlertResult {
	return gui.NativeAlertResult{
		Status:       gui.DialogError,
		ErrorCode:    "unsupported",
		ErrorMessage: "native dialogs not available on iOS",
	}
}

// ShowConfirmDialog is unsupported on iOS.
func ShowConfirmDialog(_, _ string,
	_ gui.NativeAlertLevel) gui.NativeAlertResult {
	return gui.NativeAlertResult{
		Status:       gui.DialogError,
		ErrorCode:    "unsupported",
		ErrorMessage: "native dialogs not available on iOS",
	}
}

// ShowSaveDiscardDialog is unsupported on iOS.
func ShowSaveDiscardDialog(_, _ string,
	_ gui.NativeAlertLevel) gui.NativeAlertResult {
	return gui.NativeAlertResult{
		Status:       gui.DialogError,
		ErrorCode:    "unsupported",
		ErrorMessage: "3-button save dialog not available on iOS",
	}
}

func unsupportedIOS() gui.PlatformDialogResult {
	return gui.PlatformDialogResult{
		Status:       gui.DialogError,
		ErrorCode:    "unsupported",
		ErrorMessage: "native dialogs not available on iOS",
	}
}
