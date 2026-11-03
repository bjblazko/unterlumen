package library

// The Places views' tallies (group "places"): the photos of each day in each
// cell of a tenth of a degree, from which the views find home and the
// journeys away from it; the countries each day names; and the photos per
// half-degree cell by their main hue, for the density landscape.
func (s *Store) placesInsights(sc statsScope) (Insights, error) {
	in := Insights{}
	okP := okIn(sc)
	lat, lon := "p.lat", "p.lon"
	cell := func(v string, per int) string {
		return "CAST(CAST(ROUND(" + v + " * " + itoa(per) + ") AS INTEGER) AS TEXT)"
	}
	located := " AND p.lat IS NOT NULL AND p.lon IS NOT NULL"
	queries := []tallyQuery{
		{"days", `SELECT ` + keyOf("SUBSTR(p.date_taken, 1, 10)", cell(lat, 10), cell(lon, 10)) + `, COUNT(*) FROM photos p
			WHERE p.date_taken IS NOT NULL` + located + ` AND ` + okP + ` GROUP BY 1`},
		{"countryDays", `SELECT ` + keyOf("SUBSTR(p.date_taken, 1, 10)", textOf("c.value")) + `, COUNT(*)
			FROM photos p JOIN exif_index c ON c.photo_id = p.id AND c.field = 'Country'
			WHERE p.date_taken IS NOT NULL AND ` + okP + ` GROUP BY 1`},
		{"density", `SELECT ` + keyOf(cell(lat, 2), cell(lon, 2), "COALESCE(CAST(m.hue_bin AS TEXT), '-')") + `, COUNT(*)
			FROM photos p LEFT JOIN (SELECT photo_id, hue_bin, MIN(rank) FROM photo_palette WHERE hue_bin IS NOT NULL GROUP BY photo_id) m ON m.photo_id = p.id
			WHERE 1` + located + ` AND ` + okP + ` GROUP BY 1`},
	}
	return in, s.tallyAll(in, queries, sc.args)
}
