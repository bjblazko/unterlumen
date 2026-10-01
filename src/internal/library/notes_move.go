package library

import (
	"log"
	"os"

	"huepattl.de/unterlumen/internal/media"
)

// noteRow is one note as the index holds it, with the photo's last known path.
type noteRow struct {
	photoID, path, key, value string
}

func (s *Store) noteRows() ([]noteRow, error) {
	rows, err := s.db.Query(`SELECT m.photo_id, p.path_hint, m.key, m.value
		FROM photo_meta m JOIN photos p ON p.id = m.photo_id
		WHERE m.key NOT LIKE 'built:%' AND m.key NOT LIKE 'published:%' AND m.key NOT LIKE 'pending:%'
		ORDER BY m.photo_id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []noteRow
	for rows.Next() {
		var r noteRow
		if err := rows.Scan(&r.photoID, &r.path, &r.key, &r.value); err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

// MoveAllNotesToSidecars writes the notes every library's index alone holds
// into the sidecars, once per library (ADR-0048).
func (m *Manager) MoveAllNotesToSidecars() {
	libs, err := m.ListLibraries()
	if err != nil {
		return
	}
	for _, l := range libs {
		m.moveNotesToSidecars(l)
	}
}

// moveNotesToSidecars writes the library's notes that its sidecars lack into
// them. A sidecar that already has a different value keeps it: it is what
// other installations see. A library whose folder is not reachable, or whose
// sidecars could not all be written, is tried again on the next start. It
// holds the index lock, so no scan drops a note while it is being moved.
func (m *Manager) moveNotesToSidecars(l *Library) {
	if info, err := os.Stat(l.SourcePath); err != nil || !info.IsDir() {
		return
	}
	if !m.TryLockIndex(l.ID) {
		return
	}
	defer m.UnlockIndex(l.ID)
	store, err := m.OpenStore(l.ID)
	if err != nil || store.NotesInSidecars() {
		return
	}
	rows, err := store.noteRows()
	if err != nil {
		return
	}
	written, kept, failed := moveNoteRows(rows)
	if written+kept+failed > 0 {
		log.Printf("Library %q: %d notes written to sidecars, %d kept as the sidecar has them, %d not written", l.Name, written, kept, failed)
	}
	if failed == 0 {
		store.SetProp(notesInSidecarProp, "1") //nolint:errcheck
	}
}

func moveNoteRows(rows []noteRow) (written, kept, failed int) {
	sidecars := map[string]media.Notes{}
	for _, r := range rows {
		if _, err := os.Stat(r.path); err != nil {
			continue // the photo is gone; its sidecar must not outlive it
		}
		notes, ok := sidecars[r.path]
		if !ok {
			var err error
			if notes, err = media.ReadNotes(r.path); err != nil {
				failed++
				continue
			}
			sidecars[r.path] = notes
		}
		current := notes.Fields[r.key]
		if r.key == "title" {
			current = notes.Title
		}
		switch {
		case current == r.value:
		case current != "":
			kept++
		case writeNote(r.path, r.key, r.value) != nil:
			failed++
		default:
			written++
		}
	}
	return written, kept, failed
}

func writeNote(path, key, value string) error {
	if key == "title" {
		return media.WriteTitle(path, value)
	}
	return media.WriteField(path, key, value)
}
