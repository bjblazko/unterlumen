package library

import (
	"crypto/rand"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"sync"
	"time"
)

// Manager manages the set of libraries rooted at a base directory.
type Manager struct {
	root    string
	indexMu sync.Map // map[libraryID]bool — prevents concurrent reindex of same library
	scans   sync.Map // map[libraryID]*Broadcaster — active scan progress broadcasters
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

// StartScan acquires the index lock and registers a broadcaster for the library.
// Returns the broadcaster and true on success, or nil and false if already scanning.
func (m *Manager) StartScan(id string) (*Broadcaster, bool) {
	if !m.TryLockIndex(id) {
		return nil, false
	}
	b := newBroadcaster()
	m.scans.Store(id, b)
	return b, true
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
}

// IsScanning reports whether a scan is currently active for the library.
func (m *Manager) IsScanning(id string) bool {
	_, ok := m.scans.Load(id)
	return ok
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
	return agg, nil
}

// Statistics returns aggregated statistics across the requested libraries (or all if ids is nil).
// pathPrefix, when non-empty, restricts each library's results to photos whose path starts with that prefix.
func (m *Manager) Statistics(ids []string, pathPrefix string) (*LibraryStatistics, error) {
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

	merged := &LibraryStatistics{
		Formats:        []NameCount{},
		FilmSims:       []NameCount{},
		FocalLengths:   []ValueCount{},
		FocalLengths35: []ValueCount{},
		Apertures:      []ValueCount{},
		ISOs:           []ValueCount{},
		CameraLens:     []CameraLensCount{},
		ShootingDays:   make(map[string]int),
	}
	fmtMap    := make(map[string]int)
	filmMap   := make(map[string]int)
	clMap     := make(map[[2]string]int)
	focalMap  := make(map[float64]int)
	focal35Map := make(map[float64]int)
	aperMap   := make(map[float64]int)
	isoMap    := make(map[float64]int)

	for _, l := range libs {
		store, err := m.OpenStore(l.ID)
		if err != nil {
			continue
		}
		st, err := store.Statistics(pathPrefix)
		store.Close()
		if err != nil {
			continue
		}

		merged.TotalPhotos += st.TotalPhotos
		for _, nc := range st.Formats {
			fmtMap[nc.Name] += nc.Count
		}
		for _, nc := range st.FilmSims {
			filmMap[nc.Name] += nc.Count
		}
		for _, vc := range st.FocalLengths   { focalMap[vc.Value]   += vc.Count }
		for _, vc := range st.FocalLengths35 { focal35Map[vc.Value] += vc.Count }
		for _, vc := range st.Apertures      { aperMap[vc.Value]    += vc.Count }
		for _, vc := range st.ISOs           { isoMap[vc.Value]     += vc.Count }
		for _, clc := range st.CameraLens {
			clMap[[2]string{clc.Camera, clc.Lens}] += clc.Count
		}
		for h, n := range st.ShootingHours {
			merged.ShootingHours[h] += n
		}
		for day, n := range st.ShootingDays {
			merged.ShootingDays[day] += n
		}
	}

	for name, count := range fmtMap {
		merged.Formats = append(merged.Formats, NameCount{Name: name, Count: count})
	}
	sortNameCounts(merged.Formats)

	for name, count := range filmMap {
		merged.FilmSims = append(merged.FilmSims, NameCount{Name: name, Count: count})
	}
	sortNameCounts(merged.FilmSims)

	merged.FocalLengths   = mapToValueCounts(focalMap)
	merged.FocalLengths35 = mapToValueCounts(focal35Map)
	merged.Apertures      = mapToValueCounts(aperMap)
	merged.ISOs           = mapToValueCounts(isoMap)

	for key, count := range clMap {
		merged.CameraLens = append(merged.CameraLens, CameraLensCount{Camera: key[0], Lens: key[1], Count: count})
	}
	// Sort camera×lens by count descending, cap at 100.
	for i := 1; i < len(merged.CameraLens); i++ {
		for j := i; j > 0 && merged.CameraLens[j].Count > merged.CameraLens[j-1].Count; j-- {
			merged.CameraLens[j], merged.CameraLens[j-1] = merged.CameraLens[j-1], merged.CameraLens[j]
		}
	}
	if len(merged.CameraLens) > 100 {
		merged.CameraLens = merged.CameraLens[:100]
	}

	return merged, nil
}

// mapToValueCounts converts a value→count map to a []ValueCount sorted by value ascending.
func mapToValueCounts(m map[float64]int) []ValueCount {
	out := make([]ValueCount, 0, len(m))
	for v, n := range m {
		out = append(out, ValueCount{Value: v, Count: n})
	}
	for i := 1; i < len(out); i++ {
		for j := i; j > 0 && out[j].Value < out[j-1].Value; j-- {
			out[j], out[j-1] = out[j-1], out[j]
		}
	}
	return out
}
