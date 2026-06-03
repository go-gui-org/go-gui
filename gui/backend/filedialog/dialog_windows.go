//go:build windows

// Package filedialog provides native file dialog support for Windows.
package filedialog

import (
	"path/filepath"
	"strings"
	"syscall"
	"unicode/utf16"
	"unsafe"

	"github.com/go-gui-org/go-gui/gui"
)

var (
	comdlg32                = syscall.NewLazyDLL("comdlg32.dll")
	shell32                 = syscall.NewLazyDLL("shell32.dll")
	user32                  = syscall.NewLazyDLL("user32.dll")
	ole32                   = syscall.NewLazyDLL("ole32.dll")
	procGetOpenFileNameW    = comdlg32.NewProc("GetOpenFileNameW")
	procGetSaveFileNameW    = comdlg32.NewProc("GetSaveFileNameW")
	procSHBrowseForFolderW  = shell32.NewProc("SHBrowseForFolderW")
	procSHGetPathFromIDList = shell32.NewProc("SHGetPathFromIDListW")
	procCoTaskMemFree       = ole32.NewProc("CoTaskMemFree")
	procMessageBoxW         = user32.NewProc("MessageBoxW")
)

// OPENFILENAME flags.
const (
	ofnAllowMultiSelect = 0x00000200
	ofnExplorer         = 0x00080000
	ofnFileMustExist    = 0x00001000
	ofnPathMustExist    = 0x00000800
	ofnOverwritePrompt  = 0x00000002
	ofnNoChangeDir      = 0x00000008
)

// MessageBox flags and return values.
const (
	mbOK          = 0x00000000
	mbOKCancel    = 0x00000001
	mbIconInfo    = 0x00000040
	mbIconWarning = 0x00000030
	mbIconError   = 0x00000010
	idCancel      = 2
)

// BROWSEINFO flags.
const (
	bifReturnOnlyFSDirs = 0x00000001
	bifNewDialogStyle   = 0x00000040
)

const winMaxPath = 260

// openFilenameW mirrors the Windows OPENFILENAMEW structure.
type openFilenameW struct {
	structSize    uint32
	owner         uintptr
	instance      uintptr
	filter        *uint16
	customFilter  *uint16
	maxCustFilter uint32
	filterIndex   uint32
	file          *uint16
	maxFile       uint32
	fileTitle     *uint16
	maxFileTitle  uint32
	initialDir    *uint16
	title         *uint16
	flags         uint32
	fileOffset    uint16
	fileExtension uint16
	defExt        *uint16
	custData      uintptr
	hook          uintptr
	templateName  *uint16
	pvReserved    uintptr
	dwReserved    uint32
	flagsEx       uint32
}

// browseInfoW mirrors the Windows BROWSEINFOW structure.
type browseInfoW struct {
	owner       uintptr
	root        uintptr
	displayName *uint16
	title       *uint16
	flags       uint32
	callback    uintptr
	lParam      uintptr
	image       int32
}

const fileBufferSize = 65536

// ShowOpenDialog shows a Win32 file-open dialog.
func ShowOpenDialog(title, startDir string, extensions []string,
	allowMultiple bool) gui.PlatformDialogResult {

	buf := make([]uint16, fileBufferSize)
	titlePtr, _ := syscall.UTF16PtrFromString(title)

	var initialDirPtr *uint16
	if startDir != "" {
		initialDirPtr, _ = syscall.UTF16PtrFromString(startDir)
	}

	flags := uint32(ofnExplorer | ofnFileMustExist |
		ofnPathMustExist | ofnNoChangeDir)
	if allowMultiple {
		flags |= ofnAllowMultiSelect
	}

	ofn := openFilenameW{
		structSize: uint32(unsafe.Sizeof(openFilenameW{})),
		filter:     buildFilter(extensions),
		file:       &buf[0],
		maxFile:    uint32(len(buf)),
		initialDir: initialDirPtr,
		title:      titlePtr,
		flags:      flags,
	}

	r, _, _ := procGetOpenFileNameW.Call(
		uintptr(unsafe.Pointer(&ofn)))
	if r == 0 {
		return gui.PlatformDialogResult{Status: gui.DialogCancel}
	}

	paths := parseMultiSelect(buf)
	result := make([]gui.PlatformPath, len(paths))
	for i, p := range paths {
		result[i] = gui.PlatformPath{Path: p}
	}
	return gui.PlatformDialogResult{
		Status: gui.DialogOK, Paths: result,
	}
}

// ShowSaveDialog shows a Win32 file-save dialog.
func ShowSaveDialog(title, startDir, defaultName, defaultExt string,
	extensions []string,
	confirmOverwrite bool) gui.PlatformDialogResult {

	buf := make([]uint16, fileBufferSize)
	titlePtr, _ := syscall.UTF16PtrFromString(title)

	if defaultName != "" {
		name, _ := syscall.UTF16FromString(defaultName)
		copy(buf, name)
	}

	var initialDirPtr *uint16
	if startDir != "" {
		initialDirPtr, _ = syscall.UTF16PtrFromString(startDir)
	}

	var defExtPtr *uint16
	if defaultExt != "" {
		defExtPtr, _ = syscall.UTF16PtrFromString(
			strings.TrimPrefix(defaultExt, "."))
	}

	flags := uint32(ofnExplorer | ofnPathMustExist | ofnNoChangeDir)
	if confirmOverwrite {
		flags |= ofnOverwritePrompt
	}

	ofn := openFilenameW{
		structSize: uint32(unsafe.Sizeof(openFilenameW{})),
		filter:     buildFilter(extensions),
		file:       &buf[0],
		maxFile:    uint32(len(buf)),
		initialDir: initialDirPtr,
		title:      titlePtr,
		flags:      flags,
		defExt:     defExtPtr,
	}

	r, _, _ := procGetSaveFileNameW.Call(
		uintptr(unsafe.Pointer(&ofn)))
	if r == 0 {
		return gui.PlatformDialogResult{Status: gui.DialogCancel}
	}

	path := syscall.UTF16ToString(buf[:])
	return gui.PlatformDialogResult{
		Status: gui.DialogOK,
		Paths:  []gui.PlatformPath{{Path: path}},
	}
}

