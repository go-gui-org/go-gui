package gui

import (
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"testing/fstest"
)

// mapDirFS adapts a fstest.MapFS to fileBrowserFS. The browser works
// with OS paths ("/proj/a.txt", or "\proj\a.txt" on Windows); MapFS
// takes slash paths with no leading slash ("proj/a.txt").
type mapDirFS struct{ m fstest.MapFS }

func (f mapDirFS) rel(p string) string {
	p = filepath.Clean(p)
	p = p[len(filepath.VolumeName(p)):]
	p = strings.TrimLeft(filepath.ToSlash(p), "/")
	if p == "" {
		return "."
	}
	return p
}

func (f mapDirFS) ReadDir(dir string) ([]fs.DirEntry, error) {
	return fs.ReadDir(f.m, f.rel(dir))
}

func (f mapDirFS) Stat(path string) (fs.FileInfo, error) {
	return fs.Stat(f.m, f.rel(path))
}

// fbRoot is the start folder of every test. FromSlash and
// fileBrowserAbs keep the expected paths in the form the browser
// returns: the browser makes every folder absolute, which on Windows
// adds the drive to "\proj".
var fbRoot = fileBrowserAbs(filepath.FromSlash("/proj"))

// fileBrowserTestWindow returns a test window with a fake file
// system and a nil native platform, so every file dialog falls back
// to the in-window browser.
func fileBrowserTestWindow(t *testing.T) *Window {
	t.Helper()
	w := NewTestWindow(t, WindowCfg{State: new(int)})
	w.fileBrowserFS = mapDirFS{fstest.MapFS{
		"proj/a.txt":     {},
		"proj/b.md":      {},
		"proj/.hidden":   {},
		"proj/sub/c.txt": {},
	}}
	w.SetView(func(*Window) View {
		return Column(ContainerCfg{Sizing: FillFill})
	})
	return w
}

// fbEntryNames returns the names the browser lists now.
func fbEntryNames(w *Window) []string {
	if w.fileBrowser == nil {
		return nil
	}
	names := make([]string, len(w.fileBrowser.entries))
	for i, e := range w.fileBrowser.entries {
		names[i] = e.name
	}
	return names
}

func TestFileBrowserOpenFallbackOnNilPlatform(t *testing.T) {
	w := fileBrowserTestWindow(t)
	var got NativeDialogResult
	called := false
	nativeOpenDialogImpl(w, NativeOpenDialogCfg{
		StartDir: fbRoot,
		Filters:  []NativeFileFilter{{Name: "Text", Extensions: []string{"txt"}}},
		OnDone: func(r NativeDialogResult, _ *Window) {
			got, called = r, true
		},
	}, false)
	w.TestRender(nil)
	if !w.DialogIsVisible() {
		t.Fatal("browser dialog not shown for a nil platform")
	}
	if called {
		t.Fatalf("OnDone fired before the user chose: %+v", got)
	}
	// ".." first, folders before files, dotfiles hidden, b.md
	// filtered out by the txt filter.
	if names := fbEntryNames(w); !slices.Equal(names, []string{"..", "sub", "a.txt"}) {
		t.Fatalf("entries = %q", names)
	}
	// Move to a.txt and press Enter: the file is accepted.
	for range 2 {
		if err := w.TestKey(fileBrowserListID, KeyDown, 0); err != nil {
			t.Fatal(err)
		}
	}
	if err := w.TestKey(fileBrowserListID, KeyEnter, 0); err != nil {
		t.Fatal(err)
	}
	if !called || got.Status != DialogOK {
		t.Fatalf("result = %+v, called %v", got, called)
	}
	want := filepath.Join(fbRoot, "a.txt")
	if p := got.PathStrings(); !slices.Equal(p, []string{want}) {
		t.Fatalf("paths = %q, want %q", p, want)
	}
	if w.DialogIsVisible() {
		t.Fatal("dialog still visible after accept")
	}
}

func TestFileBrowserNavigateIntoFolderAndUp(t *testing.T) {
	w := fileBrowserTestWindow(t)
	nativeOpenDialogImpl(w, NativeOpenDialogCfg{StartDir: fbRoot}, false)
	w.TestRender(nil)
	// No filter: every file but the dotfile is listed.
	if names := fbEntryNames(w); !slices.Equal(names, []string{"..", "sub", "a.txt", "b.md"}) {
		t.Fatalf("entries = %q", names)
	}
	if err := w.TestKey(fileBrowserListID, KeyDown, 0); err != nil {
		t.Fatal(err)
	}
	if err := w.TestKey(fileBrowserListID, KeyEnter, 0); err != nil {
		t.Fatal(err)
	}
	if d := w.fileBrowser.dir; d != filepath.Join(fbRoot, "sub") {
		t.Fatalf("dir = %q after Enter on sub", d)
	}
	if names := fbEntryNames(w); !slices.Equal(names, []string{"..", "c.txt"}) {
		t.Fatalf("entries = %q", names)
	}
	// The cursor is back on row 0, "..": Enter goes up.
	if err := w.TestKey(fileBrowserListID, KeyEnter, 0); err != nil {
		t.Fatal(err)
	}
	if d := w.fileBrowser.dir; d != fbRoot {
		t.Fatalf("dir = %q after Enter on ..", d)
	}
}

