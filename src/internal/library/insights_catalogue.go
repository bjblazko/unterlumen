package library

// The Cataloguing views' tallies (group "catalogue"): stars per month (0 for
// none), the keywords, and the values of own fields (key and value).
func (s *Store) catalogueInsights(sc statsScope) (Insights, error) {
	in := Insights{}
	scope, okP := inScope(sc), okIn(sc)
	queries := []tallyQuery{
		{"stars", `SELECT ` + keyOf(monthOf, `CASE WHEN r.value IN ('1','2','3','4','5') THEN r.value ELSE '0' END`) + `, COUNT(*)
			FROM photos p LEFT JOIN exif_index r ON r.photo_id = p.id AND r.field = 'Rating'
			WHERE p.date_taken IS NOT NULL AND ` + okP + ` GROUP BY 1`},
		{"keywords", `SELECT value, COUNT(*) FROM exif_entries
			WHERE field = 'Keywords' AND photo_id IN ` + scope + ` GROUP BY value`},
		// Own fields are the notes (IsNoteKey) other than the title.
		{"fields", `SELECT ` + keyOf("key", "value") + `, COUNT(*) FROM photo_meta
			WHERE value <> '' AND key NOT IN ('title', 'rating') AND key NOT LIKE 'built:%' AND key NOT LIKE 'published:%' AND key NOT LIKE 'pending:%'
			  AND photo_id IN ` + scope + ` GROUP BY 1`},
	}
	return in, s.tallyAll(in, queries, sc.args)
}
