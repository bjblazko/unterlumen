package library

import (
	"database/sql"
	"sort"
)

// statsScope is the path restriction of a statistics query, as SQL fragments
// for each kind of query. Empty fragments mean the whole library.
type statsScope struct {
	photos string // appended to a WHERE on the photos table
	exif   string // appended to a WHERE on exif_index aliased e
	camera string // appended to a WHERE on exif_index aliased c: its ok photos
	args   []any  // the one argument each non-empty fragment takes
}

func newStatsScope(pathPrefix string) statsScope {
	// "In the set of ok photos" rather than a join on photos: the set comes
	// from a covering index once, while a join read each photo's row (and its
	// EXIF JSON) to check its status — 2 s of 2.5 on 36,000 photos.
	sc := statsScope{camera: " AND c.photo_id IN (SELECT id FROM photos WHERE status='ok')"}
	if pathPrefix == "" {
		return sc
	}
	sc.photos = " AND path_hint LIKE ? ESCAPE '\\'"
	sc.exif = " AND e.photo_id IN (SELECT id FROM photos WHERE status='ok' AND path_hint LIKE ? ESCAPE '\\')"
	sc.camera = " AND c.photo_id IN (SELECT id FROM photos WHERE status='ok' AND path_hint LIKE ? ESCAPE '\\')"
	sc.args = []any{escapeLikePattern(pathPrefix) + "/%"}
	return sc
}

// Statistics returns aggregated statistics for photos with status='ok' in this library.
// pathPrefix, when non-empty, restricts results to photos whose path_hint starts with that prefix.
func (s *Store) Statistics(pathPrefix string) (*LibraryStatistics, error) {
	sc := newStatsScope(pathPrefix)
	st := &LibraryStatistics{}
	var err error
	steps := []func() error{
		func() error { return s.photoCounts(sc, st) },
		// Use the pre-computed ext column — O(distinct formats) instead of O(n).
		func() error {
			st.Formats, err = s.queryNameCounts(`SELECT ext, COUNT(*) AS n FROM photos WHERE status='ok' AND ext != ''`+sc.photos+` GROUP BY ext ORDER BY n DESC`, sc.args...)
			return err
		},
		func() error {
			st.FilmSims, err = s.filmSimCounts(sc, st.TotalPhotos)
			return err
		},
		func() error {
			st.FocalLengths, err = s.numericFieldCounts(sc, "FocalLength")
			return err
		},
		func() error {
			st.FocalLengths35, err = s.focalLength35Counts(sc)
			return err
		},
		func() error {
			st.Apertures, err = s.numericFieldCounts(sc, "FNumber")
			return err
		},
		func() error {
			st.ISOs, err = s.numericFieldCounts(sc, "ISOSpeedRatings")
			return err
		},
		func() error {
			st.CameraLens, err = s.cameraLensCounts(sc)
			return err
		},
		func() error {
			st.ShootingHours, err = s.shootingHours(sc)
			return err
		},
		func() error {
			st.ShootingDays, err = s.shootingDays(sc)
			return err
		},
	}
	for _, step := range steps {
		if err := step(); err != nil {
			return nil, err
		}
	}
	return st, nil
}

// photoCounts sets the indexed total and the in-progress count (photos still
// being scanned).
func (s *Store) photoCounts(sc statsScope, st *LibraryStatistics) error {
	if err := s.db.QueryRow(`SELECT COUNT(*) FROM photos WHERE status='ok'`+sc.photos, sc.args...).Scan(&st.TotalPhotos); err != nil {
		return err
	}
	return s.db.QueryRow(`SELECT COUNT(*) FROM photos WHERE status='missing'`+sc.photos, sc.args...).Scan(&st.IndexingPhotos)
}

// filmSimCounts is the film simulation distribution, with the photos that
// carry no film simulation tag as "None".
func (s *Store) filmSimCounts(sc statsScope, totalPhotos int) ([]NameCount, error) {
	sims, err := s.queryNameCounts(`
		SELECT value, COUNT(*) AS n FROM exif_index e WHERE e.field='FilmSimulation'`+
		sc.exif+` GROUP BY value ORDER BY n DESC`, sc.args...)
	if err != nil {
		return nil, err
	}
	simTotal := 0
	for _, nc := range sims {
		simTotal += nc.Count
	}
	if noneCount := totalPhotos - simTotal; noneCount > 0 {
		sims = append(sims, NameCount{Name: "None", Count: noneCount})
	}
	return sims, nil
}

