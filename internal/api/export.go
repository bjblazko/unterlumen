package api

import (
	"archive/zip"
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"

	"huepattl.de/unterlumen/internal/media"
)

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
