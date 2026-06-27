package library

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"huepattl.de/unterlumen/internal/media"
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
	status      TEXT NOT NULL DEFAULT 'ok',
	date_taken  TEXT,
	ext         TEXT NOT NULL DEFAULT ''
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
CREATE INDEX IF NOT EXISTS photos_status_idx ON photos(status);
CREATE INDEX IF NOT EXISTS photos_status_path_idx ON photos(status, path_hint);
CREATE INDEX IF NOT EXISTS photos_indexed_at_idx ON photos(indexed_at);
CREATE INDEX IF NOT EXISTS photos_status_indexed_at_idx ON photos(status, indexed_at);
`

// Store wraps the per-library SQLite database.
type Store struct {
	db  *sql.DB
	dir string
}

// openDB opens and migrates a SQLite database, returning the underlying *sql.DB.
// The connection is long-lived; callers must not close it — use Store.Close() which is a no-op.
func openDB(dbPath string) (*sql.DB, error) {
	dsn := fmt.Sprintf("file:%s?_journal_mode=WAL&_foreign_keys=on", dbPath)
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, err
	}
	db.SetMaxOpenConns(1)
	// Increase page cache to 64 MB and keep temp tables in RAM.
	db.Exec(`PRAGMA cache_size = -65536`)
	db.Exec(`PRAGMA temp_store = MEMORY`)
	if _, err := db.Exec(dbSchema); err != nil {
		db.Close()
		return nil, fmt.Errorf("init schema: %w", err)
	}
	// Migration: add status index to existing databases (ignored for new ones).
	db.Exec(`CREATE INDEX IF NOT EXISTS photos_status_idx ON photos(status)`)
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
	// Migration: index path_hint for fast folder-scoped stats queries.
	db.Exec(`CREATE INDEX IF NOT EXISTS photos_path_hint_idx ON photos(path_hint)`)
	// Migration: composite (status, path_hint) index and indexed_at index for browse/sort.
	db.Exec(`CREATE INDEX IF NOT EXISTS photos_status_path_idx ON photos(status, path_hint)`)
	db.Exec(`CREATE INDEX IF NOT EXISTS photos_indexed_at_idx ON photos(indexed_at)`)
	// Migration: date_taken column for fast date-based stats and timeline queries.
	db.Exec(`ALTER TABLE photos ADD COLUMN date_taken TEXT`)
	db.Exec(`UPDATE photos SET date_taken = json_extract(exif_json,'$.dateTaken') WHERE date_taken IS NULL`)
	db.Exec(`CREATE INDEX IF NOT EXISTS photos_date_taken_idx ON photos(date_taken)`)
	// Migration: ext column for fast format distribution queries.
	db.Exec(`ALTER TABLE photos ADD COLUMN ext TEXT NOT NULL DEFAULT ''`)
	db.Exec(`UPDATE photos SET ext =
		CASE
		  WHEN LOWER(SUBSTR(filename,-5)) = '.jpeg' THEN 'jpeg'
		  WHEN LOWER(SUBSTR(filename,-5)) = '.heic' THEN 'heif'
		  WHEN LOWER(SUBSTR(filename,-5)) = '.heif' THEN 'heif'
		  WHEN LOWER(SUBSTR(filename,-5)) = '.tiff' THEN 'tiff'
		  WHEN LOWER(SUBSTR(filename,-4)) = '.jpg'  THEN 'jpeg'
		  WHEN LOWER(SUBSTR(filename,-4)) = '.hif'  THEN 'heif'
		  WHEN LOWER(SUBSTR(filename,-4)) = '.raf'  THEN 'raf'
		  WHEN LOWER(SUBSTR(filename,-4)) = '.dng'  THEN 'dng'
		  WHEN LOWER(SUBSTR(filename,-4)) = '.arw'  THEN 'arw'
		  WHEN LOWER(SUBSTR(filename,-4)) = '.nef'  THEN 'nef'
		  WHEN LOWER(SUBSTR(filename,-4)) = '.cr2'  THEN 'cr2'
		  WHEN LOWER(SUBSTR(filename,-4)) = '.cr3'  THEN 'cr3'
		  WHEN LOWER(SUBSTR(filename,-4)) = '.tif'  THEN 'tif'
		  WHEN LOWER(SUBSTR(filename,-4)) = '.mov'  THEN 'mov'
		  WHEN LOWER(SUBSTR(filename,-4)) = '.mp4'  THEN 'mp4'
		  WHEN LOWER(SUBSTR(filename,-4)) = '.png'  THEN 'png'
		  WHEN LOWER(SUBSTR(filename,-4)) = '.gif'  THEN 'gif'
		  ELSE LOWER(LTRIM(SUBSTR(filename, INSTR(filename,'.')+1), '.'))
		END
		WHERE ext = ''`)
	db.Exec(`CREATE INDEX IF NOT EXISTS photos_ext_idx ON photos(status, ext)`)
	// Migration: compound (status, indexed_at) index for sorted pagination in ListPhotos.
	db.Exec(`CREATE INDEX IF NOT EXISTS photos_status_indexed_at_idx ON photos(status, indexed_at)`)
	return db, nil
}

func newStore(db *sql.DB, dir string) *Store {
	return &Store{db: db, dir: dir}
}

// Close is a no-op. The underlying *sql.DB lifetime is managed by Manager.
func (s *Store) Close() error {
	return nil
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
func (s *Store) UpsertPhoto(id, pathHint, filename string, fileSize int64, indexedAt time.Time, exifJSON, thumbPath, dateTaken, ext string) error {
	_, err := s.db.Exec(
		`INSERT INTO photos(id,path_hint,filename,file_size,indexed_at,exif_json,thumb_path,status,date_taken,ext)
		 VALUES(?,?,?,?,?,?,?,'ok',?,?)
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
	prefix := folderPath + string(filepath.Separator) + "%"
	_, err := s.db.Exec(`DELETE FROM path_cache WHERE abs_path LIKE ?`, prefix)
	return err
}

