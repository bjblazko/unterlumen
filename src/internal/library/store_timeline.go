package library

import (
	"database/sql"
	"time"
)

// Timeline returns time-series statistics for the given path scope.
// granularity is "month", "year", or "" (auto-detect from date span).
func (s *Store) Timeline(pathPrefix, granularity string) (*LibraryTimeline, error) {
	pathGlob := ""
	if pathPrefix != "" {
		pathGlob = escapeLikePattern(pathPrefix) + "/%"
	}
	pcWhere, pcArgs := s.photoCond(pathGlob)
	pWhere, pArgs := s.aliasCond(pathGlob)

	if granularity != "month" && granularity != "year" {
		granularity = tlDetectGranularity(s.db, pcWhere, pcArgs)
	}
	N := 7
	if granularity == "year" {
		N = 4
	}

	cameraRows, err := tlCameraRows(s.db, N, pWhere, pArgs)
	if err != nil {
		return nil, err
	}
	focalVals, err := tlOrderedFloats(s.db, "FocalLengthIn35mmFilm", N, pWhere, pArgs)
	if err != nil {
		return nil, err
	}
	isoVals, err := tlOrderedFloats(s.db, "ISOSpeedRatings", N, pWhere, pArgs)
	if err != nil {
		return nil, err
	}
	aperMap, err := tlApertureMap(s.db, N, pWhere, pArgs)
	if err != nil {
		return nil, err
	}
	aspectMap, err := tlAspectMap(s.db, N, pcWhere, pcArgs)
	if err != nil {
		return nil, err
	}
	mpStats, err := tlMegapixels(s.db, N, pcWhere, pcArgs)
	if err != nil {
		return nil, err
	}

	return assembleTL(granularity, cameraRows, focalVals, isoVals, aperMap, aspectMap, mpStats), nil
}

// photoCond restricts the photos table, unaliased, to the folder and the
// store's filter (ADR-0050).
func (s *Store) photoCond(pathGlob string) (string, []any) {
	where, args := "", []any(nil)
	if pathGlob != "" {
		where, args = " AND path_hint LIKE ? ESCAPE '\\'", []any{pathGlob}
	}
	fw, fa := s.filter.Cond("id")
	return where + fw, append(args, fa...)
}

// aliasCond does the same for photos aliased p.
func (s *Store) aliasCond(pathGlob string) (string, []any) {
	where, args := "", []any(nil)
	if pathGlob != "" {
		where, args = " AND p.path_hint LIKE ? ESCAPE '\\'", []any{pathGlob}
	}
	fw, fa := s.filter.Cond("p.id")
	return where + fw, append(args, fa...)
}

func tlDetectGranularity(db *sql.DB, pcWhere string, pcArgs []any) string {
	var minP, maxP sql.NullString
	db.QueryRow(`
		SELECT MIN(SUBSTR(date_taken,1,7)),
		       MAX(SUBSTR(date_taken,1,7))
		FROM photos WHERE status='ok'
		  AND date_taken IS NOT NULL`+pcWhere,
		pcArgs...).Scan(&minP, &maxP)
	if !minP.Valid || !maxP.Valid || len(minP.String) < 7 || len(maxP.String) < 7 {
		return "month"
	}
	return granularityForSpan(minP.String, maxP.String)
}

// granularityForSpan counts in years when the first and last month ("2006-01")
// lie more than four years apart, otherwise in months.
func granularityForSpan(first, last string) string {
	minT, err1 := time.Parse("2006-01", first)
	maxT, err2 := time.Parse("2006-01", last)
	if err1 != nil || err2 != nil {
		return "month"
	}
	spanMonths := (maxT.Year()-minT.Year())*12 + int(maxT.Month()-minT.Month())
	if spanMonths > 48 {
		return "year"
	}
	return "month"
}

type tlCameraRow struct {
	period string
	camera string
	count  int
}

