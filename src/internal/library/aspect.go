package library

// aspectClassSQL is the frame shape of a photo as one of "1:1", "4:3", "3:2",
// "16:9+" or "other", from the width and height columns of the photos table
// with the alias prefix p ("p." or ""). The timeline counts by it and search
// filters by it, so a shape clicked in the chart finds the photos it counted.
func aspectClassSQL(p string) string {
	ratio := "CAST(" + p + "width AS REAL) / " + p + "height"
	return `CASE
		WHEN ` + ratio + ` BETWEEN 0.98 AND 1.02 THEN '1:1'
		WHEN ` + ratio + ` BETWEEN 1.28 AND 1.42 THEN '4:3'
		WHEN ` + ratio + ` BETWEEN 1.45 AND 1.58 THEN '3:2'
		WHEN ` + ratio + ` > 1.65 THEN '16:9+'
		ELSE 'other'
	END`
}

// hasSizeSQL holds for photos whose width and height are known.
func hasSizeSQL(p string) string {
	return p + "width > 0 AND " + p + "height > 0"
}
