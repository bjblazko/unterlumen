// Package apilibrary provides HTTP handlers for the DAM library feature.
package apilibrary

import (
	"archive/zip"
	"bytes"
	"context"
	"crypto/rand"
	"encoding/json"
	"fmt"
	"image"
	_ "image/jpeg"
	_ "image/png"
	"io"
	"math"
	"net/http"
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
func Handle(mux *http.ServeMux, mgr *lib.Manager, root string, serverRole bool, chStore *channels.Store) {
	mux.HandleFunc("GET /api/library/", listLibraries(mgr, root))
	mux.HandleFunc("POST /api/library/", createLibrary(mgr, root))
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
	mux.HandleFunc("DELETE /api/library/{id}", deleteLibrary(mgr))
	mux.HandleFunc("POST /api/library/{id}/reindex", reindexLibrary(mgr))
	mux.HandleFunc("POST /api/library/{id}/scan-new", scanNewLibrary(mgr))
	mux.HandleFunc("POST /api/library/{id}/cleanup", cleanupLibrary(mgr))
	mux.HandleFunc("GET /api/library/{id}/browse", browseFolder(mgr, root))
	mux.HandleFunc("GET /api/library/{id}/browse-recursive", browseFolderRecursive(mgr))
	mux.HandleFunc("GET /api/library/{id}/folder-stats", libraryFolderStats(mgr))
	mux.HandleFunc("GET /api/library/{id}/photos", listPhotos(mgr))
	mux.HandleFunc("GET /api/library/{id}/exif-ranges", exifRanges(mgr))
	mux.HandleFunc("GET /api/library/{id}/thumb/{photoID}", serveThumb(mgr))
	mux.HandleFunc("GET /api/library/{id}/thumb-by-path", thumbByPath(mgr, root))
	mux.HandleFunc("GET /api/library/{id}/photo-id-by-path", photoIDByPath(mgr))
	mux.HandleFunc("GET /api/library/{id}/photo/{photoID}", servePhoto(mgr))
	mux.HandleFunc("GET /api/library/{id}/photo/{photoID}/info", photoInfo(mgr))
	mux.HandleFunc("GET /api/library/{id}/photo/{photoID}/meta", getMeta(mgr))
	mux.HandleFunc("PUT /api/library/{id}/photo/{photoID}/meta", upsertMeta(mgr))
	mux.HandleFunc("DELETE /api/library/{id}/photo/{photoID}/meta", deleteMeta(mgr))
	mux.HandleFunc("POST /api/library/{id}/publish", publishPhotos(mgr, chStore, root, serverRole))
	mux.HandleFunc("POST /api/library/{id}/publish-download", publishDownload(mgr, chStore))
	mux.HandleFunc("POST /api/channels/{slug}/rebuild-site", rebuildSite(chStore))
	mux.HandleFunc("GET /api/channels/{slug}/galleries", listGalleries(chStore))
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

// --- Indexing (SSE) ---

func reindexLibrary(mgr *lib.Manager) http.HandlerFunc {
	return libraryScan(mgr, func(idx *lib.Indexer, ch chan<- lib.Progress) {
		idx.Run(context.Background(), ch)
	})
}

func scanNewLibrary(mgr *lib.Manager) http.HandlerFunc {
	return libraryScan(mgr, func(idx *lib.Indexer, ch chan<- lib.Progress) {
		idx.RunScanNew(context.Background(), ch)
	})
}

func cleanupLibrary(mgr *lib.Manager) http.HandlerFunc {
	return libraryScan(mgr, func(idx *lib.Indexer, ch chan<- lib.Progress) {
		idx.RunCleanup(context.Background(), ch)
	})
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

		q := r.URL.Query()
		opts := lib.ListPhotosOpts{
			Filters:        parseTextFilters(q),
			NumericFilters: parseNumericFilters(q),
			DateMin:        q.Get("date_taken_min"),
			DateMax:        q.Get("date_taken_max"),
		}
		opts.Offset, _ = strconv.Atoi(q.Get("offset"))
		opts.Limit, _ = strconv.Atoi(q.Get("limit"))
		if opts.Limit <= 0 || opts.Limit > 500 {
			opts.Limit = 100
		}

		result, err := store.ListPhotos(opts)
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
		ids := parseIDList(q.Get("ids"))
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
			opts.MetaExists = []string{"published:" + ch}
		}
		opts.Offset, _ = strconv.Atoi(q.Get("offset"))
		opts.Limit, _ = strconv.Atoi(q.Get("limit"))
		if opts.Limit <= 0 || opts.Limit > 500 {
			opts.Limit = 100
		}
		result, err := mgr.SearchLibraries(ids, opts)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		writeJSON(w, result)
	}
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
		if keys == nil {
			keys = []string{}
		}
		writeJSON(w, keys)
	}
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

