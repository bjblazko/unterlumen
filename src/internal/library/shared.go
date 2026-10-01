package library

import (
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
)

// MarkerName is the file in a shared library's base folder that gives the
// library one identity on every installation that sees the folder (ADR-0047).
// Only a shared library has one; it holds no path, because the same folder
// has a different path on every machine.
const MarkerName = ".unterlumen-library.json"

// Marker is the content of MarkerName.
type Marker struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
}

// libraryIDPattern is what newUUID makes. A marker's ID becomes a folder name
// under -lib-dir, so nothing else is accepted from a file on a share.
var libraryIDPattern = regexp.MustCompile(`^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$`)

// ReadMarker returns the marker in base, or nil when there is none.
func ReadMarker(base string) (*Marker, error) {
	data, err := os.ReadFile(filepath.Join(base, MarkerName))
	if errors.Is(err, fs.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var mk Marker
	if err := json.Unmarshal(data, &mk); err != nil {
		return nil, fmt.Errorf("reading %s: %w", filepath.Join(base, MarkerName), err)
	}
	if !libraryIDPattern.MatchString(mk.ID) || mk.Name == "" {
		return nil, fmt.Errorf("%s names no library", filepath.Join(base, MarkerName))
	}
	return &mk, nil
}

func writeMarker(base string, mk Marker) error {
	data, err := json.MarshalIndent(mk, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(base, MarkerName), append(data, '\n'), 0o644)
}

// ownMarker is the marker in l's base folder when it names l itself.
func ownMarker(l *Library) *Marker {
	mk, err := ReadMarker(l.SourcePath)
	if err != nil || mk == nil || mk.ID != l.ID {
		return nil
	}
	return mk
}

// Annotate fills in what only the base folder knows: whether it is reachable,
// whether this library is shared, and whether another installation shares the
// folder under another identity. A shared library takes its name and
// description from the marker, so a rename on the other installation shows
// here. It reads the disk, so it is for the library list and detail only.
func (m *Manager) Annotate(l *Library) {
	if info, err := os.Stat(l.SourcePath); err != nil || !info.IsDir() {
		l.Missing = true
		return
	}
	mk, err := ReadMarker(l.SourcePath)
	if err != nil || mk == nil {
		return
	}
	if mk.ID != l.ID {
		if _, err := m.GetLibrary(mk.ID); err != nil { // already here under that ID: nothing to join
			l.JoinOffer = mk
		}
		return
	}
	l.Shared = true
	if mk.Name != l.Name || mk.Description != l.Description {
		if _, err := m.setNameAndDescription(l.ID, mk.Name, mk.Description); err == nil {
			l.Name, l.Description = mk.Name, mk.Description
		}
	}
}

// ShareLibrary writes the marker that lets other installations recognise the
// library's base folder.
func (m *Manager) ShareLibrary(id string) (*Library, error) {
	l, err := m.GetLibrary(id)
	if err != nil {
		return nil, err
	}
	if mk, err := ReadMarker(l.SourcePath); err == nil && mk != nil && mk.ID != id {
		return nil, fmt.Errorf("another installation already shares this folder as %q; join it instead", mk.Name)
	}
	if err := writeMarker(l.SourcePath, Marker{ID: id, Name: l.Name, Description: l.Description}); err != nil {
		return nil, fmt.Errorf("the folder cannot be marked as shared: %w", err)
	}
	l.Shared = true
	return l, nil
}

// UnshareLibrary removes the library's marker. A marker that names another
// library is not this installation's to remove.
func (m *Manager) UnshareLibrary(id string) (*Library, error) {
	l, err := m.GetLibrary(id)
	if err != nil {
		return nil, err
	}
	if ownMarker(l) != nil {
		if err := os.Remove(filepath.Join(l.SourcePath, MarkerName)); err != nil {
			return nil, err
		}
	}
	return l, nil
}

// AddSharedLibrary adds the library another installation shares in
// sourcePath, under that library's ID and name.
func (m *Manager) AddSharedLibrary(sourcePath string) (*Library, error) {
	mk, err := ReadMarker(sourcePath)
	if err != nil {
		return nil, err
	}
	if mk == nil {
		return nil, fmt.Errorf("%s is not a shared library", sourcePath)
	}
	if _, err := m.GetLibrary(mk.ID); err == nil {
		return nil, fmt.Errorf("the shared library %q is already here", mk.Name)
	}
	l, err := m.createLibrary(mk.ID, mk.Name, mk.Description, sourcePath)
	if err != nil {
		return nil, err
	}
	l.Shared = true
	return l, nil
}

// JoinShared makes the library id the shared library its base folder is
// marked as: it takes the marker's ID, name and description. The index and
// the thumbnails stay, so nothing is scanned again. It returns the library
// under its new ID; the caller rewrites what else refers to the old one.
func (m *Manager) JoinShared(id string) (*Library, error) {
	l, err := m.GetLibrary(id)
	if err != nil {
		return nil, err
	}
	mk, err := ReadMarker(l.SourcePath)
	if err != nil {
		return nil, err
	}
	if mk == nil || mk.ID == id {
		return nil, fmt.Errorf("%s is not shared by another installation", l.SourcePath)
	}
	if _, err := m.GetLibrary(mk.ID); err == nil {
		return nil, fmt.Errorf("the shared library %q is already here", mk.Name)
	}
	if err := m.rekey(id, mk.ID); err != nil {
		return nil, err
	}
	joined, err := m.setNameAndDescription(mk.ID, mk.Name, mk.Description)
	if err != nil {
		return nil, err
	}
	joined.Shared = true
	return joined, nil
}

// rekey moves the library's data folder from oldID to newID. A library that
// is being scanned or analysed is not moved.
func (m *Manager) rekey(oldID, newID string) error {
	if !m.TryLockIndex(oldID) {
		return fmt.Errorf("the library is being scanned; join it when the scan has finished")
	}
	defer m.UnlockIndex(oldID)
	if _, running := m.analysing.Load(oldID); running {
		return fmt.Errorf("the library's photos are being measured; join it when that has finished")
	}
	m.dbMu.Lock()
	defer m.dbMu.Unlock()
	if db, ok := m.openDBs.LoadAndDelete(oldID); ok {
		db.(*sql.DB).Close()
	}
	m.InvalidateStatsCache(oldID)
	return os.Rename(m.LibDir(oldID), m.LibDir(newID))
}
