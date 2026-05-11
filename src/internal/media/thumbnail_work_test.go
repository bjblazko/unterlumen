package media

import (
	"context"
	"sync/atomic"
	"testing"
	"time"
)

func TestThumbnailWorkCoordinatorDeduplicatesByKey(t *testing.T) {
	coord := newThumbnailWorkCoordinator(4)
	ctx := context.Background()

	var runs atomic.Int32
	done := make(chan struct{})
	for range 8 {
		go func() {
			result := coord.run(ctx, "same", func() thumbnailWorkResult {
				runs.Add(1)
				time.Sleep(20 * time.Millisecond)
				return thumbnailWorkResult{data: []byte("ok")}
			})
			if string(result.data) != "ok" {
				t.Errorf("result.data = %q, want ok", string(result.data))
			}
			done <- struct{}{}
		}()
	}

	for range 8 {
		<-done
	}

	if got := runs.Load(); got != 1 {
		t.Fatalf("runs = %d, want 1", got)
	}
}

func TestThumbnailWorkCoordinatorLimitsConcurrency(t *testing.T) {
	coord := newThumbnailWorkCoordinator(2)
	ctx := context.Background()

	var active atomic.Int32
	var maxActive atomic.Int32
	done := make(chan struct{})
	for i := range 6 {
		go func(i int) {
			coord.run(ctx, string(rune('a'+i)), func() thumbnailWorkResult {
				current := active.Add(1)
				for {
					previous := maxActive.Load()
					if current <= previous || maxActive.CompareAndSwap(previous, current) {
						break
					}
				}
				time.Sleep(20 * time.Millisecond)
				active.Add(-1)
				return thumbnailWorkResult{data: []byte("ok")}
			})
			done <- struct{}{}
		}(i)
	}

	for range 6 {
		<-done
	}

	if got := maxActive.Load(); got > 2 {
		t.Fatalf("max active = %d, want <= 2", got)
	}
}

func TestThumbnailWorkCoordinatorCancelledWaiter(t *testing.T) {
	coord := newThumbnailWorkCoordinator(1)

	started := make(chan struct{})
	go func() {
		coord.run(context.Background(), "key", func() thumbnailWorkResult {
			close(started)
			time.Sleep(100 * time.Millisecond)
			return thumbnailWorkResult{data: []byte("ok")}
		})
	}()
	<-started

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	result := coord.run(ctx, "other", func() thumbnailWorkResult {
		return thumbnailWorkResult{data: []byte("should not run")}
	})
	if result.err == nil {
		t.Fatal("expected error for cancelled context, got nil")
	}
}
