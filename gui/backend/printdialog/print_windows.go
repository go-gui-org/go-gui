//go:build windows

// Package printdialog provides native print dialog support for Windows.
package printdialog

import (
	"syscall"
	"unsafe"

	"github.com/go-gui-org/go-gui/gui"
)

var (
	shell32win       = syscall.NewLazyDLL("shell32.dll")
	procShellExecute = shell32win.NewProc("ShellExecuteW")
)

// ShowPrintDialog prints a PDF via ShellExecute "print" verb.
// This opens the system-default PDF handler's print flow.
func ShowPrintDialog(cfg gui.NativePrintParams) gui.PrintRunResult {
	if cfg.PDFPath == "" {
		return gui.PrintRunResult{
			Status:       gui.PrintRunError,
			ErrorCode:    "invalid_cfg",
			ErrorMessage: "no PDF path provided",
		}
	}

	verb, _ := syscall.UTF16PtrFromString("print")
	file, _ := syscall.UTF16PtrFromString(cfg.PDFPath)

	ret, _, _ := procShellExecute.Call(
		0,                             // hwnd
		uintptr(unsafe.Pointer(verb)), // lpOperation
		uintptr(unsafe.Pointer(file)), // lpFile
		0,                             // lpParameters
		0,                             // lpDirectory
		0,                             // SW_HIDE
	)

	// ShellExecute returns > 32 on success.
	if ret <= 32 {
		return gui.PrintRunResult{
			Status:       gui.PrintRunError,
			ErrorCode:    "shell_execute",
			ErrorMessage: "ShellExecute print failed",
		}
	}

	return gui.PrintRunResult{
		Status:  gui.PrintRunOK,
		PDFPath: cfg.PDFPath,
	}
}
