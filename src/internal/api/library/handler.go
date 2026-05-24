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
	"math"
	"net/http"
	"os"
	"path/filepath"
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
// root is the browse boundary directory; used to resolve relative paths for thumb-by-path lookups.
func Handle(mux *http.ServeMux, mgr *lib.Manager, root string, chStore *channels.Store) {
	mux.HandleFunc("GET /api/library/", listLibraries(mgr, root))
	mux.HandleFunc("POST /api/library/", createLibrary(mgr, root))
	mux.HandleFunc("GET /api/library/search", searchLibraries(mgr))
	mux.HandleFunc("GET /api/library/exif-ranges", globalExifRanges(mgr))
	mux.HandleFunc("GET /api/library/exif-values", globalExifValues(mgr))
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
	mux.HandleFunc("POST /api/library/{id}/publish", publishPhotos(mgr, chStore))
	mux.HandleFunc("POST /api/channels/{slug}/rebuild-site", rebuildSite(chStore))
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
		absPath, err := filepath.Abs(body.SourcePath)
		if err != nil {
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
		absPath, ok := pathguard.SafePath(sourcePath, relPath)
		if !ok {
			http.Error(w, "invalid path", http.StatusBadRequest)
			return
		}

		fi, statErr := os.Stat(absPath)
		if statErr != nil {
			http.Error(w, "path not found", http.StatusNotFound)
			return
		}
		if !fi.IsDir() {
			http.Error(w, "not a directory", http.StatusBadRequest)
			return
		}

		stats, err := media.WalkFolderStats(absPath, relPath)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
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

		filters := make(map[string]string)
		for k, vals := range r.URL.Query() {
			if k == "offset" || k == "limit" || k == "ids" || len(vals) == 0 {
				continue
			}
			if strings.HasSuffix(k, "_min") || strings.HasSuffix(k, "_max") {
				continue
			}
			if vals[0] != "" {
				filters[k] = vals[0]
			}
		}
		numericFilters := parseNumericFilters(r.URL.Query())

		offset, _ := strconv.Atoi(r.URL.Query().Get("offset"))
		limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
		if limit <= 0 || limit > 500 {
			limit = 100
		}

		result, err := store.ListPhotos(filters, numericFilters, offset, limit)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		writeJSON(w, result)
	}
}

