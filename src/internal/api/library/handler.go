// Package apilibrary provides HTTP handlers for the DAM library feature.
package apilibrary

import (
	"archive/zip"
	"bytes"
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"image"
	_ "image/jpeg"
	_ "image/png"
	"io"
	"math"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	_ "golang.org/x/image/webp"

	"huepattl.de/unterlumen/internal/channels"
	lib "huepattl.de/unterlumen/internal/library"
	"huepattl.de/unterlumen/internal/media"
	"huepattl.de/unterlumen/internal/pathguard"
)

// Handle registers all library API routes on mux.
// root is the browse boundary directory; serverRole is true when running in server/container mode.
func Handle(mux *http.ServeMux, mgr *lib.Manager, imgCache *media.ImageCache, root string, serverRole bool, chStore *channels.Store, draftStore *channels.DraftStore) {
	mux.HandleFunc("GET /api/library/", listLibraries(mgr, root))
	mux.HandleFunc("POST /api/library/", createLibrary(mgr, root))
	mux.HandleFunc("PUT /api/library-order", setLibraryOrder(mgr))
	mux.HandleFunc("GET /api/settings", getSettings(mgr))
	mux.HandleFunc("PATCH /api/settings", patchSettings(mgr))
	mux.HandleFunc("GET /api/library/detect", detectLibrary(mgr, root))
	mux.HandleFunc("GET /api/library/search", searchLibraries(mgr))
	mux.HandleFunc("GET /api/library/exif-ranges", globalExifRanges(mgr))
	mux.HandleFunc("GET /api/library/exif-values", globalExifValues(mgr))
	mux.HandleFunc("GET /api/library/meta-keys", globalMetaKeys(mgr))
	mux.HandleFunc("GET /api/library/meta-values", globalMetaValues(mgr))
	mux.HandleFunc("GET /api/library/album-titles", globalAlbumTitles(mgr))
	mux.HandleFunc("GET /api/library/exif-fields", globalExifFields(mgr))
	mux.HandleFunc("GET /api/library/statistics", libraryStatistics(mgr))
	mux.HandleFunc("GET /api/library/timeline", libraryTimeline(mgr))
	mux.HandleFunc("GET /api/library/{id}", getLibrary(mgr, root))
	mux.HandleFunc("PATCH /api/library/{id}", updateLibrary(mgr, root))
	mux.HandleFunc("DELETE /api/library/{id}", deleteLibrary(mgr))
	mux.HandleFunc("POST /api/library/{id}/reindex", reindexLibrary(mgr))
	mux.HandleFunc("POST /api/library/{id}/scan-new", scanNewLibrary(mgr))
	mux.HandleFunc("POST /api/library/{id}/cleanup", cleanupLibrary(mgr))
	mux.HandleFunc("POST /api/library/{id}/regen-previews-missing", regenMissingPreviewsLibrary(mgr))
	mux.HandleFunc("POST /api/library/{id}/regen-previews-all", rebuildAllPreviewsLibrary(mgr))
	mux.HandleFunc("GET /api/library/{id}/browse", browseFolder(mgr, root))
	mux.HandleFunc("GET /api/library/{id}/browse-recursive", browseFolderRecursive(mgr))
	mux.HandleFunc("GET /api/library/{id}/folder-stats", libraryFolderStats(mgr))
	mux.HandleFunc("GET /api/library/{id}/photos", listPhotos(mgr))
	mux.HandleFunc("GET /api/library/{id}/exif-ranges", exifRanges(mgr))
	mux.HandleFunc("GET /api/library/{id}/thumb/{photoID}", serveThumb(mgr))
	mux.HandleFunc("GET /api/library/{id}/thumb-by-path", thumbByPath(mgr, root))
	mux.HandleFunc("GET /api/library/{id}/photo-id-by-path", photoIDByPath(mgr))
	mux.HandleFunc("GET /api/library/{id}/photo/{photoID}", servePhoto(mgr, imgCache))
	mux.HandleFunc("GET /api/library/{id}/photo/{photoID}/info", photoInfo(mgr))
	mux.HandleFunc("DELETE /api/library/{id}/photo/{photoID}", deleteLibraryPhoto(mgr))
	mux.HandleFunc("GET /api/library/{id}/photo/{photoID}/meta", getMeta(mgr))
	mux.HandleFunc("PUT /api/library/{id}/photo/{photoID}/meta", upsertMeta(mgr))
	mux.HandleFunc("DELETE /api/library/{id}/photo/{photoID}/meta", deleteMeta(mgr, chStore, draftStore))
	mux.HandleFunc("POST /api/channels/{slug}/drafts/{draftID}/generate", generateDraft(mgr, chStore, draftStore))
	mux.HandleFunc("POST /api/library/{id}/build-download", buildDownload(mgr, chStore))
	mux.HandleFunc("POST /api/channels/{slug}/rebuild-site", rebuildSite(chStore, mgr))
	mux.HandleFunc("POST /api/channels/{slug}/rebuild-album-list", rebuildAlbumList(chStore, mgr))
	mux.HandleFunc("POST /api/channels/{slug}/rebuild-galleries", rebuildGalleries(chStore))
	mux.HandleFunc("GET /api/channels/{slug}/galleries", listGalleries(chStore))
	mux.HandleFunc("PATCH /api/channels/{slug}/galleries/{postID}", renameGallery(chStore, mgr))
	mux.HandleFunc("DELETE /api/channels/{slug}/galleries/{postID}", deleteGallery(chStore, mgr))
	mux.HandleFunc("GET /api/channels/galleries", listAllGalleries(chStore, draftStore))
	mux.HandleFunc("POST /api/channels/galleries/reachability", checkGalleryReachability())
	registerDraftRoutes(mux, mgr, chStore, draftStore)
}

func writeJSON(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(v)
}

// --- Library CRUD ---

// libraryJSON wraps Library with a computed relSourcePath for the browse API.
type libraryJSON struct {
	*lib.Library
	RelSourcePath string `json:"relSourcePath"`
	Scanning      bool   `json:"scanning"`
}

func toLibraryJSON(l *lib.Library, root string, scanning bool) libraryJSON {
	var rel string
	if root == "/" {
		rel = strings.TrimPrefix(l.SourcePath, "/")
	} else {
		rel = strings.TrimPrefix(l.SourcePath, root+"/")
		if rel == l.SourcePath {
			rel = "" // not under root
		}
	}
	return libraryJSON{Library: l, RelSourcePath: rel, Scanning: scanning}
}

func listLibraries(mgr *lib.Manager, root string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		libs, err := mgr.ListLibraries()
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		out := make([]libraryJSON, len(libs))
		for i, l := range libs {
			out[i] = toLibraryJSON(l, root, mgr.IsScanning(l.ID))
		}
		writeJSON(w, out)
	}
}

// detectLibrary returns the library (id + name) whose source path covers the
// requested path, or an empty object when no library matches.
func detectLibrary(mgr *lib.Manager, root string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		relPath := r.URL.Query().Get("path")
		absPath, ok := pathguard.SafePath(root, relPath)
		if !ok {
			writeJSON(w, struct{}{})
			return
		}
		l, ok := mgr.FindLibraryForPath(absPath)
		if !ok {
			writeJSON(w, struct{}{})
			return
		}
		writeJSON(w, struct {
			ID   string `json:"id"`
			Name string `json:"name"`
		}{ID: l.ID, Name: l.Name})
	}
}

func createLibrary(mgr *lib.Manager, root string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var body struct {
			Name        string `json:"name"`
			Description string `json:"description"`
			SourcePath  string `json:"sourcePath"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			http.Error(w, "invalid JSON", http.StatusBadRequest)
			return
		}
		if body.Name == "" || body.SourcePath == "" {
			http.Error(w, "name and sourcePath are required", http.StatusBadRequest)
			return
		}
		rel := strings.TrimPrefix(body.SourcePath, "/")
		absPath, ok := pathguard.SafePath(root, rel)
		if !ok {
			http.Error(w, "invalid sourcePath", http.StatusBadRequest)
			return
		}
		if info, err := os.Stat(absPath); err != nil || !info.IsDir() {
			http.Error(w, "sourcePath must be an existing directory", http.StatusBadRequest)
			return
		}
		created, err := mgr.CreateLibrary(body.Name, body.Description, absPath)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		w.WriteHeader(http.StatusCreated)
		writeJSON(w, toLibraryJSON(created, root, false))
	}
}

func getLibrary(mgr *lib.Manager, root string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id := r.PathValue("id")
		l, err := mgr.GetLibrary(id)
		if err != nil {
			http.Error(w, err.Error(), http.StatusNotFound)
			return
		}
		writeJSON(w, toLibraryJSON(l, root, mgr.IsScanning(id)))
	}
}

func deleteLibrary(mgr *lib.Manager) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id := r.PathValue("id")
		if err := mgr.DeleteLibrary(id); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	}
}

func updateLibrary(mgr *lib.Manager, root string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id := r.PathValue("id")
		var body struct {
			Name        string `json:"name"`
			Description string `json:"description"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			http.Error(w, "invalid JSON", http.StatusBadRequest)
			return
		}
		if body.Name == "" {
			http.Error(w, "name is required", http.StatusBadRequest)
			return
		}
		updated, err := mgr.UpdateLibrary(id, body.Name, body.Description)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		writeJSON(w, toLibraryJSON(updated, root, mgr.IsScanning(id)))
	}
}

