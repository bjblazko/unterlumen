package library

import (
	"database/sql"
	"strconv"
)

// DatedPhoto is an indexed photo with a date taken, with what the timeline
// needs to lay it out (ADR-0040).
type DatedPhoto struct {
	ID          string
	Filename    string
	Taken       string // ISO date taken, as indexed
	Width       int    // stored size, before orientation; 0 when unknown
	Height      int
	Orientation string // EXIF Orientation tag; "" when absent
}

// DatedPhotos returns the photos that have a date taken, oldest first.
// Photos marked missing are left out.
func (s *Store) DatedPhotos() ([]DatedPhoto, error) {
	rows, err := s.db.Query(`
		SELECT id, filename, date_taken,
		       json_extract(exif_json, '$.width'),
		       json_extract(exif_json, '$.height'),
		       json_extract(exif_json, '$.tags.Orientation')
		  FROM photos
		 WHERE status='ok' AND date_taken IS NOT NULL
		 ORDER BY date_taken, id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	photos := []DatedPhoto{}
	for rows.Next() {
		var p DatedPhoto
		var w, h sql.NullInt64
		var o sql.NullString
		if err := rows.Scan(&p.ID, &p.Filename, &p.Taken, &w, &h, &o); err != nil {
			return nil, err
		}
		p.Width, p.Height, p.Orientation = int(w.Int64), int(h.Int64), o.String
		photos = append(photos, p)
	}
	return photos, rows.Err()
}

// UndatedPhotoIDs returns the IDs of the photos without a date taken.
func (s *Store) UndatedPhotoIDs() ([]string, error) {
	rows, err := s.db.Query(`SELECT id FROM photos WHERE status='ok' AND date_taken IS NULL ORDER BY id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	ids := []string{}
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		ids = append(ids, id)
	}
	return ids, rows.Err()
}

// ContentStamp changes whenever the library's photos do: it is the number of
// photos that are not missing and the latest time one was indexed.
func (s *Store) ContentStamp() (string, error) {
	var n int
	var latest string
	err := s.db.QueryRow(`SELECT COUNT(*), COALESCE(CAST(MAX(indexed_at) AS TEXT), '')
		FROM photos WHERE status='ok'`).Scan(&n, &latest)
	return strconv.Itoa(n) + "|" + latest, err
}