func TestFileBrowserDoubleClickOpensFolder(t *testing.T) {
	w := fileBrowserTestWindow(t)
	nativeOpenDialogImpl(w, NativeOpenDialogCfg{StartDir: fbRoot}, false)
	w.TestRender(nil)
	// The test clock is pinned, so two clicks land inside the
	// double-click gap.
	now := goldenInstant()
	w.setVirtualNow(&now)
	for range 2 {
		if err := w.TestClick(fileBrowserRowID(1)); err != nil {
			t.Fatal(err)
		}
	}
	if d := w.fileBrowser.dir; d != filepath.Join(fbRoot, "sub") {
		t.Fatalf("dir = %q after double-click on sub", d)
	}
}

func TestFileBrowserPathBarNavigates(t *testing.T) {
	w := fileBrowserTestWindow(t)
	nativeOpenDialogImpl(w, NativeOpenDialogCfg{StartDir: filepath.Join(fbRoot, "sub")}, false)
	w.TestRender(nil)
	w.fileBrowser.pathText = fbRoot
	if err := w.TestKey(fileBrowserPathID, KeyEnter, 0); err != nil {
		t.Fatal(err)
	}
	if d := w.fileBrowser.dir; d != fbRoot {
		t.Fatalf("dir = %q after path bar Enter", d)
	}
	// A path that is not a folder leaves the folder alone and says so.
	w.fileBrowser.pathText = filepath.Join(fbRoot, "missing")
	if err := w.TestKey(fileBrowserPathID, KeyEnter, 0); err != nil {
		t.Fatal(err)
	}
	if d := w.fileBrowser.dir; d != fbRoot {
		t.Fatalf("dir = %q after a bad path", d)
	}
	if w.fileBrowser.errText == "" {
		t.Fatal("no error shown for a bad path")
	}
}

func TestFileBrowserSaveAddsDefaultExtension(t *testing.T) {
	w := fileBrowserTestWindow(t)
	var got NativeDialogResult
	nativeSaveDialogImpl(w, NativeSaveDialogCfg{
		StartDir:         fbRoot,
		DefaultExtension: ".txt",
		OnDone:           func(r NativeDialogResult, _ *Window) { got = r },
	}, false)
	w.TestRender(nil)
	if err := w.TestType(fileBrowserNameID, "notes"); err != nil {
		t.Fatal(err)
	}
	if err := w.TestKey(fileBrowserNameID, KeyEnter, 0); err != nil {
		t.Fatal(err)
	}
	want := filepath.Join(fbRoot, "notes.txt")
	if got.Status != DialogOK || !slices.Equal(got.PathStrings(), []string{want}) {
		t.Fatalf("result = %+v, want %q", got, want)
	}
}

func TestFileBrowserSaveRejectsBadName(t *testing.T) {
	for _, name := range []string{"..", "a/b", "."} {
		w := fileBrowserTestWindow(t)
		called := false
		nativeSaveDialogImpl(w, NativeSaveDialogCfg{
			StartDir:    fbRoot,
			DefaultName: name,
			OnDone:      func(NativeDialogResult, *Window) { called = true },
		}, false)
		w.TestRender(nil)
		if err := w.TestKey(fileBrowserNameID, KeyEnter, 0); err != nil {
			t.Fatal(err)
		}
		if called || w.fileBrowser == nil || w.fileBrowser.errText == "" {
			t.Fatalf("name %q: called %v, state %+v", name, called, w.fileBrowser)
		}
	}
}

func TestFileBrowserSaveConfirmsOverwrite(t *testing.T) {
	w := fileBrowserTestWindow(t)
	var got NativeDialogResult
	called := false
	nativeSaveDialogImpl(w, NativeSaveDialogCfg{
		StartDir:         fbRoot,
		DefaultName:      "a",
		DefaultExtension: "txt",
		ConfirmOverwrite: true,
		OnDone: func(r NativeDialogResult, _ *Window) {
			got, called = r, true
		},
	}, false)
	w.TestRender(nil)
	if err := w.TestKey(fileBrowserNameID, KeyEnter, 0); err != nil {
		t.Fatal(err)
	}
	// a.txt exists: the first Enter asks, it does not save.
	if called {
		t.Fatalf("saved over an existing file without asking: %+v", got)
	}
	want := filepath.Join(fbRoot, "a.txt")
	if w.fileBrowser.confirmPath != want {
		t.Fatalf("confirmPath = %q, want %q", w.fileBrowser.confirmPath, want)
	}
	// The second Enter confirms.
	if err := w.TestKey(fileBrowserNameID, KeyEnter, 0); err != nil {
		t.Fatal(err)
	}
	if !called || got.Status != DialogOK || got.PathStrings()[0] != want {
		t.Fatalf("result = %+v", got)
	}
}

