package library

// The Equipment and Exposure views' tallies (group "gear"): phone or camera
// per month, the focal lengths each lens is used at, each camera's and
// lens's months (their life lines), the risk of shake per lens, the scenes'
// light (EV100), exposure times in stops, flash and exposure compensation
// per month, and white balance.
func (s *Store) gearInsights(sc statsScope) (Insights, error) {
	in := Insights{}
	lens, model := textOf("l.value"), textOf("m.value")
	scope := inScope(sc)
	queries := []struct{ chart, q string }{
		{"device", `SELECT ` + keyOf(monthOf, `CASE WHEN LOWER(`+textOf("mk.value")+`) IN (`+phoneMakeList()+`) THEN 'phone' ELSE 'camera' END`) + `, COUNT(*)
			FROM photos p JOIN exif_index mk ON mk.photo_id = p.id AND mk.field = 'Make'
			WHERE p.date_taken IS NOT NULL AND p.id IN ` + scope + ` GROUP BY 1`},
		{"lensFocal", `SELECT ` + keyOf(lens, "CAST(CAST(ROUND(f.numeric_value) AS INTEGER) AS TEXT)") + `, COUNT(*)
			FROM exif_index l JOIN exif_index f ON f.photo_id = l.photo_id AND f.field = 'FocalLength' AND f.numeric_value > 0
			WHERE l.field = 'LensModel' AND l.photo_id IN ` + scope + ` GROUP BY 1`},
		{"lensMonth", `SELECT ` + keyOf(lens, monthOf) + `, COUNT(*)
			FROM photos p JOIN exif_index l ON l.photo_id = p.id AND l.field = 'LensModel'
			WHERE p.date_taken IS NOT NULL AND p.id IN ` + scope + ` GROUP BY 1`},
		{"cameraMonth", `SELECT ` + keyOf(model, monthOf) + `, COUNT(*)
			FROM photos p JOIN exif_index m ON m.photo_id = p.id AND m.field = 'Model'
			WHERE p.date_taken IS NOT NULL AND p.id IN ` + scope + ` GROUP BY 1`},
		{"shake", `SELECT ` + keyOf(lens, `CASE WHEN `+slowSQL+` THEN 'slow' ELSE 'ok' END`) + `, COUNT(*)
			FROM exif_index t` + focalJoins + `
			JOIN exif_index l ON l.photo_id = t.photo_id AND l.field = 'LensModel'
			WHERE t.field = 'ExposureTime' AND ` + focal35 + ` > 0 AND t.photo_id IN ` + scope + ` GROUP BY 1`},
		{"ev", `SELECT CAST(CAST(ROUND(` + evSQL + `) AS INTEGER) AS TEXT), COUNT(*)` + evFrom + ` AND t.photo_id IN ` + scope + ` GROUP BY 1`},
		{"exposureTime", `SELECT CAST(CAST(ROUND(log2(t.numeric_value)) AS INTEGER) AS TEXT), COUNT(*)
			FROM exif_index t WHERE t.field = 'ExposureTime' AND t.numeric_value > 0 AND t.photo_id IN ` + scope + ` GROUP BY 1`},
		{"flash", `SELECT ` + keyOf(monthOf, `CASE WHEN (CAST(TRIM(fl.value, '"') AS INTEGER) & 1) = 1 THEN 'fired' ELSE 'not' END`) + `, COUNT(*)
			FROM photos p JOIN exif_index fl ON fl.photo_id = p.id AND fl.field = 'Flash'
			WHERE p.date_taken IS NOT NULL AND p.id IN ` + scope + ` GROUP BY 1`},
		{"bias", `SELECT ` + keyOf(monthOf, `CASE WHEN `+biasSQL("b.value")+` < -0.15 THEN 'minus' WHEN `+biasSQL("b.value")+` > 0.15 THEN 'plus' ELSE 'zero' END`) + `, COUNT(*)
			FROM photos p JOIN exif_index b ON b.photo_id = p.id AND b.field = 'ExposureBiasValue'
			WHERE p.date_taken IS NOT NULL AND INSTR(b.value, '/') > 0 AND p.id IN ` + scope + ` GROUP BY 1`},
		{"whiteBalance", `SELECT TRIM(value, '"'), COUNT(*) FROM exif_index
			WHERE field = 'WhiteBalance' AND photo_id IN ` + scope + ` GROUP BY 1`},
	}
	for _, q := range queries {
		if err := s.tally(in, q.chart, q.q, sc.args...); err != nil {
			return nil, err
		}
	}
	return in, nil
}
