// Package download serves a photo as the file it is, for saving: the
// original bytes, never the JPEG the viewer shows for a HEIF file, under
// the file's own name.
package download

import (
	"mime"
	"net/http"
	"os"
	"path/filepath"
)

// Requested reports whether the request asks for the original file
// (?download=1) rather than the picture to show.
func Requested(r *http.Request) bool {
	return r.URL.Query().Get("download") == "1"
}

// Original writes the file at path as an attachment. path must already be
// checked against the browse root or come from a library's index.
func Original(w http.ResponseWriter, r *http.Request, path string) {
	f, err := os.Open(path)
	if err != nil {
		if os.IsNotExist(err) {
			http.Error(w, "photo not found on disk", http.StatusNotFound)
		} else {
			http.Error(w, "could not open the photo: "+err.Error(), http.StatusInternalServerError)
		}
		return
	}
	defer f.Close()
	info, err := f.Stat()
	if err != nil || info.IsDir() {
		http.Error(w, "not a photo", http.StatusBadRequest)
		return
	}
	name := filepath.Base(path)
	// FormatMediaType writes filename*=utf-8'' for names that are not
	// plain ASCII, so umlauts and spaces survive the download.
	w.Header().Set("Content-Disposition", mime.FormatMediaType("attachment", map[string]string{"filename": name}))
	w.Header().Set("Cache-Control", "no-cache")
	http.ServeContent(w, r, name, info.ModTime(), f)
}
