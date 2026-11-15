package heifjpeg

import (
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strconv"
	"testing"
	"time"

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

// A crop writes the photo again under the same URL. The browser must ask
// every time it shows the photo, or it shows the uncropped one for as long
// as it is allowed to keep it — it was an hour.
func TestTheBrowserAsksAgainAndACroppedPhotoIsAnsweredAnew(t *testing.T) {
	path := filepath.Join(t.TempDir(), "DSCF0002.HIF")
	if err := os.WriteFile(path, []byte("heif"), 0o600); err != nil {
		t.Fatal(err)
	}
	cache := media.NewImageCache(4)
	before := serveConverted(t, path, cache, "uncropped", "")
	if got := before.Header().Get("Cache-Control"); got != "private, no-cache" {
		t.Errorf("Cache-Control %q, want %q", got, "private, no-cache")
	}

	later := time.Now().Add(2 * time.Second)
	if err := os.Chtimes(path, later, later); err != nil {
		t.Fatal(err)
	}
	after := serveConverted(t, path, cache, "cropped", before.Header().Get("ETag"))
	if after.Code != http.StatusOK || after.Body.String() != "cropped" {
		t.Errorf("after the crop answered %d %q, want 200 %q", after.Code, after.Body.String(), "cropped")
	}
}

// serveConverted answers for path as if it had been converted to jpeg.
func serveConverted(t *testing.T, path string, cache *media.ImageCache, jpeg, ifNoneMatch string) *httptest.ResponseRecorder {
	t.Helper()
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	cache.Set(path+":"+strconv.FormatInt(info.ModTime().UnixNano(), 10), []byte(jpeg))
	req := httptest.NewRequest(http.MethodGet, "/api/image", nil)
	if ifNoneMatch != "" {
		req.Header.Set("If-None-Match", ifNoneMatch)
	}
	rec := httptest.NewRecorder()
	Serve(rec, req, path, cache)
	return rec
}
