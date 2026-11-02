package library

import (
	"context"
	"slices"
	"strings"
	"testing"
)

// Weekday and hour, countries and cities, and completeness (the
// Statistics views of 2026-11-02).
func TestStatisticsCountWeekHoursPlacesAndCompleteness(t *testing.T) {
	s := statsFixture(t)
	s.UpsertExifIndex("p1", map[string]string{"Model": "X-T50", "Country": "Deutschland", "City": "Kiel", "Keywords": "sea; harbour", "Title": "Kai"}, nil) //nolint:errcheck
	s.UpsertExifIndex("p2", map[string]string{"Country": "Deutschland", "Rating": "4"}, nil)                                                                //nolint:errcheck
	st, err := s.Statistics("")
	if err != nil {
		t.Fatal(err)
	}
	// 2024-05-01 is a Wednesday (2), 2023-01-02 a Monday (0).
	if st.WeekHours[2][10] != 1 || st.WeekHours[2][14] != 1 || st.WeekHours[0][10] != 1 {
		t.Errorf("week hours %v", st.WeekHours)
	}
	if !slices.Equal(st.Countries, []NameCount{{"Deutschland", 2}}) || !slices.Equal(st.Cities, []NameCount{{"Kiel", 1}}) {
		t.Errorf("countries %v, cities %v", st.Countries, st.Cities)
	}
	want := map[string]int{"title": 1, "keywords": 1, "country": 2, "rating": 1, "description": 0, "location": 0, "people": 0}
	for _, c := range st.Completeness {
		if c.Count != want[c.Name] {
			t.Errorf("%s: %d photos, want %d", c.Name, c.Count, want[c.Name])
		}
	}
	if len(st.Completeness) != len(CompletenessFields) {
		t.Errorf("completeness %v", st.Completeness)
	}
}

func TestLacksAndWeekdayFindTheirPhotos(t *testing.T) {
	s := statsFixture(t)
	s.UpsertExifIndex("p1", map[string]string{"Country": "Deutschland"}, nil) //nolint:errcheck
	ids, _ := listedIDs(t, s, ListPhotosOpts{Lacks: []string{"country"}})
	slices.Sort(ids)
	if !slices.Equal(ids, []string{"p2", "p3", "p5"}) {
		t.Errorf("without a country: %v", ids)
	}
	wed, ten := 2, 10
	if ids, _ := listedIDs(t, s, ListPhotosOpts{Weekday: &wed, Hour: &ten}); !slices.Equal(ids, []string{"p1"}) {
		t.Errorf("Wednesday at ten: %v", ids)
	}
}

func TestLacksAndWeekdayAreNotCorrelated(t *testing.T) {
	s := statsFixture(t)
	wd := 0
	for _, field := range CompletenessFields {
		fromSQL, whereSQL, args := newPhotoFilter(ListPhotosOpts{Lacks: []string{field}, Weekday: &wd}).sql()
		rows, err := s.db.QueryContext(context.Background(), `EXPLAIN QUERY PLAN SELECT COUNT(*) `+fromSQL+` WHERE `+whereSQL, args...)
		if err != nil {
			t.Fatal(err)
		}
		var plan []string
		for rows.Next() {
			var id, parent, unused int
			var detail string
			rows.Scan(&id, &parent, &unused, &detail) //nolint:errcheck
			plan = append(plan, detail)
		}
		rows.Close()
		if joined := strings.Join(plan, "; "); strings.Contains(joined, "CORRELATED") {
			t.Errorf("lacks %s runs once per photo: %s", field, joined)
		}
	}
}
