package export

import (
	"archive/zip"
	"bytes"
	"encoding/json"
	"io"
	"net/http/httptest"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"testing"
	"time"

	lib "huepattl.de/unterlumen/internal/library"
	"huepattl.de/unterlumen/internal/media"
)

func TestOriginalsGoIntoTheZipUnchangedUnderUniqueNames(t *testing.T) {
	dir := t.TempDir()
	a := filepath.Join(dir, "a", "IMG_0001.JPG")
	b := filepath.Join(dir, "b", "IMG_0001.JPG")
	for i, p := range []string{a, b} {
		os.MkdirAll(filepath.Dir(p), 0o755)
		os.WriteFile(p, []byte{byte('A' + i), 0xFF, 0xD8}, 0o644)
	}

	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	names := map[string]int{}
	for _, p := range []string{a, b} {
		if err := addZipEntry(zw, zipItem{abs: p, name: filepath.Base(p)}, FormatOriginal, media.ExportOptions{}, names); err != nil {
			t.Fatal(err)
		}
	}
	zw.Close()

	zr, err := zip.NewReader(bytes.NewReader(buf.Bytes()), int64(buf.Len()))
	if err != nil {
		t.Fatal(err)
	}
	want := map[string]string{"IMG_0001.JPG": "A\xff\xd8", "IMG_0001 (2).JPG": "B\xff\xd8"}
	if len(zr.File) != 2 {
		t.Fatalf("entries = %d, want 2", len(zr.File))
	}
	for _, f := range zr.File {
		rc, _ := f.Open()
		got, _ := io.ReadAll(rc)
		rc.Close()
		if want[f.Name] != string(got) || f.Method != zip.Store {
			t.Errorf("%s: %q (method %d), want %q stored", f.Name, got, f.Method, want[f.Name])
		}
	}
}

func TestUniqueEntryNameSkipsTakenSuffixes(t *testing.T) {
	names := map[string]int{}
	got := []string{
		uniqueEntryName(names, "x.jpg"),
		uniqueEntryName(names, "x (2).jpg"),
		uniqueEntryName(names, "x.jpg"),
	}
	want := []string{"x.jpg", "x (2).jpg", "x (2) (2).jpg"}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("got %v, want %v", got, want)
			break
		}
	}
}

func TestZipDownloadName(t *testing.T) {
	cases := map[string]string{
		"/api/export/zip-download?token=t":                         "export.zip",
		"/api/export/zip-download?token=t&name=Travel.zip":         "Travel.zip",
		"/api/export/zip-download?token=t&name=../../etc/passwd":   "export.zip",
		"/api/export/zip-download?token=t&name=..%2F..%2Fevil.zip": "evil.zip",
	}
	for url, want := range cases {
		if got := zipDownloadName(httptest.NewRequest("GET", url, nil)); got != want {
			t.Errorf("%s: %q, want %q", url, got, want)
		}
	}
}

func TestAZipOfNothingIsAnError(t *testing.T) {
	var events []zipStreamEvent
	_, err := buildZipFile(t.Context(), nil, FormatOriginal, media.ExportOptions{}, func(e zipStreamEvent) { events = append(events, e) })
	if err == nil {
		t.Fatal("no error for a ZIP with nothing in it")
	}
	if last := events[len(events)-1]; last.Error != "none of the photos could be read" || last.Complete {
		t.Errorf("last event = %+v", last)
	}
}

