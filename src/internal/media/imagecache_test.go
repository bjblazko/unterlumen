package media

import (
	"bytes"
	"fmt"
	"sync"
	"testing"
)

func TestImageCacheGetEmptyReturnsNil(t *testing.T) {
	cache := NewImageCache(10)
	result := cache.Get("missing-key")
	if result != nil {
		t.Fatalf("expected nil, got %v", result)
	}
}

func TestImageCacheSetThenGetReturnsSameBytes(t *testing.T) {
	cache := NewImageCache(10)
	data := []byte("test image data")
	cache.Set("key1", data)

	result := cache.Get("key1")
	if !bytes.Equal(result, data) {
		t.Fatalf("expected %v, got %v", data, result)
	}
}

func TestImageCacheSetDuplicateKeyReplaces(t *testing.T) {
	cache := NewImageCache(10)
	cache.Set("k", []byte("v1"))
	cache.Set("k", []byte("v2"))

	result := cache.Get("k")
	if !bytes.Equal(result, []byte("v2")) {
		t.Fatalf("expected v2, got %v", result)
	}

	if len(cache.entries) != 1 {
		t.Fatalf("expected exactly 1 entry, got %d", len(cache.entries))
	}
}

func TestImageCacheLRUEviction(t *testing.T) {
	cache := NewImageCache(3)

	cache.Set("key1", []byte("data1"))
	cache.Set("key2", []byte("data2"))
	cache.Set("key3", []byte("data3"))

	cache.Set("key4", []byte("data4"))

	if cache.Get("key1") != nil {
		t.Fatalf("key1 should be evicted after adding key4 to full cache")
	}
	if cache.Get("key2") == nil {
		t.Fatalf("key2 should still exist after eviction")
	}
	if cache.Get("key3") == nil {
		t.Fatalf("key3 should still exist after eviction")
	}
	if cache.Get("key4") == nil {
		t.Fatalf("key4 should exist")
	}
}

func TestImageCacheGetPromotesToMRU(t *testing.T) {
	cache := NewImageCache(3)

	cache.Set("key1", []byte("data1"))
	cache.Set("key2", []byte("data2"))
	cache.Set("key3", []byte("data3"))

	cache.Get("key1")

	cache.Set("key4", []byte("data4"))

	if cache.Get("key1") == nil {
		t.Fatalf("key1 should not be evicted because it was promoted to MRU")
	}
	if cache.Get("key2") != nil {
		t.Fatalf("key2 should be evicted (it was LRU)")
	}
}

func TestImageCacheThreadSafety(t *testing.T) {
	cache := NewImageCache(20)
	var wg sync.WaitGroup

	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			for j := 0; j < 10; j++ {
				cache.Set(fmt.Sprintf("key%d", j), []byte("data"))
			}
		}(i)
	}

	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func(id int) {
			defer wg.Done()
			for j := 0; j < 10; j++ {
				cache.Get(fmt.Sprintf("key%d", j))
			}
		}(i)
	}

	wg.Wait()
}
