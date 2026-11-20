//go:build darwin && !ios

// Package filedialog provides native file dialog support.
package filedialog

import (
	"math"
	"sync"

	"github.com/ebitengine/purego/objc"

	"github.com/go-gui-org/go-gui/gui"
	"github.com/go-gui-org/go-gui/gui/internal/darwinffi"
)

// The open, save and folder panels and the alerts are called through
// purego, so they work with cgo off (#1312). Each one is a modal,
// one-off call, so the objc.Send form, which allocates per call, is
// fine.

// Status codes. They were the C enum in dialog_darwin.h; the
// build*Result functions map them to gui.NativeDialogStatus.
const (
	dialogOK      = 0
	dialogCancel  = 1
	dialogError   = 2
	dialogDiscard = 3
)

// Values from the AppKit and Foundation headers.
const (
	nsModalResponseOK    = 1
	nsAlertFirstButton   = 1000 // NSAlertFirstButtonReturn
	nsAlertThirdButton   = 1002 // NSAlertThirdButtonReturn
	nsAlertStyleWarning  = 0
	nsAlertStyleInfo     = 1
	nsAlertStyleCritical = 2
	nsBookmarkSecurity   = 1 << 11 // NSURLBookmarkCreationWithSecurityScope
	nsEventMaskAny       = math.MaxUint
	activateWaitSeconds  = 0.5
)

var dl struct {
	once sync.Once

	appKit       uintptr
	runLoopMode  objc.ID // NSDefaultRunLoopMode
	clsOpenPanel objc.ID
	clsSavePanel objc.ID
	clsAlert     objc.ID
	clsURL       objc.ID
	clsUTType    objc.ID
	clsArray     objc.ID
	clsDate      objc.ID

	selOpenPanel, selSavePanel, selAlloc, selInit, selRelease       objc.SEL
	selTitle, selDirURL, selFileURL, selChooseFiles, selChooseDirs  objc.SEL
	selMultiple, selTypes, selCreateDirs, selNameField, selRunModal objc.SEL
	selURLs, selURL, selCount, selObjectAt, selFSRep, selBookmark   objc.SEL
	selTypeForExt, selArrayCap, selAddObject                        objc.SEL
	selStyle, selMessage, selInformative, selAddButton              objc.SEL
	selIsActive, selActivate, selDateSinceNow, selIntervalSinceNow  objc.SEL
	selNextEvent, selSendEvent                                      objc.SEL
}

func load() {
	dl.once.Do(func() {
		dl.appKit = darwinffi.Open(darwinffi.PathAppKit)
		foundation := darwinffi.Open(darwinffi.PathFoundation)
		darwinffi.Open(darwinffi.PathUTI)
		dl.runLoopMode = objc.ID(darwinffi.Data(foundation, "NSDefaultRunLoopMode"))
		dl.clsOpenPanel = darwinffi.Class("NSOpenPanel")
		dl.clsSavePanel = darwinffi.Class("NSSavePanel")
		dl.clsAlert = darwinffi.Class("NSAlert")
		dl.clsURL = darwinffi.Class("NSURL")
		dl.clsUTType = darwinffi.Class("UTType")
		dl.clsArray = darwinffi.Class("NSMutableArray")
		dl.clsDate = darwinffi.Class("NSDate")
		dl.selOpenPanel = darwinffi.Sel("openPanel")
		dl.selSavePanel = darwinffi.Sel("savePanel")
		dl.selAlloc = darwinffi.Sel("alloc")
		dl.selInit = darwinffi.Sel("init")
		dl.selRelease = darwinffi.Sel("release")
		dl.selTitle = darwinffi.Sel("setTitle:")
		dl.selDirURL = darwinffi.Sel("setDirectoryURL:")
		dl.selFileURL = darwinffi.Sel("fileURLWithPath:")
		dl.selChooseFiles = darwinffi.Sel("setCanChooseFiles:")
		dl.selChooseDirs = darwinffi.Sel("setCanChooseDirectories:")
		dl.selMultiple = darwinffi.Sel("setAllowsMultipleSelection:")
		dl.selTypes = darwinffi.Sel("setAllowedContentTypes:")
		dl.selCreateDirs = darwinffi.Sel("setCanCreateDirectories:")
		dl.selNameField = darwinffi.Sel("setNameFieldStringValue:")
		dl.selRunModal = darwinffi.Sel("runModal")
		dl.selURLs = darwinffi.Sel("URLs")
		dl.selURL = darwinffi.Sel("URL")
		dl.selCount = darwinffi.Sel("count")
		dl.selObjectAt = darwinffi.Sel("objectAtIndex:")
		dl.selFSRep = darwinffi.Sel("fileSystemRepresentation")
		dl.selBookmark = darwinffi.Sel(
			"bookmarkDataWithOptions:includingResourceValuesForKeys:relativeToURL:error:")
		dl.selTypeForExt = darwinffi.Sel("typeWithFilenameExtension:")
		dl.selArrayCap = darwinffi.Sel("arrayWithCapacity:")
		dl.selAddObject = darwinffi.Sel("addObject:")
		dl.selStyle = darwinffi.Sel("setAlertStyle:")
		dl.selMessage = darwinffi.Sel("setMessageText:")
		dl.selInformative = darwinffi.Sel("setInformativeText:")
		dl.selAddButton = darwinffi.Sel("addButtonWithTitle:")
		dl.selIsActive = darwinffi.Sel("isActive")
		dl.selActivate = darwinffi.Sel("activateIgnoringOtherApps:")
		dl.selDateSinceNow = darwinffi.Sel("dateWithTimeIntervalSinceNow:")
		dl.selIntervalSinceNow = darwinffi.Sel("timeIntervalSinceNow")
		dl.selNextEvent = darwinffi.Sel("nextEventMatchingMask:untilDate:inMode:dequeue:")
		dl.selSendEvent = darwinffi.Sel("sendEvent:")
	})
}

