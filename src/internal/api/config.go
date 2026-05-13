package api

import (
	"encoding/json"
	"net/http"
)

func handleConfig(boundary, startPath string, serverRole bool) http.HandlerFunc {
	type configResponse struct {
		Boundary   string `json:"boundary"`
		StartPath  string `json:"startPath"`
		ServerRole bool   `json:"serverRole"`
	}
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(configResponse{Boundary: boundary, StartPath: startPath, ServerRole: serverRole})
	}
}