func setLibraryOrder(mgr *lib.Manager) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var body struct {
			Order []string `json:"order"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			http.Error(w, "invalid JSON", http.StatusBadRequest)
			return
		}
		if err := mgr.SetLibrarySortOrder(body.Order); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	}
}

func getSettings(mgr *lib.Manager) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		s, err := mgr.GetSettings()
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		writeJSON(w, s)
	}
}

func patchSettings(mgr *lib.Manager) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		current, err := mgr.GetSettings()
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		var patch struct {
			LibrarySortMode *string `json:"librarySortMode"`
		}
		if err := json.NewDecoder(r.Body).Decode(&patch); err != nil {
			http.Error(w, "invalid JSON", http.StatusBadRequest)
			return
		}
		if patch.LibrarySortMode != nil {
			current.LibrarySortMode = *patch.LibrarySortMode
		}
		if err := mgr.SaveSettings(current); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		writeJSON(w, current)
	}
}

// --- Indexing (SSE) ---

func reindexLibrary(mgr *lib.Manager) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		subfolder := r.URL.Query().Get("subfolder")
		libraryScan(mgr, func(idx *lib.Indexer, ch chan<- lib.Progress) {
			idx.RunInFolder(context.Background(), ch, subfolder)
		})(w, r)
	}
}

func scanNewLibrary(mgr *lib.Manager) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		subfolder := r.URL.Query().Get("subfolder")
		libraryScan(mgr, func(idx *lib.Indexer, ch chan<- lib.Progress) {
			idx.RunScanNewInFolder(context.Background(), ch, subfolder)
		})(w, r)
	}
}

func cleanupLibrary(mgr *lib.Manager) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		subfolder := r.URL.Query().Get("subfolder")
		libraryScan(mgr, func(idx *lib.Indexer, ch chan<- lib.Progress) {
			idx.RunCleanupInFolder(context.Background(), ch, subfolder)
		})(w, r)
	}
}

func regenMissingPreviewsLibrary(mgr *lib.Manager) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		subfolder := r.URL.Query().Get("subfolder")
		libraryScan(mgr, func(idx *lib.Indexer, ch chan<- lib.Progress) {
			idx.RunRegenerateMissingPreviewsInFolder(context.Background(), ch, subfolder)
		})(w, r)
	}
}

func rebuildAllPreviewsLibrary(mgr *lib.Manager) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		subfolder := r.URL.Query().Get("subfolder")
		libraryScan(mgr, func(idx *lib.Indexer, ch chan<- lib.Progress) {
			idx.RunRebuildAllPreviewsInFolder(context.Background(), ch, subfolder)
		})(w, r)
	}
}

// libraryScan returns a handler that starts a scan or joins an in-progress one.
// If the library is already being scanned the caller connects to the live progress
// stream instead of receiving a 409. Scans run on context.Background() so they
// continue even when the originating HTTP connection closes.
func libraryScan(mgr *lib.Manager, scan func(*lib.Indexer, chan<- lib.Progress)) http.HandlerFunc {
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

		b, started := mgr.StartScan(id)
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

// --- Photos ---

func browseFolder(mgr *lib.Manager, _ string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id := r.PathValue("id")
		store, err := mgr.OpenStore(id)
		if err != nil {
			http.Error(w, "library not found", http.StatusNotFound)
			return
		}
		defer store.Close()

		sourcePath, ok, _ := store.GetProp("source_path")
		if !ok || sourcePath == "" {
			http.Error(w, "library has no source path", http.StatusInternalServerError)
			return
		}

		relPath := r.URL.Query().Get("path")
		// SafePathLogical: BrowseFolder uses the path only as a DB string pattern,
		// so the source volume need not be mounted (e.g. NAS offline).
		absPath, ok := pathguard.SafePathLogical(sourcePath, relPath)
		if !ok {
			http.Error(w, "invalid path", http.StatusBadRequest)
			return
		}

		result, err := store.BrowseFolder(absPath)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		writeJSON(w, result)
	}
}

func browseFolderRecursive(mgr *lib.Manager) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id := r.PathValue("id")
		store, err := mgr.OpenStore(id)
		if err != nil {
			http.Error(w, "library not found", http.StatusNotFound)
			return
		}
		defer store.Close()

		sourcePath, ok, _ := store.GetProp("source_path")
		if !ok || sourcePath == "" {
			http.Error(w, "library has no source path", http.StatusInternalServerError)
			return
		}

		relPath := r.URL.Query().Get("path")
		absPath, ok := pathguard.SafePathLogical(sourcePath, relPath)
		if !ok {
			http.Error(w, "invalid path", http.StatusBadRequest)
			return
		}

		photos, err := store.BrowseFolderRecursive(absPath)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		type entry struct {
			ID      string `json:"id"`
			RelPath string `json:"relPath"`
		}
		entries := make([]entry, 0, len(photos))
		for _, p := range photos {
			rel, relErr := filepath.Rel(sourcePath, p.PathHint)
			if relErr != nil {
				continue
			}
			entries = append(entries, entry{ID: p.ID, RelPath: filepath.ToSlash(rel)})
		}
		writeJSON(w, map[string]interface{}{"photos": entries})
	}
}

func libraryFolderStats(mgr *lib.Manager) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id := r.PathValue("id")
		relPath := r.URL.Query().Get("path")
		stats, err := mgr.FolderStats(id, relPath)
		if err != nil {
			if strings.Contains(err.Error(), "not found") {
				http.Error(w, err.Error(), http.StatusNotFound)
			} else {
				http.Error(w, err.Error(), http.StatusInternalServerError)
			}
			return
		}
		writeJSON(w, stats)
	}
}

func listPhotos(mgr *lib.Manager) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id := r.PathValue("id")
		store, err := mgr.OpenStore(id)
		if err != nil {
			http.Error(w, "library not found", http.StatusNotFound)
			return
		}
		defer store.Close()

		result, err := store.ListPhotos(parseListPhotosOpts(r.URL.Query()))
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		writeJSON(w, result)
	}
}

func searchLibraries(mgr *lib.Manager) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		q := r.URL.Query()
		result, err := mgr.SearchLibraries(parseIDList(q.Get("ids")), parseListPhotosOpts(q))
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		writeJSON(w, result)
	}
}

// parseListPhotosOpts reads the photo filter shared by a single library's
// photo list and the search across libraries: EXIF text, numeric and date
// ranges, meta, album, format, destination and paging.
func parseListPhotosOpts(q url.Values) lib.ListPhotosOpts {
	opts := lib.ListPhotosOpts{
		Filters:        parseTextFilters(q),
		NumericFilters: parseNumericFilters(q),
		DateMin:        q.Get("date_taken_min"),
		DateMax:        q.Get("date_taken_max"),
		MetaFilters:    parseMetaFilters(q),
		AlbumTitle:     q.Get("album_title"),
		ExtFilter:      q.Get("ext"),
	}
	if ch := q.Get("channel"); ch != "" {
		opts.MetaExists = []string{"built:" + ch}
	}
	opts.Offset, _ = strconv.Atoi(q.Get("offset"))
	opts.Limit, _ = strconv.Atoi(q.Get("limit"))
	if opts.Limit <= 0 || opts.Limit > 500 {
		opts.Limit = 100
	}
	return opts
}

func globalExifValues(mgr *lib.Manager) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		field := r.URL.Query().Get("field")
		if field == "" {
			http.Error(w, "field required", http.StatusBadRequest)
			return
		}
		ids := parseIDList(r.URL.Query().Get("ids"))
		vals, err := mgr.AggregateExifFieldValues(ids, field)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		if vals == nil {
			vals = []string{}
		}
		writeJSON(w, vals)
	}
}

func globalMetaKeys(mgr *lib.Manager) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ids := parseIDList(r.URL.Query().Get("ids"))
		keys, err := mgr.AggregateMetaKeys(ids)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		keys = normalizeBuiltMetaKeyNames(keys)
		if keys == nil {
			keys = []string{}
		}
		writeJSON(w, keys)
	}
}

// normalizeBuiltMetaKeyNames applies the same "published:" -> "built:" normalization
// as normalizeBuiltMetaKeys, but for a plain list of key names (as returned by
// AggregateMetaKeys) rather than full MetaEntry values. The result is re-sorted since
// renaming a key can change its sort position.
func normalizeBuiltMetaKeyNames(keys []string) []string {
	existing := make(map[string]bool, len(keys))
	for _, k := range keys {
		if strings.HasPrefix(k, "built:") {
			existing[k] = true
		}
	}
	seen := make(map[string]bool, len(keys))
	out := make([]string, 0, len(keys))
	for _, k := range keys {
		nk := k
		if strings.HasPrefix(k, "published:") {
			builtKey := "built:" + strings.TrimPrefix(k, "published:")
			if existing[builtKey] {
				continue // superseded by a current built: entry
			}
			nk = builtKey
		}
		if !seen[nk] {
			seen[nk] = true
			out = append(out, nk)
		}
	}
	sort.Strings(out)
	return out
}

func globalMetaValues(mgr *lib.Manager) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		key := r.URL.Query().Get("key")
		if key == "" {
			http.Error(w, "key required", http.StatusBadRequest)
			return
		}
		ids := parseIDList(r.URL.Query().Get("ids"))
		vals, err := mgr.AggregateMetaValues(ids, key)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		if vals == nil {
			vals = []string{}
		}
		writeJSON(w, vals)
	}
}

func globalAlbumTitles(mgr *lib.Manager) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ids := parseIDList(r.URL.Query().Get("ids"))
		titles, err := mgr.AggregateAlbumTitles(ids)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		if titles == nil {
			titles = []string{}
		}
		writeJSON(w, titles)
	}
}

func globalExifFields(mgr *lib.Manager) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ids := parseIDList(r.URL.Query().Get("ids"))
		fields, err := mgr.AggregateExifFields(ids)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		if fields == nil {
			fields = []string{}
		}
		writeJSON(w, fields)
	}
}

func globalExifRanges(mgr *lib.Manager) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ids := parseIDList(r.URL.Query().Get("ids"))
		ranges, err := mgr.AggregateExifRanges(ids)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		writeJSON(w, ranges)
	}
}

func libraryStatistics(mgr *lib.Manager) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ids := parseIDList(r.URL.Query().Get("ids"))
		pathPrefix := r.URL.Query().Get("pathPrefix")
		stats, err := mgr.Statistics(ids, pathPrefix)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		writeJSON(w, stats)
	}
}

func libraryTimeline(mgr *lib.Manager) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ids := parseIDList(r.URL.Query().Get("ids"))
		pathPrefix := r.URL.Query().Get("pathPrefix")
		granularity := r.URL.Query().Get("granularity")
		if granularity != "month" && granularity != "year" {
			granularity = ""
		}
		tl, err := mgr.Timeline(ids, pathPrefix, granularity)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		writeJSON(w, tl)
	}
}

func parseIDList(s string) []string {
	if s == "" {
		return nil
	}
	var ids []string
	for _, id := range strings.Split(s, ",") {
		if id = strings.TrimSpace(id); id != "" {
			ids = append(ids, id)
		}
	}
	return ids
}

// parseTextFilters extracts non-reserved query params as EXIF text filters.
// Reserved params: q, offset, limit, ids, date_taken_min/max, _min/_max numeric params,
// and the meta/channel/album/ext params handled separately.
func parseTextFilters(vals map[string][]string) map[string]string {
	out := make(map[string]string)
	for k, vs := range vals {
		if k == "q" || k == "offset" || k == "limit" || k == "ids" || len(vs) == 0 {
			continue
		}
		if strings.HasSuffix(k, "_min") || strings.HasSuffix(k, "_max") {
			continue
		}
		if k == "channel" || k == "album_title" || k == "ext" || k == "date_taken_min" || k == "date_taken_max" {
			continue
		}
		if strings.HasPrefix(k, "meta_") {
			continue
		}
		if vs[0] != "" {
			out[k] = vs[0]
		}
	}
	return out
}

// parseMetaFilters extracts meta_<key>=<value> params as photo_meta key→value filters.
func parseMetaFilters(vals map[string][]string) map[string]string {
	out := make(map[string]string)
	for k, vs := range vals {
		key, found := strings.CutPrefix(k, "meta_")
		if !found || len(vs) == 0 || vs[0] == "" {
			continue
		}
		out[key] = vs[0]
	}
	return out
}

func parseNumericFilters(vals map[string][]string) map[string]lib.NumericFilter {
	type bounds struct{ min, max *float64 }
	bmap := make(map[string]*bounds)
	ensure := func(field string) *bounds {
		if bmap[field] == nil {
			bmap[field] = &bounds{}
		}
		return bmap[field]
	}
	for k, vs := range vals {
		if len(vs) == 0 {
			continue
		}
		if field, suffix, ok := strings.Cut(k, "_min"); ok && suffix == "" {
			if v, err := strconv.ParseFloat(vs[0], 64); err == nil {
				ensure(field).min = &v
			}
			continue
		}
		if field, suffix, ok := strings.Cut(k, "_max"); ok && suffix == "" {
			if v, err := strconv.ParseFloat(vs[0], 64); err == nil {
				ensure(field).max = &v
			}
		}
	}
	out := make(map[string]lib.NumericFilter)
	for field, b := range bmap {
		f := lib.NumericFilter{Min: 0, Max: math.MaxFloat64}
		if b.min != nil {
			f.Min = *b.min
		}
		if b.max != nil {
			f.Max = *b.max
		}
		out[field] = f
	}
	return out
}

func exifRanges(mgr *lib.Manager) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id := r.PathValue("id")
		if _, err := mgr.OpenStore(id); err != nil {
			http.Error(w, "library not found", http.StatusNotFound)
			return
		}
		ranges, err := mgr.AggregateExifRanges([]string{id})
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		writeJSON(w, ranges)
	}
}

// --- Thumbnail & photo serving ---

func thumbByPath(mgr *lib.Manager, root string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id := r.PathValue("id")
		relPath := r.URL.Query().Get("path")
		if relPath == "" {
			http.Error(w, "path required", http.StatusBadRequest)
			return
		}
		absPath := filepath.Join(root, relPath)

		store, err := mgr.OpenStore(id)
		if err != nil {
			http.Error(w, "library not found", http.StatusNotFound)
			return
		}
		defer store.Close()

		photoID, err := store.GetPhotoIDByAbsPath(absPath)
		if err != nil || photoID == "" {
			http.Error(w, "thumbnail not found", http.StatusNotFound)
			return
		}

		thumbRel, err := store.GetPhotoThumbPath(photoID)
		if err != nil || thumbRel == "" {
			http.Error(w, "thumbnail not found", http.StatusNotFound)
			return
		}

		absThumb := filepath.Join(mgr.LibDir(id), thumbRel)
		data, err := os.ReadFile(absThumb)
		if err != nil {
			http.Error(w, "thumbnail not found", http.StatusNotFound)
			return
		}

		w.Header().Set("Content-Type", "image/jpeg")
		w.Header().Set("Cache-Control", "max-age=86400")
		w.Write(data)
	}
}

func photoIDByPath(mgr *lib.Manager) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id := r.PathValue("id")
		relPath := r.URL.Query().Get("path")
		if relPath == "" {
			http.Error(w, "path required", http.StatusBadRequest)
			return
		}

		store, err := mgr.OpenStore(id)
		if err != nil {
			http.Error(w, "library not found", http.StatusNotFound)
			return
		}
		defer store.Close()

		sourcePath, ok, _ := store.GetProp("source_path")
		if !ok || sourcePath == "" {
			http.Error(w, "library has no source path", http.StatusInternalServerError)
			return
		}

		absPath := filepath.Join(sourcePath, relPath)
		photoID, err := store.GetPhotoIDByPathHint(absPath)
		if err != nil || photoID == "" {
			http.Error(w, "not found", http.StatusNotFound)
			return
		}

		writeJSON(w, map[string]string{"photoID": photoID})
	}
}

func serveThumb(mgr *lib.Manager) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id := r.PathValue("id")
		photoID := r.PathValue("photoID")

		store, err := mgr.OpenStore(id)
		if err != nil {
			http.Error(w, "library not found", http.StatusNotFound)
			return
		}
		defer store.Close()

		thumbRel, err := store.GetPhotoThumbPath(photoID)
		if err != nil || thumbRel == "" {
			http.Error(w, "thumbnail not found", http.StatusNotFound)
			return
		}

		absThumb := filepath.Join(mgr.LibDir(id), thumbRel)
		data, err := os.ReadFile(absThumb)
		if err != nil {
			http.Error(w, "thumbnail not found", http.StatusNotFound)
			return
		}

		w.Header().Set("Content-Type", "image/jpeg")
		w.Header().Set("Cache-Control", "max-age=86400")
		w.Write(data)
	}
}

func deleteLibraryPhoto(mgr *lib.Manager) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id := r.PathValue("id")
		photoID := r.PathValue("photoID")

		store, err := mgr.OpenStore(id)
		if err != nil {
			http.Error(w, "library not found", http.StatusNotFound)
			return
		}
		defer store.Close()

		pathHint, err := store.GetPhotoPathHint(photoID)
		if err != nil || pathHint == "" {
			http.Error(w, "photo not found", http.StatusNotFound)
			return
		}

		// Remove the real file before touching the database: if path_hint is stale
		// (points somewhere the file no longer is) this must surface as a failure,
		// not silently drop the library record while the actual photo survives
		// untouched and untracked on disk.
		if err := os.Remove(pathHint); err != nil {
			writeJSON(w, map[string]any{"file": pathHint, "success": false, "error": err.Error()})
			return
		}
		os.Remove(media.SidecarPath(pathHint)) //nolint:errcheck

		_, thumbPath, err := store.DeletePhotoByID(photoID)
		if err != nil {
			http.Error(w, "photo not found: "+err.Error(), http.StatusNotFound)
			return
		}
		if thumbPath != "" {
			os.Remove(filepath.Join(mgr.LibDir(id), thumbPath)) //nolint:errcheck
		}
		writeJSON(w, map[string]any{"file": pathHint, "success": true})
	}
}

func servePhoto(mgr *lib.Manager, imgCache *media.ImageCache) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id := r.PathValue("id")
		photoID := r.PathValue("photoID")

		store, err := mgr.OpenStore(id)
		if err != nil {
			http.Error(w, "library not found", http.StatusNotFound)
			return
		}
		defer store.Close()

		pathHint, err := store.GetPhotoPathHint(photoID)
		if err != nil || pathHint == "" {
			http.Error(w, "photo not found", http.StatusNotFound)
			return
		}

		if media.IsHEIF(pathHint) {
			var key string
			var info os.FileInfo
			info, err = os.Stat(pathHint)
			if err == nil {
				key = pathHint + ":" + strconv.FormatInt(info.ModTime().UnixNano(), 10)
			}

			serveHEIF := func(data []byte) {
				h := sha256.Sum256([]byte(pathHint))
				etag := fmt.Sprintf(`"%x-%d"`, h[:4], info.ModTime().Unix())
				w.Header().Set("Cache-Control", "private, max-age=3600")
				w.Header().Set("ETag", etag)
				if r.Header.Get("If-None-Match") == etag {
					w.WriteHeader(http.StatusNotModified)
					return
				}
				w.Header().Set("Content-Type", "image/jpeg")
				http.ServeContent(w, r, "image.jpg", info.ModTime(), bytes.NewReader(data))
			}

			if key != "" {
				if cached := imgCache.Get(key); cached != nil {
					serveHEIF(cached)
					return
				}
			}

			jpegData, convErr := media.ConvertHEIFToJPEG(r.Context(), pathHint)
			if convErr != nil {
				if _, statErr := os.Stat(pathHint); os.IsNotExist(statErr) {
					http.Error(w, "photo not found on disk", http.StatusNotFound)
				} else {
					http.Error(w, "Failed to convert HEIF: "+convErr.Error(), http.StatusInternalServerError)
				}
				return
			}

			if key != "" {
				imgCache.Set(key, jpegData)
				serveHEIF(jpegData)
				return
			}

			w.Header().Set("Content-Type", "image/jpeg")
			w.Header().Set("Cache-Control", "no-cache")
			http.ServeContent(w, r, "image.jpg", time.Time{}, bytes.NewReader(jpegData))
			return
		}

		http.ServeFile(w, r, pathHint)
	}
}

// photoInfoResp mirrors the browse /api/info response shape so the frontend
// InfoPanel can consume it without modification.
type photoInfoResp struct {
	Name     string        `json:"name"`
	Path     string        `json:"path"`
	Size     int64         `json:"size"`
	Format   string        `json:"format"`
	Modified string        `json:"modified"`
	Exif     *photoExifOut `json:"exif,omitempty"`
}

type photoExifOut struct {
	Tags          map[string]string `json:"tags,omitempty"`
	Width         int               `json:"width,omitempty"`
	Height        int               `json:"height,omitempty"`
	Latitude      *float64          `json:"latitude,omitempty"`
	Longitude     *float64          `json:"longitude,omitempty"`
	DateTaken     *string           `json:"dateTaken,omitempty"`
	DateDigitized *string           `json:"dateDigitized,omitempty"`
	DateModified  *string           `json:"dateModified,omitempty"`
}

func photoInfo(mgr *lib.Manager) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id := r.PathValue("id")
		photoID := r.PathValue("photoID")

		store, err := mgr.OpenStore(id)
		if err != nil {
			http.Error(w, "library not found", http.StatusNotFound)
			return
		}
		defer store.Close()

		p, err := store.GetPhotoInfo(photoID)
		if err != nil || p == nil {
			http.Error(w, "photo not found", http.StatusNotFound)
			return
		}

		writeJSON(w, buildPhotoInfoResp(p))
	}
}

// photoExifStored mirrors media.ExifData for unmarshaling the stored exif_json blob.
type photoExifStored struct {
	Tags          map[string]string `json:"tags"`
	Width         int               `json:"width"`
	Height        int               `json:"height"`
	Latitude      *float64          `json:"latitude"`
	Longitude     *float64          `json:"longitude"`
	DateTaken     *string           `json:"dateTaken"`
	DateDigitized *string           `json:"dateDigitized"`
	DateModified  *string           `json:"dateModified"`
}

func buildPhotoInfoResp(p *lib.PhotoInfo) photoInfoResp {
	ext := strings.ToLower(strings.TrimPrefix(filepath.Ext(p.Filename), "."))
	resp := photoInfoResp{
		Name:     p.Filename,
		Path:     p.PathHint,
		Size:     p.FileSize,
		Format:   ext,
		Modified: p.IndexedAt,
	}
	if p.ExifJSON == "" {
		return resp
	}

	var stored photoExifStored
	if err := json.Unmarshal([]byte(p.ExifJSON), &stored); err != nil {
		return resp
	}

	out := &photoExifOut{
		Tags:          stored.Tags,
		Width:         stored.Width,
		Height:        stored.Height,
		DateTaken:     stored.DateTaken,
		DateDigitized: stored.DateDigitized,
		DateModified:  stored.DateModified,
	}

	// Use pre-parsed GPS coordinates when available; fall back to tag parsing.
	if stored.Latitude != nil && stored.Longitude != nil {
		out.Latitude = stored.Latitude
		out.Longitude = stored.Longitude
	} else if stored.Tags != nil {
		if lat, ok := parseGPSCoord(stored.Tags["GPSLatitude"], stored.Tags["GPSLatitudeRef"]); ok {
			if lon, ok := parseGPSCoord(stored.Tags["GPSLongitude"], stored.Tags["GPSLongitudeRef"]); ok {
				out.Latitude = &lat
				out.Longitude = &lon
			}
		}
	}

	resp.Exif = out
	return resp
}

// parseGPSCoord converts a goexif GPS tag string to decimal degrees.
// Handles rational DMS format "[48/1, 52/1, 4746/100]" and plain decimals.
func parseGPSCoord(coord, ref string) (float64, bool) {
	coord = strings.TrimSpace(coord)
	if coord == "" {
		return 0, false
	}
	// Plain decimal (e.g. "48.879850").
	if v, err := strconv.ParseFloat(coord, 64); err == nil {
		if strings.EqualFold(strings.TrimSpace(ref), "S") || strings.EqualFold(strings.TrimSpace(ref), "W") {
			v = -v
		}
		return v, true
	}
	// Rational DMS: "[d/1, m/1, s/100]".
	coord = strings.Trim(coord, "[] ")
	parts := strings.SplitN(coord, ",", 3)
	if len(parts) != 3 {
		return 0, false
	}
	vals := make([]float64, 3)
	for i, p := range parts {
		n, d, ok := parseRat(strings.TrimSpace(p))
		if !ok || d == 0 {
			return 0, false
		}
		vals[i] = n / d
	}
	deg := vals[0] + vals[1]/60 + vals[2]/3600
	if strings.EqualFold(strings.TrimSpace(ref), "S") || strings.EqualFold(strings.TrimSpace(ref), "W") {
		deg = -deg
	}
	return deg, true
}

func parseRat(s string) (float64, float64, bool) {
	idx := strings.IndexByte(s, '/')
	if idx < 0 {
		return 0, 0, false
	}
	n, err1 := strconv.ParseFloat(s[:idx], 64)
	d, err2 := strconv.ParseFloat(s[idx+1:], 64)
	if err1 != nil || err2 != nil {
		return 0, 0, false
	}
	return n, d, true
}

// --- Metadata ---

func getMeta(mgr *lib.Manager) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id := r.PathValue("id")
		photoID := r.PathValue("photoID")

		store, err := mgr.OpenStore(id)
		if err != nil {
			http.Error(w, "library not found", http.StatusNotFound)
			return
		}
		defer store.Close()

		entries, err := store.GetMeta(photoID)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		writeJSON(w, normalizeBuiltMetaKeys(entries))
	}
}

// normalizeBuiltMetaKeys rewrites legacy "published:"-prefixed meta keys to the
// current "built:" prefix for API responses, so the frontend only ever has to deal
// with the current name. If a photo already has a "built:" entry with the same
// suffix (e.g. after a rebuild), the legacy entry is dropped instead of renamed, to
// avoid presenting duplicate/conflicting entries for the same channel.
func normalizeBuiltMetaKeys(entries []lib.MetaEntry) []lib.MetaEntry {
	existing := make(map[string]bool, len(entries))
	for _, e := range entries {
		if strings.HasPrefix(e.Key, "built:") {
			existing[e.Key] = true
		}
	}
	out := make([]lib.MetaEntry, 0, len(entries))
	for _, e := range entries {
		if strings.HasPrefix(e.Key, "published:") {
			builtKey := "built:" + strings.TrimPrefix(e.Key, "published:")
			if existing[builtKey] {
				continue // superseded by a current built: entry
			}
			e.Key = builtKey
		}
		out = append(out, e)
	}
	return out
}

func upsertMeta(mgr *lib.Manager) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id := r.PathValue("id")
		photoID := r.PathValue("photoID")

		var body struct {
			Key   string `json:"key"`
			Value string `json:"value"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil || body.Key == "" {
			http.Error(w, "key and value required", http.StatusBadRequest)
			return
		}

		store, err := mgr.OpenStore(id)
		if err != nil {
			http.Error(w, "library not found", http.StatusNotFound)
			return
		}
		defer store.Close()

		if err := store.UpsertMeta(photoID, body.Key, body.Value); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		if body.Key == "title" {
			if pathHint, phErr := store.GetPhotoPathHint(photoID); phErr == nil && pathHint != "" {
				media.WriteTitle(pathHint, body.Value) //nolint:errcheck
			}
		}
		w.WriteHeader(http.StatusNoContent)
	}
}

