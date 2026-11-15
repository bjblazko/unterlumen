package apilibrary

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
	"time"
)

// A photo's thumbnail changes under the same address: a crop writes the
// photo again, and its old ID leads to the new one's thumbnail. The browser
// must ask each time — it kept the uncropped one for a day.
func TestAThumbnailIsAskedForAgainEachTime(t *testing.T) {
	mgr := newTestManager(t)
	l, err := mgr.CreateLibrary("Test", "", t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	store, err := mgr.OpenStore(l.ID)
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	if err := os.WriteFile(filepath.Join(mgr.LibDir(l.ID), "t.jpg"), []byte("thumb"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := store.UpsertPhoto("photo1", "/x/a.jpg", "a.jpg", 4, time.Now(), "{}", "t.jpg", "", "jpeg"); err != nil {
		t.Fatal(err)
	}

	req := httptest.NewRequest(http.MethodGet, "/api/library/"+l.ID+"/thumb/photo1", nil)
	req.SetPathValue("id", l.ID)
	req.SetPathValue("photoID", "photo1")
	rec := httptest.NewRecorder()
	serveThumb(mgr)(rec, req)

	if rec.Code != http.StatusOK || rec.Body.String() != "thumb" {
		t.Fatalf("answered %d %q", rec.Code, rec.Body.String())
	}
	if got := rec.Header().Get("Cache-Control"); got != "no-cache" {
		t.Errorf("Cache-Control %q, want no-cache", got)
	}
	if rec.Header().Get("Last-Modified") == "" {
		t.Error("no Last-Modified: the browser could not ask cheaply whether it changed")
	}
}
