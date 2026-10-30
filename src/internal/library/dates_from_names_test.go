package library

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestUndatedPhotosAreDatedFromTheirNames(t *testing.T) {
	s := newTestStore(t)
	for _, p := range [][2]string{{"w1", "WhatsApp Image 2026-09-20 at 17.17.36.jpeg"}, {"a1", "AIDA 1.heic"}} {
		if err := s.UpsertPhoto(p[0], "/lib/"+p[1], p[1], 10, time.Now(), "{}", "", "", "jpeg"); err != nil {
			t.Fatal(err)
		}
	}
	n, err := s.DateUndatedFromNames()
	if err != nil || n != 1 {
		t.Fatalf("dated %d, %v; want 1", n, err)
	}
	ids, _ := listedIDs(t, s, ListPhotosOpts{})
	if len(ids) != 2 || ids[0] != "w1" {
		t.Errorf("order = %v, want the dated WhatsApp image first", ids)
	}
	var taken, json string
	s.db.QueryRow(`SELECT date_taken, exif_json FROM photos WHERE id = 'w1'`).Scan(&taken, &json)
	if taken != "2026-09-20T17:17:36" || json != `{"dateTaken":"2026-09-20T17:17:36"}` {
		t.Errorf("w1 = %q %s", taken, json)
	}
}

func TestReadingAPhotoWithoutEXIFTakesTheDateOfItsName(t *testing.T) {
	file := filepath.Join(t.TempDir(), "WhatsApp Image 2026-09-20 at 19.27.29.jpeg")
	if err := os.WriteFile(file, []byte("not a jpeg"), 0o600); err != nil {
		t.Fatal(err)
	}
	ex := readPhotoExif(file)
	if ex.dateTaken != "2026-09-20T19:27:29" || ex.fields["DateTakenFrom"] != "file name" {
		t.Errorf("read %q %v", ex.dateTaken, ex.fields)
	}
}
