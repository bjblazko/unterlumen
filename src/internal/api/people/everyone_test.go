package apipeople

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"path/filepath"
	"testing"
	"time"

	"huepattl.de/unterlumen/internal/birthdays"
	"huepattl.de/unterlumen/internal/faces"
	"huepattl.de/unterlumen/internal/jobs"
	lib "huepattl.de/unterlumen/internal/library"
)

// The name links a person across libraries, whatever its case; in a
// library without the name, someone named otherwise who looks like them, or
// a group waiting for a name, is suggested.
func TestAPersonAcrossLibraries(t *testing.T) {
	mgr, err := lib.NewManager(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	h := http.NewServeMux()
	Handle(h, mgr, jobs.NewRegistry(), birthdays.New(t.TempDir()))
	nameAll := func(id, name string) {
		o := readOverview(t, h, id)
		if rec := do(t, h, "POST", "/api/library/"+id+"/faces/name", map[string]any{"faces": o.Groups[0].Faces, "name": name}); rec.Code != http.StatusOK {
			t.Fatalf("name %s: %d %s", name, rec.Code, rec.Body)
		}
	}
	family := addLibraryWithFaces(t, mgr, "Family", 0)
	nas := addLibraryWithFaces(t, mgr, "NAS", 0)
	other := addLibraryWithFaces(t, mgr, "Other spelling", 0)
	waiting := addLibraryWithFaces(t, mgr, "Waiting", 0)
	strangers := addLibraryWithFaces(t, mgr, "Strangers", 1)
	nameAll(family, "Anna")
	nameAll(nas, "anna")
	nameAll(other, "Anna Müller")

	var everyone []lib.Someone
	json.Unmarshal(do(t, h, "GET", "/api/people", nil).Body.Bytes(), &everyone) //nolint:errcheck
	if len(everyone) != 2 || len(everyone[0].Libraries) != 2 || everyone[0].Photos != 4 || len(everyone[1].Libraries) != 1 {
		t.Fatalf("everyone %+v, want Anna in two libraries and Anna Müller in one", everyone)
	}

	rec := do(t, h, "GET", "/api/people/someone?name=ANNA", nil)
	var anna lib.SomeoneEverywhere
	json.Unmarshal(rec.Body.Bytes(), &anna) //nolint:errcheck
	if len(anna.In) != 2 || len(anna.Faces) != 4 || len(anna.NotIn) != 3 {
		t.Fatalf("Anna %s", rec.Body)
	}

	// The cover chosen last, in whichever library, shows her everywhere.
	for _, in := range anna.In {
		face := int64(0)
		for _, f := range anna.Faces {
			if f.Library == in.Library && f.ID != in.Cover {
				face = f.ID
			}
		}
		if rec := do(t, h, "PUT", fmt.Sprintf("/api/library/%s/people/%d/cover", in.Library, in.Person), map[string]any{"face": face}); rec.Code >= 300 {
			t.Fatalf("choose cover in %s: %d %s", in.LibraryName, rec.Code, rec.Body)
		}
		json.Unmarshal(do(t, h, "GET", "/api/people", nil).Body.Bytes(), &everyone) //nolint:errcheck
		if want := (lib.CoverRef{Library: in.Library, Face: face}); everyone[0].Cover != want {
			t.Errorf("after choosing in %s: cover %+v, want %+v", in.LibraryName, everyone[0].Cover, want)
		}
	}
	for _, n := range anna.NotIn {
		switch n.Library {
		case other:
			if n.LooksLike == nil || n.LooksLike.Name != "Anna Müller" {
				t.Errorf("%s: %+v, want Anna Müller suggested", n.LibraryName, n)
			}
		case waiting:
			if n.Group == nil || n.Group.Size != 2 || n.LooksLike != nil {
				t.Errorf("%s: %+v, want the group of two suggested", n.LibraryName, n)
			}
		case strangers:
			if n.Group != nil || n.LooksLike != nil {
				t.Errorf("%s: %+v, want nobody suggested", n.LibraryName, n)
			}
		}
	}
	if rec := do(t, h, "GET", "/api/people/someone", nil); rec.Code != http.StatusBadRequest {
		t.Errorf("without a name: %d, want 400", rec.Code)
	}

	// A name field anywhere offers the names of every library.
	var photo struct {
		Names []string `json:"names"`
	}
	json.Unmarshal(do(t, h, "GET", "/api/library/"+waiting+"/photo/aa1/faces", nil).Body.Bytes(), &photo) //nolint:errcheck
	if len(photo.Names) != 2 {
		t.Errorf("names offered in Waiting: %v, want Anna and Anna Müller from the other libraries", photo.Names)
	}

	// What waits for a name: the groups of Waiting and Strangers, nothing
	// in the libraries whose faces were all named.
	var toName []lib.ToName
	json.Unmarshal(do(t, h, "GET", "/api/people/to-name", nil).Body.Bytes(), &toName) //nolint:errcheck
	groups := map[string]int{}
	for _, n := range toName {
		groups[n.Library] = n.Groups
	}
	if len(toName) != 5 || groups[family] != 0 || groups[waiting] == 0 || groups[strangers] == 0 {
		t.Errorf("to name %+v", toName)
	}
}

// The People statistics read a real library: who is in which photo, faces
// per photo, every face placed; and the search finds photos by their faces.
func TestPeopleStatisticsAndTheFacesFilter(t *testing.T) {
	h, mgr, id := libraryWithFaces(t)
	o := readOverview(t, h, id)
	do(t, h, "POST", "/api/library/"+id+"/faces/name", map[string]any{"faces": o.Groups[0].Faces[:1], "name": "Anna"})

	ps, err := mgr.PeopleStats(nil, "", "", lib.Filter{})
	if err != nil {
		t.Fatal(err)
	}
	if len(ps.People) != 1 || ps.People[0].Photos != 1 || len(ps.Appearances.Photo) != 1 {
		t.Errorf("people %+v appearances %+v", ps.People, ps.Appearances)
	}
	if len(ps.FaceSpace.Face) != 2 || ps.FaceSpace.Axes != "faces" {
		t.Errorf("face space %d faces, axes %q", len(ps.FaceSpace.Face), ps.FaceSpace.Axes)
	}

	store, _ := mgr.OpenStore(id)
	one, two := 1, 2
	if res, _ := store.ListPhotos(context.Background(), lib.ListPhotosOpts{FacesMin: &one, FacesMax: &one, Limit: 10}); res.Total != 2 {
		t.Errorf("photos with one face: %d, want 2", res.Total)
	}
	if res, _ := store.ListPhotos(context.Background(), lib.ListPhotosOpts{FacesMin: &two, Limit: 10}); res.Total != 0 {
		t.Errorf("photos with two faces or more: %d, want 0", res.Total)
	}
}

// A person's page across libraries names who is most often in their photos.
func TestWhoIsOftenWithSomeone(t *testing.T) {
	mgr, err := lib.NewManager(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	h := http.NewServeMux()
	Handle(h, mgr, jobs.NewRegistry(), nil)
	source := t.TempDir()
	l, _ := mgr.CreateLibrary("Family", "", source)
	store, _ := mgr.OpenStore(l.ID)
	look := func(i int) []float32 { e := make([]float32, 128); e[i] = 1; return e }
	for _, id := range []string{"p1", "p2"} {
		writeJPEG(t, filepath.Join(source, id+".jpg"))
		if err := store.UpsertPhoto(id, filepath.Join(source, id+".jpg"), id+".jpg", 1, time.Now(), "{}", "", "", "jpeg"); err != nil {
			t.Fatal(err)
		}
	}
	store.SaveFoundFaces([]lib.FoundFaces{ //nolint:errcheck
		{PhotoID: "p1", Faces: []faces.Face{{X: 0.1, Y: 0.1, W: 0.2, H: 0.2, Embedding: look(0)}, {X: 0.6, Y: 0.1, W: 0.2, H: 0.2, Embedding: look(1)}}, Names: []string{"", ""}},
		{PhotoID: "p2", Faces: []faces.Face{{X: 0.1, Y: 0.1, W: 0.2, H: 0.2, Embedding: look(0)}}, Names: []string{""}},
	})
	name := func(photo string, x float64, who string) {
		fs, _ := store.PhotoFaces(photo)
		for _, f := range fs {
			if f.X == x {
				if _, err := store.NameFaces([]int64{f.ID}, who); err != nil {
					t.Fatal(err)
				}
			}
		}
	}
	name("p1", 0.1, "Anna")
	name("p2", 0.1, "Anna")
	name("p1", 0.6, "Ben")

	var anna lib.SomeoneEverywhere
	json.Unmarshal(do(t, h, "GET", "/api/people/someone?name=Anna", nil).Body.Bytes(), &anna) //nolint:errcheck
	if len(anna.With) != 1 || anna.With[0].Name != "Ben" || anna.With[0].Photos != 1 {
		t.Fatalf("with Anna: %+v, want Ben in one photo", anna.With)
	}
	if anna.With[0].Cover.Face == 0 || anna.With[0].Cover.Library == "" {
		t.Errorf("Ben shows no face: %+v", anna.With[0])
	}
	var with []lib.Companion
	json.Unmarshal(do(t, h, "GET", "/api/people/with?name=anna", nil).Body.Bytes(), &with) //nolint:errcheck
	if len(with) != 1 || with[0].Name != "Ben" {
		t.Errorf("/api/people/with: %+v, want Ben", with)
	}
}
