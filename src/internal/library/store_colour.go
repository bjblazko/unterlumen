package library

import "database/sql"

// ColourPhoto is what the Colour statistics read of one analysed photo.
type ColourPhoto struct {
	ID     string
	Date   string // date taken as stored, "" when unknown
	Mono   string // mono, tinted or colour
	Warmth float64
}

// ColourSwatch is one main colour of a colour photo.
type ColourSwatch struct {
	PhotoID string
	HueBin  int // 0…11, or -1 for a neutral
	Share   float64
	Colour  LCh
}

// ColourSource is everything the Colour statistics are built from, for one
// library and folder.
type ColourSource struct {
	Photos     []ColourPhoto
	Swatches   []ColourSwatch
	Unanalysed int // photos without measurements yet
}

// ColourSource reads the measured photos within pathPrefix, or the whole
// library when it is empty.
func (s *Store) ColourSource(pathPrefix string) (*ColourSource, error) {
	pathGlob := ""
	if pathPrefix != "" {
		pathGlob = escapeLikePattern(pathPrefix) + "/%"
	}
	where, args := tlAliasCond(pathGlob)
	src := &ColourSource{}
	var err error
	if src.Photos, err = colourPhotos(s.db, where, args); err != nil {
		return nil, err
	}
	if src.Swatches, err = colourSwatches(s.db, where, args); err != nil {
		return nil, err
	}
	src.Unanalysed, err = s.UnanalysedCount(pathPrefix)
	return src, err
}

// UnanalysedCount counts the photos within pathPrefix, or the whole library
// when it is empty, that have no measurements yet.
func (s *Store) UnanalysedCount(pathPrefix string) (int, error) {
	pathGlob := ""
	if pathPrefix != "" {
		pathGlob = escapeLikePattern(pathPrefix) + "/%"
	}
	where, args := tlAliasCond(pathGlob)
	var n int
	err := s.db.QueryRow(`
		SELECT COUNT(*) FROM photos p
		LEFT JOIN photo_appearance a ON a.photo_id = p.id
		WHERE p.status='ok' AND a.photo_id IS NULL`+where, args...).Scan(&n)
	return n, err
}

func colourPhotos(db *sql.DB, where string, args []any) ([]ColourPhoto, error) {
	rows, err := db.Query(`
		SELECT p.id, COALESCE(p.date_taken, ''), a.mono_class, COALESCE(a.warmth, 0)
		FROM photos p JOIN photo_appearance a ON a.photo_id = p.id
		WHERE p.status='ok'`+where, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []ColourPhoto{}
	err = scanRows(rows, func() error {
		var c ColourPhoto
		if err := rows.Scan(&c.ID, &c.Date, &c.Mono, &c.Warmth); err != nil {
			return err
		}
		out = append(out, c)
		return nil
	})
	return out, err
}

// colourSwatches reads the swatches of colour photos only: a black-and-white
// photo's greys would say nothing about the colours of the library.
func colourSwatches(db *sql.DB, where string, args []any) ([]ColourSwatch, error) {
	rows, err := db.Query(`
		SELECT pp.photo_id, COALESCE(pp.hue_bin, -1), pp.share, pp.l, pp.c, pp.h
		FROM photo_palette pp
		JOIN photos p ON p.id = pp.photo_id
		JOIN photo_appearance a ON a.photo_id = p.id
		WHERE p.status='ok' AND a.mono_class='colour'`+where, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []ColourSwatch{}
	err = scanRows(rows, func() error {
		var sw ColourSwatch
		if err := rows.Scan(&sw.PhotoID, &sw.HueBin, &sw.Share, &sw.Colour.L, &sw.Colour.C, &sw.Colour.H); err != nil {
			return err
		}
		out = append(out, sw)
		return nil
	})
	return out, err
}

// ColourPoint is one analysed photo where the Colour space draws it: at its
// main colour — the largest swatch with a hue — or, for a photo without
// one, its average colour.
type ColourPoint struct {
	ID     string
	Date   string // date taken as stored, "" when unknown
	Mono   string // mono, tinted or colour
	Colour LCh
	Lum    float64 // mean brightness, 0…1
	// Contrast is the standard deviation of lightness; Colourful the
	// Hasler–Süsstrunk colourfulness, about 0 for grey and above 100 when vivid.
	Contrast  float64
	Colourful float64
}

// ColourPoints reads every analysed photo within pathPrefix, or the whole
// library when it is empty.
func (s *Store) ColourPoints(pathPrefix string) ([]ColourPoint, error) {
	pathGlob := ""
	if pathPrefix != "" {
		pathGlob = escapeLikePattern(pathPrefix) + "/%"
	}
	where, args := tlAliasCond(pathGlob)
	// The bare columns of an aggregate with MIN come from the row holding the
	// minimum in SQLite: the swatch of the lowest rank, the largest. The file
	// name is left out on purpose: reading it reaches into each photo's row with
	// its EXIF, which took 1.2 s instead of 0.2 s on 32,000 photos.
	rows, err := s.db.Query(`
		SELECT p.id, COALESCE(p.date_taken, ''), a.mono_class,
		       COALESCE(m.l, a.avg_l, 0), COALESCE(m.c, a.avg_c, 0), COALESCE(m.h, a.avg_h, 0),
		       COALESCE(a.lum_mean, 0), COALESCE(a.contrast, 0), COALESCE(a.colourfulness, 0)
		FROM photos p
		JOIN photo_appearance a ON a.photo_id = p.id
		LEFT JOIN (SELECT photo_id, l, c, h, MIN(rank) FROM photo_palette
		           WHERE hue_bin IS NOT NULL GROUP BY photo_id) m ON m.photo_id = p.id
		WHERE p.status='ok'`+where, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []ColourPoint{}
	err = scanRows(rows, func() error {
		var c ColourPoint
		if err := rows.Scan(&c.ID, &c.Date, &c.Mono, &c.Colour.L, &c.Colour.C, &c.Colour.H, &c.Lum, &c.Contrast, &c.Colourful); err != nil {
			return err
		}
		out = append(out, c)
		return nil
	})
	return out, err
}
