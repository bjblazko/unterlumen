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
// always as an absolute path. The channel's output folder (see outputPaths)
// wins; otherwise the default <outputBaseDir>/channels/<slug>/ path is used.
func (s *Store) OutputDir(slug string) string {
	if ch, err := s.Get(slug); err == nil && ch.OutputPath != "" {
		return ch.OutputPath
	}
	return filepath.Join(s.outputBase, "channels", slug)
}

// The output folder is a path on one machine, so it does not live in the
// shared channels.json but in output-paths.json under the installation's own
// output base directory (its -lib-dir): a map from slug to an absolute path,
// where an empty value means "the default, on purpose".
//
// A destination saved before that keeps its folder in the shared file. That
// value is read as this installation's only where the folder exists here, and
// Save never touches it, so saving on one installation cannot take the folder
// away from another that still reads it.
func (s *Store) outputPathsFile() string { return filepath.Join(s.outputBase, "output-paths.json") }

func (s *Store) readOutputPaths() map[string]string {
	paths := map[string]string{}
	if data, err := os.ReadFile(s.outputPathsFile()); err == nil {
		json.Unmarshal(data, &paths) //nolint:errcheck // an unreadable file means "no local paths"
	}
	return paths
}

func (s *Store) writeOutputPaths(paths map[string]string) error {
	if err := os.MkdirAll(s.outputBase, 0o700); err != nil {
		return err
	}
	data, err := json.MarshalIndent(paths, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(s.outputPathsFile(), data, 0o600)
}

// effectiveOutputPath is the output folder this installation uses for a
// channel whose shared record says sharedPath, or "" for the default.
func (s *Store) effectiveOutputPath(slug, sharedPath string, local map[string]string) string {
	if p, ok := local[slug]; ok {
		return p
	}
	if sharedPath == "" {
		return ""
	}
	abs := s.absoluteOutputPath(sharedPath)
	if info, err := os.Stat(abs); err == nil && info.IsDir() {
		return abs
	}
	return ""
}

// List returns all channels. Returns built-in defaults if channels.json does not exist yet.
func (s *Store) List() ([]*Channel, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	chs, err := s.loadLocked()
	if err != nil {
		return nil, err
	}
	local := s.readOutputPaths()
	for _, ch := range chs {
		ch.OutputPath = s.effectiveOutputPath(ch.Slug, ch.OutputPath, local)
	}
	return chs, nil
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
	// The shared record keeps whatever legacy folder it already had and never
	// gets a new one; the folder goes to this installation's own file.
	stored := *ch
	stored.OutputPath = ""
	replaced := false
	for i, existing := range chs {
		if existing.Slug == ch.Slug {
			stored.OutputPath = existing.OutputPath
			chs[i] = &stored
			replaced = true
		}
	}
	if !replaced {
		chs = append(chs, &stored)
	}
	if err := s.writeLocked(chs); err != nil {
		return err
	}
	paths := s.readOutputPaths()
	paths[ch.Slug] = ch.OutputPath
	return s.writeOutputPaths(paths)
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
	if err := s.writeLocked(filtered); err != nil {
		return err
	}
	if paths := s.readOutputPaths(); len(paths) > 0 {
		if _, ok := paths[slug]; ok {
			delete(paths, slug)
			return s.writeOutputPaths(paths)
		}
	}
	return nil
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
