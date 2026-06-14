//go:build linux

// Package filedialog provides native file dialog support for Linux.
package filedialog

import (
	"os/exec"
	"path/filepath"
	"strings"
	"sync"

	"github.com/go-gui-org/go-gui/gui"
)

type dialogTool int

const (
	toolNone dialogTool = iota
	toolZenity
	toolKdialog
)

var detectedTool dialogTool

var detectDialogTool = sync.OnceFunc(func() {
	if _, err := exec.LookPath("zenity"); err == nil {
		detectedTool = toolZenity
	} else if _, err := exec.LookPath("kdialog"); err == nil {
		detectedTool = toolKdialog
	}
})

// ShowOpenDialog shows a file-open dialog via zenity or kdialog.
func ShowOpenDialog(title, startDir string, extensions []string,
	allowMultiple bool) gui.PlatformDialogResult {

	detectDialogTool()
	var args []string

	switch detectedTool {
	case toolZenity:
		args = []string{"zenity", "--file-selection", "--title", title}
		if startDir != "" {
			args = append(args, "--filename",
				ensureTrailingSlash(startDir))
		}
		if allowMultiple {
			args = append(args, "--multiple", "--separator", "|")
		}
		if f := zenityFilter(extensions); f != "" {
			args = append(args, "--file-filter", f)
		}

	case toolKdialog:
		verb := "--getopenfilename"
		if allowMultiple {
			verb = "--getopenfilename-multiple"
		}
		args = []string{"kdialog", verb, startDirOrDot(startDir)}
		if f := kdialogFilter(extensions); f != "" {
			args = append(args, f)
		}
		args = append(args, "--title", title)

	default:
		return noTool()
	}

	out, status, err := runDialog(args)
	if err != nil || status != gui.DialogOK {
		return gui.PlatformDialogResult{
			Status: status, ErrorMessage: errStr(err),
		}
	}

	var paths []gui.PlatformPath
	if detectedTool == toolZenity {
		paths = parsePaths(out, "|")
	} else {
		paths = parsePaths(out, "\n")
	}
	return gui.PlatformDialogResult{Status: gui.DialogOK, Paths: paths}
}

// ShowSaveDialog shows a file-save dialog via zenity or kdialog.
func ShowSaveDialog(title, startDir, defaultName, _ string,
	extensions []string, confirmOverwrite bool) gui.PlatformDialogResult {

	detectDialogTool()
	var args []string
	startFile := filepath.Join(startDirOrDot(startDir), defaultName)

	switch detectedTool {
	case toolZenity:
		args = []string{"zenity", "--file-selection", "--save",
			"--title", title, "--filename", startFile}
		if confirmOverwrite {
			args = append(args, "--confirm-overwrite")
		}
		if f := zenityFilter(extensions); f != "" {
			args = append(args, "--file-filter", f)
		}

	case toolKdialog:
		args = []string{"kdialog", "--getsavefilename", startFile}
		if f := kdialogFilter(extensions); f != "" {
			args = append(args, f)
		}
		args = append(args, "--title", title)

	default:
		return noTool()
	}

	out, status, err := runDialog(args)
	if err != nil || status != gui.DialogOK {
		return gui.PlatformDialogResult{
			Status: status, ErrorMessage: errStr(err),
		}
	}
	return gui.PlatformDialogResult{
		Status: gui.DialogOK,
		Paths:  []gui.PlatformPath{{Path: strings.TrimSpace(out)}},
	}
}

// ShowFolderDialog shows a folder picker via zenity or kdialog.
func ShowFolderDialog(title, startDir string) gui.PlatformDialogResult {
	detectDialogTool()
	var args []string

	switch detectedTool {
	case toolZenity:
		args = []string{"zenity", "--file-selection", "--directory",
			"--title", title}
		if startDir != "" {
			args = append(args, "--filename",
				ensureTrailingSlash(startDir))
		}

	case toolKdialog:
		args = []string{"kdialog", "--getexistingdirectory",
			startDirOrDot(startDir), "--title", title}

	default:
		return noTool()
	}

	out, status, err := runDialog(args)
	if err != nil || status != gui.DialogOK {
		return gui.PlatformDialogResult{
			Status: status, ErrorMessage: errStr(err),
		}
	}
	return gui.PlatformDialogResult{
		Status: gui.DialogOK,
		Paths:  []gui.PlatformPath{{Path: strings.TrimSpace(out)}},
	}
}

