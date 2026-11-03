package library

import (
	"context"
	"fmt"
	"hash/fnv"
	"strconv"
	"sync"
)

// memoriesKept is how many filters' photos are kept at once; another one
// pushes the oldest out.
const memoriesKept = 4

// memoriesKeys are the filters whose photos are cached, oldest first.
type memoriesKeys struct {
	mu   sync.Mutex
	keys []string
}

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
	m.keepMemories(key, version, photos)
	return photos, version, nil
}

// keepMemories caches the photos under key, and forgets the filters asked
// least recently beyond memoriesKept.
func (m *Manager) keepMemories(key, version string, photos []MemoryPhoto) {
	m.memoriesKeys.mu.Lock()
	defer m.memoriesKeys.mu.Unlock()
	keys := []string{}
	for _, k := range m.memoriesKeys.keys {
		if k != key {
			keys = append(keys, k)
		}
	}
	keys = append(keys, key)
	for len(keys) > memoriesKept {
		m.memoriesCache.Delete(keys[0])
		keys = keys[1:]
	}
	m.memoriesKeys.keys = keys
	storeChanging(&m.memoriesCache, key, version, photos)
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

// memoryVersion hashes the filter, each library's content stamp, the number
// of measured looks and when the last was measured, and the change counters
// of people and things.
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
		var measured string
		store.db.QueryRow(`SELECT COUNT(*), COALESCE(CAST(MAX(analysed_at) AS TEXT), '') FROM photo_appearance`).Scan(&looks, &measured) //nolint:errcheck // 0 is a version too
		store.Close()
		fmt.Fprintf(h, "%s=%s/%d/%s;", l.ID, stamp, looks, measured)
	}
	return strconv.FormatUint(h.Sum64(), 36)
}
