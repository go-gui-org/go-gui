//go:build !android

// Package glescheck holds a test-only build of gles_android.c: package android
// next door is entirely android-tagged, so it never compiles for the host and
// go list ./gui/... does not see it, keeping its gomobile-bind entry points out
// of the export-audit scan. This package has no non-test file, for the same
// reason: any host-buildable export here would put android's own package back in
// that scan and flag Backend/TouchBegan/TouchEnded/TouchMoved (real gomobile
// bind consumers export-audit cannot see) as internal-only.
package glescheck

import (
	"os/exec"
	"path/filepath"
	"runtime"
	"testing"
)

// TestGLESHarness compiles ../gles_android.c on the host against the stub GL
// header in testdata/glesharness and runs the C checks in harness.c. The Android
// backend otherwise builds only for Android, so this is the one place its C
// state logic (which program is bound, which uniform a setter writes) runs in
// CI.
//
// The test skips when the host has no C compiler.
func TestGLESHarness(t *testing.T) {
	cc, err := exec.LookPath("cc")
	if err != nil {
		t.Skip("no C compiler (cc) on PATH")
	}
	stubDir := filepath.Join("testdata", "glesharness")
	bin := filepath.Join(t.TempDir(), "glesharness")
	if runtime.GOOS == "windows" {
		bin += ".exe"
	}
	// -I stubDir must come first, so <GLES3/gl3.h> resolves to the stub, not a
	// real Android GLES header.
	build := exec.Command(cc, "-I", stubDir, "-I", "..",
		filepath.Join(stubDir, "harness.c"),
		filepath.Join("..", "gles_android.c"), "-o", bin, "-lm")
	if out, buildErr := build.CombinedOutput(); buildErr != nil {
		t.Fatalf("build harness: %v\n%s", buildErr, out)
	}
	if out, runErr := exec.Command(bin).CombinedOutput(); runErr != nil {
		t.Fatalf("harness checks failed: %v\n%s", runErr, out)
	}
}
