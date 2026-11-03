//go:build probe

package library

// Times the Statistics views' tallies and point clouds (ADR-0062) on a copy
// of a real library database:
//
//	PROBE_DB=/path/to/copy/library.db go test -tags probe -run ProbeInsights -v ./internal/library/

import (
	"os"
	"sort"
	"testing"
	"time"
)

func TestProbeInsights(t *testing.T) {
	path := os.Getenv("PROBE_DB")
	if path == "" {
		t.Skip("set PROBE_DB to a copy of a library.db")
	}
	db, err := openDB(path)
	if err != nil {
		t.Fatal(err)
	}
	s := newStore(db, "")
	sc := s.statsScope("")
	timed := func(name string, f func() error) {
		start := time.Now()
		if err := f(); err != nil {
			t.Errorf("%s: %v", name, err)
		}
		t.Logf("%-22s %6d ms", name, time.Since(start).Milliseconds())
	}
	// The first query reads the pages every later one shares; timed apart
	// so it is not charged to the first group.
	timed("first read", func() error {
		return db.QueryRow(`SELECT COUNT(*) FROM photos WHERE status='ok' AND date_taken IS NOT NULL`).Scan(new(int))
	})
	groups := make([]string, 0, len(insightGroups))
	for g := range insightGroups {
		groups = append(groups, g)
	}
	sort.Strings(groups)
	tallyHeard = func(chart, _ string, _ []any, took time.Duration) {
		if took > 50*time.Millisecond {
			t.Logf("  tally %-18s %6d ms", chart, took.Milliseconds())
		}
	}
	defer func() { tallyHeard = nil }()
	for _, g := range groups {
		timed("insights "+g, func() error { _, err := insightGroups[g](s, sc); return err })
	}
	timed("palette cloud", func() error { return s.addPalettes(&PointCloud{}, sc, 0, paletteSample) })
	var vecs [][]float32
	timed("read vectors", func() error {
		rows, err := db.Query(`SELECT vector FROM photo_vectors`)
		if err != nil {
			return err
		}
		return scanRows(rows, func() error {
			var b []byte
			if err := rows.Scan(&b); err != nil {
				return err
			}
			vecs = append(vecs, decodeEmbedding(b))
			return nil
		})
	})
	timed("principal axes", func() error { principalAxes(vecs); return nil })
}
