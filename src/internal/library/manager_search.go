package library

import (
	"sort"
	"time"
)

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
