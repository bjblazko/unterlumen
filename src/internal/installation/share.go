package installation

import (
	"errors"
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

// Share makes the shared folder inside photosDir and moves this
// installation's destinations there: channels.json and the album register are
// copied from fromDir (where they were kept until now) unless the shared
// folder already has its own — what is already shared always wins.
func Share(photosDir, fromDir string) (string, error) {
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
