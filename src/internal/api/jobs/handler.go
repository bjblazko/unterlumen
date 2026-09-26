// Package jobs serves the job register to the UI's status line (ADR-0036).
package jobs

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"time"

	"huepattl.de/unterlumen/internal/jobs"
)

// pace is the least time between two writes to one client. Changes in
// between are merged by the subscription, so a scan of thousands of files
// reaches the browser a few times a second, not thousands of times.
const pace = 150 * time.Millisecond

// Handle registers the job stream on mux.
func Handle(mux *http.ServeMux, reg *jobs.Registry) {
	mux.HandleFunc("GET /api/jobs/stream", stream(reg))
}

// stream sends every job in the register, then each change, as server-sent
// events, until the client goes away.
func stream(reg *jobs.Registry) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		flusher, ok := w.(http.Flusher)
		if !ok {
			http.Error(w, "streaming not supported", http.StatusInternalServerError)
			return
		}
		snapshot, sub := reg.Subscribe()
		defer sub.Close()

		w.Header().Set("Content-Type", "text/event-stream")
		w.Header().Set("Cache-Control", "no-cache")
		w.Header().Set("X-Accel-Buffering", "no")
		w.WriteHeader(http.StatusOK)

		// A comment first, so the client has an open stream even when no job
		// is running.
		if _, err := fmt.Fprint(w, ": jobs\n\n"); err != nil {
			return
		}
		if err := writeJobs(w, snapshot); err != nil {
			return
		}
		flusher.Flush()

		for {
			select {
			case <-r.Context().Done():
				return
			case <-sub.Ready():
				if err := writeJobs(w, sub.Take()); err != nil {
					return
				}
				flusher.Flush()
				time.Sleep(pace)
			}
		}
	}
}

func writeJobs(w http.ResponseWriter, js []jobs.Job) error {
	for _, j := range js {
		data, err := json.Marshal(j)
		if err != nil {
			return err
		}
		if _, err := fmt.Fprintf(w, "data: %s\n\n", data); err != nil {
			return err
		}
	}
	return nil
}

// Track wraps a handler that does its work within the request, so the work
// shows in the status line while it runs. The job fails when the response is
// an error status, or a JSON answer with "ok": false (how deploy reports a
// failed push); the reason is the response text or its "error" field.
func Track(reg *jobs.Registry, kind, place string, title func(*http.Request) string, h http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		job := reg.Start(kind, title(r), place)
		rec := &responseRecorder{ResponseWriter: w, status: http.StatusOK}
		defer func() { job.Finish(rec.failure()) }()
		h(rec, r)
	}
}

// maxRecorded bounds how much of a response is kept to find the reason for a
// failure; answers larger than this are taken as successful.
const maxRecorded = 1 << 20

// responseRecorder passes the response through and keeps its status and body.
type responseRecorder struct {
	http.ResponseWriter
	status int
	body   bytes.Buffer
}

func (rr *responseRecorder) WriteHeader(code int) {
	rr.status = code
	rr.ResponseWriter.WriteHeader(code)
}

func (rr *responseRecorder) Write(b []byte) (int, error) {
	if rr.body.Len()+len(b) <= maxRecorded {
		rr.body.Write(b)
	}
	return rr.ResponseWriter.Write(b)
}

func (rr *responseRecorder) failure() error {
	if rr.status >= 400 {
		return errors.New(strings.TrimSpace(rr.body.String()))
	}
	var answer struct {
		OK    *bool  `json:"ok"`
		Error string `json:"error"`
	}
	if json.Unmarshal(rr.body.Bytes(), &answer) == nil && answer.OK != nil && !*answer.OK {
		return errors.New(answer.Error)
	}
	return nil
}
