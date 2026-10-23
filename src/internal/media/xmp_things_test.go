package media

import (
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
	if _, err := WriteFoundThings(photo, []string{"dog", "sea"}, "1.1"); err != nil {
		t.Fatal(err)
	}
	if _, err := WriteFoundThings(photo, []string{"dog", "beach"}, "1.2"); err != nil {
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
	WriteFoundThings(photo, []string{"dog", "cat"}, "1.1") //nolint:errcheck
	if err := RemoveThing(photo, "cat"); err != nil {
		t.Fatal(err)
	}
	WriteFoundThings(photo, []string{"dog", "cat"}, "1.2") //nolint:errcheck
	got := readThings(t, photo)
	if !slices.Equal(got.Things(), []string{"dog"}) || slices.Contains(got.Keywords, "cat") {
		t.Fatalf("%+v", got)
	}
}

// A keyword removed in another program takes its thing with it.
func TestAKeywordRemovedElsewhereTakesTheThing(t *testing.T) {
	photo := thingsPhoto(t, "")
	WriteFoundThings(photo, []string{"dog", "tree"}, "1.1") //nolint:errcheck
	data, _ := os.ReadFile(SidecarPath(photo))
	edited := strings.Replace(string(data), "\n          <rdf:li>tree</rdf:li>\n        </rdf:Bag>\n      </dc:subject>", "\n        </rdf:Bag>\n      </dc:subject>", 1)
	os.WriteFile(SidecarPath(photo), []byte(edited), 0o644) //nolint:errcheck
	if got := readThings(t, photo).Things(); !slices.Equal(got, []string{"dog"}) {
		t.Fatalf("things %v", got)
	}
}

func TestNoSidecarIsWrittenForNothing(t *testing.T) {
	photo := thingsPhoto(t, "")
	if written, err := WriteFoundThings(photo, nil, "1.1"); err != nil || written {
		t.Fatalf("written %v, %v", written, err)
	}
	if _, err := os.Stat(SidecarPath(photo)); err == nil {
		t.Fatal("a sidecar was written for a photo without things")
	}
}

func TestThingsAndFieldsAndPublicationsKeepEachOther(t *testing.T) {
	photo := thingsPhoto(t, "")
	WriteField(photo, "Ort", "Rom")                        //nolint:errcheck
	WriteFoundThings(photo, []string{"church"}, "1.1")     //nolint:errcheck
	AppendPublication(photo, Publication{Channel: "site"}) //nolint:errcheck
	WriteField(photo, "Anlass", "Urlaub")                  //nolint:errcheck
	notes, _ := ReadNotes(photo)
	pubs, _ := ReadSidecar(photo)
	got := readThings(t, photo)
	if notes.Fields["Ort"] != "Rom" || len(pubs) != 1 || !slices.Equal(got.Things(), []string{"church"}) {
		t.Fatalf("fields %v, publications %d, things %+v", notes.Fields, len(pubs), got)
	}
}
