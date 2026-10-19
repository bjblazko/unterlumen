package library

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
)

// GlobalSettings holds user preferences that are not tied to a specific library.
type GlobalSettings struct {
	LibrarySortMode string `json:"librarySortMode,omitempty"`
	// CacheLimit is the most the cache may hold, in bytes: 0 for no limit,
	// nil for DefaultCacheLimit.
	CacheLimit *int64 `json:"cacheLimit,omitempty"`
}

// DefaultCacheLimit is the cache's limit until the owner sets another.
const DefaultCacheLimit int64 = 5 << 30

// CacheLimitBytes is the cache limit the settings ask for.
func (s *GlobalSettings) CacheLimitBytes() int64 {
	if s.CacheLimit == nil {
		return DefaultCacheLimit
	}
	return *s.CacheLimit
}

func (m *Manager) settingsPath() string {
	return filepath.Join(m.root, "settings.json")
}

// GetSettings reads global settings from disk. Missing file returns empty defaults.
func (m *Manager) GetSettings() (*GlobalSettings, error) {
	data, err := os.ReadFile(m.settingsPath())
	if errors.Is(err, os.ErrNotExist) {
		return &GlobalSettings{}, nil
	}
	if err != nil {
		return nil, err
	}
	var s GlobalSettings
	if err := json.Unmarshal(data, &s); err != nil {
		return &GlobalSettings{}, nil
	}
	return &s, nil
}

// SaveSettings writes global settings to disk.
func (m *Manager) SaveSettings(s *GlobalSettings) error {
	data, err := json.Marshal(s)
	if err != nil {
		return err
	}
	return os.WriteFile(m.settingsPath(), data, 0o600)
}
