package library

import (
	"reflect"
	"sort"
	"testing"
	"time"
)

type statsPhoto struct {
	id, path, ext, date string
	exifJSON            string
	size                int64
	fields              map[string]string
	numeric             map[string]float64
	missing             bool
}

// statsFixture is a small library: two photos in 2024, one in 2023, one at
// the root without a date, and one missing photo that no statistic counts.
func statsFixture(t *testing.T) *Store {
	t.Helper()
	s := newTestStore(t)
	seedStatsPhotos(t, s, statsFixturePhotos())
	return s
}

func statsFixturePhotos() []statsPhoto {
	return []statsPhoto{
		{id: "p1", path: "/lib/2024/a.jpg", ext: "jpeg", date: "2024-05-01T10:30:00", size: 100, exifJSON: `{"width":6000,"height":4000}`,
			fields:  map[string]string{"FilmSimulation": "Velvia", "FocalLength": "23", "FocalLengthIn35mmFilm": "35", "FNumber": "2", "ISOSpeedRatings": "200", "Model": "X-T50", "LensModel": "XF23"},
			numeric: map[string]float64{"FocalLength": 23, "FocalLengthIn35mmFilm": 35, "FNumber": 2, "ISOSpeedRatings": 200}},
		{id: "p2", path: "/lib/2024/b.raf", ext: "raf", date: "2024-05-01T14:00:00", size: 300,
			fields:  map[string]string{"FilmSimulation": "Velvia", "FocalLength": "23", "FNumber": "2.8", "ISOSpeedRatings": "200", "Model": "X-T50"},
			numeric: map[string]float64{"FocalLength": 23, "FNumber": 2.8, "ISOSpeedRatings": 200}},
		{id: "p3", path: "/lib/2023/c.jpg", ext: "jpeg", date: "2023-01-02T10:05:00", size: 50,
			fields:  map[string]string{"FocalLength": "50", "FNumber": "2", "ISOSpeedRatings": "800", "Model": "X100", "LensModel": "Fixed"},
			numeric: map[string]float64{"FocalLength": 50, "FNumber": 2, "ISOSpeedRatings": 800}},
		{id: "p4", path: "/lib/2023/sub/d.jpg", ext: "jpeg", date: "2023-02-03T09:00:00", size: 999, missing: true},
		{id: "p5", path: "/lib/e.jpg", ext: "jpeg", size: 10},
	}
}

func seedStatsPhotos(t *testing.T, s *Store, photos []statsPhoto) {
	t.Helper()
	for _, p := range photos {
		exifJSON := p.exifJSON
		if exifJSON == "" {
			exifJSON = "{}"
		}
		if err := s.UpsertPhoto(p.id, p.path, p.id+".jpg", p.size, time.Now(), exifJSON, "", p.date, p.ext); err != nil {
			t.Fatalf("UpsertPhoto %s: %v", p.id, err)
		}
		if p.fields != nil {
			if err := s.UpsertExifIndex(p.id, p.fields, p.numeric); err != nil {
				t.Fatalf("UpsertExifIndex %s: %v", p.id, err)
			}
		}
		if p.missing {
			if err := s.MarkPhotoMissing(p.id); err != nil {
				t.Fatalf("MarkPhotoMissing %s: %v", p.id, err)
			}
		}
	}
}

// sortTies orders the lists whose SQL order is undefined among equal counts.
func sortTies(st *LibraryStatistics) {
	sort.Slice(st.Formats, func(i, j int) bool { return st.Formats[i].Name < st.Formats[j].Name })
	sort.Slice(st.CameraLens, func(i, j int) bool {
		a, b := st.CameraLens[i], st.CameraLens[j]
		return a.Camera+"|"+a.Lens < b.Camera+"|"+b.Lens
	})
}

func TestStatisticsWholeLibrary(t *testing.T) {
	st, err := statsFixture(t).Statistics("")
	if err != nil {
		t.Fatal(err)
	}
	want := &LibraryStatistics{
		TotalPhotos:    4,
		IndexingPhotos: 1,
		Formats:        []NameCount{{"jpeg", 3}, {"raf", 1}},
		FilmSims:       []NameCount{{"Velvia", 2}, {"None", 2}},
		FocalLengths:   []ValueCount{{23, 2}, {50, 1}},
		FocalLengths35: []ValueCount{{23, 1}, {35, 1}, {50, 1}},
		Apertures:      []ValueCount{{2, 2}, {2.8, 1}},
		ISOs:           []ValueCount{{200, 2}, {800, 1}},
		CameraLens:     []CameraLensCount{{"X-T50", "(no lens)", 1}, {"X-T50", "XF23", 1}, {"X100", "Fixed", 1}},
		// A photo without a date is stored with date_taken '' and counts as
		// the day "" — characterized as it is.
		ShootingDays: map[string]int{"2024-05-01": 2, "2023-01-02": 1, "": 1},
	}
	want.ShootingHours[10] = 2
	want.ShootingHours[14] = 1
	sortTies(st)
	if !reflect.DeepEqual(st, want) {
		t.Errorf("Statistics(\"\") =\n%+v\nwant\n%+v", st, want)
	}
}

