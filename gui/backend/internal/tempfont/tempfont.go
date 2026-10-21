// Package tempfont writes font files to the temporary directory for
// backends that must hand a persistent path to font loaders: a unique
// file per call (Write), or one fixed file reused across launches
// (WriteFixed).
package tempfont

import (
	"os"
	"path/filepath"

	"github.com/go-gui-org/go-gui/gui/internal/atomicfile"
)

// Write creates a unique temporary font file containing data
// and returns its path.
func Write(prefix string, data []byte) (string, error) {
	f, err := os.CreateTemp("", prefix+"-*.ttf")
	if err != nil {
		return "", err
	}
	path := f.Name()
	if _, err := f.Write(data); err != nil {
		_ = f.Close()
		_ = os.Remove(path)
		return "", err
	}
	if err := f.Close(); err != nil {
		_ = os.Remove(path)
		return "", err
	}
	return path, nil
}

// WriteFixed writes data to a fixed name in the temporary directory.
// It is for cache-backed platforms where process teardown is not
// reliable and a unique file on every launch would accumulate.
//
// The file is replaced atomically, so the path never holds a partly
// written font: not after a crash mid-write, and not for another
// process of the app that opens it meanwhile.
func WriteFixed(name string, data []byte) (string, error) {
	path := filepath.Join(os.TempDir(), name)
	if err := atomicfile.WriteFile(path, data, 0o600); err != nil {
		return "", err
	}
	return path, nil
}
