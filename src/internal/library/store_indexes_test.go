package library

import (
	"database/sql"
	"path/filepath"
	"strings"
	"testing"

	_ "modernc.org/sqlite"
)

// A library indexed before keeps its EXIF index when the table is rebuilt
// without rowid, and the search and statistics find it as before.
func TestCoverQueriesMigratesExifIndex(t *testing.T) {
	path := filepath.Join(t.TempDir(), "library.db")
	old, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatal(err)
	}
	for _, q := range []string{
		`CREATE TABLE photos (id TEXT PRIMARY KEY, path_hint TEXT NOT NULL, filename TEXT NOT NULL, file_size INTEGER NOT NULL,
			indexed_at DATETIME NOT NULL, exif_json TEXT, thumb_path TEXT, status TEXT NOT NULL DEFAULT 'ok')`,
		`CREATE TABLE exif_index (photo_id TEXT NOT NULL REFERENCES photos(id), field TEXT NOT NULL, value TEXT NOT NULL,
			numeric_value REAL, PRIMARY KEY (photo_id, field))`,
		`CREATE INDEX photos_status_path_idx ON photos(status, path_hint)`,
		`INSERT INTO photos (id, path_hint, filename, file_size, indexed_at, exif_json) VALUES ('p1', '/lib/a.jpg', 'a.jpg', 1, '2026-01-01', '{"dateTaken":"2024-05-01T10:00:00"}')`,
		`INSERT INTO exif_index VALUES ('p1', 'Model', '"X-T50"', NULL), ('p1', 'FNumber', '2', 2)`,
	} {
		if _, err := old.Exec(q); err != nil {
			t.Fatal(err)
		}
	}
	old.Close()

	db, err := openDB(path)
	if err != nil {
		t.Fatal(err)
	}
	var ddl string
	db.QueryRow(`SELECT sql FROM sqlite_master WHERE name='exif_index'`).Scan(&ddl)
	if !strings.Contains(ddl, "WITHOUT ROWID") {
		t.Errorf("exif_index not rebuilt: %s", ddl)
	}
	var n int
	db.QueryRow(`SELECT COUNT(*) FROM sqlite_master WHERE name='photos_status_path_idx'`).Scan(&n)
	if n != 0 {
		t.Error("the old (status, path_hint) index is still there")
	}
	s := newStore(db, "")
	ids, _ := listedIDs(t, s, ListPhotosOpts{Filters: map[string]string{"Model": "X-T50"}, NumericFilters: map[string]NumericFilter{"FNumber": {Min: 1.9, Max: 2.1}}})
	if len(ids) != 1 || ids[0] != "p1" {
		t.Errorf("search after migration: %v", ids)
	}
	// Opening again leaves the rebuilt table as it is.
	if err := coverQueries(db); err != nil {
		t.Fatal(err)
	}
}
