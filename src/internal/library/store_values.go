package library

import (
	"database/sql"
	"strings"
)

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
// Internal bookkeeping keys (built:*:account, built:*:postid, and their legacy
// published:*:account, published:*:postid equivalents) are excluded.
func (s *Store) GetMetaKeys() ([]string, error) {
	rows, err := s.db.Query(`
		SELECT DISTINCT key FROM photo_meta
		WHERE key NOT LIKE 'built:%:account'
		  AND key NOT LIKE 'built:%:postid'
		  AND key NOT LIKE 'published:%:account'
		  AND key NOT LIKE 'published:%:postid'
		  AND key NOT LIKE 'pending:%:%'
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

// GetAlbumTitles returns distinct gallery/album titles from all built:*:title (and
// legacy published:*:title) meta entries.
func (s *Store) GetAlbumTitles() ([]string, error) {
	rows, err := s.db.Query(`
		SELECT DISTINCT value FROM photo_meta
		WHERE (key LIKE 'built:%:title' OR key LIKE 'published:%:title') AND value != ''
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
