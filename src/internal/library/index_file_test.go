package library

import (
	"database/sql"
	"image"
	"image/jpeg"
	"os"
	"path/filepath"
	"testing"
)

func writeIndexJPEG(t *testing.T, path string) string {
	t.Helper()
	f, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := jpeg.Encode(f, image.NewRGBA(image.Rect(0, 0, 40, 30)), nil); err != nil {
		t.Fatal(err)
	}
	f.Close()
	id, err := hashFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return id
}

type photoRow struct {
	ext, exifJSON, thumb string
	date                 sql.NullString
}

func readPhotoRow(t *testing.T, s *Store, id string) photoRow {
	t.Helper()
	var r photoRow
	if err := s.db.QueryRow(`SELECT ext, exif_json, thumb_path, date_taken FROM photos WHERE id=?`, id).Scan(&r.ext, &r.exifJSON, &r.thumb, &r.date); err != nil {
		t.Fatalf("read photo %s: %v", id, err)
	}
	return r
}

func TestIndexFileAddsANewPhoto(t *testing.T) {
	src := t.TempDir()
	path := filepath.Join(src, "b.JPG")
	id := writeIndexJPEG(t, path)
	idx, s := newTestIndexer(t, src)

	if err := idx.IndexFile(path); err != nil {
		t.Fatal(err)
	}
	r := readPhotoRow(t, s, id)
	if r.ext != "jpeg" || r.thumb == "" || r.date.Valid || r.exifJSON == "" {
		t.Errorf("row = %+v, want ext jpeg, a thumbnail, no date, some EXIF JSON", r)
	}
	if _, err := os.Stat(filepath.Join(s.dir, r.thumb)); err != nil {
		t.Errorf("thumbnail %s not on disk: %v", r.thumb, err)
	}
	if cached, _, _, found, _ := s.GetPathCache(path); !found || cached != id {
		t.Errorf("path cache = %q (found %v), want %q", cached, found, id)
	}
	if idx.newPhotos != 1 {
		t.Errorf("newPhotos = %d, want 1", idx.newPhotos)
	}
}

func TestIndexFileSkipsAnUnchangedFile(t *testing.T) {
	src := t.TempDir()
	path := filepath.Join(src, "a.jpg")
	id := writeIndexJPEG(t, path)
	idx, s := newTestIndexer(t, src)
	if err := idx.IndexFile(path); err != nil {
		t.Fatal(err)
	}
	if err := s.UpdatePhotoExif(id, `{"marker":true}`, ""); err != nil {
		t.Fatal(err)
	}

	if err := idx.IndexFile(path); err != nil {
		t.Fatal(err)
	}
	if r := readPhotoRow(t, s, id); r.exifJSON != `{"marker":true}` {
		t.Errorf("exif_json = %s, want it untouched by the fast path", r.exifJSON)
	}
	if idx.newPhotos != 1 {
		t.Errorf("newPhotos = %d, want 1", idx.newPhotos)
	}
}

func TestForceReindexFileRefreshesAndKeepsMeta(t *testing.T) {
	src := t.TempDir()
	path := filepath.Join(src, "a.jpg")
	id := writeIndexJPEG(t, path)
	idx, s := newTestIndexer(t, src)
	if err := idx.IndexFile(path); err != nil {
		t.Fatal(err)
	}
	if err := s.UpsertMeta(id, "rating", "5"); err != nil {
		t.Fatal(err)
	}
	if err := s.UpdatePhotoExif(id, `{"marker":true}`, ""); err != nil {
		t.Fatal(err)
	}
	thumb := filepath.Join(s.dir, readPhotoRow(t, s, id).thumb)
	os.Remove(thumb) //nolint:errcheck

	if err := idx.forceReindexFile(path); err != nil {
		t.Fatal(err)
	}
	r := readPhotoRow(t, s, id)
	if r.exifJSON == `{"marker":true}` || r.ext != "jpeg" {
		t.Errorf("row = %+v, want re-extracted EXIF", r)
	}
	if _, err := os.Stat(thumb); err != nil {
		t.Errorf("thumbnail not rebuilt: %v", err)
	}
	if entries, _ := s.GetMeta(id); len(entries) != 1 || entries[0].Value != "5" {
		t.Errorf("meta = %+v, want the rating kept", entries)
	}
}

// forceReindexFile also indexes a file it has not seen, without counting it
// as a new photo — characterized as it is.
func TestForceReindexFileIndexesAnUnknownFile(t *testing.T) {
	src := t.TempDir()
	path := filepath.Join(src, "c.jpeg")
	id := writeIndexJPEG(t, path)
	idx, s := newTestIndexer(t, src)

	if err := idx.forceReindexFile(path); err != nil {
		t.Fatal(err)
	}
	if r := readPhotoRow(t, s, id); r.ext != "jpeg" || r.thumb == "" {
		t.Errorf("row = %+v, want an indexed photo with a thumbnail", r)
	}
	if _, _, _, found, _ := s.GetPathCache(path); !found {
		t.Error("path cache not written")
	}
	if idx.newPhotos != 0 {
		t.Errorf("newPhotos = %d, want 0", idx.newPhotos)
	}
}
