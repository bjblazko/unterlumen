package api

import (
	"encoding/json"
	"errors"
	"net/http"
	"os/exec"
	"runtime"

	"huepattl.de/unterlumen/internal/library"
	"huepattl.de/unterlumen/internal/media"
)

// Storage in Settings: what the cache and the data folder hold, part by
// part; clearing one part of the cache; the cache's limit; and, in the
// installed app, the folder shown in the file manager.

// startCacheUpkeep sets the cache's limit from the settings and removes
// what older versions left in it.
func startCacheUpkeep(libMgr *library.Manager) {
	limit := library.DefaultCacheLimit
	if libMgr != nil {
		if s, err := libMgr.GetSettings(); err == nil {
			limit = s.CacheLimitBytes()
		}
	}
	media.SetCacheLimit(limit)
	go media.RemoveOlderCache()
}

func handleStorage(libMgr *library.Manager, canReveal bool) http.HandlerFunc {
	type cache struct {
		Path     string               `json:"path"`
		Limit    int64                `json:"limit"`
		Segments []media.CacheSegment `json:"segments"`
	}
	return func(w http.ResponseWriter, r *http.Request) {
		resp := struct {
			Cache     cache                `json:"cache"`
			Data      *library.DataStorage `json:"data,omitempty"`
			CanReveal bool                 `json:"canReveal"`
		}{cache{media.GetCacheDir(), media.CacheLimit(), media.CacheSegments()}, nil, canReveal}
		if libMgr != nil {
			ds := libMgr.DataStorage()
			resp.Data = &ds
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp) //nolint:errcheck
	}
}

func handleStorageClear() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var body struct {
			Segment string `json:"segment"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil || body.Segment == "" {
			http.Error(w, "Say which part of the cache to clear.", http.StatusBadRequest)
			return
		}
		switch err := media.ClearCache(body.Segment); {
		case errors.Is(err, media.ErrNoSuchSegment):
			http.Error(w, "The cache has no such part.", http.StatusBadRequest)
		case err != nil:
			http.Error(w, "The cache could not be cleared: "+err.Error(), http.StatusInternalServerError)
		default:
			w.WriteHeader(http.StatusNoContent)
		}
	}
}

// handleStorageLimit sets the cache's limit in bytes, 0 for none, and keeps
// it in the installation's settings.
func handleStorageLimit(libMgr *library.Manager) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var body struct {
			Bytes *int64 `json:"bytes"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil || body.Bytes == nil || *body.Bytes < 0 {
			http.Error(w, "A limit in bytes is needed; 0 for none.", http.StatusBadRequest)
			return
		}
		if libMgr != nil {
			s, err := libMgr.GetSettings()
			if err == nil {
				s.CacheLimit = body.Bytes
				err = libMgr.SaveSettings(s)
			}
			if err != nil {
				http.Error(w, "The limit could not be saved: "+err.Error(), http.StatusInternalServerError)
				return
			}
		}
		media.SetCacheLimit(*body.Bytes)
		w.WriteHeader(http.StatusNoContent)
	}
}

// handleStorageReveal opens the cache or the data folder in the file
// manager of the computer the app runs on — only the installed app, whose
// computer is the owner's.
func handleStorageReveal(libMgr *library.Manager) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var body struct {
			Folder string `json:"folder"`
		}
		json.NewDecoder(r.Body).Decode(&body) //nolint:errcheck
		dir := media.GetCacheDir()
		if body.Folder == "data" && libMgr != nil {
			dir = libMgr.DataStorage().Path
		}
		name := map[string]string{"darwin": "open", "windows": "explorer"}[runtime.GOOS]
		if name == "" {
			name = "xdg-open"
		}
		if err := exec.Command(name, dir).Start(); err != nil {
			http.Error(w, "The folder could not be opened: "+err.Error(), http.StatusInternalServerError)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	}
}
