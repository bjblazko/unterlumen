package library

import "testing"

type photoCols struct {
	width, height *int
	lat, lon      *float64
}

func colsOf(t *testing.T, s *Store, id string) photoCols {
	t.Helper()
	var c photoCols
	if err := s.db.QueryRow(`SELECT width, height, lat, lon FROM photos WHERE id = ?`, id).Scan(&c.width, &c.height, &c.lat, &c.lon); err != nil {
		t.Fatal(err)
	}
	return c
}

// The columns follow the EXIF JSON however a row is written, and an index
// from before them gets them filled once.
func TestPhotoColumnsFollowTheJSON(t *testing.T) {
	s := newTestStore(t)
	seedStatsPhotos(t, s, []statsPhoto{
		{id: "a", path: "/lib/a.jpg", exifJSON: `{"width":4000,"height":3000,"latitude":54.3,"longitude":10.1}`},
		{id: "b", path: "/lib/b.jpg", exifJSON: `not json`},
	})
	if c := colsOf(t, s, "a"); c.width == nil || *c.width != 4000 || *c.height != 3000 || *c.lat != 54.3 || *c.lon != 10.1 {
		t.Errorf("a written: %+v", c)
	}
	if c := colsOf(t, s, "b"); c.width != nil || c.lat != nil {
		t.Errorf("b, no JSON, has columns: %+v", c)
	}
	if _, err := s.db.Exec(`UPDATE photos SET exif_json = '{"width":10,"height":20}' WHERE id = 'a'`); err != nil {
		t.Fatal(err)
	}
	if c := colsOf(t, s, "a"); *c.width != 10 || c.lat != nil {
		t.Errorf("a rewritten: width %v, lat %v; want 10 and no place", *c.width, c.lat)
	}
	// An index from before the columns: they are empty, nothing filled them.
	if _, err := s.db.Exec(`UPDATE photos SET width = NULL, height = NULL, lat = NULL, lon = NULL`); err != nil {
		t.Fatal(err)
	}
	if err := fillPhotoColumns(s.db); err != nil {
		t.Fatal(err)
	}
	if c := colsOf(t, s, "a"); c.width == nil || *c.width != 10 || *c.height != 20 {
		t.Errorf("a filled: %+v", c)
	}
}
