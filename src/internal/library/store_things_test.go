package library

import (
	"context"
	"errors"
	"image/color"
	"os"
	"path/filepath"
	"slices"
	"testing"

	"huepattl.de/unterlumen/internal/media"
	"huepattl.de/unterlumen/internal/things"
)

func thingNamesOf(t *testing.T, s *Store, id string) []string {
	t.Helper()
	found, err := s.ThingsOf(id)
	if err != nil {
		t.Fatal(err)
	}
	var names []string
	for _, f := range found {
		names = append(names, f.Name)
	}
	slices.Sort(names)
	return names
}

func TestSidecarThingsReplaceTheIndex(t *testing.T) {
	s := newTestStore(t)
	insertPhoto(t, s, "p1", nil, nil)
	if err := s.SaveFoundThings([]FoundThings{{PhotoID: "p1", Written: true, Things: []things.Found{
		{Name: "dog", Score: 0.3, Source: "siglip"}, {Name: "tree", Score: 0.27, Source: "siglip"}}}}); err != nil {
		t.Fatal(err)
	}
	// tree's keyword was removed in another program, sea came from the other installation
	st := media.SidecarThings{Record: media.ThingsRecord{Found: []string{"dog", "tree", "sea"}, Version: things.RecordVersion()},
		Keywords: []string{"dog", "sea", "Urlaub"}}
	if err := s.ApplySidecarThings("p1", st); err != nil {
		t.Fatal(err)
	}
	found, _ := s.ThingsOf("p1")
	if len(found) != 2 || found[0].Name != "dog" || found[0].Score != 0.3 || found[1].Source != "sidecar" {
		t.Fatalf("got %+v", found)
	}
}

// A sidecar that could not be written keeps the pass's things; one that
// held the record and lost it has the photo looked at again.
func TestASidecarWithoutTheRecord(t *testing.T) {
	s := newTestStore(t)
	insertPhoto(t, s, "kept", nil, nil)
	insertPhoto(t, s, "lost", nil, nil)
	s.SaveFoundThings([]FoundThings{ //nolint:errcheck
		{PhotoID: "kept", Things: []things.Found{{Name: "dog", Source: "siglip"}}},
		{PhotoID: "lost", Things: []things.Found{{Name: "dog", Source: "siglip"}}, Written: true},
	})
	for _, id := range []string{"kept", "lost"} {
		if err := s.ApplySidecarThings(id, media.SidecarThings{}); err != nil {
			t.Fatal(err)
		}
	}
	if got := thingNamesOf(t, s, "kept"); !slices.Equal(got, []string{"dog"}) {
		t.Fatalf("kept has %v", got)
	}
	var again int
	s.db.QueryRow(`SELECT COUNT(*) FROM photo_things_found WHERE photo_id = 'lost'`).Scan(&again) //nolint:errcheck
	if again != 0 {
		t.Fatal("the photo whose sidecar lost the record is not looked at again")
	}
}

// wordsFor makes a word list where each thing is one axis, so a photo
// vector along an axis is that thing and nothing else.
func wordsFor() [][]float32 {
	words := make([][]float32, len(things.Vocabulary))
	for i := range words {
		words[i] = make([]float32, len(things.Vocabulary))
		words[i][i] = 1
	}
	return words
}

func vectorOf(name string) []float32 {
	v := make([]float32, len(things.Vocabulary))
	for i, t := range things.Vocabulary {
		if t.Name == name {
			v[i] = 1
		}
	}
	return v
}

// photoWithOldWords is a photo looked at by an older word list: the pass
// scores its stored vector again without the models.
func photoWithOldWords(t *testing.T, s *Store, libDir, id, path, thing string) {
	t.Helper()
	addPhotoWithThumb(t, s, libDir, id, path, color.RGBA{})
	if err := s.SaveFoundThings([]FoundThings{{PhotoID: id, Vector: vectorOf(thing)}}); err != nil {
		t.Fatal(err)
	}
	s.db.Exec(`UPDATE photo_things_found SET vocabulary = 0 WHERE photo_id = ?`, id) //nolint:errcheck
}

func runThingsPass(t *testing.T, s *Store, libDir string) {
	t.Helper()
	ch := make(chan Progress, 8)
	look := &thingsLook{words: wordsFor()}
	go NewIndexer(s, libDir, "").RunFindThingsMissing(context.Background(), func() (*thingsLook, error) { return look, nil }, ch)
	for p := range ch {
		if p.Error != "" {
			t.Fatal(p.Error)
		}
	}
}

