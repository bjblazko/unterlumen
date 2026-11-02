package library

// The People and Things views' tallies. People (group "people"): each named
// person's photos per month (their last month is when they were last
// seen), per country and per thing. Things (group "things"): each thing's
// photos per month of the year and per country.

func (s *Store) peopleInsights(sc statsScope) (Insights, error) {
	in := Insights{}
	scope := inScope(sc)
	queries := []struct{ chart, q string }{
		{"personMonth", `SELECT ` + keyOf("x.name", "x.month") + `, COUNT(*) FROM (
			SELECT DISTINCT pe.name AS name, fa.photo_id, SUBSTR(p.date_taken, 1, 7) AS month
			FROM faces fa JOIN people pe ON pe.id = fa.person_id
			JOIN photos p ON p.id = fa.photo_id
			WHERE fa.confirmed = 1 AND pe.name <> '' AND p.date_taken IS NOT NULL AND p.id IN ` + scope + `) x GROUP BY 1`},
		{"personCountry", `SELECT ` + keyOf("x.name", "x.country") + `, COUNT(*) FROM (
			SELECT DISTINCT pe.name AS name, fa.photo_id, ` + textOf("c.value") + ` AS country
			FROM faces fa JOIN people pe ON pe.id = fa.person_id
			JOIN exif_index c ON c.photo_id = fa.photo_id AND c.field = 'Country'
			WHERE fa.confirmed = 1 AND pe.name <> '' AND fa.photo_id IN ` + scope + `) x GROUP BY 1`},
		{"personThing", `SELECT ` + keyOf("x.name", "x.thing") + `, COUNT(*) FROM (
			SELECT DISTINCT pe.name AS name, fa.photo_id, t.thing AS thing
			FROM faces fa JOIN people pe ON pe.id = fa.person_id
			JOIN photo_things t ON t.photo_id = fa.photo_id
			WHERE fa.confirmed = 1 AND pe.name <> '' AND fa.photo_id IN ` + scope + `) x GROUP BY 1`},
	}
	for _, q := range queries {
		if err := s.tally(in, q.chart, q.q, sc.args...); err != nil {
			return nil, err
		}
	}
	return in, nil
}

func (s *Store) thingsInsights(sc statsScope) (Insights, error) {
	in := Insights{}
	scope := inScope(sc)
	queries := []struct{ chart, q string }{
		{"thingMonth", `SELECT ` + keyOf("t.thing", "CAST(CAST(SUBSTR(p.date_taken, 6, 2) AS INTEGER) AS TEXT)") + `, COUNT(*)
			FROM photo_things t JOIN photos p ON p.id = t.photo_id
			WHERE p.date_taken IS NOT NULL AND p.id IN ` + scope + ` GROUP BY 1`},
		{"thingCountry", `SELECT ` + keyOf("t.thing", textOf("c.value")) + `, COUNT(*)
			FROM photo_things t JOIN exif_index c ON c.photo_id = t.photo_id AND c.field = 'Country'
			WHERE t.photo_id IN ` + scope + ` GROUP BY 1`},
	}
	for _, q := range queries {
		if err := s.tally(in, q.chart, q.q, sc.args...); err != nil {
			return nil, err
		}
	}
	return in, nil
}
