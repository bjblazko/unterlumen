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

// AggregateExifFieldValues returns the merged, deduplicated distinct string values
// for a given EXIF field across the requested libraries (or all if ids is nil).
func (m *Manager) AggregateExifFieldValues(ids []string, field string) ([]string, error) {
	libs, err := m.ListLibraries()
	if err != nil {
		return nil, err
	}
	if len(ids) > 0 {
		set := make(map[string]bool, len(ids))
		for _, id := range ids {
			set[id] = true
		}
		filtered := libs[:0]
		for _, l := range libs {
			if set[l.ID] {
				filtered = append(filtered, l)
			}
		}
		libs = filtered
	}

	seen := make(map[string]bool)
	var out []string
	for _, l := range libs {
		store, err := m.OpenStore(l.ID)
		if err != nil {
			continue
		}
		vals, err := store.GetExifFieldValues(field)
		store.Close()
		if err != nil {
			continue
		}
		for _, v := range vals {
			if !seen[v] {
				seen[v] = true
				out = append(out, v)
			}
		}
	}
	// Sort merged result.
	for i := 1; i < len(out); i++ {
		for j := i; j > 0 && out[j] < out[j-1]; j-- {
			out[j], out[j-1] = out[j-1], out[j]
		}
	}
	return out, nil
}

// SearchLibraries queries one or more libraries with the given filters and
// returns a merged, IndexedAt-sorted result. Pass nil ids to search all libraries.
// At most 200 photos are fetched per library; the total reflects the true match count.
func (m *Manager) SearchLibraries(ids []string, textFilters map[string]string, numericFilters map[string]NumericFilter, offset, limit int) (CrossLibraryResult, error) {
	libs, err := m.ListLibraries()
	if err != nil {
		return CrossLibraryResult{}, err
	}

	// Filter to requested libraries when ids is specified.
	if len(ids) > 0 {
		set := make(map[string]bool, len(ids))
		for _, id := range ids {
			set[id] = true
		}
		filtered := libs[:0]
		for _, l := range libs {
			if set[l.ID] {
				filtered = append(filtered, l)
			}
		}
		libs = filtered
	}

	type libResult struct {
		photos []LibraryPhoto
		total  int
		err    error
	}

	type job struct {
		lib    *Library
		result libResult
	}

	results := make([]job, len(libs))
	for i, l := range libs {
		results[i].lib = l
	}

	// Query each library (sequentially; stores are single-connection SQLite).
	for i, j := range results {
		store, err := m.OpenStore(j.lib.ID)
		if err != nil {
			continue
		}
		perLibLimit := offset + limit
		page, err := store.ListPhotos("", textFilters, numericFilters, 0, perLibLimit)
		store.Close()
		if err != nil {
			results[i].result.err = err
			continue
		}
		photos := make([]LibraryPhoto, len(page.Photos))
		for k, p := range page.Photos {
			photos[k] = LibraryPhoto{
				LibraryID:   j.lib.ID,
				LibraryName: j.lib.Name,
				Photo:       p,
			}
		}
		results[i].result = libResult{photos: photos, total: page.Total}
	}

	// Merge and sort by IndexedAt DESC.
	var all []LibraryPhoto
	total := 0
	for _, j := range results {
		all = append(all, j.result.photos...)
		total += j.result.total
	}
	sortLibraryPhotos(all)

	// Apply offset/limit.
	if offset >= len(all) {
		return CrossLibraryResult{Results: []LibraryPhoto{}, Total: total}, nil
	}
	end := offset + limit
	if end > len(all) {
		end = len(all)
	}
	return CrossLibraryResult{Results: all[offset:end], Total: total}, nil
}

func sortLibraryPhotos(photos []LibraryPhoto) {
	for i := 1; i < len(photos); i++ {
		for j := i; j > 0 && photos[j].IndexedAt.After(photos[j-1].IndexedAt); j-- {
			photos[j], photos[j-1] = photos[j-1], photos[j]
		}
	}
}

// AggregateExifRanges returns the combined min/max numeric EXIF ranges across
// the given libraries. Pass nil ids to aggregate all libraries.
func (m *Manager) AggregateExifRanges(ids []string) (map[string]ExifRange, error) {
	libs, err := m.ListLibraries()
	if err != nil {
		return nil, err
	}
	if len(ids) > 0 {
		set := make(map[string]bool, len(ids))
		for _, id := range ids {
			set[id] = true
		}
		filtered := libs[:0]
		for _, l := range libs {
			if set[l.ID] {
				filtered = append(filtered, l)
			}
		}
		libs = filtered
	}

	numericFields := []string{"ExposureTime", "FNumber", "FocalLength", "ISOSpeedRatings"}
	agg := make(map[string]ExifRange)

	for _, l := range libs {
		store, err := m.OpenStore(l.ID)
		if err != nil {
			continue
		}
		ranges, err := store.GetExifRanges(numericFields)
		store.Close()
		if err != nil {
			continue
		}
		for field, r := range ranges {
			if cur, ok := agg[field]; ok {
				if r.Min < cur.Min {
					cur.Min = r.Min
				}
				if r.Max > cur.Max {
					cur.Max = r.Max
				}
				agg[field] = cur
			} else {
				agg[field] = r
			}
		}
	}
	return agg, nil
}
