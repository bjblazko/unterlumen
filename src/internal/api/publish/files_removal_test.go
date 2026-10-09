package publish

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"huepattl.de/unterlumen/internal/channels"
	"huepattl.de/unterlumen/internal/media"
)

// Taking a photo out of a files destination used to only make the library
// forget: the sidecar still named the album, so the next scan put it back.
func TestDeleteBuiltMeta_FilesDestination_RemovesSidecarRecordAndFile(t *testing.T) {
	mux, mgr, chStore, draftStore := setupGenerateTestMux(t)
	if err := chStore.Save(&channels.Channel{Slug: "stream", Name: "Stream", Format: "jpeg", Quality: 85}); err != nil {
		t.Fatalf("Save channel: %v", err)
	}
	libID := seedLibraryPhoto(t, mgr, "photo1")
	draft, err := draftStore.Create("stream", channels.DraftTarget{Title: "Stream"}, []channels.DraftPhoto{{LibraryID: libID, PhotoID: "photo1"}})
	if err != nil {
		t.Fatalf("Create draft: %v", err)
	}
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, httptest.NewRequest("POST", "/api/channels/stream/drafts/"+draft.ID+"/generate", strings.NewReader("{}")))
	if rec.Code != http.StatusOK {
		t.Fatalf("generate: status = %d, body = %s", rec.Code, rec.Body.String())
	}
	var resp struct {
		PostID  string        `json:"postID"`
		Results []buildResult `json:"results"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil || len(resp.Results) != 1 {
		t.Fatalf("decode: %v, body = %s", err, rec.Body.String())
	}

	store, err := mgr.OpenStore(libID)
	if err != nil {
		t.Fatalf("OpenStore: %v", err)
	}
	defer store.Close()
	photoPath, _ := store.GetPhotoPathHint("photo1")

	handled, err := DeleteBuiltMeta(store, chStore, "photo1", "built:stream:"+resp.PostID)
	if err != nil || !handled {
		t.Fatalf("DeleteBuiltMeta: handled = %v, err = %v", handled, err)
	}

	if pubs, _ := media.ReadSidecar(photoPath); len(pubs) != 0 {
		t.Errorf("sidecar still records %+v", pubs)
	}
	entries, _ := store.GetMeta("photo1")
	for _, e := range entries {
		if strings.HasPrefix(e.Key, "built:stream") {
			t.Errorf("library still has %s", e.Key)
		}
	}
	if _, err := os.Stat(filepath.Join(chStore.OutputDir("stream"), resp.Results[0].Filename)); !os.IsNotExist(err) {
		t.Errorf("exported file still there: %v", err)
	}
	if _, err := os.Stat(photoPath); err != nil {
		t.Errorf("the photo itself is gone: %v", err)
	}
}

// A photo whose export failed was still recorded as published: buildOne wrote
// the sidecar and the library keys first. It then showed as in the gallery
// with no file in the folder, and nothing offered to export it again.
func TestGenerateDraft_FailedExport_RecordsNothing(t *testing.T) {
	mux, mgr, chStore, draftStore := setupGenerateTestMux(t)
	if err := chStore.Save(&channels.Channel{Slug: "stream", Name: "Stream", Format: "jpeg", Quality: 85}); err != nil {
		t.Fatalf("Save channel: %v", err)
	}
	source := t.TempDir()
	l, err := mgr.CreateLibrary("Broken", "", source)
	if err != nil {
		t.Fatalf("CreateLibrary: %v", err)
	}
	store, err := mgr.OpenStore(l.ID)
	if err != nil {
		t.Fatalf("OpenStore: %v", err)
	}
	defer store.Close()
	photoPath := filepath.Join(source, "broken.jpg")
	if err := os.WriteFile(photoPath, []byte("not an image"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := store.UpsertPhoto("broken", photoPath, "broken.jpg", 12, time.Now(), "{}", "", "", "jpeg"); err != nil {
		t.Fatalf("UpsertPhoto: %v", err)
	}
	draft, err := draftStore.Create("stream", channels.DraftTarget{Title: "Stream"}, []channels.DraftPhoto{{LibraryID: l.ID, PhotoID: "broken"}})
	if err != nil {
		t.Fatalf("Create draft: %v", err)
	}

	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, httptest.NewRequest("POST", "/api/channels/stream/drafts/"+draft.ID+"/generate", strings.NewReader("{}")))
	if rec.Code != http.StatusOK {
		t.Fatalf("generate: status = %d, body = %s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), `"error"`) {
		t.Fatalf("expected the export to fail, body = %s", rec.Body.String())
	}

	if pubs, _ := media.ReadSidecar(photoPath); len(pubs) != 0 {
		t.Errorf("sidecar records a failed export: %+v", pubs)
	}
	entries, _ := store.GetMeta("broken")
	for _, e := range entries {
		if strings.HasPrefix(e.Key, "built:") {
			t.Errorf("library records a failed export: %s", e.Key)
		}
	}
}
