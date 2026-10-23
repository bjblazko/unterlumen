package media

import (
	"slices"
	"strings"
	"testing"
)

const lightroomXMP = `<?xml version="1.0" encoding="UTF-8"?>
<x:xmpmeta xmlns:x="adobe:ns:meta/">
  <rdf:RDF xmlns:rdf="http://www.w3.org/1999/02/22-rdf-syntax-ns#">
    <rdf:Description rdf:about="" xmlns:xmp="http://ns.adobe.com/xap/1.0/" xmlns:dc="http://purl.org/dc/elements/1.1/" xmlns:lr="http://ns.adobe.com/lightroom/1.0/" xmp:Rating="4">
      <dc:title><rdf:Alt><rdf:li xml:lang="x-default">Rom</rdf:li></rdf:Alt></dc:title>
      <dc:subject><rdf:Bag><rdf:li>Urlaub</rdf:li></rdf:Bag></dc:subject>
      <lr:hierarchicalSubject><rdf:Bag><rdf:li>Orte|Rom</rdf:li></rdf:Bag></lr:hierarchicalSubject>
    </rdf:Description>
  </rdf:RDF>
</x:xmpmeta>`

func keepsLightroom(t *testing.T, out string) {
	t.Helper()
	for _, want := range []string{`xmp:Rating="4"`, "Urlaub", "Orte|Rom"} {
		if !strings.Contains(out, want) {
			t.Fatalf("%s was lost:\n%s", want, out)
		}
	}
}

// A title written into a sidecar where another program keeps everything in
// one description once replaced that whole description.
func TestTitleIsChangedInAnotherProgramsDescription(t *testing.T) {
	out := string(withTitle([]byte(lightroomXMP), "Rom bei Nacht"))
	keepsLightroom(t, out)
	if got, _ := parseDCTitle([]byte(out)); got != "Rom bei Nacht" || strings.Contains(out, ownDCStart) {
		t.Fatalf("title %q, own description written: %v\n%s", got, strings.Contains(out, ownDCStart), out)
	}
	out = string(withTitle([]byte(out), ""))
	keepsLightroom(t, out)
	if got, _ := parseDCTitle([]byte(out)); got != "" {
		t.Fatalf("the title %q stayed", got)
	}
}

func TestKeywordsGoIntoAnotherProgramsList(t *testing.T) {
	out := string(withSubjects([]byte(lightroomXMP), []string{"dog", "sea"}, nil))
	keepsLightroom(t, out)
	if got := Subjects([]byte(out)); !slices.Equal(got, []string{"Urlaub", "dog", "sea"}) {
		t.Fatalf("keywords %v\n%s", got, out)
	}
	if strings.Contains(out, ownDCStart) {
		t.Fatalf("a second list was written:\n%s", out)
	}
	out = string(withSubjects([]byte(out), nil, []string{"dog"}))
	if got := Subjects([]byte(out)); !slices.Equal(got, []string{"Urlaub", "sea"}) {
		t.Fatalf("keywords %v after removing dog", got)
	}
}

func TestOwnDescriptionHoldsTitleAndKeywordsTogether(t *testing.T) {
	out := withSubjects(nil, []string{"bird"}, nil)
	out = withTitle(out, "Möwe")
	if got := Subjects(out); !slices.Equal(got, []string{"bird"}) {
		t.Fatalf("the title write lost the keywords: %v\n%s", got, out)
	}
	out = withTitle(out, "")
	if got := Subjects(out); !slices.Equal(got, []string{"bird"}) {
		t.Fatalf("removing the title lost the keywords: %v", got)
	}
	out = withSubjects(out, nil, []string{"bird"})
	if strings.Contains(string(out), "rdf:Description") {
		t.Fatalf("an empty description stayed:\n%s", out)
	}
}

// Keywords Unterlumen kept before another program wrote its own list move
// into that list.
func TestOwnKeywordsMoveIntoAnotherProgramsList(t *testing.T) {
	own := string(withSubjects(nil, []string{"bird"}, nil))
	lr := strings.Replace(own, "</rdf:RDF>", `  <rdf:Description rdf:about="" xmlns:dc="http://purl.org/dc/elements/1.1/" xmlns:xmp="http://ns.adobe.com/xap/1.0/" xmp:Rating="2">
      <dc:subject><rdf:Bag><rdf:li>Urlaub</rdf:li></rdf:Bag></dc:subject>
    </rdf:Description>
  </rdf:RDF>`, 1)
	out := string(withSubjects([]byte(lr), []string{"sea"}, nil))
	if got := Subjects([]byte(out)); !slices.Equal(got, []string{"Urlaub", "bird", "sea"}) {
		t.Fatalf("keywords %v\n%s", got, out)
	}
	if strings.Count(out, "<dc:subject") != 1 {
		t.Fatalf("two lists:\n%s", out)
	}
}

// Sidecars written by earlier versions hold the title in a description of
// the same opening tag.
func TestEarlierOwnTitleIsFound(t *testing.T) {
	old := `<?xml version="1.0" encoding="UTF-8"?>
<x:xmpmeta xmlns:x="adobe:ns:meta/">
  <rdf:RDF xmlns:rdf="http://www.w3.org/1999/02/22-rdf-syntax-ns#">
    <rdf:Description rdf:about="" xmlns:dc="http://purl.org/dc/elements/1.1/">
      <dc:title>
        <rdf:Alt>
          <rdf:li xml:lang="x-default">Alt</rdf:li>
        </rdf:Alt>
      </dc:title>
    </rdf:Description>
  </rdf:RDF>
</x:xmpmeta>`
	out := string(withTitle([]byte(old), "Neu"))
	if got, _ := parseDCTitle([]byte(out)); got != "Neu" || strings.Count(out, "<dc:title") != 1 {
		t.Fatalf("title %q:\n%s", got, out)
	}
}
