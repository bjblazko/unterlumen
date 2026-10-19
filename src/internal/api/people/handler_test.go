package apipeople

import (
	"bytes"
	"encoding/json"
	"image"
	"image/jpeg"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strconv"
	"testing"
	"time"

	"huepattl.de/unterlumen/internal/faces"
	"huepattl.de/unterlumen/internal/jobs"
	lib "huepattl.de/unterlumen/internal/library"
)

// libraryWithFaces makes a library of two photos showing the same face,
// with thumbnails, and returns the routes and the library's ID.
func libraryWithFaces(t *testing.T) (http.Handler, *lib.Manager, string) {
	t.Helper()
	mgr, err := lib.NewManager(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	source := t.TempDir()
	l, err := mgr.CreateLibrary("Test", "", source)
	if err != nil {
		t.Fatal(err)
	}
	store, _ := mgr.OpenStore(l.ID)
	emb := make([]float32, 128)
	emb[0] = 1
	var found []lib.FoundFaces
	for _, id := range []string{"aa1", "bb2"} {
		thumb := filepath.Join("thumbs", id[:2], id+".jpg")
		writeJPEG(t, filepath.Join(mgr.LibDir(l.ID), thumb))
		if err := store.UpsertPhoto(id, filepath.Join(source, id+".jpg"), id+".jpg", 1, time.Now(), "{}", thumb, "", "jpeg"); err != nil {
			t.Fatal(err)
		}
		found = append(found, lib.FoundFaces{PhotoID: id, Faces: []faces.Face{{X: 0.4, Y: 0.4, W: 0.2, H: 0.2, Score: 0.9, Embedding: emb}}, Names: []string{""}})
	}
	if err := store.SaveFoundFaces(found); err != nil {
		t.Fatal(err)
	}
	if err := store.Regroup(); err != nil {
		t.Fatal(err)
	}
	mux := http.NewServeMux()
	Handle(mux, mgr, jobs.NewRegistry())
	return mux, mgr, l.ID
}

func writeJPEG(t *testing.T, path string) {
	t.Helper()
	os.MkdirAll(filepath.Dir(path), 0o700) //nolint:errcheck
	f, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	jpeg.Encode(f, image.NewRGBA(image.Rect(0, 0, 300, 200)), nil) //nolint:errcheck
}

func do(t *testing.T, h http.Handler, method, url string, body any) *httptest.ResponseRecorder {
	t.Helper()
	var buf bytes.Buffer
	if body != nil {
		json.NewEncoder(&buf).Encode(body) //nolint:errcheck
	}
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(method, url, &buf))
	return rec
}

type overviewBody struct {
	People     []lib.Person    `json:"people"`
	Groups     []lib.FaceGroup `json:"groups"`
	GroupCount int             `json:"groupCount"`
}

func readOverview(t *testing.T, h http.Handler, id string) overviewBody {
	t.Helper()
	rec := do(t, h, "GET", "/api/library/"+id+"/people", nil)
	if rec.Code != http.StatusOK {
		t.Fatalf("overview: %d %s", rec.Code, rec.Body)
	}
	var o overviewBody
	json.Unmarshal(rec.Body.Bytes(), &o) //nolint:errcheck
	return o
}

func TestNamingAGroupThroughTheAPI(t *testing.T) {
	h, _, id := libraryWithFaces(t)
	o := readOverview(t, h, id)
	if len(o.Groups) != 1 || o.Groups[0].Size != 2 || o.GroupCount != 1 {
		t.Fatalf("overview %+v, want one group of two", o)
	}
	rec := do(t, h, "POST", "/api/library/"+id+"/faces/name", map[string]any{"faces": o.Groups[0].Faces, "name": "Anna"})
	if rec.Code != http.StatusOK {
		t.Fatalf("name: %d %s", rec.Code, rec.Body)
	}
	o = readOverview(t, h, id)
	if len(o.People) != 1 || o.People[0].Name != "Anna" || len(o.Groups) != 0 {
		t.Fatalf("after naming %+v", o)
	}
	rec = do(t, h, "PATCH", "/api/library/"+id+"/people/"+itoa(o.People[0].ID), map[string]string{"name": "Anne"})
	if rec.Code != http.StatusOK {
		t.Fatalf("rename: %d %s", rec.Code, rec.Body)
	}
	if o = readOverview(t, h, id); o.People[0].Name != "Anne" {
		t.Fatalf("after renaming %+v", o.People)
	}
}

func TestChangesWithoutFacesOrNameAreRefused(t *testing.T) {
	h, _, id := libraryWithFaces(t)
	cases := []struct {
		url  string
		body any
	}{
		{"/api/library/" + id + "/faces/name", map[string]any{"faces": []int64{1}, "name": "  "}},
		{"/api/library/" + id + "/faces/name", map[string]any{"name": "Anna"}},
		{"/api/library/" + id + "/faces/hide", map[string]any{}},
	}
	for _, c := range cases {
		if rec := do(t, h, "POST", c.url, c.body); rec.Code != http.StatusBadRequest {
			t.Errorf("%s %v: %d, want 400", c.url, c.body, rec.Code)
		}
	}
	if rec := do(t, h, "GET", "/api/library/nope/people", nil); rec.Code != http.StatusNotFound {
		t.Errorf("unknown library: %d, want 404", rec.Code)
	}
}

