package publish

import (
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"huepattl.de/unterlumen/internal/channels"
)

// A photo renamed outside Unterlumen stays at its old name in the index
// until the library is scanned. When no collected photo can be exported,
// the run says why and builds nothing: an empty gallery and a 22-byte ZIP
// went online before, in place of the gallery that was there.
func TestGenerateDraft_GalleryWithNoPhotoExportedBuildsNothing(t *testing.T) {
	mux, mgr, chStore, draftStore := setupGenerateTestMux(t)
	if err := chStore.Save(&channels.Channel{Slug: "share", Name: "Share", Format: "jpeg", Quality: 85, GalleryExport: true}); err != nil {
		t.Fatal(err)
	}
	libID := seedLibraryPhoto(t, mgr, "gone1")
	store, _ := mgr.OpenStore(libID)
	path, _ := store.GetPhotoPathHint("gone1")
	store.Close()
	os.Rename(path, filepath.Join(filepath.Dir(path), "renamed.jpg")) //nolint:errcheck

	draft, err := draftStore.Create("share", channels.DraftTarget{Title: "Kites"}, []channels.DraftPhoto{{LibraryID: libID, PhotoID: "gone1"}})
	if err != nil {
		t.Fatal(err)
	}
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, httptest.NewRequest("POST", "/api/channels/share/drafts/"+draft.ID+"/generate", strings.NewReader("{}")))
	body := rec.Body.String()
	if !strings.Contains(body, `"error":"no photo could be exported`) || !strings.Contains(body, "scan the library") {
		t.Fatalf("want an error naming the missing file, got %s", body)
	}
	if strings.Contains(body, `"complete":true`) {
		t.Fatal("the run completed with nothing built")
	}
	if strings.Contains(body, `"step":"zip"`) || strings.Contains(body, `"step":"page"`) {
		t.Errorf("it went on to pack or build: %s", body)
	}
}
