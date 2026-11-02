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

var (
	testDateTaken   = NewProp("exif", "DateTimeOriginal", PropSimple)
	testDescription = NewProp("dc", "description", PropAlt)
	testCreator     = NewProp("dc", "creator", PropSeq)
	testRights      = NewProp("dc", "rights", PropAlt)
	testPerson      = NewProp("Iptc4xmpExt", "PersonInImage", PropBag)
	testCity        = NewProp("photoshop", "City", PropSimple)
	testProps       = []Prop{testDateTaken, PropRating, testDescription, testCreator, testRights, testPerson, testCity}
)

func TestPropsAreChangedWhereLightroomKeepsThem(t *testing.T) {
	out := string(withProps([]byte(lightroomPropsSidecar), testProps, map[Prop]string{
		PropRating:    "5",
		testRights:    "© 2026 Ann",
		testDateTaken: "2024-05-01T12:30:00+02:00",
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
	got := parseProps([]byte(out), testProps)
	if got[testDateTaken] != "2024-05-01T12:30:00+02:00" || got[PropRating] != "5" || got[testRights] != "© 2026 Ann" {
		t.Errorf("read back %v", got)
	}
}

func TestPropsInOwnDescriptionRoundTrip(t *testing.T) {
	out := withProps(nil, testProps, map[Prop]string{testCreator: "Ann; Bob", testDescription: "A <quiet> day"})
	got := parseProps(out, testProps)
	if got[testCreator] != "Ann; Bob" || got[testDescription] != "A <quiet> day" {
		t.Fatalf("read back %v from\n%s", got, out)
	}
	again := withProps(out, testProps, map[Prop]string{testCreator: ""})
	if strings.Contains(string(again), "dc:creator") || parseProps(again, testProps)[testDescription] != "A <quiet> day" {
		t.Errorf("removing one property changed another:\n%s", again)
	}
	empty := withProps(again, testProps, map[Prop]string{testDescription: ""})
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
	if err := WriteSidecarProps(photo, testProps, map[Prop]string{PropRating: "2"}); err != nil {
		t.Fatal(err)
	}
	if err := WriteKeywords(photo, []string{"sea"}, nil); err != nil {
		t.Fatal(err)
	}
	notes, _ := ReadNotes(photo)
	props, _ := ReadSidecarProps(photo, testProps)
	kw, _ := ReadSidecarKeywords(photo)
	if notes.Title != "Harbour" || notes.Fields["place"] != "Kiel" || props[PropRating] != "2" || len(kw) != 1 {
		t.Errorf("notes %v, props %v, keywords %v", notes, props, kw)
	}
}

func TestNoSidecarIsWrittenToRemoveAProp(t *testing.T) {
	dir := t.TempDir()
	photo := filepath.Join(dir, "a.jpg")
	os.WriteFile(photo, []byte("x"), 0o644) //nolint:errcheck
	if err := WriteSidecarProps(photo, testProps, map[Prop]string{PropRating: ""}); err != nil {
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

func TestBagAndIPTCPropsRoundTrip(t *testing.T) {
	out := withProps(nil, testProps, map[Prop]string{testPerson: "Ann; Bob", testCity: "Kiel"})
	if !strings.Contains(string(out), "<rdf:Bag><rdf:li>Ann</rdf:li><rdf:li>Bob</rdf:li></rdf:Bag>") {
		t.Errorf("person shown not written as a bag:\n%s", out)
	}
	if !strings.Contains(string(out), `xmlns:Iptc4xmpExt="http://iptc.org/std/Iptc4xmpExt/2008-02-29/"`) {
		t.Errorf("namespace not declared:\n%s", out)
	}
	got := parseProps(out, testProps)
	if got[testPerson] != "Ann; Bob" || got[testCity] != "Kiel" {
		t.Errorf("read back %v", got)
	}
}

func TestLegacyOwnDescriptionIsKeptOne(t *testing.T) {
	legacy := ensureXMP(nil)
	at := strings.LastIndex(legacy, "</rdf:RDF>")
	legacy = legacy[:at] + legacyOwnPropsStart + "<xmp:Rating>2</xmp:Rating>" + descEnd + legacy[at:]
	out := string(withProps([]byte(legacy), testProps, map[Prop]string{testCity: "Kiel"}))
	if strings.Contains(out, legacyOwnPropsStart) || strings.Count(out, ownPropsStart) != 1 {
		t.Fatalf("the old own description was not taken over:\n%s", out)
	}
	if got := parseProps([]byte(out), testProps); got[PropRating] != "2" || got[testCity] != "Kiel" {
		t.Errorf("read back %v", got)
	}
}
