// Package apisetup serves the setup place (#setup): the data folder and
// where destinations are shared, saved to config.json and applied without a
// restart (ADR-0042, ADR-0047).
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
	LibDir        string `json:"libDir"`
	DefaultLibDir string `json:"defaultLibDir"`
	HomePath      string `json:"homePath"`
	SharedDir     string `json:"sharedDir"`  // where destinations are shared, "" when they are not
	SharedPath    string `json:"sharedPath"` // the same as the folder picker names it
}

func getSetup(current func() Installation) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		inst := current()
		resp := setupResponse{
			LibDir:        inst.Saved.LibDir,
			DefaultLibDir: inst.DefaultLibDir,
		}
		if home, err := os.UserHomeDir(); err == nil {
			resp.HomePath = pickerPath(home)
		}
		if ownDir := orDefault(inst.Saved.LibDir, inst.DefaultLibDir); inst.ChannelsDir != ownDir {
			resp.SharedDir = inst.ChannelsDir
			resp.SharedPath = pickerPath(inst.ChannelsDir)
		}
		writeJSON(w, resp)
	}
}

// getShared says whether a chosen folder has a shared folder in it, so the
// setup can say so before anything is saved.
func getShared() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		dir := fromPickerPath(r.URL.Query().Get("path"))
		found := installation.FindShared(dir)
		if filepath.Base(dir) == installation.SharedDirName {
			found = dir
		}
		writeJSON(w, map[string]any{
			"sharedDir": found,
			// The shared folder is made inside the chosen one, which the top of a disk cannot hold.
			"canShare": found != "" || !installation.IsDiskRoot(dir),
		})
	}
}

type setupRequest struct {
	LibDir     string `json:"libDir"`
	SharedPath string `json:"sharedPath"` // as the folder picker names it; "" for not shared
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
		next, err := installation.Decide(inst.Saved, choice, inst.ChannelsDir)
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
	var shared string
	if req.SharedPath != "" {
		shared = fromPickerPath(req.SharedPath)
		if info, err := os.Stat(shared); err != nil || !info.IsDir() {
			return installation.Choice{}, "The folder " + shared + " is not there. Connect its disk or NAS, or choose another one."
		}
	}
	if req.LibDir != "" && !filepath.IsAbs(req.LibDir) {
		return installation.Choice{}, "The data folder has to be a full path, starting at the top of the disk."
	}
	return installation.Choice{LibDir: req.LibDir, SharedDir: shared}, ""
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
