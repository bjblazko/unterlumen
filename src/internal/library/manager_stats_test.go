package library

import (
	"reflect"
	"sort"
	"testing"
)

// managerFixture holds two libraries: the stats fixture, and one more photo.
func managerFixture(t *testing.T) (mgr *Manager, fixtureID, extraID string) {
	t.Helper()
	mgr, err := NewManager(t.TempDir())
	if err != nil {
		t.Fatalf("NewManager: %v", err)
	}
	seed := func(name string, photos []statsPhoto) string {
		l, err := mgr.CreateLibrary(name, "", t.TempDir())
		if err != nil {
			t.Fatalf("CreateLibrary: %v", err)
		}
		s, err := mgr.OpenStore(l.ID)
		if err != nil {
			t.Fatalf("OpenStore: %v", err)
		}
		defer s.Close()
		seedStatsPhotos(t, s, photos)
		return l.ID
	}
	fixtureID = seed("Fixture", statsFixturePhotos())
	extraID = seed("Extra", []statsPhoto{
		{id: "p9", path: "/lib/2024/g.jpg", ext: "jpeg", date: "2024-05-01T10:00:00", size: 5,
			fields:  map[string]string{"FilmSimulation": "Velvia", "FocalLength": "23", "FNumber": "2", "ISOSpeedRatings": "200", "Model": "X-T50", "LensModel": "XF23"},
			numeric: map[string]float64{"FocalLength": 23, "FNumber": 2, "ISOSpeedRatings": 200}},
	})
	return mgr, fixtureID, extraID
}

func TestManagerStatisticsMergesLibraries(t *testing.T) {
	mgr, _, _ := managerFixture(t)
	st, err := mgr.Statistics(nil, "")
	if err != nil {
		t.Fatal(err)
	}
	want := &LibraryStatistics{
		TotalPhotos:    5,
		IndexingPhotos: 1,
		Formats:        []NameCount{{"jpeg", 4}, {"raf", 1}},
		FilmSims:       []NameCount{{"Velvia", 3}, {"None", 2}},
		FocalLengths:   []ValueCount{{23, 3}, {50, 1}},
		FocalLengths35: []ValueCount{{23, 2}, {35, 1}, {50, 1}},
		Apertures:      []ValueCount{{2, 3}, {2.8, 1}},
		ISOs:           []ValueCount{{200, 3}, {800, 1}},
		CameraLens:     []CameraLensCount{{"X-T50", "XF23", 2}, {"X-T50", "(no lens)", 1}, {"X100", "Fixed", 1}},
		ShootingDays:   map[string]int{"2024-05-01": 3, "2023-01-02": 1, "": 1},
	}
	want.ShootingHours[10] = 3
	want.ShootingHours[14] = 1
	if len(st.CameraLens) == 0 || st.CameraLens[0] != want.CameraLens[0] {
		t.Errorf("most used camera/lens = %+v, want %+v first", st.CameraLens, want.CameraLens[0])
	}
	sortTies(st)
	sortTies(want)
	if !reflect.DeepEqual(st, want) {
		t.Errorf("Statistics =\n%+v\nwant\n%+v", st, want)
	}
}

func TestManagerStatisticsOnlyTheRequestedLibraries(t *testing.T) {
	mgr, _, extraID := managerFixture(t)
	st, err := mgr.Statistics([]string{extraID}, "")
	if err != nil {
		t.Fatal(err)
	}
	if st.TotalPhotos != 1 || !reflect.DeepEqual(st.Formats, []NameCount{{"jpeg", 1}}) {
		t.Errorf("Statistics(extra) = %+v, want only the extra library's photo", st)
	}
}

func TestSearchLibrariesMergesNewestFirstAndPages(t *testing.T) {
	mgr, fixtureID, extraID := managerFixture(t)
	res, err := mgr.SearchLibraries(nil, ListPhotosOpts{Limit: 100})
	if err != nil {
		t.Fatal(err)
	}
	var got []string
	for _, p := range res.Results {
		got = append(got, p.ID)
	}
	// Newest first, undated (p5) last; p4 is missing and not listed.
	if want := []string{"p2", "p1", "p9", "p3", "p5"}; !reflect.DeepEqual(got, want) || res.Total != 5 {
		t.Errorf("results = %v (total %d), want %v (total 5)", got, res.Total, want)
	}
	for _, p := range res.Results {
		wantLib := fixtureID
		if p.ID == "p9" {
			wantLib = extraID
		}
		if p.LibraryID != wantLib {
			t.Errorf("%s from library %s, want %s", p.ID, p.LibraryID, wantLib)
		}
	}

	page, err := mgr.SearchLibraries([]string{fixtureID}, ListPhotosOpts{Offset: 1, Limit: 2})
	if err != nil {
		t.Fatal(err)
	}
	got = got[:0]
	for _, p := range page.Results {
		got = append(got, p.ID)
	}
	if want := []string{"p1", "p3"}; !reflect.DeepEqual(got, want) || page.Total != 4 {
		t.Errorf("page = %v (total %d), want %v (total 4)", got, page.Total, want)
	}

	past, err := mgr.SearchLibraries(nil, ListPhotosOpts{Offset: 50, Limit: 10})
	if err != nil {
		t.Fatal(err)
	}
	if past.Results == nil || len(past.Results) != 0 || past.Total != 5 {
		t.Errorf("past the end = %+v, want an empty list and total 5", past)
	}
}

