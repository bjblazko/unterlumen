package installation

import (
	"os"
	"path/filepath"
	"strings"
)

// ToolsDir is where the installer puts helper programs (ffmpeg, exiftool,
// cwebp) it fetched without a package manager: beside config.json, so that
// installing a new version of the app does not remove them.
func ToolsDir() (string, error) {
	path, err := Path()
	if err != nil {
		return "", err
	}
	return filepath.Join(filepath.Dir(path), "tools"), nil
}

// AddToolsToPath puts ToolsDir in front of PATH when it exists, so the
// helper programs are found however the app was started. exiftool comes as a
// folder with its library beside it, so its own folder is added as well.
func AddToolsToPath() {
	dir, err := ToolsDir()
	if err != nil {
		return
	}
	var dirs []string
	for _, d := range []string{filepath.Join(dir, "exiftool"), dir} {
		if info, err := os.Stat(d); err == nil && info.IsDir() {
			dirs = append(dirs, d)
		}
	}
	if len(dirs) == 0 {
		return
	}
	dirs = append(dirs, os.Getenv("PATH"))
	os.Setenv("PATH", strings.Join(dirs, string(os.PathListSeparator))) //nolint:errcheck
}
