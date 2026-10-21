//go:build unix

package tempdir

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
)

// filesDir is a fresh files directory with no TMPDIR set. It is made
// before TMPDIR is cleared: t.TempDir itself reads TMPDIR.
func filesDir(t *testing.T) string {
	t.Helper()
	files := t.TempDir()
	t.Setenv("TMPDIR", "")
	return files
}

// write makes path with content, creating its directory.
func write(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
}

// exists reports whether path exists, without following a symlink.
func exists(t *testing.T, path string) bool {
	t.Helper()
	_, err := os.Lstat(path)
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		t.Fatal(err)
	}
	return err == nil
}

// tryLock reports whether a lock on path can be taken now, from a new
// open file, which a lock held through another open file blocks.
func tryLock(t *testing.T, path string) bool {
	t.Helper()
	f, err := os.OpenFile(path, os.O_RDWR, 0)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = f.Close() }()
	err = syscall.Flock(int(f.Fd()), syscall.LOCK_EX|syscall.LOCK_NB)
	if errors.Is(err, syscall.EWOULDBLOCK) {
		return false
	}
	if err != nil {
		t.Fatal(err)
	}
	return true
}

// The process gets a proc- directory of its own under go-gui-tmp, holds
// its live.lock, and temp files land in it.
func TestUnderFilesDirUsesOwnProcessDir(t *testing.T) {
	files := filesDir(t)
	if err := UnderFilesDir(files, t.Logf); err != nil {
		t.Fatal(err)
	}
	dir := os.Getenv("TMPDIR")
	if filepath.Dir(dir) != filepath.Join(files, rootName) ||
		!strings.HasPrefix(filepath.Base(dir), procPrefix) {
		t.Fatalf("TMPDIR = %q, want %s/%s*", dir, filepath.Join(files, rootName), procPrefix)
	}
	if tryLock(t, filepath.Join(dir, liveLockName)) {
		t.Fatal("this process's live.lock is not held")
	}
	f, err := os.CreateTemp("", "probe-*")
	if err != nil {
		t.Fatal(err)
	}
	_ = f.Close()
	if filepath.Dir(f.Name()) != dir {
		t.Fatalf("CreateTemp wrote to %q, want %q", filepath.Dir(f.Name()), dir)
	}
}

// Only go-gui's own proc- directories are touched: the host's files,
// a host tmp directory, unknown entries under go-gui-tmp and a symlink
// named like a process directory all stay, and the symlink's target is
// not followed.
func TestUnderFilesDirLeavesWhatItDoesNotOwn(t *testing.T) {
	files := filesDir(t)
	root := filepath.Join(files, rootName)
	outside := t.TempDir()
	keep := []string{
		filepath.Join(files, "tmp", "host-data.txt"),
		filepath.Join(files, "settings.json"),
		filepath.Join(root, "notes.txt"),
		filepath.Join(root, "other", "data.bin"),
		filepath.Join(outside, "target.txt"),
	}
	for _, p := range keep {
		write(t, p, "keep")
	}
	link := filepath.Join(root, procPrefix+"link")
	if err := os.Symlink(outside, link); err != nil {
		t.Fatal(err)
	}
	if err := UnderFilesDir(files, t.Logf); err != nil {
		t.Fatal(err)
	}
	for _, p := range append(keep, link) {
		if !exists(t, p) {
			t.Errorf("%s was removed", p)
		}
	}
}

// The directory of a process that died is removed, whether it died with
// its lock released or before it took one.
func TestUnderFilesDirRemovesDeadProcessDirs(t *testing.T) {
	files := filesDir(t)
	root := filepath.Join(files, rootName)
	dead := filepath.Join(root, procPrefix+"dead")
	write(t, filepath.Join(dead, liveLockName), "")
	write(t, filepath.Join(dead, "showcase-1.png"), "old")
	incomplete := filepath.Join(root, procPrefix+"incomplete")
	write(t, filepath.Join(incomplete, "go_gui_feathericon-1.ttf"), "old")
	if err := UnderFilesDir(files, t.Logf); err != nil {
		t.Fatal(err)
	}
	for _, d := range []string{dead, incomplete} {
		if exists(t, d) {
			t.Errorf("%s survived", d)
		}
	}
}

// The directory of a live process, one that holds its live.lock, keeps
// its files.
func TestUnderFilesDirKeepsLiveProcessDir(t *testing.T) {
	files := filesDir(t)
	live := filepath.Join(files, rootName, procPrefix+"live")
	write(t, filepath.Join(live, "in-use.pdf"), "busy")
	lockPath := filepath.Join(live, liveLockName)
	write(t, lockPath, "")
	held, err := os.OpenFile(lockPath, os.O_RDWR, 0)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = held.Close() }()
	if err := syscall.Flock(int(held.Fd()), syscall.LOCK_EX); err != nil {
		t.Fatal(err)
	}
	if err := UnderFilesDir(files, t.Logf); err != nil {
		t.Fatal(err)
	}
	if !exists(t, filepath.Join(live, "in-use.pdf")) {
		t.Fatal("a live process's file was removed")
	}
}

// Where TMPDIR is already set, nothing is made under the files
// directory.
func TestUnderFilesDirKeepsExistingTMPDIR(t *testing.T) {
	files := t.TempDir()
	t.Setenv("TMPDIR", "/data/user/0/app/cache")
	if err := UnderFilesDir(files, t.Logf); err != nil {
		t.Fatal(err)
	}
	if got := os.Getenv("TMPDIR"); got != "/data/user/0/app/cache" {
		t.Fatalf("TMPDIR = %q", got)
	}
	if exists(t, filepath.Join(files, rootName)) {
		t.Fatal("UnderFilesDir made go-gui-tmp although TMPDIR was set")
	}
}
