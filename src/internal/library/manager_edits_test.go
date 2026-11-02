package library

import (
	"path/filepath"
	"slices"
	"testing"

	"huepattl.de/unterlumen/internal/metadata"
)

// A value written beside a photo and indexed at once reaches the filter's
// values and the statistics, though no later scan finds a change — the
// country written in Metadata stayed missing from the filter.
func TestIndexedEditsRefreshWhatWasWorkedOut(t *testing.T) {
	mgr, err := NewManager(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	src := t.TempDir()
	lib, err := mgr.CreateLibrary("Edits", "", src)
	if err != nil {
		t.Fatal(err)
	}
	photo := filepath.Join(src, "a.jpg")
	writeIndexJPEG(t, photo)
	mgr.IndexFilesSync(lib.ID, []string{photo})
	if vals, _ := mgr.AggregateExifFieldValues(nil, "City"); len(vals) != 0 {
		t.Fatalf("a city before any was written: %v", vals)
	}
	if st, _ := mgr.Statistics(nil, "", Filter{}); len(st.Cities) != 0 {
		t.Fatalf("cities before any was written: %v", st.Cities)
	}
	plan := metadata.Plan{Set: map[metadata.Field]string{metadata.City: "Kiel"}}
	if err := metadata.Write(photo, metadata.TargetSidecar, plan); err != nil {
		t.Fatal(err)
	}
	mgr.MetadataWritten(photo, nil)
	if vals, _ := mgr.AggregateExifFieldValues(nil, "City"); !slices.Equal(vals, []string{"Kiel"}) {
		t.Errorf("the filter's cities after writing: %v", vals)
	}
	if st, _ := mgr.Statistics(nil, "", Filter{}); len(st.Cities) != 1 {
		t.Errorf("the statistics' cities after writing: %v", st.Cities)
	}
}
