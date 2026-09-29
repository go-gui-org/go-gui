package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/go-gui-org/go-gui/gui/appinfo"
)

const testManifest = `id      = "org.go-gui.falcon"
name    = "Falcon"
version = "1.4.0"
build   = "42"
icon    = "assets/icon.png"

[darwin]
category = "public.app-category.developer-tools"

[linux]
categories = "Development;"
`

func writeManifest(t *testing.T, dir, body string) string {
	t.Helper()
	p := filepath.Join(dir, defaultManifest)
	if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	return p
}

func TestLoadManifestExplicitPath(t *testing.T) {
	dir := t.TempDir()
	p := writeManifest(t, dir, testManifest)
	info, gotDir, err := loadManifest(p)
	if err != nil {
		t.Fatal(err)
	}
	if info.ID != "org.go-gui.falcon" || gotDir != dir {
		t.Fatalf("got id %q dir %q", info.ID, gotDir)
	}
}

func TestLoadManifestExplicitMissingErrors(t *testing.T) {
	_, _, err := loadManifest(filepath.Join(t.TempDir(), "nope.toml"))
	if err == nil {
		t.Fatal("an explicit -manifest that does not exist must be an error")
	}
}

func TestLoadManifestDefaultFromWorkingDir(t *testing.T) {
	dir := t.TempDir()
	writeManifest(t, dir, testManifest)
	t.Chdir(dir)
	info, gotDir, err := loadManifest("")
	if err != nil {
		t.Fatal(err)
	}
	if info.Name != "Falcon" || gotDir != "." {
		t.Fatalf("got name %q dir %q", info.Name, gotDir)
	}
}

// No -manifest and no appinfo.toml in the working directory is today's
// behaviour: no manifest, no error.
func TestLoadManifestDefaultAbsent(t *testing.T) {
	t.Chdir(t.TempDir())
	info, _, err := loadManifest("")
	if err != nil {
		t.Fatal(err)
	}
	if info != (appinfo.Info{}) {
		t.Fatalf("got %+v, want zero Info", info)
	}
}

func TestLoadManifestParseErrorNamesFile(t *testing.T) {
	dir := t.TempDir()
	p := writeManifest(t, dir, "name = Falcon\n")
	_, _, err := loadManifest(p)
	if err == nil || !strings.Contains(err.Error(), p) ||
		!strings.Contains(err.Error(), "line 1") {
		t.Fatalf("got %v, want an error naming %s and line 1", err, p)
	}
}

func TestApplyManifestFillsUnsetFlags(t *testing.T) {
	o := bundleOpts{Version: "1.0"} // "1.0" is the -version flag default
	applyManifest(&o, appinfo.MustParse([]byte(testManifest)), "/src/app", nil)
	want := bundleOpts{
		Name: "Falcon", ID: "org.go-gui.falcon", Version: "1.4.0", Build: "42",
		Icon:       filepath.Join("/src/app", "assets/icon.png"),
		Category:   "public.app-category.developer-tools",
		Categories: "Development;",
	}
	if o != want {
		t.Fatalf("got  %+v\nwant %+v", o, want)
	}
}

func TestApplyManifestSetFlagsWin(t *testing.T) {
	o := bundleOpts{
		Name: "Flag", ID: "flag.id", Version: "9.9.9", Build: "7", Icon: "flag.png",
	}
	set := map[string]bool{"name": true, "id": true, "version": true, "build": true, "icon": true}
	applyManifest(&o, appinfo.MustParse([]byte(testManifest)), "/src/app", set)
	if o.Name != "Flag" || o.ID != "flag.id" || o.Version != "9.9.9" ||
		o.Build != "7" || o.Icon != "flag.png" {
		t.Fatalf("a set flag must beat the manifest: %+v", o)
	}
	// Keys with no flag still come from the file.
	if o.Category == "" || o.Categories == "" {
		t.Fatalf("platform keys must still apply: %+v", o)
	}
}

// A key the file leaves out keeps the flag value, including a default.
func TestApplyManifestEmptyKeysKeepFlags(t *testing.T) {
	o := bundleOpts{Version: "1.0"}
	applyManifest(&o, appinfo.Info{Name: "Only"}, ".", nil)
	if o.Name != "Only" || o.Version != "1.0" || o.Icon != "" {
		t.Fatalf("got %+v", o)
	}
}

func TestApplyManifestAbsoluteIconKept(t *testing.T) {
	abs := filepath.Join(t.TempDir(), "icon.png")
	var o bundleOpts
	applyManifest(&o, appinfo.Info{Icon: abs}, "/src/app", nil)
	if o.Icon != abs {
		t.Fatalf("got %q, want %q", o.Icon, abs)
	}
}
