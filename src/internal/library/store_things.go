package library

import (
	"database/sql"
	"fmt"

	"huepattl.de/unterlumen/internal/media"
	"huepattl.de/unterlumen/internal/things"
)

// ThingTask is a photo to look at for things. Vector is its stored vector
// when only the word list changed since it was looked at: it is scored again
// without running the models.
type ThingTask struct {
	ID        string
	ThumbPath string // relative to the library's directory
	PathHint  string // the photo's file, for its sidecar
	Vector    []float32
}

// PhotosNeedingThings lists the photos with a thumbnail that no current
// models and word list looked at, and those whose sidecar has their things
// but not yet their vector, which this index has: it is written there, so
// the other installation need not run the models.
func (s *Store) PhotosNeedingThings() ([]ThingTask, error) {
	rows, err := s.db.Query(`
		SELECT p.id, p.thumb_path, p.path_hint,
		       CASE WHEN f.version = ? THEN v.vector END
		FROM photos p
		LEFT JOIN photo_things_found f ON f.photo_id = p.id
		LEFT JOIN photo_vectors v ON v.photo_id = p.id
		WHERE p.status = 'ok' AND p.thumb_path IS NOT NULL AND p.thumb_path != '' AND `+notVideo+`
		  AND (f.photo_id IS NULL OR f.version < ? OR f.vocabulary < ?
		       OR f.written = 1 AND f.vector_in_sidecar = 0 AND f.version = ? AND v.vector IS NOT NULL)`,
		things.Version, things.Version, things.VocabularyVersion, things.Version)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var tasks []ThingTask
	for rows.Next() {
		var t ThingTask
		var vec []byte
		if err := rows.Scan(&t.ID, &t.ThumbPath, &t.PathHint, &vec); err != nil {
			return nil, err
		}
		if len(vec) > 0 {
			t.Vector = decodeEmbedding(vec)
		}
		tasks = append(tasks, t)
	}
	return tasks, rows.Err()
}

// FoundThings is what a pass found in one photo. Vector is nil when the
// index's own stays; Boxes nil when the detector did not look. Written says
// the sidecar holds the things, VectorInSidecar the vector too.
type FoundThings struct {
	PhotoID         string
	Things          []things.Found
	Vector          []float32
	Boxes           []things.Box
	Written         bool
	VectorInSidecar bool
}

// SaveFoundThings stores what a pass found in several photos in one
// transaction, in place of what an older pass found. A photo deleted since
// is passed over: the pass runs beside scans.
func (s *Store) SaveFoundThings(items []FoundThings) error {
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback() //nolint:errcheck
	for _, it := range items {
		if err := saveFoundThings(tx, it); err != nil {
			return err
		}
	}
	return tx.Commit()
}

func saveFoundThings(tx *sql.Tx, it FoundThings) error {
	var exists int
	if err := tx.QueryRow(`SELECT COUNT(*) FROM photos WHERE id = ?`, it.PhotoID).Scan(&exists); err != nil || exists == 0 {
		return err
	}
	if it.Vector != nil {
		if _, err := tx.Exec(`INSERT OR REPLACE INTO photo_vectors (photo_id, vector) VALUES (?, ?)`, it.PhotoID, encodeEmbedding(it.Vector)); err != nil {
			return err
		}
	}
	if it.Boxes != nil {
		if err := replaceBoxes(tx, it.PhotoID, it.Boxes); err != nil {
			return err
		}
	}
	if err := replaceThings(tx, it.PhotoID, it.Things); err != nil {
		return err
	}
	_, err := tx.Exec(`INSERT OR REPLACE INTO photo_things_found (photo_id, version, vocabulary, written, vector_in_sidecar) VALUES (?, ?, ?, ?, ?)`,
		it.PhotoID, things.Version, things.VocabularyVersion, it.Written, it.VectorInSidecar)
	return err
}

