package library

import (
	"fmt"
	"path/filepath"
	"sort"
	"strings"
	"sync"
)

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
	dropEntriesOf(&m.statsCache, id)
	dropEntriesOf(&m.timelineCache, id)
	dropEntriesOf(&m.colourCache, id)
	dropEntriesOf(&m.colourSpaceCache, id)
	dropEntriesOf(&m.exifRangesCache, id)
	dropEntriesOf(&m.exifValuesCache, id)
	// Folder stats keys are "<libID>|<absPath>" — prefix match is exact.
	m.folderStatsCache.Range(func(k, _ any) bool {
		if strings.HasPrefix(k.(string), id+"|") {
			m.folderStatsCache.Delete(k)
		}
		return true
	})
}

// dropEntriesOf deletes the entries whose key ("<id>,<id>|…") names the library.
func dropEntriesOf(cache *sync.Map, id string) {
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
