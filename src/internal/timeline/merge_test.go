package timeline

import (
	"reflect"
	"testing"

	"huepattl.de/unterlumen/internal/library"
)

func ids(photos []Photo) []string {
	out := []string{}
	for _, p := range photos {
		out = append(out, p.ID)
	}
	return out
}

func TestMergeSortsAndKeepsEachPhotoOnce(t *testing.T) {
	photos, undated := merge([]LibraryPhotos{
		{LibraryID: "L1", Dated: []library.DatedPhoto{
			{ID: "x", Taken: "2024-05-01T10:00:00"}, {ID: "shared", Taken: "2021-02-03T09:00:00"},
		}, Undated: []string{"u1", "u2"}},
		{LibraryID: "L2", Dated: []library.DatedPhoto{
			{ID: "shared", Taken: "2021-02-03T09:00:00"}, {ID: "a", Taken: "2020-01-01T00:00:00"},
		}, Undated: []string{"u2"}},
	})
	if got := ids(photos); !reflect.DeepEqual(got, []string{"a", "shared", "x"}) {
		t.Fatalf("order = %v", got)
	}
	if photos[1].LibraryID != "L1" {
		t.Errorf("shared photo came from %s, want the first library L1", photos[1].LibraryID)
	}
	if undated != 2 {
		t.Errorf("undated = %d, want 2 (u2 counted once)", undated)
	}
}

func TestMergeTreatsUnreadableDatesAsUndated(t *testing.T) {
	photos, undated := merge([]LibraryPhotos{{LibraryID: "L", Dated: []library.DatedPhoto{
		{ID: "ok", Taken: "2019-08-14T12:00:00"},
		{ID: "zero", Taken: "0000:00:00 00:00:00"},
		{ID: "short", Taken: "2019"},
		{ID: "ancient", Taken: "0001-01-01T00:00:00"},
	}}})
	if got := ids(photos); !reflect.DeepEqual(got, []string{"ok"}) {
		t.Fatalf("dated = %v, want only ok", got)
	}
	if undated != 3 {
		t.Errorf("undated = %d, want 3", undated)
	}
}

func TestAssignDaysCountsFromTheFirstPhoto(t *testing.T) {
	photos := []Photo{{Taken: "2020-02-28T23:00:00"}, {Taken: "2020-03-01T01:00:00"}, {Taken: "2021-02-28T00:00:00"}}
	start := assignDays(photos)
	if start != "2020-02-28" {
		t.Errorf("start = %q", start)
	}
	var days []int
	for _, p := range photos {
		days = append(days, p.Day)
	}
	if !reflect.DeepEqual(days, []int{0, 2, 366}) {
		t.Errorf("days = %v, want [0 2 366] (2020 is a leap year)", days)
	}
}
