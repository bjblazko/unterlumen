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
	PathPrefix     string                   // absolute folder the photos must be in, as statistics take it
	Hour           *int                     // hour of the day taken, 0–23
	Aspect         string                   // frame shape, as aspectClassSQL names it
	Month          *int                     // month of the year taken, 1–12, any year
	Mono           string                   // mono, tinted or colour (photo_appearance)
	HueBin         *int                     // a main colour: a swatch of this 30° sector, 0–11, of at least HueShareMin
	Hues           []int                    // a colour combination: each of these sectors covers at least ComboShareMin (ADR-0049)
	Warmth         string                   // warm or cool, beyond WarmthThreshold
	PhotoID        string                   // one photo, by its id
	Offset         int
	Limit          int
}

// ListPhotos returns a filtered, paginated list of photos.
func (s *Store) ListPhotos(opts ListPhotosOpts) (ListPhotosResult, error) {
	fromSQL, whereSQL, args := newPhotoFilter(opts).sql()

	var total int
	if err := s.db.QueryRow(`SELECT COUNT(*) `+fromSQL+` WHERE `+whereSQL, args...).Scan(&total); err != nil {
		return ListPhotosResult{}, err
	}

	rows, err := s.db.Query(
		`SELECT p.id, p.path_hint, p.filename, p.file_size, p.indexed_at, p.status, p.date_taken,
		        (SELECT value FROM exif_index WHERE photo_id=p.id AND field='GPSLatitude' LIMIT 1),
		        (SELECT value FROM exif_index WHERE photo_id=p.id AND field='FilmSimulation' LIMIT 1)
		 `+fromSQL+` WHERE `+whereSQL+
			` ORDER BY CASE WHEN p.date_taken IS NULL OR p.date_taken = '' THEN 1 ELSE 0 END, p.date_taken DESC LIMIT ? OFFSET ?`,
		append(args, opts.Limit, opts.Offset)...,
	)
	if err != nil {
		return ListPhotosResult{}, err
	}
	photos, err := scanListedPhotos(rows)
	if err != nil {
		return ListPhotosResult{}, err
	}
	return ListPhotosResult{Photos: photos, Total: total}, nil
}

// scanListedPhotos reads ListPhotos' rows, never nil.
func scanListedPhotos(rows *sql.Rows) ([]Photo, error) {
	photos := []Photo{}
	err := scanRows(rows, func() error {
		var p Photo
		var indexedAt string
		var dateTaken sql.NullString
		var gpsLat, filmSim *string
		if err := rows.Scan(&p.ID, &p.PathHint, &p.Filename, &p.FileSize, &indexedAt, &p.Status, &dateTaken, &gpsLat, &filmSim); err != nil {
			return err
		}
		p.IndexedAt, _ = time.Parse(time.RFC3339, indexedAt)
		if dateTaken.Valid {
			p.DateTaken = dateTaken.String
		}
		p.Exif = overlayExif(gpsLat, filmSim, nil, nil)
		photos = append(photos, p)
		return nil
	})
	if err != nil {
		return nil, err
	}
	return photos, nil
}

// photoFilter is the joins and conditions of a photo list query. Joins take
// their arguments before the conditions do, since they come first in the SQL.
type photoFilter struct {
	joins     []string
	joinArgs  []any
	where     []string
	whereArgs []any
	aliases   int
}

func newPhotoFilter(opts ListPhotosOpts) *photoFilter {
	f := &photoFilter{where: []string{"p.status='ok'"}}
	f.addExifText(opts.Filters)
	f.addExifNumeric(opts.NumericFilters)
	f.addDatesAndExt(opts)
	f.addPlaceAndShape(opts)
	f.addAppearance(opts)
	f.addMeta(opts)
	return f
}

// join adds a join on exif_index under a fresh alias; clause names the alias
// as %[1]s.
func (f *photoFilter) join(prefix, clause string, args ...any) {
	a := fmt.Sprintf("%s%d", prefix, f.aliases)
	f.aliases++
	f.joins = append(f.joins, fmt.Sprintf(clause, a))
	f.joinArgs = append(f.joinArgs, args...)
}

func (f *photoFilter) cond(where string, args ...any) {
	f.where = append(f.where, where)
	f.whereArgs = append(f.whereArgs, args...)
}

// addExifText requires an exact EXIF text value per field.
func (f *photoFilter) addExifText(filters map[string]string) {
	for field, val := range filters {
		f.join("ef", `JOIN exif_index %[1]s ON %[1]s.photo_id=p.id AND %[1]s.field=? AND TRIM(TRIM(%[1]s.value,'"'))=?`, field, val)
	}
}

// addExifNumeric requires a numeric EXIF range per field. FocalLength35 is
// FocalLengthIn35mmFilm, or FocalLength for photos that lack it.
func (f *photoFilter) addExifNumeric(filters map[string]NumericFilter) {
	for field, r := range filters {
		if field != "FocalLength35" {
			f.join("en", `JOIN exif_index %[1]s ON %[1]s.photo_id=p.id AND %[1]s.field=? AND %[1]s.numeric_value BETWEEN ? AND ?`, field, r.Min, r.Max)
			continue
		}
		f.cond(`(
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
		)`, r.Min, r.Max, r.Min, r.Max)
	}
}

