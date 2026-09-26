package library

import (
	"database/sql"
	"fmt"
	"strings"
	"time"
)

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
	Filters        map[string]string        // EXIF text exact-match filters (field → value)
	NumericFilters map[string]NumericFilter // EXIF numeric range filters
	DateMin        string                   // YYYY-MM-DD lower bound on date_taken
	DateMax        string                   // YYYY-MM-DD upper bound on date_taken
	MetaFilters    map[string]string        // photo_meta key=value exact matches
	MetaExists     []string                 // photo_meta keys that must exist (any value)
	AlbumTitle     string                   // match photos with any built:*:title = value
	ExtFilter      string                   // file extension (photos.ext)
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
		// built: keys may not exist yet for photos indexed before the built: prefix
		// replaced published:; fall back to the legacy key so those still match.
		if legacyKey, ok := strings.CutPrefix(key, "built:"); ok {
			where = append(where, `EXISTS (SELECT 1 FROM photo_meta pm WHERE pm.photo_id=p.id AND (pm.key=? OR pm.key=?))`)
			whereArgs = append(whereArgs, key, "published:"+legacyKey)
		} else {
			where = append(where, `EXISTS (SELECT 1 FROM photo_meta pm WHERE pm.photo_id=p.id AND pm.key=?)`)
			whereArgs = append(whereArgs, key)
		}
	}
	if opts.AlbumTitle != "" {
		where = append(where, `EXISTS (SELECT 1 FROM photo_meta pm WHERE pm.photo_id=p.id AND (pm.key LIKE 'built:%:title' OR pm.key LIKE 'published:%:title') AND pm.value=?)`)
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
