//go:build darwin

package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func buildStubBinary(t *testing.T) string {
	t.Helper()
	if runtime.GOOS != "darwin" {
		t.Skip("buildapp targets darwin")
	}
	dir := t.TempDir()
	src := filepath.Join(dir, "stub.go")
	if err := os.WriteFile(src, []byte("package main\nfunc main(){}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	out := filepath.Join(dir, "stub")
	cmd := exec.Command("go", "build", "-o", out, src)
	if b, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("go build: %v: %s", err, b)
	}
	return out
}

func TestBuildBundleLayout(t *testing.T) {
	bin := buildStubBinary(t)
	outDir := t.TempDir()
	err := build(bundleOpts{Binary: bin, OutDir: outDir, Version: "1.0"})
	if err != nil {
		t.Fatalf("build: %v", err)
	}
	app := filepath.Join(outDir, "Stub.app")
	for _, p := range []string{
		"Contents/Info.plist",
		"Contents/MacOS/stub",
	} {
		if _, err = os.Stat(filepath.Join(app, p)); err != nil {
			t.Errorf("missing %s: %v", p, err)
		}
	}
	plist, err := os.ReadFile(filepath.Join(app, "Contents/Info.plist"))
	if err != nil {
		t.Fatal(err)
	}
	s := string(plist)
	for _, want := range []string{
		"<string>stub</string>",
		"<string>Stub</string>",
		"local.gogui.stub",
		"NSHighResolutionCapable",
	} {
		if !strings.Contains(s, want) {
			t.Errorf("plist missing %q", want)
		}
	}
	if strings.Contains(s, "CFBundleIconFile") {
		t.Errorf("plist should not reference icon when none supplied")
	}
}

func TestBuildOverwritesExisting(t *testing.T) {
	bin := buildStubBinary(t)
	outDir := t.TempDir()
	opts := bundleOpts{Binary: bin, OutDir: outDir, Version: "1.0"}
	if err := build(opts); err != nil {
		t.Fatal(err)
	}
	if err := build(opts); err != nil {
		t.Fatalf("second build: %v", err)
	}
}

func TestValidateMachORejectsText(t *testing.T) {
	dir := t.TempDir()
	junk := filepath.Join(dir, "junk")
	if err := os.WriteFile(junk, []byte("not a binary"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := validateMachO(junk); err == nil {
		t.Error("expected error for non Mach-O file")
	}
}

func TestValidateMachORejectsDirectory(t *testing.T) {
	if err := validateMachO(t.TempDir()); err == nil {
		t.Error("expected error for directory")
	}
}

func TestValidateMachORejectsMissing(t *testing.T) {
	if err := validateMachO(filepath.Join(t.TempDir(), "nope")); err == nil {
		t.Error("expected error for missing file")
	}
}

func TestBuildCustomNameAndID(t *testing.T) {
	bin := buildStubBinary(t)
	outDir := t.TempDir()
	err := build(bundleOpts{
		Binary: bin, OutDir: outDir, Name: "MyApp",
		ID: "com.example.myapp", Version: "2.3",
	})
	if err != nil {
		t.Fatal(err)
	}
	plist, err := os.ReadFile(filepath.Join(outDir, "MyApp.app/Contents/Info.plist"))
	if err != nil {
		t.Fatal(err)
	}
	s := string(plist)
	for _, want := range []string{"MyApp", "com.example.myapp", "<string>2.3</string>"} {
		if !strings.Contains(s, want) {
			t.Errorf("plist missing %q", want)
		}
	}
	if _, err = os.Stat(filepath.Join(outDir, "MyApp.app/Contents/MacOS/stub")); err != nil {
		t.Error(err)
	}
}

func TestBuildMissingBinary(t *testing.T) {
	if runtime.GOOS != "darwin" {
		t.Skip("buildapp targets darwin")
	}
	err := build(bundleOpts{
		Binary: filepath.Join(t.TempDir(), "nope"),
		OutDir: t.TempDir(), Version: "1.0",
	})
	if err == nil {
		t.Error("expected error")
	}
}

func TestInstallIconIcnsPassthrough(t *testing.T) {
	dir := t.TempDir()
	src := filepath.Join(dir, "in.icns")
	payload := []byte("fake-icns-bytes")
	if err := os.WriteFile(src, payload, 0o644); err != nil {
		t.Fatal(err)
	}
	resDir := t.TempDir()
	name, err := installIcon(src, resDir, "stub")
	if err != nil {
		t.Fatal(err)
	}
	if name != "stub.icns" {
		t.Errorf("name = %q, want stub.icns", name)
	}
	got, err := os.ReadFile(filepath.Join(resDir, name))
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != string(payload) {
		t.Error("icns content mismatch")
	}
}

func TestInstallIconRejectsUnsupported(t *testing.T) {
	dir := t.TempDir()
	src := filepath.Join(dir, "in.jpg")
	if err := os.WriteFile(src, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := installIcon(src, t.TempDir(), "stub"); err == nil {
		t.Error("expected error for .jpg")
	}
}

func TestInstallIconPngConversion(t *testing.T) {
	if runtime.GOOS != "darwin" {
		t.Skip("requires darwin tools")
	}
	if _, err := exec.LookPath("sips"); err != nil {
		t.Skip("sips not available")
	}
	if _, err := exec.LookPath("iconutil"); err != nil {
		t.Skip("iconutil not available")
	}
	dir := t.TempDir()
	src := filepath.Join(dir, "in.png")
	// 1x1 PNG
	png := []byte{
		0x89, 0x50, 0x4e, 0x47, 0x0d, 0x0a, 0x1a, 0x0a,
		0x00, 0x00, 0x00, 0x0d, 0x49, 0x48, 0x44, 0x52,
		0x00, 0x00, 0x00, 0x01, 0x00, 0x00, 0x00, 0x01,
		0x08, 0x06, 0x00, 0x00, 0x00, 0x1f, 0x15, 0xc4,
		0x89, 0x00, 0x00, 0x00, 0x0d, 0x49, 0x44, 0x41,
		0x54, 0x78, 0x9c, 0x63, 0x00, 0x01, 0x00, 0x00,
		0x05, 0x00, 0x01, 0x0d, 0x0a, 0x2d, 0xb4, 0x00,
		0x00, 0x00, 0x00, 0x49, 0x45, 0x4e, 0x44, 0xae,
		0x42, 0x60, 0x82,
	}
	if err := os.WriteFile(src, png, 0o644); err != nil {
		t.Fatal(err)
	}
	resDir := t.TempDir()
	name, err := installIcon(src, resDir, "stub")
	if err != nil {
		t.Fatalf("installIcon: %v", err)
	}
	fi, err := os.Stat(filepath.Join(resDir, name))
	if err != nil || fi.Size() == 0 {
		t.Errorf("icns missing or empty: %v", err)
	}
}

func TestBuildWithIconPlistKey(t *testing.T) {
	bin := buildStubBinary(t)
	dir := t.TempDir()
	icon := filepath.Join(dir, "icon.icns")
	if err := os.WriteFile(icon, []byte("fake"), 0o644); err != nil {
		t.Fatal(err)
	}
	outDir := t.TempDir()
	err := build(bundleOpts{
		Binary: bin, OutDir: outDir, Icon: icon, Version: "1.0",
	})
	if err != nil {
		t.Fatal(err)
	}
	plist, err := os.ReadFile(filepath.Join(outDir, "Stub.app/Contents/Info.plist"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(plist), "CFBundleIconFile") {
		t.Error("plist missing CFBundleIconFile")
	}
	if _, err = os.Stat(filepath.Join(outDir, "Stub.app/Contents/Resources/stub.icns")); err != nil {
		t.Error(err)
	}
}

func TestCopyTreeFallback(t *testing.T) {
	src := t.TempDir()
	if err := os.MkdirAll(filepath.Join(src, "a/b"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(src, "a/b/f.txt"), []byte("hi"), 0o644); err != nil {
		t.Fatal(err)
	}
	dst := filepath.Join(t.TempDir(), "out")
	if err := copyTree(src, dst); err != nil {
		t.Fatal(err)
	}
	got, err := os.ReadFile(filepath.Join(dst, "a/b/f.txt"))
	if err != nil || string(got) != "hi" {
		t.Errorf("copy failed: %v %q", err, got)
	}
}

func TestIsSystemLib(t *testing.T) {
	cases := map[string]bool{
		"/usr/lib/libSystem.B.dylib":                         true,
		"/System/Library/Frameworks/Cocoa.framework/A/Cocoa": true,
		"/Library/Apple/usr/lib/libfoo.dylib":                true,
		"/opt/homebrew/opt/somepkg/lib/libFoo-1.0.0.dylib":   false,
		"/usr/local/lib/libpng.dylib":                        false,
		"@rpath/libfoo.dylib":                                false,
	}
	for p, want := range cases {
		if got := isSystemLib(p); got != want {
			t.Errorf("isSystemLib(%q) = %v, want %v", p, got, want)
		}
	}
}

func TestBundleDepsOnStub(t *testing.T) {
	if runtime.GOOS != "darwin" {
		t.Skip("darwin only")
	}
	for _, tool := range []string{"otool", "install_name_tool", "codesign"} {
		if _, err := exec.LookPath(tool); err != nil {
			t.Skipf("%s not available", tool)
		}
	}
	bin := buildStubBinary(t)
	outDir := t.TempDir()
	err := build(bundleOpts{
		Binary: bin, OutDir: outDir, Version: "1.0", BundleDeps: true,
	})
	if err != nil {
		t.Fatalf("build: %v", err)
	}
	// stub only links libSystem (system lib), so Frameworks should be empty
	// but should exist; the rpath should still be added.
	app := filepath.Join(outDir, "Stub.app")
	if _, err = os.Stat(filepath.Join(app, "Contents/Frameworks")); err != nil {
		t.Errorf("Frameworks dir missing: %v", err)
	}
	out, err := exec.Command("otool", "-l", filepath.Join(app, "Contents/MacOS/stub")).Output()
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(out), "@executable_path/../Frameworks") {
		t.Error("rpath not added to executable")
	}
}

func TestWritePlistOmitsEmptyIcon(t *testing.T) {
	p := filepath.Join(t.TempDir(), "Info.plist")
	if err := writePlist(p, "stub", "id", "Stub", "1.0", ""); err != nil {
		t.Fatal(err)
	}
	b, err := os.ReadFile(p)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(b), "CFBundleIconFile") {
		t.Error("should omit CFBundleIconFile when empty")
	}
}
