// Package apisetup serves the setup place (#setup): the photo folder, the
// data folder and whether destinations are shared, saved to config.json and
// applied without a restart (ADR-0042).
package apisetup

import (
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"

	"huepattl.de/unterlumen/internal/installation"
)

// Installation is the running installation as the setup sees it.
type Installation struct {
	// Saved is config.json as it is now.
	Saved installation.Config
	// ChannelsDir is where destinations are kept now, shared or not.
	ChannelsDir string
	// DefaultLibDir is the data folder used when none is chosen.
	DefaultLibDir string
	// Problem says why the saved photo folder is not in use, or is "".
	Problem string
	// Apply saves a configuration and starts using it.
	Apply func(installation.Config) error
}

// Handle registers the setup routes. current is read on every request,
// because Apply replaces the installation it describes.
func Handle(mux *http.ServeMux, current func() Installation) {
	mux.HandleFunc("GET /api/setup", getSetup(current))
	mux.HandleFunc("POST /api/setup", postSetup(current))
	mux.HandleFunc("GET /api/setup/shared", getShared())
	mux.HandleFunc("GET /api/setup/dirs", listDirs())
}

type setupResponse struct {
	PhotosDir     string `json:"photosDir"`
	PhotosPath    string `json:"photosPath"` // the photo folder as the folder picker names it
	LibDir        string `json:"libDir"`
	DefaultLibDir string `json:"defaultLibDir"`
	HomePath      string `json:"homePath"`
	SharedDir     string `json:"sharedDir"` // where destinations are shared, "" when they are not
	Problem       string `json:"problem"`
}

func getSetup(current func() Installation) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		inst := current()
		resp := setupResponse{
			PhotosDir:     inst.Saved.PhotosDir,
			LibDir:        inst.Saved.LibDir,
			DefaultLibDir: inst.DefaultLibDir,
			Problem:       inst.Problem,
		}
		if inst.Saved.PhotosDir != "" {
			resp.PhotosPath = pickerPath(inst.Saved.PhotosDir)
		}
		if home, err := os.UserHomeDir(); err == nil {
			resp.HomePath = pickerPath(home)
		}
		if ownDir := orDefault(inst.Saved.LibDir, inst.DefaultLibDir); inst.ChannelsDir != ownDir {
			resp.SharedDir = inst.ChannelsDir
		}
		writeJSON(w, resp)
	}
}

// getShared says whether a photo folder has a shared folder in it, so the
// setup can say so before anything is saved.
func getShared() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		dir := fromPickerPath(r.URL.Query().Get("path"))
		writeJSON(w, map[string]string{"sharedDir": installation.FindShared(dir)})
	}
}

type setupRequest struct {
	PhotosPath string `json:"photosPath"`
	LibDir     string `json:"libDir"`
	Share      bool   `json:"share"`
}

func postSetup(current func() Installation) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req setupRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "The request could not be read.", http.StatusBadRequest)
			return
		}
		choice, msg := validChoice(req)
		if msg != "" {
			http.Error(w, msg, http.StatusBadRequest)
			return
		}
		inst := current()
		next, err := installation.Decide(inst.Saved, choice, inst.ChannelsDir, inst.DefaultLibDir)
		if err == nil {
			err = inst.Apply(next)
		}
		if err != nil {
			http.Error(w, "The settings could not be saved: "+err.Error(), http.StatusInternalServerError)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	}
}

// validChoice checks the request and says in a sentence what is wrong.
func validChoice(req setupRequest) (installation.Choice, string) {
	photos := fromPickerPath(req.PhotosPath)
	if req.PhotosPath == "" {
		return installation.Choice{}, "Choose the folder that holds your photos."
	}
	if info, err := os.Stat(photos); err != nil || !info.IsDir() {
		return installation.Choice{}, "The photo folder " + photos + " does not exist. Choose another one."
	}
	if req.LibDir != "" && !filepath.IsAbs(req.LibDir) {
		return installation.Choice{}, "The data folder has to be a full path, starting at the top of the disk."
	}
	return installation.Choice{PhotosDir: photos, LibDir: req.LibDir, Share: req.Share}, ""
}

func orDefault(v, def string) string {
	if v == "" {
		return def
	}
	return v
}

func writeJSON(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(v) //nolint:errcheck
}

// Hooks is what the router needs of the installation: the setup for the
// installed app (Setup), or sharing for a server whose photo folder is fixed
// (Sharing). Either may be nil.
type Hooks struct {
	Setup   func() Installation
	Sharing *Sharing
}

// Sharing is a server's sharing: where destinations are shared, "" while
// they are not, and making the shared folder in the photo folder.
type Sharing struct {
	SharedDir string
	Share     func() error
}

// HandleSharing registers the sharing routes of a server (server mode): the
// photo folder is fixed there, so sharing is all there is to set up. A NAS
// that shares first is found by the desk installation's setup.
func HandleSharing(mux *http.ServeMux, sh *Sharing) {
	mux.HandleFunc("GET /api/setup/sharing", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, map[string]string{"sharedDir": sh.SharedDir})
	})
	mux.HandleFunc("POST /api/setup/share", func(w http.ResponseWriter, r *http.Request) {
		if sh.SharedDir != "" {
			http.Error(w, "Destinations are shared already, through "+sh.SharedDir+".", http.StatusConflict)
			return
		}
		if err := sh.Share(); err != nil {
			http.Error(w, "The shared folder could not be made: "+err.Error(), http.StatusInternalServerError)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	})
}
