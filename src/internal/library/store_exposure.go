package library

// ExposurePoint is one photo where the Exposure space draws it: its focal
// length (35 mm equivalent, or as taken when the camera wrote none),
// aperture and ISO, and the camera that took it.
type ExposurePoint struct {
	ID     string
	Date   string // date taken as stored, "" when unknown
	Camera string // the Model tag as stored, "" when missing
	Focal  float64
	FNum   float64
	ISO    float64
}

// ExposurePoints reads every photo within pathPrefix, or the whole library
// when it is empty, that has all three settings. The photos come first, from
// the covering index of status, date and id, and each setting is looked up
// by exif_index's (photo_id, field) key. CROSS JOIN keeps that order: left
// to itself, SQLite started from the apertures and read each photo's whole
// row by id, EXIF included: 3 s instead of 0.5 s on 32,000 photos (ADR-0045).
func (s *Store) ExposurePoints(pathPrefix string) ([]ExposurePoint, error) {
	pathGlob := ""
	if pathPrefix != "" {
		pathGlob = escapeLikePattern(pathPrefix) + "/%"
	}
	where, args := tlAliasCond(pathGlob)
	rows, err := s.db.Query(`
		SELECT p.id, COALESCE(p.date_taken, ''), COALESCE(m.value, ''),
		       COALESCE(fl35.numeric_value, fl.numeric_value), fn.numeric_value, iso.numeric_value
		FROM photos p
		CROSS JOIN exif_index fn  ON fn.photo_id  = p.id AND fn.field  = 'FNumber'         AND fn.numeric_value  > 0
		CROSS JOIN exif_index iso ON iso.photo_id = p.id AND iso.field = 'ISOSpeedRatings' AND iso.numeric_value > 0
		LEFT JOIN exif_index fl   ON fl.photo_id   = p.id AND fl.field   = 'FocalLength'           AND fl.numeric_value   > 0
		LEFT JOIN exif_index fl35 ON fl35.photo_id = p.id AND fl35.field = 'FocalLengthIn35mmFilm' AND fl35.numeric_value > 0
		LEFT JOIN exif_index m    ON m.photo_id    = p.id AND m.field    = 'Model'
		WHERE p.status='ok' AND COALESCE(fl35.numeric_value, fl.numeric_value) IS NOT NULL`+where, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []ExposurePoint{}
	err = scanRows(rows, func() error {
		var e ExposurePoint
		if err := rows.Scan(&e.ID, &e.Date, &e.Camera, &e.Focal, &e.FNum, &e.ISO); err != nil {
			return err
		}
		out = append(out, e)
		return nil
	})
	return out, err
}
