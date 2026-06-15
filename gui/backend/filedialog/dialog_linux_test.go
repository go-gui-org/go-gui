//go:build linux

package filedialog

import (
	"errors"
	"testing"

	"github.com/go-gui-org/go-gui/gui"
)

func TestParsePaths(t *testing.T) {
	tests := []struct {
		name     string
		output   string
		sep      string
		wantLen  int
		wantPath string // first path
	}{
		{"pipe separator", "a.txt|b.txt|c.txt", "|", 3, "a.txt"},
		{"newline separator", "a.txt\nb.txt\nc.txt", "\n", 3, "a.txt"},
		{"empty string", "", "|", 0, ""},
		{"whitespace only", "   ", "|", 0, ""},
		{"trailing whitespace", "a.txt \n b.txt ", "\n", 2, "a.txt"},
		{"empty segments", "a.txt||b.txt", "|", 2, "a.txt"},
		{"single path", "file.txt", "|", 1, "file.txt"},
		{"leading/trailing newline", "\na.txt\nb.txt\n", "\n", 2, "a.txt"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			paths := parsePaths(tt.output, tt.sep)
			if len(paths) != tt.wantLen {
				t.Errorf("len = %d, want %d", len(paths), tt.wantLen)
			}
			if tt.wantLen > 0 && paths[0].Path != tt.wantPath {
				t.Errorf("first path = %q, want %q",
					paths[0].Path, tt.wantPath)
			}
		})
	}
}

func TestZenityFilter(t *testing.T) {
	tests := []struct {
		name       string
		extensions []string
		want       string
	}{
		{"two extensions", []string{"txt", "pdf"}, "Files | *.txt *.pdf"},
		{"single extension", []string{"txt"}, "Files | *.txt"},
		{"nil", nil, ""},
		{"empty slice", []string{}, ""},
		{"with dots", []string{"tar.gz"}, "Files | *.tar.gz"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := zenityFilter(tt.extensions)
			if got != tt.want {
				t.Errorf("zenityFilter(%v) = %q, want %q",
					tt.extensions, got, tt.want)
			}
		})
	}
}

func TestKdialogFilter(t *testing.T) {
	tests := []struct {
		name       string
		extensions []string
		want       string
	}{
		{"two extensions", []string{"txt", "pdf"}, "*.txt *.pdf"},
		{"single extension", []string{"txt"}, "*.txt"},
		{"nil", nil, ""},
		{"empty slice", []string{}, ""},
		{"with dots", []string{"tar.gz"}, "*.tar.gz"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := kdialogFilter(tt.extensions)
			if got != tt.want {
				t.Errorf("kdialogFilter(%v) = %q, want %q",
					tt.extensions, got, tt.want)
			}
		})
	}
}

func TestZenityAlertFlag(t *testing.T) {
	tests := []struct {
		name  string
		level gui.NativeAlertLevel
		want  string
	}{
		{"warning", gui.AlertWarning, "--warning"},
		{"critical", gui.AlertCritical, "--error"},
		{"info", gui.AlertInfo, "--info"},
		{"zero value", 0, "--info"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := zenityAlertFlag(tt.level)
			if got != tt.want {
				t.Errorf("zenityAlertFlag(%d) = %q, want %q",
					tt.level, got, tt.want)
			}
		})
	}
}

func TestKdialogMsgFlag(t *testing.T) {
	tests := []struct {
		name  string
		level gui.NativeAlertLevel
		want  string
	}{
		{"warning", gui.AlertWarning, "--sorry"},
		{"critical", gui.AlertCritical, "--error"},
		{"info", gui.AlertInfo, "--msgbox"},
		{"zero value", 0, "--msgbox"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := kdialogMsgFlag(tt.level)
			if got != tt.want {
				t.Errorf("kdialogMsgFlag(%d) = %q, want %q",
					tt.level, got, tt.want)
			}
		})
	}
}

func TestEnsureTrailingSlash(t *testing.T) {
	tests := []struct {
		name string
		dir  string
		want string
	}{
		{"no slash", "/home/user", "/home/user/"},
		{"has slash", "/home/user/", "/home/user/"},
		{"empty", "", ""},
		{"root", "/", "/"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ensureTrailingSlash(tt.dir)
			if got != tt.want {
				t.Errorf("ensureTrailingSlash(%q) = %q, want %q",
					tt.dir, got, tt.want)
			}
		})
	}
}

func TestStartDirOrDot(t *testing.T) {
	tests := []struct {
		name string
		dir  string
		want string
	}{
		{"provided", "/home/user", "/home/user"},
		{"empty", "", "."},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := startDirOrDot(tt.dir)
			if got != tt.want {
				t.Errorf("startDirOrDot(%q) = %q, want %q",
					tt.dir, got, tt.want)
			}
		})
	}
}

