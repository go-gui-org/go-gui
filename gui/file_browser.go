package gui

// file_browser.go — the in-window file browser. NativeOpenDialog,
// NativeSaveDialog and NativeFolderDialog show it when the platform has
// no file picker: a nil NativePlatform (tests, headless), a build with
// dialog_other.go, or Linux without zenity or kdialog (#831).
//
// It is a DialogCustom dialog, so it gets Escape, the focus trap and
// the single dialog slot from view_dialog.go. Its state lives in
// w.fileBrowser, not in a StateMap: a window shows at most one dialog,
// so it has at most one browser.
//
// The directory is read only when the folder changes, in an event
// handler or in showFileBrowser. The view function reads the cached
// entries and does no I/O, so a frame never waits on the disk.

import (
	"cmp"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
)

// fileBrowserMode selects which native dialog the browser stands in
// for.
type fileBrowserMode uint8

const (
	fileBrowserOpen fileBrowserMode = iota
	fileBrowserSave
	fileBrowserFolder
)

// fileBrowserFS is the part of the file system the browser reads. It is
// a seam for tests, which replace it with a fake (w.fileBrowserFS).
type fileBrowserFS interface {
	ReadDir(dir string) ([]fs.DirEntry, error)
	Stat(path string) (fs.FileInfo, error)
}

// osFileBrowserFS reads the real file system.
type osFileBrowserFS struct{}

func (osFileBrowserFS) ReadDir(dir string) ([]fs.DirEntry, error) { return os.ReadDir(dir) }
func (osFileBrowserFS) Stat(path string) (fs.FileInfo, error)     { return os.Stat(path) }

// fileBrowserParent is the name of the row that goes up one folder.
const fileBrowserParent = ".."

// fileBrowserListHeight is the height of the entry list, about ten rows.
// Fixed, so the list virtualizes from the first frame.
const fileBrowserListHeight = 220

// fileBrowserWidth is the dialog width. Wide enough for a long path in
// the path bar without the dialog changing width as the folder changes.
const fileBrowserWidth = 480

// The English strings below have no Locale field yet. They are the only
// text the browser shows that the locale does not supply.
const (
	fileBrowserReplacePrompt = "%s already exists. Replace it?"
	fileBrowserBadName       = "Enter a file name, not a path."
	fileBrowserNotFolder     = "Not a folder: %s"
)

// The browser's IDs. Each is absolute (it holds a ':'), so it is the
// same string inside the dialog scope and in SetFocus, which runs
// outside layout generation.
var (
	fileBrowserListID   = ScopeID(reservedDialogID, "fb_list")
	fileBrowserPathID   = ScopeID(reservedDialogID, "fb_path")
	fileBrowserNameID   = ScopeID(reservedDialogID, "fb_name")
	fileBrowserFilterID = ScopeID(reservedDialogID, "fb_filter")
	fileBrowserOKID     = ScopeID(reservedDialogID, "fb_ok")
	fileBrowserCancelID = ScopeID(reservedDialogID, "fb_cancel")
	fileBrowserRowsID   = ScopeID(reservedDialogID, "fb_row")
)

// fileBrowserRowID returns the ID of entry row i.
func fileBrowserRowID(i int) string {
	return ScopeIDN(fileBrowserRowsID, "", i)
}

// fileBrowserCfg is the part of the three native Cfgs the browser uses.
type fileBrowserCfg struct {
	onDone           func(NativeDialogResult, *Window)
	title            string
	startDir         string
	defaultName      string
	defaultExt       string // normalized: lower case, no dot
	filters          []NativeFileFilter
	mode             fileBrowserMode
	allowMultiple    bool
	confirmOverwrite bool
}

// fileBrowserEntry is one row: a file or a folder in the current folder.
type fileBrowserEntry struct {
	name  string
	isDir bool
}

