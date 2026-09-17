//go:build windows

// Package printdialog provides native print dialog support for Windows.
package printdialog

import (
	"fmt"
	"syscall"
	"unsafe"

	"github.com/go-gui-org/go-gui/gui"
)

var (
	shell32win       = syscall.NewLazyDLL("shell32.dll")
	procShellExecute = shell32win.NewProc("ShellExecuteW")
)

// ShowPrintDialog prints a PDF via ShellExecute "print" verb.
// This opens the system-default PDF handler's print flow. The
// handler takes no options, so copies, duplex, color mode,
// orientation and page ranges from the job are ignored here.
func ShowPrintDialog(cfg gui.NativePrintParams) gui.PrintRunResult {
	if cfg.PDFPath == "" {
		return gui.PrintRunResult{
			Status:       gui.PrintRunError,
			ErrorCode:    "invalid_cfg",
			ErrorMessage: "no PDF path provided",
		}
	}

	verb, err := syscall.UTF16PtrFromString("print")
	if err != nil {
		return gui.PrintRunResult{
			Status:       gui.PrintRunError,
			ErrorCode:    "invalid_cfg",
			ErrorMessage: "invalid print verb",
		}
	}
	// A NUL byte truncates at conversion; reject instead of
	// printing the wrong file.
	file, err := syscall.UTF16PtrFromString(cfg.PDFPath)
	if err != nil {
		return gui.PrintRunResult{
			Status:       gui.PrintRunError,
			ErrorCode:    "invalid_cfg",
			ErrorMessage: "PDF path must not contain NUL",
		}
	}

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
			ErrorMessage: fmt.Sprintf("ShellExecute print failed (code %d)", ret),
		}
	}

	return gui.PrintRunResult{
		Status:  gui.PrintRunOK,
		PDFPath: cfg.PDFPath,
	}
}
