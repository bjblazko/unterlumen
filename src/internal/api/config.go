package api

import (
	"encoding/json"
	"net/http"
)

// canSetup says the shared folder and the data folder are chosen in the app
// (#setup) rather than on the command line. folderDialog says the system's
// folder dialog can be asked for (/api/folder-dialog).
func handleConfig(boundary, startPath, homePath string, serverRole bool, version string, canSetup, folderDialog bool) http.HandlerFunc {
	type configResponse struct {
		Boundary     string `json:"boundary"`
		StartPath    string `json:"startPath"`
		HomePath     string `json:"homePath"`
		ServerRole   bool   `json:"serverRole"`
		Version      string `json:"version"`
		CanSetup     bool   `json:"canSetup"`
		FolderDialog bool   `json:"folderDialog"`
	}
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(configResponse{Boundary: boundary, StartPath: startPath, HomePath: homePath, ServerRole: serverRole, Version: version, CanSetup: canSetup, FolderDialog: folderDialog})
	}
}
