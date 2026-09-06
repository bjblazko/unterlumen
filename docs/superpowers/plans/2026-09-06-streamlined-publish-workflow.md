# Streamlined Publish Workflow Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Replace the scattered Build / Channels / Deploy / Published UI with a two-phase **collect → publish** model: collecting photos into a channel is a lightweight, deferred, repeatable action; publishing is one dialog that generates the artifact, lets the user review it, and (for handler-backed channels) deploys it.

**Architecture:** A new `drafts.json` per channel (managed by a new `channels.DraftStore`) holds pending `{libraryID, photoID}` references, keyed by draft ID, with no files written until Generate runs. Generate is the existing `buildPhotos` export/HTML pipeline (`internal/api/library/handler.go`), refactored to read its photo list from a draft (grouped by library) instead of an immediate request body, and to always record XMP. A photo's pending-vs-published state piggybacks on the existing library `photo_meta` mechanism the Info Panel already reads (`built:<slug>` today; this plan adds a parallel `pending:<slug>` key).

**Tech Stack:** Go 1.x backend (`net/http`, stdlib `encoding/json`), vanilla JS/HTML/CSS frontend, no build step, no new dependencies.

**Spec:** `/Users/blazko/.claude/plans/frolicking-juggling-dolphin.md` (brainstormed design, approved 2026-09-06)

## Global Constraints

