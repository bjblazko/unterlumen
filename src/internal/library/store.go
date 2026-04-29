package library

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	_ "modernc.org/sqlite"
)

const dbSchema = `
CREATE TABLE IF NOT EXISTS photos (
	id          TEXT PRIMARY KEY,
	path_hint   TEXT NOT NULL,
	filename    TEXT NOT NULL,
	file_size   INTEGER NOT NULL,
	indexed_at  DATETIME NOT NULL,
	exif_json   TEXT,
	thumb_path  TEXT,
	status      TEXT NOT NULL DEFAULT 'ok'
);

CREATE TABLE IF NOT EXISTS path_cache (
	abs_path    TEXT PRIMARY KEY,
	photo_id    TEXT NOT NULL REFERENCES photos(id),
	mtime_ns    INTEGER NOT NULL,
	file_size   INTEGER NOT NULL
);

CREATE TABLE IF NOT EXISTS exif_index (
	photo_id      TEXT NOT NULL REFERENCES photos(id),
	field         TEXT NOT NULL,
	value         TEXT NOT NULL,
	numeric_value REAL,
	PRIMARY KEY (photo_id, field)
);
CREATE INDEX IF NOT EXISTS exif_index_field_value ON exif_index(field, value);
CREATE INDEX IF NOT EXISTS exif_index_field_numeric ON exif_index(field, numeric_value);

CREATE TABLE IF NOT EXISTS photo_meta (
	photo_id    TEXT NOT NULL REFERENCES photos(id),
	key         TEXT NOT NULL,
	value       TEXT NOT NULL,
	updated_at  DATETIME NOT NULL,
	PRIMARY KEY (photo_id, key)
);

CREATE TABLE IF NOT EXISTS library_props (
	key         TEXT PRIMARY KEY,
	value       TEXT NOT NULL
);
`

// Store wraps the per-library SQLite database.
type Store struct {
	db  *sql.DB
	dir string
}

func openStore(dbPath, dir string) (*Store, error) {
	dsn := fmt.Sprintf("file:%s?_journal_mode=WAL&_foreign_keys=on", dbPath)
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, err
	}
	db.SetMaxOpenConns(1)
	if _, err := db.Exec(dbSchema); err != nil {
		db.Close()
		return nil, fmt.Errorf("init schema: %w", err)
	}
	// Migration: add numeric_value column to existing databases (ignored for new ones).
	db.Exec(`ALTER TABLE exif_index ADD COLUMN numeric_value REAL`)
	db.Exec(`CREATE INDEX IF NOT EXISTS exif_index_field_numeric ON exif_index(field, numeric_value)`)
	// Migration: backfill FocalLengthIn35mmFilm numeric_value for photos indexed before
	// this field was added to numericExifFields. The value is always a plain integer string.
	db.Exec(`UPDATE exif_index
		SET numeric_value = CAST(TRIM(value, '"') AS REAL)
		WHERE field = 'FocalLengthIn35mmFilm'
		  AND numeric_value IS NULL
		  AND CAST(TRIM(value, '"') AS REAL) > 0`)
	return &Store{db: db, dir: dir}, nil
}

func (s *Store) Close() error {
	return s.db.Close()
}

// SetProp stores a library-level property.
func (s *Store) SetProp(key, value string) error {
	_, err := s.db.Exec(
		`INSERT INTO library_props(key,value) VALUES(?,?) ON CONFLICT(key) DO UPDATE SET value=excluded.value`,
		key, value,
	)
	return err
}

// GetProp retrieves a library-level property.
func (s *Store) GetProp(key string) (string, bool, error) {
	var v string
	err := s.db.QueryRow(`SELECT value FROM library_props WHERE key=?`, key).Scan(&v)
	if err == sql.ErrNoRows {
		return "", false, nil
	}
	return v, err == nil, err
}

// GetPathCache looks up a fast-path cache entry by absolute file path.
func (s *Store) GetPathCache(absPath string) (photoID string, mtimeNs, fileSize int64, found bool, err error) {
	err = s.db.QueryRow(
		`SELECT photo_id, mtime_ns, file_size FROM path_cache WHERE abs_path=?`, absPath,
	).Scan(&photoID, &mtimeNs, &fileSize)
	if err == sql.ErrNoRows {
		return "", 0, 0, false, nil
	}
	if err != nil {
		return "", 0, 0, false, err
	}
	return photoID, mtimeNs, fileSize, true, nil
}

// UpsertPathCache stores or updates a fast-path cache entry.
func (s *Store) UpsertPathCache(absPath, photoID string, mtimeNs, fileSize int64) error {
	_, err := s.db.Exec(
		`INSERT INTO path_cache(abs_path,photo_id,mtime_ns,file_size) VALUES(?,?,?,?)
		 ON CONFLICT(abs_path) DO UPDATE SET photo_id=excluded.photo_id, mtime_ns=excluded.mtime_ns, file_size=excluded.file_size`,
		absPath, photoID, mtimeNs, fileSize,
	)
	return err
}

