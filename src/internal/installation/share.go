package installation

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
)

// SharedDirName is the folder inside the photo folder where installations
// that show the same photos keep what they share: channels.json and the album
// register. An installation that finds it there uses it, so two installations
// share by convention instead of by two matching -channels-dir flags.
const SharedDirName = ".unterlumen-shared"

// FindShared returns the shared folder inside photosDir, or "" when there is
// none.
func FindShared(photosDir string) string {
	if photosDir == "" {
		return ""
	}
	dir := filepath.Join(photosDir, SharedDirName)
	if info, err := os.Stat(dir); err == nil && info.IsDir() {
		return dir
	}
	return ""
}

// IsDiskRoot says dir is the top of a disk — "/" or "C:\\" — where nothing
// can be shared: no other installation sees this computer's whole disk, and
// the top of a macOS system disk cannot even be written.
func IsDiskRoot(dir string) bool {
	clean := filepath.Clean(dir)
	return filepath.Dir(clean) == clean
}

// ErrShareAtDiskRoot is why Share refused a disk root, in a sentence.
var ErrShareAtDiskRoot = errors.New("destinations cannot be shared from the top of a disk; choose the photo folder both installations see, for example the NAS folder, as the photo folder")

// Share makes the shared folder inside photosDir and moves this
// installation's destinations there: channels.json and the album register are
// copied from fromDir (where they were kept until now) unless the shared
// folder already has its own — what is already shared always wins.
func Share(photosDir, fromDir string) (string, error) {
	if IsDiskRoot(photosDir) {
		return "", fmt.Errorf("%w (%s)", ErrShareAtDiskRoot, photosDir)
	}
	dir := filepath.Join(photosDir, SharedDirName)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", err
	}
	if fromDir == "" || fromDir == dir {
		return dir, nil
	}
	if err := copyIfAbsent(filepath.Join(fromDir, "channels.json"), filepath.Join(dir, "channels.json")); err != nil {
		return "", err
	}
	if err := copyIfAbsent(filepath.Join(fromDir, "albums"), filepath.Join(dir, "albums")); err != nil {
		return "", err
	}
	return dir, nil
}

// copyIfAbsent copies a file or a folder tree from src to dst when src exists
// and dst does not.
func copyIfAbsent(src, dst string) error {
	if _, err := os.Stat(dst); err == nil {
		return nil
	}
	if _, err := os.Stat(src); errors.Is(err, fs.ErrNotExist) {
		return nil
	}
	return filepath.WalkDir(src, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, _ := filepath.Rel(src, path)
		target := filepath.Join(dst, rel)
		if d.IsDir() {
			return os.MkdirAll(target, 0o755)
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		return os.WriteFile(target, data, 0o644)
	})
}
