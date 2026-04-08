package api

import (
	"archive/zip"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"sync"
	"time"

	"huepattl.de/unterlumen/internal/media"
)

// zipJobs holds temp ZIP files keyed by a random token.
// Entries are removed on download or after 10 minutes.
var (
	zipJobsMu sync.Mutex
	zipJobs   = make(map[string]string) // token → temp file path
)

func generateToken() string {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return fmt.Sprintf("%d", time.Now().UnixNano())
	}
	return hex.EncodeToString(b)
}

// exportRequest is the JSON body for export endpoints.
type exportRequest struct {
	Files       []string           `json:"files"`
	Format      string             `json:"format"`
	Quality     int                `json:"quality"`
	Scale       media.ScaleOptions `json:"scale"`
	ExifMode    string             `json:"exifMode"`
	Destination string             `json:"destination"` // save-to-disk only
}

// estimateRequest is the JSON body for the estimate endpoint.
type estimateRequest struct {
	Files   []string           `json:"files"`
	Format  string             `json:"format"`
	Quality int                `json:"quality"`
	Scale   media.ScaleOptions `json:"scale"`
	Method  string             `json:"method"` // "heuristic" or "encode"
}

type estimateEntry struct {
	File        string `json:"file"`
	InputBytes  int64  `json:"inputBytes"`
	OutputBytes int64  `json:"outputBytes"`
	OrigWidth   int    `json:"origWidth"`
	OrigHeight  int    `json:"origHeight"`
	Width       int    `json:"width"`
	Height      int    `json:"height"`
	Error       string `json:"error,omitempty"`
}

type estimateResponse struct {
	Estimates []estimateEntry `json:"estimates"`
}

type exportResult struct {
	File    string `json:"file"`
	Success bool   `json:"success"`
	Error   string `json:"error,omitempty"`
}

type exportSaveResponse struct {
	Results []exportResult `json:"results"`
}

// handleExportEstimate handles POST /api/export/estimate.
// Returns estimated output sizes per file using heuristics or actual encoding.
func handleExportEstimate(root string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}

		var req estimateRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "invalid request body", http.StatusBadRequest)
			return
		}

		opts := media.ExportOptions{
			Format:  req.Format,
			Quality: req.Quality,
			Scale:   req.Scale,
		}

		var estimates []estimateEntry
		for _, relPath := range req.Files {
			absPath, ok := safePath(root, relPath)
			if !ok {
				estimates = append(estimates, estimateEntry{File: relPath})
				continue
			}

			if req.Method == "encode" {
				// Accurate: actually encode and measure
				data, err := media.ExportImage(absPath, opts)
				if err != nil {
					estimates = append(estimates, estimateEntry{File: relPath, Error: err.Error()})
					continue
				}
				info, _ := os.Stat(absPath)
				var inputBytes int64
				if info != nil {
					inputBytes = info.Size()
				}
				origW, origH := media.GetSourceDims(absPath)
				_, _, _, _, outW, outH, _ := media.EstimateSize(absPath, opts)
				estimates = append(estimates, estimateEntry{
					File:        relPath,
					InputBytes:  inputBytes,
					OutputBytes: int64(len(data)),
					OrigWidth:   origW,
					OrigHeight:  origH,
					Width:       outW,
					Height:      outH,
				})
			} else {
				// Heuristic: fast formula
				in, out, origW, origH, outW, outH, err := media.EstimateSize(absPath, opts)
				if err != nil {
					estimates = append(estimates, estimateEntry{File: relPath, Error: err.Error()})
					continue
				}
				estimates = append(estimates, estimateEntry{
					File:        relPath,
					InputBytes:  in,
					OutputBytes: out,
					OrigWidth:   origW,
					OrigHeight:  origH,
					Width:       outW,
					Height:      outH,
				})
			}
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(estimateResponse{Estimates: estimates})
	}
}

// handleExportZip handles POST /api/export/zip.
// Converts and streams all selected files as a ZIP archive.
func handleExportZip(root string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}

		var req exportRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "invalid request body", http.StatusBadRequest)
			return
		}

		opts := media.ExportOptions{
			Format:   req.Format,
			Quality:  req.Quality,
			Scale:    req.Scale,
			ExifMode: req.ExifMode,
		}

		w.Header().Set("Content-Type", "application/zip")
		w.Header().Set("Content-Disposition", `attachment; filename="export.zip"`)

		zw := zip.NewWriter(w)
		defer zw.Close()

		for _, relPath := range req.Files {
			absPath, ok := safePath(root, relPath)
			if !ok {
				continue
			}

			data, err := media.ExportImage(absPath, opts)
			if err != nil {
				continue
			}

			outName := media.ExportedName(filepath.Base(relPath), req.Format)
			fw, err := zw.Create(outName)
			if err != nil {
				continue
			}
			fw.Write(data)
		}
	}
}

