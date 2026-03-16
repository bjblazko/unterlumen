package api

import (
	"encoding/json"
	"net/http"

	"huepattl.de/unterlumen/internal/media"
)

func handleToolsCheck() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]bool{
			"exiftool": media.CheckExiftool(),
		})
	}
}
