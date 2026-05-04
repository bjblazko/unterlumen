package media

import (
	"runtime"
	"sync"
)

type thumbnailWorkResult struct {
	data        []byte
	contentType string
	err         error
}

type thumbnailWorkCall struct {
	done   chan struct{}
	result thumbnailWorkResult
}

type thumbnailWorkCoordinator struct {
	slots chan struct{}

	mu       sync.Mutex
	inflight map[string]*thumbnailWorkCall
}

func newThumbnailWorkCoordinator(limit int) *thumbnailWorkCoordinator {
	if limit < 1 {
		limit = 1
	}
	return &thumbnailWorkCoordinator{
		slots:    make(chan struct{}, limit),
		inflight: make(map[string]*thumbnailWorkCall),
	}
}

func (c *thumbnailWorkCoordinator) run(key string, fn func() thumbnailWorkResult) thumbnailWorkResult {
	c.mu.Lock()
	if call, ok := c.inflight[key]; ok {
		c.mu.Unlock()
		<-call.done
		return call.result
	}

	call := &thumbnailWorkCall{done: make(chan struct{})}
	c.inflight[key] = call
	c.mu.Unlock()

	c.slots <- struct{}{}
	result := fn()
	<-c.slots

	c.mu.Lock()
	call.result = result
	delete(c.inflight, key)
	close(call.done)
	c.mu.Unlock()

	return result
}

func defaultThumbnailWorkLimit() int {
	limit := runtime.GOMAXPROCS(0) / 2
	if limit < 2 {
		return 2
	}
	if limit > 4 {
		return 4
	}
	return limit
}

var thumbnailWork = newThumbnailWorkCoordinator(defaultThumbnailWorkLimit())
