package api

import (
	"encoding/json"
	"net/http"
)

func handleConfig(startPath string) http.HandlerFunc {
	type configResponse struct {
		StartPath string `json:"startPath"`
	}
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(configResponse{StartPath: startPath})
	}
}
