package library

import (
	"context"
	"database/sql"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"
)

// A run that stops early — the app quit, a deploy restarted it — must not
// leave photos marked missing. Both the full re-index and Remove deleted
// photos used to mark first and delete at the end, so an interrupted run left
// photos at status 'missing' for good, and the statistics counted them as
// "still being read" from then on.

// cancelAfterFirst runs a job and cancels it once it has reported progress.
func cancelAfterFirst(run func(context.Context, chan<- Progress)) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	ch := make(chan Progress)
	go run(ctx, ch)
	first := true
	for range ch {
		if first {
			cancel()
			first = false
		}
	}
}

// statusOf is the photo's status, or "removed" when it is no longer there.
func statusOf(t *testing.T, s *Store, id string) string {
	t.Helper()
	var status string
	err := s.db.QueryRow(`SELECT status FROM photos WHERE id=?`, id).Scan(&status)
	if errors.Is(err, sql.ErrNoRows) {
		return "removed"
	}
	if err != nil {
		t.Fatal(err)
	}
	return status
}

func TestInterruptedReindexLeavesNoPhotoMissing(t *testing.T) {
	dir := t.TempDir()
	idx, s := newTestIndexer(t, dir)
	for _, name := range []string{"a.jpg", "b.jpg"} {
		path := filepath.Join(dir, name)
		os.WriteFile(path, []byte("jpeg "+name), 0o644) //nolint:errcheck
		if err := s.UpsertPhoto(name, path, name, 1, time.Now(), "{}", "", "", "jpeg"); err != nil {
			t.Fatal(err)
		}
	}
	cancelAfterFirst(idx.Run)
	for _, id := range []string{"a.jpg", "b.jpg"} {
		if got := statusOf(t, s, id); got != "ok" {
			t.Errorf("%s is %q after an interrupted re-index, want ok: its file is there", id, got)
		}
	}
}

func TestInterruptedCleanupLeavesNoPhotoMissing(t *testing.T) {
	dir := t.TempDir()
	idx, s := newTestIndexer(t, dir)
	for _, name := range []string{"gone-1.jpg", "gone-2.jpg"} {
		if err := s.UpsertPhoto(name, filepath.Join(dir, name), name, 1, time.Now(), "{}", "", "", "jpeg"); err != nil {
			t.Fatal(err)
		}
	}
	cancelAfterFirst(idx.RunCleanup)
	for _, id := range []string{"gone-1.jpg", "gone-2.jpg"} {
		if got := statusOf(t, s, id); got == "missing" {
			t.Errorf("%s was left missing by an interrupted cleanup; it must be removed or left as it was", id)
		}
	}
}

func TestCleanupRemovesPhotosWhoseFilesAreGone(t *testing.T) {
	dir := t.TempDir()
	idx, s := newTestIndexer(t, dir)
	kept := filepath.Join(dir, "kept.jpg")
	os.WriteFile(kept, []byte("jpeg"), 0o644)                                                              //nolint:errcheck
	s.UpsertPhoto("kept", kept, "kept.jpg", 1, time.Now(), "{}", "", "", "jpeg")                           //nolint:errcheck
	s.UpsertPhoto("gone", filepath.Join(dir, "gone.jpg"), "gone.jpg", 1, time.Now(), "{}", "", "", "jpeg") //nolint:errcheck
	ch := make(chan Progress)
	go idx.RunCleanup(context.Background(), ch)
	for range ch {
	}
	if n, _ := s.CountPhotos(); n != 1 || statusOf(t, s, "kept") != "ok" {
		t.Errorf("after cleanup %d photos, kept is %q; want only kept, ok", n, statusOf(t, s, "kept"))
	}
}

// Photos an older version left marked missing: the ones whose files are
// there are found again, only the gone ones are removed.
func TestCleanupSortsOutPhotosLeftMarkedMissing(t *testing.T) {
	dir := t.TempDir()
	idx, s := newTestIndexer(t, dir)
	here := filepath.Join(dir, "here.jpg")
	os.WriteFile(here, []byte("jpeg"), 0o644) //nolint:errcheck
	for id, path := range map[string]string{"here": here, "gone": filepath.Join(dir, "gone.jpg")} {
		s.UpsertPhoto(id, path, id+".jpg", 1, time.Now(), "{}", "", "", "jpeg") //nolint:errcheck
		s.MarkPhotoMissing(id)                                                  //nolint:errcheck
	}
	ch := make(chan Progress)
	go idx.RunCleanup(context.Background(), ch)
	for range ch {
	}
	if got := statusOf(t, s, "here"); got != "ok" {
		t.Errorf("a photo whose file is there is %q after cleanup, want ok", got)
	}
	if got := statusOf(t, s, "gone"); got != "removed" {
		t.Errorf("a photo whose file is gone is %q after cleanup, want removed", got)
	}
}
