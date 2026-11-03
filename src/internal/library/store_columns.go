package library

import (
	"database/sql"
	"fmt"
)

// A photo's width, height and place as columns of its own (ADR-0063). The
// Frame and Places views, the map, the timeline's sizes and the search by
// shape read them for every photo; from the EXIF JSON that meant reading
// each photo's whole row (1.7 KB) and parsing it, 1–2 s per view on 33,000
// photos. As columns they sit in a covering index. Triggers fill them from
// exif_json whenever a row is written, so no writer has to remember to.

// photoColumnsDone names the filling of the columns of photos indexed before
// they existed; once done it is recorded in library_props.
const photoColumnsDone = "photo_columns"

// columnsFrom is the SET that reads the columns from a row's EXIF JSON.
func columnsFrom(exif string) string {
	at := func(path, as string) string {
		return "CASE WHEN json_valid(" + exif + ") THEN CAST(json_extract(" + exif + ", '$." + path + "') AS " + as + ") END"
	}
	return "width = " + at("width", "INTEGER") + ", height = " + at("height", "INTEGER") +
		", lat = " + at("latitude", "REAL") + ", lon = " + at("longitude", "REAL")
}

func photoColumns(db *sql.DB) error {
	for _, c := range []string{"width INTEGER", "height INTEGER", "lat REAL", "lon REAL"} {
		db.Exec(`ALTER TABLE photos ADD COLUMN ` + c) //nolint:errcheck // there already in a new or migrated index
	}
	for _, q := range []string{
		`CREATE TRIGGER IF NOT EXISTS photos_columns_insert AFTER INSERT ON photos BEGIN
			UPDATE photos SET ` + columnsFrom("NEW.exif_json") + ` WHERE id = NEW.id;
		END`,
		`CREATE TRIGGER IF NOT EXISTS photos_columns_update AFTER UPDATE OF exif_json ON photos BEGIN
			UPDATE photos SET ` + columnsFrom("NEW.exif_json") + ` WHERE id = NEW.id;
		END`,
		`CREATE INDEX IF NOT EXISTS photos_status_size_idx ON photos(status, width, height, date_taken, id)`,
		`CREATE INDEX IF NOT EXISTS photos_status_place_idx ON photos(status, lat, lon, date_taken, id)`,
	} {
		if _, err := db.Exec(q); err != nil {
			return fmt.Errorf("photo columns: %w", err)
		}
	}
	var done string
	db.QueryRow(`SELECT value FROM library_props WHERE key = ?`, photoColumnsDone).Scan(&done) //nolint:errcheck // none yet
	if done == "1" {
		return nil
	}
	return fillPhotoColumns(db)
}

// fillPhotoColumns fills the columns of the photos indexed before them, in
// one step: from the JSON, and the place of photos indexed before the
// coordinates were parsed at index time from their GPS tags.
func fillPhotoColumns(db *sql.DB) error {
	tx, err := db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback() //nolint:errcheck
	if _, err := tx.Exec(`UPDATE photos SET ` + columnsFrom("exif_json")); err != nil {
		return err
	}
	if err := placesFromTags(tx); err != nil {
		return err
	}
	if _, err := tx.Exec(`INSERT OR REPLACE INTO library_props (key, value) VALUES (?, '1')`, photoColumnsDone); err != nil {
		return err
	}
	if err := tx.Commit(); err != nil {
		return err
	}
	// Every row was rewritten; fold the log in rather than search it.
	_, err = db.Exec(`PRAGMA wal_checkpoint(TRUNCATE)`)
	return err
}

func placesFromTags(tx *sql.Tx) error {
	rows, err := tx.Query(`SELECT id,
		json_extract(exif_json, '$.tags.GPSLatitude'), json_extract(exif_json, '$.tags.GPSLatitudeRef'),
		json_extract(exif_json, '$.tags.GPSLongitude'), json_extract(exif_json, '$.tags.GPSLongitudeRef')
		FROM photos WHERE lat IS NULL AND json_valid(exif_json) AND json_extract(exif_json, '$.tags.GPSLatitude') IS NOT NULL`)
	if err != nil {
		return err
	}
	type place struct {
		id       string
		lat, lon float64
	}
	var found []place
	err = scanRows(rows, func() error {
		var id string
		var latTag, latRef, lonTag, lonRef sql.NullString
		if err := rows.Scan(&id, &latTag, &latRef, &lonTag, &lonRef); err != nil {
			return err
		}
		var p GeoPoint
		if coordsFromTags(&p, latTag.String, latRef.String, lonTag.String, lonRef.String) {
			found = append(found, place{id, p.Lat, p.Lon})
		}
		return nil
	})
	if err != nil {
		return err
	}
	for _, p := range found {
		if _, err := tx.Exec(`UPDATE photos SET lat = ?, lon = ? WHERE id = ?`, p.lat, p.lon, p.id); err != nil {
			return err
		}
	}
	return nil
}
