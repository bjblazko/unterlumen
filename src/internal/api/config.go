package api

import (
	"encoding/json"
	"net/http"
)

// canSetup says the photo folder is chosen in the app (#setup) rather than on
// the command line; needsSetup that it has not been chosen yet.
func handleConfig(boundary, startPath, homePath string, serverRole bool, version string, canSetup, needsSetup bool) http.HandlerFunc {
	type configResponse struct {
		Boundary   string `json:"boundary"`
		StartPath  string `json:"startPath"`
		HomePath   string `json:"homePath"`
		ServerRole bool   `json:"serverRole"`
		Version    string `json:"version"`
		CanSetup   bool   `json:"canSetup"`
		NeedsSetup bool   `json:"needsSetup"`
	}
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(configResponse{Boundary: boundary, StartPath: startPath, HomePath: homePath, ServerRole: serverRole, Version: version, CanSetup: canSetup, NeedsSetup: needsSetup})
	}
}
