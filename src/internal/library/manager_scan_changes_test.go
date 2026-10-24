package library

import (
	"context"
	"path/filepath"
	"testing"
	"time"
)

// A check that changes nothing keeps what the statistics worked out: every
// shared library is checked every 30 minutes, and each check used to have
// the statistics, the timeline and the filter's values worked out again.
func TestACheckThatChangesNothingKeepsTheCaches(t *testing.T) {
	src := t.TempDir()
	writePhoto(t, filepath.Join(src, "IMG_1.JPG"), "pixels", day)
	m, err := NewManager(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	lib, err := m.CreateLibrary("Checked", "", src)
	if err != nil {
		t.Fatal(err)
	}
	check := func() {
		t.Helper()
		b, ok := m.StartScan(lib.ID, "Checking")
		if !ok {
			t.Fatal("the scan did not start")
		}
		store, _ := m.OpenStore(lib.ID)
		ch := make(chan Progress, 16)
		go NewIndexer(store, m.LibDir(lib.ID), src).RunScanNew(context.Background(), ch)
		for p := range ch { // as checkLibrary does
			b.Send(p)
		}
		b.Close()
		m.EndScan(lib.ID)
	}
	check()
	check() // takes the sidecar stamps
	m.statsCache.Store(lib.ID+"|stats", 1)
	check()
	if _, ok := m.statsCache.Load(lib.ID + "|stats"); !ok {
		t.Fatal("a check that changed nothing threw the statistics away")
	}
	writePhoto(t, filepath.Join(src, "IMG_2.JPG"), "other pixels", day.Add(time.Hour))
	check()
	if _, ok := m.statsCache.Load(lib.ID + "|stats"); ok {
		t.Fatal("a check that added a photo kept the old statistics")
	}
}