func tlCameraRows(db *sql.DB, N int, pWhere string, pArgs []any) ([]tlCameraRow, error) {
	rows, err := db.Query(`
		SELECT SUBSTR(p.date_taken,1,?), e.value, COUNT(*)
		FROM photos p
		JOIN exif_index e ON e.photo_id = p.id AND e.field = 'Model'
		WHERE p.status='ok'
		  AND p.date_taken IS NOT NULL`+pWhere+`
		GROUP BY 1, 2 ORDER BY 1, 3 DESC`,
		append([]any{N}, pArgs...)...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []tlCameraRow
	for rows.Next() {
		var r tlCameraRow
		if err := rows.Scan(&r.period, &r.camera, &r.count); err != nil {
			return nil, err
		}
		r.camera = stripExifQuotes(r.camera)
		out = append(out, r)
	}
	return out, nil
}

func tlOrderedFloats(db *sql.DB, field string, N int, pWhere string, pArgs []any) (map[string][]float64, error) {
	rows, err := db.Query(`
		SELECT SUBSTR(p.date_taken,1,?), e.numeric_value
		FROM photos p
		JOIN exif_index e ON e.photo_id = p.id AND e.field = ?
		WHERE p.status='ok'
		  AND p.date_taken IS NOT NULL
		  AND e.numeric_value IS NOT NULL`+pWhere+`
		ORDER BY 1, 2`,
		append([]any{N, field}, pArgs...)...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make(map[string][]float64)
	for rows.Next() {
		var period string
		var val float64
		if err := rows.Scan(&period, &val); err != nil {
			return nil, err
		}
		out[period] = append(out[period], val)
	}
	return out, nil
}

func tlApertureMap(db *sql.DB, N int, pWhere string, pArgs []any) (map[string]map[string]int, error) {
	rows, err := db.Query(`
		SELECT SUBSTR(p.date_taken,1,?),
		       CASE
		         WHEN e.numeric_value <= 1.2  THEN 'f/1'
		         WHEN e.numeric_value <= 1.6  THEN 'f/1.4'
		         WHEN e.numeric_value <= 2.3  THEN 'f/2'
		         WHEN e.numeric_value <= 3.3  THEN 'f/2.8'
		         WHEN e.numeric_value <= 4.7  THEN 'f/4'
		         WHEN e.numeric_value <= 6.5  THEN 'f/5.6'
		         WHEN e.numeric_value <= 9.5  THEN 'f/8'
		         WHEN e.numeric_value <= 13.0 THEN 'f/11'
		         ELSE 'f/16+'
		       END,
		       COUNT(*)
		FROM photos p
		JOIN exif_index e ON e.photo_id = p.id AND e.field = 'FNumber'
		WHERE p.status='ok'
		  AND p.date_taken IS NOT NULL
		  AND e.numeric_value IS NOT NULL`+pWhere+`
		GROUP BY 1, 2 ORDER BY 1`,
		append([]any{N}, pArgs...)...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make(map[string]map[string]int)
	for rows.Next() {
		var period, bucket string
		var count int
		if err := rows.Scan(&period, &bucket, &count); err != nil {
			return nil, err
		}
		if out[period] == nil {
			out[period] = make(map[string]int)
		}
		out[period][bucket] = count
	}
	return out, nil
}

func tlAspectMap(db *sql.DB, N int, pcWhere string, pcArgs []any) (map[string]map[string]int, error) {
	rows, err := db.Query(`
		SELECT SUBSTR(date_taken,1,?), `+aspectClassSQL("exif_json")+`, COUNT(*)
		FROM photos
		WHERE status='ok'
		  AND date_taken IS NOT NULL
		  AND `+hasSizeSQL("exif_json")+pcWhere+`
		GROUP BY 1, 2 ORDER BY 1`,
		append([]any{N}, pcArgs...)...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make(map[string]map[string]int)
	for rows.Next() {
		var period, aspect string
		var count int
		if err := rows.Scan(&period, &aspect, &count); err != nil {
			return nil, err
		}
		if out[period] == nil {
			out[period] = make(map[string]int)
		}
		out[period][aspect] = count
	}
	return out, nil
}

func tlMegapixels(db *sql.DB, N int, pcWhere string, pcArgs []any) ([]MegapixelStat, error) {
	rows, err := db.Query(`
		SELECT SUBSTR(date_taken,1,?),
		       MAX(CAST(json_extract(exif_json,'$.width') AS REAL) *
		           CAST(json_extract(exif_json,'$.height') AS REAL) / 1000000.0),
		       AVG(CAST(json_extract(exif_json,'$.width') AS REAL) *
		           CAST(json_extract(exif_json,'$.height') AS REAL) / 1000000.0),
		       COUNT(*)
		FROM photos
		WHERE status='ok'
		  AND date_taken IS NOT NULL
		  AND CAST(json_extract(exif_json,'$.width') AS INTEGER) > 0
		  AND CAST(json_extract(exif_json,'$.height') AS INTEGER) > 0`+pcWhere+`
		GROUP BY 1 ORDER BY 1`,
		append([]any{N}, pcArgs...)...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []MegapixelStat
	for rows.Next() {
		var ms MegapixelStat
		if err := rows.Scan(&ms.Period, &ms.Max, &ms.Avg, &ms.Count); err != nil {
			return nil, err
		}
		out = append(out, ms)
	}
	return out, nil
}

func assembleTL(
	granularity string,
	cameraRows []tlCameraRow,
	focalVals, isoVals map[string][]float64,
	aperMap, aspectMap map[string]map[string]int,
	mpStats []MegapixelStat,
) *LibraryTimeline {
	periodSet := make(map[string]bool)
	for _, r := range cameraRows {
		periodSet[r.period] = true
	}
	for p := range focalVals {
		periodSet[p] = true
	}
	for p := range isoVals {
		periodSet[p] = true
	}
	for p := range aperMap {
		periodSet[p] = true
	}
	for p := range aspectMap {
		periodSet[p] = true
	}
	for _, ms := range mpStats {
		periodSet[ms.Period] = true
	}

	periods := make([]string, 0, len(periodSet))
	for p := range periodSet {
		periods = append(periods, p)
	}
	sortStrings(periods)
	periodIdx := make(map[string]int, len(periods))
	for i, p := range periods {
		periodIdx[p] = i
	}

	return &LibraryTimeline{
		Granularity:    granularity,
		Periods:        periods,
		CameraUsage:    assembleCameraSlices(cameraRows, periods, periodIdx),
		FocalStats:     assemblePercentileStats(focalVals, periods),
		ISOStats:       assemblePercentileStats(isoVals, periods),
		ApertureHeat:   assembleApertureRows(aperMap, periods),
		AspectRatios:   assembleAspectSlices(aspectMap, periods, periodIdx),
		MegapixelStats: assembleMPStats(mpStats, periodIdx),
	}
}

// assembleCameraSlices is every camera's count per period. A library keeps
// all of them: only the answer as a whole is cut to the most used
// (topCameras), or merging libraries would rank what one of them had
// already summed up as "Other".
func assembleCameraSlices(rows []tlCameraRow, periods []string, idx map[string]int) []CameraTimeSlice {
	grid := make(map[string][]int)
	var order []string
	for _, r := range rows {
		pi, ok := idx[r.period]
		if !ok {
			continue
		}
		if grid[r.camera] == nil {
			grid[r.camera] = make([]int, len(periods))
			order = append(order, r.camera)
		}
		grid[r.camera][pi] += r.count
	}
	out := make([]CameraTimeSlice, 0, len(order))
	for _, camera := range order {
		out = append(out, CameraTimeSlice{Camera: camera, Counts: grid[camera]})
	}
	return out
}

func assemblePercentileStats(vals map[string][]float64, periods []string) []PeriodStats {
	out := make([]PeriodStats, 0, len(periods))
	for _, p := range periods {
		sorted := vals[p]
		if len(sorted) == 0 {
			continue
		}
		p25, median, p75 := computePercentiles(sorted)
		out = append(out, PeriodStats{Period: p, Median: median, P25: p25, P75: p75, Count: len(sorted)})
	}
	return out
}

func computePercentiles(sorted []float64) (p25, median, p75 float64) {
	n := len(sorted)
	return sorted[n/4], sorted[n/2], sorted[n*3/4]
}

func assembleApertureRows(aperMap map[string]map[string]int, periods []string) []ApertureRow {
	out := make([]ApertureRow, 0, len(periods))
	for _, p := range periods {
		buckets := aperMap[p]
		if len(buckets) == 0 {
			continue
		}
		cp := make(map[string]int, len(buckets))
		for k, v := range buckets {
			cp[k] = v
		}
		out = append(out, ApertureRow{Period: p, Buckets: cp})
	}
	return out
}

var tlAspectOrder = []string{"3:2", "4:3", "16:9+", "1:1", "other"}

func assembleAspectSlices(aspectMap map[string]map[string]int, periods []string, idx map[string]int) []AspectSlice {
	grid := make(map[string][]int, len(tlAspectOrder))
	for _, ratio := range tlAspectOrder {
		grid[ratio] = make([]int, len(periods))
	}
	for period, ratios := range aspectMap {
		pi, ok := idx[period]
		if !ok {
			continue
		}
		for ratio, count := range ratios {
			if _, known := grid[ratio]; !known {
				grid[ratio] = make([]int, len(periods))
			}
			grid[ratio][pi] += count
		}
	}
	out := make([]AspectSlice, 0, len(tlAspectOrder))
	for _, ratio := range tlAspectOrder {
		counts := grid[ratio]
		for _, c := range counts {
			if c > 0 {
				out = append(out, AspectSlice{Ratio: ratio, Counts: counts})
				break
			}
		}
	}
	return out
}

func stripExifQuotes(s string) string {
	if len(s) >= 2 && s[0] == '"' && s[len(s)-1] == '"' {
		return s[1 : len(s)-1]
	}
	return s
}

func assembleMPStats(mpStats []MegapixelStat, idx map[string]int) []MegapixelStat {
	out := make([]MegapixelStat, 0, len(mpStats))
	for _, ms := range mpStats {
		if _, ok := idx[ms.Period]; ok {
			out = append(out, ms)
		}
	}
	return out
}
