package library

// The Frame views' tallies (group "frame"): how each camera is held, the
// share of cropped photos per month, and the aspect ratio in steps of 0.05.
func (s *Store) frameInsights(sc statsScope) (Insights, error) {
	in := Insights{}
	okP := okIn(sc)
	queries := []tallyQuery{
		{"orient", `SELECT ` + keyOf(textOf("m.value"), orientSQL) + `, COUNT(*)
			FROM photos p` + orientJoin + ` JOIN exif_index m ON m.photo_id = p.id AND m.field = 'Model'
			WHERE ` + sizedSQL + ` AND ` + okP + ` GROUP BY 1`},
		{"cropped", `SELECT ` + keyOf(monthOf, `CASE WHEN `+croppedSQL+` THEN 'cropped' ELSE 'full' END`) + `, COUNT(*)` +
			croppedFrom + ` AND p.date_taken IS NOT NULL AND ` + okP + ` GROUP BY 1`},
		{"ratio", `SELECT printf('%.2f', ` + ratioSQL + `), COUNT(*) FROM photos p
			WHERE ` + sizedSQL + ` AND ` + okP + ` GROUP BY 1`},
	}
	return in, s.tallyAll(in, queries, sc.args)
}
