package library

// Statistics returns aggregated statistics for photos with status='ok' in this library.
// pathPrefix, when non-empty, restricts results to photos whose path_hint starts with that prefix.
func (s *Store) Statistics(pathPrefix string) (*LibraryStatistics, error) {
	st := &LibraryStatistics{
		Formats:        []NameCount{},
		FilmSims:       []NameCount{},
		FocalLengths:   []ValueCount{},
		FocalLengths35: []ValueCount{},
		Apertures:      []ValueCount{},
		ISOs:           []ValueCount{},
		CameraLens:     []CameraLensCount{},
		ShootingDays:   make(map[string]int),
	}

	// pathGlob is the LIKE pattern used on path_hint; empty means no path filter.
	pathGlob := ""
	if pathPrefix != "" {
		pathGlob = escapeLikePattern(pathPrefix) + "/%"
	}

	// photosCond is appended to queries directly on the photos table.
	photosCond := func() (string, []any) {
		if pathGlob == "" {
			return "", nil
		}
		return " AND path_hint LIKE ? ESCAPE '\\'", []any{pathGlob}
	}

	// exifJoin is an extra JOIN clause for queries that only touch exif_index.
	exifJoin := func() (string, string, []any) {
		if pathGlob == "" {
			return "", "", nil
		}
		return "JOIN photos _ph ON _ph.id = e.photo_id AND _ph.status='ok' AND _ph.path_hint LIKE ? ESCAPE '\\'",
			" AND e.photo_id IN (SELECT id FROM photos WHERE status='ok' AND path_hint LIKE ? ESCAPE '\\')",
			[]any{pathGlob}
	}

	// Total count (indexed) and in-progress count (still being scanned).
	pcWhere, pcArgs := photosCond()
	if err := s.db.QueryRow(`SELECT COUNT(*) FROM photos WHERE status='ok'`+pcWhere,
		pcArgs...).Scan(&st.TotalPhotos); err != nil {
		return nil, err
	}
	if err := s.db.QueryRow(`SELECT COUNT(*) FROM photos WHERE status='missing'`+pcWhere,
		pcArgs...).Scan(&st.IndexingPhotos); err != nil {
		return nil, err
	}

	// Format distribution: use the pre-computed ext column — O(distinct formats) instead of O(n).
	{
		frows, ferr := s.db.Query(`SELECT ext, COUNT(*) AS n FROM photos WHERE status='ok' AND ext != ''`+pcWhere+` GROUP BY ext ORDER BY n DESC`, pcArgs...)
		if ferr != nil {
			return nil, ferr
		}
		defer frows.Close()
		for frows.Next() {
			var nc NameCount
			if err := frows.Scan(&nc.Name, &nc.Count); err != nil {
				return nil, err
			}
			st.Formats = append(st.Formats, nc)
		}
		frows.Close()
	}

	_, exifPathCond, exifPathArgs := exifJoin()

	// Film simulation distribution.
	{
		rows, err := s.db.Query(`
			SELECT value, COUNT(*) AS n FROM exif_index e WHERE e.field='FilmSimulation'`+
			exifPathCond+` GROUP BY value ORDER BY n DESC`, exifPathArgs...)
		if err != nil {
			return nil, err
		}
		defer rows.Close()
		for rows.Next() {
			var nc NameCount
			if err := rows.Scan(&nc.Name, &nc.Count); err != nil {
				return nil, err
			}
			st.FilmSims = append(st.FilmSims, nc)
		}
		rows.Close()
	}
	// Photos without any film simulation tag.
	{
		simTotal := 0
		for _, nc := range st.FilmSims {
			simTotal += nc.Count
		}
		if noneCount := st.TotalPhotos - simTotal; noneCount > 0 {
			st.FilmSims = append(st.FilmSims, NameCount{Name: "None", Count: noneCount})
		}
	}

	// Focal lengths (native mm) — deduplicated by distinct value.
	{
		rows, err := s.db.Query(`
			SELECT e.numeric_value, COUNT(*) AS n
			FROM exif_index e WHERE e.field='FocalLength' AND e.numeric_value IS NOT NULL`+
			exifPathCond+`
			GROUP BY e.numeric_value ORDER BY e.numeric_value`, exifPathArgs...)
		if err != nil {
			return nil, err
		}
		defer rows.Close()
		for rows.Next() {
			var vc ValueCount
			if err := rows.Scan(&vc.Value, &vc.Count); err != nil {
				return nil, err
			}
			st.FocalLengths = append(st.FocalLengths, vc)
		}
		rows.Close()
	}

	// Focal lengths (35mm equivalent): prefer FocalLengthIn35mmFilm, fall back to FocalLength per photo.
	// Deduplicated by distinct value via subquery.
	{
		pathCondStr, pathCondArgs := photosCond()
		rows, err := s.db.Query(`
			SELECT v, COUNT(*) AS n FROM (
				SELECT COALESCE(fl35.numeric_value, fl.numeric_value) AS v
				FROM   photos p
				LEFT JOIN exif_index fl   ON fl.photo_id  = p.id AND fl.field   = 'FocalLength'           AND fl.numeric_value   IS NOT NULL
				LEFT JOIN exif_index fl35 ON fl35.photo_id = p.id AND fl35.field = 'FocalLengthIn35mmFilm' AND fl35.numeric_value IS NOT NULL
				WHERE  p.status = 'ok'
				  AND  COALESCE(fl35.numeric_value, fl.numeric_value) IS NOT NULL`+pathCondStr+`
			) GROUP BY v ORDER BY v`, pathCondArgs...)
		if err != nil {
			return nil, err
		}
		defer rows.Close()
		for rows.Next() {
			var vc ValueCount
			if err := rows.Scan(&vc.Value, &vc.Count); err != nil {
				return nil, err
			}
			st.FocalLengths35 = append(st.FocalLengths35, vc)
		}
		rows.Close()
	}

	// Apertures (FNumber) — deduplicated by distinct value.
	{
		rows, err := s.db.Query(`
			SELECT e.numeric_value, COUNT(*) AS n
			FROM exif_index e WHERE e.field='FNumber' AND e.numeric_value IS NOT NULL`+
			exifPathCond+`
			GROUP BY e.numeric_value ORDER BY e.numeric_value`, exifPathArgs...)
		if err != nil {
			return nil, err
		}
		defer rows.Close()
		for rows.Next() {
			var vc ValueCount
			if err := rows.Scan(&vc.Value, &vc.Count); err != nil {
				return nil, err
			}
			st.Apertures = append(st.Apertures, vc)
		}
		rows.Close()
	}

	// ISOs — deduplicated by distinct value.
	{
		rows, err := s.db.Query(`
			SELECT e.numeric_value, COUNT(*) AS n
			FROM exif_index e WHERE e.field='ISOSpeedRatings' AND e.numeric_value IS NOT NULL`+
			exifPathCond+`
			GROUP BY e.numeric_value ORDER BY e.numeric_value`, exifPathArgs...)
		if err != nil {
			return nil, err
		}
		defer rows.Close()
		for rows.Next() {
			var vc ValueCount
			if err := rows.Scan(&vc.Value, &vc.Count); err != nil {
				return nil, err
			}
			st.ISOs = append(st.ISOs, vc)
		}
		rows.Close()
	}

	// Camera × lens combinations. LEFT JOIN on LensModel so cameras without a lens
	// tag (smartphones, film scanners) still appear under "(no lens)".
	{
		cameraJoin := " JOIN photos _ph ON _ph.id = c.photo_id AND _ph.status='ok'"
		var cameraArgs []any
		if pathGlob != "" {
			cameraJoin += " AND _ph.path_hint LIKE ? ESCAPE '\\'"
			cameraArgs = []any{pathGlob}
		}
		rows, err := s.db.Query(`
			SELECT c.value AS camera, COALESCE(l.value, '(no lens)') AS lens, COUNT(*) AS n
			FROM   exif_index c`+cameraJoin+`
			LEFT JOIN exif_index l ON c.photo_id = l.photo_id AND l.field = 'LensModel'
			WHERE  c.field = 'Model'
			GROUP BY camera, lens ORDER BY n DESC LIMIT 100`, cameraArgs...)
		if err != nil {
			return nil, err
		}
		defer rows.Close()
		for rows.Next() {
			var clc CameraLensCount
			if err := rows.Scan(&clc.Camera, &clc.Lens, &clc.Count); err != nil {
				return nil, err
			}
			st.CameraLens = append(st.CameraLens, clc)
		}
		rows.Close()
	}

	// Shooting hours distribution.
	{
		rows, err := s.db.Query(`
			SELECT CAST(SUBSTR(date_taken, 12, 2) AS INTEGER) AS hr, COUNT(*) AS n
			FROM   photos
			WHERE  status='ok'
			  AND  date_taken IS NOT NULL
			  AND  LENGTH(date_taken) >= 13`+pcWhere+
			` GROUP BY hr`, pcArgs...)
		if err != nil {
			return nil, err
		}
		defer rows.Close()
		for rows.Next() {
			var hr, n int
			if err := rows.Scan(&hr, &n); err != nil {
				return nil, err
			}
			if hr >= 0 && hr < 24 {
				st.ShootingHours[hr] = n
			}
		}
		rows.Close()
	}

	// Shooting days distribution (calendar heatmap).
	{
		rows, err := s.db.Query(`
			SELECT SUBSTR(date_taken, 1, 10) AS day, COUNT(*) AS n
			FROM   photos
			WHERE  status='ok' AND date_taken IS NOT NULL`+pcWhere+
			` GROUP BY day`, pcArgs...)
		if err != nil {
			return nil, err
		}
		defer rows.Close()
		for rows.Next() {
			var day string
			var n int
			if err := rows.Scan(&day, &n); err != nil {
				return nil, err
			}
			st.ShootingDays[day] = n
		}
		rows.Close()
	}

	return st, nil
}

