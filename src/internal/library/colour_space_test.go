package library

import (
	"math"
	"testing"
)

// A point sits at the photo's largest swatch with a hue, passing over a
// larger grey one; a photo without such a swatch sits at its average.
func TestColourPoints(t *testing.T) {
	s := newTestStore(t)
	seedMeasured(t, s, []measuredPhoto{
		{id: "sky", path: "/lib/a/1.jpg", date: "2024-07-01T10:00:00", mono: "colour", swatches: []ColourSwatch{
			{HueBin: -1, Share: 0.6, Colour: LCh{0.9, 0.01, 0}},
			{HueBin: 8, Share: 0.3, Colour: LCh{0.6, 0.12, 250}},
			{HueBin: 2, Share: 0.1, Colour: LCh{0.7, 0.15, 70}},
		}},
		{id: "bw", path: "/lib/b/2.jpg", date: "2024-07-02T10:00:00", mono: "mono",
			swatches: []ColourSwatch{{HueBin: -1, Share: 1, Colour: LCh{0.4, 0, 0}}}},
		{id: "new", path: "/lib/b/3.jpg", date: "2024-07-03T10:00:00"},
	})
	points, err := s.ColourPoints("")
	if err != nil {
		t.Fatal(err)
	}
	got := map[string]ColourPoint{}
	for _, p := range points {
		got[p.ID] = p
	}
	if len(points) != 2 {
		t.Fatalf("points %d, want the 2 analysed photos", len(points))
	}
	if got["sky"].Colour != (LCh{0.6, 0.12, 250}) {
		t.Errorf("sky = %+v, want the blue swatch", got["sky"])
	}
	if got["bw"].Colour != (LCh{0.5, 0.1, 60}) {
		t.Errorf("bw = %+v, want its average colour", got["bw"].Colour)
	}
	scoped, _ := s.ColourPoints("/lib/b")
	if len(scoped) != 1 || scoped[0].ID != "bw" {
		t.Errorf("scoped = %+v", scoped)
	}
	if n, _ := s.UnanalysedCount("/lib/b"); n != 1 {
		t.Errorf("unanalysed %d, want 1", n)
	}
}

// The path averages the colour photos of each period in a and b; black and
// white photos are points but stay off the path.
func TestBuildColourSpace(t *testing.T) {
	cs := BuildColourSpace([]LibraryPoints{
		{LibraryID: "x", Unanalysed: 2, Points: []ColourPoint{
			{ID: "r", Date: "2024-01-05", Mono: "colour", Colour: LCh{0.6, 0.2, 0}},
			{ID: "g", Date: "2024-01-09", Mono: "colour", Colour: LCh{0.4, 0.2, 180}},
			{ID: "bw", Date: "2024-01-10", Mono: "mono", Colour: LCh{0.9, 0, 0}},
		}},
		{LibraryID: "y", Points: []ColourPoint{
			{ID: "b", Date: "2024-03-01", Mono: "colour", Colour: LCh{0.5, 0.1, 90}},
			{ID: "undated", Mono: "colour", Colour: LCh{0.5, 0.1, 90}},
		}},
	}, "")
	if cs.Granularity != "month" || cs.AnalysedPhotos != 5 || cs.UnanalysedPhotos != 2 {
		t.Errorf("got %s, %d, %d", cs.Granularity, cs.AnalysedPhotos, cs.UnanalysedPhotos)
	}
	if len(cs.Points.ID) != 5 || cs.Points.Lib[3] != 1 || cs.Libraries[1] != "y" {
		t.Errorf("points %+v", cs.Points)
	}
	if b := cs.Points.B[3]; math.Abs(b-0.1) > 1e-4 {
		t.Errorf("b of a 90° hue = %v, want 0.1", b)
	}
	if len(cs.Path) != 2 {
		t.Fatalf("path %+v, want two months", cs.Path)
	}
	jan := cs.Path[0]
	if jan.Period != "2024-01" || jan.Photos != 2 || math.Abs(jan.L-0.5) > 1e-4 || math.Abs(jan.A) > 1e-4 {
		t.Errorf("January = %+v, want red and green to cancel out at L 0.5", jan)
	}
}

func TestBuildColourSpaceEmpty(t *testing.T) {
	cs := BuildColourSpace(nil, "year")
	if cs.Granularity != "year" || len(cs.Path) != 0 || cs.Libraries == nil {
		t.Errorf("got %+v", cs)
	}
}

func TestListPhotosByPhotoID(t *testing.T) {
	got, _ := listedIDs(t, colourStore(t), ListPhotosOpts{PhotoID: "cool"})
	if len(got) != 1 || got[0] != "cool" {
		t.Errorf("got %v, want only cool", got)
	}
}
