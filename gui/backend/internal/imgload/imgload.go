// Package imgload provides shared image path validation, safe file
// opening, and pixel decoding used by all GPU backends.
package imgload

import (
	"fmt"
	"image"
	"image/draw"
	_ "image/jpeg" // Register JPEG decoder.
	_ "image/png"  // Register PNG decoder.
	"os"
	"path/filepath"
	"strings"

	"github.com/go-gui-org/go-gui/gui/backend/internal/imgpath"
)

const (
	// DefaultMaxImageBytes is the default maximum encoded image
	// size accepted by DecodeNRGBA.
	DefaultMaxImageBytes = int64(16 * 1024 * 1024)
	// DefaultMaxImagePixels is the default maximum decoded pixel
	// count accepted by DecodeNRGBA.
	DefaultMaxImagePixels = int64(40_000_000)
)

// ResolveValidatedPath cleans, absolutizes, and resolves symlinks
// for src, then validates against allowedRoots (if non-empty).
func ResolveValidatedPath(
	src string, allowedRoots []string,
) (string, error) {
	if strings.ContainsRune(src, 0) {
		return "", fmt.Errorf("invalid image path: contains NUL")
	}
	cleanPath := filepath.Clean(src)
	if cleanPath == "." || cleanPath == "" {
		return "", fmt.Errorf("invalid image path")
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
	if err := validateFile(path, f, maxBytes, maxPixels); err != nil {
		return nil, fmt.Errorf("decode image: validate: %w", err)
	}
	if _, err := f.Seek(0, 0); err != nil {
		return nil, fmt.Errorf("decode image: seek: %w", err)
	}
	src, _, err := image.Decode(f)
	if err != nil {
		return nil, fmt.Errorf("decode image: %w", err)
	}
	if existing, ok := src.(*image.NRGBA); ok {
		return existing, nil
	}
	bounds := src.Bounds()
	nrgba := image.NewNRGBA(bounds)
	draw.Draw(nrgba, bounds, src, bounds.Min, draw.Src)
	return nrgba, nil
}

func validateFile(
	path string, f *os.File,
	maxBytes, maxPixels int64,
) error {
	if maxBytes <= 0 {
		maxBytes = DefaultMaxImageBytes
	}
	info, err := f.Stat()
	if err != nil {
		return fmt.Errorf("validate image %s: stat: %w", path, err)
	}
	if info.Size() > maxBytes {
		return fmt.Errorf("image file too large: %s", path)
	}
	if maxPixels <= 0 {
		maxPixels = DefaultMaxImagePixels
	}
	cfg, _, err := image.DecodeConfig(f)
	if err != nil {
		return fmt.Errorf("validate image %s: decode config: %w", path, err)
	}
	if cfg.Width <= 0 || cfg.Height <= 0 {
		return fmt.Errorf("invalid image dimensions: %s", path)
	}
	if int64(cfg.Width)*int64(cfg.Height) > maxPixels {
		return fmt.Errorf("image dimensions too large: %s", path)
	}
	return nil
}
