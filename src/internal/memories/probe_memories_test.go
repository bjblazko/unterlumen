//go:build probe

package memories

// Times Memories (ADR-0064) on copies of real libraries: a folder laid out
// as a data folder, each libraries/<id>/library.db copied with sqlite3
// .backup and its source_path pointed at nothing, so nothing is scanned:
//
//	PROBE_LIB_DIR=/path/to/copy go test -tags probe -run ProbeMemories -v ./internal/memories/

import (
	"context"
	"os"
	"testing"
	"time"

	"huepattl.de/unterlumen/internal/library"
)

func TestProbeMemories(t *testing.T) {
	dir := os.Getenv("PROBE_LIB_DIR")
	if dir == "" {
		t.Skip("set PROBE_LIB_DIR to a copy of a data folder")
	}
	mgr, err := library.NewManager(dir)
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()
	timed := func(name string, f func()) {
		start := time.Now()
		f()
		t.Logf("%-24s %6d ms", name, time.Since(start).Milliseconds())
	}
	// Opening a library migrates it and starts its readers; timed apart, as
	// the app has done that long before Memories is opened.
	timed("open libraries", func() {
		libs, _ := mgr.ListLibraries()
		for _, l := range libs {
			if s, err := mgr.OpenReader(l.ID); err == nil {
				s.Close()
			}
		}
	})
	var photos []library.MemoryPhoto
	timed("photos, cold", func() {
		if photos, _, err = mgr.MemoryPhotos(ctx, nil, library.Filter{}); err != nil {
			t.Fatal(err)
		}
	})
	timed("photos, cached", func() { mgr.MemoryPhotos(ctx, nil, library.Filter{}) })
	t.Logf("%d photos", len(photos))
	day := time.Date(2026, 11, 3, 0, 0, 0, 0, time.UTC)
	var o Overview
	timed("build overview", func() { o = Build(day, photos, nil) })
	for _, y := range o.Years {
		t.Logf("  %s: %d photos, %d picks", y.Date, y.Total, len(y.Picks))
	}
	for _, c := range o.Cards {
		t.Logf("  %s %s: %d photos", c.Kind, c.Key, c.Total)
		timed("  detail "+c.Kind, func() { BuildDetail(day, c.Kind, c.Key, photos, nil) })
	}
}
