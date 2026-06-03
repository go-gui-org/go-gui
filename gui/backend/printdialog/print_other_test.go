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
}
