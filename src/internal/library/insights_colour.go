package library

import "strconv"

// The Colour views' tallies (group "colour"): colourfulness in steps of five
// per film simulation and per camera, and each main colour's photos per hour
// of the day.
func (s *Store) colourInsights(sc statsScope) (Insights, error) {
	in := Insights{}
	scope, okP := inScope(sc), okIn(sc)
	bin := "CAST(CAST(a.colourfulness / 5 AS INTEGER) * 5 AS TEXT)"
	queries := []tallyQuery{
		{"colourfulFilm", `SELECT ` + keyOf(textOf("x.value"), bin) + `, COUNT(*)
			FROM photo_appearance a JOIN exif_index x ON x.photo_id = a.photo_id AND x.field = 'FilmSimulation'
			WHERE a.colourfulness IS NOT NULL AND a.mono_class = 'colour' AND a.photo_id IN ` + scope + ` GROUP BY 1`},
		{"colourfulCamera", `SELECT ` + keyOf(textOf("x.value"), bin) + `, COUNT(*)
			FROM photo_appearance a JOIN exif_index x ON x.photo_id = a.photo_id AND x.field = 'Model'
			WHERE a.colourfulness IS NOT NULL AND a.mono_class = 'colour' AND a.photo_id IN ` + scope + ` GROUP BY 1`},
		// A hue counts where it is a main colour, as the search's hue_bin
		// reads it (a swatch of at least HueShareMin), once per photo.
		{"hueHour", `SELECT ` + keyOf("x.hr", "x.hue") + `, COUNT(*) FROM (
			SELECT DISTINCT p.id, CAST(CAST(SUBSTR(p.date_taken, 12, 2) AS INTEGER) AS TEXT) AS hr, CAST(pp.hue_bin AS TEXT) AS hue
			FROM photos p JOIN photo_palette pp ON pp.photo_id = p.id
			WHERE pp.hue_bin IS NOT NULL AND pp.share >= ` + strconv.FormatFloat(HueShareMin, 'f', -1, 64) + `
			  AND LENGTH(p.date_taken) >= 13 AND ` + okP + `) x GROUP BY 1`},
	}
	return in, s.tallyAll(in, queries, sc.args)
}
