package library

import (
	"sort"
	"time"
)

// sessionGap is how long a pause ends a session: photos taken with less
// between them belong together.
const sessionGap = 2 * time.Hour

// topSessions is how many of a library's largest sessions are kept by name.
const topSessions = 12

// timeInsights are the Time views' tallies (group "time"): sessions — runs
// of photos with less than sessionGap between them — by their size, and the
// largest by their first and last moment ("2024-05-01T10:00:00").
func (s *Store) timeInsights(sc statsScope) (Insights, error) {
	rows, err := s.db.Query(`SELECT SUBSTR(date_taken, 1, 19) FROM photos p
		WHERE LENGTH(date_taken) >= 19 AND p.id IN `+inScope(sc)+` ORDER BY 1`, sc.args...)
	if err != nil {
		return nil, err
	}
	var stamps []string
	err = scanRows(rows, func() error {
		var v string
		if err := rows.Scan(&v); err != nil {
			return err
		}
		stamps = append(stamps, v)
		return nil
	})
	if err != nil {
		return nil, err
	}
	sizes, largest := map[string]int{}, map[string]int{}
	for _, ss := range sessionsOf(stamps) {
		sizes[sessionBucket(ss.n)]++
		largest[ss.first+InsightSep+ss.last] = ss.n
	}
	return Insights{"sessionSizes": sizes, "sessions": keepLargest(largest, topSessions)}, nil
}

type session struct {
	first, last string
	n           int
}

// sessionsOf splits sorted moments (local time, as taken) into sessions.
func sessionsOf(stamps []string) []session {
	var out []session
	var prev time.Time
	for _, st := range stamps {
		at, err := time.Parse("2006-01-02T15:04:05", st)
		if err != nil {
			continue
		}
		if len(out) == 0 || at.Sub(prev) >= sessionGap {
			out = append(out, session{first: st})
		}
		cur := &out[len(out)-1]
		cur.last, cur.n, prev = st, cur.n+1, at
	}
	return out
}

// sessionBuckets are the sizes the histogram counts sessions in.
var sessionBuckets = []struct {
	upTo  int
	label string
}{{1, "1"}, {5, "2–5"}, {20, "6–20"}, {50, "21–50"}, {200, "51–200"}, {1 << 30, "201+"}}

func sessionBucket(n int) string {
	for _, b := range sessionBuckets {
		if n <= b.upTo {
			return b.label
		}
	}
	return ""
}

func keepLargest(m map[string]int, n int) map[string]int {
	type kv struct {
		k string
		v int
	}
	all := make([]kv, 0, len(m))
	for k, v := range m {
		all = append(all, kv{k, v})
	}
	sort.Slice(all, func(i, j int) bool { return all[i].v > all[j].v || (all[i].v == all[j].v && all[i].k < all[j].k) })
	out := map[string]int{}
	for i := 0; i < len(all) && i < n; i++ {
		out[all[i].k] = all[i].v
	}
	return out
}