// fileBrowserState is the state of the open browser.
type fileBrowserState struct {
	cfg fileBrowserCfg

	// filterExts holds the normalized extensions of each filter, in
	// cfg.filters order. An empty set lists every file.
	filterExts [][]string

	// all holds the folder's entries, sorted, with dotfiles dropped.
	// entries is all after the active filter, with ".." first when the
	// folder has a parent.
	all     []fileBrowserEntry
	entries []fileBrowserEntry

	dir      string
	pathText string // path bar text, edited apart from dir
	name     string // save mode: the file name field
	errText  string

	// confirmPath is the save target the user was asked about. A second
	// accept of the same path saves; any other path asks again.
	confirmPath string

	// marked holds the names picked with Ctrl/Cmd-click when
	// AllowMultiple is set. Empty means the cursor row is the choice.
	marked []string

	// lastClickRow and lastClickAt detect a double-click: Event has no
	// click count.
	lastClickRow int
	lastClickAt  int64 // UnixNano; 0 = no click yet

	filterIdx int
}

// nativeFileDialogUnavailable reports whether a platform result means
// "there is no file picker here", as opposed to a real error or a
// cancel. dialog_other.go says "unsupported"; Linux without zenity or
// kdialog says "no_dialog_tool".
func nativeFileDialogUnavailable(pr PlatformDialogResult) bool {
	return pr.Status == DialogError &&
		(pr.ErrorCode == "unsupported" || pr.ErrorCode == "no_dialog_tool")
}

// fileBrowserFSOf returns the file system the browser reads.
func fileBrowserFSOf(w *Window) fileBrowserFS {
	if w.fileBrowserFS != nil {
		return w.fileBrowserFS
	}
	return osFileBrowserFS{}
}

// showFileBrowser opens the browser in a dialog. Called from the
// native dialog impls, which run as queued commands, so it may call
// window APIs.
func showFileBrowser(w *Window, cfg fileBrowserCfg) {
	focus := fileBrowserListID
	if cfg.mode == fileBrowserSave {
		focus = fileBrowserNameID
	}
	w.Dialog(DialogCfg{
		DialogType: DialogCustom,
		Title:      cfg.title,
		Width:      fileBrowserWidth,
		FocusID:    focus,
		CustomView: fileBrowserView,
		// No OnCancelNo: Escape calls DialogDismiss, and DialogDismiss
		// reports the cancel (fileBrowserCancel).
		fileBrowser: true,
	})
	// Dialog runs a replaced browser's OnDone last. If that OnDone opened
	// its own dialog, this browser never got the slot: report the cancel
	// now. Building the state anyway would leave a browser that is not
	// on screen, and a later dismiss of that dialog would report for it.
	if !w.dialogCfg.fileBrowser {
		dispatchDialogDone(w, cfg.onDone, NativeDialogResult{Status: DialogCancel})
		return
	}
	// After Dialog, not before: Dialog cancels a browser that is
	// already open, and that must be the old one, not this one.
	fileBrowserInit(w, cfg)
}

// fileBrowserInit builds the browser state and reads the first folder.
// Apart from showFileBrowser so a golden can record the body without
// the dialog around it.
func fileBrowserInit(w *Window, cfg fileBrowserCfg) {
	st := &fileBrowserState{cfg: cfg, name: cfg.defaultName}
	for _, f := range cfg.filters {
		// The impls validated the filters before this point, so the
		// error is always nil here.
		exts, _ := nativeExtensionsFromFilters([]NativeFileFilter{f})
		st.filterExts = append(st.filterExts, exts)
	}
	w.fileBrowser = st

	// Try the start folder, then the working folder, then home. The
	// first readable one wins. If none is readable, the browser opens
	// on the start folder with the error shown, so the user can type a
	// path instead of the dialog failing.
	//
	// A relative start folder is made absolute, as the native pickers
	// return absolute paths and ".." must reach above the working folder.
	start := ""
	if cfg.startDir != "" {
		start = fileBrowserAbs(cfg.startDir)
	}
	var firstErr string
	for _, dir := range fileBrowserStartDirs(start) {
		if fileBrowserNavigate(w, dir) {
			// A fallback folder opened: still say why the start folder
			// did not, or the browser opens somewhere else in silence.
			st.errText = firstErr
			return
		}
		if firstErr == "" {
			firstErr = st.errText
		}
	}
	st.dir = start
	st.pathText = start
	st.errText = firstErr
}

