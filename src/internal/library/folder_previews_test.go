package library

import (
	"reflect"
	"testing"
	"time"
)

func TestFolderPreviews(t *testing.T) {
	s := newTestStore(t)
	now := time.Now()
	add := func(id, path, taken string) {
		t.Helper()
		if err := s.UpsertPhoto(id, path, id+".jpg", 0, now, "", "", taken, "jpeg"); err != nil {
			t.Fatalf("UpsertPhoto %s: %v", id, err)
		}
	}
	// "trip" holds photos of its own; "archive" holds only more folders, so
	// its tile must show what lies further down.
	add("t1", "/root/trip/t1.jpg", "2024-05-01T10:00:00Z")
	add("t2", "/root/trip/t2.jpg", "2024-06-01T10:00:00Z")
	add("t3", "/root/trip/day2/t3.jpg", "2024-07-01T10:00:00Z")
	add("t4", "/root/trip/t4.jpg", "")
	add("t5", "/root/trip/t5.jpg", "2023-01-01T10:00:00Z")
	add("t6", "/root/trip/t6.jpg", "2022-01-01T10:00:00Z")
	add("a1", "/root/archive/2019/scans/a1.jpg", "2019-03-01T10:00:00Z")
	add("x1", "/root/direct.jpg", "2025-01-01T10:00:00Z")   // direct photo: no folder
	add("s1", "/root2/trip/s1.jpg", "2026-01-01T10:00:00Z") // sibling root: not ours

	got, err := s.FolderPreviews("/root")
	if err != nil {
		t.Fatalf("FolderPreviews: %v", err)
	}
	want := []FolderPreview{
		{Name: "archive", PhotoCount: 1, FirstTaken: "2019-03-01T10:00:00Z", LastTaken: "2019-03-01T10:00:00Z", PhotoIDs: []string{"a1"}},
		// Newest by date taken first, undated last, at most four.
		{Name: "trip", PhotoCount: 6, FirstTaken: "2022-01-01T10:00:00Z", LastTaken: "2024-07-01T10:00:00Z", PhotoIDs: []string{"t3", "t2", "t1", "t5"}},
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("FolderPreviews:\n got  %+v\n want %+v", got, want)
	}
}

func TestFolderPreviewsEmpty(t *testing.T) {
	s := newTestStore(t)
	got, err := s.FolderPreviews("/nothing")
	if err != nil {
		t.Fatalf("FolderPreviews: %v", err)
	}
	if got == nil || len(got) != 0 {
		t.Errorf("want an empty, non-nil slice, got %#v", got)
	}
}
