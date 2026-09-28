package library

import (
	"crypto/rand"
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	"huepattl.de/unterlumen/internal/jobs"
)

// Manager manages the set of libraries rooted at a base directory.
type Manager struct {
	root             string
	indexMu          sync.Map       // map[libraryID]bool — prevents concurrent reindex of same library
	scans            sync.Map       // map[libraryID]*Broadcaster — active scan progress broadcasters
	dbMu             sync.Mutex     // guards openDBs mutations so getDB can't race DeleteLibrary's evict+RemoveAll
	openDBs          sync.Map       // map[libraryID]*sql.DB — long-lived per-library connections
	statsCache       sync.Map       // map[cacheKey]*LibraryStatistics — invalidated on scan start/end
	timelineCache    sync.Map       // map[cacheKey]*LibraryTimeline — invalidated on scan start/end
	exifRangesCache  sync.Map       // map[cacheKey]map[string]ExifRange — invalidated on scan start/end
	exifValuesCache  sync.Map       // map[cacheKey+"|"+field][]string — invalidated on scan start/end
	folderStatsCache sync.Map       // map["<libID>|<absPath>"]*LibraryFolderStats — invalidated on scan start/end
	jobs             *jobs.Registry // where scans are reported to the status line; nil reports nowhere
}

// SetJobs wires the job register that scans report to.
func (m *Manager) SetJobs(r *jobs.Registry) { m.jobs = r }

// Jobs is the register that work on libraries and their galleries reports to.
func (m *Manager) Jobs() *jobs.Registry { return m.jobs }

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

// getDB returns a cached *sql.DB for the library, opening and migrating it on first access.
// The slow path (opening+caching a fresh handle) is serialized against
// DeleteLibrary via dbMu, so a request racing a delete can't reopen and
// re-cache a handle onto files that are mid-RemoveAll — see the fast-path
// comment below for the one residual, unavoidable race.
func (m *Manager) getDB(id string) (*sql.DB, error) {
	// Fast path: no lock. A concurrent DeleteLibrary could close this handle
	// just after we load it; that's an inherent close-vs-in-flight-use race
	// shared by any pooled resource and is not the bug this guards against
	// (a *stale, reopened* handle silently surviving a delete).
	if db, ok := m.openDBs.Load(id); ok {
		return db.(*sql.DB), nil
	}

	m.dbMu.Lock()
	defer m.dbMu.Unlock()

	// Re-check under the lock: another goroutine may have opened it already,
	// or DeleteLibrary may have just evicted+removed it while we waited.
	if db, ok := m.openDBs.Load(id); ok {
		return db.(*sql.DB), nil
	}
	dbPath := filepath.Join(m.LibDir(id), "library.db")
	if _, err := os.Stat(dbPath); err != nil {
		return nil, fmt.Errorf("library %s not found", id)
	}
	db, err := openDB(dbPath)
	if err != nil {
		return nil, err
	}
	m.openDBs.Store(id, db)
	return db, nil
}

