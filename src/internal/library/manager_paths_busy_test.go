package library

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

// A thumbnail in Folders asks which library a photo is in. That once read
// every library's index, so a library whose one connection a scan held
// kept every thumbnail — and every request queued behind them in the
// browser — waiting until the scan ended.
func TestFindingAPhotosLibraryDoesNotWaitForABusyLibrary(t *testing.T) {
	m, err := NewManager(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	busyDir, freeDir := t.TempDir(), t.TempDir()
	busy, err := m.CreateLibrary("Busy", "", busyDir)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := m.CreateLibrary("Free", "", freeDir); err != nil {
		t.Fatal(err)
	}
	// The libraries were listed once, as the app does when it starts.
	if _, err := m.ListLibraries(); err != nil {
		t.Fatal(err)
	}
	db, err := m.getDB(busy.ID)
	if err != nil {
		t.Fatal(err)
	}
	tx, err := db.Begin() // the scan holds the busy library's one connection
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback() //nolint:errcheck
	done := make(chan struct{})
	go func() {
		m.FindThumbnailForPath(filepath.Join(freeDir, "a.jpg"))
		m.FindThumbnailForPath(filepath.Join(os.TempDir(), "elsewhere.jpg"))
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("a photo of another library, or of none, waited for the busy library")
	}
}
