package pathguard

import (
	"path/filepath"
	"strings"
)

// withTrailingSeparator returns root with exactly one trailing path
// separator, so it can be used as an unambiguous "is-inside" prefix (e.g.
// "/foo" -> "/foo/"). Without this, blindly appending a separator breaks
// when root is already the filesystem root ("/" + "/" = "//", which is a
// prefix of nothing) — a real, deployed configuration (boundary "/" — no
// navigation restriction), not just a theoretical edge case.
func withTrailingSeparator(root string) string {
	if strings.HasSuffix(root, string(filepath.Separator)) {
		return root
	}
	return root + string(filepath.Separator)
}

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
		if resolvedParent != resolvedRoot && !strings.HasPrefix(resolvedParent, withTrailingSeparator(resolvedRoot)) {
			return "", false
		}
		return filepath.Join(resolvedParent, filepath.Base(full)), true
	}

	if resolved != resolvedRoot && !strings.HasPrefix(resolved, withTrailingSeparator(resolvedRoot)) {
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
	if !strings.HasPrefix(full, withTrailingSeparator(root)) && full != root {
		return "", false
	}

	return full, true
}