// OpenStore returns a Store backed by a cached *sql.DB for the library.
// Store.Close is a no-op; the connection lifetime is managed by Manager.
func (m *Manager) OpenStore(id string) (*Store, error) {
	db, err := m.getDB(id)
	if err != nil {
		return nil, err
	}
	return newStore(db, m.LibDir(id)), nil
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
	if v, ok, _ := store.GetProp("last_new_photos"); ok {
		t, err := time.Parse(time.RFC3339, v)
		if err == nil {
			lib.LastNewPhotos = &t
		}
	}
	if v, ok, _ := store.GetProp("photo_count"); ok {
		if n, err := strconv.Atoi(v); err == nil {
			lib.PhotoCount = n
		}
	} else {
		count, err := store.CountPhotos()
		if err != nil {
			return nil, err
		}
		lib.PhotoCount = count
	}
	if v, ok, _ := store.GetProp("sort_position"); ok {
		if n, err := strconv.Atoi(v); err == nil {
			lib.SortPosition = &n
		}
	}
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

	db, err := openDB(filepath.Join(dir, "library.db"))
	if err != nil {
		os.RemoveAll(dir)
		return nil, err
	}
	m.openDBs.Store(id, db) // cache before any failure so it's always cleaned up via DeleteLibrary
	store := newStore(db, dir)

	now := time.Now().UTC()
	for k, v := range map[string]string{
		"name":        name,
		"description": description,
		"source_path": sourcePath,
		"created_at":  now.Format(time.RFC3339),
	} {
		if err := store.SetProp(k, v); err != nil {
			m.openDBs.Delete(id)
			db.Close()
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

	// Hold dbMu across evict+RemoveAll so a concurrent getDB can't reopen
	// and re-cache a handle in the gap between eviction and removal.
	m.dbMu.Lock()
	defer m.dbMu.Unlock()
	if db, ok := m.openDBs.LoadAndDelete(id); ok {
		db.(*sql.DB).Close()
	}
	return os.RemoveAll(m.LibDir(id))
}

// UpdateLibrary updates the name and description of an existing library.
func (m *Manager) UpdateLibrary(id, name, description string) (*Library, error) {
	store, err := m.OpenStore(id)
	if err != nil {
		return nil, err
	}
	defer store.Close()
	if err := store.SetProp("name", name); err != nil {
		return nil, err
	}
	if err := store.SetProp("description", description); err != nil {
		return nil, err
	}
	return libraryFromStore(id, store)
}

// SetLibrarySortOrder writes a sort_position prop to each listed library in order.
func (m *Manager) SetLibrarySortOrder(ids []string) error {
	for i, id := range ids {
		store, err := m.OpenStore(id)
		if err != nil {
			return err
		}
		err = store.SetProp("sort_position", strconv.Itoa(i))
		store.Close()
		if err != nil {
			return err
		}
	}
	return nil
}

// ThumbDir returns the directory for storing thumbnails for a library.
func (m *Manager) ThumbDir(id string) string {
	return filepath.Join(m.LibDir(id), "thumbs")
}

// LibrariesForPath returns every library whose source path covers absPath
// (the folder itself or one above it). Libraries may overlap, so a folder can
// be in several.
func (m *Manager) LibrariesForPath(absPath string) []*Library {
	libs, err := m.ListLibraries()
	if err != nil {
		return nil
	}
	var covering []*Library
	for _, l := range libs {
		if covers(l.SourcePath, absPath) {
			covering = append(covering, l)
		}
	}
	return covering
}

func covers(sourcePath, absPath string) bool {
	sp := strings.TrimSuffix(sourcePath, "/")
	return sp != "" && (absPath == sp || strings.HasPrefix(absPath, sp+"/"))
}

// FindLibraryForPath returns the Library whose source_path covers absPath
// (exact match or a path within it). Returns nil, false if no library matches.
func (m *Manager) FindLibraryForPath(absPath string) (*Library, bool) {
	libs, err := m.ListLibraries()
	if err != nil {
		return nil, false
	}
	for _, l := range libs {
		if covers(l.SourcePath, absPath) {
			return l, true
		}
	}
	return nil, false
}

// FindThumbnailForPath returns the path of the pre-generated library thumbnail for absPath,
// if the file is indexed in a library's path_cache and the thumbnail exists on disk.
func (m *Manager) FindThumbnailForPath(absPath string) (string, bool) {
	lib, ok := m.FindLibraryForPath(absPath)
	if !ok {
		return "", false
	}
	store, err := m.OpenStore(lib.ID)
	if err != nil {
		return "", false
	}
	photoID, _, _, found, err := store.GetPathCache(absPath)
	if err != nil || !found || len(photoID) < 2 {
		return "", false
	}
	tp := filepath.Join(m.ThumbDir(lib.ID), photoID[:2], photoID+".jpg")
	if _, statErr := os.Stat(tp); statErr != nil {
		return "", false
	}
	return tp, true
}
