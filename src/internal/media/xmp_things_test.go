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
	if _, err := WriteFoundThings(photo, []string{"dog", "sea"}, "1.1", nil, english); err != nil {
		t.Fatal(err)
	}
	if _, err := WriteFoundThings(photo, []string{"dog", "beach"}, "1.2", nil, english); err != nil {
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
	WriteFoundThings(photo, []string{"dog", "cat"}, "1.1", nil, english) //nolint:errcheck
	if err := RemoveThing(photo, "cat", english); err != nil {
		t.Fatal(err)
	}
	WriteFoundThings(photo, []string{"dog", "cat"}, "1.2", nil, english) //nolint:errcheck
	got := readThings(t, photo)
	if !slices.Equal(got.Things(english), []string{"dog"}) || slices.Contains(got.Keywords, "cat") {
		t.Fatalf("%+v", got)
	}
}

// A keyword removed in another program takes its thing with it.
func TestAKeywordRemovedElsewhereTakesTheThing(t *testing.T) {
	photo := thingsPhoto(t, "")
	WriteFoundThings(photo, []string{"dog", "tree"}, "1.1", nil, english) //nolint:errcheck
	data, _ := os.ReadFile(SidecarPath(photo))
	edited := strings.Replace(string(data), "\n          <rdf:li>tree</rdf:li>\n        </rdf:Bag>\n      </dc:subject>", "\n        </rdf:Bag>\n      </dc:subject>", 1)
	os.WriteFile(SidecarPath(photo), []byte(edited), 0o644) //nolint:errcheck
	if got := readThings(t, photo).Things(english); !slices.Equal(got, []string{"dog"}) {
		t.Fatalf("things %v", got)
	}
}

func TestNoSidecarIsWrittenForNothing(t *testing.T) {
	photo := thingsPhoto(t, "")
	if written, err := WriteFoundThings(photo, nil, "1.1", nil, english); err != nil || written {
		t.Fatalf("written %v, %v", written, err)
	}
	if _, err := os.Stat(SidecarPath(photo)); err == nil {
		t.Fatal("a sidecar was written for a photo without things")
	}
}

func TestThingsAndFieldsAndPublicationsKeepEachOther(t *testing.T) {
	photo := thingsPhoto(t, "")
	WriteField(photo, "Ort", "Rom")                                  //nolint:errcheck
	WriteFoundThings(photo, []string{"church"}, "1.1", nil, english) //nolint:errcheck
	AppendPublication(photo, Publication{Channel: "site"})           //nolint:errcheck
	WriteField(photo, "Anlass", "Urlaub")                            //nolint:errcheck
	notes, _ := ReadNotes(photo)
	pubs, _ := ReadSidecar(photo)
	got := readThings(t, photo)
	if notes.Fields["Ort"] != "Rom" || len(pubs) != 1 || !slices.Equal(got.Things(english), []string{"church"}) {
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
	if _, err := WriteFoundThings(photo, []string{"dog"}, "1.1", v, english); err != nil {
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
	WriteFoundThings(photo, []string{"cat"}, "1.1", nil, english) //nolint:errcheck
	if ok, err := WriteThingsVector(photo, []float32{0.5, -0.25}); !ok || err != nil {
		t.Fatalf("written %v, %v", ok, err)
	}
	got := readThings(t, photo)
	if len(got.Record.Vector) != 2 || got.Record.Vector[1] != -0.25 || !slices.Equal(got.Things(english), []string{"cat"}) {
		t.Fatalf("%+v", got)
	}
}

// testLex is a lexicon of two languages for a few things: English names the
// things by their IDs, German by its own words.
type testLex struct{ german bool }

var (
	english = testLex{}
	german  = testLex{german: true}
	deNames = map[string]string{"dog": "Hund", "cat": "Katze", "beach": "Strand", "sea": "Meer", "tree": "Baum", "church": "Kirche", "lighthouse": "Leuchtturm", "windmill": "Windmühle"}
)

func (l testLex) Keyword(id string) string {
	if l.german && deNames[id] != "" {
		return deNames[id]
	}
	return id
}

func (l testLex) Keywords(id string) []string {
	if deNames[id] != "" {
		return []string{id, deNames[id]}
	}
	return []string{id}
}

// A library's language changes the keywords Unterlumen wrote, and only
// those; a German keyword is read back to its thing.
func TestKeywordsAreRewrittenInTheLibrarysLanguage(t *testing.T) {
	photo := thingsPhoto(t, lightroomXMP)
	WriteFoundThings(photo, []string{"dog", "beach"}, "2.1", nil, english) //nolint:errcheck
	if changed, err := RewriteThingKeywords(photo, german); !changed || err != nil {
		t.Fatalf("changed %v, %v", changed, err)
	}
	got := readThings(t, photo)
	if !slices.Equal(got.Keywords, []string{"Urlaub", "Hund", "Strand"}) {
		t.Fatalf("keywords %v", got.Keywords)
	}
	if !slices.Equal(got.Things(german), []string{"dog", "beach"}) || !slices.Equal(got.Things(english), []string{"dog", "beach"}) {
		t.Fatalf("things %v", got.Things(german))
	}
	if changed, _ := RewriteThingKeywords(photo, german); changed {
		t.Fatal("a sidecar already in the language was written again")
	}
	RemoveThing(photo, "dog", german) //nolint:errcheck
	if got := readThings(t, photo); slices.Contains(got.Keywords, "Hund") || !slices.Equal(got.Things(german), []string{"beach"}) {
		t.Fatalf("after removing: %+v", got)
	}
}

// Writing the same sidecar again changes nothing: blocks put in place of
// themselves once indented further with every write.
func TestWritingTheSameAgainChangesNothing(t *testing.T) {
	photo := thingsPhoto(t, lightroomXMP)
	write := func() []byte {
		WriteFoundThings(photo, []string{"dog"}, "2.1", nil, german)                                      //nolint:errcheck
		WriteTitle(photo, "Rom")                                                                          //nolint:errcheck
		WriteField(photo, "Ort", "Rom")                                                                   //nolint:errcheck
		WriteFaceRegions(photo, []FaceRegion{{Name: "Anna", X: 0.1, Y: 0.1, W: 0.2, H: 0.2}}, 1, 100, 80) //nolint:errcheck
		data, _ := os.ReadFile(SidecarPath(photo))
		return data
	}
	first, second := write(), write()
	if string(first) != string(second) {
		t.Fatalf("the second write changed the sidecar:\n%s\n---\n%s", first, second)
	}
}
