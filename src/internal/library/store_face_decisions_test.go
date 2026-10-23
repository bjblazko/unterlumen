package library

import (
	"os"
	"testing"

	fd "huepattl.de/unterlumen/internal/facedecisions"
)

// macAndNAS are two installations of one library: their own indexes of the
// same photos, and one shared folder.
func macAndNAS(t *testing.T, axes ...int) (mac, nas *Store) {
	t.Helper()
	dir, shared := t.TempDir(), fd.New(t.TempDir())
	mac, nas = newTestStore(t), newTestStore(t)
	photosWithFaces(t, mac, dir, axes...)
	photosWithFaces(t, nas, dir, axes...)
	mac.decisions, nas.decisions = shared, shared
	return mac, nas
}

func hiddenCount(t *testing.T, s *Store) int {
	t.Helper()
	n, err := s.HiddenCount()
	if err != nil {
		t.Fatal(err)
	}
	return n
}

func TestHidingFollowsToTheOtherInstallation(t *testing.T) {
	mac, nas := macAndNAS(t, 0, 1)
	stranger, _ := mac.GroupFaces(groups(t, mac)[1].ID)
	if _, err := mac.HideFaces([]int64{stranger[0].ID}); err != nil {
		t.Fatal(err)
	}
	if _, err := nas.ApplyDecisions(); err != nil {
		t.Fatal(err)
	}
	if hiddenCount(t, nas) != 1 {
		t.Fatalf("the NAS did not hide the stranger")
	}
	if err := mac.ShowAgain([]int64{stranger[0].ID}); err != nil {
		t.Fatal(err)
	}
	if regroup, _ := nas.ApplyDecisions(); !regroup || hiddenCount(t, nas) != 0 {
		t.Fatalf("shown again on the Mac, the NAS keeps it hidden (regroup %v)", regroup)
	}
}

// Decisions taken before the folder was shared go into it, not away.
func TestDecisionsFromBeforeSharingAreKept(t *testing.T) {
	mac, nas := macAndNAS(t, 0, 1)
	shared := nas.decisions
	nas.decisions = nil
	stranger, _ := nas.GroupFaces(groups(t, nas)[1].ID)
	nas.HideFaces([]int64{stranger[0].ID}) //nolint:errcheck
	nas.decisions = shared
	if _, err := nas.ApplyDecisions(); err != nil {
		t.Fatal(err)
	}
	if hiddenCount(t, nas) != 1 {
		t.Fatal("sharing showed a face hidden before it")
	}
	if _, err := mac.ApplyDecisions(); err != nil || hiddenCount(t, mac) != 1 {
		t.Fatalf("the Mac did not learn of it: %v", err)
	}
}

func TestANoAndACoverFollowToo(t *testing.T) {
	mac, nas := macAndNAS(t, 0, 0, 0)
	name := func(s *Store) (Person, []FaceRef) {
		faces, _ := s.GroupFaces(groups(t, s)[0].ID)
		s.NameFaces([]int64{faces[0].ID, faces[1].ID}, "Anna") //nolint:errcheck
		people, _ := s.People()
		return people[0], faces
	}
	macAnna, macFaces := name(mac)
	nasAnna, nasFaces := name(nas)

	if err := mac.NotThem([]int64{macFaces[2].ID}, macAnna.ID); err != nil {
		t.Fatal(err)
	}
	if err := mac.ChooseCover(macAnna.ID, macFaces[1].ID); err != nil {
		t.Fatal(err)
	}
	if _, err := nas.ApplyDecisions(); err != nil {
		t.Fatal(err)
	}
	if p := suggestionFor(t, nas, nasFaces[2].ID); p != nil {
		t.Errorf("the NAS still suggests %s after the Mac's No", p.Name)
	}
	people, _ := nas.People()
	if people[0].ID != nasAnna.ID || people[0].Cover != nasFaces[1].ID || people[0].CoverChosen == "" {
		t.Errorf("the NAS shows Anna by face %d, want %d chosen", people[0].Cover, nasFaces[1].ID)
	}
}

// A shared folder that is not reachable is not a file without decisions:
// the faces hidden stay hidden.
func TestAMissingFileShowsNothingAgain(t *testing.T) {
	_, nas := macAndNAS(t, 0, 1)
	stranger, _ := nas.GroupFaces(groups(t, nas)[1].ID)
	nas.HideFaces([]int64{stranger[0].ID}) //nolint:errcheck
	nas.ApplyDecisions()                   //nolint:errcheck
	os.Remove(nas.decisions.Path())        //nolint:errcheck // the share, not reachable
	if _, err := nas.ApplyDecisions(); err == nil {
		t.Error("a missing file was applied")
	}
	if hiddenCount(t, nas) != 1 {
		t.Error("a missing file showed the hidden face again")
	}
}

// Another shared folder gets the library's decisions too, rather than
// being taken for one without any.
func TestANewSharedFolderGetsTheDecisions(t *testing.T) {
	_, nas := macAndNAS(t, 0, 1)
	stranger, _ := nas.GroupFaces(groups(t, nas)[1].ID)
	nas.HideFaces([]int64{stranger[0].ID}) //nolint:errcheck
	nas.ApplyDecisions()                   //nolint:errcheck
	nas.decisions = fd.New(t.TempDir())    // moved to another shared folder
	if _, err := nas.ApplyDecisions(); err != nil {
		t.Fatal(err)
	}
	if d, _ := nas.decisions.Read(); len(d.Hidden) != 1 || hiddenCount(t, nas) != 1 {
		t.Fatalf("the new folder has %d hidden, the index %d", len(d.Hidden), hiddenCount(t, nas))
	}
}
