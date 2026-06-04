package gui

import (
	"fmt"
	"slices"
	"strings"
)

// NativeDialogStatus reports native dialog outcome.
type NativeDialogStatus uint8

// NativeDialogStatus constants.
const (
	DialogOK      NativeDialogStatus = iota
	DialogCancel                     // user cancelled
	DialogError                      // platform error
	DialogDiscard                    // user chose discard/don't-save
)

// NativeDialogResult contains native file dialog completion data.
type NativeDialogResult struct {
	ErrorCode    string
	ErrorMessage string
	Paths        []AccessiblePath
	Status       NativeDialogStatus
}

// PathStrings returns just the path strings, discarding grants.
func (r NativeDialogResult) PathStrings() []string {
	out := make([]string, len(r.Paths))
	for i, p := range r.Paths {
		out[i] = p.Path
	}
	return out
}

// NativeFileFilter groups file extensions for native dialogs.
type NativeFileFilter struct {
	Name       string
	Extensions []string
}

// NativeOpenDialogCfg configures the native open-file dialog.
type NativeOpenDialogCfg struct {
	OnDone        func(NativeDialogResult, *Window)
	Title         string
	StartDir      string
	Filters       []NativeFileFilter
	AllowMultiple bool
}

// NativeSaveDialogCfg configures the native save-file dialog.
type NativeSaveDialogCfg struct {
	OnDone           func(NativeDialogResult, *Window)
	Title            string
	StartDir         string
	DefaultName      string
	DefaultExtension string
	Filters          []NativeFileFilter
	ConfirmOverwrite bool
}

// NativeFolderDialogCfg configures the native folder picker.
type NativeFolderDialogCfg struct {
	OnDone   func(NativeDialogResult, *Window)
	Title    string
	StartDir string
}

// NativeAlertLevel controls the severity icon of a message/confirm dialog.
type NativeAlertLevel uint8

// NativeAlertLevel constants.
const (
	AlertInfo NativeAlertLevel = iota
	AlertWarning
	AlertCritical
)

// NativeAlertResult contains native alert dialog outcome.
type NativeAlertResult struct {
	ErrorCode    string
	ErrorMessage string
	Status       NativeDialogStatus
}

// NativeMessageDialogCfg configures a native message dialog.
type NativeMessageDialogCfg struct {
	OnDone func(NativeAlertResult, *Window)
	Title  string
	Body   string
	Level  NativeAlertLevel
}

// NativeConfirmDialogCfg configures a native Yes/No dialog.
type NativeConfirmDialogCfg struct {
	OnDone func(NativeAlertResult, *Window)
	Title  string
	Body   string
	Level  NativeAlertLevel
}

// NativeSaveDiscardDialogCfg configures a native Save/Discard/Cancel dialog.
// OnDone receives DialogOK (save), DialogDiscard (don't save), or DialogCancel.
type NativeSaveDiscardDialogCfg struct {
	OnDone func(NativeAlertResult, *Window)
	Title  string
	Body   string
	Level  NativeAlertLevel
}

// --- Window methods ---

// NativeOpenDialog opens a native open-file dialog.
func (w *Window) NativeOpenDialog(cfg NativeOpenDialogCfg) {
	w.QueueCommand(func(w *Window) {
		nativeOpenDialogImpl(w, cfg)
	})
}

// NativeSaveDialog opens a native save-file dialog.
func (w *Window) NativeSaveDialog(cfg NativeSaveDialogCfg) {
	w.QueueCommand(func(w *Window) {
		nativeSaveDialogImpl(w, cfg)
	})
}

// NativeFolderDialog opens a native folder picker dialog.
func (w *Window) NativeFolderDialog(cfg NativeFolderDialogCfg) {
	w.QueueCommand(func(w *Window) {
		nativeFolderDialogImpl(w, cfg)
	})
}

// NativeMessageDialog opens a native OS message box.
func (w *Window) NativeMessageDialog(cfg NativeMessageDialogCfg) {
	w.QueueCommand(func(w *Window) {
		nativeMessageDialogImpl(w, cfg)
	})
}

// NativeConfirmDialog opens a native OS Yes/No dialog.
func (w *Window) NativeConfirmDialog(cfg NativeConfirmDialogCfg) {
	w.QueueCommand(func(w *Window) {
		nativeConfirmDialogImpl(w, cfg)
	})
}

// NativeSaveDiscardDialog opens a native Save/Discard/Cancel dialog.
func (w *Window) NativeSaveDiscardDialog(cfg NativeSaveDiscardDialogCfg) {
	w.QueueCommand(func(w *Window) {
		nativeSaveDiscardDialogImpl(w, cfg)
	})
}

// --- impl functions ---

func nativeOpenDialogImpl(w *Window, cfg NativeOpenDialogCfg) {
	extensions, err := nativeExtensionsFromFilters(cfg.Filters)
	if err != nil {
		dispatchDialogDone(w, cfg.OnDone, nativeDialogErrorResult("invalid_cfg", err.Error()))
		return
	}
	if w.nativePlatform == nil {
		dispatchDialogDone(w, cfg.OnDone, nativeDialogErrorResult("unsupported", "no native platform"))
		return
	}
	pr := w.nativePlatform.ShowOpenDialog(cfg.Title, cfg.StartDir, extensions, cfg.AllowMultiple)
	dispatchDialogDone(w, cfg.OnDone, nativeResultFromPlatform(pr, w))
}

