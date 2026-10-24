package media

import (
	"math"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

func thingsPhoto(t *testing.T, sidecar string) string {
	t.Helper()
	photo := filepath.Join(t.TempDir(), "p.jpg")
	if sidecar != "" {
		if err := os.WriteFile(SidecarPath(photo), []byte(sidecar), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return photo
}

func readThings(t *testing.T, photo string) SidecarThings {
	t.Helper()
	got, err := ReadThings(photo)
	if err != nil {
		t.Fatal(err)
	}
	return got
}

func TestFoundThingsKeepTheOwnersKeywords(t *testing.T) {
	photo := thingsPhoto(t, lightroomXMP)
	if _, err := WriteFoundThings(photo, []string{"dog", "sea"}, "1.1", nil); err != nil {
		t.Fatal(err)
	}
	if _, err := WriteFoundThings(photo, []string{"dog", "beach"}, "1.2", nil); err != nil {
		t.Fatal(err)
	}
	got := readThings(t, photo)
	if !slices.Equal(got.Keywords, []string{"Urlaub", "dog", "beach"}) || got.Record.Version != "1.2" {
		t.Fatalf("%+v", got)
	}
	data, _ := os.ReadFile(SidecarPath(photo))
	keepsLightroom(t, string(data))
}

func TestARemovedThingStaysAway(t *testing.T) {
	photo := thingsPhoto(t, "")
	WriteFoundThings(photo, []string{"dog", "cat"}, "1.1", nil) //nolint:errcheck
	if err := RemoveThing(photo, "cat"); err != nil {
		t.Fatal(err)
	}
	WriteFoundThings(photo, []string{"dog", "cat"}, "1.2", nil) //nolint:errcheck
	got := readThings(t, photo)
	if !slices.Equal(got.Things(), []string{"dog"}) || slices.Contains(got.Keywords, "cat") {
		t.Fatalf("%+v", got)
	}
}

// A keyword removed in another program takes its thing with it.
func TestAKeywordRemovedElsewhereTakesTheThing(t *testing.T) {
	photo := thingsPhoto(t, "")
	WriteFoundThings(photo, []string{"dog", "tree"}, "1.1", nil) //nolint:errcheck
	data, _ := os.ReadFile(SidecarPath(photo))
	edited := strings.Replace(string(data), "\n          <rdf:li>tree</rdf:li>\n        </rdf:Bag>\n      </dc:subject>", "\n        </rdf:Bag>\n      </dc:subject>", 1)
	os.WriteFile(SidecarPath(photo), []byte(edited), 0o644) //nolint:errcheck
	if got := readThings(t, photo).Things(); !slices.Equal(got, []string{"dog"}) {
		t.Fatalf("things %v", got)
	}
}

func TestNoSidecarIsWrittenForNothing(t *testing.T) {
	photo := thingsPhoto(t, "")
	if written, err := WriteFoundThings(photo, nil, "1.1", nil); err != nil || written {
		t.Fatalf("written %v, %v", written, err)
	}
	if _, err := os.Stat(SidecarPath(photo)); err == nil {
		t.Fatal("a sidecar was written for a photo without things")
	}
}

func TestThingsAndFieldsAndPublicationsKeepEachOther(t *testing.T) {
	photo := thingsPhoto(t, "")
	WriteField(photo, "Ort", "Rom")                         //nolint:errcheck
	WriteFoundThings(photo, []string{"church"}, "1.1", nil) //nolint:errcheck
	AppendPublication(photo, Publication{Channel: "site"})  //nolint:errcheck
	WriteField(photo, "Anlass", "Urlaub")                   //nolint:errcheck
	notes, _ := ReadNotes(photo)
	pubs, _ := ReadSidecar(photo)
	got := readThings(t, photo)
	if notes.Fields["Ort"] != "Rom" || len(pubs) != 1 || !slices.Equal(got.Things(), []string{"church"}) {
		t.Fatalf("fields %v, publications %d, things %+v", notes.Fields, len(pubs), got)
	}
}

// The vector travels in the sidecar at half precision and comes back close
// enough that no similarity changes noticeably.
func TestTheVectorTravelsInTheSidecar(t *testing.T) {
	photo := thingsPhoto(t, "")
	v := make([]float32, 512)
	for i := range v {
		v[i] = float32(math.Sin(float64(i))) / 16
	}
	if _, err := WriteFoundThings(photo, []string{"dog"}, "1.1", v); err != nil {
		t.Fatal(err)
	}
	WriteField(photo, "Ort", "Rom") //nolint:errcheck // another writer keeps it
	got := readThings(t, photo).Record.Vector
	if len(got) != len(v) {
		t.Fatalf("%d numbers back of %d", len(got), len(v))
	}
	for i := range v {
		if d := math.Abs(float64(got[i] - v[i])); d > 1e-3 {
			t.Fatalf("number %d: %v, wrote %v", i, got[i], v[i])
		}
	}
}

func TestAVectorIsAddedToAnExistingRecord(t *testing.T) {
	photo := thingsPhoto(t, "")
	if ok, _ := WriteThingsVector(photo, []float32{0.5}); ok {
		t.Fatal("a vector was written without a record")
	}
	WriteFoundThings(photo, []string{"cat"}, "1.1", nil) //nolint:errcheck
	if ok, err := WriteThingsVector(photo, []float32{0.5, -0.25}); !ok || err != nil {
		t.Fatalf("written %v, %v", ok, err)
	}
	got := readThings(t, photo)
	if len(got.Record.Vector) != 2 || got.Record.Vector[1] != -0.25 || !slices.Equal(got.Things(), []string{"cat"}) {
		t.Fatalf("%+v", got)
	}
}
