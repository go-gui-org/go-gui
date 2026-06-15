//go:build darwin && !ios

package filedialog

import (
	"testing"

	"github.com/go-gui-org/go-gui/gui"
)

// C enum values from dialog_darwin.h — duplicated here because Go
// prohibits import "C" in _test.go files.
const (
	testDialogOK      = 0
	testDialogCancel  = 1
	testDialogError   = 2
	testDialogDiscard = 3
)

func TestBuildAlertResult(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name    string
		status  int
		errMsg  string
		want    gui.NativeDialogStatus
		wantErr string
	}{
		{"ok", testDialogOK, "", gui.DialogOK, ""},
		{"cancel", testDialogCancel, "", gui.DialogCancel, ""},
		{"error", testDialogError, "failed", gui.DialogError, "failed"},
		{"unknown status", 999, "", gui.DialogError, ""},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := buildAlertResult(tt.status, tt.errMsg)
			if result.Status != tt.want {
				t.Errorf("Status = %d, want %d",
					result.Status, tt.want)
			}
			if result.ErrorMessage != tt.wantErr {
				t.Errorf("ErrorMessage = %q, want %q",
					result.ErrorMessage, tt.wantErr)
			}
		})
	}
}

func TestBuildSaveDiscardResult(t *testing.T) {
	t.Parallel()
	tests := []struct {
		name   string
		status int
		errMsg string
		want   gui.NativeDialogStatus
	}{
		{"ok", testDialogOK, "", gui.DialogOK},
		{"discard", testDialogDiscard, "", gui.DialogDiscard},
		{"cancel", testDialogCancel, "", gui.DialogCancel},
		{"error with message", testDialogError, "fail", gui.DialogError},
		{"unknown status", 999, "", gui.DialogError},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := buildSaveDiscardResult(tt.status, tt.errMsg)
			if result.Status != tt.want {
				t.Errorf("Status = %d, want %d",
					result.Status, tt.want)
			}
			if tt.errMsg != "" && result.ErrorMessage != tt.errMsg {
				t.Errorf("ErrorMessage = %q, want %q",
					result.ErrorMessage, tt.errMsg)
			}
		})
	}
}

func TestBuildDialogResult(t *testing.T) {
	t.Parallel()
	t.Run("ok with paths", func(t *testing.T) {
		paths := []gui.PlatformPath{
			{Path: "/a.txt"},
			{Path: "/b.txt", BookmarkData: []byte{1, 2, 3}},
		}
		result := buildDialogResult(
			testDialogOK, paths, "")
		if result.Status != gui.DialogOK {
			t.Errorf("Status = %d, want %d",
				result.Status, gui.DialogOK)
		}
		if len(result.Paths) != 2 {
			t.Fatalf("Paths len = %d, want 2",
				len(result.Paths))
		}
		if result.Paths[0].Path != "/a.txt" {
			t.Errorf("Paths[0] = %q, want %q",
				result.Paths[0].Path, "/a.txt")
		}
		if len(result.Paths[1].BookmarkData) != 3 {
			t.Errorf("BookmarkData len = %d, want 3",
				len(result.Paths[1].BookmarkData))
		}
		if result.ErrorMessage != "" {
			t.Errorf("ErrorMessage = %q, want empty",
				result.ErrorMessage)
		}
	})

	t.Run("cancel no paths", func(t *testing.T) {
		result := buildDialogResult(
			testDialogCancel, nil, "")
		if result.Status != gui.DialogCancel {
			t.Errorf("Status = %d, want %d",
				result.Status, gui.DialogCancel)
		}
		if len(result.Paths) != 0 {
			t.Errorf("Paths len = %d, want 0",
				len(result.Paths))
		}
	})

	t.Run("error with message", func(t *testing.T) {
		result := buildDialogResult(
			testDialogError, nil, "dialog failed")
		if result.Status != gui.DialogError {
			t.Errorf("Status = %d, want %d",
				result.Status, gui.DialogError)
		}
		if result.ErrorMessage != "dialog failed" {
			t.Errorf("ErrorMessage = %q, want %q",
				result.ErrorMessage, "dialog failed")
		}
	})

	t.Run("unknown status", func(t *testing.T) {
		result := buildDialogResult(999, nil, "")
		if result.Status != gui.DialogError {
			t.Errorf("Status = %d, want %d",
				result.Status, gui.DialogError)
		}
	})
}
