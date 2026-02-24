package api

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
)

type fileOpRequest struct {
	Files       []string `json:"files"`
	Destination string   `json:"destination"`
}

type fileOpResult struct {
	File    string `json:"file"`
	Success bool   `json:"success"`
	Error   string `json:"error,omitempty"`
}

type fileOpResponse struct {
	Results []fileOpResult `json:"results"`
}

func handleDelete(root string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		var req fileOpRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "Invalid request body", http.StatusBadRequest)
			return
		}

		if len(req.Files) == 0 {
			http.Error(w, "No files specified", http.StatusBadRequest)
			return
		}

		var results []fileOpResult
		for _, file := range req.Files {
			filePath, ok := safePath(root, file)
			if !ok {
				results = append(results, fileOpResult{
					File:  file,
					Error: "invalid path",
				})
				continue
			}

			info, err := os.Stat(filePath)
			if err != nil {
				results = append(results, fileOpResult{
					File:  file,
					Error: err.Error(),
				})
				continue
			}

			if info.IsDir() {
				results = append(results, fileOpResult{
					File:  file,
					Error: "cannot delete directories",
				})
				continue
			}

			if err := os.Remove(filePath); err != nil {
				results = append(results, fileOpResult{
					File:  file,
					Error: err.Error(),
				})
			} else {
				results = append(results, fileOpResult{
					File:    file,
					Success: true,
				})
			}
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(fileOpResponse{Results: results})
	}
}

func handleCopy(root string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}
		handleFileOp(w, r, root, copyFile)
	}
}

func handleMove(root string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}
		handleFileOp(w, r, root, moveFile)
	}
}

func handleFileOp(w http.ResponseWriter, r *http.Request, root string, op func(src, dst string) error) {
	var req fileOpRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	if len(req.Files) == 0 {
		http.Error(w, "No files specified", http.StatusBadRequest)
		return
	}

	destDir, ok := safePath(root, req.Destination)
	if !ok {
		http.Error(w, "Invalid destination path", http.StatusBadRequest)
		return
	}

	// Verify destination is a directory
	info, err := os.Stat(destDir)
	if err != nil || !info.IsDir() {
		http.Error(w, "Destination is not a valid directory", http.StatusBadRequest)
		return
	}

	var results []fileOpResult
	for _, file := range req.Files {
		srcPath, ok := safePath(root, file)
		if !ok {
			results = append(results, fileOpResult{
				File:  file,
				Error: "invalid source path",
			})
			continue
		}

		dstPath := filepath.Join(destDir, filepath.Base(srcPath))

		if err := op(srcPath, dstPath); err != nil {
			results = append(results, fileOpResult{
				File:  file,
				Error: err.Error(),
			})
		} else {
			results = append(results, fileOpResult{
				File:    file,
				Success: true,
			})
		}
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(fileOpResponse{Results: results})
}

func copyFile(src, dst string) error {
	// Don't overwrite existing files
	if _, err := os.Stat(dst); err == nil {
		return fmt.Errorf("destination file already exists")
	}

	srcFile, err := os.Open(src)
	if err != nil {
		return err
	}
	defer srcFile.Close()

	dstFile, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer dstFile.Close()

	if _, err := io.Copy(dstFile, srcFile); err != nil {
		os.Remove(dst)
		return err
	}

	return nil
}

func moveFile(src, dst string) error {
	// Don't overwrite existing files
	if _, err := os.Stat(dst); err == nil {
		return fmt.Errorf("destination file already exists")
	}

	// Try rename first (fast, same filesystem)
	if err := os.Rename(src, dst); err == nil {
		return nil
	}

	// Fall back to copy + delete
	if err := copyFile(src, dst); err != nil {
		return err
	}
	return os.Remove(src)
}
