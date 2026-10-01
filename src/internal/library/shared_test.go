package library

import (
	"os"
	"path/filepath"
	"testing"
)

func newTestManager(t *testing.T) *Manager {
	t.Helper()
	mgr, err := NewManager(t.TempDir())
	if err != nil {
		t.Fatalf("NewManager: %v", err)
	}
	return mgr
}

func TestShareWritesAndUnshareRemovesTheMarker(t *testing.T) {
	mgr := newTestManager(t)
	base := t.TempDir()
	l, err := mgr.CreateLibrary("Reisen", "Trips", base)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := mgr.ShareLibrary(l.ID); err != nil {
		t.Fatalf("ShareLibrary: %v", err)
	}
	mk, err := ReadMarker(base)
	if err != nil || mk == nil {
		t.Fatalf("ReadMarker = %v, %v; want the marker", mk, err)
	}
	if mk.ID != l.ID || mk.Name != "Reisen" || mk.Description != "Trips" {
		t.Errorf("marker = %+v", mk)
	}
	if _, err := mgr.UnshareLibrary(l.ID); err != nil {
		t.Fatalf("UnshareLibrary: %v", err)
	}
	if _, err := os.Stat(filepath.Join(base, MarkerName)); !os.IsNotExist(err) {
		t.Errorf("marker still there after unsharing: %v", err)
	}
}

func TestRenameOfASharedLibraryReachesTheOtherInstallation(t *testing.T) {
	nas, mac := newTestManager(t), newTestManager(t)
	base := t.TempDir()
	l, _ := nas.CreateLibrary("Reisen", "", base)
	nas.ShareLibrary(l.ID) //nolint:errcheck
	added, err := mac.AddSharedLibrary(base)
	if err != nil {
		t.Fatalf("AddSharedLibrary: %v", err)
	}
	if added.ID != l.ID || added.Name != "Reisen" {
		t.Fatalf("added = %+v, want the NAS's ID and name", added)
	}

	if _, err := nas.UpdateLibrary(l.ID, "Travel", "far away"); err != nil {
		t.Fatal(err)
	}
	seen, _ := mac.GetLibrary(l.ID)
	mac.Annotate(seen)
	if !seen.Shared || seen.Name != "Travel" || seen.Description != "far away" {
		t.Errorf("mac sees %+v, want shared Travel/far away", seen)
	}
	if stored, _ := mac.GetLibrary(l.ID); stored.Name != "Travel" {
		t.Errorf("the new name was not kept: %q", stored.Name)
	}
}

func TestJoinSharedKeepsTheIndexUnderTheSharedID(t *testing.T) {
	nas, mac := newTestManager(t), newTestManager(t)
	base := t.TempDir()
	shared, _ := nas.CreateLibrary("Reisen", "", base)
	own, _ := mac.CreateLibrary("Trips", "", base)
	if err := os.WriteFile(filepath.Join(mac.ThumbDir(own.ID), "x.jpg"), []byte("thumb"), 0o644); err != nil {
		t.Fatal(err)
	}
	store, _ := mac.OpenStore(own.ID)
	store.SetProp("photo_count", "7") //nolint:errcheck
	nas.ShareLibrary(shared.ID)       //nolint:errcheck

	seen, _ := mac.GetLibrary(own.ID)
	mac.Annotate(seen)
	if seen.JoinOffer == nil || seen.JoinOffer.ID != shared.ID {
		t.Fatalf("JoinOffer = %+v, want the NAS's library", seen.JoinOffer)
	}

	joined, err := mac.JoinShared(own.ID)
	if err != nil {
		t.Fatalf("JoinShared: %v", err)
	}
	if joined.ID != shared.ID || joined.Name != "Reisen" || !joined.Shared {
		t.Errorf("joined = %+v", joined)
	}
	if joined.PhotoCount != 7 {
		t.Errorf("PhotoCount = %d, want the old index's 7", joined.PhotoCount)
	}
	if _, err := os.Stat(filepath.Join(mac.ThumbDir(shared.ID), "x.jpg")); err != nil {
		t.Errorf("thumbnail did not move: %v", err)
	}
	if _, err := mac.GetLibrary(own.ID); err == nil {
		t.Error("the old ID is still a library")
	}
}

func TestShareRefusesAFolderSharedAsAnotherLibrary(t *testing.T) {
	nas, mac := newTestManager(t), newTestManager(t)
	base := t.TempDir()
	shared, _ := nas.CreateLibrary("Reisen", "", base)
	nas.ShareLibrary(shared.ID) //nolint:errcheck
	own, _ := mac.CreateLibrary("Trips", "", base)
	if _, err := mac.ShareLibrary(own.ID); err == nil {
		t.Error("ShareLibrary overwrote another installation's marker")
	}
	mac.UnshareLibrary(own.ID) //nolint:errcheck
	if mk, _ := ReadMarker(base); mk == nil || mk.ID != shared.ID {
		t.Errorf("UnshareLibrary removed another library's marker: %+v", mk)
	}
}

func TestReadMarkerRejectsAnIDThatIsNoLibraryID(t *testing.T) {
	base := t.TempDir()
	os.WriteFile(filepath.Join(base, MarkerName), []byte(`{"id":"../../etc","name":"x"}`), 0o644) //nolint:errcheck
	if _, err := ReadMarker(base); err == nil {
		t.Error("a marker naming a path was accepted")
	}
}

func TestAnnotateReportsAMissingBaseFolder(t *testing.T) {
	mgr := newTestManager(t)
	l, _ := mgr.CreateLibrary("Gone", "", filepath.Join(t.TempDir(), "not-mounted"))
	mgr.Annotate(l)
	if !l.Missing {
		t.Error("Missing = false for a base folder that does not exist")
	}
}
