package metadata

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"huepattl.de/unterlumen/internal/media"
)

// iptcValues has a value for every IPTC field, lists with two entries.
var iptcValues = map[Field]string{
	Sublocation: "Hafen", City: "Kiel", State: "Schleswig-Holstein", Country: "Deutschland", CountryCode: "DE",
	Headline: "Möwen am Kai", Event: "Kieler Woche", Credit: "Ann Beispiel", IPTCSource: "Archiv",
	UsageTerms: "Nur privat", WebStatement: "https://example.org/rights", Instructions: "Nicht beschneiden",
	JobID: "KW-2026", PersonShown: "Ann; Bob", OrganisationShown: "Hafenamt",
}

func TestEveryFieldIsInTheTableOnce(t *testing.T) {
	seen := map[Field]bool{}
	for _, s := range table {
		if seen[s.field] {
			t.Errorf("%s twice", s.field)
		}
		seen[s.field] = true
		if s.field != DateTaken && s.field != Title && (s.prop == (media.Prop{}) || len(s.file) == 0 || s.nfo == "" || s.tag == "") {
			t.Errorf("%s lacks a place: %+v", s.field, s)
		}
	}
	for f := range iptcValues {
		if !seen[f] {
			t.Errorf("%s not in the table", f)
		}
	}
}

func TestIPTCFieldsRoundTripInTheSidecar(t *testing.T) {
	photo := filepath.Join(t.TempDir(), "a.jpg")
	os.WriteFile(photo, []byte("x"), 0o644) //nolint:errcheck
	if err := Write(photo, TargetSidecar, Plan{Set: iptcValues}); err != nil {
		t.Fatal(err)
	}
	fields, _, from, err := readBeside(photo)
	if err != nil || from != FromSidecar {
		t.Fatal(from, err)
	}
	for f, want := range iptcValues {
		if fields[f] != want {
			t.Errorf("%s = %q, want %q", f, fields[f], want)
		}
	}
	data := Overlay(photo, nil)
	if data.Tags["City"] != "Kiel" || data.Tags["PersonInImage"] != "Ann; Bob" {
		t.Errorf("overlay tags %v", data.Tags)
	}
}

