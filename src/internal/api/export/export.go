package export

import (
	"archive/zip"
	"context"
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
	"huepattl.de/unterlumen/internal/pathguard"
)

// zipJobs holds temp ZIP files keyed by a random token.
// Entries are removed on download or after 10 minutes.
var (
	zipJobsMu sync.Mutex
	zipJobs   = make(map[string]string) // token → temp file path
)

type exportRequest struct {
	Files       []string           `json:"files"`
	Format      string             `json:"format"`
	Quality     int                `json:"quality"`
	Scale       media.ScaleOptions `json:"scale"`
	ExifMode    string             `json:"exifMode"`
	Destination string             `json:"destination"`
}

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

type zipStreamEvent struct {
	File     string `json:"file,omitempty"`
	Done     int    `json:"done"`
	Total    int    `json:"total"`
	Complete bool   `json:"complete,omitempty"`
	Token    string `json:"token,omitempty"`
	Error    string `json:"error,omitempty"`
}

// Handle registers all /api/export/* routes on mux.
func Handle(mux *http.ServeMux, root string, serverRole bool) {
	mux.HandleFunc("/api/export/estimate", handleExportEstimate(root))
	mux.HandleFunc("/api/export/zip", handleExportZip(root))
	mux.HandleFunc("/api/export/zip-stream", handleExportZipStream(root))
	mux.HandleFunc("/api/export/zip-download", handleExportZipDownload())
	if !serverRole {
		mux.HandleFunc("/api/export/save", handleExportSave(root))
	}
}

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

		opts := media.ExportOptions{Format: req.Format, Quality: req.Quality, Scale: req.Scale}
		var estimates []estimateEntry
		for _, relPath := range req.Files {
			absPath, ok := pathguard.SafePath(root, relPath)
			if !ok {
				estimates = append(estimates, estimateEntry{File: relPath})
				continue
			}
			var entry estimateEntry
			if req.Method == "encode" {
				entry = estimateEncode(relPath, absPath, opts)
			} else {
				entry = estimateHeuristic(relPath, absPath, opts)
			}
			estimates = append(estimates, entry)
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(estimateResponse{Estimates: estimates})
	}
}

func estimateEncode(relPath, absPath string, opts media.ExportOptions) estimateEntry {
	data, err := media.ExportImage(absPath, opts)
	if err != nil {
		return estimateEntry{File: relPath, Error: err.Error()}
	}
	info, _ := os.Stat(absPath)
	var inputBytes int64
	if info != nil {
		inputBytes = info.Size()
	}
	origW, origH := media.GetSourceDims(absPath)
	_, _, _, _, outW, outH, _ := media.EstimateSize(absPath, opts)
	return estimateEntry{
		File:        relPath,
		InputBytes:  inputBytes,
		OutputBytes: int64(len(data)),
		OrigWidth:   origW,
		OrigHeight:  origH,
		Width:       outW,
		Height:      outH,
	}
}

func estimateHeuristic(relPath, absPath string, opts media.ExportOptions) estimateEntry {
	in, out, origW, origH, outW, outH, err := media.EstimateSize(absPath, opts)
	if err != nil {
		return estimateEntry{File: relPath, Error: err.Error()}
	}
	return estimateEntry{
		File:        relPath,
		InputBytes:  in,
		OutputBytes: out,
		OrigWidth:   origW,
		OrigHeight:  origH,
		Width:       outW,
		Height:      outH,
	}
}

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

		opts := exportOpts(req)
		w.Header().Set("Content-Type", "application/zip")
		w.Header().Set("Content-Disposition", `attachment; filename="export.zip"`)

		zw := zip.NewWriter(w)
		defer zw.Close()

		for _, relPath := range req.Files {
			absPath, ok := pathguard.SafePath(root, relPath)
			if !ok {
				continue
			}
			data, err := media.ExportImage(absPath, opts)
			if err != nil {
				continue
			}
			outName := media.ExportedName(filepath.Base(relPath), req.Format)
			if fw, err := zw.Create(outName); err == nil {
				fw.Write(data)
			}
		}
	}
}

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

		opts := exportOpts(req)
		results := processExportBatch(root, req.Files, req.Destination, req.Format, opts)

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(exportSaveResponse{Results: results})
	}
}

