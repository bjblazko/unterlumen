package library

import (
	"fmt"
	"strings"
)

// Filter narrows the photos the Map, the Statistics and the Timeline count to
// a span of months, to cameras and lenses, to people and to things
// (ADR-0050, ADR-0054). The zero
// Filter narrows nothing, and every query runs exactly as it does without one.
type Filter struct {
	From, Until string   // months as "YYYY-MM", inclusive; "" leaves that end open
	Models      []string // camera models as exif_index stores them; any of them
	Lenses      []string // lens models as exif_index stores them; any of them
	People      []string // names given in People; all of them in the photo, as in a library's filter
	Things      []string // things found in photos (ADR-0054); all of them in the photo
	Kind        string   // KindPhoto or KindVideo (ADR-0058); "" for both
}

// Empty says the filter narrows nothing.
func (f Filter) Empty() bool {
	return f.From == "" && f.Until == "" && len(f.Models) == 0 && len(f.Lenses) == 0 && len(f.People) == 0 && len(f.Things) == 0 && f.Kind == ""
}

// Key identifies the filter in a cache key; "" for the empty one.
func (f Filter) Key() string {
	if f.Empty() {
		return ""
	}
	return fmt.Sprintf("%s~%s~%s~%s~%s~%s~%s", f.From, f.Until, strings.Join(f.Models, "\x1f"), strings.Join(f.Lenses, "\x1f"),
		strings.Join(f.People, "\x1f"), strings.Join(f.Things, "\x1f"), f.Kind)
}

// Cond restricts the photo IDs in col to the filter's photos: " AND col IN
// (…)" once per narrowed part, each a set collected once through an index,
// never per row. "" when the filter narrows nothing.
func (f Filter) Cond(col string) (string, []any) {
	var b strings.Builder
	var args []any
	if f.From != "" || f.Until != "" {
		b.WriteString(" AND " + col + " IN (SELECT id FROM photos WHERE date_taken IS NOT NULL")
		if f.From != "" {
			b.WriteString(" AND date_taken >= ?")
			args = append(args, f.From)
		}
		if f.Until != "" {
			b.WriteString(" AND date_taken < ?")
			args = append(args, nextMonth(f.Until))
		}
		b.WriteString(")")
	}
	for _, part := range []struct {
		field  string
		values []string
	}{{"Model", f.Models}, {"LensModel", f.Lenses}} {
		if len(part.values) == 0 {
			continue
		}
		b.WriteString(" AND " + col + " IN (SELECT photo_id FROM exif_index WHERE field='" + part.field + "' AND value IN (" + placeholders(len(part.values)) + "))")
		for _, v := range part.values {
			args = append(args, v)
		}
	}
	// Each person a set of their photos, made once (ADR-0045).
	for _, name := range f.People {
		b.WriteString(" AND " + col + ` IN (SELECT fa.photo_id FROM faces fa JOIN people pe ON pe.id = fa.person_id
			WHERE fa.confirmed = 1 AND pe.name = ?)`)
		args = append(args, name)
	}
	for _, thing := range f.Things {
		b.WriteString(" AND " + col + " IN (SELECT photo_id FROM photo_things WHERE thing = ?)")
		args = append(args, thing)
	}
	b.WriteString(kindCond(col, f.Kind))
	return b.String(), args
}

// nextMonth is the month after "YYYY-MM", as the first text a date of a
// later month sorts at or after.
func nextMonth(month string) string {
	var y, m int
	if _, err := fmt.Sscanf(month, "%d-%d", &y, &m); err != nil || m < 1 || m > 12 {
		return month + "￿" // not a month: everything starting with it
	}
	if m == 12 {
		return fmt.Sprintf("%04d-01", y+1)
	}
	return fmt.Sprintf("%04d-%02d", y, m+1)
}

func placeholders(n int) string {
	return strings.TrimSuffix(strings.Repeat("?,", n), ",")
}