// ShowFolderDialog shows a Win32 folder picker.
func ShowFolderDialog(title, _ string) gui.PlatformDialogResult {
	titlePtr, _ := syscall.UTF16PtrFromString(title)
	displayName := make([]uint16, winMaxPath)

	bi := browseInfoW{
		displayName: &displayName[0],
		title:       titlePtr,
		flags:       bifReturnOnlyFSDirs | bifNewDialogStyle,
	}

	pidl, _, _ := procSHBrowseForFolderW.Call(
		uintptr(unsafe.Pointer(&bi)))
	if pidl == 0 {
		return gui.PlatformDialogResult{Status: gui.DialogCancel}
	}
	defer procCoTaskMemFree.Call(pidl)

	pathBuf := make([]uint16, winMaxPath)
	procSHGetPathFromIDList.Call(
		pidl, uintptr(unsafe.Pointer(&pathBuf[0])))
	path := syscall.UTF16ToString(pathBuf)

	return gui.PlatformDialogResult{
		Status: gui.DialogOK,
		Paths:  []gui.PlatformPath{{Path: path}},
	}
}

// ShowMessageDialog shows a Win32 message box.
func ShowMessageDialog(title, body string,
	level gui.NativeAlertLevel) gui.NativeAlertResult {

	titlePtr, _ := syscall.UTF16PtrFromString(title)
	bodyPtr, _ := syscall.UTF16PtrFromString(body)

	procMessageBoxW.Call(
		0,
		uintptr(unsafe.Pointer(bodyPtr)),
		uintptr(unsafe.Pointer(titlePtr)),
		uintptr(mbOK|alertIcon(level)))

	return gui.NativeAlertResult{Status: gui.DialogOK}
}

// ShowConfirmDialog shows a Win32 OK/Cancel message box.
func ShowConfirmDialog(title, body string,
	level gui.NativeAlertLevel) gui.NativeAlertResult {

	titlePtr, _ := syscall.UTF16PtrFromString(title)
	bodyPtr, _ := syscall.UTF16PtrFromString(body)

	r, _, _ := procMessageBoxW.Call(
		0,
		uintptr(unsafe.Pointer(bodyPtr)),
		uintptr(unsafe.Pointer(titlePtr)),
		uintptr(mbOKCancel|alertIcon(level)))

	if r == idCancel {
		return gui.NativeAlertResult{Status: gui.DialogCancel}
	}
	return gui.NativeAlertResult{Status: gui.DialogOK}
}

// ShowSaveDiscardDialog shows a Save/Discard/Cancel dialog.
// Not available on Windows via MessageBox; returns DialogError.
func ShowSaveDiscardDialog(_, _ string,
	_ gui.NativeAlertLevel) gui.NativeAlertResult {
	return gui.NativeAlertResult{
		Status:       gui.DialogError,
		ErrorCode:    "unsupported",
		ErrorMessage: "3-button save dialog not available on Windows",
	}
}

// --- helpers ---

func alertIcon(level gui.NativeAlertLevel) uint32 {
	switch level {
	case gui.AlertWarning:
		return mbIconWarning
	case gui.AlertCritical:
		return mbIconError
	default:
		return mbIconInfo
	}
}

// buildFilter creates a double-null-terminated filter string for
// OPENFILENAME. Format: "Desc\0*.ext1;*.ext2\0\0".
// Uses unicode/utf16 directly because the string contains
// embedded NUL separators that syscall.UTF16FromString cannot
// handle.
func buildFilter(extensions []string) *uint16 {
	if len(extensions) == 0 {
		return nil
	}
	globs := make([]string, len(extensions))
	for i, ext := range extensions {
		globs[i] = "*." + ext
	}
	desc := "Files (" + strings.Join(globs, ", ") + ")"
	pattern := strings.Join(globs, ";")

	descU := utf16.Encode([]rune(desc))
	patternU := utf16.Encode([]rune(pattern))

	// desc NUL pattern NUL NUL
	filter := make([]uint16, len(descU)+1+len(patternU)+2)
	copy(filter, descU)
	copy(filter[len(descU)+1:], patternU)
	return &filter[0]
}

// parseMultiSelect extracts paths from a GetOpenFileName buffer.
// Single file: "C:\path\file.txt\0\0"
// Multiple files: "C:\dir\0file1.txt\0file2.txt\0\0"
func parseMultiSelect(buf []uint16) []string {
	firstNull := 0
	for i, v := range buf {
		if v == 0 {
			firstNull = i
			break
		}
	}
	if firstNull == 0 {
		return nil
	}

	// Single selection: next element is also NUL.
	if firstNull+1 >= len(buf) || buf[firstNull+1] == 0 {
		return []string{syscall.UTF16ToString(buf[:firstNull])}
	}

	// Multiple selection: first string is directory, rest are
	// filenames terminated by double NUL.
	dir := syscall.UTF16ToString(buf[:firstNull])
	var paths []string
	start := firstNull + 1
	for start < len(buf) {
		if buf[start] == 0 {
			break
		}
		end := start
		for end < len(buf) && buf[end] != 0 {
			end++
		}
		name := syscall.UTF16ToString(buf[start:end])
		paths = append(paths, filepath.Join(dir, name))
		start = end + 1
	}
	if len(paths) == 0 {
		return []string{dir}
	}
	return paths
}
