package channels

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"

	"huepattl.de/unterlumen/internal/media"
)

var builtinChannels = []*Channel{
	{
		Slug: "instagram", Name: "Instagram",
		Format: "jpeg", Quality: 90, ExifMode: "keep_no_gps",
		Scale: media.ScaleOptions{Mode: media.ScaleModeMaxDim, MaxDimension: "width", MaxValue: 1080},
	},
	{
		Slug: "mastodon", Name: "Mastodon",
		Format: "jpeg", Quality: 85, ExifMode: "keep_no_gps",
		Scale: media.ScaleOptions{Mode: media.ScaleModeMaxDim, MaxDimension: "width", MaxValue: 1920},
	},
	{
		Slug: "website", Name: "Website",
		Format: "jpeg", Quality: 85, ExifMode: "strip",
		Scale:         media.ScaleOptions{Mode: media.ScaleModeMaxDim, MaxDimension: "width", MaxValue: 2400},
		GalleryExport: true,
	},
}

// Store manages the channels.json file.
type Store struct {
	path       string
	configDir  string
	outputBase string
	// boundary is the browse root. A channel's OutputPath is picked with the
	// folder picker, which browses inside that root and returns paths relative
	// to it — so a relative OutputPath has to be resolved against the same
	// root, not against the server process's working directory.
	boundary string
	mu       sync.Mutex
}

// NewStore creates a Store whose channels.json lives in configDir and whose default
// export output lives under outputBaseDir/channels/<slug>/. The two may be the same
// directory (e.g. ~/.unterlumen) or different — e.g. when configDir is a directory
// shared between multiple installations while outputBaseDir stays machine-local.
func NewStore(configDir, outputBaseDir string) *Store {
	return &Store{path: filepath.Join(configDir, "channels.json"), configDir: configDir, outputBase: outputBaseDir}
}

// ConfigDir is the directory holding channels.json — the one that is shared
// between installations.
func (s *Store) ConfigDir() string { return s.configDir }

// AlbumRegisterDir is where a website channel's album register lives: one file
// per album, beside channels.json so every installation sees the same albums.
// It is deliberately not under OutputDir, which is per machine.
func (s *Store) AlbumRegisterDir(slug string) string {
	return filepath.Join(s.configDir, "albums", slug)
}

// WithBoundary records the browse root used to resolve relative output paths.
// Without it a relative OutputPath silently depends on the working directory
// the server happens to be started from: the same configuration then finds a
// channel's galleries when launched from "/" and finds nothing when launched
// from a project folder.
func (s *Store) WithBoundary(boundary string) *Store {
	s.boundary = boundary
	return s
}

// absoluteOutputPath turns a folder-picker path into one that means the same
// thing wherever the server is started from. The picker browses inside the
// browse root and returns paths relative to it, which read as relative
// filesystem paths everywhere else; storing them absolute keeps what is on
// screen, in channels.json and on disk the same thing.
func (s *Store) absoluteOutputPath(path string) string {
	if path == "" || filepath.IsAbs(path) {
		return path
	}
	return filepath.Join(s.boundary, path)
}

// OutputDir returns the effective output directory for the given channel slug,
// always as an absolute path. A custom OutputPath wins; a relative one is
// resolved against the browse root. Otherwise the default
// <outputBaseDir>/channels/<slug>/ path is used.
func (s *Store) OutputDir(slug string) string {
	if ch, err := s.Get(slug); err == nil && ch.OutputPath != "" {
		// Still resolved on read: entries written before paths were stored
		// absolute keep working without rewriting anyone's configuration.
		return s.absoluteOutputPath(ch.OutputPath)
	}
	return filepath.Join(s.outputBase, "channels", slug)
}

// List returns all channels. Returns built-in defaults if channels.json does not exist yet.
func (s *Store) List() ([]*Channel, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.loadLocked()
}

// Get returns the channel with the given slug.
func (s *Store) Get(slug string) (*Channel, error) {
	chs, err := s.List()
	if err != nil {
		return nil, err
	}
	for _, ch := range chs {
		if ch.Slug == slug {
			return ch, nil
		}
	}
	return nil, fmt.Errorf("channel %q not found", slug)
}

// Save creates or replaces the channel with the matching slug.
func (s *Store) Save(ch *Channel) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	ch.OutputPath = s.absoluteOutputPath(ch.OutputPath)
	chs, err := s.loadLocked()
	if err != nil {
		return err
	}
	for i, existing := range chs {
		if existing.Slug == ch.Slug {
			chs[i] = ch
			return s.writeLocked(chs)
		}
	}
	return s.writeLocked(append(chs, ch))
}

// Delete removes the channel with the given slug.
func (s *Store) Delete(slug string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	chs, err := s.loadLocked()
	if err != nil {
		return err
	}
	filtered := chs[:0]
	for _, ch := range chs {
		if ch.Slug != slug {
			filtered = append(filtered, ch)
		}
	}
	return s.writeLocked(filtered)
}

func (s *Store) loadLocked() ([]*Channel, error) {
	data, err := os.ReadFile(s.path)
	if os.IsNotExist(err) {
		// Return a copy of builtins so callers can't mutate the package-level slice.
		out := make([]*Channel, len(builtinChannels))
		for i, ch := range builtinChannels {
			cp := *ch
			out[i] = &cp
		}
		return out, nil
	}
	if err != nil {
		return nil, err
	}
	var chs []*Channel
	return chs, json.Unmarshal(data, &chs)
}

func (s *Store) writeLocked(chs []*Channel) error {
	if err := os.MkdirAll(filepath.Dir(s.path), 0o700); err != nil {
		return err
	}
	data, err := json.MarshalIndent(chs, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(s.path, data, 0o600)
}
