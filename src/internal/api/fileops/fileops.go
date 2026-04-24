package fileops

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
	"huepattl.de/unterlumen/internal/pathguard"
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

// Handle registers all file-operation routes on mux.
func Handle(mux *http.ServeMux, root string, cache *media.ScanCache) {
	mux.HandleFunc("/api/copy", handleCopy(root, cache))
	mux.HandleFunc("/api/move", handleMove(root, cache))
	mux.HandleFunc("/api/delete", handleDelete(root, cache))
	mux.HandleFunc("/api/mkdir", handleMkdir(root, cache))
	mux.HandleFunc("/api/rename", handleRename(root, cache))
	mux.HandleFunc("/api/list-recursive", handleListRecursive(root))
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
			result, dir := deleteEntry(root, file, cache)
			results = append(results, result)
			if result.Success && dir != "" {
				dirsToInvalidate[dir] = struct{}{}
			}
		}
		for dir := range dirsToInvalidate {
			cache.Invalidate(dir)
		}

		writeJSON(w, fileOpResponse{Results: results})
	}
}

func deleteEntry(root, file string, cache *media.ScanCache) (fileOpResult, string) {
	filePath, ok := pathguard.SafePath(root, file)
	if !ok {
		return fileOpResult{File: file, Error: "invalid path"}, ""
	}

	info, err := os.Stat(filePath)
	if err != nil {
		return fileOpResult{File: file, Error: err.Error()}, ""
	}

	if info.IsDir() {
		if err := os.RemoveAll(filePath); err != nil {
			return fileOpResult{File: file, Error: err.Error()}, ""
		}
		cache.InvalidatePrefix(filePath)
		return fileOpResult{File: file, Success: true}, filepath.Dir(filePath)
	}

	if err := os.Remove(filePath); err != nil {
		return fileOpResult{File: file, Error: err.Error()}, ""
	}
	return fileOpResult{File: file, Success: true}, filepath.Dir(filePath)
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

	destDir, ok := pathguard.SafePath(root, req.Destination)
	if !ok {
		http.Error(w, "Invalid destination path", http.StatusBadRequest)
		return
	}
	info, err := os.Stat(destDir)
	if err != nil || !info.IsDir() {
		http.Error(w, "Destination is not a valid directory", http.StatusBadRequest)
		return
	}

	dirsToInvalidate := make(map[string]struct{})
	var results []fileOpResult
	for _, file := range req.Files {
		srcPath, ok := pathguard.SafePath(root, file)
		if !ok {
			results = append(results, fileOpResult{File: file, Error: "invalid source path"})
			continue
		}
		dstPath := filepath.Join(destDir, filepath.Base(srcPath))
		if err := op(srcPath, dstPath); err != nil {
			results = append(results, fileOpResult{File: file, Error: err.Error()})
		} else {
			dirsToInvalidate[filepath.Dir(srcPath)] = struct{}{}
			dirsToInvalidate[destDir] = struct{}{}
			results = append(results, fileOpResult{File: file, Success: true})
		}
	}
	for dir := range dirsToInvalidate {
		cache.Invalidate(dir)
	}

	writeJSON(w, fileOpResponse{Results: results})
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

		dirPath, ok := pathguard.SafePath(root, req.Path)
		if !ok {
			http.Error(w, "Invalid path", http.StatusBadRequest)
			return
		}

		if err := os.Mkdir(dirPath, 0755); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		cache.Invalidate(filepath.Dir(dirPath))
		writeJSON(w, map[string]bool{"success": true})
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

		if req.Name == "" || req.Name == "." || req.Name == ".." || filepath.Base(req.Name) != req.Name {
			http.Error(w, "Invalid name", http.StatusBadRequest)
			return
		}

		srcPath, ok := pathguard.SafePath(root, req.Path)
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
		writeJSON(w, map[string]bool{"success": true})
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

		dirPath, ok := pathguard.SafePath(root, req.Path)
		if !ok {
			http.Error(w, "Invalid path", http.StatusBadRequest)
			return
		}
		info, err := os.Stat(dirPath)
		if err != nil || !info.IsDir() {
			http.Error(w, "Not a directory", http.StatusBadRequest)
			return
		}

		files, dirs, err := walkDirEntries(root, dirPath)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		writeJSON(w, map[string]interface{}{"files": files, "dirs": dirs})
	}
}

func walkDirEntries(root, dirPath string) (files, dirs []string, err error) {
	const maxEntries = 100000
	count := 0

	walkErr := filepath.WalkDir(dirPath, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		count++
		if count > maxEntries {
			return fmt.Errorf("too many entries")
		}
		rel, err := filepath.Rel(root, path)
		if err != nil {
			return nil
		}
		if d.IsDir() {
			if path != dirPath {
				dirs = append(dirs, rel)
			}
		} else {
			files = append(files, rel)
		}
		return nil
	})

	if walkErr != nil && !strings.Contains(walkErr.Error(), "too many entries") {
		return nil, nil, walkErr
	}

	sort.Slice(dirs, func(i, j int) bool {
		di := strings.Count(dirs[i], string(filepath.Separator))
		dj := strings.Count(dirs[j], string(filepath.Separator))
		if di != dj {
			return di < dj
		}
		return dirs[i] < dirs[j]
	})

	return files, dirs, nil
}

func copyFile(src, dst string) error {
	info, err := os.Stat(src)
	if err != nil {
		return err
	}
	if info.IsDir() {
		return copyDir(src, dst)
	}
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
	info, err := os.Stat(src)
	if err != nil {
		return err
	}
	if info.IsDir() {
		if _, err := os.Stat(dst); err == nil {
			return fmt.Errorf("destination already exists")
		}
		if err := os.Rename(src, dst); err == nil {
			return nil
		}
		if err := copyDir(src, dst); err != nil {
			return err
		}
		return os.RemoveAll(src)
	}

	if _, err := os.Stat(dst); err == nil {
		return fmt.Errorf("destination file already exists")
	}
	if err := os.Rename(src, dst); err == nil {
		return nil
	}
	if err := copyFile(src, dst); err != nil {
		return err
	}
	return os.Remove(src)
}

func writeJSON(w http.ResponseWriter, v interface{}) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(v)
}
