package library

import (
	"database/sql"
	"strings"
)

// coverQueries gives the statistics and the search indexes that hold every
// column they read, so they never fetch a photo's row, which carries its
// whole EXIF as JSON (1.7 KB on average). Measured on a library of 36,000
// photos: the statistics took 7.8 s and now 0.4 s; a search by hue ran for
// minutes and blocked the library's only connection.
//
//   - exif_index is WITHOUT ROWID: the table is its (photo_id, field) key, so
//     a lookup by photo finds the value at once, and each secondary index
//     carries photo_id without storing it twice.
//   - photos has (status, date_taken, id) and (status, path_hint, date_taken,
//     id) for the counts by date, hour and folder.
//
// Rebuilding exif_index takes about 11 s for 1.7 million rows, once.
func coverQueries(db *sql.DB) error {
	if err := exifWithoutRowid(db); err != nil {
		return err
	}
	for _, q := range []string{
		`CREATE INDEX IF NOT EXISTS photos_status_date_id_idx ON photos(status, date_taken, id)`,
		`CREATE INDEX IF NOT EXISTS photos_status_path_date_idx ON photos(status, path_hint, date_taken, id)`,
		`DROP INDEX IF EXISTS photos_status_path_idx`, // a prefix of the one above
	} {
		if _, err := db.Exec(q); err != nil {
			return err
		}
	}
	return nil
}

func exifWithoutRowid(db *sql.DB) error {
	var ddl string
	if err := db.QueryRow(`SELECT sql FROM sqlite_master WHERE type='table' AND name='exif_index'`).Scan(&ddl); err != nil {
		return err
	}
	if strings.Contains(strings.ToUpper(ddl), "WITHOUT ROWID") {
		return nil
	}
	tx, err := db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback() //nolint:errcheck
	for _, q := range []string{
		`CREATE TABLE exif_index_new (
			photo_id      TEXT NOT NULL REFERENCES photos(id),
			field         TEXT NOT NULL,
			value         TEXT NOT NULL,
			numeric_value REAL,
			PRIMARY KEY (photo_id, field)
		) WITHOUT ROWID`,
		`INSERT INTO exif_index_new (photo_id, field, value, numeric_value)
			SELECT photo_id, field, value, numeric_value FROM exif_index`,
		`DROP TABLE exif_index`,
		`ALTER TABLE exif_index_new RENAME TO exif_index`,
		`CREATE INDEX exif_index_field_value ON exif_index(field, value)`,
		`CREATE INDEX exif_index_field_numeric ON exif_index(field, numeric_value)`,
	} {
		if _, err := tx.Exec(q); err != nil {
			return err
		}
	}
	if err := tx.Commit(); err != nil {
		return err
	}
	// The rebuilt table sits in the write-ahead log until the next write
	// folds it in; until then every read searches 500 MB of log.
	_, err = db.Exec(`PRAGMA wal_checkpoint(TRUNCATE)`)
	return err
}
