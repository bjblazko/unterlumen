package batchrename

import "testing"

func TestApplyConflictSuffixes_NoConflict(t *testing.T) {
	mappings := []batchRenameMapping{
		{File: "a.jpg", NewName: "photo1.jpg"},
		{File: "b.jpg", NewName: "photo2.jpg"},
	}
	conflicts := applyConflictSuffixes(mappings)
	if conflicts != 0 {
		t.Errorf("expected 0 conflicts, got %d", conflicts)
	}
	if mappings[0].NewName != "photo1.jpg" || mappings[1].NewName != "photo2.jpg" {
		t.Error("names should not change when there are no conflicts")
	}
}

func TestApplyConflictSuffixes_TwoWayConflict(t *testing.T) {
	mappings := []batchRenameMapping{
		{File: "a.jpg", NewName: "photo.jpg"},
		{File: "b.jpg", NewName: "photo.jpg"},
	}
	conflicts := applyConflictSuffixes(mappings)
	if conflicts != 2 {
		t.Errorf("expected 2 conflicts, got %d", conflicts)
	}
	if mappings[0].NewName == mappings[1].NewName {
		t.Error("conflict suffixes should make names unique")
	}
}

func TestApplyConflictSuffixes_ThreeWayConflict(t *testing.T) {
	mappings := []batchRenameMapping{
		{File: "a.jpg", NewName: "photo.jpg"},
		{File: "b.jpg", NewName: "photo.jpg"},
		{File: "c.jpg", NewName: "photo.jpg"},
	}
	conflicts := applyConflictSuffixes(mappings)
	if conflicts != 3 {
		t.Errorf("expected 3 conflicts, got %d", conflicts)
	}
	seen := map[string]bool{}
	for _, m := range mappings {
		if seen[m.NewName] {
			t.Errorf("duplicate name after conflict resolution: %q", m.NewName)
		}
		seen[m.NewName] = true
	}
}

func TestApplyConflictSuffixes_SkipsErrors(t *testing.T) {
	mappings := []batchRenameMapping{
		{File: "a.jpg", NewName: "photo.jpg"},
		{File: "b.jpg", NewName: "photo.jpg", Error: "invalid path"},
	}
	conflicts := applyConflictSuffixes(mappings)
	// Only one valid mapping with this name, so no conflict
	if conflicts != 0 {
		t.Errorf("entries with errors should be skipped, got %d conflicts", conflicts)
	}
}
