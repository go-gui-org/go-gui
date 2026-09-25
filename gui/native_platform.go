package gui

// NativeDialogs provides native file and message dialogs.
// Blocking — call from command queue.
type nativeDialogs interface {
	ShowOpenDialog(title, startDir string, extensions []string, allowMultiple bool) PlatformDialogResult
	ShowSaveDialog(title, startDir, defaultName, defaultExt string, extensions []string, confirmOverwrite bool) PlatformDialogResult
	ShowFolderDialog(title, startDir string) PlatformDialogResult
	ShowMessageDialog(title, body string, level NativeAlertLevel) NativeAlertResult
	ShowConfirmDialog(title, body string, level NativeAlertLevel) NativeAlertResult
	ShowSaveDiscardDialog(title, body string, level NativeAlertLevel) NativeAlertResult
}

// NativeNotifier sends OS-level notifications.
type nativeNotifier interface {
	SendNotification(title, body string) NativeNotificationResult
}

// NativePrinter shows the native print dialog.
// Blocking — call from command queue.
type nativePrinter interface {
	ShowPrintDialog(cfg NativePrintParams) PrintRunResult
}

// NativeBookmarks manages security-scoped file bookmarks.
type nativeBookmarks interface {
	BookmarkLoadAll(appID string) []BookmarkEntry
	BookmarkPersist(appID, path string, data []byte)
	BookmarkStopAccess(data []byte)
}

// NativeAccessibility bridges the OS accessibility tree.
type nativeAccessibility interface {
	A11yInit(actionCallback func(action, index int))
	A11ySync(nodes []A11yNode, count, focusedIdx int)
	A11yDestroy()
	A11yAnnounce(text string)
}

// NativeIME controls the input method editor lifecycle.
type nativeIME interface {
	IMEStart()
	IMEStop()
	IMESetRect(x, y, w, h int32)
}

// NativeSpellChecker provides OS-level spell checking.
type nativeSpellChecker interface {
	SpellCheck(text string) []SpellRange
	SpellSuggest(text string, startByte, lenBytes int) []string
	SpellLearn(word string)
}

// NativeMenubar manages the native OS menubar.
type nativeMenubar interface {
	SetNativeMenubar(cfg NativeMenubarCfg, actionCb func(string))
	ClearNativeMenubar()
}

// NativeSystemTray manages system tray icons and menus.
type nativeSystemTray interface {
	CreateSystemTray(cfg SystemTrayCfg, actionCb func(string)) (int, error)
	UpdateSystemTray(id int, cfg SystemTrayCfg)
	RemoveSystemTray(id int)
}

// NativeWindowVisibility shows and hides the OS window without
// destroying it. Hide keeps the window registered and framing; Show
// unhides and raises it (issue #779). No-op where there is no window
// manager to ask (web, iOS, Android).
type nativeWindowVisibility interface {
	ShowWindow()
	HideWindow()
}

// NativeSound plays OS-level alert sounds.
type nativeSound interface {
	// Beep plays the user's configured system alert sound, honoring
	// their system-wide alert volume and mute settings. No-op on
	// platforms without such a sound. Non-blocking.
	Beep()
	// BeepAvailable reports whether Beep produces an audible sound on
	// this platform, so callers can fall back to a visual cue.
	BeepAvailable() bool
}

// NativeAppearance reports the OS light/dark setting (issue #752).
// Noop where the OS has no setting; nil in tests reads as no setting.
type nativeAppearance interface {
	// SystemAppearance queries the current OS appearance. The second
	// result is false when the OS reports no setting, in which case
	// the app keeps its own theme.
	SystemAppearance() (Appearance, bool)
	// SetSystemAppearanceCallback registers cb for OS appearance
	// changes. The backend invokes cb on a watcher thread; the gui
	// side marshals to the frame thread with QueueCommand. A nil cb
	// unregisters and lets the backend stop its watcher.
	SetSystemAppearanceCallback(cb func(Appearance))
}

// NativePlatform composes all native OS sub-interfaces.
// Set by the backend; nil in tests (operations no-op / return error).
// exportaudit:keep — collides with the window's nativePlatform state field
type NativePlatform interface {
	nativeDialogs
	nativeNotifier
	nativePrinter
	nativeBookmarks
	nativeAccessibility
	nativeIME
	nativeSpellChecker
	nativeMenubar
	nativeSystemTray
	nativeWindowVisibility
	nativeSound
	nativeAppearance
	OpenURI(uri string) error
	TitlebarDark(dark bool)
	SetWindowVibrancy(material VibrancyMaterial)
	SetWindowOpacity(opacity float32)
	StartWindowDrag()
	StartWindowResize(edge WindowEdge)
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
	// Replay the appearance subscription: FollowSystemAppearance and
	// OnSystemAppearance are reachable before a backend attaches (in
	// OnInit, or before backend.Run), where refreshAppearanceSubscription
	// found no platform. A following window also applies the now-known
	// OS setting at once, like windowOpacity replays below 1.
	w.refreshAppearanceSubscription()
	w.appearanceMu.RLock()
	following := w.appearanceFollowing
	w.appearanceMu.RUnlock()
	if following {
		if a, ok := w.SystemAppearance(); ok {
			w.applySystemAppearance(a)
		}
	}
}

// NativePlatformBackend returns the native platform backend (nil in tests).
func (w *Window) NativePlatformBackend() NativePlatform {
	return w.nativePlatform
}
