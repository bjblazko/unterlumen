package publish

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"
	"time"

	"huepattl.de/unterlumen/internal/channels"
	lib "huepattl.de/unterlumen/internal/library"
)

// setupDraftTestMux mirrors the wiring in routes.go for the drafts endpoints
// only. It returns the manager as well so tests can register a real library
// (collectDraft validates the library exists via mgr.OpenStore).
//
// Uses the same *lib.Manager construction pattern as newTestManager in
// handler_test.go: lib.NewManager takes a root directory, not a file path —
// the brief's sketch guessed a different (incorrect) constructor call.
func setupDraftTestMux(t *testing.T) (*http.ServeMux, *lib.Manager, *channels.DraftStore) {
	t.Helper()
	mgr, err := lib.NewManager(t.TempDir())
	if err != nil {
		t.Fatalf("NewManager: %v", err)
	}
	dir := t.TempDir()
	chStore := channels.NewStore(dir, dir)
	if err := chStore.Save(&channels.Channel{Slug: "website", Name: "Website", Format: "jpeg", Quality: 85, GalleryExport: true}); err != nil {
		t.Fatalf("Save channel: %v", err)
	}
	draftStore := channels.NewDraftStore(chStore)
	mux := http.NewServeMux()
	registerDraftRoutes(mux, mgr, chStore, draftStore)
	return mux, mgr, draftStore
}

func TestCollectDraft_CreatesNewDraft(t *testing.T) {
	mux, mgr, draftStore := setupDraftTestMux(t)
	l, err := mgr.CreateLibrary("Test", "", t.TempDir())
	if err != nil {
		t.Fatalf("CreateLibrary: %v", err)
	}

	body, _ := json.Marshal(map[string]any{"photoIDs": []string{"p1", "p2"}, "title": "Summer 2026"})
	req := httptest.NewRequest("POST", "/api/library/"+l.ID+"/channels/website/drafts", bytes.NewReader(body))
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
	var draft channels.Draft
	if err := json.Unmarshal(rec.Body.Bytes(), &draft); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(draft.Photos) != 2 || draft.Photos[0].LibraryID != l.ID {
		t.Fatalf("unexpected draft: %+v", draft)
	}

	drafts, _ := draftStore.List("website")
	if len(drafts) != 1 {
		t.Fatalf("expected 1 persisted draft, got %d", len(drafts))
	}
}

func TestCollectDraft_AppendsToExistingByDraftID(t *testing.T) {
	mux, mgr, draftStore := setupDraftTestMux(t)
	l, err := mgr.CreateLibrary("Test", "", t.TempDir())
	if err != nil {
		t.Fatalf("CreateLibrary: %v", err)
	}

	d, _ := draftStore.Create("website", channels.DraftTarget{Title: "Summer 2026"}, nil)

	body, _ := json.Marshal(map[string]any{"photoIDs": []string{"p1"}, "draftID": d.ID})
	req := httptest.NewRequest("POST", "/api/library/"+l.ID+"/channels/website/drafts", bytes.NewReader(body))
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}

	drafts, _ := draftStore.List("website")
	if len(drafts) != 1 || len(drafts[0].Photos) != 1 {
		t.Fatalf("expected the photo appended to the existing draft, got %+v", drafts)
	}
}

func TestCollectDraft_UnknownLibraryReturns404(t *testing.T) {
	mux, _, _ := setupDraftTestMux(t)
	body, _ := json.Marshal(map[string]any{"photoIDs": []string{"p1"}, "title": "X"})
	req := httptest.NewRequest("POST", "/api/library/nonexistent/channels/website/drafts", bytes.NewReader(body))
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want 404, body = %s", rec.Code, rec.Body.String())
	}
}

func TestCollectDraft_UnknownChannelReturns400(t *testing.T) {
	mux, mgr, _ := setupDraftTestMux(t)
	l, err := mgr.CreateLibrary("Test", "", t.TempDir())
	if err != nil {
		t.Fatalf("CreateLibrary: %v", err)
	}
	body, _ := json.Marshal(map[string]any{"photoIDs": []string{"p1"}, "title": "X"})
	req := httptest.NewRequest("POST", "/api/library/"+l.ID+"/channels/does-not-exist/drafts", bytes.NewReader(body))
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400, body = %s", rec.Code, rec.Body.String())
	}
}

func TestListDrafts_EmptyIsEmptyArrayNotNull(t *testing.T) {
	mux, _, _ := setupDraftTestMux(t)
	req := httptest.NewRequest("GET", "/api/channels/website/drafts", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d", rec.Code)
	}
	if rec.Body.String() != "[]\n" {
		t.Fatalf("body = %q, want %q", rec.Body.String(), "[]\n")
	}
}

func TestDeleteDraft(t *testing.T) {
	mux, _, draftStore := setupDraftTestMux(t)
	d, _ := draftStore.Create("website", channels.DraftTarget{Title: "X"}, []channels.DraftPhoto{{LibraryID: "lib1", PhotoID: "p1"}})

	req := httptest.NewRequest("DELETE", "/api/channels/website/drafts/"+d.ID, nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusNoContent {
		t.Fatalf("status = %d", rec.Code)
	}
	drafts, _ := draftStore.List("website")
	if len(drafts) != 0 {
		t.Fatalf("expected draft deleted, got %d remaining", len(drafts))
	}
}

func TestRemoveDraftPhoto_PartialThenFinal(t *testing.T) {
	mux, _, draftStore := setupDraftTestMux(t)
	d, _ := draftStore.Create("website", channels.DraftTarget{Title: "X"}, []channels.DraftPhoto{
		{LibraryID: "lib1", PhotoID: "p1"}, {LibraryID: "lib1", PhotoID: "p2"},
	})

	req := httptest.NewRequest("DELETE", "/api/channels/website/drafts/"+d.ID+"/photos/lib1/p1", nil)
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}

	req = httptest.NewRequest("DELETE", "/api/channels/website/drafts/"+d.ID+"/photos/lib1/p2", nil)
	rec = httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusNoContent {
		t.Fatalf("status = %d after removing last photo", rec.Code)
	}
}

