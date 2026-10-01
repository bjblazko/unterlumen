package library

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"huepattl.de/unterlumen/internal/media"
)

// notePhoto is a photo on disk with a row in the store.
func notePhoto(t *testing.T, s *Store, id string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), id+".jpg")
	os.WriteFile(path, []byte("jpeg"), 0o644) //nolint:errcheck
	if err := s.UpsertPhoto(id, path, id+".jpg", 0, time.Now(), "", "", "", "jpeg"); err != nil {
		t.Fatal(err)
	}
	return path
}

func metaOf(t *testing.T, s *Store, id string) map[string]string {
	t.Helper()
	entries, err := s.GetMeta(id)
	if err != nil {
		t.Fatal(err)
	}
	got := map[string]string{}
	for _, e := range entries {
		got[e.Key] = e.Value
	}
	return got
}

func TestWriteNoteGoesToTheSidecarAndTheIndex(t *testing.T) {
	s := newTestStore(t)
	path := notePhoto(t, s, "p1")
	for k, v := range map[string]string{"title": "Harbour", "people": "Anna"} {
		if err := s.WriteNote("p1", k, v); err != nil {
			t.Fatalf("WriteNote(%s): %v", k, err)
		}
	}
	notes, _ := media.ReadNotes(path)
	if notes.Title != "Harbour" || notes.Fields["people"] != "Anna" {
		t.Errorf("sidecar = %+v", notes)
	}
	if got := metaOf(t, s, "p1"); got["title"] != "Harbour" || got["people"] != "Anna" {
		t.Errorf("index = %+v", got)
	}

	s.WriteNote("p1", "people", "") //nolint:errcheck
	if notes, _ := media.ReadNotes(path); notes.Fields["people"] != "" {
		t.Error("removing a field left it in the sidecar")
	}
	if _, ok := metaOf(t, s, "p1")["people"]; ok {
		t.Error("removing a field left it in the index")
	}
}

func TestWriteNoteChangesNothingWhenTheSidecarCannotBeWritten(t *testing.T) {
	s := newTestStore(t)
	path := notePhoto(t, s, "p1")
	os.Mkdir(media.SidecarPath(path), 0o755) //nolint:errcheck — a folder where the sidecar should be
	if err := s.WriteNote("p1", "people", "Anna"); err == nil {
		t.Fatal("WriteNote reported success without a sidecar")
	}
	if _, ok := metaOf(t, s, "p1")["people"]; ok {
		t.Error("the index has a note the sidecar does not")
	}
}

func TestScanTakesNotesFromTheSidecar(t *testing.T) {
	idx, s := newTestIndexer(t, t.TempDir())
	path := notePhoto(t, s, "p1")
	s.SetProp(notesInSidecarProp, "1")          //nolint:errcheck
	s.UpsertMeta("p1", "people", "Old")         //nolint:errcheck
	s.UpsertMeta("p1", "built:website", "2025") //nolint:errcheck
	media.WriteField(path, "mood", "calm")      //nolint:errcheck — written by the other installation

	idx.indexSidecar(path, "p1")
	got := metaOf(t, s, "p1")
	if got["mood"] != "calm" {
		t.Errorf("a field from the sidecar is missing: %+v", got)
	}
	if _, ok := got["people"]; ok {
		t.Errorf("a field the sidecar no longer has is still in the index: %+v", got)
	}
	if got["built:website"] != "2025" {
		t.Errorf("a publication key was dropped with the notes: %+v", got)
	}
}

func TestScanBeforeTheMoveKeepsNotesOnlyTheIndexHas(t *testing.T) {
	idx, s := newTestIndexer(t, t.TempDir())
	path := notePhoto(t, s, "p1")
	s.UpsertMeta("p1", "people", "Anna") //nolint:errcheck
	idx.indexSidecar(path, "p1")
	if metaOf(t, s, "p1")["people"] != "Anna" {
		t.Error("a scan before the move dropped a note")
	}
}

func TestMoveNotesToSidecarsWritesWhatOnlyTheIndexHas(t *testing.T) {
	mgr := newTestManager(t)
	base := t.TempDir()
	l, _ := mgr.CreateLibrary("Old", "", base)
	s, _ := mgr.OpenStore(l.ID)
	s.SetProp(notesInSidecarProp, "") //nolint:errcheck — as a library made before ADR-0048
	path := filepath.Join(base, "a.jpg")
	os.WriteFile(path, []byte("jpeg"), 0o644)                                //nolint:errcheck
	s.UpsertPhoto("p1", path, "a.jpg", 0, time.Now(), "", "", "", "jpeg")    //nolint:errcheck
	s.UpsertMeta("p1", "people", "Anna")                                     //nolint:errcheck
	s.UpsertMeta("p1", "title", "Index title")                               //nolint:errcheck
	s.UpsertMeta("p1", "built:website", "2025")                              //nolint:errcheck
	media.WriteTitle(path, "Sidecar title")                                  //nolint:errcheck
	gone := filepath.Join(base, "gone.jpg")                                  // a photo no longer on disk
	s.UpsertPhoto("p2", gone, "gone.jpg", 0, time.Now(), "", "", "", "jpeg") //nolint:errcheck
	s.UpsertMeta("p2", "people", "Ben")                                      //nolint:errcheck

	mgr.MoveAllNotesToSidecars()

	notes, _ := media.ReadNotes(path)
	if notes.Fields["people"] != "Anna" {
		t.Errorf("the field was not written: %+v", notes)
	}
	if notes.Title != "Sidecar title" {
		t.Errorf("the sidecar's title was replaced: %q", notes.Title)
	}
	if _, ok := notes.Fields["built:website"]; ok {
		t.Error("a publication key was written as a field")
	}
	if _, err := os.Stat(media.SidecarPath(gone)); err == nil {
		t.Error("a sidecar was made for a photo that is gone")
	}
	if !s.NotesInSidecars() {
		t.Error("the library is not marked as moved")
	}
}
