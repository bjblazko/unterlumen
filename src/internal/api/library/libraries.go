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
			mgr.Annotate(l)
			out[i] = toLibraryJSON(l, root, mgr.IsScanning(l.ID))
		}
		writeJSON(w, out)
	}
}

// libraryRef names a library for a link to it.
type libraryRef struct {
	ID   string `json:"id"`
	Name string `json:"name"`
}

// detectLibrary returns every library (id + name) whose source path covers
// the requested path — libraries may overlap — as {"libraries": [...]}, empty
// when none does.
func detectLibrary(mgr *lib.Manager, root string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		refs := []libraryRef{}
		if absPath, ok := pathguard.SafePath(root, r.URL.Query().Get("path")); ok {
			for _, l := range mgr.LibrariesForPath(absPath) {
				refs = append(refs, libraryRef{ID: l.ID, Name: l.Name})
			}
		}
		writeJSON(w, struct {
			Libraries []libraryRef `json:"libraries"`
		}{refs})
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
		created, err := createOrAddShared(mgr, body.Name, body.Description, absPath)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		w.WriteHeader(http.StatusCreated)
		writeJSON(w, toLibraryJSON(created, root, false))
	}
}

// createOrAddShared adds the library another installation shares in absPath
// under its identity, or creates a new one when the folder is not shared.
func createOrAddShared(mgr *lib.Manager, name, description, absPath string) (*lib.Library, error) {
	if mk, err := lib.ReadMarker(absPath); err == nil && mk != nil {
		return mgr.AddSharedLibrary(absPath)
	}
	return mgr.CreateLibrary(name, description, absPath)
}

func getLibrary(mgr *lib.Manager, root string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id := r.PathValue("id")
		l, err := mgr.GetLibrary(id)
		if err != nil {
			http.Error(w, err.Error(), http.StatusNotFound)
			return
		}
		mgr.Annotate(l)
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