// numericFieldCounts counts photos per distinct numeric value of one EXIF
// field, in ascending value order.
func (s *Store) numericFieldCounts(sc statsScope, field string) ([]ValueCount, error) {
	return s.queryValueCounts(`
		SELECT e.numeric_value, COUNT(*) AS n
		FROM exif_index e WHERE e.field=? AND e.numeric_value IS NOT NULL`+
		sc.exif+`
		GROUP BY e.numeric_value ORDER BY e.numeric_value`, append([]any{field}, sc.args...)...)
}

// focalLength35Counts counts 35mm-equivalent focal lengths: FocalLengthIn35mmFilm,
// falling back to FocalLength per photo. Deduplicated by distinct value via subquery.
func (s *Store) focalLength35Counts(sc statsScope) ([]ValueCount, error) {
	return s.queryValueCounts(`
		SELECT v, COUNT(*) AS n FROM (
			SELECT COALESCE(fl35.numeric_value, fl.numeric_value) AS v
			FROM   photos p
			LEFT JOIN exif_index fl   ON fl.photo_id  = p.id AND fl.field   = 'FocalLength'           AND fl.numeric_value   IS NOT NULL
			LEFT JOIN exif_index fl35 ON fl35.photo_id = p.id AND fl35.field = 'FocalLengthIn35mmFilm' AND fl35.numeric_value IS NOT NULL
			WHERE  p.status = 'ok'
			  AND  COALESCE(fl35.numeric_value, fl.numeric_value) IS NOT NULL`+sc.photos+`
		) GROUP BY v ORDER BY v`, sc.args...)
}

// cameraLensCounts counts camera × lens combinations, most used first, at most
// 100. LEFT JOIN on LensModel so cameras without a lens tag (smartphones, film
// scanners) still appear under "(no lens)".
func (s *Store) cameraLensCounts(sc statsScope) ([]CameraLensCount, error) {
	rows, err := s.db.Query(`
		SELECT c.value AS camera, COALESCE(l.value, '(no lens)') AS lens, COUNT(*) AS n
		FROM   exif_index c
		LEFT JOIN exif_index l ON c.photo_id = l.photo_id AND l.field = 'LensModel'
		WHERE  c.field = 'Model'`+sc.camera+`
		GROUP BY camera, lens ORDER BY n DESC LIMIT 100`, sc.args...)
	if err != nil {
		return nil, err
	}
	out := []CameraLensCount{}
	err = scanRows(rows, func() error {
		var clc CameraLensCount
		if err := rows.Scan(&clc.Camera, &clc.Lens, &clc.Count); err != nil {
			return err
		}
		out = append(out, clc)
		return nil
	})
	return out, err
}

// shootingHours counts photos per hour of the day they were taken.
func (s *Store) shootingHours(sc statsScope) ([24]int, error) {
	var hours [24]int
	rows, err := s.db.Query(`
		SELECT CAST(SUBSTR(date_taken, 12, 2) AS INTEGER) AS hr, COUNT(*) AS n
		FROM   photos
		WHERE  status='ok'
		  AND  date_taken IS NOT NULL
		  AND  LENGTH(date_taken) >= 13`+sc.photos+
		` GROUP BY hr`, sc.args...)
	if err != nil {
		return hours, err
	}
	err = scanRows(rows, func() error {
		var hr, n int
		if err := rows.Scan(&hr, &n); err != nil {
			return err
		}
		if hr >= 0 && hr < 24 {
			hours[hr] = n
		}
		return nil
	})
	return hours, err
}

// shootingDays counts photos per day taken, for the calendar heatmap.
func (s *Store) shootingDays(sc statsScope) (map[string]int, error) {
	days := make(map[string]int)
	rows, err := s.db.Query(`
		SELECT SUBSTR(date_taken, 1, 10) AS day, COUNT(*) AS n
		FROM   photos
		WHERE  status='ok' AND date_taken IS NOT NULL`+sc.photos+
		` GROUP BY day`, sc.args...)
	if err != nil {
		return nil, err
	}
	err = scanRows(rows, func() error {
		var day string
		var n int
		if err := rows.Scan(&day, &n); err != nil {
			return err
		}
		days[day] = n
		return nil
	})
	return days, err
}

