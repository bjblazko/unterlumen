package heifjpeg

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"huepattl.de/unterlumen/internal/media"
)

// A prefetch of a HEIF nobody has opened yet must not start a conversion: on
// the NAS each one took about 640 MB, and the viewer asks for the next two
// photos every time it moves on.
func TestPrefetchOfAnUnconvertedHEIFAnswersAtOnceWithNothing(t *testing.T) {
	path := filepath.Join(t.TempDir(), "DSCF0001.HIF")
	// Not a real HEIF: had Serve tried to convert it, it would answer 500.
	if err := os.WriteFile(path, []byte("not decoded"), 0o600); err != nil {
		t.Fatal(err)
	}
	req := httptest.NewRequest(http.MethodGet, "/api/image?path=DSCF0001.HIF", nil)
	req.Header.Set("X-Prefetch", "1")
	rec := httptest.NewRecorder()

	Serve(rec, req, path, media.NewImageCache(2))

	if rec.Code != http.StatusNoContent {
		t.Fatalf("prefetch answered %d, want %d", rec.Code, http.StatusNoContent)
	}
	if got := rec.Header().Get("Cache-Control"); got != "no-store" {
		t.Errorf("Cache-Control %q, want no-store: the browser must not keep the empty answer for the photo", got)
	}
}

func TestAPhotoGoneFromDiskIsNotFound(t *testing.T) {
	rec := httptest.NewRecorder()
	Serve(rec, httptest.NewRequest(http.MethodGet, "/", nil), filepath.Join(t.TempDir(), "gone.HIF"), media.NewImageCache(2))
	if rec.Code != http.StatusNotFound {
		t.Errorf("answered %d, want %d", rec.Code, http.StatusNotFound)
	}
}