func TestAFaceIsServedAsASquareJPEG(t *testing.T) {
	h, _, id := libraryWithFaces(t)
	o := readOverview(t, h, id)
	rec := do(t, h, "GET", "/api/library/"+id+"/face/"+itoa(o.Groups[0].Faces[0]), nil)
	if rec.Code != http.StatusOK || rec.Header().Get("Content-Type") != "image/jpeg" {
		t.Fatalf("face: %d %s", rec.Code, rec.Header().Get("Content-Type"))
	}
	img, err := jpeg.Decode(rec.Body)
	if err != nil || img.Bounds().Dx() != cropSize || img.Bounds().Dy() != cropSize {
		t.Fatalf("decoded %v, %v", img.Bounds(), err)
	}
	again := httptest.NewRequest("GET", "/api/library/"+id+"/face/"+itoa(o.Groups[0].Faces[0]), nil)
	again.Header.Set("If-None-Match", rec.Header().Get("ETag"))
	rec2 := httptest.NewRecorder()
	h.ServeHTTP(rec2, again)
	if rec2.Code != http.StatusNotModified {
		t.Fatalf("revalidation: %d, want 304", rec2.Code)
	}
}

func TestStatusSaysNotInstalled(t *testing.T) {
	h, _, _ := libraryWithFaces(t)
	rec := do(t, h, "GET", "/api/faces", nil)
	var s struct {
		Installed bool `json:"installed"`
	}
	if json.Unmarshal(rec.Body.Bytes(), &s) != nil || s.Installed {
		t.Fatalf("status %s", rec.Body)
	}
}

func itoa(n int64) string { return strconv.FormatInt(n, 10) }

// A named person is a filter of the library search: the photos showing them,
// with their name in any case, and the names to choose from.
func TestAPersonFiltersTheLibrarySearch(t *testing.T) {
	h, mgr, id := libraryWithFaces(t)
	o := readOverview(t, h, id)
	do(t, h, "POST", "/api/library/"+id+"/faces/name", map[string]any{"faces": o.Groups[0].Faces[:1], "name": "Anna"})

	rec := do(t, h, "GET", "/api/library/person-names", nil)
	var names []string
	json.Unmarshal(rec.Body.Bytes(), &names) //nolint:errcheck
	if len(names) != 1 || names[0] != "Anna" {
		t.Fatalf("names %s", rec.Body)
	}
	store, _ := mgr.OpenStore(id)
	res, err := store.ListPhotos(lib.ListPhotosOpts{People: []string{"anna"}, Limit: 10})
	if err != nil || res.Total != 1 || len(res.Photos) != 1 {
		t.Fatalf("photos of Anna: %+v, %v", res, err)
	}
}

// The info panel reads a photo's faces with the names to choose from; a
// person can be forgotten, which puts their faces back into a group.
func TestAPhotosFacesAndForgettingAPerson(t *testing.T) {
	h, _, id := libraryWithFaces(t)
	o := readOverview(t, h, id)
	do(t, h, "POST", "/api/library/"+id+"/faces/name", map[string]any{"faces": o.Groups[0].Faces, "name": "Anna"})

	rec := do(t, h, "GET", "/api/library/"+id+"/photo/aa1/faces", nil)
	var body struct {
		Faces []lib.PhotoFace `json:"faces"`
		Names []string        `json:"names"`
	}
	json.Unmarshal(rec.Body.Bytes(), &body) //nolint:errcheck
	if len(body.Faces) != 1 || body.Faces[0].Name != "Anna" || body.Faces[0].W == 0 || len(body.Names) != 1 {
		t.Fatalf("photo faces %s", rec.Body)
	}

	o = readOverview(t, h, id)
	if rec := do(t, h, "DELETE", "/api/library/"+id+"/people/"+itoa(o.People[0].ID), nil); rec.Code != http.StatusOK {
		t.Fatalf("forget: %d %s", rec.Code, rec.Body)
	}
	if o = readOverview(t, h, id); len(o.People) != 0 || len(o.Groups) != 1 || o.Groups[0].Size != 2 {
		t.Fatalf("after forgetting Anna: %+v", o)
	}
}

// Two people chips find the photos with both.
func TestSeveralPeopleNarrowTogether(t *testing.T) {
	h, mgr, id := libraryWithFaces(t)
	o := readOverview(t, h, id)
	do(t, h, "POST", "/api/library/"+id+"/faces/name", map[string]any{"faces": o.Groups[0].Faces, "name": "Anna"})
	store, _ := mgr.OpenStore(id)
	if res, _ := store.ListPhotos(lib.ListPhotosOpts{People: []string{"Anna"}, Limit: 10}); res.Total != 2 {
		t.Fatalf("Anna alone: %d photos, want 2", res.Total)
	}
	if res, _ := store.ListPhotos(lib.ListPhotosOpts{People: []string{"Anna", "Ben"}, Limit: 10}); res.Total != 0 {
		t.Fatalf("Anna with Ben: %d photos, want 0", res.Total)
	}
}