func TestFileBrowserSaveClickOnFileFillsName(t *testing.T) {
	w := fileBrowserTestWindow(t)
	nativeSaveDialogImpl(w, NativeSaveDialogCfg{StartDir: fbRoot}, false)
	w.TestRender(nil)
	// Row 2 is a.txt (row 0 "..", row 1 "sub").
	if err := w.TestClick(fileBrowserRowID(2)); err != nil {
		t.Fatal(err)
	}
	if w.fileBrowser.name != "a.txt" {
		t.Fatalf("name = %q after clicking a.txt", w.fileBrowser.name)
	}
}

func TestFileBrowserFolderReturnsCurrentFolder(t *testing.T) {
	w := fileBrowserTestWindow(t)
	var got NativeDialogResult
	nativeFolderDialogImpl(w, NativeFolderDialogCfg{
		StartDir: fbRoot,
		OnDone:   func(r NativeDialogResult, _ *Window) { got = r },
	}, false)
	w.TestRender(nil)
	// Folder mode lists folders only.
	if names := fbEntryNames(w); !slices.Equal(names, []string{"..", "sub"}) {
		t.Fatalf("entries = %q", names)
	}
	if err := w.TestClick(fileBrowserOKID); err != nil {
		t.Fatal(err)
	}
	if got.Status != DialogOK || !slices.Equal(got.PathStrings(), []string{fbRoot}) {
		t.Fatalf("result = %+v", got)
	}
}

func TestFileBrowserOpenMultiple(t *testing.T) {
	w := fileBrowserTestWindow(t)
	var got NativeDialogResult
	nativeOpenDialogImpl(w, NativeOpenDialogCfg{
		StartDir:      fbRoot,
		AllowMultiple: true,
		OnDone:        func(r NativeDialogResult, _ *Window) { got = r },
	}, false)
	w.TestRender(nil)
	w.fileBrowser.marked = []string{"a.txt", "b.md"}
	w.TestRender(nil)
	if err := w.TestClick(fileBrowserOKID); err != nil {
		t.Fatal(err)
	}
	want := []string{filepath.Join(fbRoot, "a.txt"), filepath.Join(fbRoot, "b.md")}
	if got.Status != DialogOK || !slices.Equal(got.PathStrings(), want) {
		t.Fatalf("result = %+v, want %q", got, want)
	}
}

func TestFileBrowserOKDisabledOnFolderRow(t *testing.T) {
	w := fileBrowserTestWindow(t)
	nativeOpenDialogImpl(w, NativeOpenDialogCfg{StartDir: fbRoot}, false)
	w.TestRender(nil)
	// The cursor starts on "..": nothing to open yet.
	if err := w.TestClick(fileBrowserOKID); err == nil {
		t.Fatal("OK accepted with a folder row under the cursor")
	}
}

func TestFileBrowserEscapeCancels(t *testing.T) {
	w := fileBrowserTestWindow(t)
	var got NativeDialogResult
	called := false
	nativeOpenDialogImpl(w, NativeOpenDialogCfg{
		StartDir: fbRoot,
		OnDone: func(r NativeDialogResult, _ *Window) {
			got, called = r, true
		},
	}, false)
	w.TestRender(nil)
	if err := w.TestKey(fileBrowserListID, KeyEscape, 0); err != nil {
		t.Fatal(err)
	}
	if !called || got.Status != DialogCancel {
		t.Fatalf("result = %+v, called %v", got, called)
	}
	if w.DialogIsVisible() || w.fileBrowser != nil {
		t.Fatal("browser still open after Escape")
	}
}

func TestFileBrowserCancelButton(t *testing.T) {
	w := fileBrowserTestWindow(t)
	var got NativeDialogResult
	nativeFolderDialogImpl(w, NativeFolderDialogCfg{
		StartDir: fbRoot,
		OnDone:   func(r NativeDialogResult, _ *Window) { got = r },
	}, false)
	w.TestRender(nil)
	if err := w.TestClick(fileBrowserCancelID); err != nil {
		t.Fatal(err)
	}
	if got.Status != DialogCancel || w.DialogIsVisible() {
		t.Fatalf("result = %+v, visible %v", got, w.DialogIsVisible())
	}
}

// unavailablePlatform reports a missing picker the way the backends
// do: "unsupported" from dialog_other.go, "no_dialog_tool" from Linux
// without zenity or kdialog.
type unavailablePlatform struct {
	noopNativePlatform
	code string
}

func (p unavailablePlatform) ShowOpenDialog(_, _ string, _ []string, _ bool) PlatformDialogResult {
	return PlatformDialogResult{Status: DialogError, ErrorCode: p.code}
}

