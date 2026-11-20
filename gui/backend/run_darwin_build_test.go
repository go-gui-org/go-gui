package backend

import (
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// buildDarwinApp cross-compiles testdata/darwinapp for darwin/arch with
// cgo off and returns the go build output and error. The build needs no
// C toolchain, so it runs on every host that can run go.
func buildDarwinApp(t *testing.T, arch string) ([]byte, error) {
	t.Helper()
	if testing.Short() {
		t.Skip("cross-compiles an app; skipped with -short")
	}
	// A js/wasm test binary cannot start a process, so it cannot run go.
	// The same holds for the mobile test runners. The native hosts cover
	// the check.
	switch runtime.GOOS {
	case "js", "wasip1", "ios", "android":
		t.Skipf("cannot run go on %s", runtime.GOOS)
	}
	if _, err := exec.LookPath("go"); err != nil {
		t.Skip("go tool not on PATH")
	}
	out := filepath.Join(t.TempDir(), "app")
	cmd := exec.Command("go", "build", "-o", out, "./testdata/darwinapp")
	cmd.Env = append(os.Environ(), "CGO_ENABLED=0", "GOOS=darwin", "GOARCH="+arch)
	return cmd.CombinedOutput()
}

// TestDarwinArm64BuildsWithoutCgo: the macOS backend has no cgo
// (#1323), so a darwin/arm64 app links with CGO_ENABLED=0.
func TestDarwinArm64BuildsWithoutCgo(t *testing.T) {
	if b, err := buildDarwinApp(t, "arm64"); err != nil {
		t.Fatalf("darwin/arm64 build with CGO_ENABLED=0 failed: %v\n%s", err, b)
	}
}

// TestDarwinAmd64LinkFails: darwin/amd64 has no backend (#1323). An
// app must fail to link and name the cause, not link and panic at
// launch as a cgo-off build did before #1159.
func TestDarwinAmd64LinkFails(t *testing.T) {
	b, err := buildDarwinApp(t, "amd64")
	if err == nil {
		t.Fatal("darwin/amd64 build linked; want a link error")
	}
	if !strings.Contains(string(b), "macOS_requires_arm64__Intel_Macs_are_not_supported") {
		t.Fatalf("link error does not name the cause:\n%s", b)
	}
}
