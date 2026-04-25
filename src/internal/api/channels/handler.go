// Package apichannels provides HTTP handlers for managing publish channels.
package apichannels

import (
	"encoding/json"
	"net/http"
	"regexp"
	"strings"

	"huepattl.de/unterlumen/internal/channels"
)

var slugRe = regexp.MustCompile(`^[a-z0-9][a-z0-9-]{0,61}[a-z0-9]$|^[a-z0-9]$`)

// Handle registers all channel API routes on mux.
func Handle(mux *http.ServeMux, store *channels.Store) {
	mux.HandleFunc("GET /api/channels/", listChannels(store))
	mux.HandleFunc("POST /api/channels/", createChannel(store))
	mux.HandleFunc("PUT /api/channels/{slug}", updateChannel(store))
	mux.HandleFunc("DELETE /api/channels/{slug}", deleteChannel(store))
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
