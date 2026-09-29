// Package settingsdir keeps each app's settings blob (gui.SaveSettings)
// as a file under a directory the host app gives at run time. The
// Android backend uses it with Context.getFilesDir().
//
// It is a separate package, not a file in gui/backend/android, so that
// it is pure Go with no build tag: its test runs on the host in CI
// instead of only on a device.
package settingsdir

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sync"

	"github.com/go-gui-org/go-gui/gui/internal/atomicfile"
)

// maxSettingsBytes matches gui's cap on one app's settings blob. gui
// checks the size again after the hook returns; this cap stops the read
// itself from allocating without bound.
const maxSettingsBytes = 1 << 20

// ErrNoDir is returned before the host app has given a directory.
var ErrNoDir = errors.New("settingsdir: no settings directory: the host must call android.SetFilesDir before Start")

// Store keeps each app's settings blob in a file under the
// app's private files directory (Context.getFilesDir()). Go cannot find
// that directory by itself: os.UserConfigDir needs HOME, which Android
// does not set for an app process.
type Store struct {
	mu  sync.Mutex
	dir string
}

// SetDir sets the directory. An empty dir makes Load and Save fail.
func (s *Store) SetDir(dir string) {
	s.mu.Lock()
	s.dir = dir
	s.mu.Unlock()
}

// path returns <dir>/<appID>/settings.json. gui has already checked
// that appID is a single path element.
func (s *Store) path(appID string) (string, error) {
	s.mu.Lock()
	dir := s.dir
	s.mu.Unlock()
	if dir == "" {
		return "", ErrNoDir
	}
	return filepath.Join(dir, appID, "settings.json"), nil
}

// Load returns the blob for appID, or (nil, nil) when none is saved.
func (s *Store) Load(appID string) ([]byte, error) {
	path, err := s.path(appID)
	if err != nil {
		return nil, err
	}
	return atomicfile.ReadFile(path, maxSettingsBytes)
}

// Save replaces the blob for appID atomically.
func (s *Store) Save(appID string, data []byte) error {
	if len(data) > maxSettingsBytes {
		return fmt.Errorf("settingsdir: settings are %d bytes, over the %d-byte limit",
			len(data), maxSettingsBytes)
	}
	path, err := s.path(appID)
	if err != nil {
		return err
	}
	if err = os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	// 0o600: the files directory is already private to the app, so no
	// other user needs to read it.
	return atomicfile.WriteFile(path, data, 0o600)
}