// PhotoExists returns true if a photo with the given SHA-256 ID is in the DB.
func (s *Store) PhotoExists(id string) (bool, error) {
	var count int
	err := s.db.QueryRow(`SELECT COUNT(1) FROM photos WHERE id=?`, id).Scan(&count)
	return count > 0, err
}

// UpsertPhoto inserts or updates a photo record.
func (s *Store) UpsertPhoto(id, pathHint, filename string, fileSize int64, indexedAt time.Time, exifJSON, thumbPath string) error {
	_, err := s.db.Exec(
		`INSERT INTO photos(id,path_hint,filename,file_size,indexed_at,exif_json,thumb_path,status)
		 VALUES(?,?,?,?,?,?,?,'ok')
		 ON CONFLICT(id) DO UPDATE SET
		   path_hint=excluded.path_hint,
		   filename=excluded.filename,
		   file_size=excluded.file_size,
		   indexed_at=excluded.indexed_at,
		   exif_json=excluded.exif_json,
		   thumb_path=excluded.thumb_path,
		   status='ok'`,
		id, pathHint, filename, fileSize, indexedAt.UTC(), exifJSON, thumbPath,
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

// GetPhotoPathHint returns the last known absolute path for a photo.
func (s *Store) GetPhotoPathHint(id string) (string, error) {
	var path string
	err := s.db.QueryRow(`SELECT path_hint FROM photos WHERE id=?`, id).Scan(&path)
	if err == sql.ErrNoRows {
		return "", nil
	}
	return path, err
}

// ListPhotosResult holds a page of photos plus the total count.
type ListPhotosResult struct {
	Photos []Photo `json:"photos"`
	Total  int     `json:"total"`
}

// NumericFilter restricts results to photos whose numeric EXIF value for a field
// falls within [Min, Max] (inclusive).
type NumericFilter struct {
	Min float64
	Max float64
}

// ListPhotos returns a filtered, paginated list of photos.
// q is a full-text search term; filters is field→value for EXIF text filtering;
// numericFilters is field→range for numeric EXIF filtering (e.g. ExposureTime).
func (s *Store) ListPhotos(q string, filters map[string]string, numericFilters map[string]NumericFilter, offset, limit int) (ListPhotosResult, error) {
	args := []any{}
	where := []string{"p.status='ok'"}

	if q != "" {
		like := "%" + q + "%"
		where = append(where,
			`(p.filename LIKE ?
			  OR EXISTS (SELECT 1 FROM exif_index e WHERE e.photo_id=p.id AND e.value LIKE ?)
			  OR EXISTS (SELECT 1 FROM photo_meta m WHERE m.photo_id=p.id AND m.value LIKE ?))`)
		args = append(args, like, like, like)
	}

	for field, val := range filters {
		where = append(where,
			`EXISTS (SELECT 1 FROM exif_index e WHERE e.photo_id=p.id AND e.field=? AND TRIM(TRIM(e.value, '"')) = ?)`)
		args = append(args, field, val)
	}

	for field, r := range numericFilters {
		if field == "FocalLength35" {
			where = append(where, `(
				EXISTS (
					SELECT 1 FROM exif_index e35
					WHERE e35.photo_id = p.id AND e35.field = 'FocalLengthIn35mmFilm'
					  AND e35.numeric_value BETWEEN ? AND ?
				)
				OR (
					NOT EXISTS (
						SELECT 1 FROM exif_index e35
						WHERE e35.photo_id = p.id AND e35.field = 'FocalLengthIn35mmFilm'
						  AND e35.numeric_value IS NOT NULL
					)
					AND EXISTS (
						SELECT 1 FROM exif_index efl
						WHERE efl.photo_id = p.id AND efl.field = 'FocalLength'
						  AND efl.numeric_value BETWEEN ? AND ?
					)
				)
			)`)
			args = append(args, r.Min, r.Max, r.Min, r.Max)
		} else {
			where = append(where,
				`EXISTS (SELECT 1 FROM exif_index e WHERE e.photo_id=p.id AND e.field=? AND e.numeric_value BETWEEN ? AND ?)`)
			args = append(args, field, r.Min, r.Max)
		}
	}

	whereClause := strings.Join(where, " AND ")

	var total int
	countArgs := append([]any{}, args...)
	if err := s.db.QueryRow(
		`SELECT COUNT(DISTINCT p.id) FROM photos p WHERE `+whereClause, countArgs...,
	).Scan(&total); err != nil {
		return ListPhotosResult{}, err
	}

	pageArgs := append(args, limit, offset)
	rows, err := s.db.Query(
		`SELECT p.id, p.path_hint, p.filename, p.file_size, p.indexed_at, p.status,
		        (SELECT value FROM exif_index WHERE photo_id=p.id AND field='GPSLatitude' LIMIT 1),
		        (SELECT value FROM exif_index WHERE photo_id=p.id AND field='FilmSimulation' LIMIT 1)
		 FROM photos p WHERE `+whereClause+
			` ORDER BY p.indexed_at DESC LIMIT ? OFFSET ?`,
		pageArgs...,
	)
	if err != nil {
		return ListPhotosResult{}, err
	}
	defer rows.Close()

	var photos []Photo
	for rows.Next() {
		var p Photo
		var indexedAt string
		var gpsLat, filmSim *string
		if err := rows.Scan(&p.ID, &p.PathHint, &p.Filename, &p.FileSize, &indexedAt, &p.Status, &gpsLat, &filmSim); err != nil {
			return ListPhotosResult{}, err
		}
		p.IndexedAt, _ = time.Parse(time.RFC3339, indexedAt)
		if gpsLat != nil || filmSim != nil {
			p.Exif = make(map[string]string)
			if gpsLat != nil {
				p.Exif["GPSLatitude"] = *gpsLat
			}
			if filmSim != nil {
				p.Exif["FilmSimulation"] = *filmSim
			}
		}
		photos = append(photos, p)
	}
	if err := rows.Err(); err != nil {
		return ListPhotosResult{}, err
	}
	if photos == nil {
		photos = []Photo{}
	}
	return ListPhotosResult{Photos: photos, Total: total}, nil
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

// ExifFields returns the sorted distinct field names present in the exif_index.
func (s *Store) ExifFields() ([]string, error) {
	rows, err := s.db.Query(`SELECT DISTINCT field FROM exif_index ORDER BY field`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var fields []string
	for rows.Next() {
		var f string
		if err := rows.Scan(&f); err != nil {
			return nil, err
		}
		fields = append(fields, f)
	}
	return fields, rows.Err()
}

// GetExifFieldValues returns the sorted distinct string values for the given EXIF field,
// with surrounding quotes stripped. Empty or missing values are excluded.
func (s *Store) GetExifFieldValues(field string) ([]string, error) {
	rows, err := s.db.Query(
		`SELECT DISTINCT TRIM(TRIM(value, '"')) FROM exif_index
		 WHERE field=? AND TRIM(TRIM(value, '"')) != ''
		 ORDER BY TRIM(TRIM(value, '"'))`,
		field,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var vals []string
	for rows.Next() {
		var v string
		if err := rows.Scan(&v); err != nil {
			return nil, err
		}
		if v != "" {
			vals = append(vals, v)
		}
	}
	return vals, rows.Err()
}

// ExifRange holds the minimum and maximum numeric_value for a single EXIF field.
type ExifRange struct {
	Min float64 `json:"min"`
	Max float64 `json:"max"`
}

// GetExifRanges returns the min/max numeric_value for each of the requested EXIF fields.
// Fields with no numeric data are omitted from the result.
// The virtual key "FocalLength35" returns the combined range of FocalLengthIn35mmFilm
// (where present) falling back to FocalLength — matching the filter semantics.
func (s *Store) GetExifRanges(fields []string) (map[string]ExifRange, error) {
	out := make(map[string]ExifRange)
	for _, field := range fields {
		var minVal, maxVal sql.NullFloat64
		if field == "FocalLength35" {
			err := s.db.QueryRow(`
				SELECT MIN(COALESCE(fl35.numeric_value, fl.numeric_value)),
				       MAX(COALESCE(fl35.numeric_value, fl.numeric_value))
				FROM   photos p
				LEFT JOIN exif_index fl35
				       ON fl35.photo_id = p.id AND fl35.field = 'FocalLengthIn35mmFilm'
				      AND fl35.numeric_value IS NOT NULL
				LEFT JOIN exif_index fl
				       ON fl.photo_id = p.id AND fl.field = 'FocalLength'
				      AND fl.numeric_value IS NOT NULL
				WHERE  p.status = 'ok'
				  AND  COALESCE(fl35.numeric_value, fl.numeric_value) IS NOT NULL`,
			).Scan(&minVal, &maxVal)
			if err != nil {
				return nil, err
			}
		} else {
			err := s.db.QueryRow(
				`SELECT MIN(numeric_value), MAX(numeric_value) FROM exif_index WHERE field=? AND numeric_value IS NOT NULL`,
				field,
			).Scan(&minVal, &maxVal)
			if err != nil {
				return nil, err
			}
		}
		if minVal.Valid && maxVal.Valid {
			out[field] = ExifRange{Min: minVal.Float64, Max: maxVal.Float64}
		}
	}
	return out, nil
}