// ShowMessageDialog shows an informational message via zenity or kdialog.
func ShowMessageDialog(title, body string,
	level gui.NativeAlertLevel) gui.NativeAlertResult {

	detectDialogTool()
	var args []string

	switch detectedTool {
	case toolZenity:
		args = []string{"zenity", zenityAlertFlag(level),
			"--title", title, "--text", body}

	case toolKdialog:
		args = []string{"kdialog", kdialogMsgFlag(level),
			body, "--title", title}

	default:
		return noToolAlert()
	}

	_, status, err := runDialog(args)
	if err != nil {
		return gui.NativeAlertResult{
			Status: gui.DialogError, ErrorMessage: err.Error(),
		}
	}
	return gui.NativeAlertResult{Status: status}
}

// ShowConfirmDialog shows an OK/Cancel confirmation via zenity or kdialog.
func ShowConfirmDialog(title, body string,
	_ gui.NativeAlertLevel) gui.NativeAlertResult {

	detectDialogTool()
	var args []string

	switch detectedTool {
	case toolZenity:
		args = []string{"zenity", "--question",
			"--title", title, "--text", body}

	case toolKdialog:
		args = []string{"kdialog", "--yesno",
			body, "--title", title}

	default:
		return noToolAlert()
	}

	_, status, err := runDialog(args)
	if err != nil {
		return gui.NativeAlertResult{
			Status: gui.DialogError, ErrorMessage: err.Error(),
		}
	}
	return gui.NativeAlertResult{Status: status}
}

// --- helpers ---

func runDialog(args []string) (string, gui.NativeDialogStatus, error) {
	// #nosec G204 — binary set by code (zenity/kdialog), no shell, argv passed directly to execve
	cmd := exec.Command(args[0], args[1:]...)
	out, err := cmd.Output()

	if err == nil {
		return string(out), gui.DialogOK, nil
	}
	if exitErr, ok := err.(*exec.ExitError); ok {
		if exitErr.ExitCode() == 1 {
			return "", gui.DialogCancel, nil
		}
	}
	return "", gui.DialogError, err
}

func parsePaths(output, sep string) []gui.PlatformPath {
	raw := strings.TrimSpace(output)
	if raw == "" {
		return nil
	}
	parts := strings.Split(raw, sep)
	paths := make([]gui.PlatformPath, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" {
			paths = append(paths, gui.PlatformPath{Path: p})
		}
	}
	return paths
}

func zenityFilter(extensions []string) string {
	if len(extensions) == 0 {
		return ""
	}
	globs := make([]string, len(extensions))
	for i, ext := range extensions {
		globs[i] = "*." + ext
	}
	return "Files | " + strings.Join(globs, " ")
}

func kdialogFilter(extensions []string) string {
	if len(extensions) == 0 {
		return ""
	}
	globs := make([]string, len(extensions))
	for i, ext := range extensions {
		globs[i] = "*." + ext
	}
	return strings.Join(globs, " ")
}

func zenityAlertFlag(level gui.NativeAlertLevel) string {
	switch level {
	case gui.AlertWarning:
		return "--warning"
	case gui.AlertCritical:
		return "--error"
	default:
		return "--info"
	}
}

func kdialogMsgFlag(level gui.NativeAlertLevel) string {
	switch level {
	case gui.AlertWarning:
		return "--sorry"
	case gui.AlertCritical:
		return "--error"
	default:
		return "--msgbox"
	}
}

func ensureTrailingSlash(dir string) string {
	if dir != "" && !strings.HasSuffix(dir, "/") {
		return dir + "/"
	}
	return dir
}

func startDirOrDot(dir string) string {
	if dir == "" {
		return "."
	}
	return dir
}

func errStr(err error) string {
	if err != nil {
		return err.Error()
	}
	return ""
}

func noTool() gui.PlatformDialogResult {
	return gui.PlatformDialogResult{
		Status:       gui.DialogError,
		ErrorCode:    "no_dialog_tool",
		ErrorMessage: "neither zenity nor kdialog found in PATH",
	}
}

func noToolAlert() gui.NativeAlertResult {
	return gui.NativeAlertResult{
		Status:       gui.DialogError,
		ErrorCode:    "no_dialog_tool",
		ErrorMessage: "neither zenity nor kdialog found in PATH",
	}
}

// ShowSaveDiscardDialog shows a Save/Discard/Cancel dialog.
// Not available on Linux; callers should handle DialogError.
func ShowSaveDiscardDialog(_, _ string,
	_ gui.NativeAlertLevel) gui.NativeAlertResult {
	return gui.NativeAlertResult{
		Status:       gui.DialogError,
		ErrorCode:    "unsupported",
		ErrorMessage: "3-button save dialog not available on Linux",
	}
}
