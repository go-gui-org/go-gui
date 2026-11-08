// Package imgload provides shared image path validation, safe file
// opening, and pixel decoding used by all GPU backends.
package imgload

import (
	"errors"
	"fmt"
	"image"
	"os"
	"path/filepath"
	"strings"

	"github.com/go-gui-org/go-gui/gui/backend/internal/imgpath"
	"github.com/go-gui-org/go-gui/gui/internal/imgdecode"
)

const (
	// DefaultMaxImageBytes is the default maximum encoded image
	// size accepted by DecodeNRGBA.
	DefaultMaxImageBytes = imgdecode.DefaultMaxImageBytes
	// DefaultMaxImagePixels is the default maximum decoded pixel
	// count accepted by DecodeNRGBA.
	DefaultMaxImagePixels = imgdecode.DefaultMaxImagePixels
)

// ResolveValidatedPath cleans, absolutizes, and resolves symlinks
// for src, then validates against allowedRoots (if non-empty).
func ResolveValidatedPath(
	src string, allowedRoots []string,
) (string, error) {
	if strings.ContainsRune(src, 0) {
		return "", errors.New("invalid image path: contains NUL")
	}
	cleanPath := filepath.Clean(src)
	if cleanPath == "." || cleanPath == "" {
		return "", errors.New("invalid image path")
	}
	pathAbs, err := filepath.Abs(cleanPath)
	if err != nil {
		return "", fmt.Errorf("invalid image path: %w", err)
	}
	resolvedPath := imgpath.ResolveWithParentFallback(pathAbs)
	if len(allowedRoots) > 0 {
		if err := imgpath.ValidateAllowed(
			resolvedPath, allowedRoots); err != nil {
			return "", err
		}
	}
	return resolvedPath, nil
}

// OpenSafe opens path with a pre/post stat TOCTOU guard.
// If allowedRoots is non-empty, the path is re-validated
// before opening.
func OpenSafe(
	path string, allowedRoots []string,
) (*os.File, error) {
	if len(allowedRoots) > 0 {
		if err := imgpath.ValidateAllowed(
			path, allowedRoots); err != nil {
			return nil, err
		}
	}
	pre, err := os.Stat(path)
	if err != nil {
		return nil, fmt.Errorf("open safe: stat: %w", err)
	}
	// #nosec G304 — path validated through AllowedImageRoots + TOCTOU guard
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("open safe: open: %w", err)
	}
	post, err := f.Stat()
	if err != nil {
		_ = f.Close()
		return nil, fmt.Errorf("open safe: fstat: %w", err)
	}
	if !os.SameFile(pre, post) {
		_ = f.Close()
		return nil, fmt.Errorf(
			"image path changed during open: %s", path)
	}
	return f, nil
}

// DecodeNRGBA validates image limits, decodes the file, and
// returns the pixels as *image.NRGBA. The file is seeked to 0
// before decoding.
func DecodeNRGBA(
	path string, f *os.File,
	maxBytes, maxPixels int64,
) (*image.NRGBA, error) {
	return imgdecode.DecodeNRGBA(path, f, maxBytes, maxPixels)
}
