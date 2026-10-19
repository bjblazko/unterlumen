package apipeople

import (
	"encoding/json"
	"net/http"
	"testing"

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
	Handle(h, mgr, jobs.NewRegistry())
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
	if len(anna.In) != 2 || len(anna.In[0].Shown) != 2 || len(anna.NotIn) != 3 {
		t.Fatalf("Anna %s", rec.Body)
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
}
