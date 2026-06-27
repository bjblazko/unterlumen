// Package apichannels provides HTTP handlers for managing publish channels.
package apichannels

import (
	"encoding/json"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"

	"huepattl.de/unterlumen/internal/channels"
	"huepattl.de/unterlumen/internal/media"
)

var slugRe = regexp.MustCompile(`^[a-z0-9][a-z0-9-]{0,61}[a-z0-9]$|^[a-z0-9]$`)

// Handle registers all channel API routes on mux.
func Handle(mux *http.ServeMux, store *channels.Store) {
	mux.HandleFunc("GET /api/channels/", listChannels(store))
	mux.HandleFunc("POST /api/channels/", createChannel(store))
	mux.HandleFunc("PUT /api/channels/{slug}", updateChannel(store))
	mux.HandleFunc("DELETE /api/channels/{slug}", deleteChannel(store))
	mux.HandleFunc("GET /api/channels/{slug}/path", channelPath(store))
	mux.HandleFunc("POST /api/channels/{slug}/reveal", revealChannel(store))
	mux.HandleFunc("GET /api/channels/{slug}/avatar", avatarStatus(store))
	mux.HandleFunc("POST /api/channels/{slug}/avatar", uploadAvatar(store))
	mux.HandleFunc("DELETE /api/channels/{slug}/avatar", deleteAvatar(store))
	mux.HandleFunc("GET /api/channels/{slug}/logo", logoStatus(store))
	mux.HandleFunc("POST /api/channels/{slug}/logo", uploadLogo(store))
	mux.HandleFunc("DELETE /api/channels/{slug}/logo", deleteLogo(store))
}

func writeJSON(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(v)
}

func listChannels(store *channels.Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		chs, err := store.List()
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		writeJSON(w, chs)
	}
}

func createChannel(store *channels.Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var ch channels.Channel
		if err := json.NewDecoder(r.Body).Decode(&ch); err != nil {
			http.Error(w, "invalid JSON", http.StatusBadRequest)
			return
		}
		ch.Slug = strings.ToLower(strings.ReplaceAll(strings.TrimSpace(ch.Slug), " ", "-"))
		if !slugRe.MatchString(ch.Slug) || ch.Name == "" {
			http.Error(w, "valid slug and name are required", http.StatusBadRequest)
			return
		}
		if _, err := store.Get(ch.Slug); err == nil {
			http.Error(w, "channel slug already exists", http.StatusConflict)
			return
		}
		if err := store.Save(&ch); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		w.WriteHeader(http.StatusCreated)
		writeJSON(w, &ch)
	}
}

func updateChannel(store *channels.Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		slug := r.PathValue("slug")
		var ch channels.Channel
		if err := json.NewDecoder(r.Body).Decode(&ch); err != nil {
			http.Error(w, "invalid JSON", http.StatusBadRequest)
			return
		}
		ch.Slug = slug // slug is immutable; always use URL param
		if ch.Name == "" {
			http.Error(w, "name is required", http.StatusBadRequest)
			return
		}
		if err := store.Save(&ch); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		writeJSON(w, &ch)
	}
}

func deleteChannel(store *channels.Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		slug := r.PathValue("slug")
		if err := store.Delete(slug); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	}
}

func channelPath(store *channels.Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		slug := r.PathValue("slug")
		writeJSON(w, map[string]any{"path": store.OutputDir(slug)})
	}
}

func revealChannel(store *channels.Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		slug := r.PathValue("slug")
		dir := store.OutputDir(slug)
		os.MkdirAll(dir, 0o700) //nolint:errcheck
		var cmd *exec.Cmd
		switch runtime.GOOS {
		case "darwin":
			cmd = exec.Command("open", dir)
		case "windows":
			cmd = exec.Command("explorer", dir)
		default:
			cmd = exec.Command("xdg-open", dir)
		}
		cmd.Start() //nolint:errcheck
		w.WriteHeader(http.StatusNoContent)
	}
}

func avatarPath(store *channels.Store, slug string) string {
	return filepath.Join(store.OutputDir(slug), "site", "assets", "avatar.jpg")
}

func logoPath(store *channels.Store, slug string) string {
	return filepath.Join(store.OutputDir(slug), "site", "assets", "logo.jpg")
}

func avatarStatus(store *channels.Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		slug := r.PathValue("slug")
		_, err := os.Stat(avatarPath(store, slug))
		writeJSON(w, map[string]any{"exists": err == nil})
	}
}

func uploadAvatar(store *channels.Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		slug := r.PathValue("slug")
		const maxSize = 30 << 20 // 30 MB — large source photos are downscaled server-side
		r.Body = http.MaxBytesReader(w, r.Body, maxSize)
		if err := r.ParseMultipartForm(maxSize); err != nil {
			http.Error(w, "file too large or invalid form", http.StatusBadRequest)
			return
		}
		f, _, err := r.FormFile("file")
		if err != nil {
			http.Error(w, "missing file field", http.StatusBadRequest)
			return
		}
		defer f.Close()
		raw, err := io.ReadAll(f)
		if err != nil {
			http.Error(w, "read error", http.StatusInternalServerError)
			return
		}
		data, err := media.ScaleImageToJPEG(raw, 800, 85)
		if err != nil {
			http.Error(w, "image processing failed: "+err.Error(), http.StatusBadRequest)
			return
		}
		dest := avatarPath(store, slug)
		if err := os.MkdirAll(filepath.Dir(dest), 0o700); err != nil {
			http.Error(w, "mkdir error", http.StatusInternalServerError)
			return
		}
		if err := os.WriteFile(dest, data, 0o644); err != nil {
			http.Error(w, "write error", http.StatusInternalServerError)
			return
		}
		writeJSON(w, map[string]any{"ok": true})
	}
}

func deleteAvatar(store *channels.Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		slug := r.PathValue("slug")
		path := avatarPath(store, slug)
		if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	}
}

func logoStatus(store *channels.Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		slug := r.PathValue("slug")
		_, err := os.Stat(logoPath(store, slug))
		writeJSON(w, map[string]any{"exists": err == nil})
	}
}

func uploadLogo(store *channels.Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		slug := r.PathValue("slug")
		const maxSize = 30 << 20 // 30 MB — large source images are downscaled server-side
		r.Body = http.MaxBytesReader(w, r.Body, maxSize)
		if err := r.ParseMultipartForm(maxSize); err != nil {
			http.Error(w, "file too large or invalid form", http.StatusBadRequest)
			return
		}
		f, _, err := r.FormFile("file")
		if err != nil {
			http.Error(w, "missing file field", http.StatusBadRequest)
			return
		}
		defer f.Close()
		raw, err := io.ReadAll(f)
		if err != nil {
			http.Error(w, "read error", http.StatusInternalServerError)
			return
		}
		data, err := media.ScaleImageToJPEG(raw, 800, 85)
		if err != nil {
			http.Error(w, "image processing failed: "+err.Error(), http.StatusBadRequest)
			return
		}
		dest := logoPath(store, slug)
		if err := os.MkdirAll(filepath.Dir(dest), 0o700); err != nil {
			http.Error(w, "mkdir error", http.StatusInternalServerError)
			return
		}
		if err := os.WriteFile(dest, data, 0o644); err != nil {
			http.Error(w, "write error", http.StatusInternalServerError)
			return
		}
		writeJSON(w, map[string]any{"ok": true})
	}
}

func deleteLogo(store *channels.Store) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		slug := r.PathValue("slug")
		path := logoPath(store, slug)
		if err := os.Remove(path); err != nil && !os.IsNotExist(err) {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	}
}