func TestThingsPassWritesTheSidecarThenTheIndex(t *testing.T) {
	s := newTestStore(t)
	libDir, photos := t.TempDir(), t.TempDir()
	photo := filepath.Join(photos, "a.jpg")
	photoWithOldWords(t, s, libDir, "aa11", photo, "lighthouse")
	runThingsPass(t, s, libDir)
	st, _ := media.ReadThings(photo)
	if !slices.Equal(st.Things(things.LanguageOf("en")), []string{"lighthouse"}) || !slices.Equal(thingNamesOf(t, s, "aa11"), []string{"lighthouse"}) {
		t.Fatalf("sidecar %+v, index %v", st, thingNamesOf(t, s, "aa11"))
	}
	if tasks, _ := s.PhotosNeedingThings(); len(tasks) != 0 {
		t.Fatalf("%d photos still to look at", len(tasks))
	}
}

// What the other installation's current models wrote is taken over, and a
// thing the owner took away is not written again.
func TestThingsPassTakesOverACurrentSidecar(t *testing.T) {
	s := newTestStore(t)
	libDir, photos := t.TempDir(), t.TempDir()
	other, removed := filepath.Join(photos, "b.jpg"), filepath.Join(photos, "c.jpg")
	os.WriteFile(other, nil, 0o644)                                                                           //nolint:errcheck
	os.WriteFile(removed, nil, 0o644)                                                                         //nolint:errcheck
	media.WriteFoundThings(other, []string{"windmill"}, things.RecordVersion(), nil, things.LanguageOf("en")) //nolint:errcheck
	media.WriteFoundThings(removed, []string{"lighthouse"}, "0.0", nil, things.LanguageOf("en"))              //nolint:errcheck
	media.RemoveThing(removed, "lighthouse", things.LanguageOf("en"))                                         //nolint:errcheck
	photoWithOldWords(t, s, libDir, "bb22", other, "lighthouse")
	photoWithOldWords(t, s, libDir, "cc33", removed, "lighthouse")
	runThingsPass(t, s, libDir)
	if got := thingNamesOf(t, s, "bb22"); !slices.Equal(got, []string{"windmill"}) {
		t.Fatalf("the other installation's things were not taken over: %v", got)
	}
	st, _ := media.ReadThings(removed)
	if len(thingNamesOf(t, s, "cc33")) != 0 || slices.Contains(st.Keywords, "lighthouse") {
		t.Fatalf("a removed thing came back: index %v, sidecar %+v", thingNamesOf(t, s, "cc33"), st)
	}
}

// The Things topic counts only the photos the shared filter lets through.
func TestThingAppearancesFollowTheFilter(t *testing.T) {
	s := newTestStore(t)
	for _, id := range []string{"p1", "p2"} {
		insertPhoto(t, s, id, nil, nil)
	}
	s.SaveFoundThings([]FoundThings{ //nolint:errcheck
		{PhotoID: "p1", Things: []things.Found{{Name: "dog", Source: "siglip"}, {Name: "sea", Source: "siglip"}}},
		{PhotoID: "p2", Things: []things.Found{{Name: "sea", Source: "siglip"}}},
	})
	s.filter = Filter{Things: []string{"dog"}}
	apps, err := s.ThingAppearances("")
	if err != nil {
		t.Fatal(err)
	}
	if len(apps) != 2 || apps[0].Photo != "p1" || apps[1].Photo != "p1" {
		t.Fatalf("got %+v", apps)
	}
	if n, _ := s.ThingsLooked(""); n != 1 {
		t.Fatalf("%d photos looked at in scope, want 1", n)
	}
}

// Photos found by words come best match first, whatever their dates.
func TestWordMatchesComeBestFirst(t *testing.T) {
	s := newTestStore(t)
	for _, id := range []string{"old", "new", "other"} {
		insertPhoto(t, s, id, nil, nil)
	}
	page, err := s.ListPhotos(context.Background(), ListPhotosOpts{Words: "dog", Limit: 10,
		matches: map[string]float32{"old": 0.31, "new": 0.27}})
	if err != nil {
		t.Fatal(err)
	}
	if page.Total != 2 || page.Photos[0].ID != "old" || page.Photos[0].Match != 0.31 {
		t.Fatalf("got %+v", page)
	}
}

