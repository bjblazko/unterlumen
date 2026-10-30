package apilibrary

import (
	"net/http"
	"os"
	"path/filepath"

	"huepattl.de/unterlumen/internal/api/download"
	"huepattl.de/unterlumen/internal/api/heifjpeg"
	"huepattl.de/unterlumen/internal/api/videoclip"
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
		pathHint, ok := photoPath(w, mgr, r)
		if !ok {
			return
		}

		if download.Requested(r) {
			download.Original(w, r, pathHint)
			return
		}

		if media.IsHEIF(pathHint) {
			heifjpeg.Serve(w, r, pathHint, imgCache)
			return
		}
		if media.IsVideo(pathHint) && media.VideoContentType(pathHint) == "" {
			download.Original(w, r, pathHint) // a video the browser cannot play
			return
		}

		http.ServeFile(w, r, pathHint)
	}
}

// serveClip serves a video's short silent clip, the one its tile plays
// under the pointer (ADR-0058).
func serveClip(mgr *lib.Manager) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		pathHint, ok := photoPath(w, mgr, r)
		if !ok {
			return
		}
		if !media.IsVideo(pathHint) {
			http.Error(w, "not a video", http.StatusBadRequest)
			return
		}
		videoclip.Serve(w, r, pathHint)
	}
}

// serveLive serves the clip of a Live Photo, the .mov beside it.
func serveLive(mgr *lib.Manager) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		pathHint, ok := photoPath(w, mgr, r)
		if !ok {
			return
		}
		clip, found := media.LiveClips(filepath.Dir(pathHint))[filepath.Base(pathHint)]
		if !found {
			http.Error(w, "this photo has no Live clip", http.StatusNotFound)
			return
		}
		w.Header().Set("Content-Type", "video/quicktime")
		http.ServeFile(w, r, clip)
	}
}

// photoPath is the file of the request's photo, or an error written.
func photoPath(w http.ResponseWriter, mgr *lib.Manager, r *http.Request) (string, bool) {
	store, err := mgr.OpenStore(r.PathValue("id"))
	if err != nil {
		http.Error(w, "library not found", http.StatusNotFound)
		return "", false
	}
	defer store.Close()
	pathHint, err := store.GetPhotoPathHint(r.PathValue("photoID"))
	if err != nil || pathHint == "" {
		http.Error(w, "photo not found", http.StatusNotFound)
		return "", false
	}
	return pathHint, true
}
