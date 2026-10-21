//go:build unix

package tempdir

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"syscall"
)

const (
	// rootName is the directory under filesDir that go-gui owns. Nothing
	// outside it is ever created or removed.
	rootName = "go-gui-tmp"
	// procPrefix names a process directory inside the root. Only entries
	// with exactly this prefix are inspected.
	procPrefix = "proc-"
	// liveLockName is held, for the life of its process, in each process
	// directory.
	liveLockName = "live.lock"
	// gcLockName serializes creating a process directory and sweeping the
	// others, so no process can sweep a directory before its owner has
	// locked it.
	gcLockName = ".gc.lock"
)

// liveLock keeps this process's live.lock open, and so locked, for the
// life of the process. It is never closed: Android can tear the backend
// down and build it again while the process, and the TMPDIR it set,
// live on. It is never read either; without the reference the file's
// finalizer would close it and release the lock.
var liveLock *os.File //nolint:unused // held, never read: keeps the lock's file open

// UnderFilesDir makes a directory of this process's own, under
// filesDir/go-gui-tmp, the TMPDIR when Go sees none: the fallback where
// the platform set no TMPDIR. filesDir is the app's private files
// directory, writable on every Android version.
//
// Nothing clears it the way Android clears the cache directory, so each
// process gets its own proc-* directory, holds a lock in it while it
// lives, and removes the directories of processes that have died. Other
// processes of the same app share filesDir; the locks keep one from
// removing a live one's files. A failure to sweep is reported through
// logf and does not stop the new directory being used. A failure to make
// or lock it is returned: TMPDIR would be unusable.
func UnderFilesDir(filesDir string, logf func(format string, args ...any)) error {
	if filesDir == "" || os.Getenv("TMPDIR") != "" {
		return nil
	}
	if logf == nil {
		logf = func(string, ...any) {}
	}
	root := filepath.Join(filesDir, rootName)
	if err := os.MkdirAll(root, 0o700); err != nil {
		return err
	}
	gc, err := openLock(filepath.Join(root, gcLockName))
	if err != nil {
		return err
	}
	// Closing the file releases the lock.
	defer func() { _ = gc.Close() }()
	if err := syscall.Flock(int(gc.Fd()), syscall.LOCK_EX); err != nil {
		return fmt.Errorf("lock %s: %w", gc.Name(), err)
	}

	dir, err := os.MkdirTemp(root, procPrefix)
	if err != nil {
		return err
	}
	live, err := openLock(filepath.Join(dir, liveLockName))
	if err != nil {
		_ = os.RemoveAll(dir)
		return err
	}
	if err := syscall.Flock(int(live.Fd()), syscall.LOCK_EX|syscall.LOCK_NB); err != nil {
		_ = live.Close()
		_ = os.RemoveAll(dir)
		return fmt.Errorf("lock %s: %w", live.Name(), err)
	}
	if err := os.Setenv("TMPDIR", dir); err != nil {
		_ = live.Close()
		_ = os.RemoveAll(dir)
		return err
	}
	liveLock = live

	sweep(root, dir, logf)
	return nil
}

// openLock opens a lock file, creating it if needed. os.OpenFile adds
// O_CLOEXEC on Unix; O_NOFOLLOW refuses a symlink planted at the path.
func openLock(path string) (*os.File, error) {
	// #nosec G304 — a fixed lock name under go-gui-tmp in the host's files dir; O_NOFOLLOW
	return os.OpenFile(path,
		os.O_RDWR|os.O_CREATE|syscall.O_NOFOLLOW, 0o600)
}

// sweep removes the process directories under root whose process is
// gone, all but own. It runs under the GC lock. Entries without the
// proc- prefix, and anything that is not a plain directory (a symlink
// with the prefix included), are left alone.
func sweep(root, own string, logf func(format string, args ...any)) {
	entries, err := os.ReadDir(root)
	if err != nil {
		logf("tempdir: read %s: %v", root, err)
		return
	}
	for _, e := range entries {
		if !strings.HasPrefix(e.Name(), procPrefix) || !e.IsDir() {
			continue
		}
		dir := filepath.Join(root, e.Name())
		if dir == own {
			continue
		}
		if err := reap(dir); err != nil {
			logf("tempdir: sweep %s: %v", dir, err)
		}
	}
}

// reap removes dir when the process that made it is gone. A missing
// live.lock means its process died between making the directory and
// locking it, which both happen under the GC lock. A lock it cannot
// take belongs to a live process. A lock it can take is held while the
// directory is removed.
func reap(dir string) error {
	// #nosec G304 — live.lock in a proc- directory under go-gui-tmp; O_NOFOLLOW
	f, err := os.OpenFile(filepath.Join(dir, liveLockName),
		os.O_RDWR|syscall.O_NOFOLLOW, 0)
	if errors.Is(err, fs.ErrNotExist) {
		return os.RemoveAll(dir)
	}
	if err != nil {
		return err
	}
	defer func() { _ = f.Close() }()
	err = syscall.Flock(int(f.Fd()), syscall.LOCK_EX|syscall.LOCK_NB)
	if errors.Is(err, syscall.EWOULDBLOCK) {
		return nil
	}
	if err != nil {
		return err
	}
	return os.RemoveAll(dir)
}
