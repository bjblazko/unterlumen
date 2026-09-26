package library

import (
	"reflect"
	"testing"
)

// listFixture is the stats fixture with a few meta keys: p3 rated, p1
// published under the legacy prefix, p2 in the album "Iceland".
func listFixture(t *testing.T) *Store {
	t.Helper()
	s := statsFixture(t)
	for _, m := range [][3]string{{"p3", "rating", "5"}, {"p1", "published:web", "t"}, {"p2", "built:web:title", "Iceland"}} {
		if err := s.UpsertMeta(m[0], m[1], m[2]); err != nil {
			t.Fatalf("UpsertMeta: %v", err)
		}
	}
	return s
}

func listedIDs(t *testing.T, s *Store, opts ListPhotosOpts) ([]string, int) {
	t.Helper()
	if opts.Limit == 0 {
		opts.Limit = 100
	}
	res, err := s.ListPhotos(opts)
	if err != nil {
		t.Fatalf("ListPhotos(%+v): %v", opts, err)
	}
	ids := []string{}
	for _, p := range res.Photos {
		ids = append(ids, p.ID)
	}
	return ids, res.Total
}

func TestListPhotosFilters(t *testing.T) {
	s := listFixture(t)
	cases := []struct {
		name string
		opts ListPhotosOpts
		want []string
	}{
		{"none: newest first, undated last", ListPhotosOpts{}, []string{"p2", "p1", "p3", "p5"}},
		{"one text filter", ListPhotosOpts{Filters: map[string]string{"Model": "X-T50"}}, []string{"p2", "p1"}},
		{"two text filters", ListPhotosOpts{Filters: map[string]string{"Model": "X-T50", "LensModel": "XF23"}}, []string{"p1"}},
		{"numeric range", ListPhotosOpts{NumericFilters: map[string]NumericFilter{"ISOSpeedRatings": {Min: 100, Max: 300}}}, []string{"p2", "p1"}},
		{"numeric and text", ListPhotosOpts{Filters: map[string]string{"LensModel": "XF23"}, NumericFilters: map[string]NumericFilter{"FNumber": {Min: 1, Max: 2}}}, []string{"p1"}},
		{"35mm focal length", ListPhotosOpts{NumericFilters: map[string]NumericFilter{"FocalLength35": {Min: 30, Max: 40}}}, []string{"p1"}},
		{"35mm falls back to focal length", ListPhotosOpts{NumericFilters: map[string]NumericFilter{"FocalLength35": {Min: 20, Max: 25}}}, []string{"p2"}},
		{"date from", ListPhotosOpts{DateMin: "2024-01-01"}, []string{"p2", "p1"}},
		// An undated photo is stored with date_taken '' and passes any
		// upper date bound — characterized as it is.
		{"date until", ListPhotosOpts{DateMax: "2023-12-31"}, []string{"p3", "p5"}},
		{"extension", ListPhotosOpts{ExtFilter: "raf"}, []string{"p2"}},
		{"meta value", ListPhotosOpts{MetaFilters: map[string]string{"rating": "5"}}, []string{"p3"}},
		{"meta key", ListPhotosOpts{MetaExists: []string{"rating"}}, []string{"p3"}},
		{"built: key matches legacy published:", ListPhotosOpts{MetaExists: []string{"built:web"}}, []string{"p1"}},
		{"album title", ListPhotosOpts{AlbumTitle: "Iceland"}, []string{"p2"}},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got, total := listedIDs(t, s, c.opts)
			if !reflect.DeepEqual(got, c.want) || total != len(c.want) {
				t.Errorf("ids = %v (total %d), want %v", got, total, c.want)
			}
		})
	}
}

func TestListPhotosPagesAndKeepsTheTotal(t *testing.T) {
	got, total := listedIDs(t, listFixture(t), ListPhotosOpts{Offset: 1, Limit: 2})
	if !reflect.DeepEqual(got, []string{"p1", "p3"}) || total != 4 {
		t.Errorf("page = %v (total %d), want [p1 p3] (total 4)", got, total)
	}
}

func TestListPhotosCarriesOverlayExif(t *testing.T) {
	res, err := listFixture(t).ListPhotos(ListPhotosOpts{Limit: 100})
	if err != nil {
		t.Fatal(err)
	}
	for _, p := range res.Photos {
		var want map[string]string
		if p.ID == "p1" || p.ID == "p2" {
			want = map[string]string{"FilmSimulation": "Velvia"}
		}
		if !reflect.DeepEqual(p.Exif, want) {
			t.Errorf("%s exif = %v, want %v", p.ID, p.Exif, want)
		}
		if p.ID == "p1" && p.DateTaken != "2024-05-01T10:30:00" {
			t.Errorf("p1 date = %q", p.DateTaken)
		}
	}
}

func TestListPhotosNoMatchIsAnEmptyList(t *testing.T) {
	res, err := listFixture(t).ListPhotos(ListPhotosOpts{ExtFilter: "tiff", Limit: 10})
	if err != nil {
		t.Fatal(err)
	}
	if res.Photos == nil || res.Total != 0 {
		t.Errorf("result = %#v, want an empty list", res)
	}
}
