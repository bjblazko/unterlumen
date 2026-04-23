package api

import (
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"huepattl.de/unterlumen/internal/media"
)

type infoResponse struct {
	Name     string          `json:"name"`
	Path     string          `json:"path"`
	Size     int64           `json:"size"`
	Modified string          `json:"modified"`
	Format   string          `json:"format"`
	Exif     *media.ExifData `json:"exif,omitempty"`
}

func handleInfo(root string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		relPath := r.URL.Query().Get("path")
		if relPath == "" {
			http.Error(w, "Missing path parameter", http.StatusBadRequest)
			return
		}

		absPath, ok := safePath(root, relPath)
		if !ok {
			http.Error(w, "Invalid path", http.StatusBadRequest)
			return
		}

		info, err := os.Stat(absPath)
		if err != nil {
			http.Error(w, "File not found", http.StatusNotFound)
			return
		}

		ext := strings.TrimPrefix(strings.ToLower(filepath.Ext(absPath)), ".")

		resp := infoResponse{
			Name:     info.Name(),
			Path:     relPath,
			Size:     info.Size(),
			Modified: info.ModTime().UTC().Format("2006-01-02T15:04:05Z"),
			Format:   ext,
		}

		// Best-effort EXIF extraction
		exifData, err := media.ExtractAllEXIF(absPath)
		if err == nil && exifData != nil {
			resp.Exif = exifData
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}
}
