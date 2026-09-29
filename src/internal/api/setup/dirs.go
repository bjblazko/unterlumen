package apisetup

import (
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strings"
)

// The setup chooses the photo folder, so it cannot browse inside it the way
// the rest of the app does: its folder picker walks the whole disk. A picker
// path is an absolute path without the leading slash — what the ordinary
// picker returns when the browse root is "/" — or, on Windows, "C:/Users/me",
// with "" for the list of drives.

// fromPickerPath turns a picker path into an absolute path.
func fromPickerPath(p string) string {
	if runtime.GOOS != "windows" {
		return filepath.Clean("/" + p)
	}
	if p == "" {
		return ""
	}
	abs := filepath.FromSlash(p)
	if strings.HasSuffix(abs, ":") {
		abs += `\`
	}
	return filepath.Clean(abs)
}

// pickerPath turns an absolute path into a picker path.
func pickerPath(abs string) string {
	return strings.Trim(filepath.ToSlash(abs), "/")
}

type dirInfo struct {
	Name string `json:"name"`
}

type dirsResponse struct {
	Path   string    `json:"path"`
	Parent *string   `json:"parent"`
	Dirs   []dirInfo `json:"dirs"`
}

// listDirs answers like /api/browse/dirs, over the whole disk.
func listDirs() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		p := strings.Trim(r.URL.Query().Get("path"), "/")
		if strings.Contains("/"+p+"/", "/../") {
			http.Error(w, "invalid path", http.StatusBadRequest)
			return
		}
		dirs, err := subfolders(p)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		var parent *string
		if p != "" {
			up := ""
			if i := strings.LastIndex(p, "/"); i >= 0 {
				up = p[:i]
			}
			parent = &up
		}
		writeJSON(w, dirsResponse{Path: p, Parent: parent, Dirs: dirs})
	}
}

// subfolders lists the visible folders in a picker path, or the drives.
func subfolders(p string) ([]dirInfo, error) {
	if runtime.GOOS == "windows" && p == "" {
		return drives(), nil
	}
	entries, err := os.ReadDir(fromPickerPath(p))
	if err != nil {
		return nil, err
	}
	var dirs []dirInfo
	for _, e := range entries {
		if e.IsDir() && !strings.HasPrefix(e.Name(), ".") {
			dirs = append(dirs, dirInfo{Name: e.Name()})
		}
	}
	return dirs, nil
}

func drives() []dirInfo {
	var dirs []dirInfo
	for c := 'A'; c <= 'Z'; c++ {
		if _, err := os.Stat(string(c) + `:\`); err == nil {
			dirs = append(dirs, dirInfo{Name: string(c) + ":"})
		}
	}
	return dirs
}
