package gui

// NativeDialogs provides native file and message dialogs.
// Blocking — call from command queue.
type NativeDialogs interface {
	ShowOpenDialog(title, startDir string, extensions []string, allowMultiple bool) PlatformDialogResult
	ShowSaveDialog(title, startDir, defaultName, defaultExt string, extensions []string, confirmOverwrite bool) PlatformDialogResult
	ShowFolderDialog(title, startDir string) PlatformDialogResult
	ShowMessageDialog(title, body string, level NativeAlertLevel) NativeAlertResult
	ShowConfirmDialog(title, body string, level NativeAlertLevel) NativeAlertResult
	ShowSaveDiscardDialog(title, body string, level NativeAlertLevel) NativeAlertResult
}

// NativeNotifier sends OS-level notifications.
type NativeNotifier interface {
	SendNotification(title, body string) NativeNotificationResult
}

// NativePrinter shows the native print dialog.
// Blocking — call from command queue.
type NativePrinter interface {
	ShowPrintDialog(cfg NativePrintParams) PrintRunResult
}

// NativeBookmarks manages security-scoped file bookmarks.
type NativeBookmarks interface {
	BookmarkLoadAll(appID string) []BookmarkEntry
	BookmarkPersist(appID, path string, data []byte)
	BookmarkStopAccess(data []byte)
}

// NativeAccessibility bridges the OS accessibility tree.
type NativeAccessibility interface {
	A11yInit(actionCallback func(action, index int))
	A11ySync(nodes []A11yNode, count, focusedIdx int)
	A11yDestroy()
	A11yAnnounce(text string)
}

// NativeIME controls the input method editor lifecycle.
type NativeIME interface {
	IMEStart()
	IMEStop()
	IMESetRect(x, y, w, h int32)
}

// NativeSpellChecker provides OS-level spell checking.
type NativeSpellChecker interface {
	SpellCheck(text string) []SpellRange
	SpellSuggest(text string, startByte, lenBytes int) []string
	SpellLearn(word string)
}

// NativeMenubar manages the native OS menubar.
type NativeMenubar interface {
	SetNativeMenubar(cfg NativeMenubarCfg, actionCb func(string))
	ClearNativeMenubar()
}

// NativeSystemTray manages system tray icons and menus.
type NativeSystemTray interface {
	CreateSystemTray(cfg SystemTrayCfg, actionCb func(string)) (int, error)
	UpdateSystemTray(id int, cfg SystemTrayCfg)
	RemoveSystemTray(id int)
}

// NativeSound plays OS-level alert sounds.
type NativeSound interface {
	// Beep plays the user's configured system alert sound, honoring
	// their system-wide alert volume and mute settings. No-op on
	// platforms without such a sound. Non-blocking.
	Beep()
	// BeepAvailable reports whether Beep produces an audible sound on
	// this platform, so callers can fall back to a visual cue.
	BeepAvailable() bool
}

// NativePlatform composes all native OS sub-interfaces.
// Set by the backend; nil in tests (operations no-op / return error).
type NativePlatform interface {
	NativeDialogs
	NativeNotifier
	NativePrinter
	NativeBookmarks
	NativeAccessibility
	NativeIME
	NativeSpellChecker
	NativeMenubar
	NativeSystemTray
	NativeSound
	OpenURI(uri string) error
	TitlebarDark(dark bool)
	SetWindowVibrancy(material VibrancyMaterial)
}

// SpellRange represents a misspelled byte range in text.
type SpellRange struct {
	StartByte int
	LenBytes  int
}

// PlatformDialogResult is the raw result from native file dialogs.
type PlatformDialogResult struct {
	ErrorCode    string
	ErrorMessage string
	Paths        []PlatformPath
	Status       NativeDialogStatus
}

// PlatformPath pairs a path with optional bookmark data.
type PlatformPath struct {
	Path         string
	BookmarkData []byte
}

// BookmarkEntry is a persisted bookmark loaded at startup.
type BookmarkEntry struct {
	Path string
	Data []byte
}

// NativePrintParams contains bridge-level print dialog parameters.
type NativePrintParams struct {
	Title        string
	JobName      string
	PDFPath      string
	PageRanges   string
	Orientation  int
	Copies       int
	DuplexMode   int
	ColorMode    int
	ScaleMode    int
	PaperWidth   float32
	PaperHeight  float32
	MarginTop    float32
	MarginRight  float32
	MarginBottom float32
	MarginLeft   float32
}

// SetNativePlatform sets the native platform backend.
func (w *Window) SetNativePlatform(np NativePlatform) {
	w.nativePlatform = np
}

// NativePlatformBackend returns the native platform backend (nil in tests).
func (w *Window) NativePlatformBackend() NativePlatform {
	return w.nativePlatform
}
