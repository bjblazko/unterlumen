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

// Store manages the global channels.json file.
type Store struct {
	path string
	mu   sync.Mutex
}

// NewStore creates a Store rooted at libRoot (e.g. ~/.unterlumen).
func NewStore(libRoot string) *Store {
	return &Store{path: filepath.Join(libRoot, "channels.json")}
}

// OutputDir returns the filesystem path where output for the given channel slug is stored.
func (s *Store) OutputDir(slug string) string {
	return filepath.Join(filepath.Dir(s.path), "channels", slug)
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
