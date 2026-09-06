package channels

import (
	"path/filepath"
	"testing"
)

func newTestDraftStore(t *testing.T) *DraftStore {
	t.Helper()
	dir := t.TempDir()
	return NewDraftStore(NewStore(dir, dir))
}

func TestDraftStore_CreateAndList(t *testing.T) {
	s := newTestDraftStore(t)
	d, err := s.Create("website", DraftTarget{Title: "Summer 2026"}, []DraftPhoto{{LibraryID: "lib1", PhotoID: "p1"}})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if d.ID == "" {
		t.Fatal("expected a non-empty draft ID")
	}
	drafts, err := s.List("website")
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(drafts) != 1 || len(drafts[0].Photos) != 1 {
		t.Fatalf("expected 1 draft with 1 photo, got %+v", drafts)
	}
	if drafts[0].Target.Title != "Summer 2026" {
		t.Fatalf("target title = %q, want %q", drafts[0].Target.Title, "Summer 2026")
	}
}

func TestDraftStore_AppendPhotos(t *testing.T) {
	s := newTestDraftStore(t)
	d, _ := s.Create("website", DraftTarget{Title: "Summer 2026"}, []DraftPhoto{{LibraryID: "lib1", PhotoID: "p1"}})
	updated, err := s.AppendPhotos("website", d.ID, []DraftPhoto{{LibraryID: "lib1", PhotoID: "p2"}, {LibraryID: "lib2", PhotoID: "p3"}})
	if err != nil {
		t.Fatalf("AppendPhotos: %v", err)
	}
	if len(updated.Photos) != 3 {
		t.Fatalf("expected 3 photos after append, got %d", len(updated.Photos))
	}
	if _, err := s.AppendPhotos("website", "does-not-exist", nil); err == nil {
		t.Fatal("expected error appending to a missing draft")
	}
}

func TestDraftStore_RemovePhoto(t *testing.T) {
	s := newTestDraftStore(t)
	d, _ := s.Create("website", DraftTarget{Title: "Summer 2026"}, []DraftPhoto{
		{LibraryID: "lib1", PhotoID: "p1"},
		{LibraryID: "lib1", PhotoID: "p2"},
	})

	remaining, err := s.RemovePhoto("website", d.ID, "lib1", "p1")
	if err != nil {
		t.Fatalf("RemovePhoto: %v", err)
	}
	if remaining == nil || len(remaining.Photos) != 1 || remaining.Photos[0].PhotoID != "p2" {
		t.Fatalf("expected 1 photo (p2) remaining, got %+v", remaining)
	}

	remaining, err = s.RemovePhoto("website", d.ID, "lib1", "p2")
	if err != nil {
		t.Fatalf("RemovePhoto (last): %v", err)
	}
	if remaining != nil {
		t.Fatalf("expected draft to be deleted once empty, got %+v", remaining)
	}
	drafts, _ := s.List("website")
	if len(drafts) != 0 {
		t.Fatalf("expected 0 drafts after removing the last photo, got %d", len(drafts))
	}
}

func TestDraftStore_Delete(t *testing.T) {
	s := newTestDraftStore(t)
	d, _ := s.Create("website", DraftTarget{Title: "Summer 2026"}, []DraftPhoto{{LibraryID: "lib1", PhotoID: "p1"}})
	if err := s.Delete("website", d.ID); err != nil {
		t.Fatalf("Delete: %v", err)
	}
	drafts, _ := s.List("website")
	if len(drafts) != 0 {
		t.Fatalf("expected 0 drafts after Delete, got %d", len(drafts))
	}
}

func TestDraftStore_TwoChannelsDoNotShareDrafts(t *testing.T) {
	s := newTestDraftStore(t)
	s.Create("website", DraftTarget{Title: "A"}, []DraftPhoto{{LibraryID: "lib1", PhotoID: "p1"}})
	s.Create("instagram", DraftTarget{Title: "B"}, []DraftPhoto{{LibraryID: "lib1", PhotoID: "p2"}})
	websiteDrafts, _ := s.List("website")
	instaDrafts, _ := s.List("instagram")
	if len(websiteDrafts) != 1 || len(instaDrafts) != 1 {
		t.Fatalf("expected 1 draft per channel, got website=%d instagram=%d", len(websiteDrafts), len(instaDrafts))
	}
	if _, statErr := filepath.Abs(s.path("website")); statErr != nil {
		t.Fatalf("path: %v", statErr)
	}
}
