// Package atomicfile replaces a file's contents so that a reader, and a
// crash, see either the old bytes or the new bytes, never a partial
// write. It also reads a file with a size limit.
//
// The settings store (gui/settings.go) and the Android backend both use
// it. The write sequence follows go-term's internal/atomicfile, so the
// two repos behave the same way. It is internal on purpose: go-edit's
// document save must also follow symlinks and keep the file mode, and a
// settings helper must not carry that logic (issue #848).
package atomicfile

import (
	"errors"
	"fmt"
	"io"
	"io/fs"
	"math"
	"math/rand/v2"
	"os"
	"path/filepath"
	"strconv"
)

// ErrTooLarge is returned by ReadFile when the file is larger than the
// limit.
var ErrTooLarge = errors.New("atomicfile: file too large")

// maxTempTries bounds the search for a free staging name. Names carry a
// random 32-bit suffix, so a collision is rare; the bound only stops a
// loop on a broken file system.
const maxTempTries = 100

// WriteFile writes data to a staging file next to path, flushes it to
// disk, and renames it over path. The directory must exist.
//
// The staging file is created with perm filtered by the process umask,
// the same as os.WriteFile. os.CreateTemp is not used: it always creates
// 0600, and a later chmod to perm would ignore the umask.
//
// Sync runs before the rename. Without it, some file systems (XFS,
// btrfs, ext4 with noauto_da_alloc) can commit the rename before the
// data after a power cut, which leaves an empty file under the real
// name. The directory is not synced after the rename: a power cut can
// then undo the rename, but that leaves the old contents, which is
// still "old or new".
//
// On any error the staging file is removed and path is left unchanged.
func WriteFile(path string, data []byte, perm fs.FileMode) (err error) {
	dir, base := filepath.Split(path)
	f, err := createTemp(dir, base, perm)
	if err != nil {
		return err
	}
	staged := f.Name()
	// One cleanup for every failure below, so a new error branch cannot
	// forget to remove the staging file. Close after a successful Close
	// is a harmless second call.
	defer func() {
		if err != nil {
			_ = f.Close()
			_ = os.Remove(staged)
		}
	}()
	if _, err = f.Write(data); err != nil {
		return err
	}
	if err = f.Sync(); err != nil {
		return err
	}
	if err = f.Close(); err != nil {
		return err
	}
	return os.Rename(staged, path)
}

// ReadFile reads the whole file at path. A missing file returns
// (nil, nil), so a first run is not an error. A file larger than limit
// bytes returns ErrTooLarge and no data, so a huge or hostile file
// cannot make the caller allocate without bound.
func ReadFile(path string, limit int64) ([]byte, error) {
	if limit < 0 {
		return nil, fmt.Errorf("atomicfile: negative read limit %d", limit)
	}
	f, err := os.Open(path)
	if errors.Is(err, fs.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	defer func() { _ = f.Close() }()
	// Read one byte past the limit: getting it back proves the file is
	// too large without trusting Stat, which can be wrong for special
	// files. At math.MaxInt64 there is no byte past the limit, and
	// limit+1 would overflow to a negative N that reads nothing, so read
	// without a cap.
	var r io.Reader = f
	if limit < math.MaxInt64 {
		r = io.LimitReader(f, limit+1)
	}
	data, err := io.ReadAll(r)
	if err != nil {
		return nil, err
	}
	if int64(len(data)) > limit {
		return nil, fmt.Errorf("%w: %s is over %d bytes", ErrTooLarge, path, limit)
	}
	return data, nil
}

// createTemp opens a new, empty staging file in dir. The name starts
// with a dot so file managers hide it, and holds the target's name so a
// file left behind by a crash shows what it belonged to.
func createTemp(dir, hint string, perm fs.FileMode) (*os.File, error) {
	for range maxTempTries {
		name := filepath.Join(dir,
			"."+hint+"-"+strconv.FormatUint(uint64(rand.Uint32()), 10)+".tmp")
		// O_EXCL makes the "is it free?" test and the claim one atomic
		// step.
		f, err := os.OpenFile(name, os.O_WRONLY|os.O_CREATE|os.O_EXCL, perm)
		if errors.Is(err, fs.ErrExist) {
			continue
		}
		return f, err
	}
	return nil, &fs.PathError{Op: "createtemp", Path: filepath.Join(dir, hint), Err: fs.ErrExist}
}