func deleteMeta(mgr *lib.Manager, chStore *channels.Store, draftStore *channels.DraftStore) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id := r.PathValue("id")
		photoID := r.PathValue("photoID")
		key := r.URL.Query().Get("key")
		if key == "" {
			http.Error(w, "key query param required", http.StatusBadRequest)
			return
		}

		store, err := mgr.OpenStore(id)
		if err != nil {
			http.Error(w, "library not found", http.StatusNotFound)
			return
		}
		defer store.Close()

		// For pending:{slug} keys, the meta row is a derived signal that this
		// photo sits in an unfinished draft for that channel — removing it must
		// also remove the photo from the draft itself (drafts.json), not just
		// delete the meta row and leave the draft still holding a reference the
		// UI no longer shows.
		const pendingPrefix = "pending:"
		if strings.HasPrefix(key, pendingPrefix) && draftStore != nil {
			// Two key shapes: "pending:<slug>" names the draft in its value,
			// "pending:<slug>:<draftID>" names it in the key itself (its value
			// is the album title).
			slug, draftID, qualified := strings.Cut(strings.TrimPrefix(key, pendingPrefix), ":")
			if !qualified {
				if entries, metaErr := store.GetMeta(photoID); metaErr == nil {
					for _, e := range entries {
						if e.Key == key {
							draftID = e.Value
							break
						}
					}
				}
			}
			if draftID != "" {
				draftStore.RemovePhoto(slug, draftID, id, photoID) //nolint:errcheck
			}
			store.DeleteMeta(photoID, key) //nolint:errcheck
			if qualified {
				clearPendingMarkers(store, photoID, slug, draftID)
			} else {
				store.DeleteMeta(photoID, pendingPrefix+slug+":"+draftID) //nolint:errcheck
			}
			w.WriteHeader(http.StatusNoContent)
			return
		}

		// For built:{slug} keys on site-export channels, remove the photo from
		// the site (site.json + physical files) and delete all related keys. The
		// frontend only ever sends the normalized built: prefix (see getMeta), but a
		// photo may still carry legacy published:{slug} entries from before this
		// channel-membership key was renamed, so both prefixes are cleaned up here.
		if strings.HasPrefix(key, buildPrefix) && chStore != nil {
			rest := strings.TrimPrefix(key, buildPrefix)
			slug, albumPostID, _ := strings.Cut(rest, ":")
			if ch, chErr := chStore.Get(slug); chErr == nil {
				switch {
				// Removing a publication from a site-export photo means taking
				// it off the site entirely — there is no per-album removal —
				// so both the channel marker and one album's key land here.
				case ch.SiteExport && !reservedMetaSuffix(albumPostID):
					if rmErr := removePhotoFromSite(store, ch, chStore, photoID, slug); rmErr != nil {
						http.Error(w, "remove from site: "+rmErr.Error(), http.StatusInternalServerError)
						return
					}
					deleteChannelPublicationKeys(store, photoID, slug)
					w.WriteHeader(http.StatusNoContent)
					return
				// A gallery channel's albums are independent: drop just this
				// album, and the channel marker only if it was the last one.
				case albumPostID != "" && !reservedMetaSuffix(albumPostID):
					deleteAlbumKeys(store, photoID, slug, albumPostID)
					if entries, metaErr := store.GetMeta(photoID); metaErr == nil && len(albumPostIDsForChannel(entries, slug)) == 0 {
						deleteChannelPublicationKeys(store, photoID, slug)
					}
					w.WriteHeader(http.StatusNoContent)
					return
				case albumPostID == "":
					deleteChannelPublicationKeys(store, photoID, slug)
					w.WriteHeader(http.StatusNoContent)
					return
				}
			}
		}

		if key == "title" {
			if pathHint, phErr := store.GetPhotoPathHint(photoID); phErr == nil && pathHint != "" {
				media.WriteTitle(pathHint, "") //nolint:errcheck
			}
		}
		if err := store.DeleteMeta(photoID, key); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	}
}