func (p unavailablePlatform) ShowSaveDialog(_, _, _, _ string, _ []string, _ bool) PlatformDialogResult {
	return PlatformDialogResult{Status: DialogError, ErrorCode: p.code}
}

func (p unavailablePlatform) ShowFolderDialog(_, _ string) PlatformDialogResult {
	return PlatformDialogResult{Status: DialogError, ErrorCode: p.code}
}

func TestFileBrowserFallbackOnUnavailablePlatform(t *testing.T) {
	shows := []struct {
		name string
		show func(w *Window, onDone func(NativeDialogResult, *Window))
	}{
		{"open", func(w *Window, onDone func(NativeDialogResult, *Window)) {
			nativeOpenDialogImpl(w, NativeOpenDialogCfg{StartDir: fbRoot, OnDone: onDone}, false)
		}},
		{"save", func(w *Window, onDone func(NativeDialogResult, *Window)) {
			nativeSaveDialogImpl(w, NativeSaveDialogCfg{StartDir: fbRoot, OnDone: onDone}, false)
		}},
		{"folder", func(w *Window, onDone func(NativeDialogResult, *Window)) {
			nativeFolderDialogImpl(w, NativeFolderDialogCfg{StartDir: fbRoot, OnDone: onDone}, false)
		}},
	}
	for _, code := range []string{"unsupported", "no_dialog_tool"} {
		for _, s := range shows {
			t.Run(code+"/"+s.name, func(t *testing.T) {
				w := fileBrowserTestWindow(t)
				w.nativePlatform = unavailablePlatform{code: code}
				called := false
				s.show(w, func(NativeDialogResult, *Window) { called = true })
				if called || !w.DialogIsVisible() {
					t.Fatalf("called %v, visible %v", called, w.DialogIsVisible())
				}
				if w.nativeDialogVisible {
					t.Fatal("native-dialog flag left set")
				}
			})
		}
	}
}

func TestFileBrowserNoFallbackOnPlatformError(t *testing.T) {
	w := fileBrowserTestWindow(t)
	w.nativePlatform = unavailablePlatform{code: "platform_error"}
	var got NativeDialogResult
	nativeOpenDialogImpl(w, NativeOpenDialogCfg{
		StartDir: fbRoot,
		OnDone:   func(r NativeDialogResult, _ *Window) { got = r },
	}, false)
	if w.DialogIsVisible() || got.ErrorCode != "platform_error" {
		t.Fatalf("a real platform error opened the browser: %+v", got)
	}
}

func TestFileBrowserUnreadableStartDirShowsError(t *testing.T) {
	w := fileBrowserTestWindow(t)
	nativeOpenDialogImpl(w, NativeOpenDialogCfg{StartDir: filepath.Join(fbRoot, "missing")}, false)
	w.TestRender(nil)
	// The fallbacks (working and home folder) do not exist in the fake
	// file system either, so the browser stays open with the error.
	if !w.DialogIsVisible() || w.fileBrowser.errText == "" {
		t.Fatalf("visible %v, state %+v", w.DialogIsVisible(), w.fileBrowser)
	}
}

func TestFileBrowserNoDebugFindings(t *testing.T) {
	shows := []func(w *Window){
		func(w *Window) { nativeOpenDialogImpl(w, NativeOpenDialogCfg{StartDir: fbRoot}, false) },
		func(w *Window) {
			nativeSaveDialogImpl(w, NativeSaveDialogCfg{
				StartDir: fbRoot,
				Filters: []NativeFileFilter{
					{Name: "Text", Extensions: []string{"txt"}},
					{Name: "Markdown", Extensions: []string{"md"}},
				},
			}, false)
		},
		func(w *Window) { nativeFolderDialogImpl(w, NativeFolderDialogCfg{StartDir: fbRoot}, false) },
	}
	for i, show := range shows {
		w := fileBrowserTestWindow(t)
		show(w)
		w.TestRender(nil)
		if found := w.TestFindings(DebugAll); len(found) > 0 {
			t.Errorf("mode %d: findings %q", i, found)
		}
	}
}

func TestFileBrowserFilterSelectChangesListing(t *testing.T) {
	w := fileBrowserTestWindow(t)
	nativeOpenDialogImpl(w, NativeOpenDialogCfg{
		StartDir: fbRoot,
		Filters: []NativeFileFilter{
			{Name: "Text", Extensions: []string{"txt"}},
			{Name: "Markdown", Extensions: []string{"md"}},
		},
	}, false)
	w.TestRender(nil)
	if names := fbEntryNames(w); !slices.Equal(names, []string{"..", "sub", "a.txt"}) {
		t.Fatalf("entries = %q", names)
	}
	fileBrowserSetFilter(w, 1)
	if names := fbEntryNames(w); !slices.Equal(names, []string{"..", "sub", "b.md"}) {
		t.Fatalf("entries after filter change = %q", names)
	}
}

