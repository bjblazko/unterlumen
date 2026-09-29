// Package apifolderdialog opens the system's folder dialog for the page
// (POST /api/folder-dialog), for an app that runs on the user's own computer.
package apifolderdialog

import (
	"encoding/json"
	"net"
	"net/http"

	"huepattl.de/unterlumen/internal/desktop"
)

// Handle registers the route. It is registered only for the installed app
// (configured by config.json, never in server mode, dev runs or the e2e
// tests) on a system that has a dialog.
func Handle(mux *http.ServeMux) {
	mux.HandleFunc("POST /api/folder-dialog", chooseFolder)
}

// Available says whether this system has a folder dialog.
func Available() bool {
	return desktop.CanChooseFolder()
}

func chooseFolder(w http.ResponseWriter, r *http.Request) {
	// The dialog opens on the computer the server runs on, so only a page on
	// that computer may ask for it — not a phone reaching it over the network.
	if !fromThisComputer(r) {
		http.Error(w, "The system's folder dialog opens only on the computer Unterlumen runs on.", http.StatusForbidden)
		return
	}
	var body struct {
		Prompt string `json:"prompt"`
	}
	json.NewDecoder(r.Body).Decode(&body) //nolint:errcheck // an empty prompt is fine
	if body.Prompt == "" {
		body.Prompt = "Choose a folder"
	}
	path, err := desktop.ChooseFolder(r.Context(), body.Prompt)
	if err != nil {
		http.Error(w, "The system's folder dialog could not be opened: "+err.Error(), http.StatusNotImplemented)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]any{"path": path, "cancelled": path == ""}) //nolint:errcheck
}

func fromThisComputer(r *http.Request) bool {
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return false
	}
	ip := net.ParseIP(host)
	return ip != nil && ip.IsLoopback()
}
