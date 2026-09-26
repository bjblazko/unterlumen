package apilibrary

import (
	"net/http"
	"path/filepath"
	"sort"
	"strings"

	lib "huepattl.de/unterlumen/internal/library"
	"huepattl.de/unterlumen/internal/pathguard"
)

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
