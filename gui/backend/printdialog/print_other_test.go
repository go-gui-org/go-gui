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
		Title:       "Print Document",
		JobName:     "document.pdf",
		Copies:      1,
		PaperWidth:  612,
		PaperHeight: 792,
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

func TestShowPrintDialogWithOrientation(t *testing.T) {
	t.Parallel()
	r := ShowPrintDialog(gui.NativePrintParams{
		Title:       "Test",
		Orientation: 1,
	})
	if r.Status != gui.PrintRunError {
		t.Errorf("Status: got %d, want %d", r.Status, gui.PrintRunError)
	}
}
