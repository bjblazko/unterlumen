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

// SafePathLogical validates path traversal without requiring the path to exist on disk.
// Use this when the resulting path is used only as a string (e.g. a DB query prefix),
// not for filesystem access, so the target directory need not be mounted or reachable.
func SafePathLogical(root, relative string) (string, bool) {
	if relative == "" {
		return root, true
	}

	cleaned := filepath.Clean(relative)
	if filepath.IsAbs(cleaned) {
		return "", false
	}

	full := filepath.Join(root, cleaned)
	if !strings.HasPrefix(full, root+string(filepath.Separator)) && full != root {
		return "", false
	}

	return full, true
}
