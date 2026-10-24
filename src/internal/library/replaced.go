package library

import (
	"os"
	"path/filepath"
	"strings"
	"time"
)

// retire deals with the content oldID that used to be at absPath, now that
// newID is there: the file was changed in place — a location set, a crop,
// another program's edit. Unless the old content is still at another of its
// paths (a byte-identical copy), its row goes, so the photo is not listed
// twice with its old data, and anything that names oldID is led to newID.
// What only the index knew about the old photo — the drafts it is in, and
// its notes in a library whose notes are not yet in the sidecars — goes over
// to the new one where the sidecar did not already say it. Publications do
// not: the sidecar alone says which galleries a photo is in.
func (idx *Indexer) retire(oldID, newID, absPath string) error {
	idx.changed = true
	if other := idx.store.otherPathOf(oldID, absPath); other != "" {
		if hint, _ := idx.store.GetPhotoPathHint(oldID); hint == absPath {
			return idx.store.MarkPhotoPresent(oldID, other, filepath.Base(other))
		}
		return nil
	}
	if err := idx.store.carryMeta(oldID, newID, !idx.notesInSidecars()); err != nil {
		return err
	}
	if _, err := idx.store.PurgePhotos([]string{oldID}); err != nil {
		return err
	}
	return idx.store.recordReplaced(oldID, newID)
}

// otherPathOf returns a path other than absPath where the library knows the
// photo id and a file still is, or "".
func (s *Store) otherPathOf(id, absPath string) string {
	rows, err := s.db.Query(`SELECT abs_path FROM path_cache WHERE photo_id = ? AND abs_path <> ?`, id, absPath)
	if err != nil {
		return ""
	}
	defer rows.Close()
	for rows.Next() {
		var p string
		if rows.Scan(&p) == nil {
			if _, err := os.Stat(p); err == nil {
				return p
			}
		}
	}
	return ""
}

// carryMeta copies the meta rows of oldID that newID lacks, except the
// built: keys of publications, which come from the sidecar; notes only when
// withNotes, since in a library whose notes live in the sidecars the new
// photo already has exactly what its sidecar says.
func (s *Store) carryMeta(oldID, newID string, withNotes bool) error {
	entries, err := s.GetMeta(oldID)
	if err != nil {
		return err
	}
	for _, e := range entries {
		if strings.HasPrefix(e.Key, "built:") || (IsNoteKey(e.Key) && !withNotes) {
			continue
		}
		if _, err := s.db.Exec(`INSERT OR IGNORE INTO photo_meta(photo_id, key, value, updated_at) VALUES(?,?,?,?)`,
			newID, e.Key, e.Value, e.UpdatedAt.UTC().Format(time.RFC3339)); err != nil {
			return err
		}
	}
	return nil
}

// recordReplaced leads oldID, and every ID that already led to it, to newID.
// Content changed back to what it was leads nowhere.
func (s *Store) recordReplaced(oldID, newID string) error {
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback() //nolint:errcheck
	for _, q := range []struct {
		sql  string
		args []any
	}{
		{`UPDATE photo_replaced SET new_id = ? WHERE new_id = ?`, []any{newID, oldID}},
		{`INSERT OR REPLACE INTO photo_replaced(old_id, new_id) VALUES(?, ?)`, []any{oldID, newID}},
		{`DELETE FROM photo_replaced WHERE old_id = new_id OR old_id = ?`, []any{newID}},
	} {
		if _, err := tx.Exec(q.sql, q.args...); err != nil {
			return err
		}
	}
	return tx.Commit()
}

// CurrentPhotoID returns the ID the photo id has now: id itself, or the ID
// of the content its file was changed to.
func (s *Store) CurrentPhotoID(id string) string {
	var current string
	if s.db.QueryRow(`SELECT new_id FROM photo_replaced WHERE old_id = ?`, id).Scan(&current) != nil {
		return id
	}
	return current
}

// ForgetFiles takes out of every library the photos Unterlumen just deleted
// at absPaths — files, or folders with everything below them — unless the
// same content is still at another path. Deletion through the app knows what
// is gone; a cleanup walking the folder would have to guess, and refuses to
// for a library folder left empty, which looks like a share not mounted.
func (m *Manager) ForgetFiles(absPaths []string) {
	for _, p := range absPaths {
		for _, l := range m.LibrariesForPath(p) {
			if store, err := m.OpenStore(l.ID); err == nil {
				store.forgetDeleted(p) //nolint:errcheck // the next check of the library settles what is left
			}
		}
	}
}

func (s *Store) forgetDeleted(absPath string) error {
	rows, err := s.db.Query(`SELECT abs_path, photo_id FROM path_cache WHERE abs_path = ? OR abs_path LIKE ? ESCAPE '\'`,
		absPath, escapeLikePattern(absPath)+string(filepath.Separator)+"%")
	if err != nil {
		return err
	}
	var paths, gone []string
	ids := map[string]bool{}
	for rows.Next() {
		var p, id string
		if rows.Scan(&p, &id) == nil {
			paths = append(paths, p)
			ids[id] = true
		}
	}
	rows.Close()
	if err := s.forgetPaths(paths); err != nil {
		return err
	}
	for id := range ids {
		if other := s.otherPathOf(id, ""); other == "" {
			gone = append(gone, id)
		} else if hint, _ := s.GetPhotoPathHint(id); hint == absPath || strings.HasPrefix(hint, absPath+string(filepath.Separator)) {
			s.MarkPhotoPresent(id, other, filepath.Base(other)) //nolint:errcheck
		}
	}
	_, err = s.PurgePhotos(gone)
	return err
}
