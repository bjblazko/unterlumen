package api

import (
	"encoding/json"
	"fmt"
	"io"
	"io/fs"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"huepattl.de/unterlumen/internal/media"
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

func handleDelete(root string, cache *media.ScanCache) http.HandlerFunc {
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

		dirsToInvalidate := make(map[string]struct{})
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
				if err := os.RemoveAll(filePath); err != nil {
					results = append(results, fileOpResult{
						File:  file,
						Error: err.Error(),
					})
				} else {
					cache.InvalidatePrefix(filePath)
					dirsToInvalidate[filepath.Dir(filePath)] = struct{}{}
					results = append(results, fileOpResult{
						File:    file,
						Success: true,
					})
				}
				continue
			}

			if err := os.Remove(filePath); err != nil {
				results = append(results, fileOpResult{
					File:  file,
					Error: err.Error(),
				})
			} else {
				dirsToInvalidate[filepath.Dir(filePath)] = struct{}{}
				results = append(results, fileOpResult{
					File:    file,
					Success: true,
				})
			}
		}

		for dir := range dirsToInvalidate {
			cache.Invalidate(dir)
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(fileOpResponse{Results: results})
	}
}

func handleCopy(root string, cache *media.ScanCache) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}
		handleFileOp(w, r, root, copyFile, cache)
	}
}

func handleMove(root string, cache *media.ScanCache) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}
		handleFileOp(w, r, root, moveFile, cache)
	}
}

func handleFileOp(w http.ResponseWriter, r *http.Request, root string, op func(src, dst string) error, cache *media.ScanCache) {
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

	dirsToInvalidate := make(map[string]struct{})
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
			dirsToInvalidate[filepath.Dir(srcPath)] = struct{}{}
			dirsToInvalidate[destDir] = struct{}{}
			results = append(results, fileOpResult{
				File:    file,
				Success: true,
			})
		}
	}

	for dir := range dirsToInvalidate {
		cache.Invalidate(dir)
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(fileOpResponse{Results: results})
}

func handleMkdir(root string, cache *media.ScanCache) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		var req struct {
			Path string `json:"path"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "Invalid request body", http.StatusBadRequest)
			return
		}

		dirPath, ok := safePath(root, req.Path)
		if !ok {
			http.Error(w, "Invalid path", http.StatusBadRequest)
			return
		}

		if err := os.Mkdir(dirPath, 0755); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		cache.Invalidate(filepath.Dir(dirPath))

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]bool{"success": true})
	}
}

func handleRename(root string, cache *media.ScanCache) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		var req struct {
			Path string `json:"path"`
			Name string `json:"name"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "Invalid request body", http.StatusBadRequest)
			return
		}

		// Validate name is a simple base name
		if req.Name == "" || req.Name == "." || req.Name == ".." ||
			filepath.Base(req.Name) != req.Name {
			http.Error(w, "Invalid name", http.StatusBadRequest)
			return
		}

		srcPath, ok := safePath(root, req.Path)
		if !ok {
			http.Error(w, "Invalid path", http.StatusBadRequest)
			return
		}

		dstPath := filepath.Join(filepath.Dir(srcPath), req.Name)

		if _, err := os.Stat(dstPath); err == nil {
			http.Error(w, "Destination already exists", http.StatusConflict)
			return
		}

		if err := os.Rename(srcPath, dstPath); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		cache.Invalidate(filepath.Dir(srcPath))

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]bool{"success": true})
	}
}

func handleListRecursive(root string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
			return
		}

		var req struct {
			Path string `json:"path"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, "Invalid request body", http.StatusBadRequest)
			return
		}

		dirPath, ok := safePath(root, req.Path)
		if !ok {
			http.Error(w, "Invalid path", http.StatusBadRequest)
			return
		}

		info, err := os.Stat(dirPath)
		if err != nil || !info.IsDir() {
			http.Error(w, "Not a directory", http.StatusBadRequest)
			return
		}

		const maxEntries = 100000
		var files []string
		var dirs []string
		count := 0

		err = filepath.WalkDir(dirPath, func(path string, d fs.DirEntry, err error) error {
			if err != nil {
				return nil // skip errors
			}
			count++
			if count > maxEntries {
				return fmt.Errorf("too many entries")
			}

			// Convert to relative path from root
			rel, err := filepath.Rel(root, path)
			if err != nil {
				return nil
			}

			if d.IsDir() {
				// Skip the root directory itself from the dirs list
				if path != dirPath {
					dirs = append(dirs, rel)
				}
			} else {
				files = append(files, rel)
			}
			return nil
		})

		if err != nil && !strings.Contains(err.Error(), "too many entries") {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		// Sort dirs shallowest first (by number of path separators, then alphabetically)
		sort.Slice(dirs, func(i, j int) bool {
			di := strings.Count(dirs[i], string(filepath.Separator))
			dj := strings.Count(dirs[j], string(filepath.Separator))
			if di != dj {
				return di < dj
			}
			return dirs[i] < dirs[j]
		})

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"files": files,
			"dirs":  dirs,
		})
	}
}

func copyFile(src, dst string) error {
	// Check if source is a directory
	info, err := os.Stat(src)
	if err != nil {
		return err
	}
	if info.IsDir() {
		return copyDir(src, dst)
	}

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

func copyDir(src, dst string) error {
	// Don't overwrite existing destination
	if _, err := os.Stat(dst); err == nil {
		return fmt.Errorf("destination already exists")
	}

	return filepath.WalkDir(src, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		rel, err := filepath.Rel(src, path)
		if err != nil {
			return err
		}
		target := filepath.Join(dst, rel)

		if d.IsDir() {
			return os.MkdirAll(target, 0755)
		}

		// Copy individual file
		in, err := os.Open(path)
		if err != nil {
			return err
		}
		defer in.Close()

		out, err := os.Create(target)
		if err != nil {
			return err
		}
		defer out.Close()

		if _, err := io.Copy(out, in); err != nil {
			os.Remove(target)
			return err
		}
		return nil
	})
}

func moveFile(src, dst string) error {
	// Check if source is a directory
	info, err := os.Stat(src)
	if err != nil {
		return err
	}
	if info.IsDir() {
		// Don't overwrite existing destination
		if _, err := os.Stat(dst); err == nil {
			return fmt.Errorf("destination already exists")
		}
		// Try rename first (fast, same filesystem)
		if err := os.Rename(src, dst); err == nil {
			return nil
		}
		// Fall back to copy + delete
		if err := copyDir(src, dst); err != nil {
			return err
		}
		return os.RemoveAll(src)
	}

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