// setString sends sel with an NSString of s. An empty s, or one that
// is not valid UTF-8, sends nothing, as the cgo form did.
func setString(obj objc.ID, sel objc.SEL, s string) {
	if s == "" {
		return
	}
	if ns := darwinffi.NSString(s); ns != 0 {
		obj.Send(sel, ns)
	}
}

// setDirectory points panel at dir. An empty dir leaves the panel's
// default.
func setDirectory(panel objc.ID, dir string) {
	if dir == "" {
		return
	}
	if ns := darwinffi.NSString(dir); ns != 0 {
		panel.Send(dl.selDirURL, dl.clsURL.Send(dl.selFileURL, ns))
	}
}

// contentTypes returns an autoreleased NSArray of UTTypes for the
// file extensions, or 0 (allow all) when none maps to a type. Call it
// inside an autorelease pool.
func contentTypes(extensions []string) objc.ID {
	if len(extensions) == 0 {
		return 0
	}
	types := dl.clsArray.Send(dl.selArrayCap, uint(len(extensions)))
	for _, ext := range extensions {
		ns := darwinffi.NSString(ext)
		if ns == 0 {
			continue
		}
		if t := dl.clsUTType.Send(dl.selTypeForExt, ns); t != 0 {
			types.Send(dl.selAddObject, t)
		}
	}
	if objc.Send[uint](types, dl.selCount) == 0 {
		return 0
	}
	return types
}

// paths copies the file paths of an NSArray of NSURLs, each with a
// security-scoped bookmark when one can be made.
func paths(urls objc.ID) []gui.PlatformPath {
	n := objc.Send[uint](urls, dl.selCount)
	if n == 0 {
		return nil
	}
	out := make([]gui.PlatformPath, n)
	for i := range n {
		url := urls.Send(dl.selObjectAt, i)
		// fileSystemRepresentation is a C string that purego copies.
		out[i].Path = objc.Send[string](url, dl.selFSRep)
		// The error out-parameter is NULL: a URL with no bookmark
		// still gives its path, as in the cgo form.
		data := url.Send(dl.selBookmark, uint(nsBookmarkSecurity),
			objc.ID(0), objc.ID(0), uintptr(0))
		if data != 0 {
			// NSData is toll-free bridged to CFData.
			if b := darwinffi.CFDataBytes(uintptr(data)); len(b) > 0 {
				out[i].BookmarkData = b
			}
		}
	}
	return out
}

