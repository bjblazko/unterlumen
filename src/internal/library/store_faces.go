package library

import (
	"database/sql"
	"encoding/binary"
	"math"

	"huepattl.de/unterlumen/internal/faces"
)

// FaceTask is a photo whose faces are still to be found.
type FaceTask struct {
	ID        string
	ThumbPath string // relative to the library's directory
	PathHint  string // the photo's file, for the names in its sidecar
	// Orientation is the photo's EXIF orientation: a sidecar's regions are
	// for the photo before it is turned upright.
	Orientation int
}

// PhotosNeedingFaces lists the photos with a thumbnail whose faces were not
// looked for, or were looked for by an older faces.Version.
func (s *Store) PhotosNeedingFaces() ([]FaceTask, error) {
	rows, err := s.db.Query(`
		SELECT p.id, p.thumb_path, p.path_hint, COALESCE(CAST(TRIM(o.value, '"') AS INTEGER), 1) FROM photos p
		LEFT JOIN photo_faces_found f ON f.photo_id = p.id
		LEFT JOIN exif_index o ON o.photo_id = p.id AND o.field = 'Orientation'
		WHERE p.status = 'ok' AND p.thumb_path IS NOT NULL AND p.thumb_path != ''
		  AND (f.photo_id IS NULL OR f.version < ?)`, faces.Version)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var tasks []FaceTask
	for rows.Next() {
		var t FaceTask
		if err := rows.Scan(&t.ID, &t.ThumbPath, &t.PathHint, &t.Orientation); err != nil {
			return nil, err
		}
		tasks = append(tasks, t)
	}
	return tasks, rows.Err()
}

// FoundFaces is what a pass found in one photo, with the name its sidecar
// gives each face, if any.
type FoundFaces struct {
	PhotoID string
	Faces   []faces.Face
	Names   []string // one per face; "" for a face the sidecar does not name
}

// SaveFoundFaces stores the faces of several photos in one transaction,
// replacing what an older pass found in them; faces marked by hand stay
// (settleMarkedFaces). A photo deleted since is
// passed over: the pass runs beside scans.
func (s *Store) SaveFoundFaces(items []FoundFaces) error {
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback() //nolint:errcheck
	for _, it := range items {
		if err := saveFoundFaces(tx, it); err != nil {
			return err
		}
	}
	return tx.Commit()
}

func saveFoundFaces(tx *sql.Tx, it FoundFaces) error {
	var exists int
	if err := tx.QueryRow(`SELECT COUNT(*) FROM photos WHERE id = ?`, it.PhotoID).Scan(&exists); err != nil || exists == 0 {
		return err
	}
	if _, err := tx.Exec(`DELETE FROM faces WHERE photo_id = ? AND manual = 0`, it.PhotoID); err != nil {
		return err
	}
	for i, f := range it.Faces {
		var person any
		confirmed := 0
		if it.Names[i] != "" {
			id, err := personID(tx, it.Names[i])
			if err != nil {
				return err
			}
			person, confirmed = id, 1
		}
		if _, err := tx.Exec(`INSERT INTO faces (photo_id, x, y, w, h, score, embedding, person_id, confirmed) VALUES (?,?,?,?,?,?,?,?,?)`,
			it.PhotoID, f.X, f.Y, f.W, f.H, f.Score, encodeEmbedding(f.Embedding), person, confirmed); err != nil {
			return err
		}
	}
	if err := settleMarkedFaces(tx, it.PhotoID); err != nil {
		return err
	}
	_, err := tx.Exec(`INSERT OR REPLACE INTO photo_faces_found (photo_id, version) VALUES (?, ?)`, it.PhotoID, faces.Version)
	return err
}

// personID returns the person called name, adding them when there is none.
// Names are compared without regard to case.
func personID(tx *sql.Tx, name string) (int64, error) {
	if _, err := tx.Exec(`INSERT OR IGNORE INTO people (name) VALUES (?)`, name); err != nil {
		return 0, err
	}
	var id int64
	err := tx.QueryRow(`SELECT id FROM people WHERE name = ?`, name).Scan(&id)
	return id, err
}

func encodeEmbedding(e []float32) []byte {
	b := make([]byte, 4*len(e))
	for i, v := range e {
		binary.LittleEndian.PutUint32(b[i*4:], math.Float32bits(v))
	}
	return b
}

func decodeEmbedding(b []byte) []float32 {
	e := make([]float32, len(b)/4)
	for i := range e {
		e[i] = math.Float32frombits(binary.LittleEndian.Uint32(b[i*4:]))
	}
	return e
}

// facesOf returns the faces found in a photo with their boxes.
func (s *Store) facesOf(photoID string) ([]int64, []faces.Face, error) {
	rows, err := s.db.Query(`SELECT id, x, y, w, h FROM faces WHERE photo_id = ?`, photoID)
	if err != nil {
		return nil, nil, err
	}
	defer rows.Close()
	var ids []int64
	var boxes []faces.Face
	for rows.Next() {
		var id int64
		var f faces.Face
		if err := rows.Scan(&id, &f.X, &f.Y, &f.W, &f.H); err != nil {
			return nil, nil, err
		}
		ids = append(ids, id)
		boxes = append(boxes, f)
	}
	return ids, boxes, rows.Err()
}