func sortNameCounts(s []NameCount) {
	for i := 1; i < len(s); i++ {
		for j := i; j > 0 && s[j].Count > s[j-1].Count; j-- {
			s[j], s[j-1] = s[j-1], s[j]
		}
	}
}

func sortStrings(s []string) {
	for i := 1; i < len(s); i++ {
		for j := i; j > 0 && s[j] < s[j-1]; j-- {
			s[j], s[j-1] = s[j-1], s[j]
		}
	}
}

// FolderStats returns DB-backed statistics for all indexed photos under folderAbs.
// Results are derived entirely from the photos table and exif_index; no filesystem access.
func (s *Store) FolderStats(folderAbs string) (*LibraryFolderStats, error) {
	prefix := folderAbs + "/"
	glob := prefix + "*"

	// Summary: total photo count, size, and date range.
	var dateFirst, dateLast *string
	st := &LibraryFolderStats{Formats: []NameCount{}, Subfolders: []LibSubfolder{}}
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

	// Format distribution by file extension.
	{
		rows, err := s.db.Query(
			`SELECT ext, COUNT(*) AS n FROM photos
			 WHERE status='ok' AND ext != '' AND path_hint GLOB ?
			 GROUP BY ext ORDER BY n DESC`,
			glob,
		)
		if err != nil {
			return nil, err
		}
		defer rows.Close()
		for rows.Next() {
			var nc NameCount
			if err := rows.Scan(&nc.Name, &nc.Count); err != nil {
				return nil, err
			}
			st.Formats = append(st.Formats, nc)
		}
		rows.Close()
	}

	// Immediate subfolders: extract first path segment below prefix.
	// Reuses the GLOB+SUBSTR pattern from BrowseFolder.
	{
		sfRows, err := s.db.Query(
			`SELECT DISTINCT SUBSTR(path_hint, length(?)+1, INSTR(SUBSTR(path_hint, length(?)+1), '/')-1)
			 FROM photos WHERE status='ok' AND path_hint GLOB ?`,
			prefix, prefix, prefix+"*/*",
		)
		if err != nil {
			return nil, err
		}
		defer sfRows.Close()
		var names []string
		for sfRows.Next() {
			var name string
			if err := sfRows.Scan(&name); err != nil {
				return nil, err
			}
			if name != "" {
				names = append(names, name)
			}
		}
		sfRows.Close()
		sortStrings(names)

		for _, name := range names {
			var sub LibSubfolder
			sub.Name = name
			if err := s.db.QueryRow(
				`SELECT COUNT(*), COALESCE(SUM(file_size),0) FROM photos
				 WHERE status='ok' AND path_hint GLOB ?`,
				prefix+name+"/*",
			).Scan(&sub.PhotoCount, &sub.TotalSize); err != nil {
				return nil, err
			}
			st.Subfolders = append(st.Subfolders, sub)
		}
	}

	return st, nil
}
