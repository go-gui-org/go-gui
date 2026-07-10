// Package imgpath provides shared image path validation used
// by all GPU backends (GL, Metal).
package imgpath

import (
	"fmt"
	"path/filepath"
	"strings"
)

// ResolveWithParentFallback resolves symlinks, falling back to
// resolving only the parent directory if the full path fails
// (e.g. file does not yet exist).
func ResolveWithParentFallback(path string) string {
	if p, err := filepath.EvalSymlinks(path); err == nil {
		return p
	}
	dir := filepath.Dir(path)
	if d, err := filepath.EvalSymlinks(dir); err == nil {
		return filepath.Join(d, filepath.Base(path))
	}
	return path
}

// ValidateAllowed checks that path falls under at least one of
// the allowed roots.
func ValidateAllowed(
	path string, allowedRoots []string,
) error {
	for i := range allowedRoots {
		root := strings.TrimSpace(allowedRoots[i])
		if root == "" {
			continue
		}
		if WithinRoot(path, root) {
			return nil
		}
		rootAbs, err := filepath.Abs(root)
		if err != nil {
			continue
		}
		if WithinRoot(path,
			ResolveWithParentFallback(rootAbs)) {
			return nil
		}
	}
	return fmt.Errorf("image path not allowed: %s", path)
}

// NormalizeRoots returns absolute, symlink-resolved versions
// of the given roots, dropping empty or invalid entries.
func NormalizeRoots(allowedRoots []string) []string {
	if len(allowedRoots) == 0 {
		return nil
	}
	roots := make([]string, 0, len(allowedRoots))
	for i := range allowedRoots {
		root := strings.TrimSpace(allowedRoots[i])
		if root == "" {
			continue
		}
		rootAbs, err := filepath.Abs(root)
		if err != nil {
			continue
		}
		roots = append(roots,
			ResolveWithParentFallback(rootAbs))
	}
	return roots
}

// WithinRoot reports whether path is equal to or a
// descendant of root.
func WithinRoot(path, root string) bool {
	rel, err := filepath.Rel(root, path)
	if err != nil {
		return false
	}
	return rel == "." || (rel != ".." &&
		!strings.HasPrefix(rel,
			".."+string(filepath.Separator)))
}
