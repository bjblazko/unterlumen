package batchrename

import (
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"

	"huepattl.de/unterlumen/internal/media"
	"huepattl.de/unterlumen/internal/pathguard"
)

type batchRenameRequest struct {
	Files   []string `json:"files"`
	Pattern string   `json:"pattern"`
}

type batchRenameMapping struct {
	File    string `json:"file"`
	NewName string `json:"newName"`
	Error   string `json:"error,omitempty"`
}

type batchRenamePreviewResponse struct {
	Mappings  []batchRenameMapping `json:"mappings"`
	Conflicts int                  `json:"conflicts"`
}

type batchRenameResult struct {
	File    string `json:"file"`
	Success bool   `json:"success"`
	Error   string `json:"error,omitempty"`
}

type batchRenameExecuteResponse struct {
	Results []batchRenameResult `json:"results"`
}

// Handle registers the batch-rename routes on mux.
func Handle(mux *http.ServeMux, root string, cache *media.ScanCache) {
	mux.HandleFunc("/api/batch-rename/preview", handleBatchRenamePreview(root, cache))
	mux.HandleFunc("/api/batch-rename/execute", handleBatchRenameExecute(root, cache))
}

func handleBatchRenamePreview(root string, cache *media.ScanCache) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		req, ok := decodeRenameRequest(w, r)
		if !ok {
			return
		}

		mappings := resolveBatchMappings(root, req.Files, req.Pattern)
		conflicts := applyConflictSuffixes(mappings)

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(batchRenamePreviewResponse{Mappings: mappings, Conflicts: conflicts})
	}
}

func handleBatchRenameExecute(root string, cache *media.ScanCache) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		req, ok := decodeRenameRequest(w, r)
		if !ok {
			return
		}

		mappings := resolveBatchMappings(root, req.Files, req.Pattern)
		applyConflictSuffixes(mappings)

		if results, hasErrors := validateNoErrors(mappings); hasErrors {
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(batchRenameExecuteResponse{Results: results})
			return
		}

		if results, hasCollision := checkExternalCollisions(root, mappings); hasCollision {
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(batchRenameExecuteResponse{Results: results})
			return
		}

		pairs := buildRenamePairs(root, mappings)
		results := executeTwoPassRename(pairs)

		for _, p := range pairs {
			for _, res := range results {
				if res.Success && res.File == p.relFile {
					cache.Invalidate(p.dir)
				}
			}
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(batchRenameExecuteResponse{Results: results})
	}
}

func decodeRenameRequest(w http.ResponseWriter, r *http.Request) (batchRenameRequest, bool) {
	var req batchRenameRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return req, false
	}
	if len(req.Files) == 0 {
		http.Error(w, "No files specified", http.StatusBadRequest)
		return req, false
	}
	if req.Pattern == "" {
		http.Error(w, "No pattern specified", http.StatusBadRequest)
		return req, false
	}
	return req, true
}

func validateNoErrors(mappings []batchRenameMapping) ([]batchRenameResult, bool) {
	for _, m := range mappings {
		if m.Error != "" {
			results := make([]batchRenameResult, len(mappings))
			for i, mm := range mappings {
				results[i] = batchRenameResult{File: mm.File, Success: mm.Error == "", Error: mm.Error}
			}
			return results, true
		}
	}
	return nil, false
}

func checkExternalCollisions(root string, mappings []batchRenameMapping) ([]batchRenameResult, bool) {
	renameSet := make(map[string]struct{})
	for _, m := range mappings {
		if abs, ok := pathguard.SafePath(root, m.File); ok {
			renameSet[abs] = struct{}{}
		}
	}

	for _, m := range mappings {
		abs, ok := pathguard.SafePath(root, m.File)
		if !ok {
			continue
		}
		destPath := filepath.Join(filepath.Dir(abs), m.NewName)
		if _, inSet := renameSet[destPath]; inSet {
			continue
		}
		if _, err := os.Stat(destPath); err == nil {
			results := make([]batchRenameResult, len(mappings))
			for i, mm := range mappings {
				results[i] = batchRenameResult{
					File:  mm.File,
					Error: fmt.Sprintf("destination '%s' already exists", mm.NewName),
				}
			}
			return results, true
		}
	}
	return nil, false
}

type renamePair struct {
	absPath  string
	dir      string
	tempName string
	newName  string
	relFile  string
}

func buildRenamePairs(root string, mappings []batchRenameMapping) []renamePair {
	pairs := make([]renamePair, 0, len(mappings))
	for i, m := range mappings {
		abs, ok := pathguard.SafePath(root, m.File)
		if !ok {
			continue
		}
		ext := filepath.Ext(abs)
		tempName := fmt.Sprintf("_batch_tmp_%03d_%s%s", i, filepath.Base(abs), ext)
		pairs = append(pairs, renamePair{
			absPath:  abs,
			dir:      filepath.Dir(abs),
			tempName: tempName,
			newName:  m.NewName,
			relFile:  m.File,
		})
	}
	return pairs
}

func executeTwoPassRename(pairs []renamePair) []batchRenameResult {
	results := make([]batchRenameResult, len(pairs))

	for i, p := range pairs {
		tempPath := filepath.Join(p.dir, p.tempName)
		if err := os.Rename(p.absPath, tempPath); err != nil {
			results[i] = batchRenameResult{File: p.relFile, Error: fmt.Sprintf("temp rename failed: %v", err)}
		}
	}

	for i, p := range pairs {
		if results[i].Error != "" {
			continue
		}
		tempPath := filepath.Join(p.dir, p.tempName)
		finalPath := filepath.Join(p.dir, p.newName)
		if err := os.Rename(tempPath, finalPath); err != nil {
			results[i] = batchRenameResult{File: p.relFile, Error: fmt.Sprintf("final rename failed: %v", err)}
			os.Rename(tempPath, p.absPath) // attempt restore
		} else {
			results[i] = batchRenameResult{File: p.relFile, Success: true}
		}
	}

	return results
}
