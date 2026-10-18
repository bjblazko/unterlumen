package library

import (
	"image/color"
	"path/filepath"
	"testing"

	"huepattl.de/unterlumen/internal/faces"
	"huepattl.de/unterlumen/internal/media"
)

// direction is a unit embedding along one axis: faces along the same axis
// are the same person, along different axes strangers.
func direction(axis int) []float32 {
	e := make([]float32, 128)
	e[axis] = 1
	return e
}

func face(x float64, axis int) faces.Face {
	return faces.Face{X: x, Y: 0.2, W: 0.1, H: 0.1, Score: 0.95, Embedding: direction(axis)}
}

// photosWithFaces stores photos p1…pn, each holding one face of the person
// along axis[i], and groups them.
func photosWithFaces(t *testing.T, s *Store, dir string, axes ...int) {
	t.Helper()
	var found []FoundFaces
	for i, axis := range axes {
		id := string(rune('a'+i)) + "1photo"
		addPhotoWithThumb(t, s, dir, id, filepath.Join(dir, id+".jpg"), color.RGBA{})
		found = append(found, FoundFaces{PhotoID: id, Faces: []faces.Face{face(0.1, axis)}, Names: []string{""}})
	}
	if err := s.SaveFoundFaces(found); err != nil {
		t.Fatal(err)
	}
	if err := s.Regroup(); err != nil {
		t.Fatal(err)
	}
}

func groups(t *testing.T, s *Store) []FaceGroup {
	t.Helper()
	g, err := s.FaceGroups(1, 100)
	if err != nil {
		t.Fatal(err)
	}
	return g
}

func TestFacesOfOnePersonFormOneGroup(t *testing.T) {
	s := newTestStore(t)
	photosWithFaces(t, s, t.TempDir(), 0, 0, 0, 1)
	g := groups(t, s)
	if len(g) != 2 || g[0].Size != 3 || g[1].Size != 1 {
		t.Fatalf("groups %+v, want one of 3 and one of 1", g)
	}
	if n, _ := s.PhotosNeedingFaces(); len(n) != 0 {
		t.Fatalf("%d photos still need faces", len(n))
	}
}

func TestNamingAGroupMakesAPersonAndSuggestsThemLater(t *testing.T) {
	s := newTestStore(t)
	dir := t.TempDir()
	photosWithFaces(t, s, dir, 0, 0)
	refs, _ := s.GroupFaces(groups(t, s)[0].ID)
	if _, err := s.NameFaces([]int64{refs[0].ID, refs[1].ID}, " Anna "); err != nil {
		t.Fatal(err)
	}
	people, _ := s.People()
	if len(people) != 1 || people[0].Name != "Anna" || people[0].Faces != 2 || people[0].Photos != 2 {
		t.Fatalf("people %+v", people)
	}
	if g := groups(t, s); len(g) != 0 {
		t.Fatalf("named faces are still in groups: %+v", g)
	}
	// A new photo of Anna arrives: its group asks whether it is her.
	addPhotoWithThumb(t, s, dir, "zz1photo", filepath.Join(dir, "zz.jpg"), color.RGBA{})
	s.SaveFoundFaces([]FoundFaces{{PhotoID: "zz1photo", Faces: []faces.Face{face(0.5, 0)}, Names: []string{""}}}) //nolint:errcheck
	s.Regroup()                                                                                                   //nolint:errcheck
	g := groups(t, s)
	if len(g) != 1 || g[0].Suggestion == nil || g[0].Suggestion.Name != "Anna" {
		t.Fatalf("groups %+v, want one suggesting Anna", g)
	}
}