func TestMergeTLs(t *testing.T) {
	a := &LibraryTimeline{
		Granularity: "month",
		Periods:     []string{"2024-01", "2024-02"},
		CameraUsage: []CameraTimeSlice{
			{"A", []int{10, 10}}, {"B", []int{9, 0}}, {"C", []int{0, 8}}, {"D", []int{7, 0}},
		},
		FocalStats:     []PeriodStats{{"2024-01", 20, 10, 30, 2}},
		ISOStats:       []PeriodStats{{"2024-02", 400, 200, 800, 1}},
		ApertureHeat:   []ApertureRow{{"2024-01", map[string]int{"f/2": 1}}},
		AspectRatios:   []AspectSlice{{"4:3", []int{1, 0}}},
		MegapixelStats: []MegapixelStat{{"2024-01", 24, 20, 2}},
	}
	b := &LibraryTimeline{
		Granularity:    "year",
		Periods:        []string{"2024-02", "2024-03"},
		CameraUsage:    []CameraTimeSlice{{"E", []int{6, 0}}, {"F", []int{0, 5}}, {"A", []int{1, 1}}},
		FocalStats:     []PeriodStats{{"2024-01", 40, 20, 60, 2}},
		ApertureHeat:   []ApertureRow{{"2024-01", map[string]int{"f/2": 2, "f/4": 1}}, {"2024-03", map[string]int{"f/8": 1}}},
		AspectRatios:   []AspectSlice{{"3:2", []int{0, 3}}, {"4:3", []int{2, 0}}},
		MegapixelStats: []MegapixelStat{{"2024-01", 40, 40, 2}},
	}
	got := mergeTLs([]*LibraryTimeline{a, b})

	want := &LibraryTimeline{
		Granularity: "year",
		Periods:     []string{"2024-01", "2024-02", "2024-03"},
		// Top five by total — A 22, B 9, C 8, D 7, E 6 — then F as Other.
		CameraUsage: []CameraTimeSlice{
			{"A", []int{10, 11, 1}}, {"B", []int{9, 0, 0}}, {"C", []int{0, 8, 0}},
			{"D", []int{7, 0, 0}}, {"E", []int{0, 6, 0}}, {"Other", []int{0, 0, 5}},
		},
		FocalStats:     []PeriodStats{{"2024-01", 30, 15, 45, 4}},
		ISOStats:       []PeriodStats{{"2024-02", 400, 200, 800, 1}},
		ApertureHeat:   []ApertureRow{{"2024-01", map[string]int{"f/2": 3, "f/4": 1}}, {"2024-03", map[string]int{"f/8": 1}}},
		AspectRatios:   []AspectSlice{{"3:2", []int{0, 0, 3}}, {"4:3", []int{1, 2, 0}}},
		MegapixelStats: []MegapixelStat{{"2024-01", 40, 30, 4}},
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("mergeTLs =\n%+v\nwant\n%+v", got, want)
	}
}

func TestMergeTLsFewCamerasHasNoOther(t *testing.T) {
	a := &LibraryTimeline{Periods: []string{"2024"}, CameraUsage: []CameraTimeSlice{{"A", []int{2}}}}
	b := &LibraryTimeline{Periods: []string{"2024"}, CameraUsage: []CameraTimeSlice{{"B", []int{1}}}}
	got := mergeTLs([]*LibraryTimeline{a, b})
	var names []string
	for _, c := range got.CameraUsage {
		names = append(names, c.Camera)
	}
	sort.Strings(names)
	if !reflect.DeepEqual(names, []string{"A", "B"}) || got.Granularity != "month" {
		t.Errorf("cameras = %v, granularity %q; want A, B and month", names, got.Granularity)
	}
}
