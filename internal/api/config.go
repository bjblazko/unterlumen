package api

import (
	"encoding/json"
	"net/http"
)

func handleConfig(startPath string, serverRole bool) http.HandlerFunc {
	type configResponse struct {
		StartPath  string `json:"startPath"`
		ServerRole bool   `json:"serverRole"`
	}
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(configResponse{StartPath: startPath, ServerRole: serverRole})
	}
}
