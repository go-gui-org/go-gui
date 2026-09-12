//go:build !js

package gui

import (
	"fmt"
	"os"
)

// storeDiagramPNG writes PNG bytes to a temp file and returns
// the file path. Native implementation. The prefix names the
// diagram kind and must come from internal constants (for
// example "mermaid" or "math"). Do not pass user input here.
// A failed write or close deletes the file and returns an
// error, so callers never cache the path of a partial file.
func storeDiagramPNG(
	pngBytes []byte, hash int64, prefix string,
) (string, error) {
	f, err := os.CreateTemp("",
		fmt.Sprintf("%s_%d_*.png", prefix, hash))
	if err != nil {
		return "", err
	}
	path := f.Name()
	written, err := f.Write(pngBytes)
	if err != nil {
		_ = f.Close()
		_ = os.Remove(path)
		return "", err
	}
	if written != len(pngBytes) {
		_ = f.Close()
		_ = os.Remove(path)
		return "", fmt.Errorf(
			"storeDiagramPNG: short write %d of %d bytes",
			written, len(pngBytes))
	}
	// Close can report a deferred write error (for example a
	// full disk). Check it before you return the path.
	if closeErr := f.Close(); closeErr != nil {
		_ = os.Remove(path)
		return "", closeErr
	}
	return path, nil
}

// removeDiagramPNG deletes a stored diagram file.
func removeDiagramPNG(path string) {
	_ = os.Remove(path)
}