// removePhotoFromSite removes a photo from every album in a site-export channel:
// updates site.json, deletes the exported file and thumbnail, regenerates album HTML
// and the site index. Meta key deletion is handled by the caller.
func removePhotoFromSite(store *lib.Store, ch *channels.Channel, chStore *channels.Store, photoID, slug string) error {
	channelDir := chStore.OutputDir(slug)
	siteDir := filepath.Join(channelDir, "site")
	sites := newSiteStore(chStore, slug)

	albums, err := sites.List()
	if err != nil {
		return fmt.Errorf("load site state: %w", err)
	}

	// Reconstruct the expected filename prefix as a fallback for pre-photoID entries.
	pathHint, _ := store.GetPhotoPathHint(photoID)
	var legacyPrefix string
	if pathHint != "" {
		if metaEntries, metaErr := store.GetMeta(photoID); metaErr == nil {
			buildKey := "built:" + slug
			legacyKey := "published:" + slug
			for _, e := range metaEntries {
				if e.Key == buildKey || e.Key == legacyKey {
					if t, tErr := time.Parse(time.RFC3339, e.Value); tErr == nil {
						ts := t.UTC().Format("20060102T150405Z")
						base := strings.TrimSuffix(filepath.Base(pathHint), filepath.Ext(pathHint))
						legacyPrefix = slug + "_" + ts + "_" + base
					}
					break
				}
			}
		}
	}

	modified := false
	touched := map[string]bool{} // postIDs of albums that lost photos
	for i := range albums {
		var kept []SitePhoto
		albumDir := filepath.Join(siteDir, "albums", albumFolderName(albums[i]))
		for _, sp := range albums[i].Photos {
			match := (sp.PhotoID != "" && sp.PhotoID == photoID) ||
				(legacyPrefix != "" && strings.HasPrefix(sp.Filename, legacyPrefix))
			if match {
				// Delete exported file and thumbnail, and take the album
				// out of the photo's own sidecar so a rebuild cannot find it.
				if pathHint != "" {
					media.RemovePublication(pathHint, slug, albums[i].PostID) //nolint:errcheck
				}
				os.Remove(filepath.Join(albumDir, sp.Filename)) //nolint:errcheck
				if sp.ThumbFilename != "" {
					os.Remove(filepath.Join(albumDir, sp.ThumbFilename)) //nolint:errcheck
				}
				modified = true
			} else {
				kept = append(kept, sp)
			}
		}
		if len(kept) != len(albums[i].Photos) {
			albums[i].Photos = kept
			albums[i].PhotoCount = len(kept)
			touched[albums[i].PostID] = true
		}
	}

	if !modified {
		return nil
	}

	// Remove albums that are now empty and regenerate HTML for those that remain.
	rootNav := buildSiteNavContext(ch, siteDir, true)
	albumNav := buildSiteNavContext(ch, siteDir, false)
	var remaining []SiteAlbum
	for _, album := range albums {
		if album.PhotoCount == 0 {
			os.RemoveAll(filepath.Join(siteDir, "albums", albumFolderName(album))) //nolint:errcheck
			if err := sites.Delete(album.PostID); err != nil {                     // its last photo was taken off the site on purpose
				return fmt.Errorf("save site state: %w", err)
			}
			continue
		}
		remaining = append(remaining, album)
		if touched[album.PostID] {
			if err := sites.Upsert(album); err != nil {
				return fmt.Errorf("save site state: %w", err)
			}
		}
		albumDir := filepath.Join(siteDir, "albums", albumFolderName(album))
		items := buildGalleryItems(album.Photos)
		zipName := ""
		zipPath := filepath.Join(albumDir, "photos.zip")
		if album.HasZip || func() bool { _, e := os.Stat(zipPath); return e == nil }() {
			// Rebuild ZIP without the removed photo.
			zipResults := make([]buildResult, len(album.Photos))
			for i, sp := range album.Photos {
				zipResults[i] = buildResult{Filename: sp.Filename}
			}
			if zipErr := createGalleryZip(zipResults, albumDir, "photos.zip"); zipErr == nil {
				zipName = "photos.zip"
			}
		}
		dateStr := dateRangeStr(album.PublishedAt, album.UpdatedAt)
		albumHTML := GenerateSiteGallery(album.Title, ch.SiteTheme, items, GalleryOptions{
			ZipFilename: zipName,
			SiteTitle:   ch.SiteTitle,
			DateStr:     dateStr,
			SiteURL:     ch.SiteURL,
			AlbumSlug:   albumFolderName(album),
			PublishedAt: album.PublishedAt,
			Unlisted:    album.Unlisted,
			Nav:         albumNav,
		})
		os.WriteFile(filepath.Join(albumDir, "index.html"), albumHTML, 0o644) //nolint:errcheck
	}

	siteHTML := GenerateSiteIndex(ch.SiteTitle, ch.SiteTheme, ch.SiteURL, remaining, rootNav)
	os.WriteFile(filepath.Join(siteDir, "index.html"), siteHTML, 0o644) //nolint:errcheck
	generateAboutPage(siteDir, ch, avatarExistsAt(siteDir), rootNav)    //nolint:errcheck
	generateImprintPage(siteDir, ch, rootNav)                           //nolint:errcheck
	generateRobotsTxt(siteDir, ch.SiteURL)                              //nolint:errcheck
	if ch.SiteURL != "" {
		generateSitemap(siteDir, remaining, ch.SiteURL) //nolint:errcheck
	}
	return nil
}

// --- Build ---

type buildResult struct {
	PhotoID       string `json:"photoID"`
	OutputPath    string `json:"outputPath,omitempty"`
	Filename      string `json:"filename,omitempty"`
	ThumbFilename string `json:"thumbFilename,omitempty"`
	Width         int    `json:"width,omitempty"`
	Height        int    `json:"height,omitempty"`
	Error         string `json:"error,omitempty"`
}

