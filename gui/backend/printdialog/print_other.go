//go:build !darwin && !linux && !windows

package printdialog

import "github.com/go-gui-org/go-gui/gui"

// ShowPrintDialog is unsupported on this platform.
func ShowPrintDialog(_ gui.NativePrintParams) gui.PrintRunResult {
	return gui.PrintRunResult{
		Status:       gui.PrintRunError,
		ErrorCode:    "unsupported",
		ErrorMessage: "native print dialog not available on this platform",
	}
}
