package library

import (
	"os"
	"sync"
	"testing"
	"time"
)

// TestDeleteLibraryConcurrentWithOpenStore guards against the getDB/DeleteLibrary
// race: getDB's slow path (open a fresh *sql.DB and cache it) used to run
// unsynchronized with DeleteLibrary's evict-then-RemoveAll, so a request
// landing in that gap could reopen and re-cache a handle onto a library
// whose directory was mid-deletion — a phantom handle that outlives the
// delete. Run with -race to also catch any data race on the shared maps.
func TestDeleteLibraryConcurrentWithOpenStore(t *testing.T) {
	mgr, err := NewManager(t.TempDir())
	if err != nil {
		t.Fatalf("NewManager: %v", err)
	}
	lib, err := mgr.CreateLibrary("t", "", t.TempDir())
	if err != nil {
		t.Fatalf("CreateLibrary: %v", err)
	}

	stop := make(chan struct{})
	var wg sync.WaitGroup
	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for {
				select {
				case <-stop:
					return
				default:
				}
				mgr.OpenStore(lib.ID) //nolint:errcheck
			}
		}()
	}

	time.Sleep(5 * time.Millisecond)
	if err := mgr.DeleteLibrary(lib.ID); err != nil {
		t.Fatalf("DeleteLibrary: %v", err)
	}
	close(stop)
	wg.Wait()

	if _, ok := mgr.openDBs.Load(lib.ID); ok {
		t.Error("openDBs still holds a cached *sql.DB for a deleted library — phantom handle survived DeleteLibrary")
	}
	if _, err := os.Stat(mgr.LibDir(lib.ID)); !os.IsNotExist(err) {
		t.Errorf("expected library directory to be removed, stat err = %v", err)
	}
}
