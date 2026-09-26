package apilibrary

import (
	"bytes"
	"crypto/sha256"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"time"

	lib "huepattl.de/unterlumen/internal/library"
	"huepattl.de/unterlumen/internal/media"
)

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
