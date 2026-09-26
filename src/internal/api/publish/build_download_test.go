package publish

import (
	"archive/zip"
	"bytes"
	"net/http"
	"net/http/httptest"
	"regexp"
	"sort"
	"strings"
	"testing"

	"huepattl.de/unterlumen/internal/channels"
	"huepattl.de/unterlumen/internal/media"
)

func downloadRequest(t *testing.T, h http.HandlerFunc, method, libID, body string) *httptest.ResponseRecorder {
	t.Helper()
	req := httptest.NewRequest(method, "/api/library/"+libID+"/build-download", strings.NewReader(body))
	req.SetPathValue("id", libID)
	rec := httptest.NewRecorder()
	h(rec, req)
	return rec
}

func downloadFixture(t *testing.T) (http.HandlerFunc, *channels.Store, string) {
	t.Helper()
	mgr := newTestManager(t)
	dir := t.TempDir()
	chStore := channels.NewStore(dir, dir)
	if err := chStore.Save(&channels.Channel{Slug: "insta", Name: "Instagram", Format: "jpeg", Quality: 85}); err != nil {
		t.Fatal(err)
	}
	libID := seedLibraryPhoto(t, mgr, "photo1")
	return buildDownload(mgr, chStore), chStore, libID
}

func zipNames(t *testing.T, data []byte) []string {
	t.Helper()
	zr, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		t.Fatalf("not a zip: %v", err)
	}
	var names []string
	for _, f := range zr.File {
		names = append(names, f.Name)
	}
	sort.Strings(names)
	return names
}

func TestBuildDownloadZipsTheKnownPhotos(t *testing.T) {
	h, _, libID := downloadFixture(t)
	rec := downloadRequest(t, h, "POST", libID, `{"photoIDs":["photo1","unknown"],"channel":"insta"}`)
	if rec.Code != 200 {
		t.Fatalf("status = %d, body = %s", rec.Code, rec.Body.String())
	}
	if ct, cd := rec.Header().Get("Content-Type"), rec.Header().Get("Content-Disposition"); ct != "application/zip" || cd != `attachment; filename="insta-export.zip"` {
		t.Errorf("headers = %q, %q", ct, cd)
	}
	names := zipNames(t, rec.Body.Bytes())
	if len(names) != 1 || !regexp.MustCompile(`^insta_\d{8}T\d{6}Z_photo1\.jpg$`).MatchString(names[0]) {
		t.Errorf("zip entries = %v, want one insta_<ts>_photo1.jpg", names)
	}
}

func TestBuildDownloadRecordsThePublicationWhenAsked(t *testing.T) {
	mgr := newTestManager(t)
	dir := t.TempDir()
	chStore := channels.NewStore(dir, dir)
	if err := chStore.Save(&channels.Channel{Slug: "insta", Name: "Instagram", Format: "jpeg", Quality: 85}); err != nil {
		t.Fatal(err)
	}
	libID := seedLibraryPhoto(t, mgr, "photo1")
	h := buildDownload(mgr, chStore)

	if rec := downloadRequest(t, h, "POST", libID, `{"photoIDs":["photo1"],"channel":"insta"}`); rec.Code != 200 {
		t.Fatalf("status = %d", rec.Code)
	}
	if meta := photoMeta(t, mgr, libID, "photo1"); len(meta) != 0 {
		t.Errorf("meta after a plain download = %v, want none", meta)
	}

	if rec := downloadRequest(t, h, "POST", libID, `{"photoIDs":["photo1"],"channel":"insta","recordXMP":true}`); rec.Code != 200 {
		t.Fatalf("status = %d", rec.Code)
	}
	meta := photoMeta(t, mgr, libID, "photo1")
	postID := meta["built:insta:postid"]
	if postID == "" || meta["built:insta"] == "" || meta["built:insta:"+postID] == "" || len(meta) != 3 {
		t.Errorf("meta = %v, want built:insta, built:insta:<postID> and built:insta:postid", meta)
	}
	store, err := mgr.OpenStore(libID)
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	hint, _ := store.GetPhotoPathHint("photo1")
	pubs, _ := media.ReadSidecar(hint)
	if len(pubs) != 1 || pubs[0].Channel != "insta" || pubs[0].PostID != postID {
		t.Errorf("sidecar = %+v, want one insta publication %s", pubs, postID)
	}
}

func TestBuildDownloadAnswersErrors(t *testing.T) {
	h, _, libID := downloadFixture(t)
	cases := []struct {
		name, method, lib, body string
		status                  int
		text                    string
	}{
		{"not POST", "GET", libID, ``, 405, "method not allowed"},
		{"not JSON", "POST", libID, `{`, 400, "invalid JSON"},
		{"no photos", "POST", libID, `{"channel":"insta"}`, 400, "photoIDs and channel required"},
		{"no destination", "POST", libID, `{"photoIDs":["photo1"]}`, 400, "photoIDs and channel required"},
		{"unknown destination", "POST", libID, `{"photoIDs":["photo1"],"channel":"nope"}`, 400, "channel not found"},
		{"unknown library", "POST", "nolib", `{"photoIDs":["photo1"],"channel":"insta"}`, 404, "library not found"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			rec := downloadRequest(t, h, c.method, c.lib, c.body)
			if rec.Code != c.status || !strings.Contains(rec.Body.String(), c.text) {
				t.Errorf("answer = %d %q, want %d containing %q", rec.Code, strings.TrimSpace(rec.Body.String()), c.status, c.text)
			}
		})
	}
}
