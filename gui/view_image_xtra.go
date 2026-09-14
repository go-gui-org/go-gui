package gui

import (
	"errors"
	"fmt"
	"path/filepath"
	"slices"
	"strings"
)

// validImageExtensions lists supported image file extensions.
var validImageExtensions = []string{
	".png", ".jpg", ".jpeg",
}

// ValidateImageExtension checks that the file has a supported
// image extension.
func validateImageExtension(fileName string) error {
	ext := strings.ToLower(filepath.Ext(fileName))
	if slices.Contains(validImageExtensions, ext) {
		return nil
	}
	return fmt.Errorf("unsupported image format: %s", ext)
}

// ValidateImagePath checks that the file path is safe and has a
// valid extension. Rejects NUL bytes, empty/dot paths, and paths
// with ".." path components, mirroring the backend gate in
// imgload.ResolveValidatedPath so a source refused at draw time
// is refused here too.
// After filepath.Clean, ".." only survives as a leading component
// (e.g. "../foo"), so a prefix check suffices.
func validateImagePath(fileName string) error {
	if strings.ContainsRune(fileName, 0) {
		return errors.New("invalid image path: contains NUL")
	}
	clean := filepath.Clean(fileName)
	if clean == "." || clean == "" {
		return errors.New("invalid image path")
	}
	if clean == ".." ||
		strings.HasPrefix(clean, ".."+string(filepath.Separator)) {
		return errors.New("invalid image path: contains parent reference")
	}
	return validateImageExtension(clean)
}
