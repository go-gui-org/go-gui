//go:build unix && !ios && !android

package instance

import (
	"os"
	"path/filepath"
	"strconv"
	"testing"
)

// TestLockDirRejectsSharedDir: without $XDG_RUNTIME_DIR the lock lives
// in os.TempDir, which other users can write. A gogui-<uid> directory
// another user made first (here: a too-open mode) must be refused, not
// used.
func TestLockDirRejectsSharedDir(t *testing.T) {
	tmp, err := os.MkdirTemp("", "gi")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.RemoveAll(tmp) })
	t.Setenv("XDG_RUNTIME_DIR", "")
	t.Setenv("TMPDIR", tmp)
	dir := filepath.Join(tmp, "gogui-"+strconv.Itoa(os.Getuid()))

	got, err := lockDir()
	if err != nil || got != dir {
		t.Fatalf("lockDir = (%q, %v), want (%q, nil)", got, err, dir)
	}
	if err := os.Chmod(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	if _, err := lockDir(); err == nil {
		t.Fatal("lockDir accepted a 0755 directory")
	}
	// A symlink in its place is refused too.
	if err := os.Remove(dir); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(tmp, dir); err != nil {
		t.Fatal(err)
	}
	if _, err := lockDir(); err == nil {
		t.Fatal("lockDir accepted a symlink")
	}
	// An election that cannot get its directory still runs the app.
	if _, primary, err := acquire("org.go-gui.test.dir", nil); err == nil || !primary {
		t.Fatalf("acquire = (%v, %v), want (true, err)", primary, err)
	}
}