func generateDraft(mgr *lib.Manager, chStore *channels.Store, draftStore *channels.DraftStore) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if chStore == nil || draftStore == nil {
			http.Error(w, "channel store not available", http.StatusServiceUnavailable)
			return
		}
		slug := r.PathValue("slug")
		draftID := r.PathValue("draftID")

		var body struct {
			PublishedAt string `json:"publishedAt,omitempty"`
		}
		json.NewDecoder(r.Body).Decode(&body) //nolint:errcheck // empty body is valid; PublishedAt defaults below

		ch, err := chStore.Get(slug)
		if err != nil {
			http.Error(w, "channel not found: "+err.Error(), http.StatusBadRequest)
			return
		}

		// draftID == "-" is the sentinel for "regenerate this gallery/album with
		// no newly collected photos" — used by the Publish dialog for a Live
		// gallery that has no pending draft at all. It subsumes the old
		// Rebuild/Rebuild-site actions: same code path below, just with an
		// empty Photos list, driven by a synthetic in-memory draft instead of
		// one loaded from drafts.json.
		var draft *channels.Draft
		if draftID == "-" {
			postID := r.URL.Query().Get("postID")
			if postID == "" {
				http.Error(w, "postID query parameter required when regenerating without a draft", http.StatusBadRequest)
				return
			}
			draft = &channels.Draft{Target: channels.DraftTarget{PostID: postID}}
		} else {
			draft, err = draftStore.Get(slug, draftID)
			if err != nil {
				http.Error(w, err.Error(), http.StatusBadRequest)
				return
			}
		}
		if draft.Target.Account != "" && ch.AccountByID(draft.Target.Account) == nil {
			http.Error(w, "account not found: "+draft.Target.Account, http.StatusBadRequest)
			return
		}

		publishedAt := time.Now().UTC()
		if body.PublishedAt != "" {
			if t, parseErr := time.Parse(time.RFC3339, body.PublishedAt); parseErr == nil {
				publishedAt = t
			}
		}

		// Open one *lib.Store per distinct library referenced by the draft —
		// a draft's photos can span multiple libraries, collected across
		// separate sessions (ADR-0016: channel output is library-independent).
		stores := map[string]*lib.Store{}
		defer func() {
			for _, s := range stores {
				s.Close()
			}
		}()
		for _, dp := range draft.Photos {
			if _, ok := stores[dp.LibraryID]; ok {
				continue
			}
			s, openErr := mgr.OpenStore(dp.LibraryID)
			if openErr != nil {
				http.Error(w, "library not found: "+dp.LibraryID, http.StatusNotFound)
				return
			}
			stores[dp.LibraryID] = s
		}

		ts := publishedAt.UTC().Format("20060102T150405Z")

		addToExisting := draft.Target.PostID != ""
		galleryMode := ch.GalleryExport && (draft.Target.Title != "" || addToExisting)
		siteMode := ch.SiteExport && (draft.Target.Title != "" || addToExisting)
		channelDir := chStore.OutputDir(slug)

		target, status, targetErr := resolveAlbumTarget(draft, channelDir, newSiteStore(chStore, slug), publishedAt, galleryMode, siteMode)
		if targetErr != nil {
			http.Error(w, targetErr.Error(), status)
			return
		}
		albumPostID := target.postID
		albumSlug := target.slug
		outDir := target.outDir
		existingPhotos := target.existingPhotos
		existingTitle := target.existingTitle
		existingPublishedAt := target.existingPublishedAt
		albumUnlisted := target.unlisted

		// PostID must name the album the photos actually land in. On
		// add-to-existing that is the target album — minting a fresh ID here
		// would write XMP sidecars and built: meta pointing at a gallery that
		// is never created.
		pub := media.Publication{
			Channel: slug, Account: draft.Target.Account, PostID: albumPostID,
			GalleryTitle: draft.Target.Title, PublishedAt: publishedAt,
		}
		if siteMode {
			// The slug is the album's URL and cannot be derived again, so
			// each photo's sidecar carries it (with the flag it encodes).
			pub.Slug, pub.Unlisted = albumSlug, albumUnlisted
		}

		if err := os.MkdirAll(outDir, 0o700); err != nil {
			http.Error(w, "create output dir: "+err.Error(), http.StatusInternalServerError)
			return
		}

		// When adding to an existing album, ensure the album title is recorded in meta
		// for each newly built photo so they appear in album-based library searches.
		if existingTitle != "" && pub.GalleryTitle == "" {
			pub.GalleryTitle = existingTitle
		}

		// clearSucceededPhotos removes only the draft photos that were actually
		// exported without error, leaving failed ones (missing source file,
		// unreadable image, disk full, ...) pending in the draft so the user can
		// see and retry them — buildOne reports failure per-photo via res.Error
		// rather than aborting the whole batch, so a draft can partially succeed.
		// clearSucceededPhotos pairs results with draft.Photos by index: both the
		// synchronous and streaming build loops append exactly one buildResult per
		// draft.Photos entry, in the same order, with no filtering in between — so
		// results[i] always corresponds to draft.Photos[i]. This index pairing (rather
		// than a map keyed by the bare content-hash PhotoID) is required because the
		// same PhotoID can legitimately appear under two different LibraryIDs in one
		// draft (duplicate-content imports across libraries): keying by PhotoID alone
		// would let one library's success mark the other library's failed entry as
		// succeeded too, silently clearing its pending: meta.
		// rememberAlbum pins the draft to the album its photos just landed in.
		// Photos that failed to export stay in the draft; without this, the
		// retry would mint a fresh postID and build a second album with the
		// same title instead of completing the first one.
		rememberAlbum := func() {
			if addToExisting || draftID == "-" {
				return
			}
			draftStore.SetTargetPostID(slug, draftID, albumPostID) //nolint:errcheck
		}

		clearSucceededPhotos := func(results []buildResult) {
			if draftID == "-" {
				return // synthetic draft — nothing was ever persisted
			}
			for i, dp := range draft.Photos {
				if i >= len(results) || results[i].Error != "" {
					continue // failed export: keep in draft and keep pending: meta
				}
				draftStore.RemovePhoto(slug, draftID, dp.LibraryID, dp.PhotoID) //nolint:errcheck
				if s, ok := stores[dp.LibraryID]; ok {
					clearPendingMarkers(s, dp.PhotoID, slug, draftID)
				}
			}
		}

		if !galleryMode && !siteMode {
			// Fast synchronous path for regular (non-gallery) builds.
			var results []buildResult
			for _, dp := range draft.Photos {
				res := buildOne(stores[dp.LibraryID], ch, pub, ts, outDir, "", dp.PhotoID, true)
				results = append(results, res)
			}
			clearSucceededPhotos(results)
			writeJSON(w, map[string]any{"postID": albumPostID, "results": results})
			return
		}

		// Gallery path: stream SSE progress, generate thumbnails, ZIP, and HTML.
		thumbDir := filepath.Join(outDir, "thumbs")
		if err := os.MkdirAll(thumbDir, 0o700); err != nil {
			http.Error(w, "create thumbs dir: "+err.Error(), http.StatusInternalServerError)
			return
		}

		flusher, ok := w.(http.Flusher)
		if !ok {
			http.Error(w, "streaming not supported", http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "text/event-stream")
		w.Header().Set("Cache-Control", "no-cache")
		w.Header().Set("X-Accel-Buffering", "no")
		w.WriteHeader(http.StatusOK)

		emit := func(v any) {
			data, _ := json.Marshal(v)
			fmt.Fprintf(w, "data: %s\n\n", data)
			flusher.Flush()
		}

		total := len(draft.Photos)
		var results []buildResult
		for i, dp := range draft.Photos {
			res := buildOne(stores[dp.LibraryID], ch, pub, ts, outDir, thumbDir, dp.PhotoID, true)
			results = append(results, res)
			emit(map[string]any{"step": "photo", "done": i + 1, "total": total, "file": res.Filename})
		}

		// Build merged items list: existing photos first, then newly exported,
		// each file once.
		items := mergePhotoItems(existingPhotos, results)

		// The ZIP holds the same files as the page, including the previous
		// photos when adding to an existing album.
		zipResults := make([]buildResult, len(items))
		for i, it := range items {
			zipResults[i] = buildResult{Filename: it.Filename}
		}

		// ZIP of full-res photos.
		emit(map[string]any{"step": "zip", "done": 0, "total": 1, "file": "Creating ZIP…"})
		zipName := "photos.zip"
		if zipErr := createGalleryZip(zipResults, outDir, zipName); zipErr != nil {
			emit(map[string]any{"step": "zip", "done": 0, "total": 1, "file": "ZIP failed: " + zipErr.Error()})
			zipName = ""
		} else {
			emit(map[string]any{"step": "zip", "done": 1, "total": 1, "file": "ZIP ready"})
		}

		// Gallery title: use existing title when adding to an existing gallery.
		galleryTitle := draft.Target.Title
		if existingTitle != "" {
			galleryTitle = existingTitle
		}

		// Compute the date range string for SEO metadata.
		albumPublishedAt := publishedAt
		var albumUpdatedAt time.Time
		if !existingPublishedAt.IsZero() {
			albumPublishedAt = existingPublishedAt
			albumUpdatedAt = publishedAt
		}
		dateStr := dateRangeStr(albumPublishedAt, albumUpdatedAt)

		// Generate HTML gallery.
		emit(map[string]any{"step": "html", "done": 0, "total": 1, "file": "Generating gallery…"})
		var html []byte
		if siteMode {
			html = GenerateSiteGallery(galleryTitle, ch.SiteTheme, items, GalleryOptions{
				ZipFilename: zipName,
				SiteTitle:   ch.SiteTitle,
				DateStr:     dateStr,
				SiteURL:     ch.SiteURL,
				AlbumSlug:   albumSlug,
				PublishedAt: albumPublishedAt,
				Unlisted:    albumUnlisted,
				Nav:         buildSiteNavContext(ch, filepath.Join(channelDir, "site"), false),
			})
		} else {
			html = GenerateGallery(galleryTitle, items, GalleryOptions{ZipFilename: zipName, DateStr: dateStr, Unlisted: albumUnlisted})
		}
		indexPath := filepath.Join(outDir, "index.html")
		if err := os.WriteFile(indexPath, html, 0o644); err != nil {
			emit(map[string]any{"error": "write gallery: " + err.Error()})
			return
		}
		if galleryMode {
			if err := writeGalleryAssets(outDir); err != nil {
				emit(map[string]any{"error": "write gallery assets: " + err.Error()})
				return
			}
		}

		// Write gallery.json statefile for single-gallery mode.
		if galleryMode && !siteMode {
			gsPublishedAt := publishedAt
			var gsUpdatedAt time.Time
			if !existingPublishedAt.IsZero() {
				gsPublishedAt = existingPublishedAt
				gsUpdatedAt = publishedAt
			}
			sitePhotos := make([]SitePhoto, len(items))
			for i, item := range items {
				sitePhotos[i] = SitePhoto{PhotoID: item.PhotoID, Filename: item.Filename, ThumbFilename: item.ThumbFilename}
			}
			// The build time is now; the deploy time belongs to the last
			// upload and is carried over, so a rebuild without an upload
			// leaves the gallery visibly "built, not uploaded" (ADR-0029).
			var gsDeployedAt time.Time
			if existing, _ := loadGalleryState(filepath.Join(outDir, "gallery.json")); existing != nil {
				gsDeployedAt = existing.DeployedAt
			}
			gs := &GalleryState{
				PostID:      albumPostID,
				Title:       galleryTitle,
				PublishedAt: gsPublishedAt,
				UpdatedAt:   gsUpdatedAt,
				PhotoCount:  len(items),
				GeneratedAt: time.Now().UTC(),
				DeployedAt:  gsDeployedAt,
				HasZip:      zipName != "",
				Unlisted:    albumUnlisted,
				Photos:      sitePhotos,
			}
			saveGalleryState(filepath.Join(outDir, "gallery.json"), gs) //nolint:errcheck
		}

		if siteMode && len(items) > 0 {
			emit(map[string]any{"step": "site", "done": 0, "total": 1, "file": "Updating site index…"})
			siteDir := filepath.Join(channelDir, "site")
			// Only update cover on initial build (new album).
			if !addToExisting {
				if cover, rdErr := os.ReadFile(filepath.Join(outDir, items[0].ThumbFilename)); rdErr == nil {
					os.WriteFile(filepath.Join(outDir, "cover.jpg"), cover, 0o644) //nolint:errcheck
				}
			}
			if assetsErr := writeSiteAssets(filepath.Join(siteDir, "assets")); assetsErr != nil {
				emit(map[string]any{"error": "write site assets: " + assetsErr.Error()})
				return
			}
			sites := newSiteStore(chStore, slug)
			siteAlbums, listErr := sites.List()
			if listErr != nil {
				emit(map[string]any{"error": "read site state: " + listErr.Error()})
				return
			}
			sitePhotos := make([]SitePhoto, len(items))
			for i, item := range items {
				sitePhotos[i] = SitePhoto{PhotoID: item.PhotoID, Filename: item.Filename, ThumbFilename: item.ThumbFilename}
			}
			// Only the album this build touched is written; the others belong
			// to whichever installation published them.
			var touched SiteAlbum
			if addToExisting {
				// Update existing album entry; preserve PublishedAt for sort
				// order and DeployedAt, which describes the last upload.
				if idx := indexOfAlbum(siteAlbums, albumPostID); idx >= 0 {
					touched = siteAlbums[idx]
					touched.Photos = sitePhotos
					touched.PhotoCount = len(items)
					touched.HasZip = zipName != ""
					touched.UpdatedAt = publishedAt
					touched.GeneratedAt = time.Now().UTC()
				}
			} else {
				touched = SiteAlbum{
					PostID:      albumPostID,
					Slug:        albumSlug,
					Title:       galleryTitle,
					PublishedAt: publishedAt,
					PhotoCount:  len(items),
					GeneratedAt: time.Now().UTC(),
					CoverFile:   "cover.jpg",
					HasZip:      zipName != "",
					Photos:      sitePhotos,
					Unlisted:    albumUnlisted,
				}
			}
			if touched.PostID != "" {
				if saveErr := sites.Upsert(touched); saveErr != nil {
					emit(map[string]any{"error": "save site state: " + saveErr.Error()})
					return
				}
			}
			// The index is derived from the whole register, not from the
			// albums this machine happens to know.
			siteAlbums, listErr = sites.List()
			if listErr != nil {
				emit(map[string]any{"error": "read site state: " + listErr.Error()})
				return
			}
			rootNav := buildSiteNavContext(ch, siteDir, true)
			siteHTML := GenerateSiteIndex(ch.SiteTitle, ch.SiteTheme, ch.SiteURL, siteAlbums, rootNav)
			if writeErr := os.WriteFile(filepath.Join(siteDir, "index.html"), siteHTML, 0o644); writeErr != nil {
				emit(map[string]any{"error": "write site index: " + writeErr.Error()})
				return
			}
			generateAboutPage(siteDir, ch, avatarExistsAt(siteDir), rootNav) //nolint:errcheck
			generateImprintPage(siteDir, ch, rootNav)                        //nolint:errcheck
			generateRobotsTxt(siteDir, ch.SiteURL)                           //nolint:errcheck
			if ch.SiteURL != "" {
				generateSitemap(siteDir, siteAlbums, ch.SiteURL) //nolint:errcheck
			}
			emit(map[string]any{"step": "site", "done": 1, "total": 1, "file": "Site index updated"})
			rememberAlbum()
			clearSucceededPhotos(results)
			emit(map[string]any{"complete": true, "postID": albumPostID, "galleryPath": outDir, "sitePath": siteDir, "results": results})
		} else {
			rememberAlbum()
			clearSucceededPhotos(results)
			emit(map[string]any{"complete": true, "postID": albumPostID, "galleryPath": outDir, "results": results})
		}
	}
}

