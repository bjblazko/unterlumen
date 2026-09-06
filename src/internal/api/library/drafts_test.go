package apilibrary

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

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
