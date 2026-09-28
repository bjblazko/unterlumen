package media

import (
	"os"
	"path/filepath"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

// Two full decodes at once pushed the NAS into swap; however many callers
// ask, only one decode runs.
func TestOneFullDecodeRunsOneAtATime(t *testing.T) {
	var running, most atomic.Int32
	var wg sync.WaitGroup
	for i := 0; i < 4; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			oneFullDecode(func() ([]byte, error) { //nolint:errcheck
				n := running.Add(1)
				for {
					m := most.Load()
					if n <= m || most.CompareAndSwap(m, n) {
						break
					}
				}
				time.Sleep(10 * time.Millisecond)
				running.Add(-1)
				return nil, nil
			})
		}()
	}
	wg.Wait()
	if got := most.Load(); got != 1 {
		t.Errorf("at most %d decodes ran at once, want 1", got)
	}
}

func TestHEIFConvertedOnlyOnceTheFullJPEGIsCached(t *testing.T) {
	path := filepath.Join(t.TempDir(), "DSCF0001.HIF")
	if HEIFConverted(path) {
		t.Fatal("a photo never converted counts as converted")
	}
	key := cacheKey(path, fullJPEGPurpose)
	writeCache(key, []byte("jpeg"))
	t.Cleanup(func() { os.Remove(filepath.Join(getCacheDir(), key)) })
	if !HEIFConverted(path) {
		t.Error("a photo with its full JPEG on disk does not count as converted")
	}
}