func servePhoto(mgr *lib.Manager) http.HandlerFunc {
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
			jpegData, convErr := media.ConvertHEIFToJPEG(r.Context(), pathHint)
			if convErr != nil {
				if _, statErr := os.Stat(pathHint); os.IsNotExist(statErr) {
					http.Error(w, "photo not found on disk", http.StatusNotFound)
				} else {
					http.Error(w, "Failed to convert HEIF: "+convErr.Error(), http.StatusInternalServerError)
				}
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
	Name     string       `json:"name"`
	Path     string       `json:"path"`
	Size     int64        `json:"size"`
	Format   string       `json:"format"`
	Modified string       `json:"modified"`
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
		writeJSON(w, entries)
	}
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
		w.WriteHeader(http.StatusNoContent)
	}
}

func deleteMeta(mgr *lib.Manager) http.HandlerFunc {
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

		if err := store.DeleteMeta(photoID, key); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	}
}

// --- Publish ---

type publishResult struct {
	PhotoID       string `json:"photoID"`
	OutputPath    string `json:"outputPath,omitempty"`
	Filename      string `json:"filename,omitempty"`
	ThumbFilename string `json:"thumbFilename,omitempty"`
	Width         int    `json:"width,omitempty"`
	Height        int    `json:"height,omitempty"`
	Error         string `json:"error,omitempty"`
}

