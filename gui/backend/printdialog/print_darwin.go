//go:build darwin && !ios

// Package printdialog provides native print dialog support.
package printdialog

/*
#cgo CFLAGS: -fobjc-arc
#cgo LDFLAGS: -framework AppKit -framework Quartz
#include "print_darwin.h"
#include <stdlib.h>
*/
import "C"

import (
	"unsafe"

	"github.com/go-gui-org/go-gui/gui"
)

// ShowPrintDialog shows the native macOS print dialog for a PDF.
func ShowPrintDialog(cfg gui.NativePrintParams) gui.PrintRunResult {
	cTitle := C.CString(cfg.Title)
	defer C.free(unsafe.Pointer(cTitle))
	cJobName := C.CString(cfg.JobName)
	defer C.free(unsafe.Pointer(cJobName))
	cPDFPath := C.CString(cfg.PDFPath)
	defer C.free(unsafe.Pointer(cPDFPath))
	cPageRanges := C.CString(cfg.PageRanges)
	defer C.free(unsafe.Pointer(cPageRanges))

	p := C.PrintParams{
		title:        cTitle,
		jobName:      cJobName,
		pdfPath:      cPDFPath,
		paperWidth:   C.double(cfg.PaperWidth),
		paperHeight:  C.double(cfg.PaperHeight),
		marginTop:    C.double(cfg.MarginTop),
		marginRight:  C.double(cfg.MarginRight),
		marginBottom: C.double(cfg.MarginBottom),
		marginLeft:   C.double(cfg.MarginLeft),
		orientation:  C.int(cfg.Orientation),
		copies:       C.int(cfg.Copies),
		pageRanges:   cPageRanges,
		duplexMode:   C.int(cfg.DuplexMode),
		colorMode:    C.int(cfg.ColorMode),
		scaleMode:    C.int(cfg.ScaleMode),
	}

	r := C.printdialogShow(p)
	defer C.printdialogFreeResult(r)

	var status gui.PrintRunStatus
	switch r.status {
	case C.PRINT_OK:
		status = gui.PrintRunOK
	case C.PRINT_CANCEL:
		status = gui.PrintRunCancel
	default:
		status = gui.PrintRunError
	}

	var errMsg string
	if r.errorMessage != nil {
		errMsg = C.GoString(r.errorMessage)
	}
	var pdfPath string
	if r.pdfPath != nil {
		pdfPath = C.GoString(r.pdfPath)
	}

	var errCode string
	if status == gui.PrintRunError {
		errCode = "print_error"
	}

	return gui.PrintRunResult{
		Status:       status,
		ErrorCode:    errCode,
		ErrorMessage: errMsg,
		PDFPath:      pdfPath,
	}
}
