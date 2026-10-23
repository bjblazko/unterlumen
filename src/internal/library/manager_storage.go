package library

import (
	"io/fs"
	"path/filepath"
)

// What this installation keeps in its data folder, measured for Settings.
// Unlike the cache, none of it is a mere copy: a library's thumbnails carry
// where its faces are, its index what the scans and the owner put there, so
// it is shown, never cleared from Settings.

// LibraryStorage is the room one library's data takes: thumbnails, faces
// and index together.
type LibraryStorage struct {
	ID    string `json:"id"`
	Name  string `json:"name"`
	Bytes int64  `json:"bytes"`
}

// DataStorage is the data folder, part by part.
type DataStorage struct {
	Path        string           `json:"path"`
	Libraries   []LibraryStorage `json:"libraries"`
	Recognition int64            `json:"recognition"` // the face models
	Things      int64            `json:"things"`      // the things models
	Runtime     int64            `json:"runtime"`     // ONNX Runtime, which both use
	Output      int64            `json:"output"`      // what destinations built on this computer
}

// DataStorage measures the data folder.
func (m *Manager) DataStorage() DataStorage {
	ds := DataStorage{Path: m.root, Libraries: []LibraryStorage{}}
	if libs, err := m.ListLibraries(); err == nil {
		for _, l := range libs {
			ds.Libraries = append(ds.Libraries, LibraryStorage{l.ID, l.Name, folderBytes(m.LibDir(l.ID))})
		}
	}
	ds.Recognition = folderBytes(m.RecognitionDir())
	ds.Things = folderBytes(m.ThingsDir())
	ds.Runtime = folderBytes(m.RuntimeDir())
	ds.Output = folderBytes(filepath.Join(m.root, "channels"))
	return ds
}

func folderBytes(dir string) int64 {
	var n int64
	filepath.WalkDir(dir, func(_ string, d fs.DirEntry, err error) error { //nolint:errcheck
		if err == nil && !d.IsDir() {
			if fi, err := d.Info(); err == nil {
				n += fi.Size()
			}
		}
		return nil
	})
	return n
}
