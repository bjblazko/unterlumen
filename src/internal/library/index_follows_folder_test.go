package library

import (
	"context"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"huepattl.de/unterlumen/internal/media"
)

// The index follows the folder: a file changed in place is one photo with
// its new content, a photo whose file is gone leaves, and a folder that
// cannot be read removes nothing.

func writePhoto(t *testing.T, path, content string, mtime time.Time) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.Chtimes(path, mtime, mtime); err != nil {
		t.Fatal(err)
	}
}

func drain(run func(context.Context, chan<- Progress)) Progress {
	ch := make(chan Progress, 16)
	go run(context.Background(), ch)
	var last Progress
	for p := range ch {
		last = p
	}
	return last
}

func photoIDs(t *testing.T, s *Store) map[string]string {
	t.Helper()
	rows, err := s.db.Query(`SELECT id, path_hint FROM photos`)
	if err != nil {
		t.Fatal(err)
	}
	defer rows.Close()
	ids := map[string]string{}
	for rows.Next() {
		var id, p string
		rows.Scan(&id, &p) //nolint:errcheck
		ids[id] = filepath.Base(p)
	}
	return ids
}

func onlyID(t *testing.T, s *Store) string {
	t.Helper()
	ids := photoIDs(t, s)
	if len(ids) != 1 {
		t.Fatalf("want one photo, have %v", ids)
	}
	for id := range ids {
		return id
	}
	return ""
}

var day = time.Date(2026, 10, 1, 12, 0, 0, 0, time.UTC)

// Setting a location rewrites the file, so its content — and the photo's
// ID — changes. The old row used to stay beside the new one, with the old
// coordinates, through every scan, reindex and cleanup.
func TestAFileChangedInPlaceIsOnePhotoWithItsNewContent(t *testing.T) {
	src := t.TempDir()
	photo := filepath.Join(src, "a.jpg")
	writePhoto(t, photo, "before", day)
	idx, s := newTestIndexer(t, src)
	drain(idx.RunScanNew)
	oldID := onlyID(t, s)
	s.UpsertMeta(oldID, "pending:blog", "draft1") //nolint:errcheck

	writePhoto(t, photo, "after, with a location", day.Add(time.Hour))
	drain(NewIndexer(s, s.dir, src).RunScanNew)

	newID := onlyID(t, s)
	if newID == oldID {
		t.Fatal("the photo kept the old content's ID")
	}
	if got := s.CurrentPhotoID(oldID); got != newID {
		t.Errorf("CurrentPhotoID(old) = %s, want the new ID", got)
	}
	if p, _ := s.GetPhotoPathHint(oldID); p != photo {
		t.Errorf("a draft naming the old ID finds %q, want %q", p, photo)
	}
	meta, _ := s.GetMeta(newID)
	if len(meta) != 1 || meta[0].Key != "pending:blog" {
		t.Errorf("the draft marker did not go over to the new photo: %v", meta)
	}
}

func TestAReindexAlsoReplacesAChangedFile(t *testing.T) {
	src := t.TempDir()
	photo := filepath.Join(src, "a.jpg")
	writePhoto(t, photo, "before", day)
	idx, s := newTestIndexer(t, src)
	drain(idx.RunScanNew)
	writePhoto(t, photo, "after", day.Add(time.Hour))
	drain(func(ctx context.Context, ch chan<- Progress) { NewIndexer(s, s.dir, src).RunInFolder(ctx, ch, "") })
	onlyID(t, s)
}

// A changed file whose old content is still at another path is a new photo;
// the copy stays what it was.
func TestAChangedFileLeavesItsUnchangedCopyAlone(t *testing.T) {
	src := t.TempDir()
	a, b := filepath.Join(src, "a.jpg"), filepath.Join(src, "b.jpg")
	writePhoto(t, a, "same", day)
	writePhoto(t, b, "same", day)
	idx, s := newTestIndexer(t, src)
	drain(idx.RunScanNew)
	copyID := onlyID(t, s)

	writePhoto(t, a, "edited", day.Add(time.Hour))
	drain(NewIndexer(s, s.dir, src).RunScanNew)

	ids := photoIDs(t, s)
	if len(ids) != 2 || ids[copyID] != "b.jpg" {
		t.Fatalf("photos %v, want the copy at b.jpg and the edited a.jpg", ids)
	}
	if s.CurrentPhotoID(copyID) != copyID {
		t.Error("the copy's ID was led to the edited photo")
	}
}

