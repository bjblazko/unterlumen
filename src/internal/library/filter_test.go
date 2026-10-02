package library

import (
	"slices"
	"sort"
	"testing"
	"time"
)

// filterLibrary holds three photos: two X-T50 with different lenses, one
// iPhone, from three months.
func filterLibrary(t *testing.T) (*Manager, *Library) {
	t.Helper()
	mgr := newTestManager(t)
	l, _ := mgr.CreateLibrary("F", "", t.TempDir())
	s, _ := mgr.OpenStore(l.ID)
	for _, p := range []struct{ id, date, model, lens string }{
		{"a", "2024-03-10T10:00:00", `"X-T50"`, `"XF23mmF2"`},
		{"b", "2025-01-05T10:00:00", `"iPhone 12 Pro"`, `"iPhone back camera"`},
		{"c", "2024-07-31T23:00:00", `"X-T50"`, `"XF16-50mm"`},
	} {
		if err := s.UpsertPhoto(p.id, "/lib/"+p.id+".jpg", p.id+".jpg", 1, time.Now(), "{}", "", p.date, "jpeg"); err != nil {
			t.Fatal(err)
		}
		for field, value := range map[string]string{"Model": p.model, "LensModel": p.lens} {
			if _, err := s.db.Exec(`INSERT INTO exif_index (photo_id, field, value) VALUES (?,?,?)`, p.id, field, value); err != nil {
				t.Fatal(err)
			}
		}
	}
	return mgr, l
}

func TestFilterNarrowsStatisticsAndSearch(t *testing.T) {
	mgr, l := filterLibrary(t)
	cases := []struct {
		name string
		f    Filter
		want []string
	}{
		{"nothing", Filter{}, []string{"a", "b", "c"}},
		{"a camera", Filter{Models: []string{`"X-T50"`}}, []string{"a", "c"}},
		{"two cameras", Filter{Models: []string{`"X-T50"`, `"iPhone 12 Pro"`}}, []string{"a", "b", "c"}},
		{"a lens", Filter{Lenses: []string{`"XF16-50mm"`}}, []string{"c"}},
		{"months, the last one whole", Filter{From: "2024-04", Until: "2024-07"}, []string{"c"}},
		{"from a month on", Filter{From: "2024-12"}, []string{"b"}},
		{"camera and months", Filter{Models: []string{`"X-T50"`}, Until: "2024-03"}, []string{"a"}},
	}
	for _, c := range cases {
		st, err := mgr.Statistics([]string{l.ID}, "", c.f)
		if err != nil {
			t.Fatal(err)
		}
		if st.TotalPhotos != len(c.want) {
			t.Errorf("%s: statistics count %d photos, want %d", c.name, st.TotalPhotos, len(c.want))
		}
		s, _ := mgr.OpenStore(l.ID)
		got, _ := listedIDs(t, s, ListPhotosOpts{Scope: c.f})
		sort.Strings(got)
		if !slices.Equal(got, c.want) {
			t.Errorf("%s: search found %v, want %v", c.name, got, c.want)
		}
	}
}

func TestScopeValuesListLensesOfTheChosenCameras(t *testing.T) {
	mgr, l := filterLibrary(t)
	all, err := mgr.ScopeValues([]string{l.ID}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(all.Cameras) != 2 || all.Cameras[0].Name != `"X-T50"` || all.Cameras[0].Count != 2 {
		t.Errorf("cameras = %+v, want X-T50 (2) first", all.Cameras)
	}
	if len(all.Lenses) != 3 || all.FirstMonth != "2024-03" || all.LastMonth != "2025-01" {
		t.Errorf("values = %+v", all)
	}
	fuji, _ := mgr.ScopeValues([]string{l.ID}, []string{`"X-T50"`})
	if len(fuji.Lenses) != 2 || len(fuji.Cameras) != 2 {
		t.Errorf("with the X-T50 chosen: lenses %+v, cameras %+v; want its two lenses and still every camera", fuji.Lenses, fuji.Cameras)
	}
}

func TestNextMonth(t *testing.T) {
	for in, want := range map[string]string{"2024-07": "2024-08", "2024-12": "2025-01"} {
		if got := nextMonth(in); got != want {
			t.Errorf("nextMonth(%s) = %s, want %s", in, got, want)
		}
	}
}