// A failed path-bar jump keeps the current folder, so folder mode can
// still return it. The folder here is empty, so only "was a folder
// read" can tell, not "does it hold entries".
func TestFileBrowserFolderOKAfterBadPath(t *testing.T) {
	w := fileBrowserTestWindow(t)
	w.fileBrowserFS = mapDirFS{fstest.MapFS{"proj": {Mode: fs.ModeDir}}}
	var got NativeDialogResult
	nativeFolderDialogImpl(w, NativeFolderDialogCfg{
		StartDir: fbRoot,
		OnDone:   func(r NativeDialogResult, _ *Window) { got = r },
	}, false)
	w.TestRender(nil)
	w.fileBrowser.pathText = filepath.Join(fbRoot, "missing")
	if err := w.TestKey(fileBrowserPathID, KeyEnter, 0); err != nil {
		t.Fatal(err)
	}
	if err := w.TestClick(fileBrowserOKID); err != nil {
		t.Fatal(err)
	}
	if got.Status != DialogOK || !slices.Equal(got.PathStrings(), []string{fbRoot}) {
		t.Fatalf("result = %+v", got)
	}
}

// A browser that loses its dialog slot reports DialogCancel, so a
// caller waiting on OnDone always hears back: a second file dialog, an
// app dialog that replaces it, or an app DialogDismiss.
func TestFileBrowserReplacedReportsCancel(t *testing.T) {
	cases := []struct {
		name    string
		replace func(w *Window)
	}{
		{"second file dialog", func(w *Window) {
			nativeFolderDialogImpl(w, NativeFolderDialogCfg{StartDir: fbRoot}, false)
		}},
		{"app dialog", func(w *Window) {
			w.Dialog(DialogCfg{Title: "other"})
		}},
		{"app dismiss", func(w *Window) { w.DialogDismiss() }},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			w := fileBrowserTestWindow(t)
			var got NativeDialogResult
			calls := 0
			nativeOpenDialogImpl(w, NativeOpenDialogCfg{
				StartDir: fbRoot,
				OnDone: func(r NativeDialogResult, _ *Window) {
					got = r
					calls++
				},
			}, false)
			w.TestRender(nil)
			c.replace(w)
			if calls != 1 || got.Status != DialogCancel {
				t.Fatalf("calls %d, result %+v", calls, got)
			}
		})
	}
	// The second file dialog is the one left open.
	w := fileBrowserTestWindow(t)
	nativeOpenDialogImpl(w, NativeOpenDialogCfg{StartDir: fbRoot}, false)
	nativeFolderDialogImpl(w, NativeFolderDialogCfg{StartDir: fbRoot}, false)
	if w.fileBrowser == nil || w.fileBrowser.cfg.mode != fileBrowserFolder || !w.DialogIsVisible() {
		t.Fatalf("second browser not open: %+v", w.fileBrowser)
	}
}

// Every accept path reports exactly once.
func TestFileBrowserAcceptReportsOnce(t *testing.T) {
	w := fileBrowserTestWindow(t)
	calls := 0
	nativeFolderDialogImpl(w, NativeFolderDialogCfg{
		StartDir: fbRoot,
		OnDone:   func(NativeDialogResult, *Window) { calls++ },
	}, false)
	w.TestRender(nil)
	if err := w.TestClick(fileBrowserOKID); err != nil {
		t.Fatal(err)
	}
	if calls != 1 {
		t.Fatalf("OnDone calls = %d, want 1", calls)
	}
}

// A path-bar entry that starts at the root is taken as given, not
// joined onto the current folder. On Windows "\proj" has no drive
// letter, so filepath.IsAbs alone calls it relative.
func TestFileBrowserPathBarRootedPath(t *testing.T) {
	// "\proj" on Windows: rooted, but with no drive, so IsAbs is false.
	rooted := filepath.FromSlash("/proj")
	if got := fileBrowserResolve(filepath.Join(fbRoot, "sub"), rooted); got != rooted {
		t.Fatalf("rooted path resolved to %q, want %q", got, rooted)
	}
	if got := fileBrowserResolve(fbRoot, "sub"); got != filepath.Join(fbRoot, "sub") {
		t.Fatalf("relative path resolved to %q", got)
	}
}

// workingPlatform has a native picker. A call to it is counted, so a
// test can prove the in-window methods never reach it.
type workingPlatform struct {
	noopNativePlatform
	calls *int
}

func (p workingPlatform) ShowOpenDialog(_, _ string, _ []string, _ bool) PlatformDialogResult {
	*p.calls++
	return PlatformDialogResult{Status: DialogCancel}
}

func (p workingPlatform) ShowSaveDialog(_, _, _, _ string, _ []string, _ bool) PlatformDialogResult {
	*p.calls++
	return PlatformDialogResult{Status: DialogCancel}
}

