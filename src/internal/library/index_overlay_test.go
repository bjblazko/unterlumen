package library

import (
	"os"
	"path/filepath"
	"testing"

	"huepattl.de/unterlumen/internal/media"
	"huepattl.de/unterlumen/internal/metadata"
)

// A date written into the sidecar of an unchanged photo reaches the index on
// the next look at the file, and goes again when the sidecar loses it
// (ADR-0060).
func TestSidecarDateReachesTheIndex(t *testing.T) {
	src := t.TempDir()
	path := filepath.Join(src, "a.jpg")
	id := writeIndexJPEG(t, path)
	idx, s := newTestIndexer(t, src)
	if err := idx.IndexFile(path); err != nil {
		t.Fatal(err)
	}
	if r := readPhotoRow(t, s, id); r.date.Valid {
		t.Fatalf("a date before any was written: %v", r.date)
	}

	plan := metadata.Plan{Set: map[metadata.Field]string{metadata.DateTaken: "2024-05-01T12:30:00"}}
	if err := metadata.Write(path, metadata.TargetSidecar, plan, metadata.Change{}); err != nil {
		t.Fatal(err)
	}
	if err := idx.IndexFile(path); err != nil {
		t.Fatal(err)
	}
	if r := readPhotoRow(t, s, id); r.date.String != "2024-05-01T12:30:00" {
		t.Errorf("date after writing the sidecar = %v", r.date)
	}

	if err := os.Remove(media.SidecarPath(path)); err != nil {
		t.Fatal(err)
	}
	if err := idx.IndexFile(path); err != nil {
		t.Fatal(err)
	}
	if r := readPhotoRow(t, s, id); r.date.Valid {
		t.Errorf("the sidecar's date stayed after the sidecar went: %v", r.date)
	}
}
