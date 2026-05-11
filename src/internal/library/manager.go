package library

import (
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
)

// Manager manages the set of libraries rooted at a base directory.
type Manager struct {
	root              string
	indexMu           sync.Map // map[libraryID]bool — prevents concurrent reindex of same library
	scans             sync.Map // map[libraryID]*Broadcaster — active scan progress broadcasters
	openDBs           sync.Map // map[libraryID]*sql.DB — long-lived per-library connections
	statsCache        sync.Map // map[cacheKey]*LibraryStatistics — invalidated on scan start/end
	timelineCache     sync.Map // map[cacheKey]*LibraryTimeline — invalidated on scan start/end
	exifRangesCache   sync.Map // map[cacheKey]map[string]ExifRange — invalidated on scan start/end
	exifValuesCache   sync.Map // map[cacheKey+"|"+field][]string — invalidated on scan start/end
}

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
func (m *Manager) getDB(id string) (*sql.DB, error) {
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
	if actual, loaded := m.openDBs.LoadOrStore(id, db); loaded {
		db.Close() // another goroutine won the race; discard ours
		return actual.(*sql.DB), nil
	}
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
	if db, ok := m.openDBs.LoadAndDelete(id); ok {
		db.(*sql.DB).Close()
	}
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
	m.InvalidateStatsCache(id)
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
	m.InvalidateStatsCache(id)
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
		page, err := store.ListPhotos(textFilters, numericFilters, 0, perLibLimit)
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

	libIDs := make([]string, len(libs))
	for i, l := range libs {
		libIDs[i] = l.ID
	}
	cacheKey := statsCacheKey(libIDs, pathPrefix)
	if v, ok := m.statsCache.Load(cacheKey); ok {
		return v.(*LibraryStatistics), nil
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
			merged.Warnings = append(merged.Warnings, fmt.Sprintf("library %q could not be read", l.Name))
			continue
		}
		st, err := store.Statistics(pathPrefix)
		store.Close()
		if err != nil {
			merged.Warnings = append(merged.Warnings, fmt.Sprintf("library %q statistics unavailable", l.Name))
			continue
		}

		merged.TotalPhotos += st.TotalPhotos
		merged.IndexingPhotos += st.IndexingPhotos
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

	m.statsCache.Store(cacheKey, merged)
	return merged, nil
}

// Timeline returns time-series statistics across the requested libraries (or all if ids is nil).
func (m *Manager) Timeline(ids []string, pathPrefix, granularity string) (*LibraryTimeline, error) {
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

	tlLibIDs := make([]string, len(libs))
	for i, l := range libs {
		tlLibIDs[i] = l.ID
	}
	tlCacheKey := timelineCacheKey(tlLibIDs, pathPrefix, granularity)
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
	// Determine granularity: prefer "year" if any lib returned it.
	granularity := "month"
	for _, r := range results {
		if r.Granularity == "year" {
			granularity = "year"
			break
		}
	}

	// Build global period set.
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
	periodIdx := make(map[string]int, len(periods))
	for i, p := range periods {
		periodIdx[p] = i
	}

	// Merge camera usage: sum per (camera, period), then re-apply top-5.
	camTotals := make(map[string]int)
	camGrid := make(map[string][]int)
	for _, r := range results {
		srcIdx := make(map[string]int, len(r.Periods))
		for i, p := range r.Periods {
			srcIdx[p] = i
		}
		for _, cs := range r.CameraUsage {
			if camGrid[cs.Camera] == nil {
				camGrid[cs.Camera] = make([]int, len(periods))
			}
			for p, pi := range periodIdx {
				if si, ok := srcIdx[p]; ok && si < len(cs.Counts) {
					camGrid[cs.Camera][pi] += cs.Counts[si]
					camTotals[cs.Camera] += cs.Counts[si]
				}
			}
		}
	}
	type kv struct {
		k string
		v int
	}
	ranked := make([]kv, 0, len(camTotals))
	for k, v := range camTotals {
		ranked = append(ranked, kv{k, v})
	}
	for i := 1; i < len(ranked); i++ {
		for j := i; j > 0 && ranked[j].v > ranked[j-1].v; j-- {
			ranked[j], ranked[j-1] = ranked[j-1], ranked[j]
		}
	}
	top := 5
	if len(ranked) < top {
		top = len(ranked)
	}
	topSet := make(map[string]bool, top)
	for _, kv := range ranked[:top] {
		topSet[kv.k] = true
	}
	cameras := make([]CameraTimeSlice, 0, top+1)
	for _, kv := range ranked[:top] {
		cameras = append(cameras, CameraTimeSlice{Camera: kv.k, Counts: camGrid[kv.k]})
	}
	other := make([]int, len(periods))
	hasOther := false
	for cam, counts := range camGrid {
		if topSet[cam] {
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

	// Merge focal and ISO stats: weighted average for median/P25/P75.
	focalStats := mergePercentileStats(results, periods, func(r *LibraryTimeline) []PeriodStats { return r.FocalStats })
	isoStats := mergePercentileStats(results, periods, func(r *LibraryTimeline) []PeriodStats { return r.ISOStats })

	// Merge aperture heatmap: sum bucket counts.
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
	aperRows := make([]ApertureRow, 0, len(periods))
	for _, p := range periods {
		if buckets := aperMap[p]; len(buckets) > 0 {
			cp := make(map[string]int, len(buckets))
			for k, v := range buckets {
				cp[k] = v
			}
			aperRows = append(aperRows, ApertureRow{Period: p, Buckets: cp})
		}
	}

	// Merge aspect ratios: sum counts per (ratio, period).
	aspectGrid := make(map[string][]int)
	for _, r := range results {
		srcIdx := make(map[string]int, len(r.Periods))
		for i, p := range r.Periods {
			srcIdx[p] = i
		}
		for _, as := range r.AspectRatios {
			if aspectGrid[as.Ratio] == nil {
				aspectGrid[as.Ratio] = make([]int, len(periods))
			}
			for p, pi := range periodIdx {
				if si, ok := srcIdx[p]; ok && si < len(as.Counts) {
					aspectGrid[as.Ratio][pi] += as.Counts[si]
				}
			}
		}
	}
	aspectSlices := make([]AspectSlice, 0, len(tlAspectOrder))
	for _, ratio := range tlAspectOrder {
		counts := aspectGrid[ratio]
		for _, c := range counts {
			if c > 0 {
				cp := make([]int, len(counts))
				copy(cp, counts)
				aspectSlices = append(aspectSlices, AspectSlice{Ratio: ratio, Counts: cp})
				break
			}
		}
	}

	// Merge megapixels: max of maxes, weighted avg.
	mpMap := make(map[string]MegapixelStat)
	for _, r := range results {
		for _, ms := range r.MegapixelStats {
			cur := mpMap[ms.Period]
			if ms.Max > cur.Max {
				cur.Max = ms.Max
			}
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
	mpStats := make([]MegapixelStat, 0, len(periods))
	for _, p := range periods {
		if ms, ok := mpMap[p]; ok {
			mpStats = append(mpStats, ms)
		}
	}

	return &LibraryTimeline{
		Granularity:    granularity,
		Periods:        periods,
		CameraUsage:    cameras,
		FocalStats:     focalStats,
		ISOStats:       isoStats,
		ApertureHeat:   aperRows,
		AspectRatios:   aspectSlices,
		MegapixelStats: mpStats,
	}
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
	for i := 1; i < len(out); i++ {
		for j := i; j > 0 && out[j].Value < out[j-1].Value; j-- {
			out[j], out[j-1] = out[j-1], out[j]
		}
	}
	return out
}
