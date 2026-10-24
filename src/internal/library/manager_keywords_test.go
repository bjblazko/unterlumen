package library

import (
	"path/filepath"
	"slices"
	"testing"
	"time"

	"huepattl.de/unterlumen/internal/media"
	"huepattl.de/unterlumen/internal/things"
)

// A library set to German writes the keywords Unterlumen found in German, in
// place of the English ones; the owner's keywords and the index's IDs stay.
func TestALibrarysKeywordsAreRewrittenInItsLanguage(t *testing.T) {
	src := t.TempDir()
	photo := filepath.Join(src, "IMG_1.JPG")
	writePhoto(t, photo, "pixels", day)
	media.WriteField(photo, "Ort", "Rom") //nolint:errcheck
	m, err := NewManager(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	lib, err := m.CreateLibrary("Rom", "", src)
	if err != nil {
		t.Fatal(err)
	}
	store, _ := m.OpenStore(lib.ID)
	drain(NewIndexer(store, m.LibDir(lib.ID), src).RunScanNew)
	id := onlyID(t, store)
	media.WriteFoundThings(photo, []string{"dog", "church"}, things.RecordVersion(), nil, things.LanguageOf("en")) //nolint:errcheck
	store.SaveFoundThings([]FoundThings{{PhotoID: id, Written: true,                                               //nolint:errcheck
		Things: []things.Found{{Name: "dog", Source: "siglip"}, {Name: "church", Source: "siglip"}}}})

	l, err := m.SetKeywordLanguage(lib.ID, "de")
	if err != nil || l.KeywordLanguage != "de" {
		t.Fatalf("language %q, %v", l.KeywordLanguage, err)
	}
	deadline := time.Now().Add(5 * time.Second)
	for !m.IsScanning(lib.ID) && time.Now().Before(deadline) {
		time.Sleep(10 * time.Millisecond)
	}
	for m.IsScanning(lib.ID) && time.Now().Before(deadline) {
		time.Sleep(10 * time.Millisecond)
	}
	st, _ := media.ReadThings(photo)
	if !slices.Equal(st.Keywords, []string{"Hund", "Kirche"}) {
		t.Fatalf("keywords %v", st.Keywords)
	}
	notes, _ := media.ReadNotes(photo)
	if notes.Fields["Ort"] != "Rom" || !slices.Equal(thingNamesOf(t, store, id), []string{"church", "dog"}) {
		t.Fatalf("fields %v, things %v", notes.Fields, thingNamesOf(t, store, id))
	}
	if _, err := m.SetKeywordLanguage(lib.ID, "xx"); err == nil {
		t.Fatal("a language without words was taken")
	}
}