func (f *photoFilter) addDatesAndExt(opts ListPhotosOpts) {
	if opts.DateMin != "" {
		f.cond(`(p.date_taken IS NOT NULL AND SUBSTR(p.date_taken, 1, 10) >= ?)`, opts.DateMin)
	}
	if opts.DateMax != "" {
		f.cond(`(p.date_taken IS NOT NULL AND SUBSTR(p.date_taken, 1, 10) <= ?)`, opts.DateMax)
	}
	if opts.Month != nil {
		f.cond(`(LENGTH(p.date_taken) >= 7 AND CAST(SUBSTR(p.date_taken, 6, 2) AS INTEGER) = ?)`, *opts.Month)
	}
	if opts.ExtFilter != "" {
		f.cond(`p.ext = ?`, opts.ExtFilter)
	}
	if opts.PhotoID != "" {
		f.cond(`p.id = ?`, opts.PhotoID)
	}
}

// addAppearance requires what a photo looks like: the marks of the Colour
// statistics, with the same thresholds they count by.
func (f *photoFilter) addAppearance(opts ListPhotosOpts) {
	if opts.Mono != "" {
		f.cond(`EXISTS (SELECT 1 FROM photo_appearance a WHERE a.photo_id=p.id AND a.mono_class=?)`, opts.Mono)
	}
	if opts.HueBin != nil {
		// The photos are collected once through the (hue_bin, share) index. As
		// a correlated EXISTS, SQLite took that index for every photo and
		// walked all swatches of the hue each time: minutes on 48,000 photos.
		f.cond(`p.id IN (SELECT photo_id FROM photo_palette WHERE hue_bin=? AND share >= ?)`, *opts.HueBin, HueShareMin)
	}
	// One set per hue, read through the same index; a hue's swatches add up.
	for _, bin := range opts.Hues {
		f.cond(`p.id IN (SELECT photo_id FROM photo_palette WHERE hue_bin=? GROUP BY photo_id HAVING SUM(share) >= ?)`, bin, ComboShareMin)
	}
	switch opts.Warmth {
	case "warm":
		f.cond(`EXISTS (SELECT 1 FROM photo_appearance a WHERE a.photo_id=p.id AND a.warmth > ?)`, WarmthThreshold)
	case "cool":
		f.cond(`EXISTS (SELECT 1 FROM photo_appearance a WHERE a.photo_id=p.id AND a.warmth < ?)`, -WarmthThreshold)
	}
}

// addPlaceAndShape requires a folder, an hour of the day and a frame shape:
// the scopes and marks of the statistics.
func (f *photoFilter) addPlaceAndShape(opts ListPhotosOpts) {
	if opts.PathPrefix != "" {
		f.cond(`p.path_hint LIKE ? ESCAPE '\'`, escapeLikePattern(opts.PathPrefix)+"/%")
	}
	if opts.Hour != nil {
		f.cond(`(LENGTH(p.date_taken) >= 13 AND CAST(SUBSTR(p.date_taken, 12, 2) AS INTEGER) = ?)`, *opts.Hour)
	}
	if opts.Aspect != "" {
		f.cond(`(`+hasSizeSQL("p.exif_json")+` AND `+aspectClassSQL("p.exif_json")+` = ?)`, opts.Aspect)
	}
}

// addMeta requires meta values, meta keys and an album title.
func (f *photoFilter) addMeta(opts ListPhotosOpts) {
	for key, val := range opts.MetaFilters {
		f.cond(`EXISTS (SELECT 1 FROM photo_meta pm WHERE pm.photo_id=p.id AND pm.key=? AND pm.value=?)`, key, val)
	}
	for _, key := range opts.MetaExists {
		// built: keys may not exist yet for photos indexed before the built: prefix
		// replaced published:; fall back to the legacy key so those still match.
		if legacyKey, ok := strings.CutPrefix(key, "built:"); ok {
			f.cond(`EXISTS (SELECT 1 FROM photo_meta pm WHERE pm.photo_id=p.id AND (pm.key=? OR pm.key=?))`, key, "published:"+legacyKey)
		} else {
			f.cond(`EXISTS (SELECT 1 FROM photo_meta pm WHERE pm.photo_id=p.id AND pm.key=?)`, key)
		}
	}
	if opts.AlbumTitle != "" {
		f.cond(`EXISTS (SELECT 1 FROM photo_meta pm WHERE pm.photo_id=p.id AND (pm.key LIKE 'built:%:title' OR pm.key LIKE 'published:%:title') AND pm.value=?)`, opts.AlbumTitle)
	}
}

// sql returns the FROM clause, the WHERE conditions and their arguments.
func (f *photoFilter) sql() (fromSQL, whereSQL string, args []any) {
	fromSQL = "FROM photos p"
	if len(f.joins) > 0 {
		fromSQL += " " + strings.Join(f.joins, " ")
	}
	args = append(append([]any{}, f.joinArgs...), f.whereArgs...)
	return fromSQL, strings.Join(f.where, " AND "), args
}
