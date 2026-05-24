package api

import (
	"encoding/json"
	"net/http"
)

func handleConfig(boundary, startPath, homePath string, serverRole bool) http.HandlerFunc {
	type configResponse struct {
		Boundary   string `json:"boundary"`
		StartPath  string `json:"startPath"`
		HomePath   string `json:"homePath"`
		ServerRole bool   `json:"serverRole"`
	}
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(configResponse{Boundary: boundary, StartPath: startPath, HomePath: homePath, ServerRole: serverRole})
	}
}