func TestRenamingToAnExistingNameMergesThePeople(t *testing.T) {
	s := newTestStore(t)
	photosWithFaces(t, s, t.TempDir(), 0, 1)
	g := groups(t, s)
	a, _ := s.GroupFaces(g[0].ID)
	b, _ := s.GroupFaces(g[1].ID)
	s.NameFaces([]int64{a[0].ID}, "Anna") //nolint:errcheck
	s.NameFaces([]int64{b[0].ID}, "Ann")  //nolint:errcheck
	people, _ := s.People()
	var ann int64
	for _, p := range people {
		if p.Name == "Ann" {
			ann = p.ID
		}
	}
	if _, err := s.RenamePerson(ann, "anna"); err != nil {
		t.Fatal(err)
	}
	people, _ = s.People()
	if len(people) != 1 || people[0].Faces != 2 {
		t.Fatalf("people %+v, want Anna with both faces", people)
	}
	if _, err := s.RenamePerson(people[0].ID, "ANNA"); err != nil {
		t.Fatal(err)
	}
	if people, _ = s.People(); people[0].Name != "ANNA" {
		t.Fatalf("a change of case was not kept: %+v", people)
	}
}

func TestHiddenFacesLeaveGroupsAndUnnamedOnesComeBack(t *testing.T) {
	s := newTestStore(t)
	photosWithFaces(t, s, t.TempDir(), 0, 0)
	refs, _ := s.GroupFaces(groups(t, s)[0].ID)
	s.HideFaces([]int64{refs[0].ID}) //nolint:errcheck
	if g := groups(t, s); len(g) != 1 || g[0].Size != 1 {
		t.Fatalf("groups after hiding %+v", g)
	}
	s.NameFaces([]int64{refs[1].ID}, "Ben") //nolint:errcheck
	s.UnnameFaces([]int64{refs[1].ID})      //nolint:errcheck
	if people, _ := s.People(); len(people) != 0 {
		t.Fatalf("a person without faces is left: %+v", people)
	}
	if g := groups(t, s); len(g) != 1 || g[0].Faces[0] != refs[1].ID {
		t.Fatalf("the unnamed face is not back in a group: %+v", g)
	}
}

func TestPurgedPhotosTakeTheirFaces(t *testing.T) {
	s := newTestStore(t)
	photosWithFaces(t, s, t.TempDir(), 0)
	if _, err := s.PurgePhotos([]string{"a1photo"}); err != nil {
		t.Fatal(err)
	}
	var n int
	s.db.QueryRow(`SELECT COUNT(*) FROM faces`).Scan(&n) //nolint:errcheck
	if n != 0 {
		t.Fatalf("%d faces of a purged photo are left", n)
	}
}

func TestNamesGoIntoTheSidecarAndComeBackFromIt(t *testing.T) {
	s := newTestStore(t)
	dir := t.TempDir()
	photosWithFaces(t, s, dir, 0)
	refs, _ := s.GroupFaces(groups(t, s)[0].ID)
	photos, _ := s.NameFaces([]int64{refs[0].ID}, "Carla")
	if skipped, err := s.WriteFaceNames(photos); err != nil || skipped != 0 {
		t.Fatalf("skipped %d, err %v", skipped, err)
	}
	regions, err := media.ReadFaceRegions(filepath.Join(dir, "a1photo.jpg"), 1)
	if err != nil || len(regions) != 1 || regions[0].Name != "Carla" {
		t.Fatalf("regions %+v, err %v", regions, err)
	}
	// The other installation found the same face and reads the name.
	other := newTestStore(t)
	photosWithFaces(t, other, dir, 0)
	if err := other.ApplyFaceNames("a1photo", regions); err != nil {
		t.Fatal(err)
	}
	if people, _ := other.People(); len(people) != 1 || people[0].Name != "Carla" {
		t.Fatalf("the other installation has %+v", people)
	}
}

func TestNamesFromRegionsNeedEnoughOverlap(t *testing.T) {
	found := []faces.Face{face(0.1, 0), face(0.6, 0)}
	regions := []media.FaceRegion{{Name: "Dora", X: 0.11, Y: 0.21, W: 0.1, H: 0.1}, {Name: "Eva", X: 0.3, Y: 0.6, W: 0.1, H: 0.1}}
	got := namesFromRegions(found, regions)
	if got[0] != "Dora" || got[1] != "" {
		t.Fatalf("got %q", got)
	}
}
