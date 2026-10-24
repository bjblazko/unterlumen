package library

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"huepattl.de/unterlumen/internal/jobs"
)

// A rename or move keeps a file's content, so the library keeps the photo it
// knew and only its path changes — without reading the file again, which on
// a network share took seconds per photo. The renamed file is made
// unreadable to show it is not read.
func TestAMovedFileKeepsItsPhotoWithoutBeingRead(t *testing.T) {
	mgr := newTestManager(t)
	root := t.TempDir()
	lib, err := mgr.CreateLibrary("Fotos", "", root)
	if err != nil {
		t.Fatal(err)
	}
	oldPath := filepath.Join(root, "a", "old.jpg")
	writePhoto(t, oldPath, "the same content", day)
	if !mgr.IndexFilesSync(lib.ID, []string{oldPath}) {
		t.Fatal("first index reported nothing")
	}
	store, _ := mgr.OpenStore(lib.ID)
	before := onlyID(t, store)

	newPath := filepath.Join(root, "b", "new.jpg")
	os.MkdirAll(filepath.Dir(newPath), 0o755) //nolint:errcheck
	if err := os.Rename(oldPath, newPath); err != nil {
		t.Fatal(err)
	}
	os.Chmod(newPath, 0)                           //nolint:errcheck
	t.Cleanup(func() { os.Chmod(newPath, 0o644) }) //nolint:errcheck

	if !mgr.IndexMovedFiles(lib.ID, [][2]string{{oldPath, newPath}}) {
		t.Fatal("IndexMovedFiles reported nothing")
	}
	ids := photoIDs(t, store)
	if len(ids) != 1 || ids[before] != "new.jpg" {
		t.Fatalf("want photo %s at new.jpg, have %v", before, ids)
	}
	if id, _, _, found, _ := store.GetPathCache(newPath); !found || id != before {
		t.Errorf("path cache at the new path: %q, %v", id, found)
	}
	if _, _, _, found, _ := store.GetPathCache(oldPath); found {
		t.Error("the old path is still known")
	}
}

// Files moved in two requests go into the index in the background, under
// one job whose count grows with them, and the moved photo keeps its ID: its
// old path is forgotten only once the new one is in.
func TestMovedFilesArriveUnderOneJob(t *testing.T) {
	mgr := newTestManager(t)
	reg := jobs.NewRegistry()
	mgr.SetJobs(reg)
	root := t.TempDir()
	lib, _ := mgr.CreateLibrary("Fotos", "", root)
	oldPath := filepath.Join(root, "a", "old.jpg")
	writePhoto(t, oldPath, "the same content", day)
	mgr.IndexFilesSync(lib.ID, []string{oldPath})
	store, _ := mgr.OpenStore(lib.ID)
	before := onlyID(t, store)

	newPath := filepath.Join(root, "b", "new.jpg")
	os.MkdirAll(filepath.Dir(newPath), 0o755) //nolint:errcheck
	os.Rename(oldPath, newPath)               //nolint:errcheck
	copied := filepath.Join(root, "b", "other.jpg")
	writePhoto(t, copied, "other content", day)

	first := mgr.AddMovedFiles(lib.ID, [][2]string{{oldPath, newPath}})
	second := mgr.AddCopiedFiles(lib.ID, []string{copied})
	job := waitForJob(t, reg, first)
	if second != first && second != "" {
		if other := waitForJob(t, reg, second); other.Total+job.Total != 2 {
			t.Fatalf("two jobs that do not add up: %+v, %+v", job, other)
		}
	} else if job.Total != 2 || job.Done != 2 {
		t.Fatalf("one job should count both files: %+v", job)
	}
	ids := photoIDs(t, store)
	if len(ids) != 2 || ids[before] != "new.jpg" {
		t.Fatalf("want photo %s at new.jpg and one more, have %v", before, ids)
	}
}

func waitForJob(t *testing.T, reg *jobs.Registry, id string) jobs.Job {
	t.Helper()
	deadline := time.Now().Add(10 * time.Second)
	for time.Now().Before(deadline) {
		for _, j := range reg.Snapshot() {
			if j.ID == id && j.Finished {
				return j
			}
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatalf("job %q did not finish", id)
	return jobs.Job{}
}