func searchLibraries(mgr *lib.Manager) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ids := parseIDList(r.URL.Query().Get("ids"))
		textFilters := parseTextFilters(r.URL.Query())
		numericFilters := parseNumericFilters(r.URL.Query())
		offset, _ := strconv.Atoi(r.URL.Query().Get("offset"))
		limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
		if limit <= 0 || limit > 500 {
			limit = 100
		}
		result, err := mgr.SearchLibraries(ids, textFilters, numericFilters, offset, limit)
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
// Reserved params: q, offset, limit, ids, and any _min/_max numeric params.
func parseTextFilters(vals map[string][]string) map[string]string {
	out := make(map[string]string)
	for k, vs := range vals {
		if k == "q" || k == "offset" || k == "limit" || k == "ids" || len(vs) == 0 {
			continue
		}
		if strings.HasSuffix(k, "_min") || strings.HasSuffix(k, "_max") {
			continue
		}
		if vs[0] != "" {
			out[k] = vs[0]
		}
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

func publishPhotos(mgr *lib.Manager, chStore *channels.Store) http.HandlerFunc {
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
			Channel:     body.Channel,
			Account:     body.Account,
			PostID:      postID,
			PublishedAt: publishedAt,
		}
		ts := publishedAt.UTC().Format("20060102T150405Z")

		galleryMode := ch.GalleryExport && body.GalleryTitle != ""
		siteMode := ch.SiteExport && body.GalleryTitle != ""
		channelDir := chStore.OutputDir(body.Channel)
		outDir := channelDir
		if galleryMode {
			outDir = filepath.Join(outDir, postID)
		} else if siteMode {
			outDir = filepath.Join(channelDir, "site", "albums", postID)
		}
		if err := os.MkdirAll(outDir, 0o700); err != nil {
			http.Error(w, "create output dir: "+err.Error(), http.StatusInternalServerError)
			return
		}

		if !galleryMode && !siteMode {
			// Fast synchronous path for regular (non-gallery) publishes.
			var results []publishResult
			for _, photoID := range body.PhotoIDs {
				res := publishOne(store, ch, pub, ts, outDir, "", photoID)
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
			res := publishOne(store, ch, pub, ts, outDir, thumbDir, photoID)
			results = append(results, res)
			emit(map[string]any{"step": "photo", "done": i + 1, "total": total, "file": res.Filename})
		}

		// ZIP of full-res photos.
		emit(map[string]any{"step": "zip", "done": 0, "total": 1, "file": "Creating ZIP…"})
		zipName := "photos.zip"
		if zipErr := createGalleryZip(results, outDir, zipName); zipErr != nil {
			emit(map[string]any{"step": "zip", "done": 0, "total": 1, "file": "ZIP failed: " + zipErr.Error()})
			zipName = ""
		} else {
			emit(map[string]any{"step": "zip", "done": 1, "total": 1, "file": "ZIP ready"})
		}

		// Generate HTML gallery.
		emit(map[string]any{"step": "html", "done": 0, "total": 1, "file": "Generating gallery…"})
		var items []GalleryItem
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
		var html []byte
		if siteMode {
			html = GenerateSiteGallery(body.GalleryTitle, ch.SiteTheme, items, GalleryOptions{ZipFilename: zipName})
		} else {
			html = GenerateGallery(body.GalleryTitle, items, GalleryOptions{ZipFilename: zipName})
		}
		indexPath := filepath.Join(outDir, "index.html")
		if err := os.WriteFile(indexPath, html, 0o644); err != nil {
			emit(map[string]any{"error": "write gallery: " + err.Error()})
			return
		}

		if siteMode && len(items) > 0 {
			emit(map[string]any{"step": "site", "done": 0, "total": 1, "file": "Updating site index…"})
			siteDir := filepath.Join(channelDir, "site")
			if cover, rdErr := os.ReadFile(filepath.Join(outDir, items[0].ThumbFilename)); rdErr == nil {
				os.WriteFile(filepath.Join(outDir, "cover.jpg"), cover, 0o644) //nolint:errcheck
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
			siteAlbums = append(siteAlbums, SiteAlbum{
				PostID:      postID,
				Title:       body.GalleryTitle,
				PublishedAt: publishedAt,
				PhotoCount:  len(items),
				CoverFile:   "cover.jpg",
				HasZip:      zipName != "",
				Photos:      sitePhotos,
			})
			if saveErr := saveSiteState(statePath, siteAlbums); saveErr != nil {
				emit(map[string]any{"error": "save site state: " + saveErr.Error()})
				return
			}
			siteHTML := GenerateSiteIndex(ch.SiteTitle, ch.SiteTheme, siteAlbums)
			if writeErr := os.WriteFile(filepath.Join(siteDir, "index.html"), siteHTML, 0o644); writeErr != nil {
				emit(map[string]any{"error": "write site index: " + writeErr.Error()})
				return
			}
			emit(map[string]any{"step": "site", "done": 1, "total": 1, "file": "Site index updated"})
			emit(map[string]any{"complete": true, "postID": postID, "galleryPath": outDir, "sitePath": siteDir, "results": results})
		} else {
			emit(map[string]any{"complete": true, "postID": postID, "galleryPath": outDir, "results": results})
		}
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

func publishOne(store *lib.Store, ch *channels.Channel, pub media.Publication, ts, outDir, thumbDir, photoID string) publishResult {
	pathHint, err := store.GetPhotoPathHint(photoID)
	if err != nil || pathHint == "" {
		return publishResult{PhotoID: photoID, Error: "photo not found"}
	}

	if err := media.AppendPublication(pathHint, pub); err != nil {
		return publishResult{PhotoID: photoID, Error: "xmp: " + err.Error()}
	}

	metaVal := pub.PublishedAt.UTC().Format(time.RFC3339)
	store.UpsertMeta(photoID, "published:"+pub.Channel, metaVal)           //nolint:errcheck
	if pub.Account != "" {
		store.UpsertMeta(photoID, "published:"+pub.Channel+":account", pub.Account) //nolint:errcheck
	}
	if pub.PostID != "" {
		store.UpsertMeta(photoID, "published:"+pub.Channel+":postid", pub.PostID) //nolint:errcheck
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

		// Regenerate every album page so data-default-theme reflects the current channel setting.
		for _, album := range albums {
			albumDir := filepath.Join(siteDir, "albums", album.PostID)
			items := make([]GalleryItem, len(album.Photos))
			for i, p := range album.Photos {
				items[i] = GalleryItem{Filename: p.Filename, ThumbFilename: p.ThumbFilename}
			}
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
			albumHTML := GenerateSiteGallery(album.Title, ch.SiteTheme, items, GalleryOptions{ZipFilename: zipName})
			os.WriteFile(filepath.Join(albumDir, "index.html"), albumHTML, 0o644) //nolint:errcheck
		}

		siteHTML := GenerateSiteIndex(ch.SiteTitle, ch.SiteTheme, albums)
		if err := os.WriteFile(filepath.Join(siteDir, "index.html"), siteHTML, 0o644); err != nil {
			http.Error(w, "write site index: "+err.Error(), http.StatusInternalServerError)
			return
		}

		writeJSON(w, map[string]any{"sitePath": siteDir, "albumCount": len(albums)})
	}
}
