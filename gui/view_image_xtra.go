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
func ValidateImageExtension(fileName string) error {
	ext := strings.ToLower(filepath.Ext(fileName))
	if slices.Contains(validImageExtensions, ext) {
		return nil
	}
	return fmt.Errorf("unsupported image format: %s", ext)
}

// ValidateImagePath checks that the file path is safe and has a
// valid extension. Rejects paths with ".." path components.
// After filepath.Clean, ".." only survives as a leading component
// (e.g. "../foo"), so a prefix check suffices.
func ValidateImagePath(fileName string) error {
	clean := filepath.Clean(fileName)
	if clean == ".." ||
		strings.HasPrefix(clean, ".."+string(filepath.Separator)) {
		return errors.New("invalid image path: contains parent reference")
	}
	return ValidateImageExtension(clean)
}
