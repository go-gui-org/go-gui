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

func TestShowSaveDiscardDialogNoPanic(t *testing.T) {
	t.Parallel()
	r := ShowSaveDiscardDialog("Document", "Save changes?", gui.AlertWarning)
	if r.Status != gui.DialogError {
		t.Errorf("Status: got %d, want %d", r.Status, gui.DialogError)
	}
	if r.ErrorCode != "unsupported" {
		t.Errorf("ErrorCode: got %q, want %q", r.ErrorCode, "unsupported")
	}
}

func TestShowOpenDialogWithParams(t *testing.T) {
	t.Parallel()
	r := ShowOpenDialog("Open File", "/home/user",
		[]string{"txt", "pdf"}, true)
	if r.Status != gui.DialogError {
		t.Errorf("Status: got %d, want %d", r.Status, gui.DialogError)
	}
	if r.ErrorCode != "unsupported" {
		t.Errorf("ErrorCode: got %q, want %q", r.ErrorCode, "unsupported")
	}
	if len(r.Paths) != 0 {
		t.Errorf("Paths: expected empty, got %v", r.Paths)
	}
}

func TestShowSaveDialogWithParams(t *testing.T) {
	t.Parallel()
	r := ShowSaveDialog("Save File", "/home/user",
		"document.txt", "txt", []string{"txt", "pdf"}, true)
	if r.Status != gui.DialogError {
		t.Errorf("Status: got %d, want %d", r.Status, gui.DialogError)
	}
	if r.ErrorCode != "unsupported" {
		t.Errorf("ErrorCode: got %q, want %q", r.ErrorCode, "unsupported")
	}
}

func TestShowFolderDialogWithParams(t *testing.T) {
	t.Parallel()
	r := ShowFolderDialog("Choose Folder", "/home/user")
	if r.Status != gui.DialogError {
		t.Errorf("Status: got %d, want %d", r.Status, gui.DialogError)
	}
	if r.ErrorCode != "unsupported" {
		t.Errorf("ErrorCode: got %q, want %q", r.ErrorCode, "unsupported")
	}
}

func TestShowMessageDialogWithParams(t *testing.T) {
	t.Parallel()
	r := ShowMessageDialog("Warning",
		"Something went wrong.", gui.AlertWarning)
	if r.Status != gui.DialogError {
		t.Errorf("Status: got %d, want %d", r.Status, gui.DialogError)
	}
	if r.ErrorCode != "unsupported" {
		t.Errorf("ErrorCode: got %q, want %q", r.ErrorCode, "unsupported")
	}
}

func TestShowConfirmDialogWithParams(t *testing.T) {
	t.Parallel()
	r := ShowConfirmDialog("Delete?", "This cannot be undone.",
		gui.AlertCritical)
	if r.Status != gui.DialogError {
		t.Errorf("Status: got %d, want %d", r.Status, gui.DialogError)
	}
	if r.ErrorCode != "unsupported" {
		t.Errorf("ErrorCode: got %q, want %q", r.ErrorCode, "unsupported")
	}
}
