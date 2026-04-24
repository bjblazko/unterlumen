package pathguard

import (
	"path/filepath"
	"strings"
)

// SafePath resolves a relative path within root and ensures it doesn't escape
// the boundary via symlinks or path traversal. Returns the absolute path or
// (_, false) if the path is invalid or escapes root.
func SafePath(root, relative string) (string, bool) {
	// Resolve symlinks in root itself so prefix comparisons work on real paths.
	resolvedRoot, err := filepath.EvalSymlinks(root)
	if err != nil {
		resolvedRoot = root
	}

	if relative == "" {
		return resolvedRoot, true
	}

	cleaned := filepath.Clean(relative)
	if filepath.IsAbs(cleaned) {
		return "", false
	}

	full := filepath.Join(resolvedRoot, cleaned)

	resolved, err := filepath.EvalSymlinks(full)
	if err != nil {
		// File might not exist yet (e.g. copy destination); validate parent instead.
		parent := filepath.Dir(full)
		resolvedParent, err := filepath.EvalSymlinks(parent)
		if err != nil {
			return "", false
		}
		if !strings.HasPrefix(resolvedParent, resolvedRoot) {
			return "", false
		}
		return filepath.Join(resolvedParent, filepath.Base(full)), true
	}

	if !strings.HasPrefix(resolved, resolvedRoot) {
		return "", false
	}

	return resolved, true
}
