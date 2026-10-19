package media

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

// withCacheDir points the cache at dir for one test, never at the real one.
func withCacheDir(t *testing.T, dir string) {
	t.Helper()
	old := getCacheDir()
	makeCacheDir(dir)
	t.Cleanup(func() { cacheDir = old })
}

func TestCacheSegmentsAndClearingOne(t *testing.T) {
	dir := t.TempDir()
	withCacheDir(t, dir)
	writeCache(cacheKey("/a.jpg", "thumb-exif-v2"), make([]byte, 100))
	writeCache(cacheKey("/b.jpg", "thumb-exif-v2"), make([]byte, 100))
	writeCache(cacheKey("/a.heic", fullJPEGPurpose), make([]byte, 1000))
	os.WriteFile(filepath.Join(dir, "0123456789abcdef01234567.jpg"), make([]byte, 10), 0o600) //nolint:errcheck
	os.WriteFile(filepath.Join(dir, "Cache.db"), []byte("macOS"), 0o600)                      //nolint:errcheck

	got := map[string]CacheSegment{}
	for _, s := range CacheSegments() {
		got[s.ID] = s
	}
	if got[cacheThumbnails].Files != 2 || got[cacheThumbnails].Bytes != 200 || got[cacheImages].Bytes != 1000 || got[CacheOlder].Files != 1 {
		t.Fatalf("segments %+v", got)
	}
	if err := ClearCache(CacheOlder); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(dir, "Cache.db")); err != nil {
		t.Error("clearing the cache removed a file that is not ours")
	}
	if err := ClearCache(cacheThumbnails); err != nil {
		t.Fatal(err)
	}
	if readCache(cacheKey("/a.heic", fullJPEGPurpose)) == nil {
		t.Error("clearing the previews removed a viewer image")
	}
	if ClearCache("everything") != ErrNoSuchSegment {
		t.Error("an unknown segment was not refused")
	}
}

// Over its limit, the cache loses the files used least recently.
func TestCacheLimitTakesTheLeastRecentlyUsed(t *testing.T) {
	dir := t.TempDir()
	withCacheDir(t, dir)
	old, recent := cacheKey("/old.jpg", "thumb-exif-v2"), cacheKey("/new.jpg", "thumb-exif-v2")
	writeCache(old, make([]byte, 600))
	writeCache(recent, make([]byte, 600))
	long := time.Now().Add(-48 * time.Hour)
	os.Chtimes(filepath.Join(dir, old), long, long) //nolint:errcheck
	cacheLimit.Store(1000)
	t.Cleanup(func() { cacheLimit.Store(0) })
	trimCache()
	if readCache(old) != nil || readCache(recent) == nil {
		t.Errorf("after trimming: old kept %v, recent kept %v", readCache(old) != nil, readCache(recent) != nil)
	}
}