func publishPhotos(mgr *lib.Manager, chStore *channels.Store, root string, serverRole bool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if chStore == nil {
			http.Error(w, "channel store not available", http.StatusServiceUnavailable)
			return
		}
		id := r.PathValue("id")

		var body struct {
			PhotoIDs     []string `json:"photoIDs"`
			Channel      string   `json:"channel"`
			Account      string   `json:"account"`
			PublishedAt  string   `json:"publishedAt"`
			GalleryTitle string   `json:"galleryTitle"`
			TargetPostID string   `json:"targetPostID,omitempty"` // non-empty = add to existing gallery/album
			RecordXMP    *bool    `json:"recordXMP,omitempty"`    // nil → true (default)
			OutputPath   string   `json:"outputPath,omitempty"`   // per-publish override
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
		if body.Account != "" && ch.AccountByID(body.Account) == nil {
			http.Error(w, "account not found: "+body.Account, http.StatusBadRequest)
			return
		}

		publishedAt := time.Now().UTC()
		if body.PublishedAt != "" {
			if t, parseErr := time.Parse(time.RFC3339, body.PublishedAt); parseErr == nil {
				publishedAt = t
			}
		}

		store, err := mgr.OpenStore(id)
		if err != nil {
			http.Error(w, "library not found", http.StatusNotFound)
			return
		}
		defer store.Close()

		postID := newPostID()
		pub := media.Publication{
			Channel:      body.Channel,
			Account:      body.Account,
			PostID:       postID,
			GalleryTitle: body.GalleryTitle,
			PublishedAt:  publishedAt,
		}
		ts := publishedAt.UTC().Format("20060102T150405Z")

		addToExisting := body.TargetPostID != ""
		galleryMode := ch.GalleryExport && (body.GalleryTitle != "" || addToExisting)
		siteMode := ch.SiteExport && (body.GalleryTitle != "" || addToExisting)
		channelDir := chStore.OutputDir(body.Channel)
		if body.OutputPath != "" {
			if serverRole && filepath.IsAbs(body.OutputPath) {
				http.Error(w, "absolute paths not allowed in server mode", http.StatusBadRequest)
				return
			}
			if !filepath.IsAbs(body.OutputPath) {
				safe, ok := pathguard.SafePath(root, body.OutputPath)
				if !ok {
					http.Error(w, "invalid output path", http.StatusBadRequest)
					return
				}
				channelDir = safe
			} else {
				channelDir = body.OutputPath
			}
		}
		// albumPostID is the album's key in site.json (immutable).
		// albumSlug is the human-readable folder name for site-mode albums.
		albumPostID := postID
		albumSlug := ""
		var existingPhotos []SitePhoto
		var existingTitle string
		var existingPublishedAt time.Time

		outDir := channelDir
		if addToExisting {
			albumPostID = body.TargetPostID
			if galleryMode {
				outDir = filepath.Join(channelDir, albumPostID)
				gs, gsErr := loadGalleryState(filepath.Join(outDir, "gallery.json"))
				if gsErr != nil || gs == nil {
					http.Error(w, "gallery not found: "+albumPostID, http.StatusBadRequest)
					return
				}
				existingPhotos = gs.Photos
				existingTitle = gs.Title
				existingPublishedAt = gs.PublishedAt
			} else if siteMode {
				siteAlbums, stateErr := loadSiteState(filepath.Join(channelDir, "site", "site.json"))
				if stateErr != nil {
					http.Error(w, "read site state: "+stateErr.Error(), http.StatusInternalServerError)
					return
				}
				for i := range siteAlbums {
					if siteAlbums[i].PostID == albumPostID {
						existingPhotos = siteAlbums[i].Photos
						existingTitle = siteAlbums[i].Title
						existingPublishedAt = siteAlbums[i].PublishedAt
						albumSlug = albumFolderName(siteAlbums[i])
						break
					}
				}
				if existingTitle == "" {
					http.Error(w, "album not found: "+albumPostID, http.StatusBadRequest)
					return
				}
				outDir = filepath.Join(channelDir, "site", "albums", albumSlug)
			}
			if _, statErr := os.Stat(outDir); os.IsNotExist(statErr) {
				http.Error(w, "gallery folder not found: "+albumPostID, http.StatusBadRequest)
				return
			}
		} else {
			if galleryMode {
				outDir = filepath.Join(outDir, albumPostID)
			} else if siteMode {
				// Compute a human-readable slug for the new album folder.
				existingAlbums, _ := loadSiteState(filepath.Join(channelDir, "site", "site.json"))
				albumSlug = computeSlug(body.GalleryTitle, publishedAt, existingAlbums)
				outDir = filepath.Join(channelDir, "site", "albums", albumSlug)
			}
		}
		if err := os.MkdirAll(outDir, 0o700); err != nil {
			http.Error(w, "create output dir: "+err.Error(), http.StatusInternalServerError)
			return
		}

		recordXMP := body.RecordXMP == nil || *body.RecordXMP

		if !galleryMode && !siteMode {
			// Fast synchronous path for regular (non-gallery) publishes.
			var results []publishResult
			for _, photoID := range body.PhotoIDs {
				res := publishOne(store, ch, pub, ts, outDir, "", photoID, recordXMP)
				results = append(results, res)
			}
			writeJSON(w, map[string]any{"postID": postID, "results": results})
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

		total := len(body.PhotoIDs)
		var results []publishResult
		for i, photoID := range body.PhotoIDs {
			res := publishOne(store, ch, pub, ts, outDir, thumbDir, photoID, recordXMP)
			results = append(results, res)
			emit(map[string]any{"step": "photo", "done": i + 1, "total": total, "file": res.Filename})
		}

		// Build merged items list: existing photos first, then newly exported.
		var items []GalleryItem
		for _, ep := range existingPhotos {
			items = append(items, GalleryItem{Filename: ep.Filename, ThumbFilename: ep.ThumbFilename})
		}
		for _, res := range results {
			if res.Error == "" && res.Filename != "" {
				items = append(items, GalleryItem{
					Filename:      res.Filename,
					ThumbFilename: res.ThumbFilename,
					Width:         res.Width,
					Height:        res.Height,
				})
			}
		}

		// For add-to-existing, include previous photos in the ZIP.
		var zipResults []publishResult
		for _, ep := range existingPhotos {
			zipResults = append(zipResults, publishResult{Filename: ep.Filename})
		}
		zipResults = append(zipResults, results...)

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
		galleryTitle := body.GalleryTitle
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
			})
		} else {
			html = GenerateGallery(galleryTitle, items, GalleryOptions{ZipFilename: zipName, DateStr: dateStr})
		}
		indexPath := filepath.Join(outDir, "index.html")
		if err := os.WriteFile(indexPath, html, 0o644); err != nil {
			emit(map[string]any{"error": "write gallery: " + err.Error()})
			return
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
				sitePhotos[i] = SitePhoto{Filename: item.Filename, ThumbFilename: item.ThumbFilename}
			}
			gs := &GalleryState{
				PostID:      albumPostID,
				Title:       galleryTitle,
				PublishedAt: gsPublishedAt,
				UpdatedAt:   gsUpdatedAt,
				PhotoCount:  len(items),
				HasZip:      zipName != "",
				Photos:      sitePhotos,
			}
			saveGalleryState(filepath.Join(outDir, "gallery.json"), gs) //nolint:errcheck
		}

		if siteMode && len(items) > 0 {
			emit(map[string]any{"step": "site", "done": 0, "total": 1, "file": "Updating site index…"})
			siteDir := filepath.Join(channelDir, "site")
			// Only update cover on initial publish (new album).
			if !addToExisting {
				if cover, rdErr := os.ReadFile(filepath.Join(outDir, items[0].ThumbFilename)); rdErr == nil {
					os.WriteFile(filepath.Join(outDir, "cover.jpg"), cover, 0o644) //nolint:errcheck
				}
			}
			if assetsErr := writeSiteAssets(filepath.Join(siteDir, "assets")); assetsErr != nil {
				emit(map[string]any{"error": "write site assets: " + assetsErr.Error()})
				return
			}
			statePath := filepath.Join(siteDir, "site.json")
			siteAlbums, _ := loadSiteState(statePath)
			sitePhotos := make([]SitePhoto, len(items))
			for i, item := range items {
				sitePhotos[i] = SitePhoto{Filename: item.Filename, ThumbFilename: item.ThumbFilename}
			}
			if addToExisting {
				// Update existing album entry; preserve PublishedAt for sort order.
				for i := range siteAlbums {
					if siteAlbums[i].PostID == albumPostID {
						siteAlbums[i].Photos = sitePhotos
						siteAlbums[i].PhotoCount = len(items)
						siteAlbums[i].HasZip = zipName != ""
						siteAlbums[i].UpdatedAt = publishedAt
						break
					}
				}
			} else {
				siteAlbums = append(siteAlbums, SiteAlbum{
					PostID:      albumPostID,
					Slug:        albumSlug,
					Title:       galleryTitle,
					PublishedAt: publishedAt,
					PhotoCount:  len(items),
					CoverFile:   "cover.jpg",
					HasZip:      zipName != "",
					Photos:      sitePhotos,
				})
			}
			if saveErr := saveSiteState(statePath, siteAlbums); saveErr != nil {
				emit(map[string]any{"error": "save site state: " + saveErr.Error()})
				return
			}
			siteHTML := GenerateSiteIndex(ch.SiteTitle, ch.SiteTheme, ch.SiteURL, siteAlbums)
			if writeErr := os.WriteFile(filepath.Join(siteDir, "index.html"), siteHTML, 0o644); writeErr != nil {
				emit(map[string]any{"error": "write site index: " + writeErr.Error()})
				return
			}
			generateRobotsTxt(siteDir, ch.SiteURL) //nolint:errcheck
			if ch.SiteURL != "" {
				generateSitemap(siteDir, siteAlbums, ch.SiteURL) //nolint:errcheck
			}
			emit(map[string]any{"step": "site", "done": 1, "total": 1, "file": "Site index updated"})
			emit(map[string]any{"complete": true, "postID": postID, "galleryPath": outDir, "sitePath": siteDir, "results": results})
		} else {
			emit(map[string]any{"complete": true, "postID": postID, "galleryPath": outDir, "results": results})
		}
	}
}

