//go:build linux

// Package printdialog provides native print dialog support for Linux.
package printdialog

import (
	"fmt"
	"os/exec"
	"sync"

	"github.com/go-gui-org/go-gui/gui"
)

type printTool int

const (
	printToolNone printTool = iota
	printToolLpr
	printToolXdgOpen
)

var detectedPrintTool printTool

var detectPrint = sync.OnceFunc(func() {
	if _, err := exec.LookPath("lpr"); err == nil {
		detectedPrintTool = printToolLpr
	} else if _, err := exec.LookPath("xdg-open"); err == nil {
		detectedPrintTool = printToolXdgOpen
	}
})

// ShowPrintDialog prints a PDF via lpr or opens it with xdg-open.
func ShowPrintDialog(cfg gui.NativePrintParams) gui.PrintRunResult {
	detectPrint()

	if cfg.PDFPath == "" {
		return gui.PrintRunResult{
			Status:       gui.PrintRunError,
			ErrorCode:    "invalid_cfg",
			ErrorMessage: "no PDF path provided",
		}
	}

	switch detectedPrintTool {
	case printToolLpr:
		return printViaLpr(cfg)
	case printToolXdgOpen:
		return printViaXdgOpen(cfg)
	default:
		return gui.PrintRunResult{
			Status:       gui.PrintRunError,
			ErrorCode:    "no_print_tool",
			ErrorMessage: "neither lpr nor xdg-open found in PATH",
		}
	}
}

func printViaLpr(cfg gui.NativePrintParams) gui.PrintRunResult {
	args := []string{}
	if cfg.Copies > 1 {
		args = append(args, fmt.Sprintf("-#%d", cfg.Copies))
	}
	if cfg.JobName != "" {
		args = append(args, "-T", cfg.JobName)
	}
	// Duplex options (CUPS).
	switch cfg.DuplexMode {
	case 2: // LongEdge
		args = append(args, "-o", "sides=two-sided-long-edge")
	case 3: // ShortEdge
		args = append(args, "-o", "sides=two-sided-short-edge")
	}
	// Color mode.
	if cfg.ColorMode == 2 { // Grayscale
		args = append(args, "-o", "ColorModel=Gray")
	}
	args = append(args, cfg.PDFPath)

	cmd := exec.Command("lpr", args...)
	if err := cmd.Run(); err != nil {
		return gui.PrintRunResult{
			Status:       gui.PrintRunError,
			ErrorCode:    "lpr_error",
			ErrorMessage: err.Error(),
		}
	}
	return gui.PrintRunResult{
		Status:  gui.PrintRunOK,
		PDFPath: cfg.PDFPath,
	}
}

func printViaXdgOpen(cfg gui.NativePrintParams) gui.PrintRunResult {
	cmd := exec.Command("xdg-open", cfg.PDFPath)
	if err := cmd.Start(); err != nil {
		return gui.PrintRunResult{
			Status:       gui.PrintRunError,
			ErrorCode:    "xdg_error",
			ErrorMessage: err.Error(),
		}
	}
	// Reap child to avoid zombie process.
	go cmd.Wait() //nolint:errcheck
	return gui.PrintRunResult{
		Status:  gui.PrintRunOK,
		PDFPath: cfg.PDFPath,
	}
}