// fileBrowserStartDirs lists the folders to try, in order. Empty and
// duplicate candidates are dropped.
func fileBrowserStartDirs(start string) []string {
	var dirs []string
	add := func(d string) {
		if d != "" && !slices.Contains(dirs, d) {
			dirs = append(dirs, d)
		}
	}
	add(start)
	if wd, err := os.Getwd(); err == nil {
		add(wd)
	}
	if home, err := os.UserHomeDir(); err == nil {
		add(home)
	}
	return dirs
}

// fileBrowserAbs cleans dir and makes it absolute. If the working
// folder cannot be read, the cleaned path is the best there is.
func fileBrowserAbs(dir string) string {
	if abs, err := filepath.Abs(dir); err == nil {
		return abs
	}
	return filepath.Clean(dir)
}

// fileBrowserNavigate reads dir and makes it the current folder. On an
// error it keeps the current folder, sets errText and returns false.
func fileBrowserNavigate(w *Window, dir string) bool {
	st := w.fileBrowser
	if st == nil {
		return false
	}
	dir = fileBrowserAbs(dir)
	fsys := fileBrowserFSOf(w)
	des, err := fsys.ReadDir(dir)
	if err != nil {
		// A folder that exists but may not be read is not "not a
		// folder": show the OS error so the user does not hunt a typo.
		if errors.Is(err, fs.ErrPermission) {
			st.errText = err.Error()
		} else {
			st.errText = fmt.Sprintf(fileBrowserNotFolder, dir)
		}
		return false
	}
	all := make([]fileBrowserEntry, 0, len(des))
	for _, de := range des {
		name := de.Name()
		if strings.HasPrefix(name, ".") {
			continue
		}
		isDir := de.IsDir()
		// A link to a folder reads as a file from its DirEntry. Stat
		// follows the link, so it is listed as what it points at.
		if de.Type()&fs.ModeSymlink != 0 {
			if fi, serr := fsys.Stat(filepath.Join(dir, name)); serr == nil {
				isDir = fi.IsDir()
			}
		}
		all = append(all, fileBrowserEntry{name: name, isDir: isDir})
	}
	slices.SortFunc(all, func(a, b fileBrowserEntry) int {
		if a.isDir != b.isDir {
			if a.isDir {
				return -1
			}
			return 1
		}
		if c := cmp.Compare(strings.ToLower(a.name), strings.ToLower(b.name)); c != 0 {
			return c
		}
		return cmp.Compare(a.name, b.name)
	})
	st.all = all
	st.dir = dir
	st.pathText = dir
	st.errText = ""
	st.confirmPath = ""
	st.marked = st.marked[:0]
	st.lastClickAt = 0
	fileBrowserRefilter(st)
	w.SetVirtualListFocusedIndex(fileBrowserListID, 0)
	return true
}

// fileBrowserRefilter rebuilds entries from all and the active filter.
func fileBrowserRefilter(st *fileBrowserState) {
	var exts []string
	if st.filterIdx < len(st.filterExts) {
		exts = st.filterExts[st.filterIdx]
	}
	st.entries = st.entries[:0]
	// No folder was ever read when dir is "": filepath.Dir("") is ".",
	// so without the guard ".." would lead to the working folder.
	if parent := filepath.Dir(st.dir); st.dir != "" && parent != st.dir {
		st.entries = append(st.entries, fileBrowserEntry{name: fileBrowserParent, isDir: true})
	}
	for _, e := range st.all {
		switch {
		case e.isDir:
		case st.cfg.mode == fileBrowserFolder:
			continue
		case len(exts) > 0 && !slices.Contains(exts, fileBrowserExt(e.name)):
			continue
		}
		st.entries = append(st.entries, e)
	}
}

// fileBrowserExt returns name's extension, lower case, without the dot.
func fileBrowserExt(name string) string {
	return strings.ToLower(strings.TrimPrefix(filepath.Ext(name), "."))
}