func replaceBoxes(tx *sql.Tx, photoID string, boxes []things.Box) error {
	if _, err := tx.Exec(`DELETE FROM thing_boxes WHERE photo_id = ?`, photoID); err != nil {
		return err
	}
	for _, b := range boxes {
		if _, err := tx.Exec(`INSERT INTO thing_boxes (photo_id, class, score, x, y, w, h) VALUES (?,?,?,?,?,?,?)`,
			photoID, b.Class, b.Score, b.X, b.Y, b.W, b.H); err != nil {
			return err
		}
	}
	return nil
}

func replaceThings(tx *sql.Tx, photoID string, found []things.Found) error {
	if _, err := tx.Exec(`DELETE FROM photo_things WHERE photo_id = ?`, photoID); err != nil {
		return err
	}
	for _, f := range found {
		if _, err := tx.Exec(`INSERT OR IGNORE INTO photo_things (photo_id, thing, score, source) VALUES (?,?,?,?)`,
			photoID, f.Name, f.Score, f.Source); err != nil {
			return err
		}
	}
	return nil
}

// ApplySidecarThings makes a photo's things what its sidecar says, when the
// sidecar holds Unterlumen's record of them (ADR-0052): things taken away
// there, or whose keyword another program removed, go; things the other
// installation found come. A sidecar without the record leaves the things
// alone — it could not be written — unless the record was written once:
// then the sidecar was replaced, and the photo is looked at again.
func (s *Store) ApplySidecarThings(photoID string, st media.SidecarThings) error {
	if st.Record.Version == "" {
		_, err := s.db.Exec(`DELETE FROM photo_things_found WHERE photo_id = ? AND written = 1`, photoID)
		return err
	}
	// Before the transaction: the lexicon may read the library's
	// properties, and the transaction holds the library's one connection.
	names := st.Things(s.lexicon())
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback() //nolint:errcheck
	scores, err := thingScores(tx, photoID)
	if err != nil {
		return err
	}
	found := make([]things.Found, 0, len(names))
	for _, name := range names {
		f, ok := scores[name]
		if !ok {
			f = things.Found{Name: name, Source: "sidecar"}
		}
		found = append(found, f)
	}
	if err := replaceThings(tx, photoID, found); err != nil {
		return err
	}
	if !things.Current(st.Record.Version) {
		return tx.Commit() // a vector of older models is no use here
	}
	if err := takeSidecarVector(tx, photoID, st.Record.Vector); err != nil {
		return err
	}
	return tx.Commit()
}

// takeSidecarVector keeps the vector the other installation wrote into the
// sidecar, when this index has none: searching by words then needs no
// models here. An index that has one keeps it — rewriting it with every
// check would make the search read every vector again.
func takeSidecarVector(tx *sql.Tx, photoID string, vector []float32) error {
	if vector == nil {
		return nil
	}
	if _, err := tx.Exec(`INSERT OR IGNORE INTO photo_vectors (photo_id, vector) VALUES (?, ?)`, photoID, encodeEmbedding(vector)); err != nil {
		return err
	}
	_, err := tx.Exec(`UPDATE photo_things_found SET vector_in_sidecar = 1 WHERE photo_id = ?`, photoID)
	return err
}

