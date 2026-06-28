package media

import "sync"

type cacheEntry struct {
	key  string
	data []byte
}

// ImageCache is a thread-safe LRU cache for full-size image bytes.
// It is keyed by a string (typically absPath + ":" + mtime.UnixNano()).
// When the cache is full, the least-recently-used entry is evicted.
type ImageCache struct {
	mu       sync.Mutex
	maxItems int
	entries  []cacheEntry
}

// NewImageCache creates an empty ImageCache capped at maxItems entries.
func NewImageCache(maxItems int) *ImageCache {
	return &ImageCache{
		maxItems: maxItems,
		entries:  make([]cacheEntry, 0, maxItems),
	}
}

// Get returns the cached bytes for key, or nil on miss.
// A hit moves the entry to most-recently-used position.
func (c *ImageCache) Get(key string) []byte {
	c.mu.Lock()
	defer c.mu.Unlock()

	for i, entry := range c.entries {
		if entry.key == key {
			result := entry.data
			c.entries = append(c.entries[:i], c.entries[i+1:]...)
			c.entries = append(c.entries, entry)
			return result
		}
	}
	return nil
}

// Set stores data under key.
// If the cache is at capacity, the least-recently-used entry is evicted first.
func (c *ImageCache) Set(key string, data []byte) {
	c.mu.Lock()
	defer c.mu.Unlock()

	for i, entry := range c.entries {
		if entry.key == key {
			c.entries = append(c.entries[:i], c.entries[i+1:]...)
			c.entries = append(c.entries, cacheEntry{key, data})
			return
		}
	}

	if len(c.entries) >= c.maxItems {
		c.entries = c.entries[1:]
	}

	c.entries = append(c.entries, cacheEntry{key, data})
}
