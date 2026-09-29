package gui

import (
	"testing"

	"github.com/go-gui-org/go-gui/gui/appinfo"
)

var testAppInfo = appinfo.Info{
	ID:      "org.go-gui.test",
	Name:    "Test App",
	Version: "1.2.3",
}

func TestNewWindowAppInfoFillsEmptyFields(t *testing.T) {
	w := NewWindow(WindowCfg{AppInfo: testAppInfo})
	if got := w.AppInfo(); got != testAppInfo {
		t.Fatalf("AppInfo() = %+v, want %+v", got, testAppInfo)
	}
	if w.Config.Title != "Test App" {
		t.Errorf("Title = %q, want the manifest name", w.Config.Title)
	}
	if w.Config.WMClass != "org.go-gui.test" {
		t.Errorf("WMClass = %q, want the manifest ID", w.Config.WMClass)
	}
	if got := fileAccessAppID(w); got != "org.go-gui.test" {
		t.Errorf("file access app ID = %q, want the manifest ID", got)
	}
}

func TestNewWindowAppInfoExplicitFieldsWin(t *testing.T) {
	w := NewWindow(WindowCfg{
		AppInfo: testAppInfo,
		Title:   "Document 1 - Test App",
		WMClass: "custom-class",
	})
	if w.Config.Title != "Document 1 - Test App" {
		t.Errorf("Title = %q, explicit title must win", w.Config.Title)
	}
	if w.Config.WMClass != "custom-class" {
		t.Errorf("WMClass = %q, explicit WMClass must win", w.Config.WMClass)
	}
	// SetFileAccessAppID after creation still replaces the manifest ID.
	w.SetFileAccessAppID("com.example.other")
	if got := fileAccessAppID(w); got != "com.example.other" {
		t.Errorf("file access app ID = %q, SetFileAccessAppID must win", got)
	}
}

func TestNewWindowNoAppInfoKeepsOldBehavior(t *testing.T) {
	w := NewWindow(WindowCfg{})
	if w.Config.Title != "" || w.Config.WMClass != "" || fileAccessAppID(w) != "" {
		t.Fatalf("zero AppInfo must set nothing: title %q, class %q, id %q",
			w.Config.Title, w.Config.WMClass, fileAccessAppID(w))
	}
}

func TestAppSetNativeMenubarAppNameFromManifest(t *testing.T) {
	app := NewApp()
	mp := &mockAppPlatform{}
	w := NewWindow(WindowCfg{AppInfo: testAppInfo})
	w.SetNativePlatform(mp)
	app.Register(1, w)

	app.SetNativeMenubar(NativeMenubarCfg{})
	if mp.menubarCfg.AppName != "Test App" {
		t.Errorf("AppName = %q, want the manifest name", mp.menubarCfg.AppName)
	}

	app.SetNativeMenubar(NativeMenubarCfg{AppName: "Explicit"})
	if mp.menubarCfg.AppName != "Explicit" {
		t.Errorf("AppName = %q, explicit AppName must win", mp.menubarCfg.AppName)
	}
}

func TestAppOpenWindowInheritsAppInfo(t *testing.T) {
	app := NewApp()
	app.Register(1, NewWindow(WindowCfg{AppInfo: testAppInfo}))

	app.OpenWindow(WindowCfg{})
	if got := (<-app.PendingOpen()).AppInfo; got != testAppInfo {
		t.Errorf("second window AppInfo = %+v, want the main window's", got)
	}

	other := appinfo.Info{ID: "org.go-gui.other"}
	app.OpenWindow(WindowCfg{AppInfo: other})
	if got := (<-app.PendingOpen()).AppInfo; got != other {
		t.Errorf("second window AppInfo = %+v, explicit AppInfo must win", got)
	}
}

// With no main window there is nothing to inherit: the request queues
// with a zero AppInfo instead of dereferencing a nil window.
func TestAppOpenWindowNoMainWindowKeepsZeroAppInfo(t *testing.T) {
	app := NewApp()
	app.OpenWindow(WindowCfg{})
	if got := (<-app.PendingOpen()).AppInfo; got != (appinfo.Info{}) {
		t.Errorf("AppInfo = %+v, want zero", got)
	}
}

func fileAccessAppID(w *Window) string {
	w.fileAccess.mu.Lock()
	defer w.fileAccess.mu.Unlock()
	return w.fileAccess.appID
}
