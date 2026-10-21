package library

import "testing"

// Across libraries, the cover chosen last shows a person; where nobody
// chose one, the library with most of their faces.
func TestTheCoverChosenLastShowsAPerson(t *testing.T) {
	ins := []NamedIn{
		{Library: "big", Faces: 9, Cover: 1},
		{Library: "small", Faces: 2, Cover: 2},
		{Library: "smallest", Faces: 1, Cover: 3},
	}
	if got := coverAmong(ins); got != (CoverRef{"big", 1}) {
		t.Errorf("nobody chose: %+v, want the library with most faces", got)
	}
	ins[2].CoverChosen = "2026-10-01T10:00:00.000000000Z"
	ins[1].CoverChosen = "2026-10-21T10:00:00.000000000Z"
	if got := coverAmong(ins); got != (CoverRef{"small", 2}) {
		t.Errorf("two chosen: %+v, want the one chosen last", got)
	}
	if got := coverAmong(nil); got != (CoverRef{}) {
		t.Errorf("nobody: %+v", got)
	}
}

// A person's faces from all libraries are one list, from the newest photo
// to the oldest, those without a date last.
func TestFacesAcrossLibrariesComeNewestFirst(t *testing.T) {
	fs := []FaceIn{
		{FaceRef{ID: 1, Date: "2020-01-01T00:00:00"}, "a"},
		{FaceRef{ID: 2}, "a"},
		{FaceRef{ID: 1, Date: "2025-01-01T00:00:00"}, "b"},
		{FaceRef{ID: 3, Date: "2020-01-01T00:00:00"}, "b"},
	}
	newestFirst(fs)
	var got []string
	for _, f := range fs {
		got = append(got, f.Library+":"+f.Date)
	}
	want := []string{"b:2025-01-01T00:00:00", "a:2020-01-01T00:00:00", "b:2020-01-01T00:00:00", "a:"}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("order %v, want %v", got, want)
		}
	}
}
