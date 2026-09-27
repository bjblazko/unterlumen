package apitimeline

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"huepattl.de/unterlumen/internal/library"
	"huepattl.de/unterlumen/internal/timeline"
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
	for i, taken := range []string{"2020-01-01T10:00:00", "2020-01-03T10:00:00", "2021-06-01T10:00:00"} {
		id := string(rune('a' + i))
		if err := s.UpsertPhoto(id, "/"+id, id+".jpg", 1, time.Now(), `{}`, "", taken, "jpeg"); err != nil {
			t.Fatal(err)
		}
	}
	mux := http.NewServeMux()
	Handle(mux, timeline.NewBuilder(mgr))
	return mux
}

func get(t *testing.T, mux *http.ServeMux, url string) *httptest.ResponseRecorder {
	t.Helper()
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, url, nil))
	return rec
}

func fetchSkeleton(t *testing.T, mux *http.ServeMux) timeline.Skeleton {
	t.Helper()
	var sk timeline.Skeleton
	if err := json.NewDecoder(get(t, mux, "/api/timeline").Body).Decode(&sk); err != nil {
		t.Fatal(err)
	}
	return sk
}

func TestSkeletonListsDays(t *testing.T) {
	sk := fetchSkeleton(t, testMux(t))
	if len(sk.Days) != 3 || sk.Days[1] != 2 || sk.Start != "2020-01-01" || sk.Version == "" {
		t.Fatalf("skeleton = %+v", sk)
	}
}

func TestPhotosReturnsAPage(t *testing.T) {
	mux := testMux(t)
	v := fetchSkeleton(t, mux).Version
	rec := get(t, mux, "/api/timeline/photos?v="+v+"&from=1&count=5")
	var body struct{ Photos [][4]string }
	if err := json.NewDecoder(rec.Body).Decode(&body); err != nil {
		t.Fatal(err)
	}
	if rec.Code != 200 || len(body.Photos) != 2 || body.Photos[0][1] != "b" || body.Photos[0][2] != "b.jpg" || body.Photos[0][3] != "2020-01-03T10:00:00" {
		t.Fatalf("code %d, photos %v", rec.Code, body.Photos)
	}
}

func TestPhotosWithStaleVersionIs409(t *testing.T) {
	if rec := get(t, testMux(t), "/api/timeline/photos?v=old&from=0&count=5"); rec.Code != http.StatusConflict {
		t.Fatalf("code = %d, want 409", rec.Code)
	}
}

func TestPhotosRejectsBadRanges(t *testing.T) {
	mux := testMux(t)
	for _, q := range []string{"from=x&count=5", "from=0&count=0", "from=-1&count=5", "count=5"} {
		if rec := get(t, mux, "/api/timeline/photos?v=any&"+q); rec.Code != http.StatusBadRequest {
			t.Errorf("%s: code = %d, want 400", q, rec.Code)
		}
	}
}
