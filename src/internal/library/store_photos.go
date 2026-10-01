package library

import (
	"database/sql"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// PhotoExists returns true if a photo with the given SHA-256 ID is in the DB.
func (s *Store) PhotoExists(id string) (bool, error) {
	var count int
	err := s.db.QueryRow(`SELECT COUNT(1) FROM photos WHERE id=?`, id).Scan(&count)
	return count > 0, err
}

// UpsertPhoto inserts or updates a photo record.
func (s *Store) UpsertPhoto(id, pathHint, filename string, fileSize int64, indexedAt time.Time, exifJSON, thumbPath, dateTaken, ext string) error {
	_, err := s.db.Exec(
		`INSERT INTO photos(id,path_hint,filename,file_size,indexed_at,exif_json,thumb_path,status,date_taken,ext)
		 VALUES(?,?,?,?,?,?,?,'ok',NULLIF(?,''),?)
		 ON CONFLICT(id) DO UPDATE SET
		   path_hint=excluded.path_hint,
		   filename=excluded.filename,
		   file_size=excluded.file_size,
		   indexed_at=excluded.indexed_at,
		   exif_json=excluded.exif_json,
		   thumb_path=excluded.thumb_path,
		   status='ok',
		   date_taken=excluded.date_taken,
		   ext=excluded.ext`,
		id, pathHint, filename, fileSize, indexedAt.UTC(), exifJSON, thumbPath, dateTaken, ext,
	)
	return err
}

// UpsertExifIndex replaces all EXIF index rows for a photo.
// numeric contains pre-parsed float64 values for numeric EXIF fields.
func (s *Store) UpsertExifIndex(photoID string, fields map[string]string, numeric map[string]float64) error {
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	if _, err := tx.Exec(`DELETE FROM exif_index WHERE photo_id=?`, photoID); err != nil {
		return err
	}
	stmt, err := tx.Prepare(`INSERT INTO exif_index(photo_id,field,value,numeric_value) VALUES(?,?,?,?)`)
	if err != nil {
		return err
	}
	defer stmt.Close()
	for k, v := range fields {
		var numVal any
		if nv, ok := numeric[k]; ok {
			numVal = nv
		}
		if _, err := stmt.Exec(photoID, k, v, numVal); err != nil {
			return err
		}
	}
	return tx.Commit()
}

// MarkAllMissing sets status='missing' for all photos.
// Called at the start of a re-index; found photos are set back to 'ok' via UpsertPhoto.
func (s *Store) MarkAllMissing() error {
	_, err := s.db.Exec(`UPDATE photos SET status='missing'`)
	return err
}

// PurgeMissingPhotos deletes all photos still at status='missing' after a re-index,
// along with their exif_index, photo_meta, and path_cache rows. Orphaned thumbnail
// DeletePhotoByID removes a single photo from the database and returns its
// pathHint and thumbPath so the caller can delete the files from disk.
func (s *Store) DeletePhotoByID(id string) (pathHint, thumbPath string, err error) {
	var tp *string
	if err = s.db.QueryRow(`SELECT path_hint, thumb_path FROM photos WHERE id = ?`, id).Scan(&pathHint, &tp); err != nil {
		return
	}
	if tp != nil {
		thumbPath = *tp
	}
	tx, txErr := s.db.Begin()
	if txErr != nil {
		err = txErr
		return
	}
	defer tx.Rollback() //nolint:errcheck
	for _, q := range []string{
		`DELETE FROM path_cache WHERE photo_id = ?`,
		`DELETE FROM exif_index WHERE photo_id = ?`,
		`DELETE FROM photo_meta WHERE photo_id = ?`,
		`DELETE FROM photo_palette    WHERE photo_id = ?`,
		`DELETE FROM photo_appearance WHERE photo_id = ?`,
		`DELETE FROM photos     WHERE id       = ?`,
	} {
		if _, err = tx.Exec(q, id); err != nil {
			return
		}
	}
	err = tx.Commit()
	return
}

// files are removed from disk. Returns the number of photos purged.
func (s *Store) PurgeMissingPhotos() (int, error) {
	rows, err := s.db.Query(`SELECT id, thumb_path FROM photos WHERE status='missing'`)
	if err != nil {
		return 0, err
	}
	type entry struct{ id, thumbPath string }
	var victims []entry
	for rows.Next() {
		var e entry
		var thumbPath *string
		if err := rows.Scan(&e.id, &thumbPath); err != nil {
			rows.Close()
			return 0, err
		}
		if thumbPath != nil {
			e.thumbPath = *thumbPath
		}
		victims = append(victims, e)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return 0, err
	}
	if len(victims) == 0 {
		return 0, nil
	}

	ids := make([]any, len(victims))
	placeholders := make([]string, len(victims))
	for i, v := range victims {
		ids[i] = v.id
		placeholders[i] = "?"
	}
	ph := strings.Join(placeholders, ",")

	tx, err := s.db.Begin()
	if err != nil {
		return 0, err
	}
	defer tx.Rollback() //nolint:errcheck
	for _, q := range []string{
		`DELETE FROM path_cache  WHERE photo_id IN (` + ph + `)`,
		`DELETE FROM exif_index  WHERE photo_id IN (` + ph + `)`,
		`DELETE FROM photo_meta  WHERE photo_id IN (` + ph + `)`,
		`DELETE FROM photo_palette    WHERE photo_id IN (` + ph + `)`,
		`DELETE FROM photo_appearance WHERE photo_id IN (` + ph + `)`,
		`DELETE FROM photos      WHERE id        IN (` + ph + `)`,
	} {
		if _, err := tx.Exec(q, ids...); err != nil {
			return 0, err
		}
	}
	if err := tx.Commit(); err != nil {
		return 0, err
	}

	for _, v := range victims {
		if v.thumbPath != "" {
			os.Remove(filepath.Join(s.dir, v.thumbPath)) //nolint:errcheck
		}
	}
	return len(victims), nil
}

// PhotoRef is a minimal photo record used for cleanup path checks.
type PhotoRef struct {
	ID       string
	PathHint string
}

// DeletePathCacheForFolder removes all path_cache entries whose abs_path is inside
// folderPath (i.e. starts with "<folderPath>/"). Forcing re-hashing on the next
// indexFile call so EXIF and thumbnails are re-evaluated even for unchanged files.
func (s *Store) DeletePathCacheForFolder(folderPath string) error {
	prefix := escapeLikePattern(folderPath) + string(filepath.Separator) + "%"
	_, err := s.db.Exec(`DELETE FROM path_cache WHERE abs_path LIKE ? ESCAPE '\'`, prefix)
	return err
}

// ListPhotoRefsInFolder returns the ID and path_hint for every ok photo whose
// path_hint lives inside folderPath (i.e. starts with "<folderPath>/").
func (s *Store) ListPhotoRefsInFolder(folderPath string) ([]PhotoRef, error) {
	prefix := escapeLikePattern(folderPath) + string(filepath.Separator) + "%"
	rows, err := s.db.Query(`SELECT id, path_hint FROM photos WHERE path_hint LIKE ? ESCAPE '\' AND status='ok'`, prefix)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var refs []PhotoRef
	for rows.Next() {
		var r PhotoRef
		if err := rows.Scan(&r.ID, &r.PathHint); err != nil {
			return nil, err
		}
		refs = append(refs, r)
	}
	return refs, rows.Err()
}

// ListAllPhotoRefs returns the ID and path_hint for every ok photo.
func (s *Store) ListAllPhotoRefs() ([]PhotoRef, error) {
	rows, err := s.db.Query(`SELECT id, path_hint FROM photos WHERE status='ok'`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var refs []PhotoRef
	for rows.Next() {
		var r PhotoRef
		if err := rows.Scan(&r.ID, &r.PathHint); err != nil {
			return nil, err
		}
		refs = append(refs, r)
	}
	return refs, rows.Err()
}

// MarkPhotoMissing sets status='missing' for a single photo by ID.
func (s *Store) MarkPhotoMissing(id string) error {
	_, err := s.db.Exec(`UPDATE photos SET status='missing' WHERE id=?`, id)
	return err
}

// CountPhotos returns the total number of indexed photos (status='ok').
func (s *Store) CountPhotos() (int, error) {
	var n int
	err := s.db.QueryRow(`SELECT COUNT(1) FROM photos WHERE status='ok'`).Scan(&n)
	return n, err
}

// GetPhoto returns a single photo with EXIF and meta populated.
func (s *Store) GetPhoto(id string) (*Photo, error) {
	var p Photo
	var indexedAt string
	err := s.db.QueryRow(
		`SELECT id, path_hint, filename, file_size, indexed_at, status FROM photos WHERE id=?`, id,
	).Scan(&p.ID, &p.PathHint, &p.Filename, &p.FileSize, &indexedAt, &p.Status)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	p.IndexedAt, _ = time.Parse(time.RFC3339, indexedAt)

	p.Exif, err = s.getExif(id)
	if err != nil {
		return nil, err
	}
	p.Meta, err = s.getMetaMap(id)
	return &p, err
}

func (s *Store) getExif(photoID string) (map[string]string, error) {
	rows, err := s.db.Query(`SELECT field, value FROM exif_index WHERE photo_id=?`, photoID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	m := make(map[string]string)
	for rows.Next() {
		var k, v string
		if err := rows.Scan(&k, &v); err != nil {
			return nil, err
		}
		m[k] = v
	}
	return m, rows.Err()
}

func (s *Store) getMetaMap(photoID string) (map[string]string, error) {
	rows, err := s.db.Query(`SELECT key, value FROM photo_meta WHERE photo_id=? ORDER BY key`, photoID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	m := make(map[string]string)
	for rows.Next() {
		var k, v string
		if err := rows.Scan(&k, &v); err != nil {
			return nil, err
		}
		m[k] = v
	}
	return m, rows.Err()
}

// GetPhotoThumbPath returns the stored thumb_path for a photo, or empty if none.
func (s *Store) GetPhotoThumbPath(id string) (string, error) {
	var thumbPath sql.NullString
	err := s.db.QueryRow(`SELECT thumb_path FROM photos WHERE id=?`, id).Scan(&thumbPath)
	if err == sql.ErrNoRows {
		return "", nil
	}
	if err != nil {
		return "", err
	}
	return thumbPath.String, nil
}

// SetPhotoThumbPath sets the thumb_path for a photo.
func (s *Store) SetPhotoThumbPath(id, thumbPath string) error {
	_, err := s.db.Exec(`UPDATE photos SET thumb_path=? WHERE id=?`, thumbPath, id)
	return err
}

// UpdatePhotoExif replaces the stored EXIF JSON and date_taken for a photo.
// Used by forced re-index to pick up EXIF changes made by external tools.
func (s *Store) UpdatePhotoExif(id, exifJSON, dateTaken string) error {
	_, err := s.db.Exec(`UPDATE photos SET exif_json=?, date_taken=NULLIF(?,'') WHERE id=?`, exifJSON, dateTaken, id)
	return err
}

// GetPhotoPathHint returns the last known absolute path for a photo.
func (s *Store) GetPhotoPathHint(id string) (string, error) {
	var path string
	err := s.db.QueryRow(`SELECT path_hint FROM photos WHERE id=?`, id).Scan(&path)
	if err == sql.ErrNoRows {
		return "", nil
	}
	return path, err
}

// PhotoInfo holds the fields needed to render the info panel for a library photo.
type PhotoInfo struct {
	Filename  string `json:"filename"`
	PathHint  string `json:"pathHint"`
	FileSize  int64  `json:"fileSize"`
	IndexedAt string `json:"indexedAt"`
	ExifJSON  string `json:"exifJSON"`
}

// GetPhotoInfo returns filename, path, size, indexed_at, and raw exif_json for a single photo.
func (s *Store) GetPhotoInfo(photoID string) (*PhotoInfo, error) {
	var p PhotoInfo
	var exifJSON *string
	err := s.db.QueryRow(
		`SELECT filename, path_hint, file_size, indexed_at, exif_json FROM photos WHERE id=?`, photoID,
	).Scan(&p.Filename, &p.PathHint, &p.FileSize, &p.IndexedAt, &exifJSON)
	if err != nil {
		return nil, err
	}
	if exifJSON != nil {
		p.ExifJSON = *exifJSON
	}
	return &p, nil
}

// GetMeta returns all user-defined metadata for a photo.
func (s *Store) GetMeta(photoID string) ([]MetaEntry, error) {
	rows, err := s.db.Query(
		`SELECT key, value, updated_at FROM photo_meta WHERE photo_id=? ORDER BY key`, photoID,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var entries []MetaEntry
	for rows.Next() {
		var e MetaEntry
		var updatedAt string
		if err := rows.Scan(&e.Key, &e.Value, &updatedAt); err != nil {
			return nil, err
		}
		e.UpdatedAt, _ = time.Parse(time.RFC3339, updatedAt)
		entries = append(entries, e)
	}
	if entries == nil {
		entries = []MetaEntry{}
	}
	return entries, rows.Err()
}

// UpsertMeta stores or updates a user-defined metadata entry.
func (s *Store) UpsertMeta(photoID, key, value string) error {
	_, err := s.db.Exec(
		`INSERT INTO photo_meta(photo_id,key,value,updated_at) VALUES(?,?,?,?)
		 ON CONFLICT(photo_id,key) DO UPDATE SET value=excluded.value, updated_at=excluded.updated_at`,
		photoID, key, value, time.Now().UTC().Format(time.RFC3339),
	)
	return err
}

// DeleteMeta removes a user-defined metadata entry.
func (s *Store) DeleteMeta(photoID, key string) error {
	_, err := s.db.Exec(`DELETE FROM photo_meta WHERE photo_id=? AND key=?`, photoID, key)
	return err
}

// MarkPhotoPresent resets status to 'ok' and updates path/filename for a photo
// without touching exif_json or thumb_path. Used by the fast-path and rename cases.
func (s *Store) MarkPhotoPresent(id, pathHint, filename string) error {
	_, err := s.db.Exec(
		`UPDATE photos SET status='ok', path_hint=?, filename=? WHERE id=?`,
		pathHint, filename, id,
	)
	return err
}

// GetPhotoIDByAbsPath returns the photo_id for a given absolute path via the path_cache.
// Returns empty string (no error) when the path is not cached.
func (s *Store) GetPhotoIDByAbsPath(absPath string) (string, error) {
	var id string
	err := s.db.QueryRow(`SELECT photo_id FROM path_cache WHERE abs_path=?`, absPath).Scan(&id)
	if err == sql.ErrNoRows {
		return "", nil
	}
	return id, err
}

// GetPhotoIDByPathHint returns the photo id for a given absolute path by querying
// the photos table directly. Returns empty string (no error) when not found.
func (s *Store) GetPhotoIDByPathHint(pathHint string) (string, error) {
	var id string
	err := s.db.QueryRow(`SELECT id FROM photos WHERE path_hint=? AND status='ok'`, pathHint).Scan(&id)
	if err == sql.ErrNoRows {
		return "", nil
	}
	return id, err
}