// fileBrowserSetFilter makes filter i the active one.
func fileBrowserSetFilter(w *Window, i int) {
	st := w.fileBrowser
	if st == nil || i < 0 || i >= len(st.filterExts) {
		return
	}
	st.filterIdx = i
	st.marked = st.marked[:0]
	fileBrowserRefilter(st)
	w.SetVirtualListFocusedIndex(fileBrowserListID, 0)
}

// fileBrowserEntryPath returns the full path of entry e.
func fileBrowserEntryPath(st *fileBrowserState, e fileBrowserEntry) string {
	if e.name == fileBrowserParent {
		return filepath.Dir(st.dir)
	}
	return filepath.Join(st.dir, e.name)
}

// fileBrowserCursor returns the entry under the keyboard cursor.
func fileBrowserCursor(w *Window) (fileBrowserEntry, bool) {
	st := w.fileBrowser
	i := w.VirtualListFocusedIndex(fileBrowserListID)
	if st == nil || i < 0 || i >= len(st.entries) {
		return fileBrowserEntry{}, false
	}
	return st.entries[i], true
}

// fileBrowserOpenChoice returns the files open mode would accept now:
// the marked files, or else the file under the cursor. It returns nil
// when the choice holds a folder or nothing.
func fileBrowserOpenChoice(w *Window) []string {
	st := w.fileBrowser
	if st == nil {
		return nil
	}
	if len(st.marked) > 0 {
		paths := make([]string, 0, len(st.marked))
		for _, name := range st.marked {
			paths = append(paths, filepath.Join(st.dir, name))
		}
		return paths
	}
	if e, ok := fileBrowserCursor(w); ok && !e.isDir {
		return []string{fileBrowserEntryPath(st, e)}
	}
	return nil
}

// fileBrowserCanAccept reports whether OK would do something now.
func fileBrowserCanAccept(w *Window) bool {
	st := w.fileBrowser
	if st == nil {
		return false
	}
	switch st.cfg.mode {
	case fileBrowserOpen:
		// Same answer as len(fileBrowserOpenChoice(w)) > 0, without
		// building the path slice: the view asks this every frame.
		if len(st.marked) > 0 {
			return true
		}
		e, ok := fileBrowserCursor(w)
		return ok && !e.isDir
	case fileBrowserSave:
		return strings.TrimSpace(st.name) != ""
	}
	// Folder mode returns the current folder, so OK needs only a folder
	// that was read. A failed path-bar jump keeps the old one; all is
	// nil only when no folder could be read at all.
	return st.all != nil
}

// fileBrowserActivate acts on entry i: a folder is opened, a file is
// accepted (open) or taken as the name and saved (save).
func fileBrowserActivate(w *Window, i int) {
	st := w.fileBrowser
	if st == nil || i < 0 || i >= len(st.entries) {
		return
	}
	e := st.entries[i]
	if e.isDir {
		fileBrowserNavigate(w, fileBrowserEntryPath(st, e))
		return
	}
	switch st.cfg.mode {
	case fileBrowserOpen:
		fileBrowserFinish(w, []string{fileBrowserEntryPath(st, e)})
	case fileBrowserSave:
		st.name = e.name
		fileBrowserAccept(w)
	}
}

// fileBrowserAccept is the OK action for the current mode.
func fileBrowserAccept(w *Window) {
	st := w.fileBrowser
	if st == nil {
		return
	}
	switch st.cfg.mode {
	case fileBrowserOpen:
		if paths := fileBrowserOpenChoice(w); len(paths) > 0 {
			fileBrowserFinish(w, paths)
			return
		}
		// A folder under the cursor: OK opens it, as Enter does.
		if e, ok := fileBrowserCursor(w); ok && e.isDir {
			fileBrowserNavigate(w, fileBrowserEntryPath(st, e))
		}
	case fileBrowserSave:
		fileBrowserAcceptSave(w)
	case fileBrowserFolder:
		fileBrowserFinish(w, []string{st.dir})
	}
}