func TestStatisticsPathPrefix(t *testing.T) {
	st, err := statsFixture(t).Statistics("/lib/2024")
	if err != nil {
		t.Fatal(err)
	}
	want := &LibraryStatistics{
		TotalPhotos:    2,
		Formats:        []NameCount{{"jpeg", 1}, {"raf", 1}},
		FilmSims:       []NameCount{{"Velvia", 2}},
		FocalLengths:   []ValueCount{{23, 2}},
		FocalLengths35: []ValueCount{{23, 1}, {35, 1}},
		Apertures:      []ValueCount{{2, 1}, {2.8, 1}},
		ISOs:           []ValueCount{{200, 2}},
		CameraLens:     []CameraLensCount{{"X-T50", "(no lens)", 1}, {"X-T50", "XF23", 1}},
		ShootingDays:   map[string]int{"2024-05-01": 2},
	}
	want.ShootingHours[10] = 1
	want.ShootingHours[14] = 1
	sortTies(st)
	if !reflect.DeepEqual(st, want) {
		t.Errorf("Statistics(\"/lib/2024\") =\n%+v\nwant\n%+v", st, want)
	}
}

func TestStatisticsEmptyLibraryHasEmptyListsNotNil(t *testing.T) {
	st, err := newTestStore(t).Statistics("")
	if err != nil {
		t.Fatal(err)
	}
	if st.Formats == nil || st.FilmSims == nil || st.FocalLengths == nil || st.FocalLengths35 == nil ||
		st.Apertures == nil || st.ISOs == nil || st.CameraLens == nil || st.ShootingDays == nil {
		t.Errorf("empty library returned a nil list: %+v", st)
	}
}

func TestFolderStats(t *testing.T) {
	st, err := statsFixture(t).FolderStats("/lib")
	if err != nil {
		t.Fatal(err)
	}
	want := &LibraryFolderStats{
		PhotoCount: 4,
		TotalSize:  460,
		Formats:    []NameCount{{"jpeg", 3}, {"raf", 1}},
		Subfolders: []LibSubfolder{{"2023", 1, 50}, {"2024", 2, 400}},
		DateFirst:  "", // the undated photo's '' sorts first
		DateLast:   "2024-05-01T14:00:00",
	}
	if !reflect.DeepEqual(st, want) {
		t.Errorf("FolderStats(\"/lib\") =\n%+v\nwant\n%+v", st, want)
	}
}

func browsedIDs(r FolderBrowseResult) []string {
	ids := []string{}
	for _, p := range r.Photos {
		ids = append(ids, p.ID)
	}
	sort.Strings(ids)
	return ids
}

func TestBrowseFolderListsDirectPhotosAndSubfolders(t *testing.T) {
	s := statsFixture(t)
	root, err := s.BrowseFolder("/lib")
	if err != nil {
		t.Fatal(err)
	}
	if got := browsedIDs(root); !reflect.DeepEqual(got, []string{"p5"}) || root.Total != 1 {
		t.Errorf("photos = %v (total %d), want [p5]", got, root.Total)
	}
	if !reflect.DeepEqual(root.Subfolders, []string{"2023", "2024"}) {
		t.Errorf("subfolders = %v, want [2023 2024]", root.Subfolders)
	}
	if root.Photos[0].Exif != nil {
		t.Errorf("p5 has no overlay data, got %v", root.Photos[0].Exif)
	}

	year, err := s.BrowseFolder("/lib/2024")
	if err != nil {
		t.Fatal(err)
	}
	if got := browsedIDs(year); !reflect.DeepEqual(got, []string{"p1", "p2"}) {
		t.Errorf("photos = %v, want [p1 p2]", got)
	}
	if year.Subfolders == nil || len(year.Subfolders) != 0 {
		t.Errorf("subfolders = %#v, want an empty list", year.Subfolders)
	}
	for _, p := range year.Photos {
		want := map[string]string{"FilmSimulation": "Velvia"}
		if p.ID == "p1" {
			want["AspectRatio"] = "3:2"
		}
		if !reflect.DeepEqual(p.Exif, want) {
			t.Errorf("%s overlay = %v, want %v", p.ID, p.Exif, want)
		}
	}
}

func TestBrowseFolderOfAnEmptyFolderHasEmptyLists(t *testing.T) {
	r, err := statsFixture(t).BrowseFolder("/nowhere")
	if err != nil {
		t.Fatal(err)
	}
	if r.Photos == nil || r.Subfolders == nil || r.Total != 0 {
		t.Errorf("result = %#v, want empty lists", r)
	}
}