// publishDownload exports selected library photos with channel settings and delivers
// the result as a ZIP download. By default it does not update XMP sidecars or the
// library database; pass RecordXMP=true to opt in.
func publishDownload(mgr *lib.Manager, chStore *channels.Store) http.HandlerFunc {
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
				store.UpsertMeta(photoID, "published:"+pub.Channel, publishedAt.Format(time.RFC3339)) //nolint:errcheck
				if pub.PostID != "" {
					store.UpsertMeta(photoID, "published:"+pub.Channel+":postid", pub.PostID) //nolint:errcheck
				}
				if pub.GalleryTitle != "" {
					store.UpsertMeta(photoID, "published:"+pub.Channel+":title", pub.GalleryTitle) //nolint:errcheck
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

func createGalleryZip(results []publishResult, outDir, zipName string) error {
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

func publishOne(store *lib.Store, ch *channels.Channel, pub media.Publication, ts, outDir, thumbDir, photoID string, recordXMP bool) publishResult {
	pathHint, err := store.GetPhotoPathHint(photoID)
	if err != nil || pathHint == "" {
		return publishResult{PhotoID: photoID, Error: "photo not found"}
	}

	if recordXMP {
		if err := media.AppendPublication(pathHint, pub); err != nil {
			return publishResult{PhotoID: photoID, Error: "xmp: " + err.Error()}
		}
		metaVal := pub.PublishedAt.UTC().Format(time.RFC3339)
		store.UpsertMeta(photoID, "published:"+pub.Channel, metaVal) //nolint:errcheck
		if pub.Account != "" {
			store.UpsertMeta(photoID, "published:"+pub.Channel+":account", pub.Account) //nolint:errcheck
		}
		if pub.PostID != "" {
			store.UpsertMeta(photoID, "published:"+pub.Channel+":postid", pub.PostID) //nolint:errcheck
		}
		if pub.GalleryTitle != "" {
			store.UpsertMeta(photoID, "published:"+pub.Channel+":title", pub.GalleryTitle) //nolint:errcheck
		}
	}

	exported, err := media.ExportImage(pathHint, ch.ExportOptions())
	if err != nil {
		return publishResult{PhotoID: photoID, Error: "export: " + err.Error()}
	}

	ext := "." + ch.Format
	if ch.Format == "jpeg" {
		ext = ".jpg"
	}
	base := strings.TrimSuffix(filepath.Base(pathHint), filepath.Ext(pathHint))
	outName := ch.Slug + "_" + ts + "_" + base + ext
	outPath := filepath.Join(outDir, outName)

	if err := os.WriteFile(outPath, exported, 0o644); err != nil {
		return publishResult{PhotoID: photoID, Error: "write export: " + err.Error()}
	}

	res := publishResult{PhotoID: photoID, OutputPath: outPath, Filename: outName}
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
// Used when rebuilding albums that were published before photo metadata was stored in site.json.
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

// galleryListItem is the JSON shape returned by GET /api/channels/{slug}/galleries.
type galleryListItem struct {
	PostID      string    `json:"postID"`
	Title       string    `json:"title"`
	PublishedAt time.Time `json:"publishedAt"`
	UpdatedAt   time.Time `json:"updatedAt,omitempty"`
	PhotoCount  int       `json:"photoCount"`
}

// listGalleries returns existing published galleries/albums for a channel.
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
		channelDir := chStore.OutputDir(slug)
		var items []galleryListItem

		switch {
		case ch.SiteExport:
			albums, err := loadSiteState(filepath.Join(channelDir, "site", "site.json"))
			if err != nil {
				http.Error(w, "read site state: "+err.Error(), http.StatusInternalServerError)
				return
			}
			for _, a := range albums {
				items = append(items, galleryListItem{
					PostID:      a.PostID,
					Title:       a.Title,
					PublishedAt: a.PublishedAt,
					UpdatedAt:   a.UpdatedAt,
					PhotoCount:  a.PhotoCount,
				})
			}
			// Sort newest first (site index sorts the same way).
			sort.Slice(items, func(i, j int) bool {
				return items[i].PublishedAt.After(items[j].PublishedAt)
			})
		case ch.GalleryExport:
			entries, rdErr := os.ReadDir(channelDir)
			if rdErr != nil && !os.IsNotExist(rdErr) {
				http.Error(w, "read channel dir: "+rdErr.Error(), http.StatusInternalServerError)
				return
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
				})
			}
			sort.Slice(items, func(i, j int) bool {
				return items[i].PublishedAt.After(items[j].PublishedAt)
			})
		}

		if items == nil {
			items = []galleryListItem{}
		}
		writeJSON(w, items)
	}
}

