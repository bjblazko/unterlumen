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

func NewImageCache(maxItems int) *ImageCache {
	return &ImageCache{
		maxItems: maxItems,
		entries:  make([]cacheEntry, 0, maxItems),
	}
}

func (c *ImageCache) Get(key string) []byte {
	c.mu.Lock()
	defer c.mu.Unlock()

	for i, entry := range c.entries {
		if entry.key == key {
			result := entry.data
			// remove element i (order-preserving, no aliasing)
			copy(c.entries[i:], c.entries[i+1:])
			c.entries = c.entries[:len(c.entries)-1]
			c.entries = append(c.entries, entry)
			return result
		}
	}
	return nil
}

func (c *ImageCache) Set(key string, data []byte) {
	c.mu.Lock()
	defer c.mu.Unlock()

	for i, entry := range c.entries {
		if entry.key == key {
			// remove element i (order-preserving, no aliasing)
			copy(c.entries[i:], c.entries[i+1:])
			c.entries = c.entries[:len(c.entries)-1]
			c.entries = append(c.entries, cacheEntry{key, data})
			return
		}
	}

	if len(c.entries) >= c.maxItems {
		// remove element 0 (evict LRU)
		copy(c.entries[0:], c.entries[1:])
		c.entries = c.entries[:len(c.entries)-1]
	}

	c.entries = append(c.entries, cacheEntry{key, data})
}
