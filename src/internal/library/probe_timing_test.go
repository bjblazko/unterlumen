//go:build probe

package library

// Times the store's statistics and search queries on a copy of a real
// library database:
//
//	PROBE_DB=/path/to/copy/library.db go test -tags probe -run Probe -v ./internal/library/

import (
	"os"
	"testing"
	"time"
)

func TestProbeTimings(t *testing.T) {
	path := os.Getenv("PROBE_DB")
	if path == "" {
		t.Skip("set PROBE_DB to a copy of a library.db")
	}
	db, err := openDB(path)
	if err != nil {
		t.Fatal(err)
	}
	s := newStore(db, "")
	var root string
	db.QueryRow(`SELECT value FROM library_props WHERE key='source_path'`).Scan(&root)
	timed := func(name string, f func() error) {
		start := time.Now()
		if err := f(); err != nil {
			t.Errorf("%s: %v", name, err)
		}
		t.Logf("%-22s %6d ms", name, time.Since(start).Milliseconds())
	}
	timed("Statistics", func() error { _, err := s.Statistics(""); return err })
	timed("Statistics (root)", func() error { _, err := s.Statistics(root); return err })
	timed("Timeline", func() error { _, err := s.Timeline("", ""); return err })
	timed("ColourSource", func() error { _, err := s.ColourSource(""); return err })
	timed("ColourPoints", func() error { _, err := s.ColourPoints(""); return err })
	timed("FolderStats (root)", func() error { _, err := s.FolderStats(root); return err })
	f := func(v float64) map[string]NumericFilter {
		return map[string]NumericFilter{"FNumber": {Min: v, Max: v + 0.7}}
	}
	for name, o := range map[string]ListPhotosOpts{
		"search: none":       {},
		"search: hue":        {Mono: "colour", HueBin: intp(2)},
		"search: mono":       {Mono: "mono"},
		"search: warm June":  {Mono: "colour", Warmth: "warm", Month: intp(6)},
		"search: model":      {Filters: map[string]string{"Model": "X-T50"}},
		"search: model+lens": {Filters: map[string]string{"Model": "X-T50", "LensModel": "XF23mmF2 R WR"}},
		"search: f-number":   {NumericFilters: f(1.6)},
		"search: focal 35":   {NumericFilters: map[string]NumericFilter{"FocalLength35": {Min: 30, Max: 40}}},
		"search: year":       {DateMin: "2019-01-01", DateMax: "2019-12-31"},
		"search: hour":       {Hour: intp(7)},
		"search: aspect":     {Aspect: "3:2"},
		"search: folder":     {PathPrefix: root},
		"search: film sim":   {Filters: map[string]string{"FilmSimulation": "Classic Chrome"}},
	} {
		o.Limit = 200
		timed(name, func() error { _, err := s.ListPhotos(o); return err })
	}
}
