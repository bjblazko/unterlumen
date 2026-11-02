package library

import (
	"fmt"
	"strings"
)

// Insights are the counts behind the Statistics views of 2026-11-02 that
// the snapshot does not carry: for each chart a tally, key → photos. A key
// joins its dimensions with InsightSep ("XF23mmF2\x1f23" is a lens and a
// focal length). Tallies of several libraries are summed, so one shape
// serves every view and its cache. They are read only by the topic that
// shows them (a group), so the overview's snapshot stays as quick as it was.
type Insights map[string]map[string]int

// InsightSep joins the dimensions of a tally's key.
const InsightSep = "\x1f"

// insightGroups are the groups a topic asks for, and what each counts.
var insightGroups = map[string]func(*Store, statsScope) (Insights, error){
	"time":      (*Store).timeInsights,
	"gear":      (*Store).gearInsights,
	"frame":     (*Store).frameInsights,
	"catalogue": (*Store).catalogueInsights,
	"places":    (*Store).placesInsights,
	"colour":    (*Store).colourInsights,
	"people":    (*Store).peopleInsights,
	"things":    (*Store).thingsInsights,
}

// Insights sums one group's tallies across the libraries asked for (all
// when ids is nil), within pathPrefix and the shared filter (ADR-0050).
func (m *Manager) Insights(ids []string, pathPrefix, group string, f Filter) (Insights, error) {
	count, ok := insightGroups[group]
	if !ok {
		return nil, fmt.Errorf("no statistics group %q", group)
	}
	libs, err := m.filterLibraries(ids)
	if err != nil {
		return nil, err
	}
	key := statsCacheKey(libraryIDs(libs), pathPrefix+f.Key()) + "|" + group
	if v, ok := m.insightsCache.Load(key); ok {
		return v.(Insights), nil
	}
	out := Insights{}
	for _, l := range libs {
		store, err := m.OpenFiltered(l.ID, f)
		if err != nil {
			continue
		}
		in, err := count(store, store.statsScope(pathPrefix))
		store.Close()
		if err != nil {
			return nil, err
		}
		out.add(in)
	}
	m.insightsCache.Store(key, out)
	return out, nil
}

func (in Insights) add(other Insights) {
	for chart, tally := range other {
		t := in[chart]
		if t == nil {
			t = map[string]int{}
			in[chart] = t
		}
		for k, n := range tally {
			t[k] += n
		}
	}
}

// tally runs a query of (key, count) rows into the chart's tally of in.
func (s *Store) tally(in Insights, chart, query string, args ...any) error {
	rows, err := s.db.Query(query, args...)
	if err != nil {
		return fmt.Errorf("%s: %w", chart, err)
	}
	t := map[string]int{}
	err = scanRows(rows, func() error {
		var k string
		var n int
		if err := rows.Scan(&k, &n); err != nil {
			return err
		}
		if k != "" {
			t[k] += n
		}
		return nil
	})
	in[chart] = t
	return err
}

// inScope is the set of the scope's photos, for "p.id IN …"; it takes the
// scope's arguments once.
func inScope(sc statsScope) string {
	return "(SELECT id FROM photos WHERE status='ok'" + sc.photos + ")"
}

// keyOf joins SQL expressions into a tally key.
func keyOf(parts ...string) string {
	return strings.Join(parts, " || char(31) || ")
}

// The month a photo was taken in, "YYYY-MM", from p.
const monthOf = "SUBSTR(p.date_taken, 1, 7)"

// textOf is a text tag's value without the quotes EXIF strings carry.
func textOf(col string) string { return "TRIM(" + col + ", '\" ')" }
