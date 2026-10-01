// Package installation is what one installation of Unterlumen remembers
// about itself: where its own data lives and whether it shares its
// destinations with another installation. Libraries name their own folders
// (ADR-0047).
//
// The launchers used to carry all of this as command-line flags, which only a
// terminal could change. It lives in config.json now, so the app can be set up
// in the browser (ADR-0042).
package installation

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"runtime"
)

// Config is the content of config.json. An empty field means "the default".
type Config struct {
	PhotosDir   string `json:"photosDir,omitempty"` // only before ADR-0047; Migrate removes it
	LibDir      string `json:"libDir,omitempty"`
	ChannelsDir string `json:"channelsDir,omitempty"`
	Port        int    `json:"port,omitempty"`
}

// DesktopPort is the installed app's port, apart from the 8080 a server or a
// development build uses. An app started from the .dmg has no config.json
// on its first start and takes it from here.
const DesktopPort = 8090

// Path is where config.json lives: the user's configuration folder, the same
// on every start whatever flags the launcher passes.
func Path() (string, error) {
	dir, err := os.UserConfigDir()
	if err != nil {
		return "", fmt.Errorf("finding the configuration folder: %w", err)
	}
	return filepath.Join(dir, "Unterlumen", "config.json"), nil
}

// Load reads config.json. found is false when there is none yet, which is
// what a first start looks like.
func Load() (cfg Config, found bool, err error) {
	path, err := Path()
	if err != nil {
		return Config{}, false, err
	}
	data, err := os.ReadFile(path)
	if errors.Is(err, fs.ErrNotExist) {
		return Config{}, false, nil
	}
	if err != nil {
		return Config{}, false, err
	}
	if err := json.Unmarshal(data, &cfg); err != nil {
		return Config{}, false, fmt.Errorf("reading %s: %w", path, err)
	}
	return cfg, true, nil
}

// Save writes config.json, creating its folder.
func Save(cfg Config) error {
	path, err := Path()
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, append(data, '\n'), 0o644)
}

// DefaultLibDir is where an installation keeps its own data — libraries,
// thumbnails, generated output — when none is chosen: beside config.json, and
// on Linux in the user's data folder.
func DefaultLibDir() (string, error) {
	if runtime.GOOS == "linux" {
		if dir := os.Getenv("XDG_DATA_HOME"); dir != "" {
			return filepath.Join(dir, "unterlumen"), nil
		}
		home, err := os.UserHomeDir()
		if err != nil {
			return "", err
		}
		return filepath.Join(home, ".local", "share", "unterlumen"), nil
	}
	path, err := Path()
	if err != nil {
		return "", err
	}
	return filepath.Dir(path), nil
}