// Discarding a draft must take its photos' pending markers with it. Without
// that, the Info Panel and the tag-chip filters keep claiming the photos are
// collected into a gallery that no longer exists — the "app says X, disk says
// Y" bug class (ADR-0026's pending:<slug> markers).
func TestDeleteDraft_ClearsPendingMarkers(t *testing.T) {
	mux, mgr, draftStore := setupDraftTestMux(t)
	libID := seedLibraryPhotos(t, mgr, "p1", "p2")
	body, _ := json.Marshal(map[string]any{"photoIDs": []string{"p1", "p2"}, "title": "Summer 2026"})
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, httptest.NewRequest("POST", "/api/library/"+libID+"/channels/website/drafts", bytes.NewReader(body)))
	if rec.Code != http.StatusOK {
		t.Fatalf("collect status = %d, body = %s", rec.Code, rec.Body.String())
	}
	drafts, _ := draftStore.List("website")
	draftID := drafts[0].ID

	store, err := mgr.OpenStore(libID)
	if err != nil {
		t.Fatalf("OpenStore: %v", err)
	}
	defer store.Close()
	if !hasPendingKey(t, store, "p1", "website") {
		t.Fatal("collect did not write a pending marker — test cannot prove anything")
	}

	rec = httptest.NewRecorder()
	mux.ServeHTTP(rec, httptest.NewRequest("DELETE", "/api/channels/website/drafts/"+draftID, nil))
	if rec.Code != http.StatusNoContent {
		t.Fatalf("delete status = %d, body = %s", rec.Code, rec.Body.String())
	}

	for _, id := range []string{"p1", "p2"} {
		if hasPendingKey(t, store, id, "website") {
			t.Errorf("photo %s still carries a pending marker after the draft was discarded", id)
		}
	}
}

// Removing one photo from a draft clears that photo's markers and leaves the
// rest of the draft alone.
func TestRemoveDraftPhoto_ClearsPendingMarkersForThatPhotoOnly(t *testing.T) {
	mux, mgr, draftStore := setupDraftTestMux(t)
	libID := seedLibraryPhotos(t, mgr, "p1", "p2")
	body, _ := json.Marshal(map[string]any{"photoIDs": []string{"p1", "p2"}, "title": "Summer 2026"})
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, httptest.NewRequest("POST", "/api/library/"+libID+"/channels/website/drafts", bytes.NewReader(body)))
	drafts, _ := draftStore.List("website")
	draftID := drafts[0].ID

	store, err := mgr.OpenStore(libID)
	if err != nil {
		t.Fatalf("OpenStore: %v", err)
	}
	defer store.Close()

	rec = httptest.NewRecorder()
	mux.ServeHTTP(rec, httptest.NewRequest("DELETE", "/api/channels/website/drafts/"+draftID+"/photos/"+libID+"/p1", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("remove status = %d, body = %s", rec.Code, rec.Body.String())
	}

	if hasPendingKey(t, store, "p1", "website") {
		t.Error("the removed photo still carries a pending marker")
	}
	if !hasPendingKey(t, store, "p2", "website") {
		t.Error("the photo still in the draft lost its pending marker")
	}
}

func hasPendingKey(t *testing.T, store *lib.Store, photoID, slug string) bool {
	t.Helper()
	entries, err := store.GetMeta(photoID)
	if err != nil {
		t.Fatalf("GetMeta(%s): %v", photoID, err)
	}
	for _, e := range entries {
		if e.Key == "pending:"+slug || e.Key == "pending:"+slug+":" {
			return true
		}
		if len(e.Key) > len("pending:"+slug) && e.Key[:len("pending:"+slug)+1] == "pending:"+slug+":" {
			return true
		}
	}
	return false
}

// seedLibraryPhotos creates a library holding real photo rows, which meta rows
// hang off — UpsertMeta on an unknown photo id writes nothing.
func seedLibraryPhotos(t *testing.T, mgr *lib.Manager, photoIDs ...string) (libID string) {
	t.Helper()
	source := t.TempDir()
	l, err := mgr.CreateLibrary("Test", "", source)
	if err != nil {
		t.Fatalf("CreateLibrary: %v", err)
	}
	store, err := mgr.OpenStore(l.ID)
	if err != nil {
		t.Fatalf("OpenStore: %v", err)
	}
	defer store.Close()
	for _, id := range photoIDs {
		realPath := filepath.Join(source, id+".jpg")
		writeTestJPEG(t, realPath, 40, 30)
		if err := store.UpsertPhoto(id, realPath, id+".jpg", 4, time.Now(), "{}", "", "", "jpeg"); err != nil {
			t.Fatalf("UpsertPhoto(%s): %v", id, err)
		}
	}
	return l.ID
}