func TestErrStr(t *testing.T) {
	if got := errStr(errors.New("fail")); got != "fail" {
		t.Errorf("errStr(error) = %q, want %q", got, "fail")
	}
	if got := errStr(nil); got != "" {
		t.Errorf("errStr(nil) = %q, want %q", got, "")
	}
}

func TestNoTool(t *testing.T) {
	r := noTool()
	if r.Status != gui.DialogError {
		t.Errorf("Status = %d, want %d", r.Status, gui.DialogError)
	}
	if r.ErrorCode != "no_dialog_tool" {
		t.Errorf("ErrorCode = %q, want %q", r.ErrorCode, "no_dialog_tool")
	}
	if r.ErrorMessage == "" {
		t.Error("ErrorMessage should not be empty")
	}
}

func TestNoToolAlert(t *testing.T) {
	r := noToolAlert()
	if r.Status != gui.DialogError {
		t.Errorf("Status = %d, want %d", r.Status, gui.DialogError)
	}
	if r.ErrorCode != "no_dialog_tool" {
		t.Errorf("ErrorCode = %q, want %q", r.ErrorCode, "no_dialog_tool")
	}
	if r.ErrorMessage == "" {
		t.Error("ErrorMessage should not be empty")
	}
}

// consumeDetectOnce fires detectDialogTool once so the sync.OnceFunc
// is spent, then pins detectedTool to toolNone. Subsequent dialog
// calls take the "no tool available" branch, which is testable
// without zenity/kdialog installed.
func consumeDetectOnce() {
	detectDialogTool()
	detectedTool = toolNone
}

func TestShowOpenDialogNoTool(t *testing.T) {
	consumeDetectOnce()
	r := ShowOpenDialog("Open", "/tmp", nil, false)
	if r.Status != gui.DialogError {
		t.Errorf("Status = %d, want %d", r.Status, gui.DialogError)
	}
	if r.ErrorCode != "no_dialog_tool" {
		t.Errorf("ErrorCode = %q, want %q", r.ErrorCode, "no_dialog_tool")
	}
}

func TestShowSaveDialogNoTool(t *testing.T) {
	consumeDetectOnce()
	r := ShowSaveDialog("Save", "/tmp", "file.txt", "txt", nil, false)
	if r.Status != gui.DialogError {
		t.Errorf("Status = %d, want %d", r.Status, gui.DialogError)
	}
	if r.ErrorCode != "no_dialog_tool" {
		t.Errorf("ErrorCode = %q, want %q", r.ErrorCode, "no_dialog_tool")
	}
}

func TestShowFolderDialogNoTool(t *testing.T) {
	consumeDetectOnce()
	r := ShowFolderDialog("Folder", "/tmp")
	if r.Status != gui.DialogError {
		t.Errorf("Status = %d, want %d", r.Status, gui.DialogError)
	}
	if r.ErrorCode != "no_dialog_tool" {
		t.Errorf("ErrorCode = %q, want %q", r.ErrorCode, "no_dialog_tool")
	}
}

func TestShowMessageDialogNoTool(t *testing.T) {
	consumeDetectOnce()
	r := ShowMessageDialog("Msg", "body", gui.AlertInfo)
	if r.Status != gui.DialogError {
		t.Errorf("Status = %d, want %d", r.Status, gui.DialogError)
	}
	if r.ErrorCode != "no_dialog_tool" {
		t.Errorf("ErrorCode = %q, want %q", r.ErrorCode, "no_dialog_tool")
	}
}

func TestShowConfirmDialogNoTool(t *testing.T) {
	consumeDetectOnce()
	r := ShowConfirmDialog("Confirm", "body", gui.AlertInfo)
	if r.Status != gui.DialogError {
		t.Errorf("Status = %d, want %d", r.Status, gui.DialogError)
	}
	if r.ErrorCode != "no_dialog_tool" {
		t.Errorf("ErrorCode = %q, want %q", r.ErrorCode, "no_dialog_tool")
	}
}

func TestShowSaveDiscardDialogNoTool(t *testing.T) {
	consumeDetectOnce()
	r := ShowSaveDiscardDialog("Save?", "body", gui.AlertWarning)
	if r.Status != gui.DialogError {
		t.Errorf("Status = %d, want %d", r.Status, gui.DialogError)
	}
	if r.ErrorCode != "no_dialog_tool" {
		t.Errorf("ErrorCode = %q, want %q", r.ErrorCode, "no_dialog_tool")
	}
}