func (p workingPlatform) ShowFolderDialog(_, _ string) PlatformDialogResult {
	*p.calls++
	return PlatformDialogResult{Status: DialogCancel}
}

func TestInWindowDialogsSkipNativePicker(t *testing.T) {
	shows := []struct {
		name string
		mode fileBrowserMode
		show func(w *Window)
	}{
		{"open", fileBrowserOpen, func(w *Window) {
			w.InWindowOpenDialog(NativeOpenDialogCfg{StartDir: fbRoot})
		}},
		{"save", fileBrowserSave, func(w *Window) {
			w.InWindowSaveDialog(NativeSaveDialogCfg{StartDir: fbRoot})
		}},
		{"folder", fileBrowserFolder, func(w *Window) {
			w.InWindowFolderDialog(NativeFolderDialogCfg{StartDir: fbRoot})
		}},
	}
	for _, s := range shows {
		t.Run(s.name, func(t *testing.T) {
			w := fileBrowserTestWindow(t)
			calls := 0
			w.nativePlatform = workingPlatform{calls: &calls}
			s.show(w)
			// The methods queue a command; settle runs it as FrameFn would.
			w.settle()
			if calls != 0 {
				t.Fatalf("native picker called %d times", calls)
			}
			if !w.DialogIsVisible() || w.fileBrowser == nil || w.fileBrowser.cfg.mode != s.mode {
				t.Fatalf("visible %v, state %+v", w.DialogIsVisible(), w.fileBrowser)
			}
		})
	}
}

func TestInWindowDialogsRejectBadCfg(t *testing.T) {
	w := fileBrowserTestWindow(t)
	var got NativeDialogResult
	w.InWindowOpenDialog(NativeOpenDialogCfg{
		StartDir: fbRoot,
		Filters:  []NativeFileFilter{{Name: "Bad", Extensions: []string{"a/b"}}},
		OnDone:   func(r NativeDialogResult, _ *Window) { got = r },
	})
	w.settle()
	if w.DialogIsVisible() || got.ErrorCode != "invalid_cfg" {
		t.Fatalf("bad Cfg opened the browser: %+v", got)
	}
}

// fbWorkingDirFS returns a fake file system whose only folder is the
// real working folder, with proj/a.txt below it and x.txt in it. The
// browser falls back to the working folder and resolves relative paths
// against it, so these tests need it to exist in the fake.
func fbWorkingDirFS(t *testing.T) (mapDirFS, string) {
	t.Helper()
	wd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	m := mapDirFS{}
	base := m.rel(wd)
	m.m = fstest.MapFS{
		base + "/x.txt":      {},
		base + "/proj/a.txt": {},
	}
	return m, wd
}

func TestFileBrowserRelativeStartDirIsAbsolute(t *testing.T) {
	w := fileBrowserTestWindow(t)
	fsys, wd := fbWorkingDirFS(t)
	w.fileBrowserFS = fsys
	nativeOpenDialogImpl(w, NativeOpenDialogCfg{StartDir: "proj"}, false)
	st := w.fileBrowser
	if want := filepath.Join(wd, "proj"); st.dir != want {
		t.Fatalf("dir = %q, want %q", st.dir, want)
	}
	if names := fbEntryNames(w); len(names) == 0 || names[0] != fileBrowserParent {
		t.Fatalf("entries = %q, want %q first", names, fileBrowserParent)
	}
}

func TestFileBrowserFallbackStartDirKeepsError(t *testing.T) {
	w := fileBrowserTestWindow(t)
	fsys, wd := fbWorkingDirFS(t)
	w.fileBrowserFS = fsys
	nativeOpenDialogImpl(w, NativeOpenDialogCfg{StartDir: filepath.Join(fbRoot, "missing")}, false)
	st := w.fileBrowser
	if st.dir != wd || st.errText == "" {
		t.Fatalf("dir = %q, err = %q: want the working folder and the start error", st.dir, st.errText)
	}
}

func TestFileBrowserCtrlClickTwiceUnmarks(t *testing.T) {
	w := fileBrowserTestWindow(t)
	called := false
	nativeOpenDialogImpl(w, NativeOpenDialogCfg{
		StartDir:      fbRoot,
		AllowMultiple: true,
		OnDone:        func(NativeDialogResult, *Window) { called = true },
	}, false)
	w.TestRender(nil)
	// A pinned clock puts every click inside the double-click gap.
	now := goldenInstant()
	w.setVirtualNow(&now)
	ctrl := EventCtx{Window: w, Event: &Event{Modifiers: ModCtrl}}
	// Rows: "..", "sub", "a.txt", "b.md".
	fileBrowserRowClick(ctrl, 2)
	fileBrowserRowClick(ctrl, 3)
	fileBrowserRowClick(ctrl, 3)
	if called || w.fileBrowser == nil {
		t.Fatal("a second Ctrl-click finished the dialog as a double-click")
	}
	if got := w.fileBrowser.marked; !slices.Equal(got, []string{"a.txt"}) {
		t.Fatalf("marked = %q, want [a.txt]", got)
	}
}

