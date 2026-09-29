package installation

import (
	"os"
	"path/filepath"
	"runtime"
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
// folder with its library beside it, so its own folder is added as well. On
// macOS Homebrew's folders are added too: an app started from the Finder
// gets a PATH without them.
func AddToolsToPath() {
	dir, err := ToolsDir()
	if err != nil {
		return
	}
	candidates := []string{filepath.Join(dir, "exiftool"), dir}
	if runtime.GOOS == "darwin" {
		candidates = append(candidates, "/opt/homebrew/bin", "/usr/local/bin")
	}
	path := string(os.PathListSeparator) + os.Getenv("PATH") + string(os.PathListSeparator)
	var dirs []string
	for _, d := range candidates {
		if strings.Contains(path, string(os.PathListSeparator)+d+string(os.PathListSeparator)) {
			continue
		}
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
