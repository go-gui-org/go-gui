package backend

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// TestDarwinNoCgoLinkFails pins #1159. The macOS backend needs cgo. A
// darwin app built with CGO_ENABLED=0 must fail to link and name the
// cause. Before the fix it linked and then panicked at launch.
//
// The test cross-compiles, so it runs on every host. No C toolchain is
// needed, because cgo is off.
func TestDarwinNoCgoLinkFails(t *testing.T) {
	if testing.Short() {
		t.Skip("cross-compiles an app; skipped with -short")
	}
	out := filepath.Join(t.TempDir(), "app")
	cmd := exec.Command("go", "build", "-o", out, "./testdata/nocgodarwin")
	cmd.Env = append(os.Environ(), "CGO_ENABLED=0", "GOOS=darwin", "GOARCH=arm64")
	b, err := cmd.CombinedOutput()
	if err == nil {
		t.Fatal("darwin build with CGO_ENABLED=0 linked; want a link error")
	}
	if !strings.Contains(string(b), "macOS_requires_cgo__set_CGO_ENABLED_1") {
		t.Fatalf("link error does not name the cause:\n%s", b)
	}
}
