package library

import (
	"errors"
	"image/color"
	"path/filepath"
	"testing"

	"huepattl.de/unterlumen/internal/faces"
	"huepattl.de/unterlumen/internal/media"
)

// onePhotoWithAFace stores a1photo with one found face at x 0.1, unnamed.
func onePhotoWithAFace(t *testing.T, s *Store, dir string) {
	t.Helper()
	addPhotoWithThumb(t, s, dir, "a1photo", filepath.Join(dir, "a1photo.jpg"), color.RGBA{})
	if err := s.SaveFoundFaces([]FoundFaces{{PhotoID: "a1photo", Faces: []faces.Face{face(0.1, 0)}, Names: []string{""}}}); err != nil {
		t.Fatal(err)
	}
}

func photoFaces(t *testing.T, s *Store) []PhotoFace {
	t.Helper()
	f, err := s.PhotoFaces("a1photo")
	if err != nil {
		t.Fatal(err)
	}
	return f
}

var benInProfile = media.FaceRegion{Name: "Ben", X: 0.6, Y: 0.2, W: 0.1, H: 0.15}

func TestAMarkedFaceIsSomeoneUntilTheirNameGoes(t *testing.T) {
	s := newTestStore(t)
	dir := t.TempDir()
	onePhotoWithAFace(t, s, dir)

	if skipped, err := s.MarkFace("a1photo", benInProfile); err != nil || skipped != 0 {
		t.Fatalf("skipped %d, err %v", skipped, err)
	}
	got := photoFaces(t, s)
	if len(got) != 2 || got[1].Name != "Ben" || !got[1].Manual || got[0].Manual {
		t.Fatalf("faces %+v, want the found one and Ben marked by hand", got)
	}
	regions, _, _ := media.ReadFaceRegions(filepath.Join(dir, "a1photo.jpg"), 1)
	if len(regions) != 1 || regions[0].Name != "Ben" {
		t.Fatalf("sidecar regions %+v, want Ben", regions)
	}
	if err := s.Regroup(); err != nil {
		t.Fatalf("grouping beside a face without a print: %v", err)
	}
	if prints, _ := s.FacePrints(""); len(prints) != 1 {
		t.Errorf("%d face prints, want only the found face's", len(prints))
	}

	if _, err := s.MarkFace("a1photo", media.FaceRegion{Name: "Eva", X: 0.11, Y: 0.21, W: 0.1, H: 0.1}); !errors.Is(err, ErrFaceAlreadyThere) {
		t.Errorf("marking a found face again: %v", err)
	}
	if _, err := s.MarkFace("a1photo", media.FaceRegion{Name: "Eva", X: 0.95, Y: 0.2, W: 0.1, H: 0.1}); !errors.Is(err, ErrBadFaceBox) {
		t.Errorf("marking outside the photo: %v", err)
	}

	if _, err := s.UnnameFaces([]int64{got[1].ID}); err != nil {
		t.Fatal(err)
	}
	if got := photoFaces(t, s); len(got) != 1 {
		t.Fatalf("faces %+v, want Ben's frame gone with his name", got)
	}
	if people, _ := s.People(); len(people) != 0 {
		t.Errorf("people %+v, want nobody", people)
	}
	if regions, _, _ := media.ReadFaceRegions(filepath.Join(dir, "a1photo.jpg"), 1); len(regions) != 0 {
		t.Errorf("sidecar regions %+v, want none", regions)
	}
}

// The other installation misses the face too: it learns of it from the
// sidecar, renamed and removed as there.
func TestAMarkedFaceReachesTheOtherInstallation(t *testing.T) {
	dir := t.TempDir()
	mac, nas := newTestStore(t), newTestStore(t)
	onePhotoWithAFace(t, mac, dir)
	onePhotoWithAFace(t, nas, dir)
	read := func(st *Store) []PhotoFace {
		regions, foreign, _ := media.ReadFaceRegions(filepath.Join(dir, "a1photo.jpg"), 1)
		if err := st.ApplyFaceNames("a1photo", regions, foreign); err != nil {
			t.Fatal(err)
		}
		return photoFaces(t, st)
	}

	mac.MarkFace("a1photo", benInProfile) //nolint:errcheck
	if got := read(nas); len(got) != 2 || got[1].Name != "Ben" || !got[1].Manual {
		t.Fatalf("the NAS has %+v, want Ben marked", got)
	}
	if got := read(nas); len(got) != 2 {
		t.Fatalf("a second scan changed the faces: %+v", got)
	}

	people, _ := mac.People()
	mac.RenamePerson(people[0].ID, "Benjamin") //nolint:errcheck
	if got := read(nas); len(got) != 2 || got[1].Name != "Benjamin" {
		t.Fatalf("after the rename the NAS has %+v", got)
	}

	ben := photoFaces(t, mac)[1]
	mac.UnnameFaces([]int64{ben.ID}) //nolint:errcheck
	if got := read(nas); len(got) != 1 {
		t.Fatalf("after the Mac removed the frame the NAS has %+v", got)
	}
	if p, _ := nas.People(); len(p) != 0 {
		t.Errorf("the NAS still knows %+v", p)
	}
}

// A new pass keeps faces marked by hand; one it now finds itself takes the
// name, and the frame drawn by hand goes.
func TestAPassKeepsMarkedFacesAndTakesOverOnesItFinds(t *testing.T) {
	s := newTestStore(t)
	dir := t.TempDir()
	onePhotoWithAFace(t, s, dir)
	s.MarkFace("a1photo", benInProfile) //nolint:errcheck

	again := func(found ...faces.Face) []PhotoFace {
		names := make([]string, len(found))
		if err := s.SaveFoundFaces([]FoundFaces{{PhotoID: "a1photo", Faces: found, Names: names}}); err != nil {
			t.Fatal(err)
		}
		return photoFaces(t, s)
	}
	if got := again(face(0.1, 0)); len(got) != 2 || !got[1].Manual || got[1].Name != "Ben" {
		t.Fatalf("after a pass: %+v, want Ben still marked", got)
	}
	better := faces.Face{X: 0.61, Y: 0.21, W: 0.1, H: 0.14, Score: 0.6, Embedding: direction(1)}
	got := again(face(0.1, 0), better)
	if len(got) != 2 || got[1].Manual || got[1].Name != "Ben" {
		t.Fatalf("after a pass that finds Ben: %+v, want his found face named", got)
	}
}

// A frame drawn over a face that was hidden names that face: it was found
// after all, only hidden.
func TestAFrameOverAHiddenFaceNamesIt(t *testing.T) {
	s := newTestStore(t)
	onePhotoWithAFace(t, s, t.TempDir())
	found := photoFaces(t, s)[0]
	s.HideFaces([]int64{found.ID}) //nolint:errcheck
	if _, err := s.MarkFace("a1photo", media.FaceRegion{Name: "Eva", X: 0.11, Y: 0.21, W: 0.1, H: 0.1}); err != nil {
		t.Fatal(err)
	}
	got := photoFaces(t, s)
	if len(got) != 1 || got[0].ID != found.ID || got[0].Name != "Eva" || got[0].Manual {
		t.Fatalf("faces %+v, want the hidden face named Eva", got)
	}
}