// buildDownload exports selected library photos with channel settings and delivers
// the result as a ZIP download. By default it does not update XMP sidecars or the
// library database; pass RecordXMP=true to opt in.
func buildDownload(mgr *lib.Manager, chStore *channels.Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		if chStore == nil {
			http.Error(w, "channel store not available", http.StatusServiceUnavailable)
			return
		}
		id := r.PathValue("id")

		var body struct {
			PhotoIDs  []string `json:"photoIDs"`
			Channel   string   `json:"channel"`
			RecordXMP *bool    `json:"recordXMP,omitempty"` // nil → false (default for download)
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			http.Error(w, "invalid JSON", http.StatusBadRequest)
			return
		}
		if len(body.PhotoIDs) == 0 || body.Channel == "" {
			http.Error(w, "photoIDs and channel required", http.StatusBadRequest)
			return
		}

		ch, err := chStore.Get(body.Channel)
		if err != nil {
			http.Error(w, "channel not found: "+err.Error(), http.StatusBadRequest)
			return
		}

		store, err := mgr.OpenStore(id)
		if err != nil {
			http.Error(w, "library not found", http.StatusNotFound)
			return
		}
		defer store.Close()

		tmpFile, err := os.CreateTemp("", "unterlumen-channel-zip-*.zip")
		if err != nil {
			http.Error(w, "create temp file: "+err.Error(), http.StatusInternalServerError)
			return
		}
		tmpPath := tmpFile.Name()
		defer os.Remove(tmpPath)

		recordXMP := body.RecordXMP != nil && *body.RecordXMP
		publishedAt := time.Now().UTC()
		ts := publishedAt.Format("20060102T150405Z")
		opts := ch.ExportOptions()
		ext := "." + ch.Format
		if ch.Format == "jpeg" {
			ext = ".jpg"
		}

		var pub media.Publication
		if recordXMP {
			pub = media.Publication{
				Channel:     body.Channel,
				PostID:      newPostID(),
				PublishedAt: publishedAt,
			}
		}

		zw := zip.NewWriter(tmpFile)
		for _, photoID := range body.PhotoIDs {
			pathHint, pathErr := store.GetPhotoPathHint(photoID)
			if pathErr != nil || pathHint == "" {
				continue
			}
			if recordXMP {
				media.AppendPublication(pathHint, pub) //nolint:errcheck
				chKey := "built:" + pub.Channel
				qualKey := chKey + ":" + pub.PostID
				tsVal := publishedAt.Format(time.RFC3339)
				store.UpsertMeta(photoID, chKey, tsVal)   //nolint:errcheck
				store.UpsertMeta(photoID, qualKey, tsVal) //nolint:errcheck
				if pub.PostID != "" {
					store.UpsertMeta(photoID, chKey+":postid", pub.PostID) //nolint:errcheck
				}
				if pub.GalleryTitle != "" {
					store.UpsertMeta(photoID, chKey+":title", pub.GalleryTitle)   //nolint:errcheck
					store.UpsertMeta(photoID, qualKey+":title", pub.GalleryTitle) //nolint:errcheck
				}
			}
			data, expErr := media.ExportImage(pathHint, opts)
			if expErr != nil {
				continue
			}
			base := strings.TrimSuffix(filepath.Base(pathHint), filepath.Ext(pathHint))
			outName := ch.Slug + "_" + ts + "_" + base + ext
			if fw, fwErr := zw.Create(outName); fwErr == nil {
				fw.Write(data) //nolint:errcheck
			}
		}
		zw.Close()
		tmpFile.Close()

		f, err := os.Open(tmpPath)
		if err != nil {
			http.Error(w, "open temp file: "+err.Error(), http.StatusInternalServerError)
			return
		}
		defer f.Close()

		if info, statErr := os.Stat(tmpPath); statErr == nil {
			w.Header().Set("Content-Length", fmt.Sprintf("%d", info.Size()))
		}
		fname := ch.Slug + "-export.zip"
		w.Header().Set("Content-Type", "application/zip")
		w.Header().Set("Content-Disposition", fmt.Sprintf(`attachment; filename="%s"`, fname))
		io.Copy(w, f) //nolint:errcheck
	}
}

func createGalleryZip(results []buildResult, outDir, zipName string) error {
	f, err := os.Create(filepath.Join(outDir, zipName))
	if err != nil {
		return err
	}
	defer f.Close()

	zw := zip.NewWriter(f)
	defer zw.Close()

	for _, res := range results {
		if res.Error != "" || res.Filename == "" {
			continue
		}
		data, err := os.ReadFile(filepath.Join(outDir, res.Filename))
		if err != nil {
			continue
		}
		w, err := zw.Create(res.Filename)
		if err != nil {
			continue
		}
		w.Write(data) //nolint:errcheck
	}
	return nil
}

var galleryThumbOpts = media.ExportOptions{
	Format:   "jpeg",
	Quality:  78,
	ExifMode: "strip",
	Scale: media.ScaleOptions{
		Mode:         media.ScaleModeMaxDim,
		MaxDimension: "width",
		MaxValue:     700,
	},
}

func buildOne(store *lib.Store, ch *channels.Channel, pub media.Publication, ts, outDir, thumbDir, photoID string, recordXMP bool) buildResult {
	pathHint, err := store.GetPhotoPathHint(photoID)
	if err != nil || pathHint == "" {
		return buildResult{PhotoID: photoID, Error: "photo not found"}
	}

	if recordXMP {
		if err := media.AppendPublication(pathHint, pub); err != nil {
			return buildResult{PhotoID: photoID, Error: "xmp: " + err.Error()}
		}
		metaVal := pub.PublishedAt.UTC().Format(time.RFC3339)
		chKey := "built:" + pub.Channel
		qualKey := chKey + ":" + pub.PostID
		store.UpsertMeta(photoID, chKey, metaVal)   //nolint:errcheck
		store.UpsertMeta(photoID, qualKey, metaVal) //nolint:errcheck
		if pub.Account != "" {
			store.UpsertMeta(photoID, chKey+":account", pub.Account)   //nolint:errcheck
			store.UpsertMeta(photoID, qualKey+":account", pub.Account) //nolint:errcheck
		}
		if pub.PostID != "" {
			store.UpsertMeta(photoID, chKey+":postid", pub.PostID) //nolint:errcheck
		}
		if pub.GalleryTitle != "" {
			store.UpsertMeta(photoID, chKey+":title", pub.GalleryTitle)   //nolint:errcheck
			store.UpsertMeta(photoID, qualKey+":title", pub.GalleryTitle) //nolint:errcheck
		}
	}

	exported, err := media.ExportImage(pathHint, ch.ExportOptions())
	if err != nil {
		return buildResult{PhotoID: photoID, Error: "export: " + err.Error()}
	}

	ext := "." + ch.Format
	if ch.Format == "jpeg" {
		ext = ".jpg"
	}
	base := strings.TrimSuffix(filepath.Base(pathHint), filepath.Ext(pathHint))
	outName := ch.Slug + "_" + ts + "_" + base + ext
	outPath := filepath.Join(outDir, outName)

	if err := os.WriteFile(outPath, exported, 0o644); err != nil {
		return buildResult{PhotoID: photoID, Error: "write export: " + err.Error()}
	}

	res := buildResult{PhotoID: photoID, OutputPath: outPath, Filename: outName}
	if cfg, _, err := image.DecodeConfig(bytes.NewReader(exported)); err == nil {
		res.Width = cfg.Width
		res.Height = cfg.Height
	}

	if thumbDir != "" {
		if thumb, err := media.ExportImage(pathHint, galleryThumbOpts); err == nil {
			thumbName := "thumbs/" + outName
			if err := os.WriteFile(filepath.Join(thumbDir, outName), thumb, 0o644); err == nil {
				res.ThumbFilename = thumbName
			}
		}
	}

	return res
}

// scanAlbumPhotos reconstructs a GalleryItem list from the files on disk.
// Used when rebuilding albums that were built before photo metadata was stored in site.json.
func scanAlbumPhotos(albumDir string) []GalleryItem {
	entries, err := os.ReadDir(albumDir)
	if err != nil {
		return nil
	}
	skip := map[string]bool{"index.html": true, "photos.zip": true, "cover.jpg": true}
	var items []GalleryItem
	for _, e := range entries {
		if e.IsDir() || skip[e.Name()] {
			continue
		}
		ext := strings.ToLower(filepath.Ext(e.Name()))
		if ext != ".jpg" && ext != ".jpeg" && ext != ".png" && ext != ".webp" {
			continue
		}
		thumbName := "thumbs/" + e.Name()
		if _, statErr := os.Stat(filepath.Join(albumDir, thumbName)); statErr != nil {
			thumbName = e.Name() // no thumb — fall back to full-res
		}
		items = append(items, GalleryItem{Filename: e.Name(), ThumbFilename: thumbName})
	}
	return items
}

func newPostID() string {
	b := make([]byte, 12)
	rand.Read(b) //nolint:errcheck
	return fmt.Sprintf("%x", b)
}

// Publication meta keys are "built:<slug>" (channel marker), "built:<slug>:<postID>"
// (one album), and either of those plus a reserved suffix. The legacy
// "published:" prefix predates the rename and is still cleaned up alongside.
const (
	buildPrefix     = "built:"
	legacyPubPrefix = "published:"
)

// reservedMetaSuffix reports whether a key segment is a field name rather than
// an album ID — "built:ch:title" is the channel's latest album title,
// "built:ch:9f2a…" is membership in album 9f2a….
func reservedMetaSuffix(s string) bool {
	return s == "title" || s == "account" || s == "postid"
}

// albumPostIDsForChannel lists every album a photo belongs to in one channel.
// The unqualified ":postid" key only ever names the most recently published
// album, so it can't stand in for this.
func albumPostIDsForChannel(entries []lib.MetaEntry, slug string) []string {
	seen := map[string]bool{}
	var out []string
	for _, e := range entries {
		for _, prefix := range []string{buildPrefix, legacyPubPrefix} {
			rest, ok := strings.CutPrefix(e.Key, prefix+slug+":")
			if !ok || strings.Contains(rest, ":") || reservedMetaSuffix(rest) || seen[rest] {
				continue
			}
			seen[rest] = true
			out = append(out, rest)
		}
	}
	return out
}

// deleteAlbumKeys removes one album's membership keys under both prefixes.
func deleteAlbumKeys(s *lib.Store, photoID, slug, albumPostID string) {
	for _, prefix := range []string{buildPrefix, legacyPubPrefix} {
		for _, suffix := range []string{"", ":account", ":title"} {
			s.DeleteMeta(photoID, prefix+slug+":"+albumPostID+suffix) //nolint:errcheck
		}
	}
}

// deleteChannelPublicationKeys removes a photo's channel marker and every
// album key it holds for that channel, under both prefixes.
func deleteChannelPublicationKeys(s *lib.Store, photoID, slug string) {
	if entries, err := s.GetMeta(photoID); err == nil {
		for _, albumPostID := range albumPostIDsForChannel(entries, slug) {
			deleteAlbumKeys(s, photoID, slug, albumPostID)
		}
	}
	for _, prefix := range []string{buildPrefix, legacyPubPrefix} {
		for _, suffix := range []string{"", ":account", ":title", ":postid"} {
			s.DeleteMeta(photoID, prefix+slug+suffix) //nolint:errcheck
		}
	}
}

