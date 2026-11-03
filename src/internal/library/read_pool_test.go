package library

import (
	"testing"
	"time"
)

// A page that only reads does not wait for a write in progress (a scan
// holds the writer for a while) and cannot write itself.
func TestReadersReadBesideAWriteAndCannotWrite(t *testing.T) {
	mgr := newTestManager(t)
	l, err := mgr.CreateLibrary("Fotos", "", t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	w, err := mgr.OpenStore(l.ID)
	if err != nil {
		t.Fatal(err)
	}
	seedStatsPhotos(t, w, []statsPhoto{{id: "a", path: "/lib/a.jpg", ext: "jpeg", date: "2024-05-01T10:00:00"}})
	tx, err := w.db.Begin()
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback() //nolint:errcheck
	if _, err := tx.Exec(`UPDATE photos SET filename = 'changing.jpg'`); err != nil {
		t.Fatal(err)
	}
	r, err := mgr.OpenReader(l.ID)
	if err != nil {
		t.Fatal(err)
	}
	done := make(chan string, 1)
	go func() {
		var name string
		r.db.QueryRow(`SELECT filename FROM photos WHERE id = 'a'`).Scan(&name) //nolint:errcheck
		done <- name
	}()
	select {
	case name := <-done:
		if name != "a.jpg" {
			t.Errorf("the reader saw %q, want what was there before the write", name)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("the reader waited for the writer")
	}
	if _, err := r.db.Exec(`UPDATE photos SET filename = 'x'`); err == nil {
		t.Error("a reader wrote")
	}
}
