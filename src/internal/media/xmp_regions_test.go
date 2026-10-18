package media

import (
	"errors"
	"math"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func near(a, b float64) bool { return math.Abs(a-b) < 1e-4 }

func sameRegion(a, b FaceRegion) bool {
	return a.Name == b.Name && near(a.X, b.X) && near(a.Y, b.Y) && near(a.W, b.W) && near(a.H, b.H)
}

func TestFaceRegionsRoundTripInEveryOrientation(t *testing.T) {
	r := FaceRegion{Name: "Anna", X: 0.1, Y: 0.2, W: 0.15, H: 0.25}
	for o := 1; o <= 8; o++ {
		if got := storedToUpright(uprightToStored(r, o), o); !sameRegion(got, r) {
			t.Errorf("orientation %d: %+v came back as %+v", o, r, got)
		}
	}
}

func TestOrientationSixTurnsTopLeftToTopRight(t *testing.T) {
	// A face at the stored photo's top left is at the top right once it is turned 90° clockwise.
	got := storedToUpright(FaceRegion{X: 0, Y: 0, W: 0.2, H: 0.1}, 6)
	if !sameRegion(got, FaceRegion{X: 0.9, Y: 0, W: 0.1, H: 0.2}) {
		t.Fatalf("got %+v", got)
	}
}

func TestWriteThenReadFaceRegions(t *testing.T) {
	photo := filepath.Join(t.TempDir(), "a.jpg")
	want := []FaceRegion{{Name: "Anna & Ben", X: 0.1, Y: 0.2, W: 0.1, H: 0.15}, {Name: "Carla", X: 0.6, Y: 0.3, W: 0.1, H: 0.12}}
	if err := WriteFaceRegions(photo, want, 6, 6000, 4000); err != nil {
		t.Fatal(err)
	}
	got, err := ReadFaceRegions(photo, 6)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 || !sameRegion(got[0], want[0]) || !sameRegion(got[1], want[1]) {
		t.Fatalf("got %+v", got)
	}
}

func TestWriteFaceRegionsKeepsTheRestOfTheSidecar(t *testing.T) {
	photo := filepath.Join(t.TempDir(), "a.jpg")
	if err := WriteField(photo, "Place", "Kiel"); err != nil {
		t.Fatal(err)
	}
	if err := WriteFaceRegions(photo, []FaceRegion{{Name: "Anna", X: 0.1, Y: 0.1, W: 0.1, H: 0.1}}, 1, 100, 100); err != nil {
		t.Fatal(err)
	}
	if err := WriteFaceRegions(photo, []FaceRegion{{Name: "Ben", X: 0.1, Y: 0.1, W: 0.1, H: 0.1}}, 1, 100, 100); err != nil {
		t.Fatal(err)
	}
	notes, _ := ReadNotes(photo)
	regions, _ := ReadFaceRegions(photo, 1)
	if notes.Fields["Place"] != "Kiel" || len(regions) != 1 || regions[0].Name != "Ben" {
		t.Fatalf("fields %v, regions %+v", notes.Fields, regions)
	}
	if err := WriteFaceRegions(photo, nil, 1, 100, 100); err != nil {
		t.Fatal(err)
	}
	data, _ := os.ReadFile(SidecarPath(photo))
	if strings.Contains(string(data), "mwg-rs") || !strings.Contains(string(data), "Kiel") {
		t.Fatalf("removing the regions left:\n%s", data)
	}
}

// A Lightroom sidecar keeps everything in one description; its regions are
// read, and never overwritten.
const lightroomSidecar = `<x:xmpmeta xmlns:x="adobe:ns:meta/">
 <rdf:RDF xmlns:rdf="http://www.w3.org/1999/02/22-rdf-syntax-ns#">
  <rdf:Description rdf:about="" xmlns:xmp="http://ns.adobe.com/xap/1.0/"
    xmlns:mwg-rs="http://www.metadataworkinggroup.com/schemas/regions/"
    xmlns:stArea="http://ns.adobe.com/xmp/sType/Area#" xmp:Rating="3">
   <mwg-rs:Regions rdf:parseType="Resource">
    <mwg-rs:RegionList>
     <rdf:Bag>
      <rdf:li>
       <rdf:Description mwg-rs:Name="Dora" mwg-rs:Type="Face">
        <mwg-rs:Area stArea:x="0.5" stArea:y="0.5" stArea:w="0.2" stArea:h="0.2" stArea:unit="normalized"/>
       </rdf:Description>
      </rdf:li>
     </rdf:Bag>
    </mwg-rs:RegionList>
   </mwg-rs:Regions>
  </rdf:Description>
 </rdf:RDF>
</x:xmpmeta>`

func TestForeignRegionsAreReadButNotWritten(t *testing.T) {
	photo := filepath.Join(t.TempDir(), "a.jpg")
	os.WriteFile(SidecarPath(photo), []byte(lightroomSidecar), 0o644) //nolint:errcheck
	got, _ := ReadFaceRegions(photo, 1)
	if len(got) != 1 || !sameRegion(got[0], FaceRegion{Name: "Dora", X: 0.4, Y: 0.4, W: 0.2, H: 0.2}) {
		t.Fatalf("got %+v", got)
	}
	err := WriteFaceRegions(photo, []FaceRegion{{Name: "Eva", X: 0.1, Y: 0.1, W: 0.1, H: 0.1}}, 1, 100, 100)
	if !errors.Is(err, ErrForeignRegions) {
		t.Fatalf("writing over Lightroom's regions: %v", err)
	}
}
