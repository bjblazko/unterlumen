package library

import "testing"

// After a scan's work the overview is waiting in the caches, and the
// topics' tallies and the filter's values with it.
func TestPrewarmLeavesTheOverviewCached(t *testing.T) {
	mgr := newTestManager(t)
	l, err := mgr.CreateLibrary("Fotos", "", t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	s, err := mgr.OpenStore(l.ID)
	if err != nil {
		t.Fatal(err)
	}
	seedStatsPhotos(t, s, []statsPhoto{{id: "a", path: "/lib/a.jpg", ext: "jpeg", date: "2024-05-01T10:00:00"}})
	mgr.prewarmStatistics(l.ID)
	// One library: all of them and it alone share one key.
	key := statsCacheKey([]string{l.ID}, Filter{}.Key())
	if _, ok := mgr.statsCache.Load(key); !ok {
		t.Errorf("no statistics cached (key %q)", key)
	}
	if _, ok := mgr.insightsCache.Load(key + "|gear"); !ok {
		t.Error("no gear tallies cached")
	}
	if _, ok := mgr.scopeValuesCache.Load(statsCacheKey([]string{l.ID}, "")); !ok {
		t.Error("the filter's values are not cached")
	}
}
