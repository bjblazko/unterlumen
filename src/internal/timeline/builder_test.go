package timeline

import (
	"testing"
	"time"

	"huepattl.de/unterlumen/internal/library"
)

func seedLibrary(t *testing.T, mgr *library.Manager, name string, photos map[string]string) string {
	t.Helper()
	l, err := mgr.CreateLibrary(name, "", t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	s, err := mgr.OpenStore(l.ID)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	for id, taken := range photos {
		if err := s.UpsertPhoto(id, "/"+id, id+".jpg", 1, time.Now(), `{"width":3,"height":2}`, "", taken, "jpeg"); err != nil {
			t.Fatal(err)
		}
	}
	return l.ID
}

func TestBuilderMergesInSidebarOrderAndCaches(t *testing.T) {
	mgr, err := library.NewManager(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	first := seedLibrary(t, mgr, "Zeta", map[string]string{"shared": "2022-01-01T10:00:00", "z": "2023-01-01T10:00:00"})
	second := seedLibrary(t, mgr, "Alpha", map[string]string{"shared": "2022-01-01T10:00:00", "u": ""})
	if err := mgr.SetLibrarySortOrder([]string{first, second}); err != nil {
		t.Fatal(err)
	}
	b := NewBuilder(mgr)
	s, err := b.Current(Scope{})
	if err != nil {
		t.Fatal(err)
	}
	if len(s.Photos) != 2 || s.Photos[0].ID != "shared" || s.Photos[0].LibraryID != first || s.Undated != 1 {
		t.Fatalf("stream = %+v", s)
	}
	if s.Photos[1].Day != 365 || s.Start != "2022-01-01" || s.Photos[0].Ratio != 1.5 {
		t.Errorf("days/ratio = %+v, start %s", s.Photos, s.Start)
	}
	again, _ := b.Current(Scope{})
	if again != s {
		t.Error("an unchanged library built the stream again")
	}
	seedLibrary(t, mgr, "Beta", map[string]string{"new": "2024-01-01T10:00:00"})
	changed, _ := b.Current(Scope{})
	if changed.Version == s.Version || len(changed.Photos) != 3 {
		t.Errorf("a new library did not change the stream: %+v", changed)
	}
}