// statErrFS fails Stat with err, and ReadDir for any folder named
// badDir. Other calls go to the wrapped fake.
type statErrFS struct {
	mapDirFS
	err    error
	badDir string
}

func (f statErrFS) Stat(p string) (fs.FileInfo, error) {
	if f.err != nil {
		return nil, f.err
	}
	return f.mapDirFS.Stat(p)
}

func (f statErrFS) ReadDir(dir string) ([]fs.DirEntry, error) {
	if f.badDir != "" && filepath.Base(dir) == f.badDir {
		return nil, fs.ErrPermission
	}
	return f.mapDirFS.ReadDir(dir)
}

func TestFileBrowserSaveStatErrorIsShown(t *testing.T) {
	w := fileBrowserTestWindow(t)
	w.fileBrowserFS = statErrFS{mapDirFS: w.fileBrowserFS.(mapDirFS), err: fs.ErrPermission}
	called := false
	nativeSaveDialogImpl(w, NativeSaveDialogCfg{
		StartDir:         fbRoot,
		DefaultName:      "a.txt",
		ConfirmOverwrite: true,
		OnDone:           func(NativeDialogResult, *Window) { called = true },
	}, false)
	fileBrowserAccept(w)
	if called || w.fileBrowser == nil || w.fileBrowser.errText == "" {
		t.Fatalf("a Stat error saved without a check (called %v)", called)
	}
}

func TestFileBrowserSaveUnreadableFolderKeepsName(t *testing.T) {
	w := fileBrowserTestWindow(t)
	w.fileBrowserFS = statErrFS{mapDirFS: w.fileBrowserFS.(mapDirFS), badDir: "sub"}
	nativeSaveDialogImpl(w, NativeSaveDialogCfg{StartDir: fbRoot, DefaultName: "sub"}, false)
	fileBrowserAccept(w)
	st := w.fileBrowser
	if st == nil || st.name != "sub" || st.dir != fbRoot || st.errText == "" {
		t.Fatalf("state %+v: want the name kept and the error shown", st)
	}
}

func TestFileBrowserCancelOpensDialog(t *testing.T) {
	w := fileBrowserTestWindow(t)
	// The browser's OnDone opens its own dialog when it is cancelled.
	nativeOpenDialogImpl(w, NativeOpenDialogCfg{
		StartDir: fbRoot,
		OnDone: func(_ NativeDialogResult, w *Window) {
			w.Dialog(DialogCfg{Title: "from OnDone"})
		},
	}, false)
	w.Dialog(DialogCfg{Title: "app"})
	if got := w.dialogCfg.Title; got != "from OnDone" {
		t.Fatalf("dialog = %q: the dialog opened by OnDone was overwritten", got)
	}
}

func TestFileBrowserUnreadableFolderShowsCause(t *testing.T) {
	w := fileBrowserTestWindow(t)
	w.fileBrowserFS = statErrFS{mapDirFS: w.fileBrowserFS.(mapDirFS), badDir: "sub"}
	nativeOpenDialogImpl(w, NativeOpenDialogCfg{StartDir: fbRoot}, false)
	fileBrowserNavigate(w, filepath.Join(fbRoot, "sub"))
	// A folder the user may not read is a folder: "Not a folder" would
	// send them looking for a typo.
	if got := w.fileBrowser.errText; !strings.Contains(got, fs.ErrPermission.Error()) {
		t.Fatalf("errText = %q, want the permission error", got)
	}
}

func TestFileBrowserCanAcceptDoesNotAllocate(t *testing.T) {
	w := fileBrowserTestWindow(t)
	nativeOpenDialogImpl(w, NativeOpenDialogCfg{StartDir: fbRoot, AllowMultiple: true}, false)
	w.fileBrowser.marked = append(w.fileBrowser.marked, "a.txt")
	// The view asks this every frame.
	if n := testing.AllocsPerRun(100, func() { fileBrowserCanAccept(w) }); n != 0 {
		t.Fatalf("fileBrowserCanAccept allocates %v per call", n)
	}
}

func TestFileBrowserBadSaveName(t *testing.T) {
	cases := []struct {
		name, goos string
		bad        bool
	}{
		{"a.txt", "darwin", false},
		{"a:b.txt", "darwin", false}, // ':' is legal on POSIX
		{"a.txt:x", "windows", true}, // NTFS alternate data stream
		{"C:x", "windows", true},     // drive-relative path
		{"a?.txt", "windows", true},
		{"a.txt", "windows", false},
		{"", "linux", true},
		{"..", "linux", true},
		{"d/a.txt", "linux", true},
		{"a\x00b", "linux", true},
	}
	for _, c := range cases {
		if got := fileBrowserBadSaveName(c.name, c.goos); got != c.bad {
			t.Errorf("fileBrowserBadSaveName(%q, %s) = %v, want %v", c.name, c.goos, got, c.bad)
		}
	}
}

