//go:build !darwin && !linux && !windows

package filedialog

import (
	"testing"

	"github.com/go-gui-org/go-gui/gui"
)

func TestShowOpenDialogNoPanic(t *testing.T) {
	t.Parallel()
	r := ShowOpenDialog("", "", nil, false)
	if r.Status != gui.DialogError {
		t.Errorf("Status: got %d, want %d", r.Status, gui.DialogError)
	}
	if r.ErrorCode != "unsupported" {
		t.Errorf("ErrorCode: got %q, want %q", r.ErrorCode, "unsupported")
	}
}

func TestShowSaveDialogNoPanic(t *testing.T) {
	t.Parallel()
	r := ShowSaveDialog("", "", "", "", nil, false)
	if r.Status != gui.DialogError {
		t.Errorf("Status: got %d, want %d", r.Status, gui.DialogError)
	}
	if r.ErrorCode != "unsupported" {
		t.Errorf("ErrorCode: got %q, want %q", r.ErrorCode, "unsupported")
	}
}

func TestShowFolderDialogNoPanic(t *testing.T) {
	t.Parallel()
	r := ShowFolderDialog("", "")
	if r.Status != gui.DialogError {
		t.Errorf("Status: got %d, want %d", r.Status, gui.DialogError)
	}
	if r.ErrorCode != "unsupported" {
		t.Errorf("ErrorCode: got %q, want %q", r.ErrorCode, "unsupported")
	}
}

func TestShowMessageDialogNoPanic(t *testing.T) {
	t.Parallel()
	r := ShowMessageDialog("", "", gui.AlertInfo)
	if r.Status != gui.DialogError {
		t.Errorf("Status: got %d, want %d", r.Status, gui.DialogError)
	}
	if r.ErrorCode != "unsupported" {
		t.Errorf("ErrorCode: got %q, want %q", r.ErrorCode, "unsupported")
	}
}

func TestShowConfirmDialogNoPanic(t *testing.T) {
	t.Parallel()
	r := ShowConfirmDialog("", "", gui.AlertInfo)
	if r.Status != gui.DialogError {
		t.Errorf("Status: got %d, want %d", r.Status, gui.DialogError)
	}
	if r.ErrorCode != "unsupported" {
		t.Errorf("ErrorCode: got %q, want %q", r.ErrorCode, "unsupported")
	}
}
