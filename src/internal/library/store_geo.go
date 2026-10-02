package library

import (
	"database/sql"
	"math"
	"strings"
)

// GeoPoint is one photo with a known location.
type GeoPoint struct {
	ID       string
	Filename string
	Lat      float64
	Lon      float64
	Taken    string // ISO date taken; empty when the photo has none
}

// GeoPoints returns every indexed photo that has a usable location.
// The parsed coordinates written at index time are preferred; libraries
// indexed before they existed fall back to the raw GPS tags.
func (s *Store) GeoPoints() ([]GeoPoint, error) {
	fw, fa := s.filter.Cond("id")
	rows, err := s.db.Query(`
		SELECT id, filename, COALESCE(date_taken, ''),
		       json_extract(exif_json, '$.latitude'),
		       json_extract(exif_json, '$.longitude'),
		       json_extract(exif_json, '$.tags.GPSLatitude'),
		       json_extract(exif_json, '$.tags.GPSLatitudeRef'),
		       json_extract(exif_json, '$.tags.GPSLongitude'),
		       json_extract(exif_json, '$.tags.GPSLongitudeRef')
		  FROM photos
		 WHERE status='ok' AND exif_json IS NOT NULL
		   AND (json_extract(exif_json, '$.latitude') IS NOT NULL
		        OR json_extract(exif_json, '$.tags.GPSLatitude') IS NOT NULL)`+fw, fa...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	points := []GeoPoint{}
	for rows.Next() {
		var p GeoPoint
		var lat, lon sql.NullFloat64
		var latTag, latRef, lonTag, lonRef sql.NullString
		if err := rows.Scan(&p.ID, &p.Filename, &p.Taken, &lat, &lon, &latTag, &latRef, &lonTag, &lonRef); err != nil {
			return nil, err
		}
		if lat.Valid && lon.Valid {
			p.Lat, p.Lon = lat.Float64, lon.Float64
		} else if !coordsFromTags(&p, latTag.String, latRef.String, lonTag.String, lonRef.String) {
			continue
		}
		if usableLocation(p.Lat, p.Lon) {
			points = append(points, p)
		}
	}
	return points, rows.Err()
}

func coordsFromTags(p *GeoPoint, latTag, latRef, lonTag, lonRef string) bool {
	lat, ok := ParseGPSCoord(latTag, latRef)
	if !ok {
		return false
	}
	lon, ok := ParseGPSCoord(lonTag, lonRef)
	if !ok {
		return false
	}
	p.Lat, p.Lon = lat, lon
	return true
}

// usableLocation rejects coordinates outside the globe and the 0,0 that
// cameras write when they had no fix.
func usableLocation(lat, lon float64) bool {
	if math.IsNaN(lat) || math.IsNaN(lon) || math.Abs(lat) > 90 || math.Abs(lon) > 180 {
		return false
	}
	return lat != 0 || lon != 0
}

// LocatedPhotos returns the photos within pathPrefix (or the whole library
// when it is empty) that have a usable location and a date, for the Space
// and time statistics. Unlike GeoPoints it reads the GPS tags from
// exif_index and the photos from their covering index, never a photo's row
// with its EXIF: 0.17 s against 0.33 s on 32,000 photos (ADR-0045). Every
// photo whose parsed location GeoPoints prefers has these tags as well.
// Filename stays empty.
func (s *Store) LocatedPhotos(pathPrefix string) ([]GeoPoint, error) {
	pathGlob := ""
	if pathPrefix != "" {
		pathGlob = escapeLikePattern(pathPrefix) + "/%"
	}
	where, args := s.aliasCond(pathGlob)
	rows, err := s.db.Query(`
		SELECT p.id, p.date_taken, la.value, COALESCE(lar.value, ''), lo.value, COALESCE(lor.value, '')
		FROM photos p
		CROSS JOIN exif_index la ON la.photo_id = p.id AND la.field = 'GPSLatitude'
		CROSS JOIN exif_index lo ON lo.photo_id = p.id AND lo.field = 'GPSLongitude'
		LEFT JOIN exif_index lar ON lar.photo_id = p.id AND lar.field = 'GPSLatitudeRef'
		LEFT JOIN exif_index lor ON lor.photo_id = p.id AND lor.field = 'GPSLongitudeRef'
		WHERE p.status='ok' AND p.date_taken IS NOT NULL`+where, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	points := []GeoPoint{}
	unquote := func(s string) string { return strings.ReplaceAll(s, `"`, "") }
	err = scanRows(rows, func() error {
		var p GeoPoint
		var lat, latRef, lon, lonRef string
		if err := rows.Scan(&p.ID, &p.Taken, &lat, &latRef, &lon, &lonRef); err != nil {
			return err
		}
		if coordsFromTags(&p, unquote(lat), unquote(latRef), unquote(lon), unquote(lonRef)) && usableLocation(p.Lat, p.Lon) {
			points = append(points, p)
		}
		return nil
	})
	return points, err
}
