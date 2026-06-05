package api

import (
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"

	"huepattl.de/unterlumen/internal/media"
)

func handleCacheInfo() http.HandlerFunc {
	type response struct {
		Path  string `json:"path"`
		Bytes int64  `json:"bytes"`
	}
	return func(w http.ResponseWriter, r *http.Request) {
		dir := media.GetCacheDir()
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(response{Path: dir, Bytes: cacheSize(dir)})
	}
}

func handleCacheClear() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		dir := media.GetCacheDir()
		entries, err := os.ReadDir(dir)
		if err != nil {
			http.Error(w, "failed to read cache dir", http.StatusInternalServerError)
			return
		}
		for _, e := range entries {
			if !e.IsDir() {
				os.Remove(filepath.Join(dir, e.Name()))
			}
		}
		w.WriteHeader(http.StatusNoContent)
	}
}

func handleCacheEvict(boundary string) http.HandlerFunc {
	type request struct {
		Paths []string `json:"paths"`
	}
	type response struct {
		Evicted int `json:"evicted"`
	}
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		var req request
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "invalid request", http.StatusBadRequest)
			return
		}

		var evicted int
		for _, p := range req.Paths {
			clean := filepath.Clean(p)
			if !filepath.IsAbs(clean) {
				clean = filepath.Join(boundary, clean)
			}
			info, err := os.Stat(clean)
			if err != nil || !info.IsDir() {
				media.EvictFile(clean)
				evicted++
			} else {
				filepath.WalkDir(clean, func(path string, d os.DirEntry, err error) error {
					if err == nil && !d.IsDir() {
						media.EvictFile(path)
						evicted++
					}
					return nil
				})
			}
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(response{Evicted: evicted})
	}
}

func cacheSize(dir string) int64 {
	var total int64
	filepath.Walk(dir, func(_ string, info os.FileInfo, err error) error {
		if err == nil && !info.IsDir() {
			total += info.Size()
		}
		return nil
	})
	return total
}
