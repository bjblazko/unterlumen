package library

import "testing"

// What Statistics keep of the faces and things is read again as soon as
// they change, whichever way: here a bare UPDATE, as a pass or a decision
// from the other installation would make it, with no cache emptied.
func TestStatisticsSeeANamingAtOnce(t *testing.T) {
	mgr := newTestManager(t)
	l, err := mgr.CreateLibrary("Fotos", "", t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	s, err := mgr.OpenStore(l.ID)
	if err != nil {
		t.Fatal(err)
	}
	seedStatsPhotos(t, s, []statsPhoto{{id: "a", path: "/lib/a.jpg", ext: "jpeg", date: "2024-05-01T10:00:00"}})
	exec := func(q string) {
		t.Helper()
		if _, err := s.db.Exec(q); err != nil {
			t.Fatalf("%s: %v", q, err)
		}
	}
	exec(`INSERT INTO people (id, name) VALUES (1, 'Ann')`)
	exec(`INSERT INTO faces (photo_id, x, y, w, h, score, embedding, person_id, confirmed) VALUES ('a', 0, 0, 1, 1, 1, x'', 1, 1)`)
	exec(`INSERT INTO photo_things (photo_id, thing, score, source) VALUES ('a', 'dog', 0.9, 'clip')`)
	named := func() string {
		t.Helper()
		ps, err := mgr.PeopleStats(nil, "", "month", Filter{})
		if err != nil || len(ps.People) != 1 {
			t.Fatalf("PeopleStats = %+v, %v", ps, err)
		}
		in, err := mgr.Insights(nil, "", "people", Filter{})
		if err != nil {
			t.Fatal(err)
		}
		if in["personMonth"][ps.People[0].Name+InsightSep+"2024-05"] != 1 {
			t.Errorf("personMonth %v does not name %s", in["personMonth"], ps.People[0].Name)
		}
		return ps.People[0].Name
	}
	thing := func() string {
		t.Helper()
		ts, err := mgr.ThingsStats(nil, "", "month", Filter{})
		if err != nil || len(ts.Things) != 1 {
			t.Fatalf("ThingsStats = %+v, %v", ts, err)
		}
		return ts.Things[0].Name
	}
	if named() != "Ann" || thing() != "dog" {
		t.Fatal("the first reading is wrong")
	}
	exec(`UPDATE people SET name = 'Anna' WHERE id = 1`)
	exec(`UPDATE photo_things SET thing = 'cat'`)
	if got := named(); got != "Anna" {
		t.Errorf("after the renaming Statistics say %q, want Anna", got)
	}
	if got := thing(); got != "cat" {
		t.Errorf("after the change Statistics say %q, want cat", got)
	}
}
