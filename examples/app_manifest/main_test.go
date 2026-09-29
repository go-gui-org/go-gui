package main

import "testing"

// The embedded appinfo.toml parses, and the window takes its identity
// from it rather than from strings spelled in Go.
func TestWindowIdentityFromManifest(t *testing.T) {
	if info.ID != "org.go-gui.app-manifest" || info.Name != "App Manifest Demo" {
		t.Fatalf("embedded manifest: %+v", info)
	}
	w := newWindow()
	if w.Config.Title != info.Name {
		t.Errorf("Title = %q, want %q", w.Config.Title, info.Name)
	}
	if w.AppInfo() != info {
		t.Errorf("AppInfo() = %+v, want %+v", w.AppInfo(), info)
	}
}

func TestMainViewNoPanic(t *testing.T) {
	t.Parallel()
	w := newWindow()
	_ = mainView(w).GenerateLayout(w)
}
