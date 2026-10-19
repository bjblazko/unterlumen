package library

import (
	"image/color"
	"os"
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
	if skipped, err := s.NameFaces([]int64{refs[0].ID}, "Carla"); err != nil || skipped != 0 {
		t.Fatalf("skipped %d, err %v", skipped, err)
	}
	regions, foreign, err := media.ReadFaceRegions(filepath.Join(dir, "a1photo.jpg"), 1)
	if err != nil || foreign || len(regions) != 1 || regions[0].Name != "Carla" {
		t.Fatalf("regions %+v, foreign %v, err %v", regions, foreign, err)
	}
	// The other installation found the same face and reads the name.
	other := newTestStore(t)
	photosWithFaces(t, other, dir, 0)
	if err := other.ApplyFaceNames("a1photo", regions, foreign); err != nil {
		t.Fatal(err)
	}
	if people, _ := other.People(); len(people) != 1 || people[0].Name != "Carla" {
		t.Fatalf("the other installation has %+v", people)
	}
}

// Two installations name faces in the same photo: a name taken away on one
// leaves the other at its next scan, a rename arrives as a rename, and a
// name written on one while the other names another face is kept by both.
func TestNamesFollowTheSidecarBetweenInstallations(t *testing.T) {
	dir := t.TempDir()
	two := func() *Store {
		st := newTestStore(t)
		addPhotoWithThumb(t, st, dir, "a1photo", filepath.Join(dir, "a1photo.jpg"), color.RGBA{})
		st.SaveFoundFaces([]FoundFaces{{PhotoID: "a1photo", Faces: []faces.Face{face(0.1, 0), face(0.6, 1)}, Names: []string{"", ""}}}) //nolint:errcheck
		return st
	}
	mac, nas := two(), two()
	faceAt := func(st *Store, x float64) int64 {
		ids, boxes, _ := st.facesOf("a1photo")
		for i, b := range boxes {
			if b.X == x {
				return ids[i]
			}
		}
		t.Fatalf("no face at %v", x)
		return 0
	}
	read := func(st *Store) {
		regions, foreign, _ := media.ReadFaceRegions(filepath.Join(dir, "a1photo.jpg"), 1)
		if err := st.ApplyFaceNames("a1photo", regions, foreign); err != nil {
			t.Fatal(err)
		}
	}
	names := func(st *Store) map[string]bool {
		got := map[string]bool{}
		people, _ := st.People()
		for _, p := range people {
			got[p.Name] = true
		}
		return got
	}

	mac.NameFaces([]int64{faceAt(mac, 0.1)}, "Anna") //nolint:errcheck
	nas.NameFaces([]int64{faceAt(nas, 0.6)}, "Ben")  //nolint:errcheck — before the NAS read Anna
	read(mac)
	read(nas)
	if !names(mac)["Anna"] || !names(mac)["Ben"] || !names(nas)["Anna"] || !names(nas)["Ben"] {
		t.Fatalf("a name was lost: mac %v, nas %v", names(mac), names(nas))
	}

	nas.UnnameFaces([]int64{faceAt(nas, 0.1)}) //nolint:errcheck — "This is not Anna"
	people, _ := nas.People()
	var ben int64
	for _, p := range people {
		if p.Name == "Ben" {
			ben = p.ID
		}
	}
	nas.RenamePerson(ben, "Benjamin") //nolint:errcheck
	read(mac)
	if got := names(mac); got["Anna"] || got["Ben"] || !got["Benjamin"] {
		t.Fatalf("after the NAS took Anna away and renamed Ben, the Mac has %v", got)
	}
	if p, _ := nas.People(); len(p) != 1 || p[0].ID != ben {
		t.Errorf("renaming gave the person another ID: %+v", p)
	}
}

// A sidecar whose regions Lightroom keeps cannot hold Unterlumen's names, so
// a name missing from it is not taken away; hidden faces are this
// installation's own decision and never reach a sidecar.
func TestNamesStayWhereTheSidecarCannotSayThem(t *testing.T) {
	dir := t.TempDir()
	s := newTestStore(t)
	photosWithFaces(t, s, dir, 0, 0)
	refs, _ := s.GroupFaces(groups(t, s)[0].ID)
	photo := filepath.Join(dir, "a1photo.jpg")
	os.WriteFile(media.SidecarPath(photo), []byte(lightroomSidecar), 0o644) //nolint:errcheck
	var inA1 int64
	for _, r := range refs {
		if r.PhotoID == "a1photo" {
			inA1 = r.ID
		}
	}
	if skipped, err := s.NameFaces([]int64{inA1}, "Eva"); err != nil || skipped != 1 {
		t.Fatalf("naming beside Lightroom's regions: skipped %d, err %v", skipped, err)
	}
	regions, foreign, _ := media.ReadFaceRegions(photo, 1)
	s.ApplyFaceNames("a1photo", regions, foreign) //nolint:errcheck
	if people, _ := s.People(); len(people) != 1 || people[0].Name != "Eva" {
		t.Fatalf("the name kept in the index was taken away: %+v", people)
	}

	for _, r := range refs {
		if r.PhotoID == "b1photo" {
			s.HideFaces([]int64{r.ID}) //nolint:errcheck
		}
	}
	if _, err := os.Stat(media.SidecarPath(filepath.Join(dir, "b1photo.jpg"))); err == nil {
		t.Error("hiding a face wrote a sidecar")
	}
	s.ApplyFaceNames("b1photo", nil, false) //nolint:errcheck
	if g := groups(t, s); len(g) != 0 {
		t.Errorf("a scan brought the hidden face back: %+v", g)
	}
}

const lightroomSidecar = `<x:xmpmeta xmlns:x="adobe:ns:meta/">
 <rdf:RDF xmlns:rdf="http://www.w3.org/1999/02/22-rdf-syntax-ns#">
  <rdf:Description rdf:about="" xmlns:mwg-rs="http://www.metadataworkinggroup.com/schemas/regions/"
    xmlns:stArea="http://ns.adobe.com/xmp/sType/Area#">
   <mwg-rs:Regions rdf:parseType="Resource"><mwg-rs:RegionList><rdf:Bag>
    <rdf:li><rdf:Description mwg-rs:Name="Dora" mwg-rs:Type="Face">
     <mwg-rs:Area stArea:x="0.8" stArea:y="0.8" stArea:w="0.1" stArea:h="0.1" stArea:unit="normalized"/>
    </rdf:Description></rdf:li>
   </rdf:Bag></mwg-rs:RegionList></mwg-rs:Regions>
  </rdf:Description>
 </rdf:RDF>
</x:xmpmeta>`

func TestNamesFromRegionsNeedEnoughOverlap(t *testing.T) {
	found := []faces.Face{face(0.1, 0), face(0.6, 0)}
	regions := []media.FaceRegion{{Name: "Dora", X: 0.11, Y: 0.21, W: 0.1, H: 0.1}, {Name: "Eva", X: 0.3, Y: 0.6, W: 0.1, H: 0.1}}
	got := namesFromRegions(found, regions)
	if got[0] != "Dora" || got[1] != "" {
		t.Fatalf("got %q", got)
	}
}
