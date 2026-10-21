package library

import (
	"math"
	"testing"
)

func unit(d, i int, wobble float32) []float32 {
	e := make([]float32, d)
	e[i] = 1
	e[(i+1)%d] = wobble
	return e
}

// People are put together across libraries by name, counted by photo, and
// the photos looked at are counted by how many faces they hold.
func TestPeopleStatsAcrossLibraries(t *testing.T) {
	libs := []LibraryPeople{
		{LibraryID: "a",
			People: []Person{{ID: 1, Name: "Anna", Faces: 2, Cover: 11}, {ID: 2, Name: "Ben", Faces: 1, Cover: 12}},
			Appearances: []Appearance{
				{"Anna", "p1", "2024-01-05"}, {"Ben", "p1", "2024-01-05"}, {"Anna", "p2", "2025-03-01"},
			},
			FaceCounts: []FaceCount{{"2024-01-05", 2}, {"2025-03-01", 1}, {"2025-04-01", 0}, {"", 7}},
		},
		{LibraryID: "b",
			People:      []Person{{ID: 5, Name: "anna", Faces: 5, Cover: 51}},
			Appearances: []Appearance{{"anna", "q1", "2025-06-01"}},
			FaceCounts:  []FaceCount{{"2025-06-01", 4}},
		},
	}
	ps := BuildPeopleStats(libs, "year")
	if len(ps.People) != 2 || ps.People[0].Name != "Anna" || ps.People[0].Photos != 3 || ps.People[1].Photos != 1 {
		t.Fatalf("people %+v, want Anna in 3 photos, then Ben", ps.People)
	}
	if ps.People[0].Lib != 1 || ps.People[0].Cover != 51 {
		t.Errorf("Anna's cover %+v, want the library with most of her faces", ps.People[0])
	}
	libs[0].People[0].CoverChosen = "2026-10-21T08:00:00.000000000Z"
	if c := BuildPeopleStats(libs, "year").People[0]; c.Lib != 0 || c.Cover != 11 {
		t.Errorf("Anna's cover %+v, want the one chosen, though in the library with fewer of her faces", c)
	}
	libs[0].People[0].CoverChosen = ""
	if got := len(ps.Appearances.Person); got != 4 {
		t.Errorf("%d appearances, want 4", got)
	}
	want := []int{1, 1, 1, 1, 1} // none, one, two, three to five, six or more
	for i, n := range ps.Crowd.Total {
		if n != want[i] {
			t.Fatalf("crowd %v, want %v", ps.Crowd.Total, want)
		}
	}
	if len(ps.Crowd.Periods) != 2 || ps.Crowd.Periods[0] != "2024" || ps.Crowd.Counts[1][0] != 1 {
		t.Errorf("crowd periods %v %v", ps.Crowd.Periods, ps.Crowd.Counts)
	}
}

// With four or more people named, each person's faces stay together and
// apart from the others in the Face space; hidden faces are not among them.
func TestFaceSpaceKeepsAPersonTogether(t *testing.T) {
	const d = 16
	var prints []FacePrint
	names := []string{"Anna", "Ben", "Cleo", "Dan"}
	for p, name := range names {
		for k := range 5 {
			prints = append(prints, FacePrint{Face: int64(p*10 + k), Photo: "x", Name: name, Embedding: unit(d, p*3, float32(k)/20)})
		}
	}
	prints = append(prints, FacePrint{Face: 99, Photo: "y", Embedding: unit(d, 13, 0)})
	libs := []LibraryPeople{{LibraryID: "a", Prints: prints, Appearances: []Appearance{
		{"Anna", "x", ""}, {"Ben", "x", ""}, {"Cleo", "x", ""}, {"Dan", "x", ""},
	}}}
	fs := BuildPeopleStats(libs, "").FaceSpace
	if fs.Axes != "people" || len(fs.Face) != 21 || fs.Person[20] != -1 {
		t.Fatalf("face space axes %q, %d faces, last person %d", fs.Axes, len(fs.Face), fs.Person[20])
	}
	dist := func(i, j int) float64 {
		return math.Hypot(math.Hypot(fs.X[i]-fs.X[j], fs.Y[i]-fs.Y[j]), fs.Z[i]-fs.Z[j])
	}
	within, between := dist(0, 4), dist(0, 5)
	if within >= between {
		t.Errorf("Anna's faces %.3f apart, Anna and Ben %.3f: a person should stay together", within, between)
	}
}

// The axes are the same for the same faces, so the picture does not change
// from one visit to the next.
func TestFaceAxesAreStable(t *testing.T) {
	vs := [][]float32{unit(8, 0, 0.1), unit(8, 2, 0.3), unit(8, 5, 0.2), unit(8, 6, 0)}
	_, a := faceAxes(vs)
	_, b := faceAxes(vs)
	for k := range a {
		for i := range a[k] {
			if a[k][i] != b[k][i] {
				t.Fatalf("axis %d differs between two runs", k)
			}
		}
	}
}
