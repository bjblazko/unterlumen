package apilibrary

import (
	"encoding/json"
	"net/http"
	"os"
	"strings"

	lib "huepattl.de/unterlumen/internal/library"
	"huepattl.de/unterlumen/internal/pathguard"
)

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

func updateLibrary(mgr *lib.Manager, root string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id := r.PathValue("id")
		var body struct {
			Name        string `json:"name"`
			Description string `json:"description"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			http.Error(w, "invalid JSON", http.StatusBadRequest)
			return
		}
		if body.Name == "" {
			http.Error(w, "name is required", http.StatusBadRequest)
			return
		}
		updated, err := mgr.UpdateLibrary(id, body.Name, body.Description)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		writeJSON(w, toLibraryJSON(updated, root, mgr.IsScanning(id)))
	}
}

func setLibraryOrder(mgr *lib.Manager) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var body struct {
			Order []string `json:"order"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			http.Error(w, "invalid JSON", http.StatusBadRequest)
			return
		}
		if err := mgr.SetLibrarySortOrder(body.Order); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	}
}

func getSettings(mgr *lib.Manager) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		s, err := mgr.GetSettings()
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		writeJSON(w, s)
	}
}

func patchSettings(mgr *lib.Manager) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		current, err := mgr.GetSettings()
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		var patch struct {
			LibrarySortMode *string `json:"librarySortMode"`
		}
		if err := json.NewDecoder(r.Body).Decode(&patch); err != nil {
			http.Error(w, "invalid JSON", http.StatusBadRequest)
			return
		}
		if patch.LibrarySortMode != nil {
			current.LibrarySortMode = *patch.LibrarySortMode
		}
		if err := mgr.SaveSettings(current); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		writeJSON(w, current)
	}
}
