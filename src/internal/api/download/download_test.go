package download

import (
	"mime"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
)

func TestOriginalServesTheFileUnderItsOwnName(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "Straße am Meer.hif")
	if err := os.WriteFile(path, []byte("original bytes"), 0o644); err != nil {
		t.Fatal(err)
	}
	rec := httptest.NewRecorder()
	Original(rec, httptest.NewRequest("GET", "/x?download=1", nil), path)

	if rec.Code != 200 || rec.Body.String() != "original bytes" {
		t.Fatalf("got %d %q", rec.Code, rec.Body.String())
	}
	disp, params, err := mime.ParseMediaType(rec.Header().Get("Content-Disposition"))
	if err != nil || disp != "attachment" || params["filename"] != "Straße am Meer.hif" {
		t.Errorf("Content-Disposition = %q (%v)", rec.Header().Get("Content-Disposition"), err)
	}
}

func TestOriginalSaysWhenTheFileIsGone(t *testing.T) {
	rec := httptest.NewRecorder()
	Original(rec, httptest.NewRequest("GET", "/x?download=1", nil), filepath.Join(t.TempDir(), "gone.jpg"))
	if rec.Code != 404 {
		t.Errorf("code = %d, want 404", rec.Code)
	}
}

func TestRequested(t *testing.T) {
	if !Requested(httptest.NewRequest("GET", "/api/image?path=a.jpg&download=1", nil)) {
		t.Error("download=1 not recognised")
	}
	if Requested(httptest.NewRequest("GET", "/api/image?path=a.jpg", nil)) {
		t.Error("plain request taken for a download")
	}
}