// ListPhotoRefsInFolder returns the ID and path_hint for every ok photo whose
// path_hint lives inside folderPath (i.e. starts with "<folderPath>/").
func (s *Store) ListPhotoRefsInFolder(folderPath string) ([]PhotoRef, error) {
	prefix := folderPath + string(filepath.Separator) + "%"
	rows, err := s.db.Query(`SELECT id, path_hint FROM photos WHERE path_hint LIKE ? AND status='ok'`, prefix)
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
	_, err := s.db.Exec(`UPDATE photos SET exif_json=?, date_taken=? WHERE id=?`, exifJSON, dateTaken, id)
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

// ListPhotosOpts holds all filter and pagination options for ListPhotos.
type ListPhotosOpts struct {
	Filters        map[string]string       // EXIF text exact-match filters (field → value)
	NumericFilters map[string]NumericFilter // EXIF numeric range filters
	DateMin        string                  // YYYY-MM-DD lower bound on date_taken
	DateMax        string                  // YYYY-MM-DD upper bound on date_taken
	MetaFilters    map[string]string       // photo_meta key=value exact matches
	MetaExists     []string                // photo_meta keys that must exist (any value)
	AlbumTitle     string                  // match photos with any published:*:title = value
	ExtFilter      string                  // file extension (photos.ext)
	Offset         int
	Limit          int
}

// ListPhotos returns a filtered, paginated list of photos.
func (s *Store) ListPhotos(opts ListPhotosOpts) (ListPhotosResult, error) {
	var joinClauses []string
	var joinArgs []any
	var whereArgs []any
	where := []string{"p.status='ok'"}
	i := 0

	for field, val := range opts.Filters {
		a := fmt.Sprintf("ef%d", i)
		i++
		joinClauses = append(joinClauses,
			fmt.Sprintf(`JOIN exif_index %s ON %s.photo_id=p.id AND %s.field=? AND TRIM(TRIM(%s.value,'"'))=?`, a, a, a, a))
		joinArgs = append(joinArgs, field, val)
	}

	for field, r := range opts.NumericFilters {
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
			whereArgs = append(whereArgs, r.Min, r.Max, r.Min, r.Max)
		} else {
			a := fmt.Sprintf("en%d", i)
			i++
			joinClauses = append(joinClauses,
				fmt.Sprintf(`JOIN exif_index %s ON %s.photo_id=p.id AND %s.field=? AND %s.numeric_value BETWEEN ? AND ?`, a, a, a, a))
			joinArgs = append(joinArgs, field, r.Min, r.Max)
		}
	}

	if opts.DateMin != "" {
		where = append(where, `(p.date_taken IS NOT NULL AND SUBSTR(p.date_taken, 1, 10) >= ?)`)
		whereArgs = append(whereArgs, opts.DateMin)
	}
	if opts.DateMax != "" {
		where = append(where, `(p.date_taken IS NOT NULL AND SUBSTR(p.date_taken, 1, 10) <= ?)`)
		whereArgs = append(whereArgs, opts.DateMax)
	}
	if opts.ExtFilter != "" {
		where = append(where, `p.ext = ?`)
		whereArgs = append(whereArgs, opts.ExtFilter)
	}
	for key, val := range opts.MetaFilters {
		where = append(where, `EXISTS (SELECT 1 FROM photo_meta pm WHERE pm.photo_id=p.id AND pm.key=? AND pm.value=?)`)
		whereArgs = append(whereArgs, key, val)
	}
	for _, key := range opts.MetaExists {
		where = append(where, `EXISTS (SELECT 1 FROM photo_meta pm WHERE pm.photo_id=p.id AND pm.key=?)`)
		whereArgs = append(whereArgs, key)
	}
	if opts.AlbumTitle != "" {
		where = append(where, `EXISTS (SELECT 1 FROM photo_meta pm WHERE pm.photo_id=p.id AND pm.key LIKE 'published:%:title' AND pm.value=?)`)
		whereArgs = append(whereArgs, opts.AlbumTitle)
	}

	joinSQL := strings.Join(joinClauses, " ")
	whereSQL := strings.Join(where, " AND ")
	allArgs := append(joinArgs, whereArgs...)

	fromSQL := "FROM photos p"
	if joinSQL != "" {
		fromSQL += " " + joinSQL
	}

	var total int
	countArgs := append([]any{}, allArgs...)
	if err := s.db.QueryRow(
		`SELECT COUNT(p.id) `+fromSQL+` WHERE `+whereSQL, countArgs...,
	).Scan(&total); err != nil {
		return ListPhotosResult{}, err
	}

	pageArgs := append(allArgs, opts.Limit, opts.Offset)
	rows, err := s.db.Query(
		`SELECT p.id, p.path_hint, p.filename, p.file_size, p.indexed_at, p.status, p.date_taken,
		        (SELECT value FROM exif_index WHERE photo_id=p.id AND field='GPSLatitude' LIMIT 1),
		        (SELECT value FROM exif_index WHERE photo_id=p.id AND field='FilmSimulation' LIMIT 1)
		 `+fromSQL+` WHERE `+whereSQL+
			` ORDER BY CASE WHEN p.date_taken IS NULL OR p.date_taken = '' THEN 1 ELSE 0 END, p.date_taken DESC LIMIT ? OFFSET ?`,
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
		var dateTaken sql.NullString
		var gpsLat, filmSim *string
		if err := rows.Scan(&p.ID, &p.PathHint, &p.Filename, &p.FileSize, &indexedAt, &p.Status, &dateTaken, &gpsLat, &filmSim); err != nil {
			return ListPhotosResult{}, err
		}
		p.IndexedAt, _ = time.Parse(time.RFC3339, indexedAt)
		if dateTaken.Valid {
			p.DateTaken = dateTaken.String
		}
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
// The special field "ext" queries the photos.ext column instead of exif_index.
func (s *Store) GetExifFieldValues(field string) ([]string, error) {
	if field == "ext" {
		rows, err := s.db.Query(`SELECT DISTINCT ext FROM photos WHERE status='ok' AND ext != '' ORDER BY ext`)
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
			vals = append(vals, v)
		}
		return vals, rows.Err()
	}
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

// GetMetaKeys returns all distinct photo_meta keys that are visible for filtering.
// Internal bookkeeping keys (published:*:account, published:*:postid) are excluded.
func (s *Store) GetMetaKeys() ([]string, error) {
	rows, err := s.db.Query(`
		SELECT DISTINCT key FROM photo_meta
		WHERE key NOT LIKE 'published:%:account'
		  AND key NOT LIKE 'published:%:postid'
		ORDER BY key`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var keys []string
	for rows.Next() {
		var k string
		if err := rows.Scan(&k); err != nil {
			return nil, err
		}
		keys = append(keys, k)
	}
	return keys, rows.Err()
}

// GetMetaValues returns distinct values for the given photo_meta key.
func (s *Store) GetMetaValues(key string) ([]string, error) {
	rows, err := s.db.Query(
		`SELECT DISTINCT value FROM photo_meta WHERE key=? AND value != '' ORDER BY value`, key)
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
		vals = append(vals, v)
	}
	return vals, rows.Err()
}

// GetAlbumTitles returns distinct gallery/album titles from all published:*:title meta entries.
func (s *Store) GetAlbumTitles() ([]string, error) {
	rows, err := s.db.Query(`
		SELECT DISTINCT value FROM photo_meta
		WHERE key LIKE 'published:%:title' AND value != ''
		ORDER BY value`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var titles []string
	for rows.Next() {
		var v string
		if err := rows.Scan(&v); err != nil {
			return nil, err
		}
		titles = append(titles, v)
	}
	return titles, rows.Err()
}

// FolderBrowseResult holds the immediate subfolders and direct photos at a given folder level.
type FolderBrowseResult struct {
	Subfolders []string `json:"subfolders"`
	Photos     []Photo  `json:"photos"`
	Total      int      `json:"total"`
}

// BrowseFolder returns the immediate subdirectory names and photos directly inside folderAbs.
// Photos nested in subdirectories are excluded from Photos but their parent directory appears
// in Subfolders. No filesystem reads are performed; all data comes from the DB.
func (s *Store) BrowseFolder(folderAbs string) (FolderBrowseResult, error) {
	prefix := folderAbs + "/"

	// Direct photos only — GLOB rules out any nested path (extra slash).
	// DateTaken is joined from exif_index for client-side sorting support.
	// GPS, film simulation, and image dimensions are fetched for overlay badges.
	photoRows, err := s.db.Query(
		`SELECT p.id, p.path_hint, p.filename, p.file_size, p.indexed_at,
		        COALESCE(e.value, '') AS date_taken,
		        (SELECT value FROM exif_index WHERE photo_id=p.id AND field='GPSLatitude' LIMIT 1),
		        (SELECT value FROM exif_index WHERE photo_id=p.id AND field='FilmSimulation' LIMIT 1),
		        CAST(json_extract(p.exif_json,'$.width')  AS INTEGER),
		        CAST(json_extract(p.exif_json,'$.height') AS INTEGER)
		 FROM photos p
		 LEFT JOIN exif_index e ON e.photo_id = p.id AND e.field = 'DateTaken'
		 WHERE p.status='ok' AND p.path_hint GLOB ? AND p.path_hint NOT GLOB ?`,
		prefix+"*", prefix+"*/*",
	)
	if err != nil {
		return FolderBrowseResult{}, err
	}
	defer photoRows.Close()

	var directPhotos []Photo
	for photoRows.Next() {
		var p Photo
		var indexedAt string
		var gpsLat, filmSim *string
		var imgWidth, imgHeight *int
		if err := photoRows.Scan(&p.ID, &p.PathHint, &p.Filename, &p.FileSize, &indexedAt, &p.DateTaken, &gpsLat, &filmSim, &imgWidth, &imgHeight); err != nil {
			return FolderBrowseResult{}, err
		}
		p.IndexedAt, _ = time.Parse(time.RFC3339, indexedAt)
		if gpsLat != nil || filmSim != nil || (imgWidth != nil && imgHeight != nil) {
			p.Exif = make(map[string]string)
			if gpsLat != nil {
				p.Exif["GPSLatitude"] = *gpsLat
			}
			if filmSim != nil {
				p.Exif["FilmSimulation"] = *filmSim
			}
			if imgWidth != nil && imgHeight != nil && *imgWidth > 0 && *imgHeight > 0 {
				if ar := media.AspectRatioLabel(*imgWidth, *imgHeight); ar != "" {
					p.Exif["AspectRatio"] = ar
				}
			}
		}
		directPhotos = append(directPhotos, p)
	}
	if err := photoRows.Err(); err != nil {
		return FolderBrowseResult{}, err
	}

	// Subfolders: extract the first path segment below prefix for all nested photos.
	// SUBSTR/INSTR in SQL avoids returning full rows; DISTINCT collapses duplicates.
	sfRows, err := s.db.Query(
		`SELECT DISTINCT SUBSTR(path_hint, length(?)+1, INSTR(SUBSTR(path_hint, length(?)+1), '/')-1)
		 FROM photos
		 WHERE status='ok' AND path_hint GLOB ?`,
		prefix, prefix, prefix+"*/*",
	)
	if err != nil {
		return FolderBrowseResult{}, err
	}
	defer sfRows.Close()

	var subfolders []string
	for sfRows.Next() {
		var name string
		if err := sfRows.Scan(&name); err != nil {
			return FolderBrowseResult{}, err
		}
		if name != "" {
			subfolders = append(subfolders, name)
		}
	}
	if err := sfRows.Err(); err != nil {
		return FolderBrowseResult{}, err
	}
	sortStrings(subfolders)

	if directPhotos == nil {
		directPhotos = []Photo{}
	}
	if subfolders == nil {
		subfolders = []string{}
	}
	return FolderBrowseResult{
		Subfolders: subfolders,
		Photos:     directPhotos,
		Total:      len(directPhotos),
	}, nil
}

// BrowseFolderRecursive returns all photos nested anywhere under folderAbs (including subdirectories).
// No filesystem reads are performed; all data comes from the DB.
func (s *Store) BrowseFolderRecursive(folderAbs string) ([]Photo, error) {
	prefix := folderAbs + "/"
	rows, err := s.db.Query(
		`SELECT p.id, p.path_hint, p.filename, p.file_size, p.indexed_at,
		        COALESCE(e.value, '') AS date_taken
		 FROM photos p
		 LEFT JOIN exif_index e ON e.photo_id = p.id AND e.field = 'DateTaken'
		 WHERE p.status='ok' AND p.path_hint GLOB ?`,
		prefix+"*",
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var photos []Photo
	for rows.Next() {
		var p Photo
		var indexedAt string
		if err := rows.Scan(&p.ID, &p.PathHint, &p.Filename, &p.FileSize, &indexedAt, &p.DateTaken); err != nil {
			return nil, err
		}
		p.IndexedAt, _ = time.Parse(time.RFC3339, indexedAt)
		photos = append(photos, p)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	if photos == nil {
		photos = []Photo{}
	}
	return photos, nil
}

// Statistics returns aggregated statistics for photos with status='ok' in this library.
// pathPrefix, when non-empty, restricts results to photos whose path_hint starts with that prefix.
func (s *Store) Statistics(pathPrefix string) (*LibraryStatistics, error) {
	st := &LibraryStatistics{
		Formats:        []NameCount{},
		FilmSims:       []NameCount{},
		FocalLengths:   []ValueCount{},
		FocalLengths35: []ValueCount{},
		Apertures:      []ValueCount{},
		ISOs:           []ValueCount{},
		CameraLens:     []CameraLensCount{},
		ShootingDays:   make(map[string]int),
	}

	// pathGlob is the LIKE pattern used on path_hint; empty means no path filter.
	pathGlob := ""
	if pathPrefix != "" {
		pathGlob = pathPrefix + "/%"
	}

	// photosCond is appended to queries directly on the photos table.
	photosCond := func() (string, []any) {
		if pathGlob == "" {
			return "", nil
		}
		return " AND path_hint LIKE ?", []any{pathGlob}
	}

	// exifJoin is an extra JOIN clause for queries that only touch exif_index.
	exifJoin := func() (string, string, []any) {
		if pathGlob == "" {
			return "", "", nil
		}
		return "JOIN photos _ph ON _ph.id = e.photo_id AND _ph.status='ok' AND _ph.path_hint LIKE ?",
			" AND e.photo_id IN (SELECT id FROM photos WHERE status='ok' AND path_hint LIKE ?)",
			[]any{pathGlob}
	}

	// Total count (indexed) and in-progress count (still being scanned).
	pcWhere, pcArgs := photosCond()
	if err := s.db.QueryRow(`SELECT COUNT(*) FROM photos WHERE status='ok'`+pcWhere,
		pcArgs...).Scan(&st.TotalPhotos); err != nil {
		return nil, err
	}
	if err := s.db.QueryRow(`SELECT COUNT(*) FROM photos WHERE status='missing'`+pcWhere,
		pcArgs...).Scan(&st.IndexingPhotos); err != nil {
		return nil, err
	}

	// Format distribution: use the pre-computed ext column — O(distinct formats) instead of O(n).
	{
		frows, ferr := s.db.Query(`SELECT ext, COUNT(*) AS n FROM photos WHERE status='ok' AND ext != ''`+pcWhere+` GROUP BY ext ORDER BY n DESC`, pcArgs...)
		if ferr != nil {
			return nil, ferr
		}
		defer frows.Close()
		for frows.Next() {
			var nc NameCount
			if err := frows.Scan(&nc.Name, &nc.Count); err != nil {
				return nil, err
			}
			st.Formats = append(st.Formats, nc)
		}
		frows.Close()
	}

	_, exifPathCond, exifPathArgs := exifJoin()

	// Film simulation distribution.
	{
		rows, err := s.db.Query(`
			SELECT value, COUNT(*) AS n FROM exif_index e WHERE e.field='FilmSimulation'`+
			exifPathCond+` GROUP BY value ORDER BY n DESC`, exifPathArgs...)
		if err != nil {
			return nil, err
		}
		defer rows.Close()
		for rows.Next() {
			var nc NameCount
			if err := rows.Scan(&nc.Name, &nc.Count); err != nil {
				return nil, err
			}
			st.FilmSims = append(st.FilmSims, nc)
		}
		rows.Close()
	}
	// Photos without any film simulation tag.
	{
		simTotal := 0
		for _, nc := range st.FilmSims {
			simTotal += nc.Count
		}
		if noneCount := st.TotalPhotos - simTotal; noneCount > 0 {
			st.FilmSims = append(st.FilmSims, NameCount{Name: "None", Count: noneCount})
		}
	}

	// Focal lengths (native mm) — deduplicated by distinct value.
	{
		rows, err := s.db.Query(`
			SELECT e.numeric_value, COUNT(*) AS n
			FROM exif_index e WHERE e.field='FocalLength' AND e.numeric_value IS NOT NULL`+
			exifPathCond+`
			GROUP BY e.numeric_value ORDER BY e.numeric_value`, exifPathArgs...)
		if err != nil {
			return nil, err
		}
		defer rows.Close()
		for rows.Next() {
			var vc ValueCount
			if err := rows.Scan(&vc.Value, &vc.Count); err != nil {
				return nil, err
			}
			st.FocalLengths = append(st.FocalLengths, vc)
		}
		rows.Close()
	}

	// Focal lengths (35mm equivalent): prefer FocalLengthIn35mmFilm, fall back to FocalLength per photo.
	// Deduplicated by distinct value via subquery.
	{
		pathCondStr, pathCondArgs := photosCond()
		rows, err := s.db.Query(`
			SELECT v, COUNT(*) AS n FROM (
				SELECT COALESCE(fl35.numeric_value, fl.numeric_value) AS v
				FROM   photos p
				LEFT JOIN exif_index fl   ON fl.photo_id  = p.id AND fl.field   = 'FocalLength'           AND fl.numeric_value   IS NOT NULL
				LEFT JOIN exif_index fl35 ON fl35.photo_id = p.id AND fl35.field = 'FocalLengthIn35mmFilm' AND fl35.numeric_value IS NOT NULL
				WHERE  p.status = 'ok'
				  AND  COALESCE(fl35.numeric_value, fl.numeric_value) IS NOT NULL`+pathCondStr+`
			) GROUP BY v ORDER BY v`, pathCondArgs...)
		if err != nil {
			return nil, err
		}
		defer rows.Close()
		for rows.Next() {
			var vc ValueCount
			if err := rows.Scan(&vc.Value, &vc.Count); err != nil {
				return nil, err
			}
			st.FocalLengths35 = append(st.FocalLengths35, vc)
		}
		rows.Close()
	}

	// Apertures (FNumber) — deduplicated by distinct value.
	{
		rows, err := s.db.Query(`
			SELECT e.numeric_value, COUNT(*) AS n
			FROM exif_index e WHERE e.field='FNumber' AND e.numeric_value IS NOT NULL`+
			exifPathCond+`
			GROUP BY e.numeric_value ORDER BY e.numeric_value`, exifPathArgs...)
		if err != nil {
			return nil, err
		}
		defer rows.Close()
		for rows.Next() {
			var vc ValueCount
			if err := rows.Scan(&vc.Value, &vc.Count); err != nil {
				return nil, err
			}
			st.Apertures = append(st.Apertures, vc)
		}
		rows.Close()
	}

	// ISOs — deduplicated by distinct value.
	{
		rows, err := s.db.Query(`
			SELECT e.numeric_value, COUNT(*) AS n
			FROM exif_index e WHERE e.field='ISOSpeedRatings' AND e.numeric_value IS NOT NULL`+
			exifPathCond+`
			GROUP BY e.numeric_value ORDER BY e.numeric_value`, exifPathArgs...)
		if err != nil {
			return nil, err
		}
		defer rows.Close()
		for rows.Next() {
			var vc ValueCount
			if err := rows.Scan(&vc.Value, &vc.Count); err != nil {
				return nil, err
			}
			st.ISOs = append(st.ISOs, vc)
		}
		rows.Close()
	}

	// Camera × lens combinations. LEFT JOIN on LensModel so cameras without a lens
	// tag (smartphones, film scanners) still appear under "(no lens)".
	{
		cameraJoin := " JOIN photos _ph ON _ph.id = c.photo_id AND _ph.status='ok'"
		var cameraArgs []any
		if pathGlob != "" {
			cameraJoin += " AND _ph.path_hint LIKE ?"
			cameraArgs = []any{pathGlob}
		}
		rows, err := s.db.Query(`
			SELECT c.value AS camera, COALESCE(l.value, '(no lens)') AS lens, COUNT(*) AS n
			FROM   exif_index c`+cameraJoin+`
			LEFT JOIN exif_index l ON c.photo_id = l.photo_id AND l.field = 'LensModel'
			WHERE  c.field = 'Model'
			GROUP BY camera, lens ORDER BY n DESC LIMIT 100`, cameraArgs...)
		if err != nil {
			return nil, err
		}
		defer rows.Close()
		for rows.Next() {
			var clc CameraLensCount
			if err := rows.Scan(&clc.Camera, &clc.Lens, &clc.Count); err != nil {
				return nil, err
			}
			st.CameraLens = append(st.CameraLens, clc)
		}
		rows.Close()
	}

	// Shooting hours distribution.
	{
		rows, err := s.db.Query(`
			SELECT CAST(SUBSTR(date_taken, 12, 2) AS INTEGER) AS hr, COUNT(*) AS n
			FROM   photos
			WHERE  status='ok'
			  AND  date_taken IS NOT NULL
			  AND  LENGTH(date_taken) >= 13`+pcWhere+
			` GROUP BY hr`, pcArgs...)
		if err != nil {
			return nil, err
		}
		defer rows.Close()
		for rows.Next() {
			var hr, n int
			if err := rows.Scan(&hr, &n); err != nil {
				return nil, err
			}
			if hr >= 0 && hr < 24 {
				st.ShootingHours[hr] = n
			}
		}
		rows.Close()
	}

	// Shooting days distribution (calendar heatmap).
	{
		rows, err := s.db.Query(`
			SELECT SUBSTR(date_taken, 1, 10) AS day, COUNT(*) AS n
			FROM   photos
			WHERE  status='ok' AND date_taken IS NOT NULL`+pcWhere+
			` GROUP BY day`, pcArgs...)
		if err != nil {
			return nil, err
		}
		defer rows.Close()
		for rows.Next() {
			var day string
			var n int
			if err := rows.Scan(&day, &n); err != nil {
				return nil, err
			}
			st.ShootingDays[day] = n
		}
		rows.Close()
	}

	return st, nil
}

func sortNameCounts(s []NameCount) {
	for i := 1; i < len(s); i++ {
		for j := i; j > 0 && s[j].Count > s[j-1].Count; j-- {
			s[j], s[j-1] = s[j-1], s[j]
		}
	}
}

func sortStrings(s []string) {
	for i := 1; i < len(s); i++ {
		for j := i; j > 0 && s[j] < s[j-1]; j-- {
			s[j], s[j-1] = s[j-1], s[j]
		}
	}
}

// FolderStats returns DB-backed statistics for all indexed photos under folderAbs.
// Results are derived entirely from the photos table and exif_index; no filesystem access.
func (s *Store) FolderStats(folderAbs string) (*LibraryFolderStats, error) {
	prefix := folderAbs + "/"
	glob := prefix + "*"

	// Summary: total photo count, size, and date range.
	var dateFirst, dateLast *string
	st := &LibraryFolderStats{Formats: []NameCount{}, Subfolders: []LibSubfolder{}}
	if err := s.db.QueryRow(
		`SELECT COUNT(*), COALESCE(SUM(file_size),0), MIN(date_taken), MAX(date_taken)
		 FROM photos WHERE status='ok' AND path_hint GLOB ?`,
		glob,
	).Scan(&st.PhotoCount, &st.TotalSize, &dateFirst, &dateLast); err != nil {
		return nil, err
	}
	if dateFirst != nil {
		st.DateFirst = *dateFirst
	}
	if dateLast != nil {
		st.DateLast = *dateLast
	}

	// Format distribution by file extension.
	{
		rows, err := s.db.Query(
			`SELECT ext, COUNT(*) AS n FROM photos
			 WHERE status='ok' AND ext != '' AND path_hint GLOB ?
			 GROUP BY ext ORDER BY n DESC`,
			glob,
		)
		if err != nil {
			return nil, err
		}
		defer rows.Close()
		for rows.Next() {
			var nc NameCount
			if err := rows.Scan(&nc.Name, &nc.Count); err != nil {
				return nil, err
			}
			st.Formats = append(st.Formats, nc)
		}
		rows.Close()
	}

	// Immediate subfolders: extract first path segment below prefix.
	// Reuses the GLOB+SUBSTR pattern from BrowseFolder.
	{
		sfRows, err := s.db.Query(
			`SELECT DISTINCT SUBSTR(path_hint, length(?)+1, INSTR(SUBSTR(path_hint, length(?)+1), '/')-1)
			 FROM photos WHERE status='ok' AND path_hint GLOB ?`,
			prefix, prefix, prefix+"*/*",
		)
		if err != nil {
			return nil, err
		}
		defer sfRows.Close()
		var names []string
		for sfRows.Next() {
			var name string
			if err := sfRows.Scan(&name); err != nil {
				return nil, err
			}
			if name != "" {
				names = append(names, name)
			}
		}
		sfRows.Close()
		sortStrings(names)

		for _, name := range names {
			var sub LibSubfolder
			sub.Name = name
			if err := s.db.QueryRow(
				`SELECT COUNT(*), COALESCE(SUM(file_size),0) FROM photos
				 WHERE status='ok' AND path_hint GLOB ?`,
				prefix+name+"/*",
			).Scan(&sub.PhotoCount, &sub.TotalSize); err != nil {
				return nil, err
			}
			st.Subfolders = append(st.Subfolders, sub)
		}
	}

	return st, nil
}

// Timeline returns time-series statistics for the given path scope.
// granularity is "month", "year", or "" (auto-detect from date span).
func (s *Store) Timeline(pathPrefix, granularity string) (*LibraryTimeline, error) {
	pathGlob := ""
	if pathPrefix != "" {
		pathGlob = pathPrefix + "/%"
	}
	pcWhere, pcArgs := tlPhotoCond(pathGlob)
	pWhere, pArgs := tlAliasCond(pathGlob)

	if granularity != "month" && granularity != "year" {
		granularity = tlDetectGranularity(s.db, pcWhere, pcArgs)
	}
	N := 7
	if granularity == "year" {
		N = 4
	}

	cameraRows, err := tlCameraRows(s.db, N, pWhere, pArgs)
	if err != nil {
		return nil, err
	}
	focalVals, err := tlOrderedFloats(s.db, "FocalLengthIn35mmFilm", N, pWhere, pArgs)
	if err != nil {
		return nil, err
	}
	isoVals, err := tlOrderedFloats(s.db, "ISOSpeedRatings", N, pWhere, pArgs)
	if err != nil {
		return nil, err
	}
	aperMap, err := tlApertureMap(s.db, N, pWhere, pArgs)
	if err != nil {
		return nil, err
	}
	aspectMap, err := tlAspectMap(s.db, N, pcWhere, pcArgs)
	if err != nil {
		return nil, err
	}
	mpStats, err := tlMegapixels(s.db, N, pcWhere, pcArgs)
	if err != nil {
		return nil, err
	}

	return assembleTL(granularity, cameraRows, focalVals, isoVals, aperMap, aspectMap, mpStats), nil
}

func tlPhotoCond(pathGlob string) (string, []any) {
	if pathGlob == "" {
		return "", nil
	}
	return " AND path_hint LIKE ?", []any{pathGlob}
}

func tlAliasCond(pathGlob string) (string, []any) {
	if pathGlob == "" {
		return "", nil
	}
	return " AND p.path_hint LIKE ?", []any{pathGlob}
}

func tlDetectGranularity(db *sql.DB, pcWhere string, pcArgs []any) string {
	var minP, maxP sql.NullString
	db.QueryRow(`
		SELECT MIN(SUBSTR(date_taken,1,7)),
		       MAX(SUBSTR(date_taken,1,7))
		FROM photos WHERE status='ok'
		  AND date_taken IS NOT NULL`+pcWhere,
		pcArgs...).Scan(&minP, &maxP)
	if !minP.Valid || !maxP.Valid || len(minP.String) < 7 || len(maxP.String) < 7 {
		return "month"
	}
	minT, err1 := time.Parse("2006-01", minP.String)
	maxT, err2 := time.Parse("2006-01", maxP.String)
	if err1 != nil || err2 != nil {
		return "month"
	}
	spanMonths := (maxT.Year()-minT.Year())*12 + int(maxT.Month()-minT.Month())
	if spanMonths > 48 {
		return "year"
	}
	return "month"
}

type tlCameraRow struct {
	period string
	camera string
	count  int
}

func tlCameraRows(db *sql.DB, N int, pWhere string, pArgs []any) ([]tlCameraRow, error) {
	rows, err := db.Query(`
		SELECT SUBSTR(p.date_taken,1,?), e.value, COUNT(*)
		FROM photos p
		JOIN exif_index e ON e.photo_id = p.id AND e.field = 'Model'
		WHERE p.status='ok'
		  AND p.date_taken IS NOT NULL`+pWhere+`
		GROUP BY 1, 2 ORDER BY 1, 3 DESC`,
		append([]any{N}, pArgs...)...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []tlCameraRow
	for rows.Next() {
		var r tlCameraRow
		if err := rows.Scan(&r.period, &r.camera, &r.count); err != nil {
			return nil, err
		}
		r.camera = stripExifQuotes(r.camera)
		out = append(out, r)
	}
	return out, nil
}

func tlOrderedFloats(db *sql.DB, field string, N int, pWhere string, pArgs []any) (map[string][]float64, error) {
	rows, err := db.Query(`
		SELECT SUBSTR(p.date_taken,1,?), e.numeric_value
		FROM photos p
		JOIN exif_index e ON e.photo_id = p.id AND e.field = ?
		WHERE p.status='ok'
		  AND p.date_taken IS NOT NULL
		  AND e.numeric_value IS NOT NULL`+pWhere+`
		ORDER BY 1, 2`,
		append([]any{N, field}, pArgs...)...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make(map[string][]float64)
	for rows.Next() {
		var period string
		var val float64
		if err := rows.Scan(&period, &val); err != nil {
			return nil, err
		}
		out[period] = append(out[period], val)
	}
	return out, nil
}

func tlApertureMap(db *sql.DB, N int, pWhere string, pArgs []any) (map[string]map[string]int, error) {
	rows, err := db.Query(`
		SELECT SUBSTR(p.date_taken,1,?),
		       CASE
		         WHEN e.numeric_value <= 1.2  THEN 'f/1'
		         WHEN e.numeric_value <= 1.6  THEN 'f/1.4'
		         WHEN e.numeric_value <= 2.3  THEN 'f/2'
		         WHEN e.numeric_value <= 3.3  THEN 'f/2.8'
		         WHEN e.numeric_value <= 4.7  THEN 'f/4'
		         WHEN e.numeric_value <= 6.5  THEN 'f/5.6'
		         WHEN e.numeric_value <= 9.5  THEN 'f/8'
		         WHEN e.numeric_value <= 13.0 THEN 'f/11'
		         ELSE 'f/16+'
		       END,
		       COUNT(*)
		FROM photos p
		JOIN exif_index e ON e.photo_id = p.id AND e.field = 'FNumber'
		WHERE p.status='ok'
		  AND p.date_taken IS NOT NULL
		  AND e.numeric_value IS NOT NULL`+pWhere+`
		GROUP BY 1, 2 ORDER BY 1`,
		append([]any{N}, pArgs...)...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make(map[string]map[string]int)
	for rows.Next() {
		var period, bucket string
		var count int
		if err := rows.Scan(&period, &bucket, &count); err != nil {
			return nil, err
		}
		if out[period] == nil {
			out[period] = make(map[string]int)
		}
		out[period][bucket] = count
	}
	return out, nil
}

func tlAspectMap(db *sql.DB, N int, pcWhere string, pcArgs []any) (map[string]map[string]int, error) {
	rows, err := db.Query(`
		SELECT SUBSTR(date_taken,1,?),
		       CASE
		         WHEN CAST(json_extract(exif_json,'$.width') AS REAL) /
		              CAST(json_extract(exif_json,'$.height') AS REAL) BETWEEN 0.98 AND 1.02 THEN '1:1'
		         WHEN CAST(json_extract(exif_json,'$.width') AS REAL) /
		              CAST(json_extract(exif_json,'$.height') AS REAL) BETWEEN 1.28 AND 1.42 THEN '4:3'
		         WHEN CAST(json_extract(exif_json,'$.width') AS REAL) /
		              CAST(json_extract(exif_json,'$.height') AS REAL) BETWEEN 1.45 AND 1.58 THEN '3:2'
		         WHEN CAST(json_extract(exif_json,'$.width') AS REAL) /
		              CAST(json_extract(exif_json,'$.height') AS REAL) > 1.65 THEN '16:9+'
		         ELSE 'other'
		       END,
		       COUNT(*)
		FROM photos
		WHERE status='ok'
		  AND date_taken IS NOT NULL
		  AND CAST(json_extract(exif_json,'$.width') AS INTEGER) > 0
		  AND CAST(json_extract(exif_json,'$.height') AS INTEGER) > 0`+pcWhere+`
		GROUP BY 1, 2 ORDER BY 1`,
		append([]any{N}, pcArgs...)...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make(map[string]map[string]int)
	for rows.Next() {
		var period, aspect string
		var count int
		if err := rows.Scan(&period, &aspect, &count); err != nil {
			return nil, err
		}
		if out[period] == nil {
			out[period] = make(map[string]int)
		}
		out[period][aspect] = count
	}
	return out, nil
}

func tlMegapixels(db *sql.DB, N int, pcWhere string, pcArgs []any) ([]MegapixelStat, error) {
	rows, err := db.Query(`
		SELECT SUBSTR(date_taken,1,?),
		       MAX(CAST(json_extract(exif_json,'$.width') AS REAL) *
		           CAST(json_extract(exif_json,'$.height') AS REAL) / 1000000.0),
		       AVG(CAST(json_extract(exif_json,'$.width') AS REAL) *
		           CAST(json_extract(exif_json,'$.height') AS REAL) / 1000000.0),
		       COUNT(*)
		FROM photos
		WHERE status='ok'
		  AND date_taken IS NOT NULL
		  AND CAST(json_extract(exif_json,'$.width') AS INTEGER) > 0
		  AND CAST(json_extract(exif_json,'$.height') AS INTEGER) > 0`+pcWhere+`
		GROUP BY 1 ORDER BY 1`,
		append([]any{N}, pcArgs...)...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []MegapixelStat
	for rows.Next() {
		var ms MegapixelStat
		if err := rows.Scan(&ms.Period, &ms.Max, &ms.Avg, &ms.Count); err != nil {
			return nil, err
		}
		out = append(out, ms)
	}
	return out, nil
}

func assembleTL(
	granularity string,
	cameraRows []tlCameraRow,
	focalVals, isoVals map[string][]float64,
	aperMap, aspectMap map[string]map[string]int,
	mpStats []MegapixelStat,
) *LibraryTimeline {
	periodSet := make(map[string]bool)
	for _, r := range cameraRows {
		periodSet[r.period] = true
	}
	for p := range focalVals {
		periodSet[p] = true
	}
	for p := range isoVals {
		periodSet[p] = true
	}
	for p := range aperMap {
		periodSet[p] = true
	}
	for p := range aspectMap {
		periodSet[p] = true
	}
	for _, ms := range mpStats {
		periodSet[ms.Period] = true
	}

	periods := make([]string, 0, len(periodSet))
	for p := range periodSet {
		periods = append(periods, p)
	}
	sortStrings(periods)
	periodIdx := make(map[string]int, len(periods))
	for i, p := range periods {
		periodIdx[p] = i
	}

	return &LibraryTimeline{
		Granularity:    granularity,
		Periods:        periods,
		CameraUsage:    assembleCameraSlices(cameraRows, periods, periodIdx),
		FocalStats:     assemblePercentileStats(focalVals, periods),
		ISOStats:       assemblePercentileStats(isoVals, periods),
		ApertureHeat:   assembleApertureRows(aperMap, periods),
		AspectRatios:   assembleAspectSlices(aspectMap, periods, periodIdx),
		MegapixelStats: assembleMPStats(mpStats, periodIdx),
	}
}

func assembleCameraSlices(rows []tlCameraRow, periods []string, idx map[string]int) []CameraTimeSlice {
	totals := make(map[string]int)
	grid := make(map[string][]int)
	for _, r := range rows {
		pi, ok := idx[r.period]
		if !ok {
			continue
		}
		if grid[r.camera] == nil {
			grid[r.camera] = make([]int, len(periods))
		}
		grid[r.camera][pi] += r.count
		totals[r.camera] += r.count
	}

	type kv struct {
		k string
		v int
	}
	ranked := make([]kv, 0, len(totals))
	for k, v := range totals {
		ranked = append(ranked, kv{k, v})
	}
	for i := 1; i < len(ranked); i++ {
		for j := i; j > 0 && ranked[j].v > ranked[j-1].v; j-- {
			ranked[j], ranked[j-1] = ranked[j-1], ranked[j]
		}
	}

	top := 5
	if len(ranked) < top {
		top = len(ranked)
	}
	topSet := make(map[string]bool, top)
	for _, kv := range ranked[:top] {
		topSet[kv.k] = true
	}

	out := make([]CameraTimeSlice, 0, top+1)
	for _, kv := range ranked[:top] {
		out = append(out, CameraTimeSlice{Camera: kv.k, Counts: grid[kv.k]})
	}
	other := make([]int, len(periods))
	hasOther := false
	for cam, counts := range grid {
		if topSet[cam] {
			continue
		}
		hasOther = true
		for i, c := range counts {
			other[i] += c
		}
	}
	if hasOther {
		out = append(out, CameraTimeSlice{Camera: "Other", Counts: other})
	}
	return out
}

func assemblePercentileStats(vals map[string][]float64, periods []string) []PeriodStats {
	out := make([]PeriodStats, 0, len(periods))
	for _, p := range periods {
		sorted := vals[p]
		if len(sorted) == 0 {
			continue
		}
		p25, median, p75 := computePercentiles(sorted)
		out = append(out, PeriodStats{Period: p, Median: median, P25: p25, P75: p75, Count: len(sorted)})
	}
	return out
}

func computePercentiles(sorted []float64) (p25, median, p75 float64) {
	n := len(sorted)
	return sorted[n/4], sorted[n/2], sorted[n*3/4]
}

func assembleApertureRows(aperMap map[string]map[string]int, periods []string) []ApertureRow {
	out := make([]ApertureRow, 0, len(periods))
	for _, p := range periods {
		buckets := aperMap[p]
		if len(buckets) == 0 {
			continue
		}
		cp := make(map[string]int, len(buckets))
		for k, v := range buckets {
			cp[k] = v
		}
		out = append(out, ApertureRow{Period: p, Buckets: cp})
	}
	return out
}

var tlAspectOrder = []string{"3:2", "4:3", "16:9+", "1:1", "other"}

func assembleAspectSlices(aspectMap map[string]map[string]int, periods []string, idx map[string]int) []AspectSlice {
	grid := make(map[string][]int, len(tlAspectOrder))
	for _, ratio := range tlAspectOrder {
		grid[ratio] = make([]int, len(periods))
	}
	for period, ratios := range aspectMap {
		pi, ok := idx[period]
		if !ok {
			continue
		}
		for ratio, count := range ratios {
			if _, known := grid[ratio]; !known {
				grid[ratio] = make([]int, len(periods))
			}
			grid[ratio][pi] += count
		}
	}
	out := make([]AspectSlice, 0, len(tlAspectOrder))
	for _, ratio := range tlAspectOrder {
		counts := grid[ratio]
		for _, c := range counts {
			if c > 0 {
				out = append(out, AspectSlice{Ratio: ratio, Counts: counts})
				break
			}
		}
	}
	return out
}

func stripExifQuotes(s string) string {
	if len(s) >= 2 && s[0] == '"' && s[len(s)-1] == '"' {
		return s[1 : len(s)-1]
	}
	return s
}

func assembleMPStats(mpStats []MegapixelStat, idx map[string]int) []MegapixelStat {
	out := make([]MegapixelStat, 0, len(mpStats))
	for _, ms := range mpStats {
		if _, ok := idx[ms.Period]; ok {
			out = append(out, ms)
		}
	}
	return out
}

// ExifRange holds the minimum and maximum numeric_value for a single EXIF field.
type ExifRange struct {
	Min float64 `json:"min"`
	Max float64 `json:"max"`
}

// GetExifRanges returns the min/max numeric_value for each of the requested EXIF fields.
// Fields with no numeric data are omitted from the result.
// The virtual key "FocalLength35" returns the combined range of FocalLengthIn35mmFilm
// and FocalLength — matching the filter semantics for the 35mm slider.
func (s *Store) GetExifRanges(fields []string) (map[string]ExifRange, error) {
	out := make(map[string]ExifRange)

	// Separate "FocalLength35" (virtual, needs special query) from real fields.
	var realFields []string
	hasFocal35 := false
	for _, f := range fields {
		if f == "FocalLength35" {
			hasFocal35 = true
		} else {
			realFields = append(realFields, f)
		}
	}

	// Batch all real fields into a single GROUP BY query.
	if len(realFields) > 0 {
		placeholders := strings.Repeat("?,", len(realFields))
		placeholders = placeholders[:len(placeholders)-1]
		args := make([]any, len(realFields))
		for i, f := range realFields {
			args[i] = f
		}
		rows, err := s.db.Query(
			`SELECT field, MIN(numeric_value), MAX(numeric_value)
			 FROM exif_index
			 WHERE field IN (`+placeholders+`) AND numeric_value IS NOT NULL
			 GROUP BY field`,
			args...,
		)
		if err != nil {
			return nil, err
		}
		defer rows.Close()
		for rows.Next() {
			var field string
			var minVal, maxVal sql.NullFloat64
			if err := rows.Scan(&field, &minVal, &maxVal); err != nil {
				return nil, err
			}
			if minVal.Valid && maxVal.Valid {
				out[field] = ExifRange{Min: minVal.Float64, Max: maxVal.Float64}
			}
		}
		if err := rows.Err(); err != nil {
			return nil, err
		}
	}

	// FocalLength35: combined range of FocalLengthIn35mmFilm and FocalLength.
	if hasFocal35 {
		var minVal, maxVal sql.NullFloat64
		err := s.db.QueryRow(
			`SELECT MIN(numeric_value), MAX(numeric_value)
			 FROM exif_index
			 WHERE field IN ('FocalLengthIn35mmFilm','FocalLength')
			   AND numeric_value IS NOT NULL`,
		).Scan(&minVal, &maxVal)
		if err != nil {
			return nil, err
		}
		if minVal.Valid && maxVal.Valid {
			out["FocalLength35"] = ExifRange{Min: minVal.Float64, Max: maxVal.Float64}
		}
	}

	return out, nil
}
