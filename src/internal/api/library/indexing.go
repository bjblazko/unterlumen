package apilibrary

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"

	lib "huepattl.de/unterlumen/internal/library"
)

// --- Indexing (SSE) ---

func reindexLibrary(mgr *lib.Manager) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		subfolder := r.URL.Query().Get("subfolder")
		libraryScan(mgr, "Indexing", func(idx *lib.Indexer, ch chan<- lib.Progress) {
			idx.RunInFolder(context.Background(), ch, subfolder)
		})(w, r)
	}
}

func scanNewLibrary(mgr *lib.Manager) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		subfolder := r.URL.Query().Get("subfolder")
		libraryScan(mgr, "Scanning", func(idx *lib.Indexer, ch chan<- lib.Progress) {
			idx.RunScanNewInFolder(context.Background(), ch, subfolder)
		})(w, r)
	}
}

func cleanupLibrary(mgr *lib.Manager) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		subfolder := r.URL.Query().Get("subfolder")
		libraryScan(mgr, "Checking", func(idx *lib.Indexer, ch chan<- lib.Progress) {
			idx.RunCleanupInFolder(context.Background(), ch, subfolder)
		})(w, r)
	}
}

func regenMissingPreviewsLibrary(mgr *lib.Manager) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		subfolder := r.URL.Query().Get("subfolder")
		libraryScan(mgr, "Generating previews for", func(idx *lib.Indexer, ch chan<- lib.Progress) {
			idx.RunRegenerateMissingPreviewsInFolder(context.Background(), ch, subfolder)
		})(w, r)
	}
}

func rebuildAllPreviewsLibrary(mgr *lib.Manager) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		subfolder := r.URL.Query().Get("subfolder")
		libraryScan(mgr, "Rebuilding previews for", func(idx *lib.Indexer, ch chan<- lib.Progress) {
			idx.RunRebuildAllPreviewsInFolder(context.Background(), ch, subfolder)
		})(w, r)
	}
}

// libraryScan returns a handler that starts a scan or joins an in-progress one.
// If the library is already being scanned the caller connects to the live progress
// stream instead of receiving a 409. Scans run on context.Background() so they
// continue even when the originating HTTP connection closes.
func libraryScan(mgr *lib.Manager, verb string, scan func(*lib.Indexer, chan<- lib.Progress)) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id := r.PathValue("id")

		flusher, ok := w.(http.Flusher)
		if !ok {
			http.Error(w, "streaming not supported", http.StatusInternalServerError)
			return
		}

		libInfo, err := mgr.GetLibrary(id)
		if err != nil {
			http.Error(w, "library not found", http.StatusNotFound)
			return
		}

		var viewerCh <-chan lib.Progress

		b, started := mgr.StartScan(id, verb)
		if started {
			store, err := mgr.OpenStore(id)
			if err != nil {
				mgr.EndScan(id)
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}
			// Subscribe before starting goroutines to avoid missing early events.
			viewerCh = b.Subscribe()
			rawCh := make(chan lib.Progress, 8)
			go func() {
				defer b.Close() // safety net for interrupted scans
				for p := range rawCh {
					b.Send(p)
				}
			}()
			go func() {
				defer store.Close()
				defer mgr.EndScan(id)
				indexer := lib.NewIndexer(store, mgr.LibDir(id), libInfo.SourcePath)
				scan(indexer, rawCh)
			}()
		} else {
			existing, ok := mgr.JoinScan(id)
			if !ok {
				// Scan ended between TryLockIndex and here — extremely rare race.
				http.Error(w, "indexing already in progress", http.StatusConflict)
				return
			}
			viewerCh = existing.Subscribe()
		}

		w.Header().Set("Content-Type", "text/event-stream")
		w.Header().Set("Cache-Control", "no-cache")
		w.Header().Set("X-Accel-Buffering", "no")
		w.WriteHeader(http.StatusOK)

		enc := json.NewEncoder(w)
		for {
			select {
			case p, ok := <-viewerCh:
				if !ok {
					return
				}
				fmt.Fprintf(w, "data: ")
				enc.Encode(p) //nolint:errcheck
				fmt.Fprintf(w, "\n")
				flusher.Flush()
			case <-r.Context().Done():
				return
			}
		}
	}
}