// Libraries scanned before the fix still hold the old rows; a full run or a
// cleanup removes them, and forgets paths no file answers to.
func TestARunRemovesRowsLeftByEarlierChanges(t *testing.T) {
	src := t.TempDir()
	photo := filepath.Join(src, "a.jpg")
	writePhoto(t, photo, "now", day)
	idx, s := newTestIndexer(t, src)
	drain(idx.RunScanNew)
	current := onlyID(t, s)
	// What an earlier version left: the old content's row at the same path,
	// and a path the file was renamed away from.
	s.UpsertPhoto("0ld", photo, "a.jpg", 3, day, "{}", "", "", "jpeg")                    //nolint:errcheck
	s.UpsertPathCache(filepath.Join(src, "renamed-away.jpg"), current, day.UnixNano(), 3) //nolint:errcheck

	if p := drain(NewIndexer(s, s.dir, src).RunCleanup); p.Error != "" {
		t.Fatal(p.Error)
	}
	if ids := photoIDs(t, s); len(ids) != 1 || ids[current] == "" {
		t.Fatalf("photos %v, want only the current one", ids)
	}
	var paths int
	s.db.QueryRow(`SELECT COUNT(*) FROM path_cache`).Scan(&paths) //nolint:errcheck
	if paths != 1 {
		t.Errorf("%d paths cached, want 1", paths)
	}
}

func TestADeletedFileStillLeaves(t *testing.T) {
	src := t.TempDir()
	keep, gone := filepath.Join(src, "keep.jpg"), filepath.Join(src, "gone.jpg")
	writePhoto(t, keep, "keep", day)
	writePhoto(t, gone, "gone", day)
	idx, s := newTestIndexer(t, src)
	drain(idx.RunScanNew)
	os.Remove(gone) //nolint:errcheck
	drain(NewIndexer(s, s.dir, src).Run)
	if ids := photoIDs(t, s); len(ids) != 1 {
		t.Fatalf("photos %v, want only keep.jpg", ids)
	}
}

// A share that is not mounted, or a folder that cannot be read, looks like
// photos that are gone. Removing them then emptied a library.
func TestAFolderThatCannotBeReadRemovesNothing(t *testing.T) {
	src := t.TempDir()
	writePhoto(t, filepath.Join(src, "top.jpg"), "top", day)
	writePhoto(t, filepath.Join(src, "locked", "inner.jpg"), "inner", day)
	idx, s := newTestIndexer(t, src)
	drain(idx.RunScanNew)

	t.Run("share not mounted", func(t *testing.T) {
		gone := filepath.Join(t.TempDir(), "not-mounted")
		for name, run := range map[string]func(context.Context, chan<- Progress){
			"index":   NewIndexer(s, s.dir, gone).Run,
			"cleanup": NewIndexer(s, s.dir, gone).RunCleanup,
		} {
			if p := drain(run); p.Error == "" {
				t.Errorf("%s: no error", name)
			}
			if n := len(photoIDs(t, s)); n != 2 {
				t.Fatalf("%s left %d photos, want 2", name, n)
			}
		}
	})

	t.Run("empty mount point", func(t *testing.T) {
		drain(NewIndexer(s, s.dir, t.TempDir()).RunCleanup)
		if n := len(photoIDs(t, s)); n != 2 {
			t.Fatalf("left %d photos, want 2", n)
		}
	})

	t.Run("unreadable subfolder", func(t *testing.T) {
		if runtime.GOOS == "windows" || os.Getuid() == 0 {
			t.Skip("permissions do not lock a folder here")
		}
		locked := filepath.Join(src, "locked")
		os.Chmod(locked, 0)                           //nolint:errcheck
		t.Cleanup(func() { os.Chmod(locked, 0o755) }) //nolint:errcheck
		writePhoto(t, filepath.Join(src, "new.jpg"), "new", day)

		p := drain(NewIndexer(s, s.dir, src).Run)
		if p.Error == "" {
			t.Error("the run did not say a folder could not be read")
		}
		if n := len(photoIDs(t, s)); n != 3 {
			t.Fatalf("%d photos, want the 2 known plus the new one", n)
		}
	})
}