// runPanel shows panel as a modal and returns the result. pick reads
// the chosen URLs from the panel after an OK.
func runPanel(panel objc.ID, pick func(objc.ID) objc.ID) gui.PlatformDialogResult {
	activateForModal()
	if objc.Send[int](panel, dl.selRunModal) != nsModalResponseOK {
		return buildDialogResult(dialogCancel, nil, "")
	}
	return buildDialogResult(dialogOK, paths(pick(panel)), "")
}

func panelURLs(panel objc.ID) objc.ID { return panel.Send(dl.selURLs) }

// ShowOpenDialog shows a native open-file dialog.
func ShowOpenDialog(title, startDir string,
	extensions []string, allowMultiple bool) gui.PlatformDialogResult {
	load()
	pool := darwinffi.PoolPush()
	defer darwinffi.PoolPop(pool)

	panel := dl.clsOpenPanel.Send(dl.selOpenPanel)
	setString(panel, dl.selTitle, title)
	setDirectory(panel, startDir)
	panel.Send(dl.selChooseFiles, true)
	panel.Send(dl.selChooseDirs, false)
	panel.Send(dl.selMultiple, allowMultiple)
	if types := contentTypes(extensions); types != 0 {
		panel.Send(dl.selTypes, types)
	}
	return runPanel(panel, panelURLs)
}

// ShowSaveDialog shows a native save-file dialog. NSSavePanel always
// asks before it overwrites, so confirmOverwrite has no effect.
// defaultExt is not used either, as in the cgo form: extensions sets
// the allowed types.
func ShowSaveDialog(title, startDir, defaultName, _ string,
	extensions []string, _ bool) gui.PlatformDialogResult {
	load()
	pool := darwinffi.PoolPush()
	defer darwinffi.PoolPop(pool)

	panel := dl.clsSavePanel.Send(dl.selSavePanel)
	setString(panel, dl.selTitle, title)
	setDirectory(panel, startDir)
	panel.Send(dl.selCreateDirs, true)
	setString(panel, dl.selNameField, defaultName)
	if types := contentTypes(extensions); types != 0 {
		panel.Send(dl.selTypes, types)
	}
	return runPanel(panel, func(p objc.ID) objc.ID {
		urls := dl.clsArray.Send(dl.selArrayCap, uint(1))
		if u := p.Send(dl.selURL); u != 0 {
			urls.Send(dl.selAddObject, u)
		}
		return urls
	})
}

// ShowFolderDialog shows a native folder picker dialog.
func ShowFolderDialog(title, startDir string) gui.PlatformDialogResult {
	load()
	pool := darwinffi.PoolPush()
	defer darwinffi.PoolPop(pool)

	panel := dl.clsOpenPanel.Send(dl.selOpenPanel)
	setString(panel, dl.selTitle, title)
	setDirectory(panel, startDir)
	panel.Send(dl.selChooseFiles, false)
	panel.Send(dl.selChooseDirs, true)
	panel.Send(dl.selMultiple, false)
	return runPanel(panel, panelURLs)
}

// alertStyle maps a gui alert level to an NSAlertStyle.
func alertStyle(level gui.NativeAlertLevel) int {
	switch level {
	case gui.AlertWarning:
		return nsAlertStyleWarning
	case gui.AlertCritical:
		return nsAlertStyleCritical
	default:
		return nsAlertStyleInfo
	}
}

// newAlert returns a new NSAlert with the text, style and buttons.
// The caller releases it. It shows nothing, so a test can call it.
func newAlert(title, body string, level gui.NativeAlertLevel,
	buttons ...string) objc.ID {
	alert := dl.clsAlert.Send(dl.selAlloc).Send(dl.selInit)
	alert.Send(dl.selStyle, alertStyle(level))
	setString(alert, dl.selMessage, title)
	setString(alert, dl.selInformative, body)
	for _, b := range buttons {
		alert.Send(dl.selAddButton, darwinffi.NSString(b))
	}
	return alert
}

// runAlert shows an alert with the buttons and returns the
// NSModalResponse of the button the user pressed.
func runAlert(title, body string, level gui.NativeAlertLevel,
	buttons ...string) int {
	load()
	pool := darwinffi.PoolPush()
	defer darwinffi.PoolPop(pool)
	alert := newAlert(title, body, level, buttons...)
	defer alert.Send(dl.selRelease)
	activateForModal()
	return objc.Send[int](alert, dl.selRunModal)
}

