package api

import (
	"encoding/json"
	"net/http"
	"runtime"

	"huepattl.de/unterlumen/internal/media"
)

func handleToolsCheck() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ffmpeg := media.CheckFFmpeg()
		type toolStatus struct {
			Available   bool `json:"available"`
			HEIFSupport bool `json:"heifSupport,omitempty"`
		}
		resp := struct {
			Platform string     `json:"platform"`
			Exiftool toolStatus `json:"exiftool"`
			FFmpeg   toolStatus `json:"ffmpeg"`
			Sips     toolStatus `json:"sips"`
		}{
			Platform: runtime.GOOS,
			Exiftool: toolStatus{Available: media.CheckExiftool()},
			FFmpeg:   toolStatus{Available: ffmpeg.Available, HEIFSupport: ffmpeg.HEIFSupport},
			Sips:     toolStatus{Available: media.CheckSips()},
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}
}