func processExportBatch(root string, files []string, dest, format string, opts media.ExportOptions) []exportResult {
	var results []exportResult
	for _, relPath := range files {
		absPath, ok := pathguard.SafePath(root, relPath)
		if !ok {
			results = append(results, exportResult{File: relPath, Error: "invalid path"})
			continue
		}
		data, err := media.ExportImage(absPath, opts)
		if err != nil {
			results = append(results, exportResult{File: relPath, Error: err.Error()})
			continue
		}
		outPath := filepath.Join(dest, media.ExportedName(filepath.Base(relPath), format))
		if err := os.WriteFile(outPath, data, 0644); err != nil {
			results = append(results, exportResult{File: relPath, Error: err.Error()})
			continue
		}
		results = append(results, exportResult{File: relPath, Success: true})
	}
	return results
}

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

		flusher, ok := w.(http.Flusher)
		if !ok {
			http.Error(w, "streaming not supported", http.StatusInternalServerError)
			return
		}

		w.Header().Set("Content-Type", "text/event-stream")
		w.Header().Set("Cache-Control", "no-cache")
		w.Header().Set("X-Accel-Buffering", "no")
		w.WriteHeader(http.StatusOK)

		send := sseWriter(w, flusher)
		opts := exportOpts(req)

		tmpPath, err := buildZipFile(r.Context(), root, req.Files, req.Format, opts, send)
		if err != nil {
			return // buildZipFile already sent the error event or client disconnected
		}

		token := generateToken()
		zipJobsMu.Lock()
		zipJobs[token] = tmpPath
		zipJobsMu.Unlock()

		scheduleZipExpiry(token, tmpPath)
		send(zipStreamEvent{Done: len(req.Files), Total: len(req.Files), Complete: true, Token: token})
	}
}

func buildZipFile(ctx context.Context, root string, files []string, format string, opts media.ExportOptions, send func(zipStreamEvent)) (string, error) {
	tmpFile, err := os.CreateTemp("", "unterlumen-zip-*.zip")
	if err != nil {
		send(zipStreamEvent{Error: err.Error()})
		return "", err
	}
	tmpPath := tmpFile.Name()

	zw := zip.NewWriter(tmpFile)
	total := len(files)

	for i, relPath := range files {
		select {
		case <-ctx.Done():
			zw.Close()
			tmpFile.Close()
			os.Remove(tmpPath)
			return "", ctx.Err()
		default:
		}

		send(zipStreamEvent{File: filepath.Base(relPath), Done: i, Total: total})

		absPath, ok := pathguard.SafePath(root, relPath)
		if !ok {
			continue
		}
		data, err := media.ExportImage(absPath, opts)
		if err != nil {
			send(zipStreamEvent{File: filepath.Base(relPath), Done: i + 1, Total: total, Error: err.Error()})
			continue
		}
		outName := media.ExportedName(filepath.Base(relPath), format)
		if fw, err := zw.Create(outName); err == nil {
			fw.Write(data)
		}
	}

	zw.Close()
	tmpFile.Close()
	return tmpPath, nil
}

func scheduleZipExpiry(token, path string) {
	go func() {
		time.Sleep(10 * time.Minute)
		zipJobsMu.Lock()
		if p, exists := zipJobs[token]; exists {
			os.Remove(p)
			delete(zipJobs, token)
		}
		zipJobsMu.Unlock()
	}()
}

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

		if info, err := f.Stat(); err == nil {
			w.Header().Set("Content-Length", fmt.Sprintf("%d", info.Size()))
		}
		w.Header().Set("Content-Type", "application/zip")
		w.Header().Set("Content-Disposition", `attachment; filename="export.zip"`)
		io.Copy(w, f)
	}
}

func exportOpts(req exportRequest) media.ExportOptions {
	return media.ExportOptions{
		Format:   req.Format,
		Quality:  req.Quality,
		Scale:    req.Scale,
		ExifMode: req.ExifMode,
	}
}

func sseWriter(w http.ResponseWriter, flusher http.Flusher) func(zipStreamEvent) {
	return func(evt zipStreamEvent) {
		data, _ := json.Marshal(evt)
		fmt.Fprintf(w, "data: %s\n\n", data)
		flusher.Flush()
	}
}

func generateToken() string {
	b := make([]byte, 16)
	if _, err := rand.Read(b); err != nil {
		return fmt.Sprintf("%d", time.Now().UnixNano())
	}
	return hex.EncodeToString(b)
}