// clearPendingMarkers drops one draft's pending marker for a photo. The
// unqualified per-channel marker goes only once no other draft of that channel
// still holds the photo — a photo can be collected into several albums of one
// channel, and publishing one of them must not clear the others' pending state.
func clearPendingMarkers(s *lib.Store, photoID, slug, draftID string) {
	s.DeleteMeta(photoID, "pending:"+slug+":"+draftID) //nolint:errcheck
	entries, err := s.GetMeta(photoID)
	if err != nil {
		return
	}
	for _, e := range entries {
		if strings.HasPrefix(e.Key, "pending:"+slug+":") {
			return
		}
	}
	s.DeleteMeta(photoID, "pending:"+slug) //nolint:errcheck
}

// albumTarget is where a draft's photos are written: the album's stable ID, its
// output folder, and — when appending to an album that already exists — that
// album's current state.
type albumTarget struct {
	postID              string
	slug                string // human-readable folder name; site mode only
	outDir              string
	existingPhotos      []SitePhoto
	existingTitle       string
	existingPublishedAt time.Time
	unlisted            bool
}

// resolveAlbumTarget decides whether a draft appends to an existing album or
// starts a new one, loading the existing album's state in the former case. A
// non-nil error carries the HTTP status the request should fail with.
//
// Unlisted is fixed at album creation: on add-to-existing it comes from the
// stored album, never from the draft, so appending photos can't silently
// un-hide an album whose link has already been shared.
func resolveAlbumTarget(draft *channels.Draft, channelDir string, sites *siteStore, publishedAt time.Time, galleryMode, siteMode bool) (albumTarget, int, error) {
	t := albumTarget{outDir: channelDir}

	if draft.Target.PostID == "" {
		t.postID = newPostID()
		t.unlisted = draft.Target.Unlisted
		switch {
		case galleryMode:
			t.outDir = filepath.Join(channelDir, t.postID)
		case siteMode:
			existingAlbums, _ := sites.List()
			t.slug = computeSlug(draft.Target.Title, publishedAt, existingAlbums, draft.Target.Unlisted)
			t.outDir = filepath.Join(channelDir, "site", "albums", t.slug)
		}
		return t, 0, nil
	}

	t.postID = draft.Target.PostID
	switch {
	case galleryMode:
		t.outDir = filepath.Join(channelDir, t.postID)
		gs, err := loadGalleryState(filepath.Join(t.outDir, "gallery.json"))
		if err != nil || gs == nil {
			return t, http.StatusBadRequest, fmt.Errorf("gallery not found: %s", t.postID)
		}
		t.existingPhotos, t.existingTitle, t.existingPublishedAt = gs.Photos, gs.Title, gs.PublishedAt
		t.unlisted = gs.Unlisted
	case siteMode:
		siteAlbums, err := sites.List()
		if err != nil {
			return t, http.StatusInternalServerError, fmt.Errorf("read site state: %w", err)
		}
		for i := range siteAlbums {
			if siteAlbums[i].PostID != t.postID {
				continue
			}
			t.existingPhotos, t.existingTitle = siteAlbums[i].Photos, siteAlbums[i].Title
			t.existingPublishedAt, t.unlisted = siteAlbums[i].PublishedAt, siteAlbums[i].Unlisted
			t.slug = albumFolderName(siteAlbums[i])
			break
		}
		if t.existingTitle == "" {
			return t, http.StatusBadRequest, fmt.Errorf("album not found: %s", t.postID)
		}
		t.outDir = filepath.Join(channelDir, "site", "albums", t.slug)
	}

	if _, err := os.Stat(t.outDir); os.IsNotExist(err) {
		return t, http.StatusBadRequest, fmt.Errorf("gallery folder not found: %s", t.postID)
	}
	return t, 0, nil
}

// galleryListItem is the JSON shape returned by GET /api/channels/{slug}/galleries.
type galleryListItem struct {
	PostID      string    `json:"postID"`
	Title       string    `json:"title"`
	PublishedAt time.Time `json:"publishedAt"`
	UpdatedAt   time.Time `json:"updatedAt"` // zero-valued when never updated; omitempty does nothing on a time.Time
	PhotoCount  int       `json:"photoCount"`
	// GeneratedAt / DeployedAt let the frontend tell "built, not uploaded"
	// from "online" (ADR-0029). Zero means not recorded, not "never".
	GeneratedAt time.Time `json:"generatedAt"`
	DeployedAt  time.Time `json:"deployedAt"`
	// FolderName is the actual on-disk (and on-URL) folder name for this
	// album — the slugified title, or for unlisted albums the slug plus its
	// random token. Empty for GalleryExport (non-site) channels, which have
	// no per-album folder distinct from PostID.
	FolderName string `json:"folderName,omitempty"`
	// Unlisted mirrors SiteAlbum.Unlisted / GalleryState.Unlisted — true if
	// this album carries a noindex tag and is only reachable via direct link
	// (site channels additionally exclude it from their index and sitemap).
	Unlisted bool `json:"unlisted,omitempty"`
}

// collectGalleryItems loads the published-gallery list for a single channel
// from its statefile(s) — site.json for SiteExport channels, one gallery.json
// per subfolder for GalleryExport channels. Shared by listGalleries (scoped to
// one channel) and listAllGalleries (aggregated across every channel).
func collectGalleryItems(ch *channels.Channel, chStore *channels.Store) ([]galleryListItem, error) {
	var items []galleryListItem
	channelDir := chStore.OutputDir(ch.Slug)

	switch {
	case ch.SiteExport:
		albums, err := newSiteStore(chStore, ch.Slug).List()
		if err != nil {
			return nil, fmt.Errorf("read site state: %w", err)
		}
		for _, a := range albums {
			items = append(items, galleryListItem{
				PostID:      a.PostID,
				Title:       a.Title,
				PublishedAt: a.PublishedAt,
				UpdatedAt:   a.UpdatedAt,
				PhotoCount:  a.PhotoCount,
				GeneratedAt: a.GeneratedAt,
				DeployedAt:  a.DeployedAt,
				FolderName:  albumFolderName(a),
				Unlisted:    a.Unlisted,
			})
		}
		// Sort newest first (site index sorts the same way).
		sort.Slice(items, func(i, j int) bool {
			return items[i].PublishedAt.After(items[j].PublishedAt)
		})
	case ch.GalleryExport:
		entries, rdErr := os.ReadDir(channelDir)
		if rdErr != nil && !os.IsNotExist(rdErr) {
			return nil, fmt.Errorf("read channel dir: %w", rdErr)
		}
		for _, e := range entries {
			if !e.IsDir() {
				continue
			}
			gs, gsErr := loadGalleryState(filepath.Join(channelDir, e.Name(), "gallery.json"))
			if gsErr != nil || gs == nil {
				continue
			}
			items = append(items, galleryListItem{
				PostID:      gs.PostID,
				Title:       gs.Title,
				PublishedAt: gs.PublishedAt,
				UpdatedAt:   gs.UpdatedAt,
				PhotoCount:  gs.PhotoCount,
				GeneratedAt: gs.GeneratedAt,
				DeployedAt:  gs.DeployedAt,
				FolderName:  e.Name(), // gallery-export mode: folder name == PostID, use the actual dir name on disk
				Unlisted:    gs.Unlisted,
			})
		}
		sort.Slice(items, func(i, j int) bool {
			return items[i].PublishedAt.After(items[j].PublishedAt)
		})
	}

	return items, nil
}

// listGalleries returns existing built galleries/albums for a channel.
func listGalleries(chStore *channels.Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if chStore == nil {
			http.Error(w, "channel store not available", http.StatusServiceUnavailable)
			return
		}
		slug := r.PathValue("slug")
		ch, err := chStore.Get(slug)
		if err != nil {
			http.Error(w, "channel not found: "+err.Error(), http.StatusNotFound)
			return
		}
		items, err := collectGalleryItems(ch, chStore)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		if items == nil {
			items = []galleryListItem{}
		}
		writeJSON(w, items)
	}
}

// rebuildSite regenerates site assets and HTML from site.json, cross-checking each
// photo against: (1) file existence on disk, (2) build meta in the library DB
// (when a photoID is stored) or a base-name reverse lookup (for pre-photoID entries).
// Photos that fail either check are pruned from site.json before regenerating.
func rebuildSite(chStore *channels.Store, mgr *lib.Manager) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if chStore == nil {
			http.Error(w, "channel store not available", http.StatusServiceUnavailable)
			return
		}
		channelSlug := r.PathValue("slug")

		ch, err := chStore.Get(channelSlug)
		if err != nil {
			http.Error(w, "channel not found: "+err.Error(), http.StatusBadRequest)
			return
		}
		if !ch.SiteExport {
			http.Error(w, "channel is not configured for site export", http.StatusBadRequest)
			return
		}

		siteDir, albumCount, err := rebuildSiteChannel(chStore, mgr, ch)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		writeJSON(w, map[string]any{"sitePath": siteDir, "albumCount": albumCount})
	}
}