// rebuildSite regenerates site assets and root index from the existing site.json statefile.
func rebuildSite(chStore *channels.Store) http.HandlerFunc {
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

		siteDir := filepath.Join(chStore.OutputDir(channelSlug), "site")
		albums, err := loadSiteState(filepath.Join(siteDir, "site.json"))
		if err != nil {
			http.Error(w, "read site state: "+err.Error(), http.StatusInternalServerError)
			return
		}

		if err := writeSiteAssets(filepath.Join(siteDir, "assets")); err != nil {
			http.Error(w, "write site assets: "+err.Error(), http.StatusInternalServerError)
			return
		}

		// Assign slugs to any albums that don't have one yet (migration for pre-slug albums).
		// Only renames the statefile entry; does not move the disk folder (backward compat).
		stateDirty := false
		for i := range albums {
			if albums[i].Slug == "" {
				others := make([]SiteAlbum, 0, len(albums)-1)
				others = append(others, albums[:i]...)
				others = append(others, albums[i+1:]...)
				albums[i].Slug = computeSlug(albums[i].Title, albums[i].PublishedAt, others)
				stateDirty = true
			}
		}
		if stateDirty {
			saveSiteState(filepath.Join(siteDir, "site.json"), albums) //nolint:errcheck
		}

		// Regenerate every album page so data-default-theme reflects the current channel setting.
		for _, album := range albums {
			albumDir := filepath.Join(siteDir, "albums", albumFolderName(album))
			items := buildGalleryItems(album.Photos)
			if len(items) == 0 {
				// Albums published before photo list was stored: reconstruct from disk.
				items = scanAlbumPhotos(albumDir)
			}
			if len(items) == 0 {
				continue
			}
			// Detect ZIP even for albums that predate the HasZip field.
			zipName := ""
			if album.HasZip {
				zipName = "photos.zip"
			} else if _, err := os.Stat(filepath.Join(albumDir, "photos.zip")); err == nil {
				zipName = "photos.zip"
			}
			dateStr := dateRangeStr(album.PublishedAt, album.UpdatedAt)
			albumHTML := GenerateSiteGallery(album.Title, ch.SiteTheme, items, GalleryOptions{
				ZipFilename: zipName,
				SiteTitle:   ch.SiteTitle,
				DateStr:     dateStr,
				SiteURL:     ch.SiteURL,
				AlbumSlug:   albumFolderName(album),
				PublishedAt: album.PublishedAt,
			})
			os.WriteFile(filepath.Join(albumDir, "index.html"), albumHTML, 0o644) //nolint:errcheck
		}

		siteHTML := GenerateSiteIndex(ch.SiteTitle, ch.SiteTheme, ch.SiteURL, albums)
		if err := os.WriteFile(filepath.Join(siteDir, "index.html"), siteHTML, 0o644); err != nil {
			http.Error(w, "write site index: "+err.Error(), http.StatusInternalServerError)
			return
		}
		generateRobotsTxt(siteDir, ch.SiteURL) //nolint:errcheck
		if ch.SiteURL != "" {
			generateSitemap(siteDir, albums, ch.SiteURL) //nolint:errcheck
		}

		writeJSON(w, map[string]any{"sitePath": siteDir, "albumCount": len(albums)})
	}
}
