//go:build !darwin && !linux && !windows

package printdialog

import (
	"testing"

	"github.com/go-gui-org/go-gui/gui"
)

func TestShowPrintDialogNoPanic(t *testing.T) {
	t.Parallel()
	r := ShowPrintDialog(gui.NativePrintParams{})
	if r.Status != gui.PrintRunError {
		t.Errorf("Status: got %d, want %d", r.Status, gui.PrintRunError)
	}
	if r.ErrorCode != "unsupported" {
		t.Errorf("ErrorCode: got %q, want %q", r.ErrorCode, "unsupported")
	}
	if r.ErrorMessage == "" {
		t.Error("expected non-empty ErrorMessage")
	}
}

func TestShowPrintDialogWithParams(t *testing.T) {
	t.Parallel()
	r := ShowPrintDialog(gui.NativePrintParams{
		Title:     "Print Document",
		NumPages:  3,
		Copies:    1,
		RasterDPI: 300,
		PDFData:   []byte("%PDF-1.4 fake"),
	})
	if r.Status != gui.PrintRunError {
		t.Errorf("Status: got %d, want %d", r.Status, gui.PrintRunError)
	}
	if r.ErrorCode != "unsupported" {
		t.Errorf("ErrorCode: got %q, want %q", r.ErrorCode, "unsupported")
	}
}

func TestShowPrintDialogZeroCopies(t *testing.T) {
	t.Parallel()
	r := ShowPrintDialog(gui.NativePrintParams{Copies: 0})
	if r.Status != gui.PrintRunError {
		t.Errorf("Status: got %d, want %d", r.Status, gui.PrintRunError)
	}
}

func TestShowPrintDialogMinDPI(t *testing.T) {
	t.Parallel()
	r := ShowPrintDialog(gui.NativePrintParams{
		Title:     "Test",
		RasterDPI: 72,
	})
	if r.Status != gui.PrintRunError {
		t.Errorf("Status: got %d, want %d", r.Status, gui.PrintRunError)
	}
}
