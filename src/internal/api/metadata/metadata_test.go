package metadata

import (
	"bytes"
	"encoding/json"
	"image"
	"image/jpeg"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"

	"huepattl.de/unterlumen/internal/library"
	"huepattl.de/unterlumen/internal/media"
)

type fixture struct {
	root, photo string
	mgr         *library.Manager
	libID       string
	mux         *http.ServeMux
}

func newFixture(t *testing.T) fixture {
	t.Helper()
	root, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	src := filepath.Join(root, "lib")
	os.MkdirAll(src, 0o755) //nolint:errcheck
	photo := filepath.Join(src, "2019-03-30_11-40-49_a.jpg")
	var buf bytes.Buffer
	jpeg.Encode(&buf, image.NewGray(image.Rect(0, 0, 8, 8)), nil) //nolint:errcheck
	os.WriteFile(photo, buf.Bytes(), 0o644)                       //nolint:errcheck
	mgr, err := library.NewManager(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	lib, err := mgr.CreateLibrary("Test", "", src)
	if err != nil {
		t.Fatal(err)
	}
	mgr.IndexFilesSync(lib.ID, []string{photo})
	mux := http.NewServeMux()
	Handle(mux, root, media.NewScanCache(), mgr)
	return fixture{root, photo, mgr, lib.ID, mux}
}

func (f fixture) post(t *testing.T, path string, body any, out any) {
	t.Helper()
	b, _ := json.Marshal(body)
	rec := httptest.NewRecorder()
	f.mux.ServeHTTP(rec, httptest.NewRequest(http.MethodPost, path, bytes.NewReader(b)))
	if rec.Code != http.StatusOK {
		t.Fatalf("%s: %d %s", path, rec.Code, rec.Body.String())
	}
	if err := json.Unmarshal(rec.Body.Bytes(), out); err != nil {
		t.Fatal(err)
	}
}

func (f fixture) indexed(t *testing.T) (string, string) {
	t.Helper()
	store, err := f.mgr.OpenStore(f.libID)
	if err != nil {
		t.Fatal(err)
	}
	id, _, _, _, _ := store.GetPathCache(f.photo)
	title := ""
	meta, _ := store.GetMeta(id)
	for _, e := range meta {
		if e.Key == "title" {
			title = e.Value
		}
	}
	return id, title
}

func TestWriteToTheSidecarLeavesTheFile(t *testing.T) {
	f := newFixture(t)
	idBefore, _ := f.indexed(t)
	before, _ := os.ReadFile(f.photo)

	var preview readResponse
	f.post(t, "/api/metadata/read", map[string]any{
		"files":  []string{"lib/2019-03-30_11-40-49_a.jpg"},
		"change": map[string]any{"date": map[string]any{"mode": "day", "day": "2024-05-01"}},
	}, &preview)
	if got := preview.Files[0].Plan.Set["dateTaken"]; got != "2024-05-01T11:40:49" {
		t.Fatalf("preview date %q", got)
	}

	var res writeResponse
	f.post(t, "/api/metadata/write", map[string]any{
		"file":   "lib/2019-03-30_11-40-49_a.jpg",
		"target": "sidecar",
		"change": map[string]any{"set": map[string]string{"title": "Harbour"}, "date": map[string]any{"mode": "day", "day": "2024-05-01"}},
	}, &res)
	if !res.Success || res.After.Fields["dateTaken"].Value != "2024-05-01T11:40:49" || res.After.Fields["dateTaken"].From != "sidecar" {
		t.Fatalf("write %+v", res)
	}
	if now, _ := os.ReadFile(f.photo); !bytes.Equal(now, before) {
		t.Error("the photo changed for a sidecar write")
	}
	id, title := f.indexed(t)
	if id != idBefore || title != "Harbour" || res.IDs[f.libID] != id {
		t.Errorf("index: id %s (was %s), title %q, ids %v", id, idBefore, title, res.IDs)
	}
}

func TestWriteIntoTheFileGivesANewID(t *testing.T) {
	if !media.CheckExiftool() {
		t.Skip("exiftool is not installed")
	}
	f := newFixture(t)
	idBefore, _ := f.indexed(t)
	var res writeResponse
	f.post(t, "/api/metadata/write", map[string]any{
		"file":   "lib/2019-03-30_11-40-49_a.jpg",
		"target": "file",
		"change": map[string]any{"set": map[string]string{"creator": "Ann"}},
	}, &res)
	if !res.Success || res.After.Fields["creator"].Value != "Ann" || res.After.Fields["creator"].From != "file" {
		t.Fatalf("write %+v", res)
	}
	if _, err := os.Stat(media.SidecarPath(f.photo)); err == nil {
		t.Error("a sidecar was written for a write into the file")
	}
	if id, _ := f.indexed(t); id == idBefore || res.IDs[f.libID] != id {
		t.Errorf("the library still knows the old content: %s, ids %v", id, res.IDs)
	}
}
