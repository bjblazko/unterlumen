package library

import "testing"

// annaAndAFaceLikeHers stores three faces of one person and a stranger,
// names two of the three Anna and returns her and the third face.
func annaAndAFaceLikeHers(t *testing.T, s *Store) (Person, int64) {
	t.Helper()
	photosWithFaces(t, s, t.TempDir(), 0, 0, 0, 1)
	var hers []FaceRef
	for _, g := range groups(t, s) {
		if g.Size == 3 {
			hers, _ = s.GroupFaces(g.ID)
		}
	}
	if _, err := s.NameFaces([]int64{hers[0].ID, hers[1].ID}, "Anna"); err != nil {
		t.Fatal(err)
	}
	people, _ := s.People()
	return people[0], hers[2].ID
}

func suggestionFor(t *testing.T, s *Store, face int64) *Person {
	t.Helper()
	var photo string
	s.db.QueryRow(`SELECT photo_id FROM faces WHERE id = ?`, face).Scan(&photo) //nolint:errcheck
	found, err := s.PhotoFaces(photo)
	if err != nil {
		t.Fatal(err)
	}
	return found[0].Suggestion
}

func TestAFaceLikeANamedPersonsIsSuggestedAsThem(t *testing.T) {
	s := newTestStore(t)
	anna, third := annaAndAFaceLikeHers(t, s)
	if p := suggestionFor(t, s, third); p == nil || p.ID != anna.ID {
		t.Fatalf("the face is suggested as %+v, want Anna", p)
	}
	var suggested, strangers int
	for _, g := range groups(t, s) {
		switch {
		case g.Suggestion != nil && g.Suggestion.ID == anna.ID:
			suggested++
		case g.Suggestion == nil:
			strangers++
		}
	}
	if suggested != 1 || strangers != 1 {
		t.Errorf("%d groups suggested as Anna, %d without a suggestion; want one each", suggested, strangers)
	}
	maybe, err := s.MaybeFaces(anna.ID)
	if err != nil || len(maybe) != 1 || maybe[0].ID != third {
		t.Fatalf("maybe Anna: %+v, %v; want the third face", maybe, err)
	}
}

func TestANoKeepsASuggestionFromComingBack(t *testing.T) {
	s := newTestStore(t)
	anna, third := annaAndAFaceLikeHers(t, s)
	if err := s.NotThem([]int64{third}, anna.ID); err != nil {
		t.Fatal(err)
	}
	if p := suggestionFor(t, s, third); p != nil {
		t.Errorf("after No the face is still suggested as %s", p.Name)
	}
	for _, g := range groups(t, s) {
		if g.Suggestion != nil {
			t.Errorf("after No a group is still suggested as %s", g.Suggestion.Name)
		}
	}
	if maybe, _ := s.MaybeFaces(anna.ID); len(maybe) != 0 {
		t.Errorf("after No, maybe Anna still lists %+v", maybe)
	}
}

// "This is not Anna" is a No too; naming the face Anna after all takes the
// No back.
func TestTakingANameAwayIsANo(t *testing.T) {
	s := newTestStore(t)
	anna, third := annaAndAFaceLikeHers(t, s)
	s.NameFaces([]int64{third}, "Anna") //nolint:errcheck
	if _, err := s.UnnameFaces([]int64{third}); err != nil {
		t.Fatal(err)
	}
	if p := suggestionFor(t, s, third); p != nil {
		t.Errorf("a face taken from Anna is suggested as %s again", p.Name)
	}
	s.NameFaces([]int64{third}, "Anna") //nolint:errcheck
	var nos int
	s.db.QueryRow(`SELECT COUNT(*) FROM face_not WHERE face = ? AND person = ?`, third, anna.ID).Scan(&nos) //nolint:errcheck
	if nos != 0 {
		t.Errorf("named Anna after all, the face keeps %d No for her", nos)
	}
}

func TestHiddenFacesAreListedAndShownAgain(t *testing.T) {
	s := newTestStore(t)
	photosWithFaces(t, s, t.TempDir(), 0, 1)
	g := groups(t, s)
	faces, _ := s.GroupFaces(g[0].ID)
	s.HideFaces([]int64{faces[0].ID}) //nolint:errcheck
	if n, _ := s.HiddenCount(); n != 1 {
		t.Fatalf("%d hidden, want 1", n)
	}
	hidden, err := s.HiddenFaces()
	if err != nil || len(hidden) != 1 || hidden[0].ID != faces[0].ID {
		t.Fatalf("hidden %+v, %v", hidden, err)
	}
	if err := s.ShowAgain([]int64{faces[0].ID}); err != nil {
		t.Fatal(err)
	}
	if n, _ := s.HiddenCount(); n != 0 {
		t.Errorf("%d still hidden", n)
	}
	if got := groups(t, s); len(got) != 2 {
		t.Errorf("%d groups, want the face back in one of its own", len(got))
	}
}