func TestFileBrowserFilterLabel(t *testing.T) {
	cases := []struct {
		f    NativeFileFilter
		want string
	}{
		{NativeFileFilter{Name: "Text", Extensions: []string{"txt", ".MD"}}, "Text (*.txt, *.md)"},
		{NativeFileFilter{Extensions: []string{"go"}}, "*.go"},
		{NativeFileFilter{Name: "All"}, "All"},
	}
	for _, c := range cases {
		if got := fileBrowserFilterLabel(c.f); got != c.want {
			t.Errorf("label(%+v) = %q, want %q", c.f, got, c.want)
		}
	}
}

func TestFileBrowserEnterAcceptsMarkedFiles(t *testing.T) {
	w := fileBrowserTestWindow(t)
	var got NativeDialogResult
	nativeOpenDialogImpl(w, NativeOpenDialogCfg{
		StartDir:      fbRoot,
		AllowMultiple: true,
		OnDone:        func(r NativeDialogResult, _ *Window) { got = r },
	}, false)
	w.TestRender(nil)
	// The cursor is on "..": Enter must take the marks, not open the parent.
	w.fileBrowser.marked = []string{"b.md"}
	if err := w.TestKey(fileBrowserListID, KeyEnter, 0); err != nil {
		t.Fatal(err)
	}
	want := []string{filepath.Join(fbRoot, "b.md")}
	if got.Status != DialogOK || !slices.Equal(got.PathStrings(), want) {
		t.Fatalf("result = %+v, want %q", got, want)
	}
}

func TestFileBrowserSaveFolderNameWithDefaultExt(t *testing.T) {
	w := fileBrowserTestWindow(t)
	called := false
	nativeSaveDialogImpl(w, NativeSaveDialogCfg{
		StartDir:         fbRoot,
		DefaultName:      "sub",
		DefaultExtension: "txt",
		OnDone:           func(NativeDialogResult, *Window) { called = true },
	}, false)
	fileBrowserAccept(w)
	// "sub" is a folder: go into it, do not save "sub.txt".
	if called || w.fileBrowser == nil || w.fileBrowser.dir != filepath.Join(fbRoot, "sub") {
		t.Fatalf("called %v, state %+v: want the browser inside sub", called, w.fileBrowser)
	}
}

func TestFileBrowserReplacedByOnDoneDialogReportsCancel(t *testing.T) {
	w := fileBrowserTestWindow(t)
	// The first browser's OnDone opens an app dialog when it is
	// cancelled, which happens while the second browser opens.
	nativeOpenDialogImpl(w, NativeOpenDialogCfg{
		StartDir: fbRoot,
		OnDone: func(_ NativeDialogResult, w *Window) {
			w.Dialog(DialogCfg{Title: "from OnDone"})
		},
	}, false)
	var second []NativeDialogStatus
	nativeOpenDialogImpl(w, NativeOpenDialogCfg{
		StartDir: fbRoot,
		OnDone:   func(r NativeDialogResult, _ *Window) { second = append(second, r.Status) },
	}, false)
	if got := w.dialogCfg.Title; got != "from OnDone" {
		t.Fatalf("dialog = %q, want the dialog OnDone opened", got)
	}
	// The second browser never got the slot: it reports now, and the
	// app dialog's dismiss must not report for it again.
	if w.fileBrowser != nil || !slices.Equal(second, []NativeDialogStatus{DialogCancel}) {
		t.Fatalf("state %v, second reports %v: want one cancel and no state", w.fileBrowser != nil, second)
	}
	w.DialogDismiss()
	if len(second) != 1 {
		t.Fatalf("second reports %v after dismiss", second)
	}
}

func TestFileBrowserEmptyPathBarStays(t *testing.T) {
	w := fileBrowserTestWindow(t)
	// The working folder must be readable, or the jump fails and hides
	// the bug.
	fsys, wd := fbWorkingDirFS(t)
	w.fileBrowserFS = fsys
	start := filepath.Join(wd, "proj")
	nativeOpenDialogImpl(w, NativeOpenDialogCfg{StartDir: start}, false)
	w.TestRender(nil)
	w.fileBrowser.pathText = ""
	if err := w.TestKey(fileBrowserPathID, KeyEnter, 0); err != nil {
		t.Fatal(err)
	}
	if got := w.fileBrowser.dir; got != start {
		t.Fatalf("dir = %q: an empty path bar moved the browser", got)
	}
}

func TestFileBrowserNoParentRowWithoutFolder(t *testing.T) {
	st := &fileBrowserState{filterExts: [][]string{nil}}
	fileBrowserRefilter(st)
	if len(st.entries) != 0 {
		t.Fatalf("entries = %+v: no folder, so no %q row", st.entries, fileBrowserParent)
	}
}
