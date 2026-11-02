package library

// What the Cataloguing topic counts as described (ADR-0061): for each field,
// the set of photos that have it. The count in Statistics and the search
// parameter `lacks` read the same set, so a click on "missing" shows exactly
// the photos counted as missing. Each is a set from an index, never a
// subquery per photo (ADR-0045).

// CompletenessFields are the fields, in the order the chart shows them.
var CompletenessFields = []string{"title", "description", "keywords", "location", "country", "people", "rating"}

var completenessSets = map[string]string{
	"title": `SELECT photo_id FROM photo_meta WHERE key='title' AND value<>''
		UNION SELECT photo_id FROM exif_index WHERE field='Title' AND TRIM(value,'" ')<>''`,
	"description": `SELECT photo_id FROM exif_index WHERE field='ImageDescription' AND TRIM(value,'" ')<>''`,
	"keywords":    `SELECT photo_id FROM exif_entries WHERE field='Keywords'`,
	"location":    `SELECT photo_id FROM exif_index WHERE field='GPSLatitude'`,
	"country":     `SELECT photo_id FROM exif_index WHERE field='Country' AND TRIM(value,'" ')<>''`,
	"people":      `SELECT photo_id FROM faces WHERE confirmed=1 AND person_id IS NOT NULL`,
	"rating":      `SELECT photo_id FROM exif_index WHERE field='Rating' AND value IN ('1','2','3','4','5')`,
}

// lacksCond narrows col to the photos without field; "" for an unknown one.
func lacksCond(col, field string) string {
	set, ok := completenessSets[field]
	if !ok {
		return ""
	}
	return col + " NOT IN (" + set + ")"
}

// completeness counts, per field, the photos in scope that have it.
func (s *Store) completeness(sc statsScope) ([]NameCount, error) {
	out := make([]NameCount, 0, len(CompletenessFields))
	for _, f := range CompletenessFields {
		var n int
		q := `SELECT COUNT(*) FROM photos WHERE status='ok' AND id IN (` + completenessSets[f] + `)` + sc.photos
		if err := s.db.QueryRow(q, sc.args...).Scan(&n); err != nil {
			return nil, err
		}
		out = append(out, NameCount{Name: f, Count: n})
	}
	return out, nil
}
