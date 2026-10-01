package media

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func photoIn(t *testing.T) string {
	t.Helper()
	p := filepath.Join(t.TempDir(), "photo.jpg")
	os.WriteFile(p, []byte("jpeg"), 0o644) //nolint:errcheck
	return p
}

func TestFieldsRoundTripBesideTitleAndPublications(t *testing.T) {
	photo := photoIn(t)
	pub := Publication{Channel: "website", PostID: "abc", GalleryTitle: "Rome", PublishedAt: time.Date(2025, 9, 14, 12, 0, 0, 0, time.UTC)}
	if err := AppendPublication(photo, pub); err != nil {
		t.Fatal(err)
	}
	if err := WriteTitle(photo, "Trevi at noon"); err != nil {
		t.Fatal(err)
	}
	if err := WriteField(photo, "rating", "4"); err != nil {
		t.Fatal(err)
	}
	if err := WriteField(photo, "people", "Anna & Ben <3"); err != nil {
		t.Fatal(err)
	}

	notes, err := ReadNotes(photo)
	if err != nil {
		t.Fatal(err)
	}
	if notes.Title != "Trevi at noon" || notes.Fields["rating"] != "4" || notes.Fields["people"] != "Anna & Ben <3" {
		t.Errorf("notes = %+v", notes)
	}
	if pubs, _ := ReadSidecar(photo); len(pubs) != 1 || pubs[0].PostID != "abc" {
		t.Errorf("publications after writing fields = %+v", pubs)
	}

	// A publication written afterwards keeps the fields.
	if err := RemovePublication(photo, "website", "abc"); err != nil {
		t.Fatal(err)
	}
	if notes, _ := ReadNotes(photo); len(notes.Fields) != 2 {
		t.Errorf("fields after removing a publication = %+v", notes.Fields)
	}
}

func TestWriteFieldWithAnEmptyValueRemovesIt(t *testing.T) {
	photo := photoIn(t)
	WriteField(photo, "rating", "4")  //nolint:errcheck
	WriteField(photo, "mood", "calm") //nolint:errcheck
	if err := WriteField(photo, "rating", ""); err != nil {
		t.Fatal(err)
	}
	notes, _ := ReadNotes(photo)
	if _, ok := notes.Fields["rating"]; ok || notes.Fields["mood"] != "calm" {
		t.Errorf("fields = %+v, want only mood", notes.Fields)
	}
	raw, _ := os.ReadFile(SidecarPath(photo))
	if strings.Contains(string(raw), "<ul:Publications>") {
		t.Error("a sidecar with fields only has an empty Publications element")
	}
}

func TestWriteFieldKeepsOtherProgramsContent(t *testing.T) {
	photo := photoIn(t)
	foreign := `<?xml version="1.0" encoding="UTF-8"?>
<x:xmpmeta xmlns:x="adobe:ns:meta/">
  <rdf:RDF xmlns:rdf="http://www.w3.org/1999/02/22-rdf-syntax-ns#">
    <rdf:Description rdf:about="" xmlns:xmp="http://ns.adobe.com/xap/1.0/" xmp:Rating="5"/>
  </rdf:RDF>
</x:xmpmeta>`
	os.WriteFile(SidecarPath(photo), []byte(foreign), 0o644) //nolint:errcheck
	if err := WriteField(photo, "mood", "calm"); err != nil {
		t.Fatal(err)
	}
	raw, _ := os.ReadFile(SidecarPath(photo))
	if !strings.Contains(string(raw), `xmp:Rating="5"`) {
		t.Errorf("another program's rating was lost:\n%s", raw)
	}
	if notes, _ := ReadNotes(photo); notes.Fields["mood"] != "calm" {
		t.Errorf("fields = %+v", notes.Fields)
	}
}

func TestReadNotesWithoutASidecarIsEmpty(t *testing.T) {
	notes, err := ReadNotes(photoIn(t))
	if err != nil || notes.Title != "" || len(notes.Fields) != 0 {
		t.Errorf("ReadNotes = %+v, %v", notes, err)
	}
}
