package library

import (
	"context"
	"crypto/rand"
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"sort"
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

func statsCacheKey(ids []string, pathPrefix string) string {
	sorted := append([]string(nil), ids...)
	sort.Strings(sorted)
	return strings.Join(sorted, ",") + "|" + pathPrefix
}

func timelineCacheKey(ids []string, pathPrefix, granularity string) string {
	sorted := append([]string(nil), ids...)
	sort.Strings(sorted)
	return strings.Join(sorted, ",") + "|" + pathPrefix + "|" + granularity
}

// InvalidateStatsCache removes cached statistics for all entries that include the given library ID.
func (m *Manager) InvalidateStatsCache(id string) {
	invalidateByID := func(cache *sync.Map) {
		cache.Range(func(k, _ any) bool {
			before, _, _ := strings.Cut(k.(string), "|")
			for _, part := range strings.Split(before, ",") {
				if part == id {
					cache.Delete(k)
					break
				}
			}
			return true
		})
	}
	invalidateByID(&m.statsCache)
	invalidateByID(&m.timelineCache)
	invalidateByID(&m.exifRangesCache)
	invalidateByID(&m.exifValuesCache)
	// Folder stats keys are "<libID>|<absPath>" — prefix match is exact.
	m.folderStatsCache.Range(func(k, _ any) bool {
		if strings.HasPrefix(k.(string), id+"|") {
			m.folderStatsCache.Delete(k)
		}
		return true
	})
}

// FolderStats returns DB-backed folder statistics for the given library and relative path.
// Results are cached and invalidated when the library is scanned.
func (m *Manager) FolderStats(id, relPath string) (*LibraryFolderStats, error) {
	store, err := m.OpenStore(id)
	if err != nil {
		return nil, err
	}
	defer store.Close()

	sourcePath, ok, _ := store.GetProp("source_path")
	if !ok || sourcePath == "" {
		return nil, fmt.Errorf("library has no source path")
	}

	folderAbs := sourcePath
	if relPath != "" {
		folderAbs = filepath.Join(sourcePath, relPath)
	}

	cacheKey := id + "|" + folderAbs
	if v, ok := m.folderStatsCache.Load(cacheKey); ok {
		return v.(*LibraryFolderStats), nil
	}

	stats, err := store.FolderStats(folderAbs)
	if err != nil {
		return nil, err
	}
	m.folderStatsCache.Store(cacheKey, stats)
	return stats, nil
}

// prewarmFolderStats pre-computes and caches FolderStats for every unique ancestor
// directory found in the library's indexed photos. Called in a background goroutine
// after each scan so that first-access latency is zero.
func (m *Manager) prewarmFolderStats(id string) {
	store, err := m.OpenStore(id)
	if err != nil {
		return
	}
	defer store.Close()

	sourcePath, ok, _ := store.GetProp("source_path")
	if !ok || sourcePath == "" {
		return
	}

	refs, err := store.ListAllPhotoRefs()
	if err != nil {
		return
	}

	// Collect all unique ancestor directories between each photo and sourcePath.
	dirs := map[string]bool{sourcePath: true}
	for _, ref := range refs {
		dir := filepath.Dir(ref.PathHint)
		for dir != sourcePath && strings.HasPrefix(dir, sourcePath) && !dirs[dir] {
			dirs[dir] = true
			dir = filepath.Dir(dir)
		}
	}

	for absDir := range dirs {
		relPath, err := filepath.Rel(sourcePath, absDir)
		if err != nil {
			continue
		}
		if relPath == "." {
			relPath = ""
		}
		m.FolderStats(id, relPath) //nolint:errcheck
	}
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

// FindLibraryForPath returns the Library whose source_path covers absPath
// (exact match or a path within it). Returns nil, false if no library matches.
func (m *Manager) FindLibraryForPath(absPath string) (*Library, bool) {
	libs, err := m.ListLibraries()
	if err != nil {
		return nil, false
	}
	for _, l := range libs {
		sp := strings.TrimSuffix(l.SourcePath, "/")
		if sp == "" {
			continue
		}
		if absPath == sp || strings.HasPrefix(absPath, sp+"/") {
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

// TriggerScanNewBackground starts an incremental scan for the library in a background
// goroutine. It is a no-op when a scan is already running for that library.
func (m *Manager) TriggerScanNewBackground(id string) {
	b, started := m.StartScan(id, "Scanning")
	if !started {
		return
	}
	store, err := m.OpenStore(id)
	if err != nil {
		m.EndScan(id)
		b.Close()
		return
	}
	libInfo, err := libraryFromStore(id, store)
	if err != nil || libInfo.SourcePath == "" {
		m.EndScan(id)
		b.Close()
		return
	}
	rawCh := make(chan Progress, 8)
	go func() {
		defer b.Close()
		for p := range rawCh {
			b.Send(p)
		}
	}()
	go func() {
		defer m.EndScan(id)
		idx := NewIndexer(store, m.LibDir(id), libInfo.SourcePath)
		idx.RunScanNew(context.Background(), rawCh)
	}()
}

// IndexFilesSync synchronously indexes the given absolute file paths in the named
// library. Returns true if at least one new photo was indexed. Errors per file are
// silently skipped so partial results are still returned.
// IndexFilesSync indexes absPaths into the library and reports whether the
// caller should refresh any view of this library. This covers both brand-new
// photos and existing ones whose path_hint just changed (e.g. a rename or a
// move within the same library) — NewPhotos alone would miss the latter and
// leave callers thinking nothing happened when a file's path was in fact
// updated in the database.
func (m *Manager) IndexFilesSync(id string, absPaths []string) bool {
	store, err := m.OpenStore(id)
	if err != nil {
		return false
	}
	libInfo, err := libraryFromStore(id, store)
	if err != nil || libInfo.SourcePath == "" {
		return false
	}
	idx := NewIndexer(store, m.LibDir(id), libInfo.SourcePath)
	for _, p := range absPaths {
		_ = idx.IndexFile(p)
	}
	return len(absPaths) > 0
}

// TriggerScanNewInFolderBackground is like TriggerScanNewBackground but scoped to
// a single subfolder (relative to the library source path). A no-op if a scan is
// already running for this library.
func (m *Manager) TriggerScanNewInFolderBackground(id, subfolder string) {
	b, started := m.StartScan(id, "Scanning")
	if !started {
		return
	}
	store, err := m.OpenStore(id)
	if err != nil {
		m.EndScan(id)
		b.Close()
		return
	}
	libInfo, err := libraryFromStore(id, store)
	if err != nil || libInfo.SourcePath == "" {
		m.EndScan(id)
		b.Close()
		return
	}
	rawCh := make(chan Progress, 8)
	go func() {
		defer b.Close()
		for p := range rawCh {
			b.Send(p)
		}
	}()
	go func() {
		defer m.EndScan(id)
		idx := NewIndexer(store, m.LibDir(id), libInfo.SourcePath)
		idx.RunScanNewInFolder(context.Background(), rawCh, subfolder)
	}()
}

// TriggerCleanupInFolderBackground marks photos in subfolder as missing when their
// source files no longer exist. Used after moves to clean up the source library.
// A no-op if a scan is already running for this library.
func (m *Manager) TriggerCleanupInFolderBackground(id, subfolder string) {
	b, started := m.StartScan(id, "Checking")
	if !started {
		return
	}
	store, err := m.OpenStore(id)
	if err != nil {
		m.EndScan(id)
		b.Close()
		return
	}
	libInfo, err := libraryFromStore(id, store)
	if err != nil || libInfo.SourcePath == "" {
		m.EndScan(id)
		b.Close()
		return
	}
	rawCh := make(chan Progress, 8)
	go func() {
		defer b.Close()
		for p := range rawCh {
			b.Send(p)
		}
	}()
	go func() {
		defer m.EndScan(id)
		idx := NewIndexer(store, m.LibDir(id), libInfo.SourcePath)
		idx.RunCleanupInFolder(context.Background(), rawCh, subfolder)
	}()
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

// StartScan acquires the index lock and registers a broadcaster for the library.
// Returns the broadcaster and true on success, or nil and false if already scanning.
// verb says what the scan does ("Scanning", "Indexing"); with the library's
// name it is the job's title in the status line.
func (m *Manager) StartScan(id, verb string) (*Broadcaster, bool) {
	if !m.TryLockIndex(id) {
		return nil, false
	}
	m.InvalidateStatsCache(id)
	b := newBroadcaster()
	b.job = m.jobs.Start("library", m.scanTitle(id, verb), "libraries")
	m.scans.Store(id, b)
	return b, true
}

func (m *Manager) scanTitle(id, verb string) string {
	if l, err := m.readLibrary(id); err == nil && l.Name != "" {
		return fmt.Sprintf("%s %q", verb, l.Name)
	}
	return verb + " a library"
}

// JoinScan returns the active broadcaster for the library, if any.
func (m *Manager) JoinScan(id string) (*Broadcaster, bool) {
	if v, ok := m.scans.Load(id); ok {
		return v.(*Broadcaster), true
	}
	return nil, false
}

// EndScan removes the broadcaster and releases the index lock.
// The broadcaster itself must be closed separately (by the bridge goroutine).
func (m *Manager) EndScan(id string) {
	m.scans.Delete(id)
	m.UnlockIndex(id)
	m.InvalidateStatsCache(id)
	go m.prewarmFolderStats(id)
}

// IsScanning reports whether a scan is currently active for the library.
func (m *Manager) IsScanning(id string) bool {
	_, ok := m.scans.Load(id)
	return ok
}

// AggregateExifFieldValues returns the merged, deduplicated distinct string values
// for a given EXIF field across the requested libraries (or all if ids is nil).
func (m *Manager) AggregateExifFieldValues(ids []string, field string) ([]string, error) {
	libs, err := m.filterLibraries(ids)
	if err != nil {
		return nil, err
	}

	libIDs := make([]string, len(libs))
	for i, l := range libs {
		libIDs[i] = l.ID
	}
	cacheKey := statsCacheKey(libIDs, "") + "|" + field
	if v, ok := m.exifValuesCache.Load(cacheKey); ok {
		return v.([]string), nil
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
	m.exifValuesCache.Store(cacheKey, out)
	return out, nil
}

// SearchLibraries queries one or more libraries with the given filter opts and
// returns a merged, date-taken-sorted result. Pass nil ids to search all libraries.
// At most (Offset + Limit) photos are fetched per library; the total reflects the true match count.
func (m *Manager) SearchLibraries(ids []string, opts ListPhotosOpts) (CrossLibraryResult, error) {
	libs, err := m.filterLibraries(ids)
	if err != nil {
		return CrossLibraryResult{}, err
	}

	// Query each library (sequentially; stores are single-connection SQLite).
	// At most Offset+Limit photos per library can land on the requested page.
	perLibOpts := opts
	perLibOpts.Offset = 0
	perLibOpts.Limit = opts.Offset + opts.Limit
	var all []LibraryPhoto
	total := 0
	for _, l := range libs {
		photos, n := m.searchLibrary(l, perLibOpts)
		all = append(all, photos...)
		total += n
	}
	// Merge and sort by date taken, newest first, undated last.
	sortLibraryPhotos(all)

	// Apply offset/limit.
	if opts.Offset >= len(all) {
		return CrossLibraryResult{Results: []LibraryPhoto{}, Total: total}, nil
	}
	end := opts.Offset + opts.Limit
	if end > len(all) {
		end = len(all)
	}
	return CrossLibraryResult{Results: all[opts.Offset:end], Total: total}, nil
}

// searchLibrary returns one library's matching photos and their total. A
// library that cannot be opened or queried contributes nothing.
func (m *Manager) searchLibrary(l *Library, opts ListPhotosOpts) ([]LibraryPhoto, int) {
	store, err := m.OpenStore(l.ID)
	if err != nil {
		return nil, 0
	}
	page, err := store.ListPhotos(opts)
	store.Close()
	if err != nil {
		return nil, 0
	}
	photos := make([]LibraryPhoto, len(page.Photos))
	for k, p := range page.Photos {
		photos[k] = LibraryPhoto{LibraryID: l.ID, LibraryName: l.Name, Photo: p}
	}
	return photos, page.Total
}

func takenOrZero(p LibraryPhoto) time.Time {
	if len(p.DateTaken) < 10 {
		return time.Time{}
	}
	s := p.DateTaken
	if len(s) > 19 {
		s = s[:19]
	}
	t, err := time.Parse("2006-01-02T15:04:05", s)
	if err != nil {
		return time.Time{}
	}
	return t
}

func sortLibraryPhotos(photos []LibraryPhoto) {
	sort.SliceStable(photos, func(i, j int) bool {
		ti := takenOrZero(photos[i])
		tj := takenOrZero(photos[j])
		if ti.IsZero() && tj.IsZero() {
			return false
		}
		if ti.IsZero() {
			return false // nulls last
		}
		if tj.IsZero() {
			return true // nulls last
		}
		return ti.After(tj) // newest first
	})
}

// AggregateExifRanges returns the combined min/max numeric EXIF ranges across
// the given libraries. Pass nil ids to aggregate all libraries.
func (m *Manager) AggregateExifRanges(ids []string) (map[string]ExifRange, error) {
	libs, err := m.filterLibraries(ids)
	if err != nil {
		return nil, err
	}

	libIDs := make([]string, len(libs))
	for i, l := range libs {
		libIDs[i] = l.ID
	}
	cacheKey := statsCacheKey(libIDs, "")
	if v, ok := m.exifRangesCache.Load(cacheKey); ok {
		return v.(map[string]ExifRange), nil
	}

	numericFields := []string{"ExposureTime", "FNumber", "FocalLength", "FocalLengthIn35mmFilm", "FocalLength35", "ISOSpeedRatings"}
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
	m.exifRangesCache.Store(cacheKey, agg)
	return agg, nil
}

// AggregateMetaKeys returns merged distinct photo_meta keys across the given libraries.
func (m *Manager) AggregateMetaKeys(ids []string) ([]string, error) {
	libs, err := m.filterLibraries(ids)
	if err != nil {
		return nil, err
	}
	seen := make(map[string]bool)
	var out []string
	for _, l := range libs {
		store, err := m.OpenStore(l.ID)
		if err != nil {
			continue
		}
		keys, err := store.GetMetaKeys()
		store.Close()
		if err != nil {
			continue
		}
		for _, k := range keys {
			if !seen[k] {
				seen[k] = true
				out = append(out, k)
			}
		}
	}
	sortStrings(out)
	return out, nil
}

// AggregateMetaValues returns merged distinct values for the given photo_meta key across the given libraries.
func (m *Manager) AggregateMetaValues(ids []string, key string) ([]string, error) {
	libs, err := m.filterLibraries(ids)
	if err != nil {
		return nil, err
	}
	seen := make(map[string]bool)
	var out []string
	for _, l := range libs {
		store, err := m.OpenStore(l.ID)
		if err != nil {
			continue
		}
		vals, err := store.GetMetaValues(key)
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
	sortStrings(out)
	return out, nil
}

// AggregateAlbumTitles returns merged distinct gallery/album titles across the given libraries.
func (m *Manager) AggregateAlbumTitles(ids []string) ([]string, error) {
	libs, err := m.filterLibraries(ids)
	if err != nil {
		return nil, err
	}
	seen := make(map[string]bool)
	var out []string
	for _, l := range libs {
		store, err := m.OpenStore(l.ID)
		if err != nil {
			continue
		}
		titles, err := store.GetAlbumTitles()
		store.Close()
		if err != nil {
			continue
		}
		for _, t := range titles {
			if !seen[t] {
				seen[t] = true
				out = append(out, t)
			}
		}
	}
	sortStrings(out)
	return out, nil
}

// AggregateExifFields returns the merged distinct EXIF field names across the given libraries.
func (m *Manager) AggregateExifFields(ids []string) ([]string, error) {
	libs, err := m.filterLibraries(ids)
	if err != nil {
		return nil, err
	}
	seen := make(map[string]bool)
	var out []string
	for _, l := range libs {
		store, err := m.OpenStore(l.ID)
		if err != nil {
			continue
		}
		fields, err := store.ExifFields()
		store.Close()
		if err != nil {
			continue
		}
		for _, f := range fields {
			if !seen[f] {
				seen[f] = true
				out = append(out, f)
			}
		}
	}
	sortStrings(out)
	return out, nil
}

// filterLibraries returns the subset of all libraries matching the given ids (or all if ids is nil).
func (m *Manager) filterLibraries(ids []string) ([]*Library, error) {
	libs, err := m.ListLibraries()
	if err != nil {
		return nil, err
	}
	if len(ids) == 0 {
		return libs, nil
	}
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
	return filtered, nil
}

// Statistics returns aggregated statistics across the requested libraries (or all if ids is nil).
// pathPrefix, when non-empty, restricts each library's results to photos whose path starts with that prefix.
func (m *Manager) Statistics(ids []string, pathPrefix string) (*LibraryStatistics, error) {
	libs, err := m.filterLibraries(ids)
	if err != nil {
		return nil, err
	}
	cacheKey := statsCacheKey(libraryIDs(libs), pathPrefix)
	if v, ok := m.statsCache.Load(cacheKey); ok {
		return v.(*LibraryStatistics), nil
	}

	sm := newStatsMerger()
	for _, l := range libs {
		st, warning := m.libraryStatistics(l, pathPrefix)
		if warning != "" {
			sm.merged.Warnings = append(sm.merged.Warnings, warning)
			continue
		}
		sm.add(st)
	}
	merged := sm.result()
	m.statsCache.Store(cacheKey, merged)
	return merged, nil
}

func libraryIDs(libs []*Library) []string {
	ids := make([]string, len(libs))
	for i, l := range libs {
		ids[i] = l.ID
	}
	return ids
}

// libraryStatistics returns one library's statistics, or the warning to show
// when they cannot be read.
func (m *Manager) libraryStatistics(l *Library, pathPrefix string) (*LibraryStatistics, string) {
	store, err := m.OpenStore(l.ID)
	if err != nil {
		return nil, fmt.Sprintf("library %q could not be read", l.Name)
	}
	st, err := store.Statistics(pathPrefix)
	store.Close()
	if err != nil {
		return nil, fmt.Sprintf("library %q statistics unavailable", l.Name)
	}
	return st, ""
}

// statsMerger sums the statistics of several libraries.
type statsMerger struct {
	merged     *LibraryStatistics
	formats    map[string]int
	filmSims   map[string]int
	cameraLens map[[2]string]int
	focal      map[float64]int
	focal35    map[float64]int
	apertures  map[float64]int
	isos       map[float64]int
}

func newStatsMerger() *statsMerger {
	return &statsMerger{
		merged:     &LibraryStatistics{ShootingDays: make(map[string]int)},
		formats:    make(map[string]int),
		filmSims:   make(map[string]int),
		cameraLens: make(map[[2]string]int),
		focal:      make(map[float64]int),
		focal35:    make(map[float64]int),
		apertures:  make(map[float64]int),
		isos:       make(map[float64]int),
	}
}

func (sm *statsMerger) add(st *LibraryStatistics) {
	sm.merged.TotalPhotos += st.TotalPhotos
	sm.merged.IndexingPhotos += st.IndexingPhotos
	addNameCounts(sm.formats, st.Formats)
	addNameCounts(sm.filmSims, st.FilmSims)
	addValueCounts(sm.focal, st.FocalLengths)
	addValueCounts(sm.focal35, st.FocalLengths35)
	addValueCounts(sm.apertures, st.Apertures)
	addValueCounts(sm.isos, st.ISOs)
	for _, clc := range st.CameraLens {
		sm.cameraLens[[2]string{clc.Camera, clc.Lens}] += clc.Count
	}
	for h, n := range st.ShootingHours {
		sm.merged.ShootingHours[h] += n
	}
	for day, n := range st.ShootingDays {
		sm.merged.ShootingDays[day] += n
	}
}

// result sorts the sums: names by count, values by value, and camera × lens
// by count, capped at 100.
func (sm *statsMerger) result() *LibraryStatistics {
	merged := sm.merged
	merged.Formats = mapToNameCounts(sm.formats)
	merged.FilmSims = mapToNameCounts(sm.filmSims)
	merged.FocalLengths = mapToValueCounts(sm.focal)
	merged.FocalLengths35 = mapToValueCounts(sm.focal35)
	merged.Apertures = mapToValueCounts(sm.apertures)
	merged.ISOs = mapToValueCounts(sm.isos)
	merged.CameraLens = make([]CameraLensCount, 0, len(sm.cameraLens))
	for key, count := range sm.cameraLens {
		merged.CameraLens = append(merged.CameraLens, CameraLensCount{Camera: key[0], Lens: key[1], Count: count})
	}
	sort.SliceStable(merged.CameraLens, func(i, j int) bool { return merged.CameraLens[i].Count > merged.CameraLens[j].Count })
	if len(merged.CameraLens) > 100 {
		merged.CameraLens = merged.CameraLens[:100]
	}
	return merged
}

func addNameCounts(sums map[string]int, ncs []NameCount) {
	for _, nc := range ncs {
		sums[nc.Name] += nc.Count
	}
}

func addValueCounts(sums map[float64]int, vcs []ValueCount) {
	for _, vc := range vcs {
		sums[vc.Value] += vc.Count
	}
}

// mapToNameCounts converts a name→count map to a []NameCount sorted by count descending.
func mapToNameCounts(m map[string]int) []NameCount {
	out := make([]NameCount, 0, len(m))
	for name, count := range m {
		out = append(out, NameCount{Name: name, Count: count})
	}
	sortNameCounts(out)
	return out
}

// Timeline returns time-series statistics across the requested libraries (or all if ids is nil).
func (m *Manager) Timeline(ids []string, pathPrefix, granularity string) (*LibraryTimeline, error) {
	libs, err := m.filterLibraries(ids)
	if err != nil {
		return nil, err
	}

	tlCacheKey := timelineCacheKey(libraryIDs(libs), pathPrefix, granularity)
	if v, ok := m.timelineCache.Load(tlCacheKey); ok {
		return v.(*LibraryTimeline), nil
	}

	var results []*LibraryTimeline
	for _, l := range libs {
		store, err := m.OpenStore(l.ID)
		if err != nil {
			continue
		}
		tl, err := store.Timeline(pathPrefix, granularity)
		store.Close()
		if err != nil {
			continue
		}
		results = append(results, tl)
	}
	if len(results) == 0 {
		return &LibraryTimeline{
			Granularity:    coalesceGranularity(granularity),
			Periods:        []string{},
			CameraUsage:    []CameraTimeSlice{},
			FocalStats:     []PeriodStats{},
			ISOStats:       []PeriodStats{},
			ApertureHeat:   []ApertureRow{},
			AspectRatios:   []AspectSlice{},
			MegapixelStats: []MegapixelStat{},
		}, nil
	}
	var tl *LibraryTimeline
	if len(results) == 1 {
		tl = results[0]
	} else {
		tl = mergeTLs(results)
	}
	m.timelineCache.Store(tlCacheKey, tl)
	return tl, nil
}

func coalesceGranularity(g string) string {
	if g == "year" {
		return "year"
	}
	return "month"
}

func mergeTLs(results []*LibraryTimeline) *LibraryTimeline {
	periods := unionPeriods(results)
	return &LibraryTimeline{
		Granularity:    mergedGranularity(results),
		Periods:        periods,
		CameraUsage:    mergeCameraUsage(results, periods),
		FocalStats:     mergePercentileStats(results, periods, func(r *LibraryTimeline) []PeriodStats { return r.FocalStats }),
		ISOStats:       mergePercentileStats(results, periods, func(r *LibraryTimeline) []PeriodStats { return r.ISOStats }),
		ApertureHeat:   mergeApertureHeat(results, periods),
		AspectRatios:   mergeAspectRatios(results, periods),
		MegapixelStats: mergeMegapixels(results, periods),
	}
}

// mergedGranularity prefers "year" if any library returned it.
func mergedGranularity(results []*LibraryTimeline) string {
	for _, r := range results {
		if r.Granularity == "year" {
			return "year"
		}
	}
	return "month"
}

// unionPeriods returns every period of any result, sorted.
func unionPeriods(results []*LibraryTimeline) []string {
	periodSet := make(map[string]bool)
	for _, r := range results {
		for _, p := range r.Periods {
			periodSet[p] = true
		}
	}
	periods := make([]string, 0, len(periodSet))
	for p := range periodSet {
		periods = append(periods, p)
	}
	sortStrings(periods)
	return periods
}

// periodGrid sums per-key count rows, each aligned to its own library's
// periods, onto the merged periods.
type periodGrid struct {
	idx  map[string]int
	rows map[string][]int
}

func newPeriodGrid(periods []string) *periodGrid {
	idx := make(map[string]int, len(periods))
	for i, p := range periods {
		idx[p] = i
	}
	return &periodGrid{idx: idx, rows: make(map[string][]int)}
}

// add sums counts, aligned to srcPeriods, into key's row. It returns their
// total and whether any count lined up with a period.
func (g *periodGrid) add(key string, srcPeriods []string, counts []int) (total int, overlapped bool) {
	if g.rows[key] == nil {
		g.rows[key] = make([]int, len(g.idx))
	}
	srcIdx := make(map[string]int, len(srcPeriods))
	for i, p := range srcPeriods {
		srcIdx[p] = i
	}
	for p, si := range srcIdx {
		if si < len(counts) {
			g.rows[key][g.idx[p]] += counts[si]
			total += counts[si]
			overlapped = true
		}
	}
	return total, overlapped
}

// mergeCameraUsage sums usage per (camera, period), then keeps the five most
// used cameras and adds up the rest as "Other".
func mergeCameraUsage(results []*LibraryTimeline, periods []string) []CameraTimeSlice {
	grid := newPeriodGrid(periods)
	totals := make(map[string]int)
	for _, r := range results {
		for _, cs := range r.CameraUsage {
			// A camera is ranked only once one of its counts lines up with a period.
			if n, overlapped := grid.add(cs.Camera, r.Periods, cs.Counts); overlapped {
				totals[cs.Camera] += n
			}
		}
	}
	ranked := make([]string, 0, len(totals))
	for camera := range totals {
		ranked = append(ranked, camera)
	}
	sort.SliceStable(ranked, func(i, j int) bool { return totals[ranked[i]] > totals[ranked[j]] })
	top := min(5, len(ranked))

	cameras := make([]CameraTimeSlice, 0, top+1)
	topSet := make(map[string]bool, top)
	for _, camera := range ranked[:top] {
		topSet[camera] = true
		cameras = append(cameras, CameraTimeSlice{Camera: camera, Counts: grid.rows[camera]})
	}
	other := make([]int, len(periods))
	hasOther := false
	for camera, counts := range grid.rows {
		if topSet[camera] {
			continue
		}
		hasOther = true
		for i, c := range counts {
			other[i] += c
		}
	}
	if hasOther {
		cameras = append(cameras, CameraTimeSlice{Camera: "Other", Counts: other})
	}
	return cameras
}

// mergeApertureHeat sums the aperture bucket counts per period.
func mergeApertureHeat(results []*LibraryTimeline, periods []string) []ApertureRow {
	aperMap := make(map[string]map[string]int)
	for _, r := range results {
		for _, row := range r.ApertureHeat {
			if aperMap[row.Period] == nil {
				aperMap[row.Period] = make(map[string]int)
			}
			for k, v := range row.Buckets {
				aperMap[row.Period][k] += v
			}
		}
	}
	rows := make([]ApertureRow, 0, len(periods))
	for _, p := range periods {
		if buckets := aperMap[p]; len(buckets) > 0 {
			rows = append(rows, ApertureRow{Period: p, Buckets: buckets})
		}
	}
	return rows
}

// mergeAspectRatios sums counts per (ratio, period), in the fixed ratio order,
// leaving out ratios no photo has.
func mergeAspectRatios(results []*LibraryTimeline, periods []string) []AspectSlice {
	grid := newPeriodGrid(periods)
	for _, r := range results {
		for _, as := range r.AspectRatios {
			grid.add(as.Ratio, r.Periods, as.Counts)
		}
	}
	slices := make([]AspectSlice, 0, len(tlAspectOrder))
	for _, ratio := range tlAspectOrder {
		counts := grid.rows[ratio]
		if anyPositive(counts) {
			slices = append(slices, AspectSlice{Ratio: ratio, Counts: counts})
		}
	}
	return slices
}

func anyPositive(counts []int) bool {
	for _, c := range counts {
		if c > 0 {
			return true
		}
	}
	return false
}

// mergeMegapixels takes the max of the maxes and the count-weighted average
// per period.
func mergeMegapixels(results []*LibraryTimeline, periods []string) []MegapixelStat {
	mpMap := make(map[string]MegapixelStat)
	for _, r := range results {
		for _, ms := range r.MegapixelStats {
			cur := mpMap[ms.Period]
			cur.Max = max(cur.Max, ms.Max)
			// Weighted average: (cur.Avg*cur.Count + ms.Avg*ms.Count) / (cur.Count + ms.Count)
			total := cur.Count + ms.Count
			if total > 0 {
				cur.Avg = (cur.Avg*float64(cur.Count) + ms.Avg*float64(ms.Count)) / float64(total)
			}
			cur.Count = total
			cur.Period = ms.Period
			mpMap[ms.Period] = cur
		}
	}
	stats := make([]MegapixelStat, 0, len(periods))
	for _, p := range periods {
		if ms, ok := mpMap[p]; ok {
			stats = append(stats, ms)
		}
	}
	return stats
}

func mergePercentileStats(results []*LibraryTimeline, periods []string, getter func(*LibraryTimeline) []PeriodStats) []PeriodStats {
	type acc struct {
		sumMedian, sumP25, sumP75 float64
		count                     int
	}
	byPeriod := make(map[string]acc)
	for _, r := range results {
		for _, ps := range getter(r) {
			a := byPeriod[ps.Period]
			a.sumMedian += ps.Median * float64(ps.Count)
			a.sumP25 += ps.P25 * float64(ps.Count)
			a.sumP75 += ps.P75 * float64(ps.Count)
			a.count += ps.Count
			byPeriod[ps.Period] = a
		}
	}
	out := make([]PeriodStats, 0, len(periods))
	for _, p := range periods {
		a, ok := byPeriod[p]
		if !ok || a.count == 0 {
			continue
		}
		out = append(out, PeriodStats{
			Period: p,
			Median: a.sumMedian / float64(a.count),
			P25:    a.sumP25 / float64(a.count),
			P75:    a.sumP75 / float64(a.count),
			Count:  a.count,
		})
	}
	return out
}

// mapToValueCounts converts a value→count map to a []ValueCount sorted by value ascending.
func mapToValueCounts(m map[float64]int) []ValueCount {
	out := make([]ValueCount, 0, len(m))
	for v, n := range m {
		out = append(out, ValueCount{Value: v, Count: n})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Value < out[j].Value })
	return out
}