// A check whose walk began before a things pass wrote a sidecar must not
// take the record as lost: the sidecar is there, on the disk.
func TestASidecarWrittenAfterTheWalkKeepsItsRecord(t *testing.T) {
	s := newTestStore(t)
	libDir, photos := t.TempDir(), t.TempDir()
	photo := filepath.Join(photos, "late.jpg")
	os.WriteFile(photo, nil, 0o644) //nolint:errcheck
	insertPhoto(t, s, "late", nil, nil)
	s.SaveFoundThings([]FoundThings{{PhotoID: "late", Written: true, Things: []things.Found{{Name: "dog", Source: "siglip"}}}}) //nolint:errcheck
	media.WriteFoundThings(photo, []string{"dog"}, things.RecordVersion(), nil, things.LanguageOf("en"))                        //nolint:errcheck
	idx := NewIndexer(s, libDir, photos)
	idx.sidecars = map[string]string{} // the walk saw no sidecar
	idx.indexSidecar(photo, "late")
	if !s.thingsWritten("late") || !slices.Equal(thingNamesOf(t, s, "late"), []string{"dog"}) {
		t.Fatal("a sidecar written after the walk was taken as lost")
	}
	gone := filepath.Join(photos, "gone.jpg")
	insertPhoto(t, s, "gone", nil, nil)
	s.SaveFoundThings([]FoundThings{{PhotoID: "gone", Written: true}}) //nolint:errcheck
	idx.indexSidecar(gone, "gone")
	if s.thingsWritten("gone") {
		t.Fatal("a sidecar that is really gone kept its record")
	}
}

func noModels() (*thingsLook, error) { return nil, errors.New("the models were loaded") }

// What the other installation found and saw is taken over, vector included,
// and the models are never loaded for it.
func TestAPassTakesOverThingsAndVectorWithoutModels(t *testing.T) {
	s := newTestStore(t)
	libDir, photos := t.TempDir(), t.TempDir()
	photo := filepath.Join(photos, "seen.jpg")
	addPhotoWithThumb(t, s, libDir, "ee55", photo, color.RGBA{})
	media.WriteFoundThings(photo, []string{"windmill"}, things.RecordVersion(), vectorOf("windmill"), things.LanguageOf("en")) //nolint:errcheck
	ch := make(chan Progress, 8)
	go NewIndexer(s, libDir, "").RunFindThingsMissing(context.Background(), noModels, ch)
	for range ch {
	}
	if got := thingNamesOf(t, s, "ee55"); !slices.Equal(got, []string{"windmill"}) {
		t.Fatalf("things %v", got)
	}
	var n int
	s.db.QueryRow(`SELECT COUNT(*) FROM photo_vectors WHERE photo_id = 'ee55'`).Scan(&n) //nolint:errcheck
	if n != 1 {
		t.Fatal("the sidecar's vector was not taken")
	}
	if tasks, _ := s.PhotosNeedingThings(); len(tasks) != 0 {
		t.Fatalf("%d photos still to look at", len(tasks))
	}
}

// A sidecar written before vectors travelled gets this index's vector, by a
// pass that runs no model.
func TestAPassWritesItsVectorIntoASidecarWithout(t *testing.T) {
	s := newTestStore(t)
	libDir, photos := t.TempDir(), t.TempDir()
	photo := filepath.Join(photos, "old.jpg")
	addPhotoWithThumb(t, s, libDir, "ff66", photo, color.RGBA{})
	media.WriteFoundThings(photo, []string{"lighthouse"}, things.RecordVersion(), nil, things.LanguageOf("en")) //nolint:errcheck
	s.SaveFoundThings([]FoundThings{{PhotoID: "ff66", Vector: vectorOf("lighthouse"), Written: true,            //nolint:errcheck
		Things: []things.Found{{Name: "lighthouse", Source: "siglip"}}}})
	ch := make(chan Progress, 8)
	go NewIndexer(s, libDir, "").RunFindThingsMissing(context.Background(), noModels, ch)
	for range ch {
	}
	st, _ := media.ReadThings(photo)
	if len(st.Record.Vector) != len(things.Vocabulary) {
		t.Fatalf("the sidecar has %d numbers", len(st.Record.Vector))
	}
	if tasks, _ := s.PhotosNeedingThings(); len(tasks) != 0 {
		t.Fatalf("%d photos still to look at", len(tasks))
	}
}

// A scan takes the vector from the sidecar when the index has none.
func TestAScanTakesTheSidecarsVector(t *testing.T) {
	s := newTestStore(t)
	insertPhoto(t, s, "gg77", nil, nil)
	st := media.SidecarThings{Record: media.ThingsRecord{Found: []string{"dog"}, Version: things.RecordVersion(), Vector: []float32{0.6, 0.8}},
		Keywords: []string{"dog"}}
	if err := s.ApplySidecarThings("gg77", st); err != nil {
		t.Fatal(err)
	}
	var n int
	s.db.QueryRow(`SELECT COUNT(*) FROM photo_vectors WHERE photo_id = 'gg77'`).Scan(&n) //nolint:errcheck
	if n != 1 {
		t.Fatal("the sidecar's vector was not taken")
	}
}