// rebuildSiteChannel regenerates site assets and HTML from site.json for a
// site-export channel, cross-checking each photo against: (1) file existence
// on disk, (2) build meta in the library DB (when a photoID is stored) or a
// base-name reverse lookup (for pre-photoID entries). Photos that fail either
// check are pruned from site.json before regenerating. Shared by the
// rebuild-site HTTP handler and renameGallery/deleteGallery, which need the
// same full regeneration after editing site.json directly.
func rebuildSiteChannel(chStore *channels.Store, mgr *lib.Manager, ch *channels.Channel) (siteDir string, albumCount int, err error) {
	channelSlug := ch.Slug
	siteDir = filepath.Join(chStore.OutputDir(channelSlug), "site")
	sites := newSiteStore(chStore, channelSlug)
	albums, err := sites.List()
	if err != nil {
		return "", 0, fmt.Errorf("read site state: %w", err)
	}

	if err := writeSiteAssets(filepath.Join(siteDir, "assets")); err != nil {
		return "", 0, fmt.Errorf("write site assets: %w", err)
	}

	// Assign slugs to any albums that don't have one yet (migration for pre-slug albums).
	for i := range albums {
		if albums[i].Slug == "" {
			others := make([]SiteAlbum, 0, len(albums)-1)
			others = append(others, albums[:i]...)
			others = append(others, albums[i+1:]...)
			albums[i].Slug = computeSlug(albums[i].Title, albums[i].PublishedAt, others, albums[i].Unlisted)
		}
	}

	// Build a reverse index (base name → photoID) from all library stores once,
	// so we avoid scanning every library for every photo in the album.
	baseToPhotoID := buildBasePhotoIndex(mgr)

	// Prune photos whose build metadata was removed from the library.
	// Photos with a stored PhotoID can be re-exported from library source files if
	// their exported file is missing, so we only remove them on a metadata failure.
	// Legacy entries without a PhotoID cannot be re-exported; they are also pruned
	// when their exported file is absent from disk.
	modifiedIdx := make(map[int]bool) // album index → was modified
	// Repair repeats left by older runs (a photo added to an album it was
	// already in was listed twice).
	for i := range albums {
		if unique := dedupePhotos(albums[i].Photos); len(unique) != len(albums[i].Photos) {
			albums[i].Photos, albums[i].PhotoCount = unique, len(unique)
			modifiedIdx[i] = true
		}
	}
	for i := range albums {
		albumDir := filepath.Join(siteDir, "albums", albumFolderName(albums[i]))
		_, albumDirErr := os.Stat(albumDir)
		albumDirPresent := albumDirErr == nil
		var kept []SitePhoto
		for _, sp := range albums[i].Photos {
			pid := sp.PhotoID
			if pid == "" {
				pid = baseToPhotoID[sitePhotoBase(sp.Filename, channelSlug)]
			}
			// Remove photos that lost their build metadata in the library.
			if pid != "" && mgr != nil && !sitePhotoHasMeta(mgr, pid, channelSlug, albums[i].PostID) {
				modifiedIdx[i] = true
				continue
			}
			// Legacy entries (no stored PhotoID) are also pruned when their exported
			// file is absent — they cannot be re-exported from the library.
			if sp.PhotoID == "" && albumDirPresent {
				if _, statErr := os.Stat(filepath.Join(albumDir, sp.Filename)); os.IsNotExist(statErr) {
					modifiedIdx[i] = true
					continue
				}
			}
			kept = append(kept, sp)
		}
		if modifiedIdx[i] {
			albums[i].Photos = kept
			albums[i].PhotoCount = len(kept)
		}
	}

	// Write only what pruning changed: albums that lost photos are upserted,
	// albums that became empty are removed. Untouched albums stay as they are
	// in the shared register.
	var remaining []SiteAlbum
	for i, album := range albums {
		switch {
		case !modifiedIdx[i]:
			remaining = append(remaining, album)
		case album.PhotoCount == 0:
			os.RemoveAll(filepath.Join(siteDir, "albums", albumFolderName(album))) //nolint:errcheck
			sites.Remove(album.PostID)                                             //nolint:errcheck
		default:
			remaining = append(remaining, album)
			sites.Upsert(album) //nolint:errcheck
		}
	}

	rootNav := buildSiteNavContext(ch, siteDir, true)
	albumNav := buildSiteNavContext(ch, siteDir, false)

	// Regenerate every album page and rebuild its ZIP if one exists.
	for i := range remaining {
		album := &remaining[i]
		albumDir := filepath.Join(siteDir, "albums", albumFolderName(*album))
		thumbDir := filepath.Join(albumDir, "thumbs")
		os.MkdirAll(thumbDir, 0o700) //nolint:errcheck

		// Re-export any photos whose exported file is missing from disk.
		// This restores the full album after the output folder has been wiped,
		// using the original source files in the library.
		if mgr != nil {
			for j := range album.Photos {
				sp := &album.Photos[j]
				if sp.PhotoID == "" {
					continue // legacy entry — no source link, cannot re-export
				}
				photoPath := filepath.Join(albumDir, sp.Filename)
				if _, statErr := os.Stat(photoPath); statErr == nil {
					continue // file already on disk
				}
				srcPath, srcErr := findPhotoSourcePath(mgr, sp.PhotoID)
				if srcErr != nil || srcPath == "" {
					continue
				}
				if exported, exportErr := media.ExportImage(srcPath, ch.ExportOptions()); exportErr == nil {
					os.WriteFile(photoPath, exported, 0o644) //nolint:errcheck
				}
				if sp.ThumbFilename != "" {
					thumbPath := filepath.Join(albumDir, sp.ThumbFilename)
					if _, statErr := os.Stat(thumbPath); os.IsNotExist(statErr) {
						if thumb, thumbErr := media.ExportImage(srcPath, galleryThumbOpts); thumbErr == nil {
							os.WriteFile(thumbPath, thumb, 0o644) //nolint:errcheck
						}
					}
				}
			}
		}

		items := buildGalleryItems(album.Photos)
		if len(items) == 0 {
			items = scanAlbumPhotos(albumDir)
		}
		if len(items) == 0 {
			continue
		}
		zipName := ""
		zipPath := filepath.Join(albumDir, "photos.zip")
		if album.HasZip || func() bool { _, e := os.Stat(zipPath); return e == nil }() {
			zipResults := make([]buildResult, len(album.Photos))
			for j, sp := range album.Photos {
				zipResults[j] = buildResult{Filename: sp.Filename}
			}
			if zipErr := createGalleryZip(zipResults, albumDir, "photos.zip"); zipErr == nil {
				zipName = "photos.zip"
				album.HasZip = true
			}
		}
		dateStr := dateRangeStr(album.PublishedAt, album.UpdatedAt)
		albumHTML := GenerateSiteGallery(album.Title, ch.SiteTheme, items, GalleryOptions{
			ZipFilename: zipName,
			SiteTitle:   ch.SiteTitle,
			DateStr:     dateStr,
			SiteURL:     ch.SiteURL,
			AlbumSlug:   albumFolderName(*album),
			PublishedAt: album.PublishedAt,
			Unlisted:    album.Unlisted,
			Nav:         albumNav,
		})
		os.WriteFile(filepath.Join(albumDir, "index.html"), albumHTML, 0o644) //nolint:errcheck
	}

	siteHTML := GenerateSiteIndex(ch.SiteTitle, ch.SiteTheme, ch.SiteURL, remaining, rootNav)
	if err := os.WriteFile(filepath.Join(siteDir, "index.html"), siteHTML, 0o644); err != nil {
		return "", 0, fmt.Errorf("write site index: %w", err)
	}
	generateAboutPage(siteDir, ch, avatarExistsAt(siteDir), rootNav) //nolint:errcheck
	generateImprintPage(siteDir, ch, rootNav)                        //nolint:errcheck
	generateRobotsTxt(siteDir, ch.SiteURL)                           //nolint:errcheck
	if ch.SiteURL != "" {
		generateSitemap(siteDir, remaining, ch.SiteURL) //nolint:errcheck
	}

	return siteDir, len(remaining), nil
}

// rebuildGalleries regenerates index.html for every existing single-gallery
// build on a GalleryExport channel, from each build's gallery.json state and
// its already-exported photo files on disk. It re-runs the current
// GenerateGallery template (picking up template fixes/changes) without
// re-exporting or duplicating any photos — the counterpart to rebuildSite
// for channels that use single-gallery export instead of a multi-album site.
func rebuildGalleries(chStore *channels.Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if chStore == nil {
			http.Error(w, "channel store not available", http.StatusServiceUnavailable)
			return
		}
		slug := r.PathValue("slug")
		ch, err := chStore.Get(slug)
		if err != nil {
			http.Error(w, "channel not found: "+err.Error(), http.StatusBadRequest)
			return
		}
		if !ch.GalleryExport {
			http.Error(w, "channel is not configured for single-gallery export", http.StatusBadRequest)
			return
		}

		channelDir := chStore.OutputDir(slug)
		entries, err := os.ReadDir(channelDir)
		if err != nil {
			if os.IsNotExist(err) {
				writeJSON(w, map[string]any{"rebuilt": 0})
				return
			}
			http.Error(w, "read channel dir: "+err.Error(), http.StatusInternalServerError)
			return
		}

		var rebuilt int
		var errs []string
		for _, e := range entries {
			if !e.IsDir() {
				continue
			}
			outDir := filepath.Join(channelDir, e.Name())
			gs, gsErr := loadGalleryState(filepath.Join(outDir, "gallery.json"))
			if gsErr != nil || gs == nil {
				continue // not a gallery folder (or unreadable state) — skip, not an error
			}
			if regenErr := regenerateGalleryFolder(outDir, gs); regenErr != nil {
				errs = append(errs, e.Name()+": "+regenErr.Error())
				continue
			}
			rebuilt++
		}

		if len(errs) > 0 {
			writeJSON(w, map[string]any{"rebuilt": rebuilt, "errors": errs})
			return
		}
		writeJSON(w, map[string]any{"rebuilt": rebuilt})
	}
}

// regenerateGalleryFolder regenerates index.html (and gallery assets) for one
// already-exported single-gallery build, from its gallery.json state and the
// photo files already on disk in outDir. Shared by rebuildGalleries (which
// loops every folder in a channel) and renameGallery/deleteGallery (which
// only need to touch the one affected folder after editing gallery.json).
func regenerateGalleryFolder(outDir string, gs *GalleryState) error {
	items := make([]GalleryItem, 0, len(gs.Photos))
	for _, sp := range gs.Photos {
		w, h := readImageDimensions(filepath.Join(outDir, sp.Filename))
		items = append(items, GalleryItem{
			PhotoID:       sp.PhotoID,
			Filename:      sp.Filename,
			ThumbFilename: sp.ThumbFilename,
			Width:         w,
			Height:        h,
		})
	}

	zipName := ""
	if gs.HasZip {
		zipName = "photos.zip"
	}
	html := GenerateGallery(gs.Title, items, GalleryOptions{
		ZipFilename: zipName,
		DateStr:     dateRangeStr(gs.PublishedAt, gs.UpdatedAt),
		Unlisted:    gs.Unlisted,
	})
	if err := os.WriteFile(filepath.Join(outDir, "index.html"), html, 0o644); err != nil {
		return err
	}
	return writeGalleryAssets(outDir)
}

// readImageDimensions decodes just enough of the image at path to report its
// pixel dimensions, without loading the full image into memory. Used by
// rebuildGalleries to recover width/height for GalleryItem, since
// GalleryState/SitePhoto don't persist them (they're only known at export
// time) — returns 0, 0 (which GalleryItem/thumbDimensions treats as "omit
// width/height attributes") if the file is missing or undecodable.
func readImageDimensions(path string) (int, int) {
	f, err := os.Open(path)
	if err != nil {
		return 0, 0
	}
	defer f.Close()
	cfg, _, err := image.DecodeConfig(f)
	if err != nil {
		return 0, 0
	}
	return cfg.Width, cfg.Height
}

// sitePhotoBase extracts the original source-file base name from a SitePhoto filename.
// SitePhoto filenames follow the pattern "{slug}_{ts}_{base}{ext}".
func sitePhotoBase(filename, slug string) string {
	name := strings.TrimSuffix(filename, filepath.Ext(filename))
	prefix := slug + "_"
	if !strings.HasPrefix(name, prefix) {
		return ""
	}
	rest := name[len(prefix):]
	idx := strings.IndexByte(rest, '_')
	if idx < 0 {
		return ""
	}
	return strings.ToLower(rest[idx+1:])
}

// buildBasePhotoIndex scans all library stores and returns a map from
// lower-cased path_hint base name → photo ID. Used to resolve legacy SitePhoto
// entries that don't have a stored photoID.
func buildBasePhotoIndex(mgr *lib.Manager) map[string]string {
	m := make(map[string]string)
	if mgr == nil {
		return m
	}
	libs, err := mgr.ListLibraries()
	if err != nil {
		return m
	}
	for _, l := range libs {
		store, err := mgr.OpenStore(l.ID)
		if err != nil {
			continue
		}
		refs, err := store.ListAllPhotoRefs()
		store.Close()
		if err != nil {
			continue
		}
		for _, ref := range refs {
			base := strings.ToLower(strings.TrimSuffix(filepath.Base(ref.PathHint), filepath.Ext(ref.PathHint)))
			if base != "" {
				m[base] = ref.ID
			}
		}
	}
	return m
}

// findPhotoSourcePath searches all library stores for photoID and returns its disk path.
func findPhotoSourcePath(mgr *lib.Manager, photoID string) (string, error) {
	libs, err := mgr.ListLibraries()
	if err != nil {
		return "", err
	}
	for _, l := range libs {
		store, err := mgr.OpenStore(l.ID)
		if err != nil {
			continue
		}
		path, storeErr := store.GetPhotoPathHint(photoID)
		store.Close()
		if storeErr == nil && path != "" {
			return path, nil
		}
	}
	return "", fmt.Errorf("photo %s not found in any library", photoID)
}

// sitePhotoHasMeta reports whether any library store confirms this photo is still
// built to the given channel+album. It checks the qualified key
// built:{channelSlug}:{albumPostID} first (written by current builds), then falls
// back to the unqualified built:{channelSlug} for legacy entries. It also checks the
// pre-rename published:{channelSlug}[:{albumPostID}] keys, for photos that haven't
// been rebuilt since the built: key prefix replaced published:.
func sitePhotoHasMeta(mgr *lib.Manager, photoID, channelSlug, albumPostID string) bool {
	libs, err := mgr.ListLibraries()
	if err != nil {
		return true // keep on error
	}
	qualKey := "built:" + channelSlug + ":" + albumPostID
	legacyKey := "built:" + channelSlug
	legacyQualKey := "published:" + channelSlug + ":" + albumPostID
	legacyUnqualKey := "published:" + channelSlug
	for _, l := range libs {
		store, err := mgr.OpenStore(l.ID)
		if err != nil {
			continue
		}
		entries, err := store.GetMeta(photoID)
		store.Close()
		if err != nil {
			continue
		}
		for _, e := range entries {
			if albumPostID != "" && (e.Key == qualKey || e.Key == legacyQualKey) {
				return true
			}
			if e.Key == legacyKey || e.Key == legacyUnqualKey {
				return true
			}
		}
	}
	return false
}