// fileBrowserAcceptSave checks the name, adds the default extension and
// asks before replacing a file when ConfirmOverwrite is set.
func fileBrowserAcceptSave(w *Window) {
	st := w.fileBrowser
	name := strings.TrimSpace(st.name)
	if fileBrowserBadSaveName(name, runtime.GOOS) {
		st.errText = fileBrowserBadName
		return
	}
	fsys := fileBrowserFSOf(w)
	// The bare name first: a folder name has no extension, so adding
	// the default one before this check would save "sub.txt" instead of
	// going into "sub".
	if fi, err := fsys.Stat(filepath.Join(st.dir, name)); err == nil && fi.IsDir() {
		// As a native save dialog does. If the folder cannot be read,
		// keep the typed name; navigate has set the error.
		if fileBrowserNavigate(w, filepath.Join(st.dir, name)) {
			st.name = ""
		}
		return
	}
	if filepath.Ext(name) == "" && st.cfg.defaultExt != "" {
		name += "." + st.cfg.defaultExt
	}
	target := filepath.Join(st.dir, name)
	fi, err := fsys.Stat(target)
	if err != nil && !errors.Is(err, fs.ErrNotExist) {
		// Not "no such file": the check failed, so whether the save
		// would replace a file is unknown. Say so rather than save.
		st.errText = err.Error()
		return
	}
	if err == nil && fi.IsDir() {
		// The name is a folder: go into it, as a native save dialog
		// does. If it cannot be read, keep the typed name; navigate
		// has set the error.
		if fileBrowserNavigate(w, target) {
			st.name = ""
		}
		return
	}
	if err == nil && st.cfg.confirmOverwrite && st.confirmPath != target {
		st.confirmPath = target
		st.errText = ""
		return
	}
	fileBrowserFinish(w, []string{target})
}

// fileBrowserBadSaveName reports whether name cannot be a plain file
// name in the current folder. goos is a parameter so tests cover the
// Windows rules on every OS.
//
// On Windows the native save dialog also refuses <>:"|?*. ':' matters
// most: "a.txt:x" names an NTFS alternate data stream, so the save
// would land hidden inside a.txt, and "C:x" is relative to drive C.
func fileBrowserBadSaveName(name, goos string) bool {
	if name == "" || name == "." || name == fileBrowserParent ||
		strings.ContainsAny(name, `/\`) || strings.ContainsRune(name, 0) {
		return true
	}
	return goos == "windows" && strings.ContainsAny(name, `<>:"|?*`)
}

// fileBrowserFinish closes the dialog and reports paths as DialogOK.
//
// The state is cleared before DialogDismiss, so the dismiss does not
// also report a cancel.
func fileBrowserFinish(w *Window, paths []string) {
	st := w.fileBrowser
	w.fileBrowser = nil
	w.DialogDismiss()
	fileBrowserReport(w, st, DialogOK, paths)
}

// fileBrowserCancel ends an open browser with DialogCancel. Dialog and
// DialogDismiss call it, so the browser reports exactly once however
// it loses the dialog slot: Escape, Cancel, a second file dialog, an
// app dialog that replaces it, or an app DialogDismiss. Without it the
// caller's OnDone would never run. A no-op when no browser is open.
func fileBrowserCancel(w *Window) {
	st := w.fileBrowser
	if st == nil {
		return
	}
	w.fileBrowser = nil
	fileBrowserReport(w, st, DialogCancel, nil)
}

// fileBrowserReport calls st's OnDone. The caller has already cleared
// w.fileBrowser.
func fileBrowserReport(w *Window, st *fileBrowserState, status NativeDialogStatus, paths []string) {
	if st == nil {
		return
	}
	res := NativeDialogResult{Status: status}
	if len(paths) > 0 {
		res.Paths = make([]AccessiblePath, len(paths))
		for i, p := range paths {
			// No security-scoped grant: the browser opens nothing
			// through the OS picker, so the zero Grant is the truth.
			res.Paths[i] = AccessiblePath{Path: p}
		}
	}
	dispatchDialogDone(w, st.cfg.onDone, res)
}