// ShowMessageDialog shows a native message alert.
func ShowMessageDialog(title, body string,
	level gui.NativeAlertLevel) gui.NativeAlertResult {
	runAlert(title, body, level, "OK")
	return buildAlertResult(dialogOK, "")
}

// ShowConfirmDialog shows a native OK/Cancel alert.
func ShowConfirmDialog(title, body string,
	level gui.NativeAlertLevel) gui.NativeAlertResult {
	status := dialogCancel
	if runAlert(title, body, level, "OK", "Cancel") == nsAlertFirstButton {
		status = dialogOK
	}
	return buildAlertResult(status, "")
}

// ShowSaveDiscardDialog shows a native Save/Don't Save/Cancel alert.
// The buttons are added in the order Save, Cancel, Don't Save, so the
// first button is Save and the third is Don't Save.
func ShowSaveDiscardDialog(title, body string,
	level gui.NativeAlertLevel) gui.NativeAlertResult {
	status := dialogCancel
	switch runAlert(title, body, level, "Save", "Cancel", "Don't Save") {
	case nsAlertFirstButton:
		status = dialogOK
	case nsAlertThirdButton:
		status = dialogDiscard
	}
	return buildSaveDiscardResult(status, "")
}

// activateForModal makes the app frontmost and pumps events until the
// activation lands, so a modal alert or panel becomes key the first
// time it shows. The metal backend pumps its own events, so a modal
// loop can start before the asynchronous activateIgnoringOtherApps:
// takes effect. The dialog then is not key (Return, Esc and Tab do
// nothing) until a later dialog shows after the activation. The wait
// is bounded: if activation never lands, the dialog shows as before.
// Call it inside an autorelease pool.
func activateForModal() {
	// NSApp is a global that NSApplication sets, so read it each call.
	// With NSApp nil, every send returns at once, and the loop below
	// would spin for the whole wait.
	app := objc.ID(darwinffi.Data(dl.appKit, "NSApp"))
	if app == 0 || objc.Send[bool](app, dl.selIsActive) {
		return
	}
	// Deprecated in macOS 14. The cgo form used it too.
	app.Send(dl.selActivate, true)
	deadline := dl.clsDate.Send(dl.selDateSinceNow, float64(activateWaitSeconds))
	for !objc.Send[bool](app, dl.selIsActive) &&
		objc.Send[float64](deadline, dl.selIntervalSinceNow) > 0 {
		e := app.Send(dl.selNextEvent, uint(nsEventMaskAny), deadline,
			dl.runLoopMode, true)
		if e != 0 {
			app.Send(dl.selSendEvent, e)
		}
	}
}

// buildAlertResult maps a raw status code and error message to a
// NativeAlertResult.
func buildAlertResult(status int, errMsg string) gui.NativeAlertResult {
	var s gui.NativeDialogStatus
	switch status {
	case dialogOK:
		s = gui.DialogOK
	case dialogCancel:
		s = gui.DialogCancel
	default:
		s = gui.DialogError
	}
	return gui.NativeAlertResult{Status: s, ErrorMessage: errMsg}
}

// buildSaveDiscardResult maps a raw status code and error message to
// a NativeAlertResult including the discard variant.
func buildSaveDiscardResult(status int, errMsg string) gui.NativeAlertResult {
	var s gui.NativeDialogStatus
	switch status {
	case dialogOK:
		s = gui.DialogOK
	case dialogDiscard:
		s = gui.DialogDiscard
	case dialogCancel:
		s = gui.DialogCancel
	default:
		s = gui.DialogError
	}
	return gui.NativeAlertResult{Status: s, ErrorMessage: errMsg}
}

// buildDialogResult constructs a PlatformDialogResult from status,
// paths, and error message.
func buildDialogResult(status int, paths []gui.PlatformPath,
	errMsg string) gui.PlatformDialogResult {
	var s gui.NativeDialogStatus
	switch status {
	case dialogOK:
		s = gui.DialogOK
	case dialogCancel:
		s = gui.DialogCancel
	default:
		s = gui.DialogError
	}
	return gui.PlatformDialogResult{
		Status:       s,
		Paths:        paths,
		ErrorMessage: errMsg,
	}
}