func thingScores(tx *sql.Tx, photoID string) (map[string]things.Found, error) {
	rows, err := tx.Query(`SELECT thing, score, source FROM photo_things WHERE photo_id = ?`, photoID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := map[string]things.Found{}
	for rows.Next() {
		var f things.Found
		if err := rows.Scan(&f.Name, &f.Score, &f.Source); err != nil {
			return nil, err
		}
		out[f.Name] = f
	}
	return out, rows.Err()
}

// ThingsOf lists the things in a photo, surest first.
func (s *Store) ThingsOf(photoID string) ([]things.Found, error) {
	rows, err := s.db.Query(`SELECT thing, score, source FROM photo_things WHERE photo_id = ? ORDER BY score DESC, thing`, photoID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []things.Found
	for rows.Next() {
		var f things.Found
		if err := rows.Scan(&f.Name, &f.Score, &f.Source); err != nil {
			return nil, err
		}
		out = append(out, f)
	}
	return out, rows.Err()
}

// forgetThing removes a thing from a photo in the index; RemoveThing writes
// the sidecar first.
func (s *Store) forgetThing(photoID, thing string) error {
	_, err := s.db.Exec(`DELETE FROM photo_things WHERE photo_id = ? AND thing = ?`, photoID, thing)
	return err
}

// ThingCount is a thing and the number of photos it is in.
type ThingCount struct {
	Thing  string `json:"thing"`
	Photos int    `json:"photos"`
}

// ThingCounts lists every thing found in the library with its number of
// photos, most first.
func (s *Store) ThingCounts() ([]ThingCount, error) {
	rows, err := s.db.Query(`SELECT thing, COUNT(*) FROM photo_things GROUP BY thing ORDER BY COUNT(*) DESC, thing`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []ThingCount
	for rows.Next() {
		var c ThingCount
		if err := rows.Scan(&c.Thing, &c.Photos); err != nil {
			return nil, err
		}
		out = append(out, c)
	}
	return out, rows.Err()
}

// boxesOf returns the detector's boxes in a photo.
func (s *Store) boxesOf(photoID string) ([]things.Box, error) {
	rows, err := s.db.Query(`SELECT class, score, x, y, w, h FROM thing_boxes WHERE photo_id = ?`, photoID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []things.Box
	for rows.Next() {
		var b things.Box
		if err := rows.Scan(&b.Class, &b.Score, &b.X, &b.Y, &b.W, &b.H); err != nil {
			return nil, err
		}
		out = append(out, b)
	}
	return out, rows.Err()
}

// RemoveThing takes a thing away from a photo: from its sidecar first, where
// it is remembered so no pass finds it again, then from the index. When the
// sidecar cannot be written the index is left as it is.
func (s *Store) RemoveThing(photoID, thing string) error {
	path, err := s.GetPhotoPathHint(photoID)
	if err != nil || path == "" {
		return fmt.Errorf("the photo's file is not known; scan the library and try again")
	}
	if err := media.RemoveThing(path, thing, s.lexicon()); err != nil {
		return fmt.Errorf("the sidecar beside the photo could not be written, so nothing was changed: %w", err)
	}
	return s.forgetThing(photoID, thing)
}

// ThingAppearances lists every thing in every photo in scope, through
// aliasCond so the folder and the shared filter apply (ADR-0050).
func (s *Store) ThingAppearances(pathGlob string) ([]Appearance, error) {
	where, args := s.aliasCond(pathGlob)
	rows, err := s.db.Query(`
		SELECT t.thing, p.id, COALESCE(p.date_taken, '')
		FROM photo_things t JOIN photos p ON p.id = t.photo_id
		WHERE p.status = 'ok'`+where, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Appearance{}
	err = scanRows(rows, func() error {
		var a Appearance
		if err := rows.Scan(&a.Name, &a.Photo, &a.Date); err != nil {
			return err
		}
		out = append(out, a)
		return nil
	})
	return out, err
}

// ThingsLooked counts the photos in scope that the current models looked
// at for things; those of older ones are still to be looked at again.
func (s *Store) ThingsLooked(pathGlob string) (int, error) {
	where, args := s.aliasCond(pathGlob)
	var n int
	err := s.db.QueryRow(`SELECT COUNT(*) FROM photos p JOIN photo_things_found f ON f.photo_id = p.id AND f.version = ?
		WHERE p.status = 'ok'`+where, append([]any{things.Version}, args...)...).Scan(&n)
	return n, err
}

// thingsWritten says whether the photo's sidecar was given its things record.
func (s *Store) thingsWritten(photoID string) bool {
	var n int
	s.db.QueryRow(`SELECT COUNT(*) FROM photo_things_found WHERE photo_id = ? AND written = 1`, photoID).Scan(&n) //nolint:errcheck
	return n > 0
}
