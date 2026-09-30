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
		ShootingDays:   map[string]int{"2024-05-01": 3, "2023-01-02": 1},
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
		// Every camera, summed; topCameras cuts the list afterwards.
		CameraUsage: []CameraTimeSlice{
			{"A", []int{10, 11, 1}}, {"B", []int{9, 0, 0}}, {"C", []int{0, 8, 0}},
			{"D", []int{7, 0, 0}}, {"E", []int{0, 6, 0}}, {"F", []int{0, 0, 5}},
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

// A camera whose counts line up with no period is left out.
func TestMergeTLsCameraWithoutCountsIsLeftOut(t *testing.T) {
	a := &LibraryTimeline{Periods: []string{"2024"}, CameraUsage: []CameraTimeSlice{{"A", []int{2}}}}
	b := &LibraryTimeline{Periods: []string{"2024"}, CameraUsage: []CameraTimeSlice{{"Z", []int{}}}}
	got := mergeTLs([]*LibraryTimeline{a, b}).CameraUsage
	want := []CameraTimeSlice{{"A", []int{2}}}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("cameras = %+v, want %+v", got, want)
	}
}

// Five cameras by their total, the rest summed as Other.
func TestTopCameras(t *testing.T) {
	in := []CameraTimeSlice{
		{"F", []int{0, 5}}, {"A", []int{10, 12}}, {"B", []int{9, 0}},
		{"C", []int{0, 8}}, {"D", []int{7, 0}}, {"E", []int{0, 6}}, {"G", []int{1, 1}},
	}
	got := topCameras(in, 2)
	want := []CameraTimeSlice{
		{"A", []int{10, 12}}, {"B", []int{9, 0}}, {"C", []int{0, 8}},
		{"D", []int{7, 0}}, {"E", []int{0, 6}}, {"Other", []int{1, 6}},
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("topCameras =\n%+v\nwant\n%+v", got, want)
	}
	if few := topCameras(in[:2], 2); len(few) != 2 || few[0].Camera != "A" {
		t.Errorf("two cameras = %+v, want A then F and no Other", few)
	}
}

// A library hands over every camera. It used to cut its own list to five
// plus "Other", and merging two libraries then ranked that "Other" as a
// camera: the chart showed "Other" twice.
func TestAssembleCameraSlicesKeepsEveryCamera(t *testing.T) {
	var rows []tlCameraRow
	for i, camera := range []string{"A", "B", "C", "D", "E", "F", "G"} {
		rows = append(rows, tlCameraRow{period: "2024", camera: camera, count: 10 - i})
	}
	got := assembleCameraSlices(rows, []string{"2024"}, map[string]int{"2024": 0})
	if len(got) != 7 {
		t.Fatalf("got %d cameras, want all 7", len(got))
	}
	for _, cs := range got {
		if cs.Camera == "Other" {
			t.Errorf("a library summed cameras as Other: %+v", got)
		}
	}
}

// Cameras are ranked by what the libraries hold together.
func TestManagerTimelineRanksCamerasAcrossLibraries(t *testing.T) {
	a := &LibraryTimeline{Periods: []string{"2024"}, CameraUsage: []CameraTimeSlice{
		{"A", []int{9}}, {"B", []int{8}}, {"C", []int{7}}, {"D", []int{6}}, {"E", []int{5}}, {"X", []int{4}},
	}}
	b := &LibraryTimeline{Periods: []string{"2024"}, CameraUsage: []CameraTimeSlice{
		{"X", []int{5}}, {"Y", []int{1}},
	}}
	merged := mergeTLs([]*LibraryTimeline{a, b})
	got := topCameras(merged.CameraUsage, len(merged.Periods))
	var names []string
	for _, c := range got {
		names = append(names, c.Camera)
	}
	if want := []string{"A", "X", "B", "C", "D", "Other"}; !reflect.DeepEqual(names, want) {
		t.Errorf("cameras = %v, want %v", names, want)
	}
	if other := got[len(got)-1].Counts[0]; other != 6 {
		t.Errorf("Other = %d, want E 5 + Y 1 = 6", other)
	}
}