- No new third-party dependencies (Go or JS) — this project has none beyond what's already vendored.
- Go: `cd src && go vet ./...` must pass after every backend task.
- New Go packages/complex functions get a `_test.go` (CLAUDE.md coding standard).
- New user-visible features get an e2e spec in `e2e/specs/` (CLAUDE.md coding standard).
- CSS: group new rules under a `/* --- Component --- */` comment; no speculative utility classes.
- New dialogs must follow one of the two documented keyboard-guard patterns in `app-keyboard.js` (CLAUDE.md) — this plan uses the `.modal-overlay`... actually this codebase's existing dialogs use `.modal-backdrop` — see Task 6's note on which pattern applies.
- Every documentation file (except README.md) needs a `*Last modified: YYYY-MM-DD*` line, updated on every edit.
- `CHANGELOG.md` entries go under `## [Unreleased]`, one `### <Type>` heading per version block (merge into existing heading, don't duplicate).
- Package names: `internal/channels` → `package channels`; `internal/api/library` → `package apilibrary` (imported as `apilibrary` in `internal/api/routes.go`, but files inside the package itself just say `package apilibrary` with no import alias needed); `internal/library` → `package library`, imported as `lib "huepattl.de/unterlumen/internal/library"` in `apilibrary` files.

---

## Task 1: Draft data model (`internal/channels`)

**Files:**
- Create: `src/internal/channels/draft.go`
- Test: `src/internal/channels/draft_test.go`

**Interfaces:**
- Consumes: `channels.Store.OutputDir(slug string) string` (existing, `src/internal/channels/store.go:50`).
- Produces (used by Task 2 and Task 3):
  - `type DraftPhoto struct { LibraryID, PhotoID string }`
  - `type DraftTarget struct { PostID, Title string; Unlisted bool; Account string }`
  - `type Draft struct { ID string; Target DraftTarget; Photos []DraftPhoto }`
  - `func NewDraftStore(channelStore *Store) *DraftStore`
  - `func (s *DraftStore) List(slug string) ([]*Draft, error)`
  - `func (s *DraftStore) Get(slug, draftID string) (*Draft, error)`
  - `func (s *DraftStore) Create(slug string, target DraftTarget, photos []DraftPhoto) (*Draft, error)`
  - `func (s *DraftStore) AppendPhotos(slug, draftID string, photos []DraftPhoto) (*Draft, error)`
  - `func (s *DraftStore) RemovePhoto(slug, draftID, libraryID, photoID string) (*Draft, error)` — returns `nil, nil` if the draft became empty and was deleted.
  - `func (s *DraftStore) Delete(slug, draftID string) error`

- [ ] **Step 1: Write the failing tests**

```go
// src/internal/channels/draft_test.go
package channels

import (
	"path/filepath"
	"testing"
)

func newTestDraftStore(t *testing.T) *DraftStore {
	t.Helper()
	dir := t.TempDir()
	return NewDraftStore(NewStore(dir, dir))
}

func TestDraftStore_CreateAndList(t *testing.T) {
	s := newTestDraftStore(t)
	d, err := s.Create("website", DraftTarget{Title: "Summer 2026"}, []DraftPhoto{{LibraryID: "lib1", PhotoID: "p1"}})
	if err != nil {
		t.Fatalf("Create: %v", err)
	}
	if d.ID == "" {
		t.Fatal("expected a non-empty draft ID")
	}
	drafts, err := s.List("website")
	if err != nil {
		t.Fatalf("List: %v", err)
	}
	if len(drafts) != 1 || len(drafts[0].Photos) != 1 {
		t.Fatalf("expected 1 draft with 1 photo, got %+v", drafts)
	}
	if drafts[0].Target.Title != "Summer 2026" {
		t.Fatalf("target title = %q, want %q", drafts[0].Target.Title, "Summer 2026")
	}
}

func TestDraftStore_AppendPhotos(t *testing.T) {
	s := newTestDraftStore(t)
	d, _ := s.Create("website", DraftTarget{Title: "Summer 2026"}, []DraftPhoto{{LibraryID: "lib1", PhotoID: "p1"}})
	updated, err := s.AppendPhotos("website", d.ID, []DraftPhoto{{LibraryID: "lib1", PhotoID: "p2"}, {LibraryID: "lib2", PhotoID: "p3"}})
	if err != nil {
		t.Fatalf("AppendPhotos: %v", err)
	}
	if len(updated.Photos) != 3 {
		t.Fatalf("expected 3 photos after append, got %d", len(updated.Photos))
	}
	if _, err := s.AppendPhotos("website", "does-not-exist", nil); err == nil {
		t.Fatal("expected error appending to a missing draft")
	}
}

func TestDraftStore_RemovePhoto(t *testing.T) {
	s := newTestDraftStore(t)
	d, _ := s.Create("website", DraftTarget{Title: "Summer 2026"}, []DraftPhoto{
		{LibraryID: "lib1", PhotoID: "p1"},
		{LibraryID: "lib1", PhotoID: "p2"},
	})

	remaining, err := s.RemovePhoto("website", d.ID, "lib1", "p1")
	if err != nil {
		t.Fatalf("RemovePhoto: %v", err)
	}
	if remaining == nil || len(remaining.Photos) != 1 || remaining.Photos[0].PhotoID != "p2" {
		t.Fatalf("expected 1 photo (p2) remaining, got %+v", remaining)
	}

	remaining, err = s.RemovePhoto("website", d.ID, "lib1", "p2")
	if err != nil {
		t.Fatalf("RemovePhoto (last): %v", err)
	}
	if remaining != nil {
		t.Fatalf("expected draft to be deleted once empty, got %+v", remaining)
	}
	drafts, _ := s.List("website")
	if len(drafts) != 0 {
		t.Fatalf("expected 0 drafts after removing the last photo, got %d", len(drafts))
	}
}

func TestDraftStore_Delete(t *testing.T) {
	s := newTestDraftStore(t)
	d, _ := s.Create("website", DraftTarget{Title: "Summer 2026"}, []DraftPhoto{{LibraryID: "lib1", PhotoID: "p1"}})
	if err := s.Delete("website", d.ID); err != nil {
		t.Fatalf("Delete: %v", err)
	}
	drafts, _ := s.List("website")
	if len(drafts) != 0 {
		t.Fatalf("expected 0 drafts after Delete, got %d", len(drafts))
	}
}

func TestDraftStore_TwoChannelsDoNotShareDrafts(t *testing.T) {
	s := newTestDraftStore(t)
	s.Create("website", DraftTarget{Title: "A"}, []DraftPhoto{{LibraryID: "lib1", PhotoID: "p1"}})
	s.Create("instagram", DraftTarget{Title: "B"}, []DraftPhoto{{LibraryID: "lib1", PhotoID: "p2"}})
	websiteDrafts, _ := s.List("website")
	instaDrafts, _ := s.List("instagram")
	if len(websiteDrafts) != 1 || len(instaDrafts) != 1 {
		t.Fatalf("expected 1 draft per channel, got website=%d instagram=%d", len(websiteDrafts), len(instaDrafts))
	}
	if _, statErr := filepath.Abs(s.path("website")); statErr != nil {
		t.Fatalf("path: %v", statErr)
	}
}
```

- [ ] **Step 2: Run tests to verify they fail**

Run: `cd src && go test ./internal/channels/... -run TestDraftStore -v`
Expected: FAIL — `DraftStore`, `DraftPhoto`, `DraftTarget`, `NewDraftStore` undefined.

- [ ] **Step 3: Write the implementation**

```go
// src/internal/channels/draft.go
package channels

import (
	"crypto/rand"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
)

// DraftPhoto references one photo, in one library, pending export to a channel.
// Nothing is exported and no file is written for a photo until Generate runs —
// this is purely an association recorded ahead of time so photos can be
// collected into a gallery incrementally, across sessions and libraries.
type DraftPhoto struct {
	LibraryID string `json:"libraryID"`
	PhotoID   string `json:"photoID"`
}

// DraftTarget identifies where a draft's photos will land once generated:
// either an existing gallery/album (PostID set) or a brand-new one (Title set).
// Mirrors the addToExisting/new-gallery distinction already used by the
// existing build pipeline (internal/api/library/handler.go's buildPhotos).
type DraftTarget struct {
	PostID   string `json:"postID,omitempty"`   // non-empty = add to an existing gallery/album
	Title    string `json:"title,omitempty"`    // new gallery/album title; ignored if PostID is set
	Unlisted bool   `json:"unlisted,omitempty"` // site-export only; fixed at draft creation
	Account  string `json:"account,omitempty"`
}

// Draft is one pending collection of photos for a channel, not yet generated.
type Draft struct {
	ID     string       `json:"id"`
	Target DraftTarget  `json:"target"`
	Photos []DraftPhoto `json:"photos"`
}

// DraftStore manages drafts.json, one per channel output directory
// (sibling to that channel's gallery.json/site.json statefiles).
type DraftStore struct {
	channelStore *Store
	mu           sync.Mutex
}

// NewDraftStore creates a DraftStore backed by channelStore's output directories.
func NewDraftStore(channelStore *Store) *DraftStore {
	return &DraftStore{channelStore: channelStore}
}

func (s *DraftStore) path(slug string) string {
	return filepath.Join(s.channelStore.OutputDir(slug), "drafts.json")
}

// List returns all pending drafts for a channel. Returns an empty slice
// (never nil) when the channel has no drafts.json yet.
func (s *DraftStore) List(slug string) ([]*Draft, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	drafts, err := s.loadLocked(slug)
	if err != nil {
		return nil, err
	}
	if drafts == nil {
		drafts = []*Draft{}
	}
	return drafts, nil
}

// Get returns one draft by ID.
func (s *DraftStore) Get(slug, draftID string) (*Draft, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	drafts, err := s.loadLocked(slug)
	if err != nil {
		return nil, err
	}
	for _, d := range drafts {
		if d.ID == draftID {
			return d, nil
		}
	}
	return nil, fmt.Errorf("draft %q not found", draftID)
}

// Create starts a new draft for slug with the given target and initial photos.
func (s *DraftStore) Create(slug string, target DraftTarget, photos []DraftPhoto) (*Draft, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	drafts, err := s.loadLocked(slug)
	if err != nil {
		return nil, err
	}
	d := &Draft{ID: newDraftID(), Target: target, Photos: photos}
	drafts = append(drafts, d)
	if err := s.writeLocked(slug, drafts); err != nil {
		return nil, err
	}
	return d, nil
}

// AppendPhotos adds photos to an existing draft.
func (s *DraftStore) AppendPhotos(slug, draftID string, photos []DraftPhoto) (*Draft, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	drafts, err := s.loadLocked(slug)
	if err != nil {
		return nil, err
	}
	for _, d := range drafts {
		if d.ID == draftID {
			d.Photos = append(d.Photos, photos...)
			if err := s.writeLocked(slug, drafts); err != nil {
				return nil, err
			}
			return d, nil
		}
	}
	return nil, fmt.Errorf("draft %q not found", draftID)
}

// RemovePhoto removes one photo from a draft. If the draft becomes empty it is
// deleted and (nil, nil) is returned; otherwise the updated draft is returned.
func (s *DraftStore) RemovePhoto(slug, draftID, libraryID, photoID string) (*Draft, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	drafts, err := s.loadLocked(slug)
	if err != nil {
		return nil, err
	}
	for i, d := range drafts {
		if d.ID != draftID {
			continue
		}
		filtered := d.Photos[:0]
		for _, p := range d.Photos {
			if !(p.LibraryID == libraryID && p.PhotoID == photoID) {
				filtered = append(filtered, p)
			}
		}
		d.Photos = filtered
		if len(d.Photos) == 0 {
			drafts = append(drafts[:i], drafts[i+1:]...)
			if err := s.writeLocked(slug, drafts); err != nil {
				return nil, err
			}
			return nil, nil
		}
		if err := s.writeLocked(slug, drafts); err != nil {
			return nil, err
		}
		return d, nil
	}
	return nil, fmt.Errorf("draft %q not found", draftID)
}

// Delete removes an entire draft, discarding its pending photos.
func (s *DraftStore) Delete(slug, draftID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	drafts, err := s.loadLocked(slug)
	if err != nil {
		return err
	}
	filtered := drafts[:0]
	for _, d := range drafts {
		if d.ID != draftID {
			filtered = append(filtered, d)
		}
	}
	return s.writeLocked(slug, filtered)
}

func newDraftID() string {
	b := make([]byte, 8)
	rand.Read(b) //nolint:errcheck
	return fmt.Sprintf("%x", b)
}

func (s *DraftStore) loadLocked(slug string) ([]*Draft, error) {
	data, err := os.ReadFile(s.path(slug))
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var drafts []*Draft
	return drafts, json.Unmarshal(data, &drafts)
}

func (s *DraftStore) writeLocked(slug string, drafts []*Draft) error {
	p := s.path(slug)
	if err := os.MkdirAll(filepath.Dir(p), 0o700); err != nil {
		return err
	}
	data, err := json.MarshalIndent(drafts, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(p, data, 0o600)
}
```

- [ ] **Step 4: Run tests to verify they pass**

Run: `cd src && go test ./internal/channels/... -v`
Expected: PASS (all `TestDraftStore_*` plus pre-existing `store_test.go` tests).

- [ ] **Step 5: `go vet` and commit**

```bash
cd src && go vet ./...
git add src/internal/channels/draft.go src/internal/channels/draft_test.go
git commit -m "feat: add channel draft store for deferred publish collection"
```

---

## Task 2: Collect / list / delete draft HTTP endpoints

**Files:**
- Create: `src/internal/api/library/drafts.go`
- Test: `src/internal/api/library/drafts_test.go`
- Modify: `src/internal/api/library/handler.go:35` (`Handle` signature + route registration)
- Modify: `src/internal/api/routes.go:47-52` (pass a `*channels.DraftStore` into `apilibrary.Handle`)

**Interfaces:**
- Consumes: `channels.DraftStore` from Task 1; `lib.Manager.OpenStore(id string) (*lib.Store, error)` (existing, `src/internal/library/manager.go:204`) — used only to validate the library exists at collect time (photos aren't read yet).
- Produces (used by Task 3's frontend work and by Task 4):
  - Route `POST /api/library/{id}/channels/{slug}/drafts` → creates or appends to a draft; body `{photoIDs: string[], draftID?, postID?, title?, unlisted?, account?}`; returns the `channels.Draft` JSON.
  - Route `GET /api/channels/{slug}/drafts` → `[]*channels.Draft` JSON (never null).
  - Route `DELETE /api/channels/{slug}/drafts/{draftID}` → 204.
  - Route `DELETE /api/channels/{slug}/drafts/{draftID}/photos/{libID}/{photoID}` → 204 if the draft still exists, else the updated `channels.Draft` JSON if photos remain (matches `DraftStore.RemovePhoto`'s "nil = deleted" contract, inverted for the two response shapes — see implementation below for the exact rule: **200 + Draft body** when photos remain, **204** when the draft was deleted).

- [ ] **Step 1: Write the failing tests**

```go
// src/internal/api/library/drafts_test.go
package apilibrary

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"

	"huepattl.de/unterlumen/internal/channels"
	lib "huepattl.de/unterlumen/internal/library"
)

// setupDraftTestMux mirrors the wiring in routes.go for the drafts endpoints only.
func setupDraftTestMux(t *testing.T) (*http.ServeMux, *channels.DraftStore) {
	t.Helper()
	dir := t.TempDir()
	mgr, err := lib.NewManager(filepath.Join(dir, "libs.json"))
	if err != nil {
		t.Fatalf("NewManager: %v", err)
	}
	chStore := channels.NewStore(dir, dir)
	chStore.Save(&channels.Channel{Slug: "website", Name: "Website", Format: "jpeg", Quality: 85, GalleryExport: true}) //nolint:errcheck
	draftStore := channels.NewDraftStore(chStore)
	mux := http.NewServeMux()
	registerDraftRoutes(mux, mgr, chStore, draftStore)
	return mux, draftStore
}

func TestCollectDraft_CreatesNewDraft(t *testing.T) {
	mux, draftStore := setupDraftTestMux(t)
	body, _ := json.Marshal(map[string]any{"photoIDs": []string{"p1", "p2"}, "title": "Summer 2026"})
	req := httptest.NewRequest("POST", "/api/library/lib1/channels/website/drafts", bytes.NewReader(body))
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
	var draft channels.Draft
	if err := json.Unmarshal(rec.Body.Bytes(), &draft); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(draft.Photos) != 2 || draft.Photos[0].LibraryID != "lib1" {
		t.Fatalf("unexpected draft: %+v", draft)
	}

	drafts, _ := draftStore.List("website")
	if len(drafts) != 1 {
		t.Fatalf("expected 1 persisted draft, got %d", len(drafts))
	}
}

func TestCollectDraft_AppendsToExistingByDraftID(t *testing.T) {
	mux, draftStore := setupDraftTestMux(t)
	d, _ := draftStore.Create("website", channels.DraftTarget{Title: "Summer 2026"}, nil)

	body, _ := json.Marshal(map[string]any{"photoIDs": []string{"p1"}, "draftID": d.ID})
	req := httptest.NewRequest("POST", "/api/library/lib1/channels/website/drafts", bytes.NewReader(body))
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

func TestListDrafts_EmptyIsEmptyArrayNotNull(t *testing.T) {
	mux, _ := setupDraftTestMux(t)
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
	mux, draftStore := setupDraftTestMux(t)
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
	mux, draftStore := setupDraftTestMux(t)
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
```

Note: `lib.NewManager` — check its real signature in `src/internal/library/manager.go` before writing this test file for real (the constructor name/args used above is a placeholder guess for a fresh Manager backed by a temp libs.json; look at how existing tests in `src/internal/api/library/handler_test.go` construct a `*lib.Manager` for tests and copy that exact pattern instead of inventing a new one).

- [ ] **Step 2: Run tests to verify they fail**

Run: `cd src && go test ./internal/api/library/... -run 'TestCollectDraft|TestListDrafts|TestDeleteDraft|TestRemoveDraftPhoto' -v`
Expected: FAIL — `registerDraftRoutes` undefined.

- [ ] **Step 3: Write the implementation**

```go
// src/internal/api/library/drafts.go
package apilibrary

import (
	"encoding/json"
	"net/http"

	"huepattl.de/unterlumen/internal/channels"
	lib "huepattl.de/unterlumen/internal/library"
)

// registerDraftRoutes wires the collect/list/delete draft endpoints onto mux.
// Split out from Handle (handler.go) because drafts are a self-contained
// concern layered on top of the channel store, not core library CRUD.
func registerDraftRoutes(mux *http.ServeMux, mgr *lib.Manager, chStore *channels.Store, draftStore *channels.DraftStore) {
	mux.HandleFunc("POST /api/library/{id}/channels/{slug}/drafts", collectDraft(mgr, chStore, draftStore))
	mux.HandleFunc("GET /api/channels/{slug}/drafts", listDrafts(draftStore))
	mux.HandleFunc("DELETE /api/channels/{slug}/drafts/{draftID}", deleteDraft(draftStore))
	mux.HandleFunc("DELETE /api/channels/{slug}/drafts/{draftID}/photos/{libID}/{photoID}", removeDraftPhoto(draftStore))
}

func collectDraft(mgr *lib.Manager, chStore *channels.Store, draftStore *channels.DraftStore) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if chStore == nil || draftStore == nil {
			http.Error(w, "channel store not available", http.StatusServiceUnavailable)
			return
		}
		libID := r.PathValue("id")
		slug := r.PathValue("slug")

		var body struct {
			PhotoIDs []string `json:"photoIDs"`
			DraftID  string   `json:"draftID,omitempty"`
			PostID   string   `json:"postID,omitempty"`
			Title    string   `json:"title,omitempty"`
			Unlisted bool     `json:"unlisted,omitempty"`
			Account  string   `json:"account,omitempty"`
		}
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			http.Error(w, "invalid JSON", http.StatusBadRequest)
			return
		}
		if len(body.PhotoIDs) == 0 {
			http.Error(w, "photoIDs required", http.StatusBadRequest)
			return
		}
		if _, err := chStore.Get(slug); err != nil {
			http.Error(w, "channel not found: "+err.Error(), http.StatusBadRequest)
			return
		}
		if _, err := mgr.OpenStore(libID); err != nil {
			http.Error(w, "library not found", http.StatusNotFound)
			return
		}

		photos := make([]channels.DraftPhoto, len(body.PhotoIDs))
		for i, id := range body.PhotoIDs {
			photos[i] = channels.DraftPhoto{LibraryID: libID, PhotoID: id}
		}

		var (
			draft *channels.Draft
			err   error
		)
		if body.DraftID != "" {
			draft, err = draftStore.AppendPhotos(slug, body.DraftID, photos)
		} else {
			draft, err = draftStore.Create(slug, channels.DraftTarget{
				PostID: body.PostID, Title: body.Title, Unlisted: body.Unlisted, Account: body.Account,
			}, photos)
		}
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}

		if store, openErr := mgr.OpenStore(libID); openErr == nil {
			defer store.Close()
			for _, id := range body.PhotoIDs {
				store.UpsertMeta(id, "pending:"+slug, draft.ID) //nolint:errcheck
			}
		}

		writeJSON(w, draft)
	}
}

func listDrafts(draftStore *channels.DraftStore) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		drafts, err := draftStore.List(r.PathValue("slug"))
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		writeJSON(w, drafts)
	}
}

func deleteDraft(draftStore *channels.DraftStore) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if err := draftStore.Delete(r.PathValue("slug"), r.PathValue("draftID")); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		w.WriteHeader(http.StatusNoContent)
	}
}

func removeDraftPhoto(draftStore *channels.DraftStore) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		draft, err := draftStore.RemovePhoto(
			r.PathValue("slug"), r.PathValue("draftID"), r.PathValue("libID"), r.PathValue("photoID"),
		)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		if draft == nil {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		writeJSON(w, draft)
	}
}
```

Then wire it into `Handle` (`src/internal/api/library/handler.go`):

```go
// handler.go:35 — add a draftStore parameter
func Handle(mux *http.ServeMux, mgr *lib.Manager, imgCache *media.ImageCache, root string, serverRole bool, chStore *channels.Store, draftStore *channels.DraftStore) {
	// ... existing routes unchanged ...
	registerDraftRoutes(mux, mgr, chStore, draftStore) // add at the end of Handle, after existing route registrations
}
```

And in `src/internal/api/routes.go`:

```go
// routes.go:47-52 — construct the draft store alongside chStore and pass it through
if chStore != nil {
	apichannels.Handle(mux, chStore)
}
draftStore := channels.NewDraftStore(chStore) // safe to construct even if chStore is nil; unused if libMgr is nil
if libMgr != nil {
	apilibrary.Handle(mux, libMgr, imageCache, boundary, serverRole, chStore, draftStore)
}
```

Check `channels.NewDraftStore`/`DraftStore` methods for a nil-`channelStore` receiver before relying on "safe to construct" above — if `NewStore`'s zero value isn't nil-safe, guard with `if chStore != nil { draftStore = channels.NewDraftStore(chStore) }` instead and pass a possibly-nil `*channels.DraftStore` through (matching how `chStore` itself is already threaded through as possibly-nil).

- [ ] **Step 4: Run tests to verify they pass**

Run: `cd src && go build ./... && go test ./internal/api/library/... ./internal/channels/... -v`
Expected: PASS.

- [ ] **Step 5: `go vet` and commit**

```bash
cd src && go vet ./...
git add src/internal/api/library/drafts.go src/internal/api/library/drafts_test.go \
        src/internal/api/library/handler.go src/internal/api/routes.go
git commit -m "feat: add collect/list/delete HTTP endpoints for channel drafts"
```

---

## Task 3: Generate endpoint (refactor `buildPhotos` into a draft-driven, multi-library pipeline)

This is the highest-risk task — it repurposes the existing `/api/library/{id}/build` gallery/site pipeline (`handler.go:1487-1856`, already read in full during planning) to run against a draft's photos instead of an immediate request body, across however many distinct libraries the draft's photos came from.

**Files:**
- Modify: `src/internal/api/library/handler.go` — remove the `POST /api/library/{id}/build` route and the `buildPhotos` function; add `generateDraft` in its place (same file, since it depends on the unexported `GalleryState`/`SiteAlbum`/`GalleryItem`/`buildResult` types and `loadGalleryState`/`saveGalleryState`/`loadSiteState`/`saveSiteState`/`computeSlug`/`albumFolderName`/`dateRangeStr`/`buildSiteNavContext`/`newPostID`/`createGalleryZip`/`writeGalleryAssets`/`writeSiteAssets`/generate*Page helpers already living in this package).
- Modify: `src/internal/api/library/handler.go:35-82` (`Handle`) — remove the `buildPhotos` route registration, add the `generateDraft` route.
- Keep unchanged: `buildDownload` (`handler.go:1861+`) and its route — the redesign doesn't touch the "download as ZIP" utility path, which was never part of the channel-publish lifecycle being redesigned here (YAGNI: nobody asked for it to change).
- Test: extend `src/internal/api/library/handler_test.go` (already has build-related tests per `git status`; read it first to match its existing test-setup helpers rather than duplicating them).

**Interfaces:**
- Consumes: `channels.DraftStore.Get(slug, draftID)` (Task 1), `buildOne` (existing, `handler.go:2015`), `mgr.OpenStore(libraryID)` (existing).
- Produces: Route `POST /api/channels/{slug}/drafts/{draftID}/generate`, body `{publishedAt?: string}` (RFC3339; defaults to now). Response: for plain-export channels, synchronous `{"postID": "...", "results": [...]}`; for gallery/site-export channels, an SSE stream identical in shape to today's (`{"step": ..., "done": ..., "total": ..., "file": ...}` progress events, terminated by `{"complete": true, "postID", "galleryPath", ["sitePath"], "results"}`).

**Scope decisions locked in by this task** (apply these while adapting the existing code, don't ask — they follow directly from the approved design):
- No per-generate `outputPath` override — always use `chStore.OutputDir(slug)`. The old `body.OutputPath` handling (`handler.go:1553-1568`) is dropped entirely.
- `recordXMP` is always `true` — the Info Panel's pending→published transition (Task 8) depends on `built:<slug>` always being written on generate. The old `RecordXMP *bool` toggle is dropped.
- A draft's `Account` (`draft.Target.Account`) replaces `body.Account`; validate it against the channel exactly as `buildPhotos` did (`handler.go:1520-1523`).
- On success, delete the draft (`draftStore.Delete`) and delete each processed photo's `pending:<slug>` meta key (the pairing write to `pending:<slug>` happens in Task 2's `collectDraft`).

- [ ] **Step 1: Read the existing test setup**

Read `src/internal/api/library/handler_test.go` in full before writing new tests — it already has a working `*lib.Manager` + `*channels.Store` test harness (used for existing `buildPhotos`-adjacent tests per the current `git status` diff) and a `TestBuildPhotos`-style test to model the new `TestGenerateDraft` tests on, including however it fakes/seeds library photos (this project doesn't have real photo files in its Go test fixtures — check how the existing tests get a photo ID that `buildOne`/`store.GetPhotoPathHint` can resolve, and copy that fixture pattern exactly).

- [ ] **Step 2: Write the failing tests**

Add to `handler_test.go` (adapt the exact harness helpers found in Step 1 — the sketch below shows the required *behavioral* coverage, not the literal fixture plumbing, which must match whatever the file already does):

```go
func TestGenerateDraft_PlainChannel_ExportsAndClearsDraft(t *testing.T) {
	// Arrange: a plain (no galleryExport/siteExport) channel, a library with
	// one real seeded photo, and a draft collecting that photo.
	// ... use the harness from Step 1 to get mgr, chStore, draftStore, libID, photoID ...

	draft, err := draftStore.Create("instagram", channels.DraftTarget{}, []channels.DraftPhoto{{LibraryID: libID, PhotoID: photoID}})
	if err != nil { t.Fatalf("Create draft: %v", err) }

	req := httptest.NewRequest("POST", "/api/channels/instagram/drafts/"+draft.ID+"/generate", strings.NewReader("{}"))
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}

	// Draft is gone.
	if _, err := draftStore.Get("instagram", draft.ID); err == nil {
		t.Fatal("expected draft to be deleted after generate")
	}
	// pending: meta cleared, built: meta written.
	store, _ := mgr.OpenStore(libID)
	defer store.Close()
	entries, _ := store.GetMeta(photoID)
	var hasPending, hasBuilt bool
	for _, e := range entries {
		if e.Key == "pending:instagram" { hasPending = true }
		if e.Key == "built:instagram" { hasBuilt = true }
	}
	if hasPending { t.Error("expected pending:instagram meta to be cleared") }
	if !hasBuilt { t.Error("expected built:instagram meta to be written") }
}

func TestGenerateDraft_GalleryChannel_CreatesGalleryAndClearsDraft(t *testing.T) {
	// Same shape as above but against a galleryExport channel; assert the
	// SSE stream's final event has `complete: true` and a non-empty `galleryPath`,
	// and that gallery.json exists on disk at that path afterward.
}

func TestGenerateDraft_MultiLibraryDraft_MergesAllPhotos(t *testing.T) {
	// Seed two separate libraries with one photo each, collect both into the
	// same draft (two collectDraft calls with different libIDs), generate,
	// and assert the resulting gallery/site statefile has PhotoCount == 2.
}

func TestGenerateDraft_UnknownDraft_Returns400(t *testing.T) {
	req := httptest.NewRequest("POST", "/api/channels/instagram/drafts/does-not-exist/generate", strings.NewReader("{}"))
	rec := httptest.NewRecorder()
	mux.ServeHTTP(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want 400", rec.Code)
	}
}
```

- [ ] **Step 3: Run tests to verify they fail**

Run: `cd src && go test ./internal/api/library/... -run TestGenerateDraft -v`
Expected: FAIL — route not registered / `generateDraft` undefined.

- [ ] **Step 4: Implement `generateDraft` by adapting `buildPhotos`**

Delete the `buildPhotos` function and its route registration (`handler.go:35-82`'s `mux.HandleFunc("POST /api/library/{id}/build", ...)` line, and the function body at `handler.go:1487-1856`). Replace with:

```go
func generateDraft(mgr *lib.Manager, chStore *channels.Store, draftStore *channels.DraftStore) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if chStore == nil || draftStore == nil {
			http.Error(w, "channel store not available", http.StatusServiceUnavailable)
			return
		}
		slug := r.PathValue("slug")
		draftID := r.PathValue("draftID")

		var body struct {
			PublishedAt string `json:"publishedAt,omitempty"`
		}
		json.NewDecoder(r.Body).Decode(&body) //nolint:errcheck // empty body is valid; PublishedAt defaults below

		ch, err := chStore.Get(slug)
		if err != nil {
			http.Error(w, "channel not found: "+err.Error(), http.StatusBadRequest)
			return
		}
		draft, err := draftStore.Get(slug, draftID)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		if draft.Target.Account != "" && ch.AccountByID(draft.Target.Account) == nil {
			http.Error(w, "account not found: "+draft.Target.Account, http.StatusBadRequest)
			return
		}

		publishedAt := time.Now().UTC()
		if body.PublishedAt != "" {
			if t, parseErr := time.Parse(time.RFC3339, body.PublishedAt); parseErr == nil {
				publishedAt = t
			}
		}

		// Open one *lib.Store per distinct library referenced by the draft —
		// a draft's photos can span multiple libraries, collected across
		// separate sessions (ADR-0016: channel output is library-independent).
		stores := map[string]*lib.Store{}
		defer func() {
			for _, s := range stores {
				s.Close()
			}
		}()
		for _, dp := range draft.Photos {
			if _, ok := stores[dp.LibraryID]; ok {
				continue
			}
			s, openErr := mgr.OpenStore(dp.LibraryID)
			if openErr != nil {
				http.Error(w, "library not found: "+dp.LibraryID, http.StatusNotFound)
				return
			}
			stores[dp.LibraryID] = s
		}

		postID := newPostID()
		pub := media.Publication{
			Channel: slug, Account: draft.Target.Account, PostID: postID,
			GalleryTitle: draft.Target.Title, PublishedAt: publishedAt,
		}
		ts := publishedAt.UTC().Format("20060102T150405Z")

		addToExisting := draft.Target.PostID != ""
		galleryMode := ch.GalleryExport && (draft.Target.Title != "" || addToExisting)
		siteMode := ch.SiteExport && (draft.Target.Title != "" || addToExisting)
		channelDir := chStore.OutputDir(slug)

		albumPostID := postID
		albumSlug := ""
		var existingPhotos []SitePhoto
		var existingTitle string
		var existingPublishedAt time.Time
		var existingUnlisted bool

		outDir := channelDir
		if addToExisting {
			albumPostID = draft.Target.PostID
			if galleryMode {
				outDir = filepath.Join(channelDir, albumPostID)
				gs, gsErr := loadGalleryState(filepath.Join(outDir, "gallery.json"))
				if gsErr != nil || gs == nil {
					http.Error(w, "gallery not found: "+albumPostID, http.StatusBadRequest)
					return
				}
				existingPhotos, existingTitle, existingPublishedAt = gs.Photos, gs.Title, gs.PublishedAt
			} else if siteMode {
				siteAlbums, stateErr := loadSiteState(filepath.Join(channelDir, "site", "site.json"))
				if stateErr != nil {
					http.Error(w, "read site state: "+stateErr.Error(), http.StatusInternalServerError)
					return
				}
				for i := range siteAlbums {
					if siteAlbums[i].PostID == albumPostID {
						existingPhotos = siteAlbums[i].Photos
						existingTitle = siteAlbums[i].Title
						existingPublishedAt = siteAlbums[i].PublishedAt
						existingUnlisted = siteAlbums[i].Unlisted
						albumSlug = albumFolderName(siteAlbums[i])
						break
					}
				}
				if existingTitle == "" {
					http.Error(w, "album not found: "+albumPostID, http.StatusBadRequest)
					return
				}
				outDir = filepath.Join(channelDir, "site", "albums", albumSlug)
			}
			if _, statErr := os.Stat(outDir); os.IsNotExist(statErr) {
				http.Error(w, "gallery folder not found: "+albumPostID, http.StatusBadRequest)
				return
			}
		} else {
			if galleryMode {
				outDir = filepath.Join(outDir, albumPostID)
			} else if siteMode {
				existingAlbums, _ := loadSiteState(filepath.Join(channelDir, "site", "site.json"))
				albumSlug = computeSlug(draft.Target.Title, publishedAt, existingAlbums, draft.Target.Unlisted)
				outDir = filepath.Join(channelDir, "site", "albums", albumSlug)
			}
		}
		if err := os.MkdirAll(outDir, 0o700); err != nil {
			http.Error(w, "create output dir: "+err.Error(), http.StatusInternalServerError)
			return
		}
		if existingTitle != "" && pub.GalleryTitle == "" {
			pub.GalleryTitle = existingTitle
		}

		clearDraft := func() {
			draftStore.Delete(slug, draftID) //nolint:errcheck
			for _, dp := range draft.Photos {
				if s, ok := stores[dp.LibraryID]; ok {
					s.DeleteMeta(dp.PhotoID, "pending:"+slug) //nolint:errcheck
				}
			}
		}

		if !galleryMode && !siteMode {
			var results []buildResult
			for _, dp := range draft.Photos {
				res := buildOne(stores[dp.LibraryID], ch, pub, ts, outDir, "", dp.PhotoID, true)
				results = append(results, res)
			}
			clearDraft()
			writeJSON(w, map[string]any{"postID": postID, "results": results})
			return
		}

		thumbDir := filepath.Join(outDir, "thumbs")
		if err := os.MkdirAll(thumbDir, 0o700); err != nil {
			http.Error(w, "create thumbs dir: "+err.Error(), http.StatusInternalServerError)
			return
		}

		flusher, ok := w.(http.Flusher)
		if !ok {
			http.Error(w, "streaming not supported", http.StatusInternalServerError)
			return
		}
		w.Header().Set("Content-Type", "text/event-stream")
		w.Header().Set("Cache-Control", "no-cache")
		w.Header().Set("X-Accel-Buffering", "no")
		w.WriteHeader(http.StatusOK)
		emit := func(v any) {
			data, _ := json.Marshal(v)
			fmt.Fprintf(w, "data: %s\n\n", data)
			flusher.Flush()
		}

		total := len(draft.Photos)
		var results []buildResult
		for i, dp := range draft.Photos {
			res := buildOne(stores[dp.LibraryID], ch, pub, ts, outDir, thumbDir, dp.PhotoID, true)
			results = append(results, res)
			emit(map[string]any{"step": "photo", "done": i + 1, "total": total, "file": res.Filename})
		}

		var items []GalleryItem
		for _, ep := range existingPhotos {
			items = append(items, GalleryItem{PhotoID: ep.PhotoID, Filename: ep.Filename, ThumbFilename: ep.ThumbFilename})
		}
		for _, res := range results {
			if res.Error == "" && res.Filename != "" {
				items = append(items, GalleryItem{
					PhotoID: res.PhotoID, Filename: res.Filename, ThumbFilename: res.ThumbFilename,
					Width: res.Width, Height: res.Height,
				})
			}
		}

		var zipResults []buildResult
		for _, ep := range existingPhotos {
			zipResults = append(zipResults, buildResult{Filename: ep.Filename})
		}
		zipResults = append(zipResults, results...)

		emit(map[string]any{"step": "zip", "done": 0, "total": 1, "file": "Creating ZIP…"})
		zipName := "photos.zip"
		if zipErr := createGalleryZip(zipResults, outDir, zipName); zipErr != nil {
			emit(map[string]any{"step": "zip", "done": 0, "total": 1, "file": "ZIP failed: " + zipErr.Error()})
			zipName = ""
		} else {
			emit(map[string]any{"step": "zip", "done": 1, "total": 1, "file": "ZIP ready"})
		}

		galleryTitle := draft.Target.Title
		if existingTitle != "" {
			galleryTitle = existingTitle
		}

		albumPublishedAt := publishedAt
		var albumUpdatedAt time.Time
		if !existingPublishedAt.IsZero() {
			albumPublishedAt = existingPublishedAt
			albumUpdatedAt = publishedAt
		}
		dateStr := dateRangeStr(albumPublishedAt, albumUpdatedAt)

		albumUnlisted := draft.Target.Unlisted
		if addToExisting {
			albumUnlisted = existingUnlisted
		}

		emit(map[string]any{"step": "html", "done": 0, "total": 1, "file": "Generating gallery…"})
		var html []byte
		if siteMode {
			html = GenerateSiteGallery(galleryTitle, ch.SiteTheme, items, GalleryOptions{
				ZipFilename: zipName, SiteTitle: ch.SiteTitle, DateStr: dateStr, SiteURL: ch.SiteURL,
				AlbumSlug: albumSlug, PublishedAt: albumPublishedAt, Unlisted: albumUnlisted,
				Nav: buildSiteNavContext(ch, filepath.Join(channelDir, "site"), false),
			})
		} else {
			html = GenerateGallery(galleryTitle, items, GalleryOptions{ZipFilename: zipName, DateStr: dateStr})
		}
		indexPath := filepath.Join(outDir, "index.html")
		if err := os.WriteFile(indexPath, html, 0o644); err != nil {
			emit(map[string]any{"error": "write gallery: " + err.Error()})
			return
		}
		if galleryMode {
			if err := writeGalleryAssets(outDir); err != nil {
				emit(map[string]any{"error": "write gallery assets: " + err.Error()})
				return
			}
		}

		if galleryMode && !siteMode {
			gsPublishedAt := publishedAt
			var gsUpdatedAt time.Time
			if !existingPublishedAt.IsZero() {
				gsPublishedAt = existingPublishedAt
				gsUpdatedAt = publishedAt
			}
			sitePhotos := make([]SitePhoto, len(items))
			for i, item := range items {
				sitePhotos[i] = SitePhoto{PhotoID: item.PhotoID, Filename: item.Filename, ThumbFilename: item.ThumbFilename}
			}
			gs := &GalleryState{
				PostID: albumPostID, Title: galleryTitle, PublishedAt: gsPublishedAt, UpdatedAt: gsUpdatedAt,
				PhotoCount: len(items), HasZip: zipName != "", Photos: sitePhotos,
			}
			saveGalleryState(filepath.Join(outDir, "gallery.json"), gs) //nolint:errcheck
		}

		if siteMode && len(items) > 0 {
			emit(map[string]any{"step": "site", "done": 0, "total": 1, "file": "Updating site index…"})
			siteDir := filepath.Join(channelDir, "site")
			if !addToExisting {
				if cover, rdErr := os.ReadFile(filepath.Join(outDir, items[0].ThumbFilename)); rdErr == nil {
					os.WriteFile(filepath.Join(outDir, "cover.jpg"), cover, 0o644) //nolint:errcheck
				}
			}
			if assetsErr := writeSiteAssets(filepath.Join(siteDir, "assets")); assetsErr != nil {
				emit(map[string]any{"error": "write site assets: " + assetsErr.Error()})
				return
			}
			statePath := filepath.Join(siteDir, "site.json")
			siteAlbums, _ := loadSiteState(statePath)
			sitePhotos := make([]SitePhoto, len(items))
			for i, item := range items {
				sitePhotos[i] = SitePhoto{PhotoID: item.PhotoID, Filename: item.Filename, ThumbFilename: item.ThumbFilename}
			}
			if addToExisting {
				for i := range siteAlbums {
					if siteAlbums[i].PostID == albumPostID {
						siteAlbums[i].Photos = sitePhotos
						siteAlbums[i].PhotoCount = len(items)
						siteAlbums[i].HasZip = zipName != ""
						siteAlbums[i].UpdatedAt = publishedAt
						break
					}
				}
			} else {
				siteAlbums = append(siteAlbums, SiteAlbum{
					PostID: albumPostID, Slug: albumSlug, Title: galleryTitle, PublishedAt: publishedAt,
					PhotoCount: len(items), CoverFile: "cover.jpg", HasZip: zipName != "",
					Photos: sitePhotos, Unlisted: albumUnlisted,
				})
			}
			if saveErr := saveSiteState(statePath, siteAlbums); saveErr != nil {
				emit(map[string]any{"error": "save site state: " + saveErr.Error()})
				return
			}
			rootNav := buildSiteNavContext(ch, siteDir, true)
			siteHTML := GenerateSiteIndex(ch.SiteTitle, ch.SiteTheme, ch.SiteURL, siteAlbums, rootNav)
			if writeErr := os.WriteFile(filepath.Join(siteDir, "index.html"), siteHTML, 0o644); writeErr != nil {
				emit(map[string]any{"error": "write site index: " + writeErr.Error()})
				return
			}
			generateAboutPage(siteDir, ch, avatarExistsAt(siteDir), rootNav) //nolint:errcheck
			generateImprintPage(siteDir, ch, rootNav)                        //nolint:errcheck
			generateRobotsTxt(siteDir, ch.SiteURL)                           //nolint:errcheck
			if ch.SiteURL != "" {
				generateSitemap(siteDir, siteAlbums, ch.SiteURL) //nolint:errcheck
			}
			emit(map[string]any{"step": "site", "done": 1, "total": 1, "file": "Site index updated"})
			clearDraft()
			emit(map[string]any{"complete": true, "postID": postID, "galleryPath": outDir, "sitePath": siteDir, "results": results})
		} else {
			clearDraft()
			emit(map[string]any{"complete": true, "postID": postID, "galleryPath": outDir, "results": results})
		}
	}
}
```

And update `Handle`:

```go
// handler.go — remove this line:
// mux.HandleFunc("POST /api/library/{id}/build", buildPhotos(mgr, chStore, root, serverRole))
// add:
mux.HandleFunc("POST /api/channels/{slug}/drafts/{draftID}/generate", generateDraft(mgr, chStore, draftStore))
```

Since `buildPhotos` no longer exists, `root` and `serverRole` may become unused by other code in this file — check before removing them from `Handle`'s signature (they're very likely still used elsewhere, e.g. by `browseFolder`; don't remove them without checking every remaining call site).

- [ ] **Step 5: Run tests, fix, run again**

Run: `cd src && go build ./... && go test ./internal/api/library/... -v`
Expected: PASS. Fix compile errors from removed/renamed identifiers as they surface — the exact set of "still referenced" helpers (`buildSiteNavContext`, `avatarExistsAt`, etc.) must already exist since they were used by the original `buildPhotos`; this is a pure relocation, not new logic, so failures here mean a copy-paste mismatch against the real source, not a design gap.

- [ ] **Step 6: `go vet`, remove now-dead code, and commit**

Search for any other caller of the deleted `buildPhotos` (frontend `LibraryAPI.build`/`buildStream` in `library.js` — these get rewritten in Task 6, don't touch them here) and confirm nothing else in Go references it.

```bash
cd src && go vet ./...
git add -A src/internal/api/library/
git commit -m "feat: replace immediate build with draft-driven generate endpoint"
```

---

## Task 4: Published overview shows Draft/Generated/Live status

**Files:**
- Modify: `src/internal/api/library/galleries_overview.go` (read it in full first — it was written for the still-uncommitted "published galleries overview" feature per `git status`, so match its existing `PublishedGallery` struct and `listAllGalleries` shape rather than guessing).
- Modify: `src/internal/api/library/galleries_overview_test.go` accordingly.
- Modify: `src/internal/api/library/handler.go` — `listAllGalleries`'s route registration needs a `*channels.DraftStore` parameter added.
- Modify: `src/internal/api/routes.go` — thread `draftStore` into wherever `listAllGalleries` (or its containing `Handle` call) is constructed (same `draftStore` variable added in Task 2).

**Interfaces:**
- Consumes: `channels.DraftStore.List(slug)` (Task 1).
- Produces: extends the existing `PublishedGallery` JSON (read the current struct definition first) with two new fields: `status` (`"draft" | "generated" | "live" | "live-pending"`) and `pendingCount` (int, 0 when not applicable). Also emits **draft-only** rows — one per draft that has no corresponding generated gallery/album yet — with `status: "draft"`, `postID: ""`, `photoCount: <len(draft.Photos)>`, and enough identifying info (channel slug/name, draft ID, target title) for the frontend to open the Publish dialog against that draft.

- [ ] **Step 1: Read the existing file and write the failing test**

Read `src/internal/api/library/galleries_overview.go` and `galleries_overview_test.go` completely — they already exist and work; this task extends them, it does not replace them. Write a new test (following the existing file's exact helper/fixture pattern) asserting:
1. A channel with one generated (live-eligible) gallery and no draft → `status: "generated"` (or `"live"` if the existing code already models deploy success — match whatever status concept, if any, already exists there, since the git-status diff shows this file was recently added and may already have partial status logic).
2. A channel with only a draft (no generated gallery yet) → one row with `status: "draft"`, `pendingCount` equal to the draft's photo count.
3. A channel with a generated gallery plus a draft targeting that same `postID` → one row (not two) with `status` reflecting "has pending changes" and `pendingCount` > 0.

- [ ] **Step 2: Run the test to verify it fails**

Run: `cd src && go test ./internal/api/library/... -run TestListAllGalleries -v`

- [ ] **Step 3: Implement**

Extend `listAllGalleries`'s per-channel loop to also call `draftStore.List(ch.Slug)`, and merge: for each draft with `Target.PostID` matching an already-collected generated row, attach `pendingCount = len(draft.Photos)` and mark status `"live-pending"` (or `"generated-pending"` if the row wasn't deployed — reuse whatever "is this deployed" signal the existing code already computes, don't invent a new one); for each draft with no matching `PostID` (i.e. a brand-new, never-generated gallery), append a synthetic row with `status: "draft"`.

- [ ] **Step 4: Run tests to verify they pass**

Run: `cd src && go test ./internal/api/library/... -v`

- [ ] **Step 5: `go vet` and commit**

```bash
cd src && go vet ./...
git add src/internal/api/library/galleries_overview.go src/internal/api/library/galleries_overview_test.go \
        src/internal/api/library/handler.go src/internal/api/routes.go
git commit -m "feat: surface draft/pending status in the published galleries overview"
```

---

## Task 5: Frontend API helpers (`ChannelAPI` + `LibraryAPI` additions)

**Files:**
- Modify: `src/web/js/channels.js` — the `ChannelAPI` object (`channels.js:5-102`; read it in full to match its existing fetch-wrapper style before adding methods).
- Modify: `src/web/js/library.js` — the `LibraryAPI` object (`library.js:21+`).

**Interfaces:**
- Produces (consumed by Tasks 6, 7, 8, 9):
  - `LibraryAPI.collect(libID, { photoIDs, draftID, postID, title, unlisted, account })` → `POST /api/library/{id}/channels/{slug}/drafts` — wait, slug isn't in this signature; **fix**: `LibraryAPI.collect(libID, slug, {...})`.
  - `ChannelAPI.listDrafts(slug)` → `GET /api/channels/{slug}/drafts`.
  - `ChannelAPI.deleteDraft(slug, draftID)` → `DELETE /api/channels/{slug}/drafts/{draftID}`.
  - `ChannelAPI.removeDraftPhoto(slug, draftID, libID, photoID)` → `DELETE /api/channels/{slug}/drafts/{draftID}/photos/{libID}/{photoID}`.
  - `ChannelAPI.generateStream(slug, draftID, { publishedAt }, onProgress)` → `POST /api/channels/{slug}/drafts/{draftID}/generate`, reusing the exact SSE-parsing loop already implemented in `LibraryAPI.buildStream` (`library.js:172-203`) — for a plain-export channel the response isn't SSE at all (see Task 3), so this helper must first read the response as JSON if `Content-Type` isn't `text/event-stream`, and only fall into the streaming reader otherwise.

- [ ] **Step 1: Add the frontend API methods** (no separate test step — these are thin fetch wrappers with no branching logic beyond the content-type check below; they're exercised end-to-end by the e2e spec in Task 10)

```js
// library.js — add near the existing build/buildStream/buildDownload methods (library.js:163-212)
async collect(libID, slug, { photoIDs, draftID, postID, title, unlisted, account }) {
    const r = await fetch(`/api/library/${libID}/channels/${slug}/drafts`, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ photoIDs, draftID, postID, title, unlisted, account }),
    });
    if (!r.ok) throw new Error(await r.text());
    return r.json();
},
```

```js
// channels.js — add to ChannelAPI (channels.js:5-102), following its existing style
async listDrafts(slug) {
    const r = await fetch(`/api/channels/${slug}/drafts`);
    if (!r.ok) throw new Error(await r.text());
    return r.json();
},
async deleteDraft(slug, draftID) {
    const r = await fetch(`/api/channels/${slug}/drafts/${draftID}`, { method: 'DELETE' });
    if (!r.ok) throw new Error(await r.text());
},
async removeDraftPhoto(slug, draftID, libID, photoID) {
    const r = await fetch(`/api/channels/${slug}/drafts/${draftID}/photos/${libID}/${photoID}`, { method: 'DELETE' });
    if (!r.ok) throw new Error(await r.text());
    return r.status === 204 ? null : r.json();
},
async generateStream(slug, draftID, { publishedAt } = {}, onProgress) {
    const r = await fetch(`/api/channels/${slug}/drafts/${draftID}/generate`, {
        method: 'POST',
        headers: { 'Content-Type': 'application/json' },
        body: JSON.stringify({ publishedAt }),
    });
    if (!r.ok) throw new Error(await r.text());
    if (!r.headers.get('content-type')?.includes('text/event-stream')) {
        return r.json();
    }
    const reader = r.body.getReader();
    const decoder = new TextDecoder();
    let buffer = '';
    let finalEvt = null;
    while (true) {
        const { done, value } = await reader.read();
        if (done) break;
        buffer += decoder.decode(value, { stream: true });
        const blocks = buffer.split('\n\n');
        buffer = blocks.pop() ?? '';
        for (const block of blocks) {
            const line = block.split('\n').find(l => l.startsWith('data: '));
            if (!line) continue;
            try {
                const evt = JSON.parse(line.slice(6));
                if (evt.complete) finalEvt = evt;
                else if (onProgress) onProgress(evt);
            } catch { /* skip malformed */ }
        }
    }
    if (!finalEvt) throw new Error('Generate stream ended without completion event');
    return finalEvt;
},
```

- [ ] **Step 2: Commit**

```bash
git add src/web/js/library.js src/web/js/channels.js
git commit -m "feat: add frontend API helpers for channel drafts and generate"
```

---

## Task 6: Replace "Build…" with "Add to channel…" in `library.js`

**Files:**
- Modify: `src/web/js/library.js`:
  - `_renderList` header (`library.js:574-577`) and `_renderDetail` header (`library.js:1061-1063`): replace the `Build…`/`Deploy` button pair with a single `Add to channel…` button; keep `Channels ›` as-is.
  - Delete `_lastBuiltChannel` (`library.js:412`), `_deployEligible` (`library.js:423`), `_registerBuiltChannel` (`library.js:429`), `_makeDeployHandler` and its two call sites (`library.js:440-460`, `596-597`, `1079-1080`), and the `.lib-deploy-output`/`.lib-deploy-caveat` DOM elements referenced there.
  - Replace `_openBuildModal` (`library.js:1233-1680`) with a new `_openCollectModal`, called from the new button's click handler in place of the old `this._openBuildModal()` wiring at `library.js:631` and `library.js:1095`.

**Interfaces:**
- Consumes: `LibraryAPI.collect` (Task 5), `ChannelAPI.list`/`ChannelAPI.galleries`/`ChannelAPI.listDrafts` (existing + Task 5), `App.showToast` (existing, used throughout `library.js`).

**Design note:** `_openCollectModal` keeps the existing photo-selection resolution logic from `_openBuildModal` verbatim (`library.js:1234-1267`, the `searchPane`/`photoGroups`/`selectedPaths`/`hasDirs` block, and the `photoIDByPath` resolution at `library.js:1552-1563`) — that part is unrelated to build-vs-collect and already correctly handles cross-library selections. What changes is everything from channel/target selection onward: no Date/Output/XMP-toggle fields (those move into the Publish dialog's Generate step in Task 9 or are dropped per Task 3's scope decisions), and the confirm action calls `LibraryAPI.collect` once per library group instead of `LibraryAPI.build`/`buildStream`.

- [ ] **Step 1: Write the new button markup**

```html
<!-- library.js:574-577, replacing the Build…/Deploy pair -->
<button class="btn lib-collect-btn" id="lib-list-collect-btn" disabled>Add to channel…</button>
<span class="btn-sep"></span>
<button class="btn" id="lib-channels-btn">Channels ›</button>
```

```html
<!-- library.js:1061-1063, same replacement for the detail view -->
<button class="btn btn-sm lib-collect-btn" id="lib-collect-btn" disabled>Add to channel…</button>
<button class="btn btn-sm" id="lib-channels-btn" title="Manage channels">Channels ›</button>
```

Whatever existing logic enables/disables the old `#lib-list-build-btn`/`#lib-build-btn` based on selection (search for its enable/disable call sites elsewhere in `library.js` — it wasn't in the code read during planning, so locate it before editing) must be re-pointed at the new button IDs (`#lib-list-collect-btn`/`#lib-collect-btn`).

- [ ] **Step 2: Implement `_openCollectModal`**

```js
async _openCollectModal() {
    const searchPane = (() => {
        if (this._searchPane && this._detailEl?.querySelector('#lib-search-pane')?.style.display !== 'none'
            && this._searchPane.selection.selected.size > 0) return this._searchPane;
        if (this._listSearchPanel?._searchPane?.selection.selected.size > 0)
            return this._listSearchPanel._searchPane;
        return null;
    })();

    let lib = this.currentLibrary;
    let selectedPaths, hasDirs, selectedDirs = [];
    let photoGroups = null;

    if (searchPane) {
        const hints = searchPane.getSelectedFiles();
        const byLib = new Map();
        for (const h of hints) {
            const info = searchPane.getPhotoInfo(h);
            if (!info) continue;
            if (!byLib.has(info.libID)) byLib.set(info.libID, []);
            byLib.get(info.libID).push(info.photoID);
        }
        photoGroups = Array.from(byLib.entries()).map(([libID, photoIDs]) => ({ libID, photoIDs }));
        selectedPaths = hints;
        hasDirs = false;
    } else {
        const pane = this._pane;
        if (!pane) return;
        const selectedFiles = pane.getSelectedFiles();
        selectedDirs = Array.from(pane.selectedDirs || []);
        hasDirs = selectedDirs.length > 0;
        if (selectedFiles.length === 0 && !hasDirs) return;
        selectedPaths = hasDirs ? [] : selectedFiles;
    }

    let channelList;
    try {
        channelList = await ChannelAPI.list();
    } catch (err) {
        alert('Failed to load channels: ' + err.message);
        return;
    }
    if (channelList.length === 0) {
        alert('No channels configured. Use the Channels button to add one.');
        return;
    }

    const initialTitle = hasDirs ? 'Counting photos…'
        : `Add ${selectedPaths.length} photo${selectedPaths.length !== 1 ? 's' : ''} to a channel`;

    const dlg = document.createElement('div');
    dlg.className = 'modal-backdrop';
    dlg.innerHTML = `
        <div class="modal collect-modal">
            <div class="modal-header">
                <span class="modal-title">${initialTitle}</span>
                <button class="modal-close" id="collect-close">&times;</button>
            </div>
            <div class="modal-body">
                <label class="form-label">Channel</label>
                <select class="form-select" id="collect-channel">
                    ${channelList.map(c => `<option value="${escapeHtml(c.slug)}">${escapeHtml(c.name)}</option>`).join('')}
                </select>
                <div id="collect-account-wrap" style="display:none">
                    <label class="form-label">Account</label>
                    <select class="form-select" id="collect-account"></select>
                </div>
                <div id="collect-gallery-wrap" style="display:none">
                    <label class="form-label">Add to</label>
                    <select class="form-select" id="collect-target">
                        <option value="">New…</option>
                    </select>
                    <div id="collect-title-wrap">
                        <label class="form-label" id="collect-gallery-label">Gallery title</label>
                        <input class="form-input" id="collect-gallery-title" placeholder="e.g. Summer 2026" autocomplete="off">
                        <div id="collect-unlisted-wrap" style="display:none">
                            <label class="export-radio-row">
                                <input type="checkbox" id="collect-unlisted">
                                Unlisted (not listed on the site index or sitemap — reachable only via direct link)
                            </label>
                        </div>
                    </div>
                </div>
                <div class="build-info" id="collect-info"></div>
            </div>
            <div class="modal-footer">
                <div class="build-error" id="collect-error" style="display:none"></div>
                <button class="btn" id="collect-cancel">Cancel</button>
                <button class="btn btn-accent" id="collect-confirm"${hasDirs ? ' disabled' : ''}>Add to channel</button>
            </div>
        </div>`;
    document.body.appendChild(dlg);

    if (hasDirs) {
        Promise.all(selectedDirs.map(d => this._pane.fetchRecursivePhotoPaths(d)))
            .then(arrays => {
                selectedPaths = arrays.flat();
                if (dlg.isConnected) {
                    dlg.querySelector('.modal-title').textContent =
                        `Add ${selectedPaths.length} photo${selectedPaths.length !== 1 ? 's' : ''} to a channel`;
                    dlg.querySelector('#collect-confirm').disabled = selectedPaths.length === 0;
                }
            });
    }

    // draftsBySlug caches ChannelAPI.listDrafts() results per channel so the
    // "Add to" dropdown can offer in-progress drafts alongside already-generated
    // galleries without refetching on every channel switch.
    const draftsBySlug = new Map();

    const updateChannel = async () => {
        const slug = dlg.querySelector('#collect-channel').value;
        const ch = channelList.find(c => c.slug === slug);
        if (!ch) return;

        const accountWrap = dlg.querySelector('#collect-account-wrap');
        const accountSel = dlg.querySelector('#collect-account');
        const accounts = ch.accounts || [];
        if (accounts.length > 0) {
            accountSel.innerHTML = accounts.map(a => `<option value="${escapeHtml(a.id)}">${escapeHtml(a.label || a.id)}</option>`).join('');
            accountWrap.style.display = '';
        } else {
            accountWrap.style.display = 'none';
        }

        const galleryWrap = dlg.querySelector('#collect-gallery-wrap');
        const targetSel = dlg.querySelector('#collect-target');
        const titleWrap = dlg.querySelector('#collect-title-wrap');
        const titleLabel = dlg.querySelector('#collect-gallery-label');
        titleLabel.textContent = ch.siteExport ? 'Album title' : 'Gallery title';

        if (ch.galleryExport || ch.siteExport) {
            galleryWrap.style.display = '';
            let generated = [];
            let drafts = draftsBySlug.get(slug);
            try {
                generated = await ChannelAPI.galleries(slug);
                if (!drafts) {
                    drafts = await ChannelAPI.listDrafts(slug);
                    draftsBySlug.set(slug, drafts);
                }
            } catch { /* fall back to New-only */ }
            const generatedOpts = generated.map(g =>
                `<option value="postID:${escapeHtml(g.postID)}">${escapeHtml(g.title)} (${g.photoCount}) · ${_galleryDateRange(g)}</option>`
            );
            const draftOpts = drafts.filter(d => !d.target.postID).map(d =>
                `<option value="draftID:${escapeHtml(d.id)}">${escapeHtml(d.target.title)} (draft, ${d.photos.length} pending)</option>`
            );
            targetSel.innerHTML = `<option value="">New ${ch.siteExport ? 'album' : 'gallery'}</option>` + generatedOpts.join('') + draftOpts.join('');
            targetSel.onchange = () => {
                titleWrap.style.display = targetSel.value === '' ? '' : 'none';
            };
            titleWrap.style.display = targetSel.value === '' ? '' : 'none';
        } else {
            galleryWrap.style.display = 'none';
        }

        const unlistedWrap = dlg.querySelector('#collect-unlisted-wrap');
        unlistedWrap.style.display = ch.siteExport ? '' : 'none';
        dlg.querySelector('#collect-unlisted').checked = false;

        const scaleDesc = _scaleDesc(ch.scale);
        const handlerNote = ch.handler ? ` · handler: ${ch.handler}` : '';
        dlg.querySelector('#collect-info').textContent =
            `Export: ${ch.format.toUpperCase()} · quality ${ch.quality}${scaleDesc ? ' · ' + scaleDesc : ''}${handlerNote}`;
    };

    dlg.querySelector('#collect-channel').addEventListener('change', updateChannel);
    updateChannel();

    dlg.querySelector('#collect-close').addEventListener('click', () => dlg.remove());
    dlg.querySelector('#collect-cancel').addEventListener('click', () => dlg.remove());
    dlg.addEventListener('click', e => { if (e.target === dlg) dlg.remove(); });

    dlg.querySelector('#collect-confirm').addEventListener('click', async () => {
        const confirmBtn = dlg.querySelector('#collect-confirm');
        const errEl = dlg.querySelector('#collect-error');
        const slug = dlg.querySelector('#collect-channel').value;
        const ch = channelList.find(c => c.slug === slug);
        const targetVal = dlg.querySelector('#collect-target')?.value || '';
        const [targetKind, targetID] = targetVal.includes(':') ? targetVal.split(':') : [null, null];
        const titleWrap = dlg.querySelector('#collect-title-wrap');
        const galleryTitle = (titleWrap && titleWrap.style.display !== 'none')
            ? dlg.querySelector('#collect-gallery-title').value.trim()
            : undefined;
        const isGalleryMode = ch?.galleryExport || ch?.siteExport;
        if (isGalleryMode && !targetKind && !galleryTitle) {
            errEl.textContent = ch.siteExport ? 'Album title is required for a new album.' : 'Gallery title is required for a new gallery.';
            errEl.style.display = '';
            return;
        }
        const accountWrap = dlg.querySelector('#collect-account-wrap');
        const account = accountWrap.style.display !== 'none' ? (dlg.querySelector('#collect-account').value || undefined) : undefined;
        const unlisted = (ch?.siteExport && !targetKind) ? dlg.querySelector('#collect-unlisted').checked : undefined;

        confirmBtn.disabled = true;
        confirmBtn.textContent = 'Adding…';
        errEl.style.display = 'none';

        try {
            let validGroups;
            if (photoGroups) {
                validGroups = photoGroups.filter(g => g.photoIDs.length > 0);
                if (validGroups.length === 0) throw new Error('No matching library photos found.');
            } else {
                const photoIDs = await Promise.all(selectedPaths.map(p => LibraryAPI.photoIDByPath(lib.id, p)));
                validGroups = [{ libID: lib.id, photoIDs: photoIDs.filter(Boolean) }];
                if (validGroups[0].photoIDs.length === 0) throw new Error('No matching library photos found for selection.');
            }

            let total = 0;
            for (const g of validGroups) {
                await LibraryAPI.collect(g.libID, slug, {
                    photoIDs: g.photoIDs,
                    draftID: targetKind === 'draftID' ? targetID : undefined,
                    postID: targetKind === 'postID' ? targetID : undefined,
                    title: !targetKind ? galleryTitle : undefined,
                    unlisted, account,
                });
                total += g.photoIDs.length;
            }
            dlg.remove();
            App.showToast(`Added ${total} photo${total !== 1 ? 's' : ''} to "${galleryTitle || ch.name}" — not yet published.`);
        } catch (err) {
            errEl.textContent = err.message;
            errEl.style.display = '';
            confirmBtn.disabled = false;
            confirmBtn.textContent = 'Add to channel';
        }
    });
}
```

Rewire the click handlers previously pointing at `_openBuildModal` (`library.js:631`, `library.js:1095`) to `_openCollectModal`, and the two `#lib-channels-btn` click handlers (`library.js:594`, `library.js:1091`) are unaffected.

- [ ] **Step 3: Manual verification (no automated frontend test framework in this repo beyond e2e)**

Run the `unterlumen-dev` skill, select photos in the Libraries view, click "Add to channel…", add to a new gallery, confirm the toast and that nothing was written to the channel's output folder (`ls` the channel's output dir before/after — it must be unchanged).

- [ ] **Step 4: Commit**

```bash
git add src/web/js/library.js
git commit -m "feat: replace Build with a lightweight Add-to-channel collect dialog"
```

---

## Task 7: Slim down the Channels dialog

**Files:**
- Modify: `src/web/js/channels.js`:
  - `_row` (`channels.js:190-267`): remove the `ch-rebuild`, `ch-rebuild-galleries`, `ch-albums`, `ch-published`, and `ch-deploy` buttons and their event-listener wiring; keep `Visit site` link, `Path ▾` menu, `Edit`, `Delete`.
  - Delete `_rebuildSite` (`channels.js:279-292`), `_rebuildGalleries` (`channels.js:298-314`), `_deployChannel` (`channels.js:316-344`), `_deployResultLinkHTML` (`channels.js:352-374`), `_renderPersistedDeployStatus` (`channels.js:380-394`), `_showAlbums` (`channels.js:396+`, read to find its full extent before deleting).
  - Add a new async `_statusLine(ch)` that fetches `ChannelAPI.galleries(ch.slug)` + `ChannelAPI.listDrafts(ch.slug)` and renders e.g. `"3 galleries · 1 with pending changes"`, as a clickable element that calls `App.showPublishedForChannel(ch.slug)` (existing bridge, `channels.js:231-234`, `app.js:321`) — same behavior the removed `ch-published` button had, just always visible instead of conditional.
- Modify: `src/internal/api/channels/handler.go` — the `rebuild-site`/`rebuild-galleries` HTTP endpoints (`internal/api/library/handler.go:2216`, `2423` — note these already live in `apilibrary`, not `apichannels`, per the routing convention discovered during planning) lose their only caller; per Task 3's note on `rebuildSite`/`rebuildGalleries`, leave the Go functions and routes in place for now (Generate already subsumes their purpose for the redesigned flow, but removing working, tested backend code that a future maintenance script might still want is out of scope for a UI redesign plan — flag this as a follow-up cleanup in the feature doc instead of doing it here).

**Interfaces:**
- Consumes: `ChannelAPI.galleries`, `ChannelAPI.listDrafts` (Task 5), `App.showPublishedForChannel` (existing).

- [ ] **Step 1: Simplify `_row`**

```js
// channels.js:190-267, replacing the actions block and adding a status line
_row(ch) {
    const row = document.createElement('div');
    row.className = 'channel-row';
    const scaleDesc = _scaleDesc(ch.scale);
    const accountCount = (ch.accounts || []).length;
    const handlerDesc = ch.handler ? ` · handler: ${escapeHtml(ch.handler)}` : '';
    const accountDesc = accountCount > 0 ? ` · ${accountCount} account${accountCount !== 1 ? 's' : ''}` : '';
    const outputDesc = ch.outputMode === 'download'
        ? ' · → download ZIP'
        : (ch.outputPath ? ` · → ${escapeHtml(ch.outputPath.split('/').pop() || ch.outputPath)}` : '');
    const base = _deployBaseURL(ch);
    row.innerHTML = `
        <div class="channel-row-top">
            <div class="channel-row-header">
                <span class="channel-row-name">${escapeHtml(ch.name)}</span>
                <span class="channel-row-slug">${escapeHtml(ch.slug)}</span>
            </div>
            <div class="channel-row-actions">
                ${base ? `<a class="btn btn-sm ch-visit-site" href="${escapeHtml(base.url)}" target="_blank" rel="noopener">Visit site</a>` : ''}
                <div class="ch-path-wrap">
                    <button class="btn btn-sm ch-path-toggle">Path ▾</button>
                    <div class="ch-path-menu" hidden>
                        <button class="ch-path-item ch-copy-path">Copy path</button>
                        <button class="ch-path-item ch-reveal">Show in Files</button>
                        <button class="ch-path-item ch-commander">Open in Commander</button>
                    </div>
                </div>
                <button class="btn btn-sm ch-edit">Edit</button>
                <button class="btn btn-sm ch-delete">Delete</button>
            </div>
        </div>
        <span class="channel-row-detail">${escapeHtml(ch.format.toUpperCase())} · q${ch.quality} · ${escapeHtml(scaleDesc)} · ${escapeHtml(ch.exifMode)}${escapeHtml(handlerDesc)}${escapeHtml(accountDesc)}${escapeHtml(outputDesc)}</span>
        ${(ch.siteExport || ch.galleryExport) ? '<button class="link-btn ch-status-line">Loading status…</button>' : ''}`;

    row.querySelector('.ch-edit').addEventListener('click', () => this._openForm(ch));
    row.querySelector('.ch-delete').addEventListener('click', () => this._deleteChannel(ch, row));

    const toggle = row.querySelector('.ch-path-toggle');
    const menu = row.querySelector('.ch-path-menu');
    toggle.addEventListener('click', e => {
        e.stopPropagation();
        const open = !menu.hidden;
        document.querySelectorAll('.ch-path-menu').forEach(m => { m.hidden = true; });
        menu.hidden = open;
    });
    row.querySelector('.ch-copy-path').addEventListener('click', () => { menu.hidden = true; this._copyPath(ch, toggle); });
    row.querySelector('.ch-reveal').addEventListener('click', () => { menu.hidden = true; this._revealChannel(ch); });
    row.querySelector('.ch-commander').addEventListener('click', () => { menu.hidden = true; this._openInCommander(ch); });

    const statusBtn = row.querySelector('.ch-status-line');
    if (statusBtn) {
        statusBtn.addEventListener('click', () => { this.close(); App.showPublishedForChannel(ch.slug); });
        this._loadStatusLine(ch, statusBtn);
    }
    return row;
}

async _loadStatusLine(ch, statusBtn) {
    try {
        const [galleries, drafts] = await Promise.all([ChannelAPI.galleries(ch.slug), ChannelAPI.listDrafts(ch.slug)]);
        const pending = drafts.length;
        const galleryWord = ch.siteExport ? 'album' : 'gallery';
        let text = `${galleries.length} ${galleryWord}${galleries.length !== 1 ? 's' : ''}`;
        if (pending > 0) text += ` · ${pending} with pending changes`;
        if (statusBtn.isConnected) statusBtn.textContent = text;
    } catch {
        if (statusBtn.isConnected) statusBtn.textContent = 'Status unavailable';
    }
}
```

Remove the now-orphaned `_rebuildSite`, `_rebuildGalleries`, `_deployChannel`, `_deployResultLinkHTML`, `_renderPersistedDeployStatus`, `_showAlbums` methods entirely (their logic is superseded by Task 9's publish dialog, which reimplements deploy directly against `ChannelAPI.deploy`).

- [ ] **Step 2: Manual verification**

Open Channels dialog in the running dev app; confirm each row shows Visit site (if applicable) / Path / Edit / Delete and a clickable status line, and that clicking it opens the Published tab filtered to that channel.

- [ ] **Step 3: Commit**

```bash
git add src/web/js/channels.js
git commit -m "feat: slim Channels dialog down to settings + status"
```

---

## Task 8: Publish dialog (new component) + Published tab status column

**Files:**
- Create: `src/web/js/publish-dialog.js` (new file — a 4-step dialog is enough independent state-machine logic to deserve its own file per the single-responsibility coding standard, rather than growing `published-galleries.js` or `channels.js`).
- Modify: `src/web/index.html` — add `<script src="js/publish-dialog.js?v=1"></script>` near the existing `channels.js`/`published-galleries.js` script tags (read the exact surrounding lines first to match the versioned-query-string convention already in use, e.g. `channels.js` is loaded as `?v=10` per the earlier exploration — bump to whatever's next, don't guess a version number without checking the current one).
- Modify: `src/web/js/published-galleries.js` — add a `Status` column (read the existing table-rendering code, likely around the `.pub-gal-table` construction, before editing) and a `Publish` action button per row that calls `new PublishDialog().open(row)`.

**Interfaces:**
- Consumes: `ChannelAPI.listDrafts`/`generateStream`/`deploy` (existing + Task 5), the extended `GET /api/channels/galleries` response from Task 4 (`status`, `pendingCount`, and draft identification fields).
- Produces: `class PublishDialog { open(galleryOrDraftRow) }` — used by `published-galleries.js`.

- [ ] **Step 1: Implement `PublishDialog`**

```js
// src/web/js/publish-dialog.js
// PublishDialog walks one gallery/album/draft through the collect→publish
// lifecycle's second half: review pending photos, Generate (export + build
// HTML), review the generated artifact, and Deploy if the channel has a
// handler. Reused for both a channel's first publish and any later republish.
class PublishDialog {
    constructor() {
        this._el = null;
    }

    // row: one item from GET /api/channels/galleries (Task 4's extended shape) —
    // { channelSlug, channelName, handler, status, postID, draftID, title, pendingCount, url, urlGuessed }
    async open(row) {
        this._row = row;
        this._el = document.createElement('div');
        this._el.className = 'modal-backdrop';
        this._el.innerHTML = `
            <div class="modal publish-dialog">
                <div class="modal-header">
                    <span class="modal-title">Publish — ${escapeHtml(row.title || row.channelName)}</span>
                    <button class="modal-close" id="pub-close">&times;</button>
                </div>
                <div class="modal-body" id="pub-body"></div>
            </div>`;
        document.body.appendChild(this._el);
        this._el.querySelector('#pub-close').addEventListener('click', () => this.close());
        this._el.addEventListener('click', e => { if (e.target === this._el) this.close(); });
        await this._renderReviewStep();
    }

    close() {
        this._el?.remove();
        this._el = null;
    }

    async _renderReviewStep() {
        const body = this._el.querySelector('#pub-body');
        body.innerHTML = '<div class="channel-loading">Loading pending photos…</div>';
        let drafts = [];
        try {
            drafts = await ChannelAPI.listDrafts(this._row.channelSlug);
        } catch (err) {
            body.innerHTML = `<div class="channel-error">Failed to load: ${escapeHtml(err.message)}</div>`;
            return;
        }
        const draft = drafts.find(d => d.id === this._row.draftID) || null;
        this._draft = draft;

        const photoRows = (draft?.photos || []).map(p => `
            <div class="publish-step-photo" data-lib="${escapeHtml(p.libraryID)}" data-photo="${escapeHtml(p.photoID)}">
                <img class="publish-step-thumb" src="/api/library/${encodeURIComponent(p.libraryID)}/thumb/${encodeURIComponent(p.photoID)}" alt="">
                <button class="publish-step-remove" title="Remove from this publish">×</button>
            </div>`).join('');

        body.innerHTML = `
            <p class="form-hint">${draft ? draft.photos.length : 0} photo${(draft?.photos.length ?? 0) !== 1 ? 's' : ''} pending for "${escapeHtml(this._row.title || this._row.channelName)}".</p>
            <div class="publish-step-photos">${photoRows || '<span class="channel-empty">Nothing pending — Generate will just refresh the current gallery.</span>'}</div>
            <div class="modal-footer">
                <button class="btn" id="pub-cancel">Cancel</button>
                <button class="btn btn-accent" id="pub-generate">Generate</button>
            </div>`;

        body.querySelectorAll('.publish-step-remove').forEach(btn => {
            btn.addEventListener('click', async () => {
                const card = btn.closest('.publish-step-photo');
                try {
                    await ChannelAPI.removeDraftPhoto(this._row.channelSlug, draft.id, card.dataset.lib, card.dataset.photo);
                    await this._renderReviewStep();
                } catch (err) {
                    alert('Remove failed: ' + err.message);
                }
            });
        });
        body.querySelector('#pub-cancel').addEventListener('click', () => this.close());
        body.querySelector('#pub-generate').addEventListener('click', () => this._runGenerate());
    }

    async _runGenerate() {
        const body = this._el.querySelector('#pub-body');
        body.innerHTML = '<div class="channel-loading" id="pub-progress">Generating…</div>';
        const progressEl = body.querySelector('#pub-progress');
        try {
            const result = await ChannelAPI.generateStream(
                this._row.channelSlug, this._row.draftID,
                {},
                (evt) => {
                    if (evt.step === 'photo') progressEl.textContent = `Exporting photo ${evt.done} of ${evt.total}…`;
                    else if (evt.file) progressEl.textContent = evt.file;
                }
            );
            this._result = result;
            await this._renderArtifactStep();
        } catch (err) {
            body.innerHTML = `<div class="channel-error">Generate failed: ${escapeHtml(err.message)}</div>
                <div class="modal-footer"><button class="btn" id="pub-cancel">Close</button></div>`;
            body.querySelector('#pub-cancel').addEventListener('click', () => this.close());
        }
    }

    async _renderArtifactStep() {
        const body = this._el.querySelector('#pub-body');
        const isSite = !!this._result.sitePath;
        const localPath = this._result.sitePath || this._result.galleryPath;
        const showsHTML = isSite || !!this._result.galleryPath; // gallery/site export channels only
        const canDeploy = this._row.handler === 'rsync';

        body.innerHTML = `
            <p class="form-hint">Generated. Nothing has been published live yet.</p>
            <div class="publish-step-review">
                <div class="build-destination">${escapeHtml(localPath)}</div>
                <div class="publish-step-review-actions">
                    <button class="btn btn-sm" id="pub-copy-path">Copy path</button>
                    <button class="btn btn-sm" id="pub-open-folder">Open in Finder/Explorer</button>
                    ${showsHTML ? '<button class="btn btn-sm" id="pub-open-browser">Open in browser</button>' : ''}
                </div>
            </div>
            <div class="modal-footer">
                <button class="btn" id="pub-close-2">${canDeploy ? 'Not now' : 'Done'}</button>
                ${canDeploy ? '<button class="btn btn-accent" id="pub-deploy">Deploy</button>' : ''}
            </div>`;

        body.querySelector('#pub-copy-path').addEventListener('click', () => navigator.clipboard.writeText(localPath));
        body.querySelector('#pub-open-folder').addEventListener('click', () => ChannelAPI.reveal(this._row.channelSlug));
        body.querySelector('#pub-open-browser')?.addEventListener('click', () => {
            window.open('file://' + localPath + '/index.html', '_blank');
        });
        body.querySelector('#pub-close-2').addEventListener('click', () => this.close());
        body.querySelector('#pub-deploy')?.addEventListener('click', () => this._runDeploy());
    }

    async _runDeploy() {
        const body = this._el.querySelector('#pub-body');
        const deployBtn = body.querySelector('#pub-deploy');
        deployBtn.disabled = true;
        deployBtn.textContent = 'Deploying…';
        try {
            const res = await ChannelAPI.deploy(this._row.channelSlug);
            if (res.ok) {
                App.showToast('Deployed.');
                this.close();
            } else {
                deployBtn.disabled = false;
                deployBtn.textContent = 'Deploy';
                alert('Deploy failed: ' + (res.error || 'unknown error'));
            }
        } catch (err) {
            deployBtn.disabled = false;
            deployBtn.textContent = 'Deploy';
            alert('Deploy failed: ' + err.message);
        }
    }
}
```

Check `ChannelAPI.reveal`'s real signature (`channels.js` — it existed pre-redesign per the earlier exploration, `ChannelAPI.reveal`) before wiring `#pub-open-folder` — it may need the channel's output directory rather than just the slug, or may already default to "reveal the channel's folder"; match its existing contract rather than the guess above.

- [ ] **Step 2: Wire `Publish` into `published-galleries.js`**

Read the existing row-rendering code in `published-galleries.js` (its `Actions` column already renders `Edit`/`Delete`) before editing. Add a `Status` `<td>` per row using the `status`/`pendingCount` fields from Task 4, and a `Publish` button in the Actions cell:

```js
// wherever the row's actions cell is built in published-galleries.js
`<button class="btn btn-sm pub-gal-publish">Publish</button>` // add alongside existing Edit/Delete buttons
```

```js
row.querySelector('.pub-gal-publish').addEventListener('click', () => new PublishDialog().open(g));
```

Status cell rendering (add a helper near the existing status-badge CSS classes `.pub-gal-status--pending/--ok/--down`):

```js
function _statusLabel(g) {
    switch (g.status) {
        case 'draft': return `Draft · ${g.pendingCount} photo${g.pendingCount !== 1 ? 's' : ''}`;
        case 'generated': return 'Generated';
        case 'live-pending': return `Live · ${g.pendingCount} pending`;
        case 'live': return 'Live';
        default: return g.status;
    }
}
```

Draft-only rows' Delete action must call `ChannelAPI.deleteDraft(g.channelSlug, g.draftID)` instead of the existing `PublishedGalleryAPI.remove` (which assumes a generated gallery exists on disk) — branch on `g.status === 'draft'` in the existing delete handler.

- [ ] **Step 3: Manual verification**

Full loop in the running dev app: Add to channel → open Published tab, see the Draft badge → click Publish → remove one pending photo → Generate → see the artifact-review step → (for a site channel) Deploy → confirm status flips to Live and the Info Panel shows "published" (this last check depends on Task 9).

- [ ] **Step 4: Commit**

```bash
git add src/web/js/publish-dialog.js src/web/js/published-galleries.js src/web/index.html
git commit -m "feat: add Publish dialog and status column to the Published tab"
```

---

## Task 9: Info Panel shows pending + published state

**Files:**
- Modify: `src/web/js/infopanel.js`:
  - `_renderPublicationsSection` (`infopanel.js:381-408`) — also render `pending:<slug>` entries.
  - `_renderMetaSection`'s `genericEntries` filter (`infopanel.js:426-428`) — exclude `pending:` keys too, so they don't double-render as raw meta rows.
- Modify: `src/internal/api/library/handler.go` — `deleteMeta` (`handler.go:1283-1359`) — add a `pending:` case alongside the existing `built:` case, calling `draftStore.RemovePhoto` instead of the site-removal surgery the `built:` case does.
- Modify: `deleteMeta`'s registration (`handler.go:72`) needs a `*channels.DraftStore` parameter threaded through, same as Tasks 2/3/4.
- Test: extend whichever `_test.go` already covers `deleteMeta` (check `handler_test.go`).

- [ ] **Step 1: Backend — write the failing test**

```go
func TestDeleteMeta_PendingKey_RemovesFromDraft(t *testing.T) {
	// Arrange: collect one photo into a draft (via collectDraft or draftStore.Create
	// directly), which also writes pending:website meta on that photo per Task 2.
	// Act: DELETE /api/library/{id}/photo/{photoID}/meta?key=pending:website
	// Assert: 204, the draft's photo list no longer contains this photo (or the
	// draft is gone if it was the last photo), and the pending:website meta key
	// is gone from store.GetMeta(photoID).
}
```

- [ ] **Step 2: Run to verify it fails**

Run: `cd src && go test ./internal/api/library/... -run TestDeleteMeta_PendingKey -v`

- [ ] **Step 3: Implement**

```go
// handler.go:1283 — add draftStore param
func deleteMeta(mgr *lib.Manager, chStore *channels.Store, draftStore *channels.DraftStore) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		id := r.PathValue("id")
		photoID := r.PathValue("photoID")
		key := r.URL.Query().Get("key")
		if key == "" {
			http.Error(w, "key query param required", http.StatusBadRequest)
			return
		}

		store, err := mgr.OpenStore(id)
		if err != nil {
			http.Error(w, "library not found", http.StatusNotFound)
			return
		}
		defer store.Close()

		const pendingPrefix = "pending:"
		if strings.HasPrefix(key, pendingPrefix) && draftStore != nil {
			slug := strings.TrimPrefix(key, pendingPrefix)
			entries, metaErr := store.GetMeta(photoID)
			if metaErr == nil {
				for _, e := range entries {
					if e.Key == key {
						draftStore.RemovePhoto(slug, e.Value, id, photoID) //nolint:errcheck // e.Value is the draftID
						break
					}
				}
			}
			store.DeleteMeta(photoID, key) //nolint:errcheck
			w.WriteHeader(http.StatusNoContent)
			return
		}

		// ... existing built:/legacy published: handling unchanged (handler.go:1300-1346) ...
		// ... existing title/default handling unchanged (handler.go:1348-1357) ...
	}
}
```

Update the call site (`handler.go:72`): `mux.HandleFunc("DELETE /api/library/{id}/photo/{photoID}/meta", deleteMeta(mgr, chStore, draftStore))`.

- [ ] **Step 4: Run tests, `go vet`**

Run: `cd src && go build ./... && go test ./internal/api/library/... -v && go vet ./...`

- [ ] **Step 5: Frontend — extend the Publications section**

```js
// infopanel.js:381-408, replacing _renderPublicationsSection
_renderPublicationsSection() {
    const ctx = this._metaContext;
    if (!ctx || !ctx.entries) return '';

    const publishedPubs = ctx.entries.filter(e =>
        e.key.startsWith('built:') && !e.key.slice('built:'.length).includes(':')
    );
    const pendingPubs = ctx.entries.filter(e => e.key.startsWith('pending:'));
    if (publishedPubs.length === 0 && pendingPubs.length === 0) return '';

    const publishedCards = publishedPubs.map(e => {
        const slug = e.key.slice('built:'.length);
        const channelName = this._humanizeChannelSlug(slug);
        const date = this.formatDate(e.value);
        const titleEntry = ctx.entries.find(te => te.key === `built:${slug}:title`);
        const galleryTitle = titleEntry ? escapeHtml(titleEntry.value) : '';
        return `<div class="info-pub-card">` +
            `<div class="info-pub-card-header">` +
                `<span class="info-pub-channel">${escapeHtml(channelName)}</span>` +
                `<button class="info-meta-del" title="Remove publication" data-key="${escapeHtml(e.key)}">×</button>` +
            `</div>` +
            `<div class="info-pub-date">published ${escapeHtml(date)}</div>` +
            (galleryTitle ? `<div class="info-pub-title">${galleryTitle}</div>` : '') +
        `</div>`;
    });

    const pendingCards = pendingPubs.map(e => {
        const slug = e.key.slice('pending:'.length);
        const channelName = this._humanizeChannelSlug(slug);
        return `<div class="info-pub-card info-pub-card--pending">` +
            `<div class="info-pub-card-header">` +
                `<span class="info-pub-channel">${escapeHtml(channelName)}</span>` +
                `<button class="info-meta-del" title="Remove from pending" data-key="${escapeHtml(e.key)}">×</button>` +
            `</div>` +
            `<div class="info-pub-date">pending</div>` +
        `</div>`;
    });

    return this.section('Publications', [...publishedCards, ...pendingCards]);
}
```

```js
// infopanel.js:426-428
const genericEntries = (ctx.entries || []).filter(e =>
    e.key !== 'title' && !e.key.startsWith('built:') && !e.key.startsWith('pending:')
);
```

The existing generic `.info-meta-del` click handler (`infopanel.js:630-648`) already calls `ctx.onDelete(key)` then filters `ctx.entries` locally — no change needed there, since `pending:<slug>` has no related sub-keys to also purge (unlike `built:<slug>`'s `isMainBuildKey` refresh branch).

- [ ] **Step 6: Manual verification**

In the running dev app: collect a photo into a new draft, open its Info Panel, confirm a "pending" Publications card appears; remove it via the × button, confirm it disappears and the draft's photo count drops (check via the Channels dialog status line or Published tab).

- [ ] **Step 7: Commit**

```bash
git add src/internal/api/library/handler.go src/web/js/infopanel.js
git commit -m "feat: show pending and published state in the Info Panel Publications card"
```

---

## Task 10: CSS

**Files:**
- Modify: `src/web/css/style.css`.

- [ ] **Step 1: Add new rules**

```css
/* --- Publish dialog --- */
.publish-dialog { width: 520px; max-width: 90vw; }
.publish-step-photos { display: flex; flex-wrap: wrap; gap: var(--space-2); margin: var(--space-3) 0; }
.publish-step-photo { position: relative; width: 72px; height: 72px; border-radius: var(--radius-sm); overflow: hidden; }
.publish-step-thumb { width: 100%; height: 100%; object-fit: cover; display: block; }
.publish-step-remove {
    position: absolute; top: 2px; right: 2px;
    width: 18px; height: 18px; border-radius: 50%;
    background: rgba(0,0,0,0.6); color: #fff; border: none;
    font-size: 12px; line-height: 1; cursor: pointer;
}
.publish-step-review { margin: var(--space-3) 0; }
.publish-step-review-actions { display: flex; gap: var(--space-2); margin-top: var(--space-2); }

/* --- Channel status line (replaces the old per-row action buttons) --- */
.ch-status-line {
    display: block; margin-top: var(--space-1);
    font-family: var(--font-mono); font-size: 0.8rem; color: var(--text-dim);
    background: none; border: none; padding: 0; text-align: left; cursor: pointer;
}
.ch-status-line:hover { color: var(--text); text-decoration: underline; }

/* --- Published tab status badges (extends existing .pub-gal-status--pending/--ok/--down) --- */
.pub-gal-status--draft { color: var(--text-dim); }
.pub-gal-status--generated { color: var(--accent); }

/* --- Info Panel pending publication card --- */
.info-pub-card--pending { opacity: 0.75; border-style: dashed; }
```

Check the exact existing `.pub-gal-status--pending/--ok/--down` rule (`style.css` around line 5374-5460 per the earlier exploration) before adding `--draft`/`--generated` — match its selector shape (likely `.pub-gal-status--pending { ... }` on a `<span>`, not a modifier needing a base class) exactly, and place the two new rules directly after it rather than in a separate block, per the CLAUDE.md rule to keep one `/* --- Component --- */` section per component.

Also remove now-dead CSS for classes deleted in Task 7 (`.channel-albums-modal`, `.album-row*`, `.album-badge*`, `.channel-row-deploy-result*` if `_showAlbums`/`_deployChannel` are fully gone and nothing else uses them — grep the whole `src/web/js/` tree for each class name first to be sure) and Task 6 (`.build-modal`-specific rules no longer used once `_openBuildModal` is deleted — but check `_openCollectModal`'s new `.collect-modal` doesn't still want most of `.build-*`'s styling; if the new dialog reuses `.build-info`/`.build-error`/`.build-destination` classes as written in Task 6's markup, keep those, only remove genuinely orphaned ones like `.build-date-row`, `.build-time-toggle`, `.build-xmp-row` if nothing references them anymore).

- [ ] **Step 2: Manual visual check**

Run `unterlumen-dev`, open each touched dialog (Add to channel, Channels, Publish), confirm no unstyled/broken elements in both light and dark mode (this app has a theme toggle — check both).

- [ ] **Step 3: Commit**

```bash
git add src/web/css/style.css
git commit -m "style: add publish-dialog and channel-status CSS, remove dead rules"
```

---

## Task 11: Documentation

**Files:**
- Create: `doc/features/open/2026-09-06-streamlined-publish-workflow.md`
- Create: `doc/architecture/adr/0026-deferred-publish-drafts.md`
- Modify: `doc/architecture/arc42.md` (section 9 ADR index)
- Modify: `CHANGELOG.md` (`## [Unreleased]` → `### Changed`, merge into existing heading if present)
- Modify: `README.md` if the Documentation section lists ADRs (per CLAUDE.md reminder)

- [ ] **Step 1: Feature doc**

```markdown
# Streamlined Publish Workflow

*Last modified: 2026-09-06*

## Summary

Replaces the scattered Build / Channels-actions / Deploy / Published surfaces
with a two-phase collect → publish model: collecting photos into a channel is
lightweight and deferred (no export, no HTML, until you're ready); publishing
is one dialog that generates the artifact, lets you review it, and deploys it.

## Details

- "Add to channel…" (library toolbar) replaces "Build…" — adds selected
  photos to a channel's pending draft. Nothing is written to disk.
- The Published tab shows Draft / Generated / Live / Live · N pending status
  per gallery/album, with a Publish action that opens the new 4-step dialog:
  review pending photos → Generate (export + build HTML) → review the
  artifact (copy path / open folder / open in browser) → Deploy (rsync
  channels only).
- The Channels dialog is settings-only now, with a status line linking into
  the Published tab instead of per-row action buttons.
- The Info Panel's Publications card shows both pending and published state
  per channel.

## Acceptance Criteria

- [ ] Selecting photos and clicking "Add to channel…" creates/updates a draft
      with no files written to the channel's output directory.
- [ ] The Published tab shows a Draft-status row for a channel with only
      pending photos, and a "Live · N pending" row for a channel with both
      generated and newly-collected photos.
- [ ] The Publish dialog's Generate step produces the same gallery/site
      output as the old Build action did, for both single-gallery and
      multi-album site channels.
- [ ] The Publish dialog's review step offers copy-path / open-folder for
      plain-export channels, plus open-in-browser for gallery/site channels.
- [ ] Deploy only appears for channels with an rsync handler, and only after
      Generate has run.
- [ ] The Channels dialog has no Rebuild/Albums/Deploy/Published buttons left
      on channel rows.
- [ ] The Info Panel shows a "pending" Publications card for a collected but
      not-yet-generated photo, and a "published" card after Generate.
```

- [ ] **Step 2: ADR**

```markdown
# 0026. Deferred publish drafts replace immediate build

*Last modified: 2026-09-06*

## Status

Accepted

## Context

The previous "Build" action exported photos and generated gallery/site HTML
in a single, immediate step, with no way to preview the result before an
optional later Deploy. Collecting photos into a channel and publishing them
were the same action, which made it impossible to build up a gallery
incrementally over multiple sessions without repeatedly regenerating and
re-deploying partial content.

## Decision

Introduce a `drafts.json` per channel (`internal/channels.DraftStore`)
holding pending `{libraryID, photoID}` references with no files written.
Collecting photos into a channel only touches this draft. A separate
Generate action (the old build pipeline, refactored to read from a draft)
performs the actual export and HTML generation; deploy remains a further,
optional, separate step for handler-backed channels. A photo's pending vs.
published state is tracked via a `pending:<slug>` library-meta key, written
on collect and replaced by the existing `built:<slug>` key on generate,
reusing the mechanism the Info Panel's Publications card already read.

## Consequences

- Channel output directories are only ever written to by Generate, never by
  collect — makes it safe to add and remove photos from a pending gallery
  freely before committing to a build.
- A draft can span multiple libraries (each collect call is scoped to one
  library, like the old build action was); Generate opens one `*lib.Store`
  per distinct library referenced by the draft.
- The old immediate `POST /api/library/{id}/build` endpoint and its
  `Rebuild`/`Rebuild site`/`Albums` per-channel actions are superseded by
  Generate (with zero pending photos, Generate is the new "Rebuild"). The
  Go handlers backing the old maintenance-only `rebuild-site`/
  `rebuild-galleries` routes are left in place as unreferenced code pending a
  follow-up cleanup, since removing working backend code is out of scope for
  this UI-focused change.
```

- [ ] **Step 3: Update `arc42.md` section 9 index and `CHANGELOG.md`**

Read both files' existing structure first (per CLAUDE.md: don't duplicate a `### Changed` heading if one already exists under `## [Unreleased]`) and add:
- `arc42.md` section 9: `- [ADR-0026](adr/0026-deferred-publish-drafts.md) — Deferred publish drafts replace immediate build`
- `CHANGELOG.md` under `## [Unreleased]` → `### Changed` (create the heading only if missing): `- Replaced the Build/Channels-actions/Deploy/Published workflow with a collect-then-publish model: "Add to channel…" defers export/generation until you explicitly Publish, which now includes a review step before any deploy.`

- [ ] **Step 4: Commit**

```bash
git add doc/features/open/2026-09-06-streamlined-publish-workflow.md \
        doc/architecture/adr/0026-deferred-publish-drafts.md \
        doc/architecture/arc42.md CHANGELOG.md README.md
git commit -m "docs: document the streamlined publish workflow (feature doc + ADR-0026)"
```

---

## Task 12: End-to-end test

**Files:**
- Create: `e2e/specs/publish-workflow.spec.js`
- Read first: `e2e/NOTES.md` (app-init race, selector quirks, library search async build, modifier key differences — all called out in CLAUDE.md as non-obvious gotchas that this spec must account for) and `e2e/specs/published-galleries.spec.js` (already exists per `git status`, covers the pre-redesign Published tab — copy its library/fixture setup pattern rather than reinventing it).

- [ ] **Step 1: Write the spec**

Cover, for a plain-export channel (Instagram-style) and a site-export channel, in one spec file with two `test()` blocks sharing a setup helper:
1. Select photos in the Libraries view, click "Add to channel…", fill in a new gallery title (site channel) or just confirm (plain channel), assert a toast appears and the Published tab shows a `Draft` badge.
2. Open the Publish dialog from that row, assert the pending photo thumbnails render, remove one, assert the count drops.
3. Click Generate, assert the artifact-review step appears with a Copy Path button (and, for the site channel, an Open in Browser button).
4. For the site channel only (no rsync configured in the e2e fixture environment — confirm via `e2e/NOTES.md`/existing fixture setup whether a fake rsync target is available; if not, assert the Deploy button is simply absent when the channel has no handler configured, which is the actual behavior to verify either way): close the dialog and assert the row now shows `Generated` (or `Live` if a deploy path is available in the test environment).
5. Open the Info Panel for one of the published photos and assert a "published" Publications card is present (and, before Generate, that the same photo showed "pending").

- [ ] **Step 2: Run it**

Run: `cd src && go build -o ../unterlumen . && cd ../e2e && npm test -- publish-workflow`
Expected: PASS. Iterate against real selector/timing issues per `e2e/NOTES.md`'s guidance rather than adding blind `waitForTimeout` calls.

- [ ] **Step 3: Commit**

```bash
git add e2e/specs/publish-workflow.spec.js
git commit -m "test: add e2e coverage for the collect-then-publish workflow"
```

---

## Final verification

- [ ] `cd src && go vet ./...` clean.
- [ ] `cd src && go test ./...` all green.
- [ ] `cd e2e && npm test` all green (full suite, not just the new spec — confirm nothing in `published-galleries.spec.js` or elsewhere broke from the removed Build/Deploy UI).
- [ ] Manual pass in `unterlumen-dev`: full collect → publish → deploy loop for both a plain and a site-export channel, in both light and dark theme.
- [ ] `doc/features/open/2026-09-06-streamlined-publish-workflow.md` moved to `doc/features/done/` once all acceptance criteria are checked.