// queryNameCounts returns the (name, count) rows of a query, never nil.
func (s *Store) queryNameCounts(query string, args ...any) ([]NameCount, error) {
	rows, err := s.db.Query(query, args...)
	if err != nil {
		return nil, err
	}
	out := []NameCount{}
	err = scanRows(rows, func() error {
		var nc NameCount
		if err := rows.Scan(&nc.Name, &nc.Count); err != nil {
			return err
		}
		out = append(out, nc)
		return nil
	})
	return out, err
}

// queryValueCounts returns the (value, count) rows of a query, never nil.
func (s *Store) queryValueCounts(query string, args ...any) ([]ValueCount, error) {
	rows, err := s.db.Query(query, args...)
	if err != nil {
		return nil, err
	}
	out := []ValueCount{}
	err = scanRows(rows, func() error {
		var vc ValueCount
		if err := rows.Scan(&vc.Value, &vc.Count); err != nil {
			return err
		}
		out = append(out, vc)
		return nil
	})
	return out, err
}

// scanRows calls scan for every row, then closes rows and reports the first
// scan error or the error that ended the iteration.
func scanRows(rows *sql.Rows, scan func() error) error {
	defer rows.Close()
	for rows.Next() {
		if err := scan(); err != nil {
			return err
		}
	}
	return rows.Err()
}

// sortNameCounts orders by count, largest first, keeping the order of ties.
func sortNameCounts(s []NameCount) {
	sort.SliceStable(s, func(i, j int) bool { return s[i].Count > s[j].Count })
}

func sortStrings(s []string) { sort.Strings(s) }

// FolderStats returns DB-backed statistics for all indexed photos under folderAbs.
// Results are derived entirely from the photos table and exif_index; no filesystem access.
func (s *Store) FolderStats(folderAbs string) (*LibraryFolderStats, error) {
	prefix := folderAbs + "/"
	glob := prefix + "*"

	// Summary: total photo count, size, and date range.
	var dateFirst, dateLast *string
	st := &LibraryFolderStats{}
	if err := s.db.QueryRow(
		`SELECT COUNT(*), COALESCE(SUM(file_size),0), MIN(date_taken), MAX(date_taken)
		 FROM photos WHERE status='ok' AND path_hint GLOB ?`,
		glob,
	).Scan(&st.PhotoCount, &st.TotalSize, &dateFirst, &dateLast); err != nil {
		return nil, err
	}
	if dateFirst != nil {
		st.DateFirst = *dateFirst
	}
	if dateLast != nil {
		st.DateLast = *dateLast
	}

	var err error
	if st.Formats, err = s.queryNameCounts(
		`SELECT ext, COUNT(*) AS n FROM photos
		 WHERE status='ok' AND ext != '' AND path_hint GLOB ?
		 GROUP BY ext ORDER BY n DESC`,
		glob,
	); err != nil {
		return nil, err
	}
	if st.Subfolders, err = s.subfolderStats(prefix); err != nil {
		return nil, err
	}
	return st, nil
}

// subfolderStats counts photos and bytes in each immediate subfolder below
// prefix, in name order.
func (s *Store) subfolderStats(prefix string) ([]LibSubfolder, error) {
	names, err := s.subfolderNames(prefix)
	if err != nil {
		return nil, err
	}
	subs := []LibSubfolder{}
	for _, name := range names {
		sub := LibSubfolder{Name: name}
		if err := s.db.QueryRow(
			`SELECT COUNT(*), COALESCE(SUM(file_size),0) FROM photos
			 WHERE status='ok' AND path_hint GLOB ?`,
			prefix+name+"/*",
		).Scan(&sub.PhotoCount, &sub.TotalSize); err != nil {
			return nil, err
		}
		subs = append(subs, sub)
	}
	return subs, nil
}

// subfolderNames returns the sorted names of the immediate subfolders below
// prefix that hold indexed photos: the first path segment below prefix of
// every nested photo. SUBSTR/INSTR in SQL avoids returning full rows; DISTINCT
// collapses duplicates.
func (s *Store) subfolderNames(prefix string) ([]string, error) {
	rows, err := s.db.Query(
		`SELECT DISTINCT SUBSTR(path_hint, length(?)+1, INSTR(SUBSTR(path_hint, length(?)+1), '/')-1)
		 FROM photos WHERE status='ok' AND path_hint GLOB ?`,
		prefix, prefix, prefix+"*/*",
	)
	if err != nil {
		return nil, err
	}
	names := []string{}
	err = scanRows(rows, func() error {
		var name string
		if err := rows.Scan(&name); err != nil {
			return err
		}
		if name != "" {
			names = append(names, name)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	sortStrings(names)
	return names, nil
}
