package library

import (
	"context"
	"fmt"
	"hash/fnv"
	"strconv"
)

// MemoryPhotos returns the photos Memories choose from in the libraries
// asked for (all when ids is empty), within the shared filter, each photo
// once: from the first library that has it. version changes whenever what
// they were read from does — a scan, a face named, a thing found, a look
// measured — and the photos are cached under it.
func (m *Manager) MemoryPhotos(ctx context.Context, ids []string, f Filter) ([]MemoryPhoto, string, error) {
	libs, err := m.filterLibraries(ids)
	if err != nil {
		return nil, "", err
	}
	version := m.memoryVersion(libs, f)
	key := statsCacheKey(libraryIDs(libs), f.Key())
	if v, ok := loadChanging(&m.memoriesCache, key, version); ok {
		return v.([]MemoryPhoto), version, nil
	}
	photos, err := m.readMemoryPhotos(ctx, libs, f)
	if err != nil {
		return nil, "", err
	}
	storeChanging(&m.memoriesCache, key, version, photos)
	return photos, version, nil
}

func (m *Manager) readMemoryPhotos(ctx context.Context, libs []*Library, f Filter) ([]MemoryPhoto, error) {
	seen := map[string]bool{}
	all := []MemoryPhoto{}
	for _, l := range libs {
		store, err := m.OpenFiltered(l.ID, f)
		if err != nil {
			continue // a library that cannot be read is left out, as on the Map
		}
		photos, err := store.MemoryPhotos(ctx)
		store.Close()
		if err != nil {
			return nil, err
		}
		for _, p := range photos {
			if seen[p.ID] {
				continue
			}
			seen[p.ID] = true
			p.LibraryID = l.ID
			all = append(all, p)
		}
	}
	return all, nil
}

// memoryVersion hashes the filter, each library's content stamp and number
// of measured looks, and the change counters of people and things.
func (m *Manager) memoryVersion(libs []*Library, f Filter) string {
	h := fnv.New64a()
	fmt.Fprintf(h, "%s;%s;%s;", f.Key(), m.changesOf(libs, "people"), m.changesOf(libs, "things"))
	for _, l := range libs {
		store, err := m.OpenReader(l.ID)
		if err != nil {
			continue
		}
		stamp, _ := store.ContentStamp()
		var looks int
		store.db.QueryRow(`SELECT COUNT(*) FROM photo_appearance`).Scan(&looks) //nolint:errcheck // 0 is a version too
		store.Close()
		fmt.Fprintf(h, "%s=%s/%d;", l.ID, stamp, looks)
	}
	return strconv.FormatUint(h.Sum64(), 36)
}
