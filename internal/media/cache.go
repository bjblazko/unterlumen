package media

import (
	"sync"
	"sync/atomic"
	"time"
)

// CachedScan holds a cached directory scan result with optional EXIF date overrides.
type CachedScan struct {
	Entries    []Entry
	DirModTime time.Time
	ExifDates  map[string]time.Time
	exifDone   atomic.Bool
	mu         sync.Mutex // guards ExifDates writes
}

// ExifDone returns whether background EXIF extraction has completed.
func (c *CachedScan) ExifDone() bool {
	return c.exifDone.Load()
}

// SetExifDate stores an EXIF date for a filename (thread-safe).
func (c *CachedScan) SetExifDate(name string, t time.Time) {
	c.mu.Lock()
	c.ExifDates[name] = t
	c.mu.Unlock()
}

// MarkExifDone signals that background EXIF extraction is complete.
func (c *CachedScan) MarkExifDone() {
	c.exifDone.Store(true)
}

// ScanCache is a concurrency-safe in-memory cache for directory scan results.
type ScanCache struct {
	mu    sync.RWMutex
	items map[string]*CachedScan
}

// NewScanCache creates an empty ScanCache.
func NewScanCache() *ScanCache {
	return &ScanCache{
		items: make(map[string]*CachedScan),
	}
}

// Get returns the cached scan for absPath if the directory mod-time matches.
// Returns nil if there is no entry or if the entry is stale.
func (sc *ScanCache) Get(absPath string, dirModTime time.Time) *CachedScan {
	sc.mu.RLock()
	cached, ok := sc.items[absPath]
	sc.mu.RUnlock()
	if !ok {
		return nil
	}
	if !cached.DirModTime.Equal(dirModTime) {
		sc.mu.Lock()
		delete(sc.items, absPath)
		sc.mu.Unlock()
		return nil
	}
	return cached
}

// Put stores a new cache entry and returns it so the caller can mutate it
// (e.g. populate ExifDates from a background goroutine).
func (sc *ScanCache) Put(absPath string, entries []Entry, dirModTime time.Time) *CachedScan {
	cached := &CachedScan{
		Entries:    entries,
		DirModTime: dirModTime,
		ExifDates:  make(map[string]time.Time),
	}
	sc.mu.Lock()
	sc.items[absPath] = cached
	sc.mu.Unlock()
	return cached
}

// Invalidate removes the cache entry for absPath.
func (sc *ScanCache) Invalidate(absPath string) {
	sc.mu.Lock()
	delete(sc.items, absPath)
	sc.mu.Unlock()
}
