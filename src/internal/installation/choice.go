package installation

import (
	"os"
	"path/filepath"
)

// Choice is what the setup asks: where the app keeps its own data ("" for
// the default) and the folder whose shared folder holds the destinations of
// every installation that uses it ("" when they are not shared). Libraries
// are not part of it: each one names its own folder (ADR-0047).
type Choice struct {
	LibDir    string
	SharedDir string
}

// Decide turns a choice into the configuration to save. cur is the saved
// configuration and fromDir the folder that holds the destinations now.
//
// Sharing means the shared folder inside the chosen folder, made there if it
// is not there yet; choosing a shared folder itself, or the one in use, keeps
// it. Not sharing keeps the destinations in the data folder.
func Decide(cur Config, ch Choice, fromDir string) (Config, error) {
	next := cur
	next.PhotosDir = ""
	next.LibDir = ch.LibDir
	switch {
	case ch.SharedDir == "":
		next.ChannelsDir = ""
	case ch.SharedDir == cur.ChannelsDir:
		// shared there already, e.g. a -channels-dir kept from an older launcher
	default:
		dir, err := Share(sharingFolder(ch.SharedDir), fromDir)
		if err != nil {
			return Config{}, err
		}
		next.ChannelsDir = dir
	}
	return next, nil
}

// sharingFolder is the folder the shared folder is made in: picked, or its
// parent when the shared folder itself was picked.
func sharingFolder(picked string) string {
	if filepath.Base(picked) == SharedDirName {
		return filepath.Dir(picked)
	}
	return picked
}

// Migrate turns a config.json from the time of one photo folder into one
// without: a shared folder found in the photo folder becomes the chosen
// shared folder, and the photo folder is forgotten. A photo folder that is
// not there (a NAS not mounted) is left for a later start, or its shared
// folder would be lost. changed says whether there is anything to save.
func Migrate(cfg Config) (migrated Config, changed bool) {
	if cfg.PhotosDir == "" {
		return cfg, false
	}
	if info, err := os.Stat(cfg.PhotosDir); err != nil || !info.IsDir() {
		return cfg, false
	}
	if cfg.ChannelsDir == "" {
		cfg.ChannelsDir = FindShared(cfg.PhotosDir)
	}
	cfg.PhotosDir = ""
	return cfg, true
}