func TestIPTCFieldsRoundTripInTheNFO(t *testing.T) {
	out, err := withNFO([]byte(kodiNFO), iptcValues, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	s := string(out)
	for _, want := range []string{"<country>Deutschland</country>", "<tagline>Möwen am Kai</tagline>",
		"<actor><name>Ann</name></actor>", "<ul:city>Kiel</ul:city>", "<ul:organisation>Hafenamt</ul:organisation>"} {
		if !strings.Contains(s, want) {
			t.Errorf("missing %s in\n%s", want, s)
		}
	}
	fields, _, _ := parseNFO(out)
	for f, want := range iptcValues {
		if fields[f] != want {
			t.Errorf("%s = %q, want %q", f, fields[f], want)
		}
	}
}

func TestNFOActorKeepsItsRole(t *testing.T) {
	nfo := `<movie>
    <actor>
        <name>Ann</name>
        <role>Captain</role>
    </actor>
    <actor><name>Cid</name></actor>
</movie>`
	out, err := withNFO([]byte(nfo), map[Field]string{PersonShown: "Ann; Bob"}, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	s := string(out)
	if !strings.Contains(s, "<role>Captain</role>") || strings.Contains(s, "Cid") || !strings.Contains(s, "<name>Bob</name>") {
		t.Errorf("actors not changed by name:\n%s", s)
	}
}

func TestListFieldsAreAddedToAndRemovedFrom(t *testing.T) {
	cur := Current{Fields: map[Field]Value{PersonShown: {"Ann; Bob", FromSidecar}}}
	plan := PlanFor(Change{Add: map[Field][]string{PersonShown: {"Cid", "Ann"}}, Remove: map[Field][]string{PersonShown: {"Bob"}}}, cur)
	if plan.Set[PersonShown] != "Ann; Cid" {
		t.Errorf("plan %v", plan.Set)
	}
	same := PlanFor(Change{Add: map[Field][]string{PersonShown: {"Ann"}}}, cur)
	if _, ok := same.Set[PersonShown]; ok {
		t.Errorf("a list that stays was planned: %v", same.Set)
	}
}

func TestEmbeddedXMPIsReadWithoutExiftool(t *testing.T) {
	photo := filepath.Join(t.TempDir(), "a.jpg")
	packet := `<x:xmpmeta xmlns:x="adobe:ns:meta/"><rdf:RDF xmlns:rdf="http://www.w3.org/1999/02/22-rdf-syntax-ns#">
<rdf:Description xmlns:photoshop="http://ns.adobe.com/photoshop/1.0/" photoshop:City="Kiel">
<photoshop:Headline>Kai</photoshop:Headline></rdf:Description></rdf:RDF></x:xmpmeta>`
	os.WriteFile(photo, []byte("\xff\xd8"+packet), 0o644) //nolint:errcheck
	got := valuesFromMeta(photo)
	if got.fields[City] != "Kiel" || got.fields[Headline] != "Kai" {
		t.Errorf("fields %v", got.fields)
	}
	data := Overlay(photo, nil)
	if data == nil || data.Tags["City"] != "Kiel" || data.Tags[OverlaidTag] != "" {
		t.Errorf("the file's own XMP was not laid over: %+v", data)
	}
	os.WriteFile(media.SidecarPath(photo), []byte(""), 0o644) //nolint:errcheck
	if err := Write(photo, TargetSidecar, Plan{Set: map[Field]string{City: "Lübeck"}}); err != nil {
		t.Fatal(err)
	}
	if data := Overlay(photo, nil); data.Tags["City"] != "Lübeck" {
		t.Errorf("the sidecar did not win over the file's XMP: %v", data.Tags)
	}
}

// Every IPTC field into the file and out again, with exiftool itself.
func TestIPTCFieldsRoundTripInTheFile(t *testing.T) {
	if !media.CheckExiftool() {
		t.Skip("exiftool is not installed")
	}
	p := tinyJPEG(t, t.TempDir())
	if err := Write(p, TargetFile, Plan{Set: iptcValues}); err != nil {
		t.Fatal(err)
	}
	got, err := readFiles([]string{p})
	if err != nil {
		t.Fatal(err)
	}
	for f, want := range iptcValues {
		if got[p].fields[f] != want {
			t.Errorf("%s = %q, want %q", f, got[p].fields[f], want)
		}
	}
	iim, _ := runExiftool("-s3", "-IPTC:City", "-IPTC:Headline", p)
	if !bytes.Contains(iim, []byte("Kiel")) || !bytes.Contains(iim, []byte("Möwen am Kai")) {
		t.Errorf("IPTC-IIM not written: %s", iim)
	}
	if list, _ := runExiftool("-j", "-XMP-iptcExt:PersonInImage", p); !bytes.Contains(list, []byte(`["Ann","Bob"]`)) {
		t.Errorf("person shown not written as two entries: %s", list)
	}
}

func TestCountryComesFromTheGPSPosition(t *testing.T) {
	kiel := Current{Fields: map[Field]Value{}, GPS: &GPS{54.3233, 10.1228}}
	plan := PlanFor(Change{CountryFromGPS: &CountryFromGPS{Lang: "de"}}, kiel)
	if plan.Set[Country] != "Deutschland" || plan.Set[CountryCode] != "DE" || plan.Skipped != "" {
		t.Errorf("plan %+v", plan)
	}
	kiel.Fields[Country] = Value{"Deutschland", FromSidecar}
	kiel.Fields[CountryCode] = Value{"DE", FromSidecar}
	if plan := PlanFor(Change{CountryFromGPS: &CountryFromGPS{Lang: "de"}}, kiel); len(plan.Set) != 0 || plan.Skipped != "" {
		t.Errorf("a country already right was planned again: %+v", plan)
	}
	if plan := PlanFor(Change{CountryFromGPS: &CountryFromGPS{Lang: "en"}}, Current{}); plan.Skipped != "no GPS position" {
		t.Errorf("a file without GPS: %+v", plan)
	}
	sea := Current{GPS: &GPS{0, -30}}
	if plan := PlanFor(Change{CountryFromGPS: &CountryFromGPS{Lang: "en"}}, sea); plan.Skipped != "no country at its position" {
		t.Errorf("the open sea: %+v", plan)
	}
}

// The overlay says which values lie beside the file, and what the file
// itself says under them.
func TestOverlaySaysWhereEachValueLies(t *testing.T) {
	photo := filepath.Join(t.TempDir(), "a.jpg")
	packet := `<x:xmpmeta xmlns:x="adobe:ns:meta/"><rdf:RDF xmlns:rdf="http://www.w3.org/1999/02/22-rdf-syntax-ns#">
<rdf:Description xmlns:photoshop="http://ns.adobe.com/photoshop/1.0/" photoshop:City="Kiel" photoshop:Country="Deutschland"/></rdf:RDF></x:xmpmeta>`
	os.WriteFile(photo, []byte("\xff\xd8"+packet), 0o644) //nolint:errcheck
	plan := Plan{Set: map[Field]string{City: "Lübeck", Title: "Am Kai"}}
	if err := Write(photo, TargetSidecar, plan); err != nil {
		t.Fatal(err)
	}
	data := Overlay(photo, &media.ExifData{Tags: map[string]string{"Artist": `"Ann"`}})
	if data.Beside["City"] != "sidecar" || data.Beside["Title"] != "sidecar" || data.Beside["Country"] != "" || data.Beside["Artist"] != "" {
		t.Errorf("beside %v", data.Beside)
	}
	if data.Shadowed["City"] != "Kiel" || len(data.Shadowed) != 1 {
		t.Errorf("shadowed %v", data.Shadowed)
	}
	if data.Tags["Title"] != "Am Kai" || data.Tags["Country"] != "Deutschland" {
		t.Errorf("tags %v", data.Tags)
	}
}
