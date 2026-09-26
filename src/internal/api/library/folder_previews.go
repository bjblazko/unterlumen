package apilibrary

import (
	"net/http"

	lib "huepattl.de/unterlumen/internal/library"
	"huepattl.de/unterlumen/internal/pathguard"
)

// folderPreviews serves the subfolder tiles of a library folder: count, years
// and newest photos per subfolder. It is separate from the folder listing so
// the folder names appear at once and the tiles fill in behind them.
func folderPreviews(mgr *lib.Manager) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		store, err := mgr.OpenStore(r.PathValue("id"))
		if err != nil {
			http.Error(w, "library not found", http.StatusNotFound)
			return
		}
		defer store.Close()

		sourcePath, ok, _ := store.GetProp("source_path")
		if !ok || sourcePath == "" {
			http.Error(w, "library has no source path", http.StatusInternalServerError)
			return
		}
		// Logical only: the previews come from the index, so the source
		// volume need not be mounted.
		absPath, ok := pathguard.SafePathLogical(sourcePath, r.URL.Query().Get("path"))
		if !ok {
			http.Error(w, "invalid path", http.StatusBadRequest)
			return
		}

		previews, err := store.FolderPreviews(absPath)
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		writeJSON(w, previews)
	}
}
