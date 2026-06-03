//go:build darwin && !ios

// Package filedialog provides native file dialog support.
package filedialog

/*
#cgo CFLAGS: -fobjc-arc
#cgo LDFLAGS: -framework AppKit -framework UniformTypeIdentifiers
#include "dialog_darwin.h"
#include <stdlib.h>
*/
import "C"

import (
	"unsafe"

	"github.com/go-gui-org/go-gui/gui"
)

// ShowOpenDialog shows a native open-file dialog.
func ShowOpenDialog(title, startDir string,
	extensions []string, allowMultiple bool) gui.PlatformDialogResult {

	cTitle := C.CString(title)
	defer C.free(unsafe.Pointer(cTitle))
	cStartDir := C.CString(startDir)
	defer C.free(unsafe.Pointer(cStartDir))

	cExts, cExtCount := toCStringArray(extensions)
	defer freeCStringArray(cExts, cExtCount)

	multi := C.int(0)
	if allowMultiple {
		multi = 1
	}

	r := C.filedialogOpen(cTitle, cStartDir, cExts, cExtCount, multi)
	defer C.filedialogFreeResult(r)
	return toResult(r)
}

// ShowSaveDialog shows a native save-file dialog.
func ShowSaveDialog(title, startDir, defaultName, defaultExt string,
	extensions []string, confirmOverwrite bool) gui.PlatformDialogResult {

	cTitle := C.CString(title)
	defer C.free(unsafe.Pointer(cTitle))
	cStartDir := C.CString(startDir)
	defer C.free(unsafe.Pointer(cStartDir))
	cName := C.CString(defaultName)
	defer C.free(unsafe.Pointer(cName))
	cExt := C.CString(defaultExt)
	defer C.free(unsafe.Pointer(cExt))

	cExts, cExtCount := toCStringArray(extensions)
	defer freeCStringArray(cExts, cExtCount)

	confirm := C.int(0)
	if confirmOverwrite {
		confirm = 1
	}

	r := C.filedialogSave(cTitle, cStartDir, cName, cExt,
		cExts, cExtCount, confirm)
	defer C.filedialogFreeResult(r)
	return toResult(r)
}

// ShowFolderDialog shows a native folder picker dialog.
func ShowFolderDialog(title, startDir string) gui.PlatformDialogResult {
	cTitle := C.CString(title)
	defer C.free(unsafe.Pointer(cTitle))
	cStartDir := C.CString(startDir)
	defer C.free(unsafe.Pointer(cStartDir))

	r := C.filedialogFolder(cTitle, cStartDir)
	defer C.filedialogFreeResult(r)
	return toResult(r)
}

// ShowMessageDialog shows a native message alert.
func ShowMessageDialog(title, body string,
	level gui.NativeAlertLevel) gui.NativeAlertResult {

	cTitle := C.CString(title)
	defer C.free(unsafe.Pointer(cTitle))
	cBody := C.CString(body)
	defer C.free(unsafe.Pointer(cBody))

	r := C.filedialogMessage(cTitle, cBody, C.int(level))
	return toAlertResult(r)
}

// ShowConfirmDialog shows a native OK/Cancel alert.
func ShowConfirmDialog(title, body string,
	level gui.NativeAlertLevel) gui.NativeAlertResult {

	cTitle := C.CString(title)
	defer C.free(unsafe.Pointer(cTitle))
	cBody := C.CString(body)
	defer C.free(unsafe.Pointer(cBody))

	r := C.filedialogConfirm(cTitle, cBody, C.int(level))
	return toAlertResult(r)
}

// ShowSaveDiscardDialog shows a native Save/Don't Save/Cancel alert.
func ShowSaveDiscardDialog(title, body string,
	level gui.NativeAlertLevel) gui.NativeAlertResult {

	cTitle := C.CString(title)
	defer C.free(unsafe.Pointer(cTitle))
	cBody := C.CString(body)
	defer C.free(unsafe.Pointer(cBody))

	r := C.filedialogSaveDiscard(cTitle, cBody, C.int(level))
	return toSaveDiscardResult(r)
}

func toAlertResult(r C.AlertResult) gui.NativeAlertResult {
	var status gui.NativeDialogStatus
	switch r.status {
	case C.DIALOG_OK:
		status = gui.DialogOK
	case C.DIALOG_CANCEL:
		status = gui.DialogCancel
	default:
		status = gui.DialogError
	}
	var errMsg string
	if r.errorMessage != nil {
		errMsg = C.GoString(r.errorMessage)
		C.free(unsafe.Pointer(r.errorMessage))
	}
	return gui.NativeAlertResult{
		Status:       status,
		ErrorMessage: errMsg,
	}
}

func toSaveDiscardResult(r C.AlertResult) gui.NativeAlertResult {
	var status gui.NativeDialogStatus
	switch r.status {
	case C.DIALOG_OK:
		status = gui.DialogOK
	case C.DIALOG_DISCARD:
		status = gui.DialogDiscard
	case C.DIALOG_CANCEL:
		status = gui.DialogCancel
	default:
		status = gui.DialogError
	}
	var errMsg string
	if r.errorMessage != nil {
		errMsg = C.GoString(r.errorMessage)
		C.free(unsafe.Pointer(r.errorMessage))
	}
	return gui.NativeAlertResult{
		Status:       status,
		ErrorMessage: errMsg,
	}
}

// --- helpers ---

func toCStringArray(strs []string) (**C.char, C.int) {
	if len(strs) == 0 {
		return nil, 0
	}
	arr := C.malloc(C.size_t(len(strs)) * C.size_t(unsafe.Sizeof((*C.char)(nil))))
	slice := unsafe.Slice((**C.char)(arr), len(strs))
	for i, s := range strs {
		slice[i] = C.CString(s)
	}
	return (**C.char)(arr), C.int(len(strs))
}

func freeCStringArray(arr **C.char, count C.int) {
	if arr == nil {
		return
	}
	slice := unsafe.Slice(arr, int(count))
	for _, p := range slice {
		C.free(unsafe.Pointer(p))
	}
	C.free(unsafe.Pointer(arr))
}

func toResult(r C.DialogResult) gui.PlatformDialogResult {
	var status gui.NativeDialogStatus
	switch r.status {
	case C.DIALOG_OK:
		status = gui.DialogOK
	case C.DIALOG_CANCEL:
		status = gui.DialogCancel
	default:
		status = gui.DialogError
	}

	var paths []gui.PlatformPath
	if r.pathCount > 0 {
		cPaths := unsafe.Slice(r.paths, int(r.pathCount))
		cBookmarks := unsafe.Slice(r.bookmarkData, int(r.pathCount))
		cLens := unsafe.Slice(r.bookmarkLens, int(r.pathCount))
		paths = make([]gui.PlatformPath, int(r.pathCount))
		for i := range paths {
			paths[i].Path = C.GoString(cPaths[i])
			if cLens[i] > 0 && cBookmarks[i] != nil {
				paths[i].BookmarkData = C.GoBytes(cBookmarks[i],
					cLens[i])
			}
		}
	}

	var errMsg string
	if r.errorMessage != nil {
		errMsg = C.GoString(r.errorMessage)
	}

	return gui.PlatformDialogResult{
		Status:       status,
		Paths:        paths,
		ErrorMessage: errMsg,
	}
}
