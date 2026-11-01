package fileops

import (
	"encoding/json"
	"io/fs"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"huepattl.de/unterlumen/internal/library"
	"huepattl.de/unterlumen/internal/media"
	"huepattl.de/unterlumen/internal/pathguard"
)

// --- Folders: renaming, and removing what a delete left empty ---

// libraryInside names a library whose folder is absDir or lies inside it.
// Renaming or deleting such a folder would leave the library pointing at
// nothing, so it is refused and the library is named.
func libraryInside(libMgr *library.Manager, absDir string) (*library.Library, bool) {
	if libMgr == nil {
		return nil, false
	}
	libs, err := libMgr.ListLibraries()
	if err != nil {
		return nil, false
	}
	dir := strings.TrimSuffix(absDir, "/")
	for _, l := range libs {
		sp := strings.TrimSuffix(l.SourcePath, "/")
		if sp != "" && (sp == dir || strings.HasPrefix(sp, dir+"/")) {
			return l, true
		}
	}
	return nil, false
}

// movedFilePairs lists the photos of a folder that was renamed or moved
// from srcDir to dstDir, as [old, new] absolute paths, so the library keeps
// what it knew of each instead of forgetting the folder and finding its
// photos again as new ones.
func movedFilePairs(srcDir, dstDir string) [][2]string {
	var pairs [][2]string
	filepath.WalkDir(dstDir, func(path string, d fs.DirEntry, err error) error { //nolint:errcheck
		if err != nil || d.IsDir() || !media.IsSupportedMedia(d.Name()) {
			return nil
		}
		rel, err := filepath.Rel(dstDir, path)
		if err == nil {
			pairs = append(pairs, [2]string{filepath.Join(srcDir, rel), path})
		}
		return nil
	})
	return pairs
}

// filePairs turns the paths a copy or move took into file pairs: a folder
// stands for every photo in it.
func filePairs(srcPaths, dstPaths []string) [][2]string {
	var pairs [][2]string
	for i := range dstPaths {
		if info, err := os.Stat(dstPaths[i]); err == nil && info.IsDir() {
			pairs = append(pairs, movedFilePairs(srcPaths[i], dstPaths[i])...)
			continue
		}
		pairs = append(pairs, [2]string{srcPaths[i], dstPaths[i]})
	}
	return pairs
}

// reindexRenamedFolder moves a renamed folder's photos to their new paths
// in the library that holds them.
func reindexRenamedFolder(libMgr *library.Manager, srcDir, dstDir string) {
	if libMgr == nil {
		return
	}
	if lib, ok := libMgr.FindLibraryForPath(dstDir); ok {
		libMgr.IndexMovedFiles(lib.ID, movedFilePairs(srcDir, dstDir))
	}
}

// leftBehind are files a system puts into folders by itself. A folder that
// holds nothing else is empty to the person who emptied it.
func leftBehind(name string) bool {
	switch strings.ToLower(name) {
	case ".ds_store", "thumbs.db", "desktop.ini", ".localized":
		return true
	}
	return strings.HasPrefix(name, "._")
}

// emptyFolder says whether dir holds nothing but what a system leaves behind.
func emptyFolder(dir string) bool {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return false
	}
	for _, e := range entries {
		if e.IsDir() || !leftBehind(e.Name()) {
			return false
		}
	}
	return true
}

// removableEmptyFolder resolves a browse-root relative folder and says
// whether it may be removed as empty: never the browse root itself, never
// a library's folder, and only while it is empty.
func removableEmptyFolder(root, rel string, libMgr *library.Manager) (string, bool) {
	if rel == "" || rel == "." {
		return "", false
	}
	abs, ok := pathguard.SafePath(root, rel)
	if !ok {
		return "", false
	}
	if _, isLibrary := libraryInside(libMgr, abs); isLibrary {
		return "", false
	}
	return abs, emptyFolder(abs)
}

type foldersRequest struct {
	Folders []string `json:"folders"`
}

func decodeFolders(w http.ResponseWriter, r *http.Request) ([]string, bool) {
	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return nil, false
	}
	var req foldersRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return nil, false
	}
	return req.Folders, true
}

// handleEmptiedFolders answers which of the folders (browse-root relative)
// a delete left empty, so the person can be asked to remove them too.
func handleEmptiedFolders(root string, libMgr *library.Manager) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		folders, ok := decodeFolders(w, r)
		if !ok {
			return
		}
		empty := []string{}
		for _, rel := range folders {
			if _, ok := removableEmptyFolder(root, rel, libMgr); ok {
				empty = append(empty, rel)
			}
		}
		writeJSON(w, map[string][]string{"empty": empty})
	}
}

// handleRemoveEmptyFolders removes folders that are still empty, with what
// a system left in them; a folder that got something in the meantime stays.
func handleRemoveEmptyFolders(root string, cache *media.ScanCache, libMgr *library.Manager) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		folders, ok := decodeFolders(w, r)
		if !ok {
			return
		}
		var results []fileOpResult
		for _, rel := range folders {
			results = append(results, removeEmptyFolder(root, rel, cache, libMgr))
		}
		writeJSON(w, fileOpResponse{Results: results})
	}
}

func removeEmptyFolder(root, rel string, cache *media.ScanCache, libMgr *library.Manager) fileOpResult {
	abs, ok := removableEmptyFolder(root, rel, libMgr)
	if !ok {
		return fileOpResult{File: rel, Error: "the folder is not empty any more, or is a library's folder"}
	}
	entries, _ := os.ReadDir(abs)
	for _, e := range entries {
		os.Remove(filepath.Join(abs, e.Name())) //nolint:errcheck // os.Remove of the folder says what is left
	}
	if err := os.Remove(abs); err != nil {
		return fileOpResult{File: rel, Error: err.Error()}
	}
	cache.InvalidatePrefix(abs)
	cache.Invalidate(filepath.Dir(abs))
	return fileOpResult{File: rel, Success: true}
}
