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
	OnDone func(NativeDialogResult, *Window)
	Title  string
	// StartDir is the directory the picker opens in. Empty (the zero
	// value) takes the platform default — the last-used directory on
	// most desktops. A value that cannot name a directory (one starting
	// with "-", or holding a NUL) is screened back to that default; see
	// safeStartDir.
	// exportaudit:keep — caller-facing config (issue #372)
	StartDir      string
	Filters       []NativeFileFilter
	AllowMultiple bool
}

// NativeSaveDialogCfg configures the native save-file dialog.
type NativeSaveDialogCfg struct {
	OnDone func(NativeDialogResult, *Window)
	Title  string
	// StartDir is the directory the picker opens in. Empty (the zero
	// value) takes the platform default, as does a value screened out by
	// safeStartDir.
	// exportaudit:keep — caller-facing config (issue #372)
	StartDir         string
	DefaultName      string
	DefaultExtension string
	Filters          []NativeFileFilter
	ConfirmOverwrite bool
}

// NativeFolderDialogCfg configures the native folder picker.
type NativeFolderDialogCfg struct {
	OnDone func(NativeDialogResult, *Window)
	Title  string
	// StartDir is the directory the picker opens in. Empty (the zero
	// value) takes the platform default, as does a value screened out by
	// safeStartDir.
	// exportaudit:keep — caller-facing config (issue #372)
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
		nativeOpenDialogImpl(w, cfg, false)
	})
}

// NativeSaveDialog opens a native save-file dialog.
func (w *Window) NativeSaveDialog(cfg NativeSaveDialogCfg) {
	w.QueueCommand(func(w *Window) {
		nativeSaveDialogImpl(w, cfg, false)
	})
}

// NativeFolderDialog opens a native folder picker dialog.
func (w *Window) NativeFolderDialog(cfg NativeFolderDialogCfg) {
	w.QueueCommand(func(w *Window) {
		nativeFolderDialogImpl(w, cfg, false)
	})
}

// InWindowOpenDialog opens the in-window file browser in open mode. It
// never calls the native picker, so it looks and acts the same on every
// OS. NativeOpenDialog shows this browser on its own when there is no
// native picker; call this method only to force it.
//
// Result.Paths carry a zero Grant. On macOS a sandboxed app gets no
// security-scoped access to the chosen file this way; use
// NativeOpenDialog there.
func (w *Window) InWindowOpenDialog(cfg NativeOpenDialogCfg) {
	w.QueueCommand(func(w *Window) {
		nativeOpenDialogImpl(w, cfg, true)
	})
}

// InWindowSaveDialog opens the in-window file browser in save mode.
// See InWindowOpenDialog for when to use it and the macOS sandbox limit.
func (w *Window) InWindowSaveDialog(cfg NativeSaveDialogCfg) {
	w.QueueCommand(func(w *Window) {
		nativeSaveDialogImpl(w, cfg, true)
	})
}

