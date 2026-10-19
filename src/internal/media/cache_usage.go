package media

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"sync"
	"sync/atomic"
)

// CacheOlder is the segment of files older versions left at the top of the
// cache folder, before each kind had a folder. Nothing reads them any more.
const CacheOlder = "older"

// CacheSegment is one kind of file in the cache and the room it takes.
type CacheSegment struct {
	ID    string `json:"id"`
	Bytes int64  `json:"bytes"`
	Files int    `json:"files"`
}

// olderCacheFile matches a file an older version wrote: a hex name, at the
// top of the folder. Anything else there (macOS keeps its own Cache.db in a
// folder of the app's name) is not ours and stays.
var olderCacheFile = regexp.MustCompile(`^[0-9a-f]{24}\.jpg$`)

// CacheSegments measures the cache, kind by kind.
func CacheSegments() []CacheSegment {
	dir := getCacheDir()
	out := []CacheSegment{}
	for _, kind := range []string{cacheThumbnails, cacheImages} {
		seg := CacheSegment{ID: kind}
		for _, f := range cacheFiles(filepath.Join(dir, kind)) {
			seg.Bytes += f.size
			seg.Files++
		}
		out = append(out, seg)
	}
	older := CacheSegment{ID: CacheOlder}
	for _, f := range olderCacheFiles() {
		older.Bytes += f.size
		older.Files++
	}
	return append(out, older)
}

// ErrNoSuchSegment says a segment to clear is not one of the cache's.
var ErrNoSuchSegment = errors.New("no such part of the cache")

// ClearCache removes every file of one segment; "" clears all of them.
func ClearCache(segment string) error {
	dir := getCacheDir()
	var files []cacheFile
	switch segment {
	case cacheThumbnails, cacheImages:
		files = cacheFiles(filepath.Join(dir, segment))
	case CacheOlder:
		files = olderCacheFiles()
	case "":
		for _, s := range []string{cacheThumbnails, cacheImages, CacheOlder} {
			if err := ClearCache(s); err != nil {
				return err
			}
		}
		return nil
	default:
		return ErrNoSuchSegment
	}
	for _, f := range files {
		os.Remove(f.path) //nolint:errcheck
	}
	return nil
}

type cacheFile struct {
	path  string
	size  int64
	mtime int64
}

func cacheFiles(dir string) []cacheFile {
	var out []cacheFile
	filepath.WalkDir(dir, func(path string, d fs.DirEntry, err error) error { //nolint:errcheck
		if err != nil || d.IsDir() {
			return nil
		}
		if fi, err := d.Info(); err == nil {
			out = append(out, cacheFile{path, fi.Size(), fi.ModTime().UnixNano()})
		}
		return nil
	})
	return out
}

func olderCacheFiles() []cacheFile {
	dir := getCacheDir()
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil
	}
	var out []cacheFile
	for _, e := range entries {
		if e.IsDir() || !olderCacheFile.MatchString(e.Name()) {
			continue
		}
		if fi, err := e.Info(); err == nil {
			out = append(out, cacheFile{filepath.Join(dir, e.Name()), fi.Size(), fi.ModTime().UnixNano()})
		}
	}
	return out
}

/* --- The limit --- */

var (
	cacheLimit  atomic.Int64 // bytes; 0 for none
	cacheWrites atomic.Int64
	trimming    sync.Mutex
)

// trimEvery is how many files are written between two checks of the limit.
const trimEvery = 500

// SetCacheLimit keeps the cache within bytes, 0 for no limit, and checks
// it now.
func SetCacheLimit(bytes int64) {
	cacheLimit.Store(max(0, bytes))
	go trimCache()
}

// CacheLimit is the limit in bytes, 0 for none.
func CacheLimit() int64 { return cacheLimit.Load() }

func noteCacheWrite() {
	if cacheWrites.Add(1)%trimEvery == 0 {
		go trimCache()
	}
}

// trimCache removes the files used least recently until the cache is a
// tenth below its limit, so it is not trimmed again with the next file.
func trimCache() {
	limit := cacheLimit.Load()
	if limit <= 0 || !trimming.TryLock() {
		return
	}
	defer trimming.Unlock()
	dir := getCacheDir()
	files := append(cacheFiles(filepath.Join(dir, cacheThumbnails)), cacheFiles(filepath.Join(dir, cacheImages))...)
	var total int64
	for _, f := range files {
		total += f.size
	}
	if total <= limit {
		return
	}
	sort.Slice(files, func(i, j int) bool { return files[i].mtime < files[j].mtime })
	target := limit - limit/10
	for _, f := range files {
		if total <= target {
			break
		}
		if os.Remove(f.path) == nil {
			total -= f.size
		}
	}
}

// RemoveOlderCache removes what older versions left in the cache: nothing
// reads it, and it was never trimmed.
func RemoveOlderCache() {
	ClearCache(CacheOlder) //nolint:errcheck
}
