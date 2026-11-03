package library

import (
	"context"
	"reflect"
	"strings"
	"testing"
)

func memoriesFixture(t *testing.T) *Store {
	t.Helper()
	s := newTestStore(t)
	seedStatsPhotos(t, s, []statsPhoto{
		{id: "m1", path: "/lib/a.jpg", ext: "jpeg", date: "2022-11-03T10:00:00", exifJSON: `{"width":6000,"height":4000,"latitude":38.7,"longitude":-9.1,"tags":{"Orientation":"6"}}`,
			fields: map[string]string{"Country": `"Portugal"`}},
		{id: "m2", path: "/lib/b.jpg", ext: "jpeg", date: "2023-11-03T10:00:00"},
		{id: "undated", path: "/lib/c.jpg", ext: "jpeg"},
		{id: "film", path: "/lib/d.mov", ext: "mov", date: "2022-11-03T11:00:00"},
		{id: "gone", path: "/lib/e.jpg", ext: "jpeg", date: "2022-11-03T12:00:00", missing: true},
	})
	exec := func(q string, args ...any) {
		t.Helper()
		if _, err := s.db.Exec(q, args...); err != nil {
			t.Fatalf("%s: %v", q, err)
		}
	}
	exec(`INSERT INTO people (id, name) VALUES (1, 'Ben'), (2, 'Anna')`)
	for _, f := range []struct {
		photo    string
		person   any
		confirmd int
	}{{"m1", 1, 1}, {"m1", 1, 1}, {"m1", 2, 1}, {"m1", nil, 1}, {"m1", 2, 0}, {"m2", 1, 1}} {
		exec(`INSERT INTO faces (photo_id, x, y, w, h, score, embedding, person_id, confirmed) VALUES (?, 0, 0, 1, 1, 1, x'', ?, ?)`, f.photo, f.person, f.confirmd)
	}
	exec(`INSERT INTO photo_things (photo_id, thing, score, source) VALUES ('m1', 'sea', 0.9, 'clip'), ('m1', 'boat', 0.8, 'clip')`)
	exec(`INSERT INTO photo_appearance (photo_id, version, analysed_at, mono_class, tone_key, dhash) VALUES ('m1', 1, datetime('now'), 'colour', 'low', -5)`)
	exec(`INSERT INTO photo_palette (photo_id, rank, l, c, h, hue_bin, share) VALUES ('m1', 0, 0.5, 0.1, 200, 7, 0.6), ('m1', 1, 0.5, 0.1, 20, 1, 0.2)`)
	return s
}

func memoryByID(t *testing.T, photos []MemoryPhoto) map[string]MemoryPhoto {
	t.Helper()
	out := map[string]MemoryPhoto{}
	for _, p := range photos {
		out[p.ID] = p
	}
	return out
}

func TestMemoryPhotosGathersWhatAPhotoShows(t *testing.T) {
	s := memoriesFixture(t)
	photos, err := s.MemoryPhotos(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	m1 := memoryByID(t, photos)["m1"]
	want := MemoryPhoto{ID: "m1", Filename: "m1.jpg", Taken: "2022-11-03T10:00:00", Width: 6000, Height: 4000, Orientation: "6",
		Lat: 38.7, Lon: -9.1, Located: true, Country: "Portugal", Things: []string{"boat", "sea"}, People: []string{"Anna", "Ben"},
		HueBin: 7, Tone: "low", DHash: uint64(0xFFFFFFFFFFFFFFFB), Hashed: true}
	if !reflect.DeepEqual(m1, want) {
		t.Errorf("m1 =\n %+v\nwant\n %+v", m1, want)
	}
	m2 := memoryByID(t, photos)["m2"]
	if m2.HueBin != -1 || m2.Hashed || m2.Located || m2.Tone != "" || len(m2.Things) != 0 || !reflect.DeepEqual(m2.People, []string{"Ben"}) {
		t.Errorf("m2 = %+v; want no hue (-1), no hash, no place, Ben", m2)
	}
}

func TestMemoryPhotosLeavesOut(t *testing.T) {
	s := memoriesFixture(t)
	photos, err := s.MemoryPhotos(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	got := memoryByID(t, photos)
	for _, id := range []string{"undated", "film", "gone"} {
		if _, ok := got[id]; ok {
			t.Errorf("%s is among the memory photos", id)
		}
	}
	if len(got) != 2 {
		t.Errorf("got %d photos, want m1 and m2", len(got))
	}
	s.filter = Filter{People: []string{"Anna"}}
	photos, err = s.MemoryPhotos(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(photos) != 1 || photos[0].ID != "m1" {
		t.Errorf("filtered to Anna: %+v; want m1 only", photos)
	}
}

func TestMemoryPhotosIsNotCorrelated(t *testing.T) {
	s := memoriesFixture(t)
	s.filter = Filter{People: []string{"Anna"}, Things: []string{"sea"}}
	for _, q := range memoryQueries {
		sql, args := s.memoryQuery(q)
		rows, err := s.db.Query(`EXPLAIN QUERY PLAN `+sql, args...)
		if err != nil {
			t.Fatalf("%s: %v", sql, err)
		}
		var plan []string
		for rows.Next() {
			var id, parent, unused int
			var detail string
			if err := rows.Scan(&id, &parent, &unused, &detail); err != nil {
				t.Fatal(err)
			}
			plan = append(plan, detail)
		}
		rows.Close()
		if joined := strings.Join(plan, "; "); strings.Contains(joined, "CORRELATED") {
			t.Errorf("runs once per photo: %s\n%s", sql, joined)
		}
	}
}

func TestManagerMemoryPhotosOnceAcrossLibraries(t *testing.T) {
	mgr := newTestManager(t)
	var stores []*Store
	for _, name := range []string{"A", "B"} {
		l, err := mgr.CreateLibrary(name, "", t.TempDir())
		if err != nil {
			t.Fatal(err)
		}
		s, err := mgr.OpenStore(l.ID)
		if err != nil {
			t.Fatal(err)
		}
		defer s.Close()
		seedStatsPhotos(t, s, []statsPhoto{{id: "same", path: "/x/same.jpg", ext: "jpeg", date: "2020-01-01T10:00:00"}})
		stores = append(stores, s)
	}
	ctx := context.Background()
	photos, v1, err := mgr.MemoryPhotos(ctx, nil, Filter{})
	if err != nil {
		t.Fatal(err)
	}
	if len(photos) != 1 || photos[0].LibraryID == "" {
		t.Fatalf("photos = %+v; want one, with its library", photos)
	}
	if _, v, _ := mgr.MemoryPhotos(ctx, nil, Filter{}); v != v1 {
		t.Errorf("version moved without a change: %s → %s", v1, v)
	}
	for _, s := range stores { // whichever library is listed first
		if _, err := s.db.Exec(`INSERT INTO photo_things (photo_id, thing, score, source) VALUES ('same', 'dog', 0.9, 'clip')`); err != nil {
			t.Fatal(err)
		}
	}
	photos, v2, _ := mgr.MemoryPhotos(ctx, nil, Filter{})
	if v2 == v1 {
		t.Error("version did not move after a thing was found")
	}
	if len(photos) != 1 || len(photos[0].Things) != 1 {
		t.Errorf("after the change: %+v; want the new thing", photos)
	}
	seedStatsPhotos(t, stores[1], []statsPhoto{{id: "new", path: "/x/new.jpg", ext: "jpeg", date: "2021-01-01T10:00:00"}})
	if _, v3, _ := mgr.MemoryPhotos(ctx, nil, Filter{}); v3 == v2 {
		t.Error("version did not move after a photo was added")
	}
}
