// Package metadata serves the metadata editor (ADR-0060): what the chosen
// files say now, what a change would write to each, and the writing itself,
// one file per request so the editor can show its progress.
package metadata

import (
	"encoding/json"
	"net/http"
	"path/filepath"
	"strconv"

	"huepattl.de/unterlumen/internal/library"
	"huepattl.de/unterlumen/internal/media"
	meta "huepattl.de/unterlumen/internal/metadata"
	"huepattl.de/unterlumen/internal/pathguard"
)

// Files are given relative to the browse boundary, as every path-taking API
// takes them.
type readRequest struct {
	Files  []string    `json:"files"`
	Change meta.Change `json:"change"`
}

type fileState struct {
	File    string        `json:"file"`
	Current *meta.Current `json:"current,omitempty"`
	Plan    *meta.Plan    `json:"plan,omitempty"`
	Error   string        `json:"error,omitempty"`
	abs     string
}

type readResponse struct {
	Files []fileState `json:"files"`
	// Exiftool says whether anything can be written into the files.
	Exiftool bool `json:"exiftool"`
}

type writeRequest struct {
	File   string      `json:"file"`
	Target meta.Target `json:"target"`
	Change meta.Change `json:"change"`
}

type writeResponse struct {
	File    string            `json:"file"`
	Success bool              `json:"success"`
	Skipped string            `json:"skipped,omitempty"`
	Error   string            `json:"error,omitempty"`
	Before  *meta.Current     `json:"before,omitempty"`
	After   *meta.Current     `json:"after,omitempty"`
	IDs     map[string]string `json:"ids,omitempty"` // the photo's ID in each library afterwards
}

// Handle registers the editor's routes on mux. libMgr may be nil.
func Handle(mux *http.ServeMux, root string, cache *media.ScanCache, libMgr *library.Manager) {
	mux.HandleFunc("POST /api/metadata/read", handleRead(root))
	mux.HandleFunc("POST /api/metadata/write", handleWrite(root, cache, libMgr))
	mux.HandleFunc("POST /api/metadata/rating", handleRating(root, cache, libMgr))
}

type ratingRequest struct {
	Files  []string `json:"files"`
	Rating int      `json:"rating"`
}

type ratingResponse struct {
	Results []writeResponse `json:"results"`
}

// handleRating sets the stars of the files from a key or a click: always in
// the sidecar of a photo, the NFO of a video, so it is quick, works without
// exiftool and leaves the files and their IDs as they are. 0 is no rating;
// it is written, so it also hides a rating the file itself carries.
func handleRating(root string, cache *media.ScanCache, libMgr *library.Manager) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req ratingRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil || len(req.Files) == 0 || req.Rating < 0 || req.Rating > 5 {
			http.Error(w, "Name the files and a rating from 0 to 5", http.StatusBadRequest)
			return
		}
		change := meta.Change{Set: map[meta.Field]string{meta.Rating: strconv.Itoa(req.Rating)}}
		res := ratingResponse{}
		for _, f := range req.Files {
			abs, ok := pathguard.SafePath(root, f)
			if !ok {
				res.Results = append(res.Results, writeResponse{File: f, Error: "invalid path"})
				continue
			}
			res.Results = append(res.Results, write(abs, writeRequest{File: f, Target: meta.TargetSidecar, Change: change}, cache, libMgr, false))
		}
		writeJSON(w, res)
	}
}

// handleRead says what each file says now and, with a change, what it
// would write to each.
func handleRead(root string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req readRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil || len(req.Files) == 0 {
			http.Error(w, "Name the files to read", http.StatusBadRequest)
			return
		}
		states, abs := resolve(root, req.Files)
		current, err := meta.Read(abs)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		for i := range states {
			if states[i].Error != "" {
				continue
			}
			cur := current[states[i].abs]
			plan := meta.PlanFor(req.Change, cur)
			states[i].Current, states[i].Plan = &cur, &plan
		}
		writeJSON(w, readResponse{Files: states, Exiftool: media.CheckExiftool()})
	}
}

// resolve turns the files into absolute paths; a file outside the boundary
// keeps its place in the answer with an error.
func resolve(root string, files []string) ([]fileState, []string) {
	var abs []string
	states := make([]fileState, len(files))
	for i, f := range files {
		states[i].File = f
		p, ok := pathguard.SafePath(root, f)
		if !ok {
			states[i].Error = "invalid path"
			continue
		}
		states[i].abs = p
		abs = append(abs, p)
	}
	return states, abs
}

func handleWrite(root string, cache *media.ScanCache, libMgr *library.Manager) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var req writeRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.File == "" {
			http.Error(w, "Name the file to write", http.StatusBadRequest)
			return
		}
		if req.Target != meta.TargetFile && req.Target != meta.TargetSidecar {
			http.Error(w, "Say where to write: file or sidecar", http.StatusBadRequest)
			return
		}
		abs, ok := pathguard.SafePath(root, req.File)
		if !ok {
			writeJSON(w, writeResponse{File: req.File, Error: "invalid path"})
			return
		}
		writeJSON(w, write(abs, req, cache, libMgr, true))
	}
}

// write writes one file. What the file says now is read only for a date
// correction, which needs the file's own date; afterwards only when asked,
// so a rating from a key stays quick.
func write(abs string, req writeRequest, cache *media.ScanCache, libMgr *library.Manager, readAfter bool) writeResponse {
	res := writeResponse{File: req.File}
	var cur meta.Current
	if req.Change.Date != nil {
		before, err := meta.Read([]string{abs})
		if err != nil {
			res.Error = err.Error()
			return res
		}
		cur = before[abs]
		res.Before = &cur
	}
	plan := meta.PlanFor(req.Change, cur)
	if plan.Skipped != "" {
		res.Success, res.Skipped = true, plan.Skipped
		return res
	}
	if err := meta.Write(abs, req.Target, plan, req.Change); err != nil {
		res.Error = err.Error()
		return res
	}
	res.Success = true
	cache.Invalidate(filepath.Dir(abs))
	if libMgr != nil {
		res.IDs = libMgr.MetadataWritten(abs, sidecarTitle(abs, req.Target, plan))
	}
	if !readAfter {
		return res
	}
	if after, err := meta.Read([]string{abs}); err == nil {
		a := after[abs]
		res.After = &a
	}
	return res
}

// sidecarTitle is the title written to a photo's sidecar, which a library
// keeps as a note; nil when none was.
func sidecarTitle(abs string, target meta.Target, plan meta.Plan) *string {
	title, ok := plan.Set[meta.Title]
	if !ok || target != meta.TargetSidecar || media.IsVideo(abs) {
		return nil
	}
	return &title
}

func writeJSON(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(v) //nolint:errcheck
}
