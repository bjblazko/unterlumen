package library

import (
	"sort"
)

// ScopeValues is what the shared filter offers (ADR-0050): the cameras and
// lenses of the chosen libraries with their photo counts — the lenses only of
// the chosen cameras, when there are any — and the months the photos span.
type ScopeValues struct {
	Cameras    []NameCount `json:"cameras"` // name as exif_index stores it
	Lenses     []NameCount `json:"lenses"`
	FirstMonth string      `json:"firstMonth"` // "YYYY-MM", "" without dated photos
	LastMonth  string      `json:"lastMonth"`
}

// ScopeValues reads the filter's choices across the requested libraries (or
// all if ids is nil). A library that cannot be read is left out.
func (m *Manager) ScopeValues(ids, models []string) (*ScopeValues, error) {
	libs, err := m.filterLibraries(ids)
	if err != nil {
		return nil, err
	}
	cameras, lenses := map[string]int{}, map[string]int{}
	out := &ScopeValues{}
	for _, l := range libs {
		store, err := m.OpenStore(l.ID)
		if err != nil {
			continue
		}
		store.addValueCounts(cameras, "Model", nil)
		store.addValueCounts(lenses, "LensModel", models)
		first, last := store.monthSpan()
		if first != "" && (out.FirstMonth == "" || first < out.FirstMonth) {
			out.FirstMonth = first
		}
		if last > out.LastMonth {
			out.LastMonth = last
		}
	}
	out.Cameras, out.Lenses = sortedCounts(cameras), sortedCounts(lenses)
	return out, nil
}

// addValueCounts adds the photos per value of field, among the photos of
// any of models when there are any.
func (s *Store) addValueCounts(into map[string]int, field string, models []string) {
	where, args := "", []any{field}
	if len(models) > 0 {
		where = " AND photo_id IN (SELECT photo_id FROM exif_index WHERE field='Model' AND value IN (" + placeholders(len(models)) + "))"
		for _, m := range models {
			args = append(args, m)
		}
	}
	rows, err := s.db.Query(`SELECT value, COUNT(*) FROM exif_index
		WHERE field = ? AND photo_id IN (SELECT id FROM photos WHERE status='ok')`+where+`
		GROUP BY value`, args...)
	if err != nil {
		return
	}
	defer rows.Close()
	for rows.Next() {
		var v string
		var n int
		if rows.Scan(&v, &n) == nil && v != "" {
			into[v] += n
		}
	}
}

func (s *Store) monthSpan() (first, last string) {
	s.db.QueryRow(`SELECT COALESCE(MIN(SUBSTR(date_taken,1,7)), ''), COALESCE(MAX(SUBSTR(date_taken,1,7)), '')
		FROM photos WHERE status='ok' AND date_taken IS NOT NULL`).Scan(&first, &last) //nolint:errcheck
	return first, last
}

// sortedCounts lists the most used first, then by value.
func sortedCounts(m map[string]int) []NameCount {
	out := make([]NameCount, 0, len(m))
	for v, n := range m {
		out = append(out, NameCount{Name: v, Count: n})
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Count != out[j].Count {
			return out[i].Count > out[j].Count
		}
		return out[i].Name < out[j].Name
	})
	return out
}
