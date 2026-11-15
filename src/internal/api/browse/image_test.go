package browse

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"slices"
	"testing"

	"huepattl.de/unterlumen/internal/media"
)

// A crop writes the photo again under the same URL, so the browser must ask
// each time; without a Cache-Control it guessed from the file's age and kept
// an old photo's uncropped bytes for days.
func TestAPhotoIsAskedForAgainEachTime(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "a.jpg"), []byte("jpeg"), 0o600); err != nil {
		t.Fatal(err)
	}
	rec := httptest.NewRecorder()
	handleImage(root, media.NewImageCache(2))(rec, httptest.NewRequest(http.MethodGet, "/api/image?path=a.jpg", nil))

	if rec.Code != http.StatusOK {
		t.Fatalf("answered %d", rec.Code)
	}
	if got := rec.Header().Get("Cache-Control"); got != "no-cache" {
		t.Errorf("Cache-Control %q, want no-cache", got)
	}
}

// A slideshow of a folder and its subfolders shows what the folder view
// shows: nothing hidden — no ._ files a Mac leaves on a NAS, nothing from
// .unterlumen-shared.
func TestTheRecursivePhotosLeaveOutWhatIsHidden(t *testing.T) {
	root, err := filepath.EvalSymlinks(t.TempDir()) // as main resolves it
	if err != nil {
		t.Fatal(err)
	}
	for _, p := range []string{"a.jpg", "sub/b.jpg", "sub/._b.jpg", ".unterlumen-shared/c.jpg"} {
		full := filepath.Join(root, p)
		if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(full, []byte("jpeg"), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	rec := httptest.NewRecorder()
	handleBrowseRecursive(root)(rec, httptest.NewRequest(http.MethodGet, "/api/browse/recursive?path=", nil))

	var got browseRecursiveResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &got); err != nil {
		t.Fatal(err)
	}
	if want := []string{"a.jpg", "sub/b.jpg"}; !slices.Equal(got.Paths, want) {
		t.Errorf("paths %v, want %v", got.Paths, want)
	}
}
