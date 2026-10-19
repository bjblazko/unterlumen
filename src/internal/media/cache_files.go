package media

import (
	"crypto/sha256"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"
)

// The cache holds what is made from the photos to show them quickly: small
// previews for the grids and full images for the viewer (HEIF converted to
// JPEG). Each kind has a folder of its own, so Settings can say what takes
// the room and clear one kind without the other. Nothing in it is anything
// but a copy: a cleared file is made again when it is needed.

var (
	cacheDir     string
	cacheDirOnce sync.Once
)

// The cache's folders, one per kind.
const (
	cacheThumbnails = "thumbnails"
	cacheImages     = "images"
)

// GetCacheDir returns the persistent cache directory used for thumbnails and conversions.
func GetCacheDir() string { return getCacheDir() }

// SetCacheDir overrides the auto-detected cache location. Must be called before
// any thumbnail or conversion operation (i.e. before the HTTP server starts).
func SetCacheDir(dir string) {
	cacheDirOnce.Do(func() { makeCacheDir(dir) })
}

// getCacheDir returns a persistent cache directory (~/Library/Caches/unterlumen on macOS),
// falling back to the OS temp dir if UserCacheDir is unavailable.
func getCacheDir() string {
	cacheDirOnce.Do(func() {
		if dir, err := os.UserCacheDir(); err == nil {
			makeCacheDir(filepath.Join(dir, "unterlumen"))
		} else {
			makeCacheDir(filepath.Join(os.TempDir(), "unterlumen-cache"))
		}
	})
	return cacheDir
}

func makeCacheDir(dir string) {
	cacheDir = dir
	for _, kind := range []string{cacheThumbnails, cacheImages} {
		os.MkdirAll(filepath.Join(dir, kind), 0o700) //nolint:errcheck
	}
}

// cacheKind is the folder of a purpose: previews for the grids, or images
// for the viewer.
func cacheKind(purpose string) string {
	if strings.HasPrefix(purpose, "thumb") {
		return cacheThumbnails
	}
	return cacheImages
}

// cacheKey returns where in the cache a source file's copy for purpose is.
// The key is path-stable so cached conversions are found even when the source
// volume is offline (e.g. NAS unmounted). Photos in a library are read-only, so
// cache invalidation on modification time is unnecessary.
func cacheKey(path string, purpose string) string {
	h := sha256.Sum256([]byte(path + "|" + purpose))
	return filepath.Join(cacheKind(purpose), fmt.Sprintf("%x.jpg", h[:12]))
}

// readCache returns cached bytes or nil. A file read is marked as used, at
// most once an hour, so the limit takes the files nobody looked at for
// longest (trimCache).
func readCache(key string) []byte {
	p := filepath.Join(getCacheDir(), key)
	data, err := os.ReadFile(p)
	if err != nil {
		return nil
	}
	if fi, err := os.Stat(p); err == nil && time.Since(fi.ModTime()) > time.Hour {
		now := time.Now()
		os.Chtimes(p, now, now) //nolint:errcheck
	}
	return data
}

// writeCache stores bytes in the cache, and now and then keeps it within
// its limit.
func writeCache(key string, data []byte) {
	os.WriteFile(filepath.Join(getCacheDir(), key), data, 0600) //nolint:errcheck
	noteCacheWrite()
}

// EvictFile removes all cached thumbnails and conversions for the given source file.
// Called after in-place edits (e.g. crop) to prevent stale cache entries.
func EvictFile(srcPath string) {
	sizes := []int{50, 100, 150, 200, 300, 400, 600, 800, 1024}
	orientations := []int{0, 1, 2, 3, 4, 5, 6, 7, 8}
	streamIndices := []int{0, 1, 2, 3, 4}

	purposes := []string{"thumb-exif-v2", "full-v3", "full-v4", "full-v5"}
	for _, sz := range sizes {
		purposes = append(purposes,
			fmt.Sprintf("thumb-heif-preview-%s-%d", thumbnailCacheVersion, sz),
			fmt.Sprintf("thumb-heif-source-%s-%d", thumbnailCacheVersion, sz),
			// also evict previous cache versions
			fmt.Sprintf("thumb-heif-preview-v3-%d", sz),
			fmt.Sprintf("thumb-heif-source-v3-%d", sz),
			fmt.Sprintf("thumb-heif-preview-v2-%d", sz),
			fmt.Sprintf("thumb-heif-source-v2-%d", sz),
		)
		for _, ori := range orientations {
			purposes = append(purposes,
				fmt.Sprintf("thumb-source-%s-%d-%d", thumbnailCacheVersion, ori, sz),
			)
		}
		for _, si := range streamIndices {
			purposes = append(purposes,
				fmt.Sprintf("preview-v5-%d-q80", si),
				fmt.Sprintf("preview-v4-%d-q80", si),
			)
		}
	}

	dir := getCacheDir()
	for _, p := range purposes {
		os.Remove(filepath.Join(dir, cacheKey(srcPath, p)))
	}
}