// A gallery the photo was taken out of on the other installation is gone
// from its sidecar; the index used to keep claiming it.
func TestAScanDropsPublicationsTheSidecarNoLongerHas(t *testing.T) {
	idx, s := newTestIndexer(t, t.TempDir())
	path := notePhoto(t, s, "p1")
	at := time.Date(2026, 9, 1, 0, 0, 0, 0, time.UTC)
	for _, id := range []string{"a1", "b2"} {
		media.AppendPublication(path, media.Publication{Channel: "blog", PostID: id, GalleryTitle: id, PublishedAt: at}) //nolint:errcheck
		at = at.Add(time.Hour)
	}
	idx.indexSidecar(path, "p1")
	if got := metaOf(t, s, "p1"); got["built:blog:a1"] == "" || got["built:blog:postid"] != "b2" {
		t.Fatalf("publications not read: %v", got)
	}

	media.RemovePublication(path, "blog", "b2") //nolint:errcheck — on the other installation
	idx.indexSidecar(path, "p1")
	got := metaOf(t, s, "p1")
	if _, ok := got["built:blog:b2"]; ok {
		t.Errorf("the removed gallery is still claimed: %v", got)
	}
	if got["built:blog:a1"] == "" || got["built:blog:postid"] != "a1" || got["built:blog:title"] != "a1" {
		t.Errorf("the gallery left should now be the destination's latest: %v", got)
	}

	media.RemovePublication(path, "blog", "a1") //nolint:errcheck
	idx.indexSidecar(path, "p1")
	for k := range metaOf(t, s, "p1") {
		if strings.HasPrefix(k, "built:") {
			t.Errorf("unpublished everywhere, yet %s is left", k)
		}
	}
}

// A scan takes the sidecars from its walk of the folder instead of asking
// the disk for each photo's; a sidecar it passes is read, and a photo whose
// sidecar is gone loses what only that sidecar said.
func TestAScanReadsTheSidecarsItsWalkFound(t *testing.T) {
	src := t.TempDir()
	photo := filepath.Join(src, "Sub", "IMG_1.JPG")
	writePhoto(t, photo, "pixels", day)
	media.WriteField(photo, "place", "Hagen") //nolint:errcheck
	idx, s := newTestIndexer(t, src)
	s.SetProp(notesInSidecarProp, "1") //nolint:errcheck
	drain(idx.RunScanNew)
	id := onlyID(t, s)
	if got := metaOf(t, s, id); got["place"] != "Hagen" {
		t.Fatalf("the sidecar's field was not read: %v", got)
	}
	os.Remove(media.SidecarPath(photo)) //nolint:errcheck
	drain(NewIndexer(s, s.dir, src).RunScanNew)
	if got := metaOf(t, s, id); got["place"] != "" {
		t.Errorf("the field outlived its sidecar: %v", got)
	}
}

// Unterlumen knows which file it deleted: the photo leaves the library even
// when it was the last one, which a cleanup would take for a share that is
// not mounted. A photo moved within the library stays, at its new path.
func TestForgetFilesTakesOutWhatTheAppDeleted(t *testing.T) {
	src := t.TempDir()
	mgr, err := NewManager(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	l, _ := mgr.CreateLibrary("One", "", src)
	only, moved := filepath.Join(src, "only.jpg"), filepath.Join(src, "a", "moved.jpg")
	writePhoto(t, only, "only", day)
	writePhoto(t, moved, "moved", day)
	mgr.IndexFilesSync(l.ID, []string{only, moved})
	s, _ := mgr.OpenStore(l.ID)

	newPlace := filepath.Join(src, "b", "moved.jpg")
	os.MkdirAll(filepath.Dir(newPlace), 0o755) //nolint:errcheck
	os.Rename(moved, newPlace)                 //nolint:errcheck
	mgr.IndexFilesSync(l.ID, []string{newPlace})
	os.Remove(only) //nolint:errcheck
	mgr.ForgetFiles([]string{only, moved})

	ids := photoIDs(t, s)
	if len(ids) != 1 {
		t.Fatalf("photos %v, want only the moved one", ids)
	}
	for id := range ids {
		if p, _ := s.GetPhotoPathHint(id); p != newPlace {
			t.Errorf("the moved photo is at %s, want %s", p, newPlace)
		}
	}
}
