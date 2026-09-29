package api

import (
	"context"
	"encoding/json"
	"net/http"
	"runtime"
	"sync"

	"huepattl.de/unterlumen/internal/jobs"
	"huepattl.de/unterlumen/internal/media"
	"huepattl.de/unterlumen/internal/toolinstall"
)

// handleToolsInstall installs the missing helper programs. The work is a job
// in the status line and goes on if the page is left; the request waits for
// it and answers with how it ended.
func handleToolsInstall(reg *jobs.Registry) http.HandlerFunc {
	var busy sync.Mutex
	return func(w http.ResponseWriter, r *http.Request) {
		if !busy.TryLock() {
			http.Error(w, "The helper programs are being installed already.", http.StatusConflict)
			return
		}
		job := reg.Start("tools", "Installing helper programs", "settings")
		done := make(chan error, 1)
		go func() {
			defer busy.Unlock()
			err := toolinstall.Install(context.Background(), job)
			job.Finish(err)
			done <- err
		}()
		select {
		case err := <-done:
			if err != nil {
				http.Error(w, "The helper programs were not all installed: "+err.Error(), http.StatusBadGateway)
				return
			}
			w.WriteHeader(http.StatusNoContent)
		case <-r.Context().Done():
		}
	}
}

// handleToolsCheck says which helper programs are there. For the installed
// app (installable) it also says what "Install helper programs" would do.
func handleToolsCheck(installable bool) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ffmpeg := media.CheckFFmpeg()
		type toolStatus struct {
			Available   bool `json:"available"`
			HEIFSupport bool `json:"heifSupport,omitempty"`
			WebPSupport bool `json:"webpSupport,omitempty"`
		}
		resp := struct {
			Platform      string            `json:"platform"`
			Exiftool      toolStatus        `json:"exiftool"`
			FFmpeg        toolStatus        `json:"ffmpeg"`
			Sips          toolStatus        `json:"sips"`
			HeifConvert   toolStatus        `json:"heifConvert"`
			WebPAvailable bool              `json:"webpAvailable"`
			Install       *toolinstall.Plan `json:"install,omitempty"`
		}{
			Platform:      runtime.GOOS,
			Exiftool:      toolStatus{Available: media.CheckExiftool()},
			FFmpeg:        toolStatus{Available: ffmpeg.Available, HEIFSupport: ffmpeg.HEIFSupport, WebPSupport: ffmpeg.WebPSupport},
			Sips:          toolStatus{Available: media.CheckSips()},
			HeifConvert:   toolStatus{Available: media.CheckHeifConvert()},
			WebPAvailable: ffmpeg.WebPSupport || media.CheckCwebp(),
		}
		if installable {
			plan := toolinstall.PlanNow()
			resp.Install = &plan
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(resp)
	}
}