// handleExportSave handles POST /api/export/save.
// Converts and writes files to a local directory on disk.
// Only registered when serverRole=false.
func handleExportSave(root string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}

		var req exportRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "invalid request body", http.StatusBadRequest)
			return
		}

		if req.Destination == "" {
			http.Error(w, "destination is required", http.StatusBadRequest)
			return
		}

		info, err := os.Stat(req.Destination)
		if err != nil || !info.IsDir() {
			http.Error(w, "destination directory does not exist", http.StatusBadRequest)
			return
		}

		opts := media.ExportOptions{
			Format:   req.Format,
			Quality:  req.Quality,
			Scale:    req.Scale,
			ExifMode: req.ExifMode,
		}

		var results []exportResult
		for _, relPath := range req.Files {
			absPath, ok := safePath(root, relPath)
			if !ok {
				results = append(results, exportResult{
					File:  relPath,
					Error: "invalid path",
				})
				continue
			}

			data, err := media.ExportImage(absPath, opts)
			if err != nil {
				results = append(results, exportResult{
					File:  relPath,
					Error: err.Error(),
				})
				continue
			}

			outName := media.ExportedName(filepath.Base(relPath), req.Format)
			outPath := filepath.Join(req.Destination, outName)
			if err := os.WriteFile(outPath, data, 0644); err != nil {
				results = append(results, exportResult{
					File:  relPath,
					Error: err.Error(),
				})
				continue
			}

			results = append(results, exportResult{
				File:    relPath,
				Success: true,
			})
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(exportSaveResponse{Results: results})
	}
}

// zipStreamEvent is a single SSE payload for ZIP stream progress.
type zipStreamEvent struct {
	File     string `json:"file,omitempty"`
	Done     int    `json:"done"`
	Total    int    `json:"total"`
	Complete bool   `json:"complete,omitempty"`
	Token    string `json:"token,omitempty"`
	Error    string `json:"error,omitempty"`
}

// handleExportZipStream handles POST /api/export/zip-stream.
// Streams SSE progress events while building a ZIP, then stores it under a
// short-lived token for the client to download via /api/export/zip-download.
func handleExportZipStream(root string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}

		var req exportRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "invalid request body", http.StatusBadRequest)
			return
		}

		opts := media.ExportOptions{
			Format:   req.Format,
			Quality:  req.Quality,
			Scale:    req.Scale,
			ExifMode: req.ExifMode,
		}

		flusher, ok := w.(http.Flusher)
		if !ok {
			http.Error(w, "streaming not supported", http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "text/event-stream")
		w.Header().Set("Cache-Control", "no-cache")
		w.Header().Set("X-Accel-Buffering", "no")
		w.WriteHeader(http.StatusOK)

		send := func(evt zipStreamEvent) {
			data, _ := json.Marshal(evt)
			fmt.Fprintf(w, "data: %s\n\n", data)
			flusher.Flush()
		}

		tmpFile, err := os.CreateTemp("", "unterlumen-zip-*.zip")
		if err != nil {
			send(zipStreamEvent{Error: err.Error()})
			return
		}
		tmpPath := tmpFile.Name()

		zw := zip.NewWriter(tmpFile)
		total := len(req.Files)

		for i, relPath := range req.Files {
			select {
			case <-r.Context().Done():
				zw.Close()
				tmpFile.Close()
				os.Remove(tmpPath)
				return
			default:
			}

			send(zipStreamEvent{File: filepath.Base(relPath), Done: i, Total: total})

			absPath, ok := safePath(root, relPath)
			if !ok {
				continue
			}

			data, err := media.ExportImage(absPath, opts)
			if err != nil {
				send(zipStreamEvent{File: filepath.Base(relPath), Done: i + 1, Total: total, Error: err.Error()})
				continue
			}

			outName := media.ExportedName(filepath.Base(relPath), req.Format)
			if fw, err := zw.Create(outName); err == nil {
				fw.Write(data)
			}
		}

		zw.Close()
		tmpFile.Close()

		token := generateToken()
		zipJobsMu.Lock()
		zipJobs[token] = tmpPath
		zipJobsMu.Unlock()

		// Auto-expire after 10 minutes in case the client never downloads.
		go func() {
			time.Sleep(10 * time.Minute)
			zipJobsMu.Lock()
			if p, exists := zipJobs[token]; exists {
				os.Remove(p)
				delete(zipJobs, token)
			}
			zipJobsMu.Unlock()
		}()

		send(zipStreamEvent{Done: total, Total: total, Complete: true, Token: token})
	}
}

// handleExportZipDownload handles GET /api/export/zip-download?token=…
// Serves the prepared ZIP and removes it from disk.
func handleExportZipDownload() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		token := r.URL.Query().Get("token")
		if token == "" {
			http.Error(w, "missing token", http.StatusBadRequest)
			return
		}

		zipJobsMu.Lock()
		path, ok := zipJobs[token]
		if ok {
			delete(zipJobs, token)
		}
		zipJobsMu.Unlock()

		if !ok {
			http.Error(w, "token not found or expired", http.StatusNotFound)
			return
		}
		defer os.Remove(path)

		f, err := os.Open(path)
		if err != nil {
			http.Error(w, "could not open ZIP", http.StatusInternalServerError)
			return
		}
		defer f.Close()

		info, err := f.Stat()
		if err == nil {
			w.Header().Set("Content-Length", fmt.Sprintf("%d", info.Size()))
		}
		w.Header().Set("Content-Type", "application/zip")
		w.Header().Set("Content-Disposition", `attachment; filename="export.zip"`)
		io.Copy(w, f)
	}
}
