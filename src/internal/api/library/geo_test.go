package apilibrary

import (
	"encoding/json"
	"net/http/httptest"
	"testing"
	"time"
)

func TestLibraryGeoListsLocatedPhotosPerLibrary(t *testing.T) {
	mgr := newTestManager(t)
	l, err := mgr.CreateLibrary("Trips", "", t.TempDir())
	if err != nil {
		t.Fatalf("CreateLibrary: %v", err)
	}
	store, err := mgr.OpenStore(l.ID)
	if err != nil {
		t.Fatalf("OpenStore: %v", err)
	}
	if err := store.UpsertPhoto("p1", "/x/a.jpg", "a.jpg", 1, time.Now(), `{"latitude":48.1,"longitude":11.5}`, "", "2024-05-01T10:30:00", "jpeg"); err != nil {
		t.Fatal(err)
	}
	if err := store.UpsertPhoto("p2", "/x/b.jpg", "b.jpg", 1, time.Now(), `{}`, "", "", "jpeg"); err != nil {
		t.Fatal(err)
	}
	store.Close()

	rec := httptest.NewRecorder()
	libraryGeo(mgr)(rec, httptest.NewRequest("GET", "/api/library/geo", nil))

	var got struct {
		Libraries []struct {
			ID     string  `json:"id"`
			Points [][]any `json:"points"`
		} `json:"libraries"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatalf("decode %q: %v", rec.Body.String(), err)
	}
	if len(got.Libraries) != 1 || got.Libraries[0].ID != l.ID || len(got.Libraries[0].Points) != 1 {
		t.Fatalf("body = %s", rec.Body.String())
	}
	p := got.Libraries[0].Points[0]
	if p[0] != "p1" || p[1] != 48.1 || p[2] != 11.5 || p[3] != "2024-05-01T10:30:00" || p[4] != "a.jpg" {
		t.Errorf("point = %v", p)
	}
}
