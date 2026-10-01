package library

import (
	"slices"
	"sort"
	"strings"
	"testing"
	"time"
)

type measuredPhoto struct {
	id, path, date, mono string
	warmth               float64
	swatches             []ColourSwatch
}

// seedMeasured indexes photos and stores their measurements directly.
func seedMeasured(t *testing.T, s *Store, photos []measuredPhoto) {
	t.Helper()
	for _, p := range photos {
		if err := s.UpsertPhoto(p.id, p.path, p.id+".jpg", 1, time.Now(), "{}", "", p.date, "jpeg"); err != nil {
			t.Fatal(err)
		}
		if p.mono == "" {
			continue
		}
		if _, err := s.db.Exec(`INSERT INTO photo_appearance (photo_id, version, analysed_at, mono_class, avg_l, avg_c, avg_h, warmth, tone_key)
			VALUES (?, 1, ?, ?, 0.5, 0.1, 60, ?, 'normal')`, p.id, time.Now(), p.mono, p.warmth); err != nil {
			t.Fatal(err)
		}
		for rank, sw := range p.swatches {
			var bin any
			if sw.HueBin >= 0 {
				bin = sw.HueBin
			}
			if _, err := s.db.Exec(`INSERT INTO photo_palette (photo_id, rank, l, c, h, hue_bin, share) VALUES (?,?,?,?,?,?,?)`,
				p.id, rank, sw.Colour.L, sw.Colour.C, sw.Colour.H, bin, sw.Share); err != nil {
				t.Fatal(err)
			}
		}
	}
}

func colourStore(t *testing.T) *Store {
	t.Helper()
	s := newTestStore(t)
	seedMeasured(t, s, []measuredPhoto{
		{id: "warm", path: "/lib/2024/a.jpg", date: "2024-07-01T10:00:00", mono: "colour", warmth: 0.6,
			swatches: []ColourSwatch{{HueBin: 2, Share: 0.7}, {HueBin: 8, Share: 0.1}, {HueBin: -1, Share: 0.2}}},
		{id: "cool", path: "/lib/2024/b.jpg", date: "2023-07-02T10:00:00", mono: "colour", warmth: -0.3,
			swatches: []ColourSwatch{{HueBin: 8, Share: 0.9}}},
		{id: "edge", path: "/lib/2024/c.jpg", date: "2024-08-01T10:00:00", mono: "colour", warmth: 0.15,
			swatches: []ColourSwatch{{HueBin: 2, Share: 0.2}}},
		{id: "bw", path: "/lib/old/d.jpg", date: "2024-07-03T10:00:00", mono: "mono",
			swatches: []ColourSwatch{{HueBin: -1, Share: 1}}},
		{id: "new", path: "/lib/old/e.jpg", date: "2024-07-04T10:00:00"}, // not measured yet
	})
	return s
}

func TestColourSource(t *testing.T) {
	src, err := colourStore(t).ColourSource("")
	if err != nil {
		t.Fatal(err)
	}
	if len(src.Photos) != 4 || src.Unanalysed != 1 {
		t.Errorf("photos %d, unanalysed %d", len(src.Photos), src.Unanalysed)
	}
	// The black-and-white photo's grey swatch is left out.
	if len(src.Swatches) != 5 {
		t.Errorf("swatches %d, want 5 of the colour photos", len(src.Swatches))
	}
	scoped, err := colourStore(t).ColourSource("/lib/old")
	if err != nil {
		t.Fatal(err)
	}
	if len(scoped.Photos) != 1 || scoped.Unanalysed != 1 {
		t.Errorf("scoped photos %d, unanalysed %d", len(scoped.Photos), scoped.Unanalysed)
	}
}

func intp(n int) *int { return &n }

// A click shows exactly the photos the chart counted.
func TestListPhotosColourFilters(t *testing.T) {
	s := colourStore(t)
	cases := []struct {
		name string
		opts ListPhotosOpts
		want []string
	}{
		{"mono", ListPhotosOpts{Mono: "mono"}, []string{"bw"}},
		{"hue covers 20 %", ListPhotosOpts{HueBin: intp(2)}, []string{"edge", "warm"}},
		{"hue below 20 % is not a main colour", ListPhotosOpts{HueBin: intp(8)}, []string{"cool"}},
		{"warm, beyond the threshold only", ListPhotosOpts{Warmth: "warm"}, []string{"warm"}},
		{"cool", ListPhotosOpts{Warmth: "cool"}, []string{"cool"}},
		{"July of any year", ListPhotosOpts{Month: intp(7), Mono: "colour"}, []string{"cool", "warm"}},
	}
	for _, c := range cases {
		got, _ := listedIDs(t, s, c.opts)
		sort.Strings(got)
		if !slices.Equal(got, c.want) {
			t.Errorf("%s: got %v, want %v", c.name, got, c.want)
		}
	}
}

// The hue filter must collect its photos once, not search the swatches of
// the hue for every photo: as a correlated subquery it ran for minutes on
// the owner's 48,000 photos and blocked the app.
func TestHueFilterIsNotCorrelated(t *testing.T) {
	s := colourStore(t)
	from, where, args := newPhotoFilter(ListPhotosOpts{HueBin: intp(2)}).sql()
	rows, err := s.db.Query(`EXPLAIN QUERY PLAN SELECT COUNT(p.id) `+from+` WHERE `+where, args...)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	var plan []string
	for rows.Next() {
		var id, parent, unused int
		var detail string
		if err := rows.Scan(&id, &parent, &unused, &detail); err != nil {
			t.Fatal(err)
		}
		plan = append(plan, detail)
	}
	// SQLite writes a per-photo search as "CORRELATED SCALAR SUBQUERY" or
	// "SEARCH pp EXISTS …" depending on its version; the list built once
	// is "LIST SUBQUERY" in both.
	joined := strings.Join(plan, "\n")
	if !strings.Contains(joined, "LIST SUBQUERY") || strings.Contains(joined, "CORRELATED") || strings.Contains(joined, " EXISTS ") {
		t.Errorf("hue filter runs per photo:\n%s", joined)
	}
}