func nativeSaveDialogImpl(w *Window, cfg NativeSaveDialogCfg) {
	extensions, err := nativeSaveExtensions(cfg.Filters, cfg.DefaultExtension)
	if err != nil {
		dispatchDialogDone(w, cfg.OnDone, nativeDialogErrorResult("invalid_cfg", err.Error()))
		return
	}
	defaultExt, err := nativeNormalizeExtension(cfg.DefaultExtension)
	if err != nil {
		dispatchDialogDone(w, cfg.OnDone, nativeDialogErrorResult("invalid_cfg", err.Error()))
		return
	}
	if w.nativePlatform == nil {
		dispatchDialogDone(w, cfg.OnDone, nativeDialogErrorResult("unsupported", "no native platform"))
		return
	}
	pr := w.nativePlatform.ShowSaveDialog(cfg.Title, cfg.StartDir, cfg.DefaultName, defaultExt, extensions, cfg.ConfirmOverwrite)
	dispatchDialogDone(w, cfg.OnDone, nativeResultFromPlatform(pr, w))
}

func nativeFolderDialogImpl(w *Window, cfg NativeFolderDialogCfg) {
	if w.nativePlatform == nil {
		dispatchDialogDone(w, cfg.OnDone, nativeDialogErrorResult("unsupported", "no native platform"))
		return
	}
	pr := w.nativePlatform.ShowFolderDialog(cfg.Title, cfg.StartDir)
	dispatchDialogDone(w, cfg.OnDone, nativeResultFromPlatform(pr, w))
}

func nativeMessageDialogImpl(w *Window, cfg NativeMessageDialogCfg) {
	if w.nativePlatform == nil {
		dispatchAlertDone(w, cfg.OnDone, nativeAlertErrorResult("unsupported", "no native platform"))
		return
	}
	result := w.nativePlatform.ShowMessageDialog(cfg.Title, cfg.Body, cfg.Level)
	dispatchAlertDone(w, cfg.OnDone, result)
}

func nativeConfirmDialogImpl(w *Window, cfg NativeConfirmDialogCfg) {
	if w.nativePlatform == nil {
		dispatchAlertDone(w, cfg.OnDone, nativeAlertErrorResult("unsupported", "no native platform"))
		return
	}
	result := w.nativePlatform.ShowConfirmDialog(cfg.Title, cfg.Body, cfg.Level)
	dispatchAlertDone(w, cfg.OnDone, result)
}

func nativeSaveDiscardDialogImpl(w *Window, cfg NativeSaveDiscardDialogCfg) {
	if w.nativePlatform == nil {
		dispatchAlertDone(w, cfg.OnDone, nativeAlertErrorResult("unsupported", "no native platform"))
		return
	}
	result := w.nativePlatform.ShowSaveDiscardDialog(cfg.Title, cfg.Body, cfg.Level)
	dispatchAlertDone(w, cfg.OnDone, result)
}

// --- dispatch helpers ---

func dispatchDialogDone(w *Window, onDone func(NativeDialogResult, *Window), result NativeDialogResult) {
	if onDone != nil {
		onDone(result, w)
	}
}

func dispatchAlertDone(w *Window, onDone func(NativeAlertResult, *Window), result NativeAlertResult) {
	if onDone != nil {
		onDone(result, w)
	}
}

func nativeDialogErrorResult(code, message string) NativeDialogResult {
	return NativeDialogResult{Status: DialogError, ErrorCode: code, ErrorMessage: message}
}

func nativeAlertErrorResult(code, message string) NativeAlertResult {
	return NativeAlertResult{Status: DialogError, ErrorCode: code, ErrorMessage: message}
}

func nativeResultFromPlatform(pr PlatformDialogResult, w *Window) NativeDialogResult {
	paths := make([]AccessiblePath, len(pr.Paths))
	for i, pp := range pr.Paths {
		var grant Grant
		if len(pp.BookmarkData) > 0 {
			grant = w.storeBookmark(pp.Path, pp.BookmarkData)
		}
		paths[i] = AccessiblePath{Path: pp.Path, Grant: grant}
	}
	return NativeDialogResult{
		Status:       pr.Status,
		Paths:        paths,
		ErrorCode:    pr.ErrorCode,
		ErrorMessage: pr.ErrorMessage,
	}
}

// --- extension validation ---

func nativeIsValidExtension(ext string) bool {
	if len(ext) == 0 {
		return false
	}
	for _, c := range ext {
		valid := (c >= 'a' && c <= 'z') || (c >= '0' && c <= '9') ||
			c == '_' || c == '-' || c == '+'
		if !valid {
			return false
		}
	}
	return true
}

func nativeNormalizeExtension(raw string) (string, error) {
	ext := strings.ToLower(strings.TrimSpace(raw))
	for strings.HasPrefix(ext, ".") {
		ext = ext[1:]
	}
	if ext == "" {
		return "", nil
	}
	if !nativeIsValidExtension(ext) {
		return "", fmt.Errorf("invalid extension: %s", raw)
	}
	return ext, nil
}

func nativeExtensionsFromFilters(filters []NativeFileFilter) ([]string, error) {
	var all []string
	for _, f := range filters {
		for _, raw := range f.Extensions {
			ext, err := nativeNormalizeExtension(raw)
			if err != nil {
				return nil, err
			}
			if ext != "" {
				all = append(all, ext)
			}
		}
	}
	return all, nil
}

func nativeSaveExtensions(filters []NativeFileFilter, defaultExt string) ([]string, error) {
	all, err := nativeExtensionsFromFilters(filters)
	if err != nil {
		return nil, err
	}
	if defaultExt != "" {
		ext, err := nativeNormalizeExtension(defaultExt)
		if err != nil {
			return nil, err
		}
		if ext != "" && !slices.Contains(all, ext) {
			all = append(all, ext)
		}
	}
	return all, nil
}
