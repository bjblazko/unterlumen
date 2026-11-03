package library

import (
	"fmt"
	"strings"
	"sync"
	"time"
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

// insightChanges are the groups that count faces or things, which change
// outside scans: naming a face is seen at once (index_changes).
var insightChanges = map[string]string{"people": "people", "things": "things"}

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
	changes := ""
	if topic, ok := insightChanges[group]; ok {
		changes = m.changesOf(libs, topic)
	}
	if v, ok := loadChanging(&m.insightsCache, key, changes); ok {
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
	storeChanging(&m.insightsCache, key, changes, out)
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

// tallyHeard, when set (tests and probes), hears each tally's query and how
// long it took.
var tallyHeard func(chart, query string, args []any, took time.Duration)

// tallyQuery is one chart's query of (key, count) rows.
type tallyQuery struct{ chart, q string }

// tallyAll runs a group's queries side by side, as many at once as the
// store has connections (readersPerLibrary through OpenFiltered).
func (s *Store) tallyAll(in Insights, queries []tallyQuery, args []any) error {
	var mu sync.Mutex
	errs := make([]error, len(queries))
	var wg sync.WaitGroup
	for i, q := range queries {
		wg.Add(1)
		go func() {
			defer wg.Done()
			one := Insights{}
			errs[i] = s.tally(one, q.chart, q.q, args...)
			mu.Lock()
			in[q.chart] = one[q.chart]
			mu.Unlock()
		}()
	}
	wg.Wait()
	for _, err := range errs {
		if err != nil {
			return err
		}
	}
	return nil
}

// tally runs a query of (key, count) rows into the chart's tally of in.
func (s *Store) tally(in Insights, chart, query string, args ...any) error {
	if tallyHeard != nil {
		defer func(start time.Time) { tallyHeard(chart, query, args, time.Since(start)) }(time.Now())
	}
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

// okIn is the WHERE of a query on photos aliased p: the scope's ok photos.
// Filtering p itself lets SQLite walk the covering index on (status,
// date_taken, id) instead of reading each photo's row with its EXIF JSON to
// test "p.id IN" — 0.08 s instead of 3 s on 33,000 photos.
func okIn(sc statsScope) string { return "p.status = 'ok'" + sc.p }

// keyOf joins SQL expressions into a tally key.
func keyOf(parts ...string) string {
	return strings.Join(parts, " || char(31) || ")
}

// The month a photo was taken in, "YYYY-MM", from p.
const monthOf = "SUBSTR(p.date_taken, 1, 7)"

// textOf is a text tag's value without the quotes EXIF strings carry.
func textOf(col string) string { return "TRIM(" + col + ", '\" ')" }
