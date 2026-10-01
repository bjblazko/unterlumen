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

func TestDraftStore_AppendPhotosDeduplicates(t *testing.T) {
	s := newTestDraftStore(t)
	d, _ := s.Create("fotoshare", DraftTarget{Title: "Uli"}, []DraftPhoto{{LibraryID: "lib1", PhotoID: "p1"}})

	// Collecting the same selection again must not queue the photo twice —
	// it would otherwise be exported twice into the same gallery.
	updated, err := s.AppendPhotos("fotoshare", d.ID, []DraftPhoto{
		{LibraryID: "lib1", PhotoID: "p1"},
		{LibraryID: "lib2", PhotoID: "p1"}, // same hash, different library: a real second file
		{LibraryID: "lib1", PhotoID: "p2"},
	})
	if err != nil {
		t.Fatalf("AppendPhotos: %v", err)
	}
	if len(updated.Photos) != 3 {
		t.Fatalf("photos = %+v, want 3 (p1@lib1, p1@lib2, p2@lib1)", updated.Photos)
	}
}

func TestDraftStore_CreateDeduplicates(t *testing.T) {
	s := newTestDraftStore(t)
	d, err := s.Create("fotoshare", DraftTarget{Title: "Uli"}, []DraftPhoto{
		{LibraryID: "lib1", PhotoID: "p1"},
		{LibraryID: "lib1", PhotoID: "p1"},
	})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if len(d.Photos) != 1 {
		t.Fatalf("photos = %+v, want 1", d.Photos)
	}
}

func TestDraftStore_SetTargetPostID(t *testing.T) {
	s := newTestDraftStore(t)
	d, _ := s.Create("fotoshare", DraftTarget{Title: "Uli"}, []DraftPhoto{{LibraryID: "lib1", PhotoID: "p1"}})

	if err := s.SetTargetPostID("fotoshare", d.ID, "90a04848ecc797dd42115350"); err != nil {
		t.Fatalf("SetTargetPostID: %v", err)
	}
	drafts, _ := s.List("fotoshare")
	if len(drafts) != 1 || drafts[0].Target.PostID != "90a04848ecc797dd42115350" {
		t.Fatalf("target = %+v, want PostID pinned to the published album", drafts[0].Target)
	}
	// Title must survive, so the retry still labels the album correctly.
	if drafts[0].Target.Title != "Uli" {
		t.Fatalf("title = %q, want %q", drafts[0].Target.Title, "Uli")
	}
}

// Two independent albums may be pending in one channel at the same time —
// that is the whole point of a single-gallery channel serving one host.
func TestDraftStore_MultiplePendingGalleriesPerChannel(t *testing.T) {
	s := newTestDraftStore(t)
	a, _ := s.Create("fotoshare", DraftTarget{Title: "Uli"}, []DraftPhoto{{LibraryID: "lib1", PhotoID: "p1"}})
	b, _ := s.Create("fotoshare", DraftTarget{Title: "Regenwanderung"}, []DraftPhoto{{LibraryID: "lib1", PhotoID: "p2"}})

	if a.ID == b.ID {
		t.Fatal("expected two distinct drafts")
	}
	drafts, _ := s.List("fotoshare")
	if len(drafts) != 2 {
		t.Fatalf("drafts = %+v, want 2", drafts)
	}
	if err := s.SetTargetPostID("fotoshare", a.ID, "aaa"); err != nil {
		t.Fatalf("SetTargetPostID: %v", err)
	}
	drafts, _ = s.List("fotoshare")
	for _, d := range drafts {
		if d.ID == b.ID && d.Target.PostID != "" {
			t.Fatalf("publishing one album must not pin the other; got %+v", d.Target)
		}
	}
}

func TestDraftStore_RekeyLibrary(t *testing.T) {
	dir := t.TempDir()
	chs := NewStore(dir, dir)
	if err := chs.Save(&Channel{Slug: "website", Name: "Website", Format: "jpeg", Quality: 85}); err != nil {
		t.Fatal(err)
	}
	s := NewDraftStore(chs)
	d, _ := s.Create("website", DraftTarget{Title: "Summer"}, []DraftPhoto{{LibraryID: "old", PhotoID: "p1"}, {LibraryID: "other", PhotoID: "p2"}})
	if err := s.RekeyLibrary("old", "new"); err != nil {
		t.Fatalf("RekeyLibrary: %v", err)
	}
	got, _ := s.Get("website", d.ID)
	if got.Photos[0].LibraryID != "new" || got.Photos[1].LibraryID != "other" {
		t.Errorf("photos = %+v, want old→new and other untouched", got.Photos)
	}
}
