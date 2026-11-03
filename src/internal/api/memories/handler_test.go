package apimemories

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"huepattl.de/unterlumen/internal/library"
	"huepattl.de/unterlumen/internal/memories"
)

func testMux(t *testing.T) *http.ServeMux {
	t.Helper()
	mgr, err := library.NewManager(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	l, err := mgr.CreateLibrary("L", "", t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	s, _ := mgr.OpenStore(l.ID)
	defer s.Close()
	for i, taken := range []string{"2020-11-03T10:00:00", "2020-11-03T15:00:00", "2023-11-03T10:00:00", "2021-06-01T10:00:00"} {
		id := string(rune('a' + i))
		if err := s.UpsertPhoto(id, "/"+id, id+".jpg", 1, time.Now(), `{}`, "", taken, "jpeg"); err != nil {
			t.Fatal(err)
		}
	}
	mux := http.NewServeMux()
	Handle(mux, mgr, nil)
	return mux
}

func get(t *testing.T, mux *http.ServeMux, url string) *httptest.ResponseRecorder {
	t.Helper()
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, url, nil))
	return rec
}

func TestHandlerOverview(t *testing.T) {
	rec := get(t, testMux(t), "/api/memories?day=2026-11-03")
	if rec.Code != http.StatusOK {
		t.Fatalf("status %d: %s", rec.Code, rec.Body)
	}
	var o memories.Overview
	if err := json.NewDecoder(rec.Body).Decode(&o); err != nil {
		t.Fatal(err)
	}
	if o.Day != "2026-11-03" || len(o.Years) != 2 || o.Years[1].Total != 2 || o.Years[0].Picks[0].Lib == "" {
		t.Errorf("overview = %+v", o)
	}
}

func TestHandlerRejectsBadDay(t *testing.T) {
	mux := testMux(t)
	for _, url := range []string{"/api/memories", "/api/memories?day=03.11.2026", "/api/memories/detail?day=x&kind=day&key=2020-11-03"} {
		if rec := get(t, mux, url); rec.Code != http.StatusBadRequest {
			t.Errorf("%s: status %d, want 400", url, rec.Code)
		}
	}
}

func TestHandlerIgnoresRating(t *testing.T) {
	mux := testMux(t)
	plain := get(t, mux, "/api/memories?day=2026-11-03").Body.String()
	if rated := get(t, mux, "/api/memories?day=2026-11-03&rating=5").Body.String(); rated != plain {
		t.Errorf("stars changed the memories:\n%s\n%s", plain, rated)
	}
}

func TestHandlerDetail(t *testing.T) {
	mux := testMux(t)
	rec := get(t, mux, "/api/memories/detail?day=2026-11-03&kind=day&key=2020-11-03")
	var d memories.Detail
	if rec.Code != http.StatusOK || json.NewDecoder(rec.Body).Decode(&d) != nil || len(d.Photos) != 2 {
		t.Fatalf("status %d, detail %+v", rec.Code, d)
	}
	if rec := get(t, mux, "/api/memories/detail?day=2026-11-03&kind=trip&key=1900-01-01"); rec.Code != http.StatusNotFound {
		t.Errorf("unknown trip: status %d, want 404", rec.Code)
	}
}
