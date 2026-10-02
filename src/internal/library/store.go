package library

import (
	"database/sql"
	"fmt"
	"strings"

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
) WITHOUT ROWID;
CREATE INDEX IF NOT EXISTS exif_index_field_value ON exif_index(field, value);
CREATE INDEX IF NOT EXISTS exif_index_field_numeric ON exif_index(field, numeric_value);

CREATE TABLE IF NOT EXISTS photo_meta (
	photo_id    TEXT NOT NULL REFERENCES photos(id),
	key         TEXT NOT NULL,
	value       TEXT NOT NULL,
	updated_at  DATETIME NOT NULL,
	PRIMARY KEY (photo_id, key)
);

-- What a photo looks like, measured from its thumbnail (ADR-0044).
CREATE TABLE IF NOT EXISTS photo_appearance (
	photo_id      TEXT PRIMARY KEY REFERENCES photos(id),
	version       INTEGER NOT NULL,
	analysed_at   DATETIME NOT NULL,
	mono_class    TEXT NOT NULL,
	tint_hue      REAL,
	avg_l REAL, avg_c REAL, avg_h REAL,
	colourfulness REAL,
	warmth        REAL,
	lum_mean REAL, lum_median REAL, lum_p05 REAL, lum_p95 REAL,
	contrast      REAL,
	tone_key      TEXT NOT NULL,
	clip_high REAL, clip_low REAL,
	lum_hist      BLOB,
	entropy REAL, sharpness REAL, edge_density REAL,
	centroid_x REAL, centroid_y REAL,
	dhash         INTEGER
);
CREATE INDEX IF NOT EXISTS photo_appearance_version ON photo_appearance(version);

CREATE TABLE IF NOT EXISTS photo_palette (
	photo_id TEXT NOT NULL REFERENCES photos(id),
	rank     INTEGER NOT NULL,
	l REAL NOT NULL, c REAL NOT NULL, h REAL NOT NULL,
	hue_bin  INTEGER,
	share    REAL NOT NULL,
	PRIMARY KEY (photo_id, rank)
);
CREATE INDEX IF NOT EXISTS photo_palette_hue ON photo_palette(hue_bin, share);

CREATE TABLE IF NOT EXISTS library_props (
	key         TEXT PRIMARY KEY,
	value       TEXT NOT NULL
);
CREATE INDEX IF NOT EXISTS photos_status_idx ON photos(status);
CREATE INDEX IF NOT EXISTS photos_indexed_at_idx ON photos(indexed_at);
CREATE INDEX IF NOT EXISTS photos_status_indexed_at_idx ON photos(status, indexed_at);
`

// Store wraps the per-library SQLite database.
type Store struct {
	db     *sql.DB
	dir    string
	filter Filter // narrows the statistics, map and timeline queries (ADR-0050)
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
	// Migration: indexed_at index for browse/sort.
	db.Exec(`CREATE INDEX IF NOT EXISTS photos_indexed_at_idx ON photos(indexed_at)`)
	// Migration: date_taken column for fast date-based stats and timeline queries.
	db.Exec(`ALTER TABLE photos ADD COLUMN date_taken TEXT`)
	db.Exec(`UPDATE photos SET date_taken = json_extract(exif_json,'$.dateTaken') WHERE date_taken IS NULL`)
	// Migration: a photo without a date has no date_taken. Older indexes stored ''
	// instead, which counted as a day and passed every upper date bound.
	db.Exec(`UPDATE photos SET date_taken = NULL WHERE date_taken = ''`)
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
	if err := coverQueries(db); err != nil {
		db.Close()
		return nil, fmt.Errorf("migrate indexes: %w", err)
	}
	return db, nil
}

func newStore(db *sql.DB, dir string) *Store {
	return &Store{db: db, dir: dir}
}

// likeEscaper escapes SQL LIKE metacharacters (\, %, _) so a folder path
// can be used as a literal prefix in a LIKE pattern. Every query built with
// this must pair the pattern with "LIKE ? ESCAPE '\'" — without escaping,
// an underscore in a real folder name (e.g. "2024_Trip") matches any single
// character, silently pulling in sibling folders like "2024XTrip".
var likeEscaper = strings.NewReplacer(`\`, `\\`, `%`, `\%`, `_`, `\_`)

func escapeLikePattern(s string) string {
	return likeEscaper.Replace(s)
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
