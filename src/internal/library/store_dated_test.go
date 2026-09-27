package library

import (
	"reflect"
	"testing"
)

func datedFixture(t *testing.T) *Store {
	t.Helper()
	s := newTestStore(t)
	seedStatsPhotos(t, s, []statsPhoto{
		{id: "b", path: "/l/b.jpg", date: "2024-05-01T10:30:00", exifJSON: `{"width":6000,"height":4000,"tags":{"Orientation":"6"}}`},
		{id: "a", path: "/l/a.jpg", date: "2023-01-02T08:00:00"},
		{id: "c", path: "/l/c.jpg", date: "2024-05-01T10:30:00"},
		{id: "u", path: "/l/u.jpg"},
		{id: "m", path: "/l/m.jpg", date: "2022-01-01T00:00:00", missing: true},
	})
	return s
}

func TestDatedPhotosSortedWithoutMissingOrUndated(t *testing.T) {
	got, err := datedFixture(t).DatedPhotos()
	if err != nil {
		t.Fatal(err)
	}
	var ids []string
	for _, p := range got {
		ids = append(ids, p.ID)
	}
	if !reflect.DeepEqual(ids, []string{"a", "b", "c"}) {
		t.Fatalf("ids = %v, want a b c", ids)
	}
	if b := got[1]; b.Width != 6000 || b.Height != 4000 || b.Orientation != "6" || b.Filename != "b.jpg" || b.Taken != "2024-05-01T10:30:00" {
		t.Errorf("b = %+v", b)
	}
	if a := got[0]; a.Width != 0 || a.Height != 0 || a.Orientation != "" {
		t.Errorf("a without size = %+v", a)
	}
}

func TestUndatedPhotoIDsLeaveOutMissing(t *testing.T) {
	ids, err := datedFixture(t).UndatedPhotoIDs()
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(ids, []string{"u"}) {
		t.Fatalf("undated = %v, want [u]", ids)
	}
}

func TestContentStampChangesWithPhotos(t *testing.T) {
	s := newTestStore(t)
	empty, err := s.ContentStamp()
	if err != nil {
		t.Fatal(err)
	}
	seedStatsPhotos(t, s, []statsPhoto{{id: "p", path: "/l/p.jpg", date: "2024-01-01T00:00:00"}})
	one, _ := s.ContentStamp()
	if one == empty {
		t.Fatalf("stamp did not change after adding a photo: %q", one)
	}
	if err := s.MarkPhotoMissing("p"); err != nil {
		t.Fatal(err)
	}
	if gone, _ := s.ContentStamp(); gone == one {
		t.Fatalf("stamp did not change after marking missing: %q", gone)
	}
}

func TestContentStampChangesWhenADateChanges(t *testing.T) {
	s := newTestStore(t)
	seedStatsPhotos(t, s, []statsPhoto{{id: "p", path: "/l/p.jpg", date: "2024-01-01T00:00:00"}})
	before, _ := s.ContentStamp()
	if _, err := s.db.Exec(`UPDATE photos SET date_taken='2019-06-01T00:00:00' WHERE id='p'`); err != nil {
		t.Fatal(err)
	}
	if after, _ := s.ContentStamp(); after == before {
		t.Fatalf("stamp %q did not change with the date taken", after)
	}
}
