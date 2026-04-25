package library

import (
	"crypto/rand"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// Manager manages the set of libraries rooted at a base directory.
type Manager struct {
	root    string
	indexMu sync.Map // map[libraryID]bool — prevents concurrent reindex of same library
}

func newUUID() (string, error) {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	b[6] = (b[6] & 0x0f) | 0x40
	b[8] = (b[8] & 0x3f) | 0x80
	return fmt.Sprintf("%08x-%04x-%04x-%04x-%012x", b[0:4], b[4:6], b[6:8], b[8:10], b[10:]), nil
}

// NewManager creates a Manager for the given root (e.g. ~/.unterlumen).
// Creates the libraries subdirectory if it does not exist.
func NewManager(root string) (*Manager, error) {
	if err := os.MkdirAll(filepath.Join(root, "libraries"), 0o700); err != nil {
		return nil, fmt.Errorf("create libraries dir: %w", err)
	}
	return &Manager{root: root}, nil
}

// LibDir returns the data directory for the given library ID.
func (m *Manager) LibDir(id string) string {
	return filepath.Join(m.root, "libraries", id)
}

// OpenStore opens the SQLite store for the library with the given ID.
// The caller is responsible for closing the store.
func (m *Manager) OpenStore(id string) (*Store, error) {
	dir := m.LibDir(id)
	dbPath := filepath.Join(dir, "library.db")
	if _, err := os.Stat(dbPath); err != nil {
		return nil, fmt.Errorf("library %s not found", id)
	}
	return openStore(dbPath, dir)
}

// ListLibraries returns all known libraries by scanning the libraries directory.
func (m *Manager) ListLibraries() ([]*Library, error) {
	entries, err := os.ReadDir(filepath.Join(m.root, "libraries"))
	if err != nil {
		return nil, err
	}
	var libs []*Library
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		lib, err := m.readLibrary(e.Name())
		if err != nil {
			continue // skip corrupt entries
		}
		libs = append(libs, lib)
	}
	if libs == nil {
		libs = []*Library{}
	}
	return libs, nil
}

// GetLibrary returns the library with the given ID.
func (m *Manager) GetLibrary(id string) (*Library, error) {
	return m.readLibrary(id)
}

func (m *Manager) readLibrary(id string) (*Library, error) {
	store, err := m.OpenStore(id)
	if err != nil {
		return nil, err
	}
	defer store.Close()
	return libraryFromStore(id, store)
}

func libraryFromStore(id string, store *Store) (*Library, error) {
	lib := &Library{ID: id}

	if v, ok, _ := store.GetProp("name"); ok {
		lib.Name = v
	}
	if v, ok, _ := store.GetProp("description"); ok {
		lib.Description = v
	}
	if v, ok, _ := store.GetProp("source_path"); ok {
		lib.SourcePath = v
	}
	if v, ok, _ := store.GetProp("created_at"); ok {
		lib.CreatedAt, _ = time.Parse(time.RFC3339, v)
	}
	if v, ok, _ := store.GetProp("last_indexed"); ok {
		t, err := time.Parse(time.RFC3339, v)
		if err == nil {
			lib.LastIndexed = &t
		}
	}
	count, err := store.CountPhotos()
	if err != nil {
		return nil, err
	}
	lib.PhotoCount = count
	return lib, nil
}

// CreateLibrary creates a new library with the given name, description, and source path.
func (m *Manager) CreateLibrary(name, description, sourcePath string) (*Library, error) {
	id, err := newUUID()
	if err != nil {
		return nil, err
	}
	dir := m.LibDir(id)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return nil, fmt.Errorf("create library dir: %w", err)
	}
	if err := os.MkdirAll(filepath.Join(dir, "thumbs"), 0o700); err != nil {
		return nil, fmt.Errorf("create thumbs dir: %w", err)
	}

	store, err := openStore(filepath.Join(dir, "library.db"), dir)
	if err != nil {
		os.RemoveAll(dir)
		return nil, err
	}
	defer store.Close()

	now := time.Now().UTC()
	for k, v := range map[string]string{
		"name":        name,
		"description": description,
		"source_path": sourcePath,
		"created_at":  now.Format(time.RFC3339),
	} {
		if err := store.SetProp(k, v); err != nil {
			os.RemoveAll(dir)
			return nil, err
		}
	}

	return &Library{
		ID:          id,
		Name:        name,
		Description: description,
		SourcePath:  sourcePath,
		CreatedAt:   now,
		PhotoCount:  0,
	}, nil
}

// DeleteLibrary removes the library directory and all its data.
// The original photos are never touched.
func (m *Manager) DeleteLibrary(id string) error {
	if _, loaded := m.indexMu.LoadOrStore(id, true); loaded {
		return fmt.Errorf("library %s is currently being indexed", id)
	}
	defer m.indexMu.Delete(id)
	return os.RemoveAll(m.LibDir(id))
}

// ThumbDir returns the directory for storing thumbnails for a library.
func (m *Manager) ThumbDir(id string) string {
	return filepath.Join(m.LibDir(id), "thumbs")
}

// TryLockIndex acquires the indexing lock for a library.
// Returns true if the lock was acquired (not already indexing).
func (m *Manager) TryLockIndex(id string) bool {
	_, loaded := m.indexMu.LoadOrStore(id, true)
	return !loaded
}

// UnlockIndex releases the indexing lock for a library.
func (m *Manager) UnlockIndex(id string) {
	m.indexMu.Delete(id)
}
