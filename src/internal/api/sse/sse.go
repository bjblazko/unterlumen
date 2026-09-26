// Package sse writes server-sent event streams.
package sse

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
)

// Start opens an event stream: it sends the event-stream headers and a 200.
// When w cannot flush it answers 500 instead and returns false.
func Start(w http.ResponseWriter) (http.Flusher, bool) {
	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "streaming not supported", http.StatusInternalServerError)
		return nil, false
	}
	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("X-Accel-Buffering", "no")
	w.WriteHeader(http.StatusOK)
	return flusher, true
}

// Send writes v as one JSON data event. It does not flush, so a caller can
// send several events and flush once.
func Send(w io.Writer, v any) error {
	data, err := json.Marshal(v)
	if err != nil {
		return err
	}
	_, err = fmt.Fprintf(w, "data: %s\n\n", data)
	return err
}
