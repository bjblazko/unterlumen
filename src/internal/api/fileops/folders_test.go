package fileops

import (
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"testing"

	"huepattl.de/unterlumen/internal/media"
)

type renameBody struct {
	Path string `json:"path"`
	Name string `json:"name"`
}

func TestHandleDelete_TakesTheSidecar(t *testing.T) {
	root := resolvedTempDir(t)
	photo := filepath.Join(root, "a.jpg")
	writeTestJPEG(t, photo, 11)
	writeTestSidecar(t, photo, "notes")

	rec := doPost(t, handleDelete(root, media.NewScanCache(), nil), fileOpRequest{Files: []string{"a.jpg"}})
	if rec.Code != http.StatusOK {
		t.Fatalf("delete: HTTP %d: %s", rec.Code, rec.Body.String())
	}
	assertNoSidecar(t, photo)
}

func TestHandleDelete_KeepsASidecarARawStillOwns(t *testing.T) {
	root := resolvedTempDir(t)
	photo := filepath.Join(root, "a.jpg")
	writeTestJPEG(t, photo, 12)
	writeTestJPEG(t, filepath.Join(root, "a.raf"), 13)
	writeTestSidecar(t, photo, "edits")

	doPost(t, handleDelete(root, media.NewScanCache(), nil), fileOpRequest{Files: []string{"a.jpg"}})
	assertSidecar(t, filepath.Join(root, "a.raf"), "edits")
}

// A renamed folder's photos keep what the library knew of them: the
// index follows them to the new path.
func TestHandleRename_FolderKeepsItsPhotosInTheLibrary(t *testing.T) {
	root := resolvedTempDir(t)
	os.MkdirAll(filepath.Join(root, "trip", "day 1"), 0755) //nolint:errcheck
	old := filepath.Join(root, "trip", "day 1", "a.jpg")
	writeTestJPEG(t, old, 21)
	mgr, lib := newTestLibrary(t, root)
	if !mgr.IndexFilesSync(lib.ID, []string{old}) {
		t.Fatal("expected the photo to be indexed")
	}

	rec := doPost(t, handleRename(root, media.NewScanCache(), mgr), renameBody{Path: "trip", Name: "Norway"})
	if rec.Code != http.StatusOK {
		t.Fatalf("rename: HTTP %d: %s", rec.Code, rec.Body.String())
	}
	waitForScanIdle(t, mgr, lib.ID)

	statuses := libPhotoStatuses(t, mgr, lib.ID)
	if _, ok := statuses[filepath.Join(root, "Norway", "day 1", "a.jpg")]; !ok {
		t.Errorf("the photo is not at its new path in the library: %v", statuses)
	}
	if _, ok := statuses[old]; ok {
		t.Error("the library still names the old path")
	}
}

func TestHandleRename_RefusesALibrarysFolder(t *testing.T) {
	root := resolvedTempDir(t)
	os.MkdirAll(filepath.Join(root, "Pictures"), 0755) //nolint:errcheck
	mgr, _ := newTestLibrary(t, filepath.Join(root, "Pictures"))

	rec := doPost(t, handleRename(root, media.NewScanCache(), mgr), renameBody{Path: "Pictures", Name: "Photos"})
	if rec.Code != http.StatusConflict {
		t.Fatalf("rename of a library's folder: HTTP %d, want 409", rec.Code)
	}
	if _, err := os.Stat(filepath.Join(root, "Pictures")); err != nil {
		t.Error("the library's folder was renamed")
	}
}

func TestHandleDelete_RefusesAFolderHoldingALibrary(t *testing.T) {
	root := resolvedTempDir(t)
	os.MkdirAll(filepath.Join(root, "all", "Pictures"), 0755) //nolint:errcheck
	mgr, _ := newTestLibrary(t, filepath.Join(root, "all", "Pictures"))

	rec := doPost(t, handleDelete(root, media.NewScanCache(), mgr), fileOpRequest{Files: []string{"all"}})
	var resp fileOpResponse
	json.Unmarshal(rec.Body.Bytes(), &resp) //nolint:errcheck
	if len(resp.Results) != 1 || resp.Results[0].Success {
		t.Fatalf("deleting a folder holding a library should fail: %s", rec.Body.String())
	}
	if _, err := os.Stat(filepath.Join(root, "all", "Pictures")); err != nil {
		t.Error("the library's folder was deleted")
	}
}

func TestEmptiedFolders(t *testing.T) {
	root := resolvedTempDir(t)
	for _, d := range []string{"empty", "junk", "kept", "parent/child"} {
		os.MkdirAll(filepath.Join(root, d), 0755) //nolint:errcheck
	}
	os.WriteFile(filepath.Join(root, "junk", ".DS_Store"), nil, 0644)   //nolint:errcheck
	os.WriteFile(filepath.Join(root, "kept", "notes.txt"), nil, 0644)   //nolint:errcheck
	os.WriteFile(filepath.Join(root, "kept", "._notes.txt"), nil, 0644) //nolint:errcheck

	rec := doPost(t, handleEmptiedFolders(root, nil), foldersRequest{Folders: []string{"", "empty", "junk", "kept", "parent", "../x"}})
	var got struct{ Empty []string }
	json.Unmarshal(rec.Body.Bytes(), &got) //nolint:errcheck
	want := []string{"empty", "junk"}
	if len(got.Empty) != len(want) || got.Empty[0] != want[0] || got.Empty[1] != want[1] {
		t.Fatalf("empty = %v, want %v (never the root, a folder with a file or a subfolder)", got.Empty, want)
	}

	rec = doPost(t, handleRemoveEmptyFolders(root, media.NewScanCache(), nil), foldersRequest{Folders: []string{"junk", "kept"}})
	var resp fileOpResponse
	json.Unmarshal(rec.Body.Bytes(), &resp) //nolint:errcheck
	if !resp.Results[0].Success || resp.Results[1].Success {
		t.Fatalf("remove: %s", rec.Body.String())
	}
	if _, err := os.Stat(filepath.Join(root, "junk")); !os.IsNotExist(err) {
		t.Error("junk should be gone with its .DS_Store")
	}
	if _, err := os.Stat(filepath.Join(root, "kept", "notes.txt")); err != nil {
		t.Error("a folder with a file in it must stay")
	}
}