func write(t *testing.T, p string) {
	t.Helper()
	os.MkdirAll(filepath.Dir(p), 0o755)
	if err := os.WriteFile(p, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
}

func itemNames(items []zipItem) []string {
	var names []string
	for _, it := range items {
		names = append(names, it.name)
	}
	sort.Strings(names)
	return names
}

func TestCollectWalksFoldersKeepingTheirShape(t *testing.T) {
	root := t.TempDir()
	write(t, filepath.Join(root, "Travel", "a.jpg"))
	write(t, filepath.Join(root, "Travel", "2024", "b.HEIC"))
	write(t, filepath.Join(root, "Travel", "notes.txt"))
	write(t, filepath.Join(root, "Travel", ".DS_Store"))
	write(t, filepath.Join(root, "Travel", ".unterlumen", "c.jpg"))
	write(t, filepath.Join(root, "single.jpg"))

	items, refused := zipSources{root: root, serverRole: true}.collect(exportRequest{
		Files: []string{"single.jpg", "/etc/passwd"},
		Dirs:  []string{"Travel", "../outside"},
	})
	want := []string{"Travel/2024/b.HEIC", "Travel/a.jpg", "single.jpg"}
	if got := itemNames(items); !reflect.DeepEqual(got, want) {
		t.Errorf("items = %v, want %v", got, want)
	}
	if !reflect.DeepEqual(refused, []string{"passwd", "outside"}) {
		t.Errorf("refused = %v", refused)
	}
}

func TestCollectFindsLibraryPhotosByID(t *testing.T) {
	root := t.TempDir()
	photo := filepath.Join(root, "2023", "IMG_7.jpg")
	write(t, photo)
	mgr, err := lib.NewManager(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	l, err := mgr.CreateLibrary("L", "", root)
	if err != nil {
		t.Fatal(err)
	}
	store, _ := mgr.OpenStore(l.ID)
	store.UpsertPhoto("p7", photo, "IMG_7.jpg", 1, time.Now(), "{}", "", "", "jpeg")
	store.Close()

	items, refused := zipSources{root: t.TempDir(), serverRole: true, libs: mgr}.collect(exportRequest{
		Photos: []photoRef{{Library: l.ID, ID: "p7"}, {Library: l.ID, ID: "nope"}},
	})
	if len(items) != 1 || items[0].abs != photo || items[0].name != "IMG_7.jpg" {
		t.Errorf("items = %+v", items)
	}
	if !reflect.DeepEqual(refused, []string{"nope"}) {
		t.Errorf("refused = %v", refused)
	}
}

func TestServerModeKeepsTheSourceFolderInsideTheRoot(t *testing.T) {
	root := t.TempDir()
	inside := filepath.Join(root, "lib")
	os.MkdirAll(inside, 0o755)
	outside := t.TempDir()

	if got := effectiveRoot(root, outside, true); got == outside {
		t.Error("server mode took a source folder outside the root")
	}
	if got, _ := filepath.EvalSymlinks(effectiveRoot(root, inside, true)); got != mustEval(t, inside) {
		t.Errorf("server mode refused a source folder inside the root: %s", got)
	}
	if got := effectiveRoot(root, outside, false); got != outside {
		t.Error("desktop mode should keep a library's own source folder")
	}
}

func mustEval(t *testing.T, p string) string {
	t.Helper()
	r, err := filepath.EvalSymlinks(p)
	if err != nil {
		t.Fatal(err)
	}
	return r
}

func TestEstimateFindsLibraryPhotosByIDAndAnswersUnderTheirKey(t *testing.T) {
	root := t.TempDir()
	photo := filepath.Join(root, "IMG_9.jpg")
	write(t, photo)
	mgr, err := lib.NewManager(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	l, _ := mgr.CreateLibrary("L", "", root)
	store, _ := mgr.OpenStore(l.ID)
	store.UpsertPhoto("p9", photo, "IMG_9.jpg", 1, time.Now(), "{}", "", "", "jpeg")
	store.Close()

	body := `{"format":"jpeg","quality":80,"method":"heuristic","photos":[{"library":"` + l.ID + `","id":"p9","key":"/abs/IMG_9.jpg"}]}`
	rec := httptest.NewRecorder()
	handleExportEstimate(t.TempDir(), true, mgr)(rec, httptest.NewRequest("POST", "/api/export/estimate", strings.NewReader(body)))

	var resp estimateResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &resp); err != nil {
		t.Fatalf("decode %q: %v", rec.Body.String(), err)
	}
	if len(resp.Estimates) != 1 || resp.Estimates[0].File != "/abs/IMG_9.jpg" || resp.Estimates[0].InputBytes != 1 {
		t.Errorf("estimates = %+v", resp.Estimates)
	}
}