// InWindowFolderDialog opens the in-window file browser in folder mode.
// See InWindowOpenDialog for when to use it and the macOS sandbox limit.
func (w *Window) InWindowFolderDialog(cfg NativeFolderDialogCfg) {
	w.QueueCommand(func(w *Window) {
		nativeFolderDialogImpl(w, cfg, true)
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

// markNativeDialogVisible flags the window as showing a native modal
// for the duration of the blocking platform call. The returned cleanup
// must be deferred. Lets DialogIsVisible — and quit/close dedup — see
// native dialogs, which otherwise never touch dialogCfg. The platform
// call and this flag both run on the command goroutine, so the modal's
// nested event loop sees the flag set.
func markNativeDialogVisible(w *Window) func() {
	w.nativeDialogVisible = true
	return func() { w.nativeDialogVisible = false }
}

// The three file dialog impls check the Cfg, then hand the platform
// call to nativeFileDialog, which falls back to the in-window file
// browser (file_browser.go) when there is no native picker (#831). A
// bad Cfg is an error on both the native and the in-window path.

func nativeOpenDialogImpl(w *Window, cfg NativeOpenDialogCfg, inWindow bool) {
	extensions, err := nativeExtensionsFromFilters(cfg.Filters)
	if err != nil {
		dispatchDialogDone(w, cfg.OnDone, nativeDialogErrorResult("invalid_cfg", err.Error()))
		return
	}
	browser := fileBrowserCfg{
		mode:          fileBrowserOpen,
		onDone:        cfg.OnDone,
		title:         cfg.Title,
		startDir:      safeStartDir(cfg.StartDir),
		filters:       cfg.Filters,
		allowMultiple: cfg.AllowMultiple,
	}
	nativeFileDialog(w, browser, inWindow, func(p NativePlatform) PlatformDialogResult {
		return p.ShowOpenDialog(cfg.Title, browser.startDir, extensions, cfg.AllowMultiple)
	})
}

func nativeSaveDialogImpl(w *Window, cfg NativeSaveDialogCfg, inWindow bool) {
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
	browser := fileBrowserCfg{
		mode:             fileBrowserSave,
		onDone:           cfg.OnDone,
		title:            cfg.Title,
		startDir:         safeStartDir(cfg.StartDir),
		defaultName:      cfg.DefaultName,
		defaultExt:       defaultExt,
		filters:          cfg.Filters,
		confirmOverwrite: cfg.ConfirmOverwrite,
	}
	nativeFileDialog(w, browser, inWindow, func(p NativePlatform) PlatformDialogResult {
		return p.ShowSaveDialog(cfg.Title, browser.startDir, cfg.DefaultName, defaultExt, extensions, cfg.ConfirmOverwrite)
	})
}

func nativeFolderDialogImpl(w *Window, cfg NativeFolderDialogCfg, inWindow bool) {
	browser := fileBrowserCfg{
		mode:     fileBrowserFolder,
		onDone:   cfg.OnDone,
		title:    cfg.Title,
		startDir: safeStartDir(cfg.StartDir),
	}
	nativeFileDialog(w, browser, inWindow, func(p NativePlatform) PlatformDialogResult {
		return p.ShowFolderDialog(cfg.Title, browser.startDir)
	})
}

// nativeFileDialog runs a checked file dialog. It opens the in-window
// browser when inWindow is set (the InWindow*Dialog methods), when the
// platform is nil, or when show reports no picker
// (nativeFileDialogUnavailable). Any other result, a real error
// included, goes to OnDone.
//
// The native-dialog flag is cleared before the browser opens, so the
// browser counts as an in-app dialog, not a native one. On the native
// path OnDone runs while the flag is still set, as before.
func nativeFileDialog(w *Window, browser fileBrowserCfg, inWindow bool, show func(NativePlatform) PlatformDialogResult) {
	if inWindow || w.nativePlatform == nil {
		showFileBrowser(w, browser)
		return
	}
	done := markNativeDialogVisible(w)
	pr := show(w.nativePlatform)
	if nativeFileDialogUnavailable(pr) {
		done()
		showFileBrowser(w, browser)
		return
	}
	defer done()
	dispatchDialogDone(w, browser.onDone, nativeResultFromPlatform(pr, w))
}

func nativeMessageDialogImpl(w *Window, cfg NativeMessageDialogCfg) {
	if w.nativePlatform == nil {
		dispatchAlertDone(w, cfg.OnDone, nativeAlertErrorResult("unsupported", "no native platform"))
		return
	}
	defer markNativeDialogVisible(w)()
	result := w.nativePlatform.ShowMessageDialog(cfg.Title, cfg.Body, cfg.Level)
	dispatchAlertDone(w, cfg.OnDone, result)
}

func nativeConfirmDialogImpl(w *Window, cfg NativeConfirmDialogCfg) {
	if w.nativePlatform == nil {
		dispatchAlertDone(w, cfg.OnDone, nativeAlertErrorResult("unsupported", "no native platform"))
		return
	}
	defer markNativeDialogVisible(w)()
	result := w.nativePlatform.ShowConfirmDialog(cfg.Title, cfg.Body, cfg.Level)
	dispatchAlertDone(w, cfg.OnDone, result)
}

func nativeSaveDiscardDialogImpl(w *Window, cfg NativeSaveDiscardDialogCfg) {
	if w.nativePlatform == nil {
		dispatchAlertDone(w, cfg.OnDone, nativeAlertErrorResult("unsupported", "no native platform"))
		return
	}
	defer markNativeDialogVisible(w)()
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

// safeStartDir screens a caller-supplied StartDir before it reaches a
// platform backend. StartDir became caller-settable with issue #372, and the
// Linux backend spends it as an argv element for zenity/kdialog — kdialog
// takes the directory positionally, so a value that begins with "-" is read
// as an option rather than a path. A NUL byte is rejected for the same class
// of reason: exec refuses it and the Cocoa path would truncate at it.
//
// Neither case is worth an error. The documented meaning of an empty
// StartDir is "platform default", so an unusable value degrades to exactly
// that instead of failing a dialog the user asked for.
func safeStartDir(dir string) string {
	if strings.HasPrefix(dir, "-") || strings.ContainsRune(dir, 0) {
		return ""
	}
	return dir
}

func nativeResultFromPlatform(pr PlatformDialogResult, w *Window) NativeDialogResult {
	paths := make([]AccessiblePath, len(pr.Paths))
	for i, pp := range pr.Paths {
		var grant Grant
		if pp.Path != "" && len(pp.BookmarkData) > 0 {
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
