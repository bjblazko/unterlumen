package library

import (
	"database/sql"
	"fmt"
	"huepattl.de/unterlumen/internal/things"
	"strings"
	"sync"

	_ "modernc.org/sqlite"

	"huepattl.de/unterlumen/internal/facedecisions"
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

-- The faces found in a photo's thumbnail and who they are (ADR-0051).
-- person_id NULL with confirmed 0 is a face nobody has named yet; with
-- confirmed 1 one the owner said not to name. grp groups unnamed faces
-- that look alike; it is rebuilt after every pass. manual 1 is a face the
-- owner marked by hand where none was found: always named, no embedding.
CREATE TABLE IF NOT EXISTS faces (
	id         INTEGER PRIMARY KEY,
	photo_id   TEXT NOT NULL REFERENCES photos(id),
	x REAL NOT NULL, y REAL NOT NULL, w REAL NOT NULL, h REAL NOT NULL,
	score      REAL NOT NULL,
	embedding  BLOB NOT NULL,
	person_id  INTEGER REFERENCES people(id),
	confirmed  INTEGER NOT NULL DEFAULT 0,
	grp        INTEGER,
	manual     INTEGER NOT NULL DEFAULT 0
);
CREATE INDEX IF NOT EXISTS faces_photo  ON faces(photo_id);
CREATE INDEX IF NOT EXISTS faces_person ON faces(person_id);
CREATE INDEX IF NOT EXISTS faces_grp    ON faces(grp);

-- The photos whose faces were looked for, with zero faces or more.
CREATE TABLE IF NOT EXISTS photo_faces_found (
	photo_id TEXT PRIMARY KEY REFERENCES photos(id),
	version  INTEGER NOT NULL
);

-- A No: the face is not that person, so it is not suggested as them
-- again (ADR-0051, decision 13). It goes with its face and its person.
CREATE TABLE IF NOT EXISTS face_not (
	face   INTEGER NOT NULL REFERENCES faces(id) ON DELETE CASCADE,
	person INTEGER NOT NULL REFERENCES people(id) ON DELETE CASCADE,
	PRIMARY KEY (face, person)
);

-- Things (ADR-0054). photo_vectors is the model's vector of a photo, for
-- searching by words. photo_things_found says which models (version) and
-- word list (vocabulary) looked at a photo, whether its sidecar holds what
-- they found (written), and the vector too (vector_in_sidecar). photo_things are the things in a photo, from
-- clip, the detector, or the sidecar alone. thing_boxes are the detector's
-- boxes, kept for telling pets apart.
CREATE TABLE IF NOT EXISTS photo_vectors (
	photo_id TEXT PRIMARY KEY REFERENCES photos(id),
	vector   BLOB NOT NULL
);
CREATE TABLE IF NOT EXISTS photo_things_found (
	photo_id   TEXT PRIMARY KEY REFERENCES photos(id),
	version    INTEGER NOT NULL,
	vocabulary INTEGER NOT NULL,
	written    INTEGER NOT NULL DEFAULT 0,
	vector_in_sidecar INTEGER NOT NULL DEFAULT 0
);
CREATE TABLE IF NOT EXISTS photo_things (
	photo_id TEXT NOT NULL REFERENCES photos(id),
	thing    TEXT NOT NULL,
	score    REAL NOT NULL,
	source   TEXT NOT NULL,
	PRIMARY KEY (photo_id, thing)
);
CREATE INDEX IF NOT EXISTS photo_things_thing ON photo_things(thing, photo_id);
CREATE TABLE IF NOT EXISTS thing_boxes (
	id       INTEGER PRIMARY KEY,
	photo_id TEXT NOT NULL REFERENCES photos(id),
	class    TEXT NOT NULL,
	score    REAL NOT NULL,
	x REAL NOT NULL, y REAL NOT NULL, w REAL NOT NULL, h REAL NOT NULL
);
CREATE INDEX IF NOT EXISTS thing_boxes_photo ON thing_boxes(photo_id);

CREATE TABLE IF NOT EXISTS people (
	id    INTEGER PRIMARY KEY,
	name  TEXT NOT NULL UNIQUE COLLATE NOCASE,
	cover INTEGER, -- the face the owner chose to show them; their largest when NULL or no longer theirs
	cover_chosen TEXT -- when cover was chosen: across libraries, the cover chosen last shows them
);

-- A photo whose file was changed in place (a location set, a crop) has new
-- content and so a new ID; what still names the old ID — a gallery draft,
-- the shared album register — is led to the new one.
CREATE TABLE IF NOT EXISTS photo_replaced (
	old_id TEXT PRIMARY KEY,
	new_id TEXT NOT NULL
);

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
	// decisions is where hiding, No's and covers are shared with the other
	// installation; nil keeps them in this index only.
	decisions *facedecisions.Store
	lex       *things.Language // the library's keyword language, read once (lexicon)
	lexOnce   sync.Once
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
	// Migration: the face chosen to show a person.
	db.Exec(`ALTER TABLE people ADD COLUMN cover INTEGER`)
	db.Exec(`ALTER TABLE people ADD COLUMN cover_chosen TEXT`)
	// Migration: faces marked by hand.
	db.Exec(`ALTER TABLE faces ADD COLUMN manual INTEGER NOT NULL DEFAULT 0`)
	// Migration: the stamp of the sidecar a scan read last.
	db.Exec(`ALTER TABLE path_cache ADD COLUMN sidecar_stamp TEXT`)
	// Migration: whether a photo's sidecar holds its vector too (ADR-0054).
	db.Exec(`ALTER TABLE photo_things_found ADD COLUMN vector_in_sidecar INTEGER NOT NULL DEFAULT 0`)
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

// props is every property of the library.
func (s *Store) props() (map[string]string, error) {
	rows, err := s.db.Query(`SELECT key, value FROM library_props`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := map[string]string{}
	for rows.Next() {
		var k, v string
		if err := rows.Scan(&k, &v); err != nil {
			return nil, err
		}
		out[k] = v
	}
	return out, rows.Err()
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

// sidecarReading names what a scan reads from a sidecar. When it changes —
// a version that reads something new from them — every sidecar is read once
// again, whatever its stamp (forgetSidecarStamps).
const sidecarReading = "1"

func (s *Store) sidecarStampOf(absPath string) string {
	var stamp sql.NullString
	s.db.QueryRow(`SELECT sidecar_stamp FROM path_cache WHERE abs_path = ?`, absPath).Scan(&stamp) //nolint:errcheck
	return stamp.String
}

func (s *Store) setSidecarStamp(absPath, stamp string) error {
	if stamp == "" {
		return nil
	}
	_, err := s.db.Exec(`UPDATE path_cache SET sidecar_stamp = ? WHERE abs_path = ?`, stamp, absPath)
	return err
}

// forgetSidecarStamps has every sidecar read again when what a scan reads
// from them changed since the stamps were taken.
func (s *Store) forgetSidecarStamps() {
	// Notes moving into the sidecars changes what a scan does with them too.
	reading := sidecarReading
	if s.NotesInSidecars() {
		reading += "+notes"
	}
	if v, _, _ := s.GetProp("sidecar_reading"); v == reading {
		return
	}
	if _, err := s.db.Exec(`UPDATE path_cache SET sidecar_stamp = NULL`); err == nil {
		s.SetProp("sidecar_reading", reading) //nolint:errcheck
	}
}

// keywordLanguageProp names the library's keyword language (ADR-0056).
const keywordLanguageProp = "keyword_language"

// lexicon is the language this library writes things in as keywords,
// read once per store: a scan asks for every photo.
func (s *Store) lexicon() *things.Language {
	s.lexOnce.Do(func() {
		code, _, _ := s.GetProp(keywordLanguageProp)
		s.lex = things.LanguageOf(code)
	})
	return s.lex
}
