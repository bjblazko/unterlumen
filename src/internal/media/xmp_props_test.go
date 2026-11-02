package media

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const lightroomPropsSidecar = `<?xml version="1.0" encoding="UTF-8"?>
<x:xmpmeta xmlns:x="adobe:ns:meta/">
  <rdf:RDF xmlns:rdf="http://www.w3.org/1999/02/22-rdf-syntax-ns#">
    <rdf:Description rdf:about=""
        xmlns:xmp="http://ns.adobe.com/xap/1.0/"
        xmlns:crs="http://ns.adobe.com/camera-raw-settings/1.0/"
        xmlns:dc="http://purl.org/dc/elements/1.1/"
        xmp:Rating="3"
        crs:Exposure2012="+0.50">
      <dc:rights><rdf:Alt><rdf:li xml:lang="x-default">Old</rdf:li></rdf:Alt></dc:rights>
    </rdf:Description>
  </rdf:RDF>
</x:xmpmeta>`

func TestPropsAreChangedWhereLightroomKeepsThem(t *testing.T) {
	out := string(withProps([]byte(lightroomPropsSidecar), map[Prop]string{
		PropRating:    "5",
		PropRights:    "© 2026 Ann",
		PropDateTaken: "2024-05-01T12:30:00+02:00",
	}))
	if !strings.Contains(out, `xmp:Rating="5"`) || strings.Contains(out, `<xmp:Rating>`) {
		t.Errorf("rating not changed in its attribute:\n%s", out)
	}
	if strings.Count(out, "<dc:rights>") != 1 || !strings.Contains(out, "© 2026 Ann") {
		t.Errorf("rights not changed in place:\n%s", out)
	}
	if !strings.Contains(out, `crs:Exposure2012="+0.50"`) {
		t.Errorf("another program's setting was lost:\n%s", out)
	}
	got := parseProps([]byte(out))
	if got[PropDateTaken] != "2024-05-01T12:30:00+02:00" || got[PropRating] != "5" || got[PropRights] != "© 2026 Ann" {
		t.Errorf("read back %v", got)
	}
}

func TestPropsInOwnDescriptionRoundTrip(t *testing.T) {
	out := withProps(nil, map[Prop]string{PropCreator: "Ann; Bob", PropDescription: "A <quiet> day"})
	got := parseProps(out)
	if got[PropCreator] != "Ann; Bob" || got[PropDescription] != "A <quiet> day" {
		t.Fatalf("read back %v from\n%s", got, out)
	}
	again := withProps(out, map[Prop]string{PropCreator: ""})
	if strings.Contains(string(again), "dc:creator") || parseProps(again)[PropDescription] != "A <quiet> day" {
		t.Errorf("removing one property changed another:\n%s", again)
	}
	empty := withProps(again, map[Prop]string{PropDescription: ""})
	if strings.Contains(string(empty), ownPropsStart) {
		t.Errorf("an empty own description was left:\n%s", empty)
	}
}

func TestPropsLeaveTitleAndUnterlumenBlock(t *testing.T) {
	dir := t.TempDir()
	photo := filepath.Join(dir, "a.jpg")
	os.WriteFile(photo, []byte("x"), 0o644) //nolint:errcheck
	if err := WriteTitle(photo, "Harbour"); err != nil {
		t.Fatal(err)
	}
	if err := WriteField(photo, "place", "Kiel"); err != nil {
		t.Fatal(err)
	}
	if err := WriteSidecarProps(photo, map[Prop]string{PropRating: "2"}); err != nil {
		t.Fatal(err)
	}
	if err := WriteKeywords(photo, []string{"sea"}, nil); err != nil {
		t.Fatal(err)
	}
	notes, _ := ReadNotes(photo)
	props, _ := ReadSidecarProps(photo)
	kw, _ := ReadSidecarKeywords(photo)
	if notes.Title != "Harbour" || notes.Fields["place"] != "Kiel" || props[PropRating] != "2" || len(kw) != 1 {
		t.Errorf("notes %v, props %v, keywords %v", notes, props, kw)
	}
}

func TestNoSidecarIsWrittenToRemoveAProp(t *testing.T) {
	dir := t.TempDir()
	photo := filepath.Join(dir, "a.jpg")
	os.WriteFile(photo, []byte("x"), 0o644) //nolint:errcheck
	if err := WriteSidecarProps(photo, map[Prop]string{PropRating: ""}); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(SidecarPath(photo)); err == nil {
		t.Error("a sidecar was made to remove a property")
	}
}

func TestEmbeddedRatingIsRead(t *testing.T) {
	data := []byte("\xff\xd8junk<x:xmpmeta xmlns:x=\"adobe:ns:meta/\"><rdf:RDF xmlns:rdf=\"http://www.w3.org/1999/02/22-rdf-syntax-ns#\"><rdf:Description xmlns:xmp=\"http://ns.adobe.com/xap/1.0/\" xmp:Rating=\"4\"/></rdf:RDF></x:xmpmeta>more")
	if got := embeddedRating(bytes.NewReader(data)); got != "4" {
		t.Errorf("rating %q", got)
	}
	if got := embeddedRating(bytes.NewReader([]byte("no packet"))); got != "" {
		t.Errorf("rating %q from nothing", got)
	}
}
