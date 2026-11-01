package media

import (
	"os"
	"path/filepath"
	"strings"
)

// RemoveSidecar deletes the XMP sidecar of a photo that was just deleted,
// unless another file beside it has the same name up to its extension — a
// RAW beside its JPEG, a Live Photo's video beside its still — and so still
// owns that sidecar. It is a no-op when there is no sidecar, and when the
// deleted file was a sidecar itself.
func RemoveSidecar(photoPath string) error {
	sidecar := SidecarPath(photoPath)
	if sidecar == photoPath || SidecarShared(photoPath) {
		return nil
	}
	if err := os.Remove(sidecar); err != nil && !os.IsNotExist(err) {
		return err
	}
	return nil
}

// SidecarShared says whether another file beside path has the same name up
// to its extension, and so the same sidecar.
func SidecarShared(path string) bool {
	stem := strings.TrimSuffix(path, filepath.Ext(path))
	others, _ := filepath.Glob(globEscape(stem) + ".*")
	for _, o := range others {
		if o != path && !strings.EqualFold(filepath.Ext(o), ".xmp") {
			return true
		}
	}
	return false
}

// globEscape keeps the characters a pattern reads specially as themselves.
func globEscape(s string) string {
	return strings.NewReplacer(`\`, `\\`, "*", `\*`, "?", `\?`, "[", `\[`).Replace(s)
}
