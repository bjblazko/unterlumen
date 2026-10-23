package library

import (
	"database/sql"
	"sort"
	"sync"

	"huepattl.de/unterlumen/internal/faces"
)

// Suggestions (ADR-0051, decision 13): a face, or a group of them, is
// suggested as the named person whose nearest faces it looks most like
// (faces.Resemblance), never as someone the owner said it is not
// (face_not). Every face named makes the next suggestions better; every No
// keeps one from coming back. Nothing is named without the owner.

// lookalikes are the named people of a library with the prints of their
// confirmed faces; faces marked by hand have none.
type lookalikes struct {
	people  []Person
	prints  [][][]float32 // per person, in the order of people
	centres [][]float32   // the average of each person's prints, to choose whom to compare closely
	at      map[int64]int // person ID → index
}

// Comparing a face with every face of everyone takes seconds on a library
// of ten thousand named faces. The average of a person's faces says
// cheaply who comes into question; only those are compared face by face.
const (
	closelyCompared = 5    // people, the nearest by their average
	worthComparing  = 0.25 // how near a person's average a face must be to be one of theirs
)

// best is the person e looks most like, if enough to suggest, leaving out
// those not (person IDs) holds.
func (l *lookalikes) best(e []float32, not map[int64]bool) (*Person, float32) {
	at, bestSim := -1, float32(-2)
	for _, i := range l.nearestByAverage(e, not) {
		if sim := faces.Resemblance(e, l.prints[i]); sim > bestSim {
			at, bestSim = i, sim
		}
	}
	if at < 0 || bestSim < faces.SuggestThreshold {
		return nil, bestSim
	}
	return &l.people[at], bestSim
}

// nearestByAverage is the closelyCompared people whose average is nearest e.
func (l *lookalikes) nearestByAverage(e []float32, not map[int64]bool) []int {
	type near struct {
		i   int
		sim float32
	}
	var all []near
	for i, p := range l.people {
		if !not[p.ID] && l.centres[i] != nil {
			all = append(all, near{i, faces.Similarity(e, l.centres[i])})
		}
	}
	sort.Slice(all, func(a, b int) bool { return all[a].sim > all[b].sim })
	out := make([]int, 0, closelyCompared)
	for k := 0; k < len(all) && k < closelyCompared; k++ {
		out = append(out, all[k].i)
	}
	return out
}

// lookalikesCache keeps a library's prints between requests — the viewer
// asks for every photo — until its confirmed faces change, which the key
// tells. A Store is made per request, so the cache goes with the library's
// one connection.
type lookalikesCache struct {
	mu  sync.Mutex
	key string
	val *lookalikes
}

var lookalikesByDB sync.Map // *sql.DB → *lookalikesCache

// forgetLookalikes lets go of the prints of a connection that is closed.
func forgetLookalikes(db *sql.DB) { lookalikesByDB.Delete(db) }

func (s *Store) lookalikes() (*lookalikes, error) {
	var key string
	if err := s.db.QueryRow(`SELECT COUNT(*) || ':' || COALESCE(SUM(person_id), 0) || ':' || COALESCE(MAX(id), 0)
		FROM faces WHERE person_id IS NOT NULL AND confirmed = 1`).Scan(&key); err != nil {
		return nil, err
	}
	v, _ := lookalikesByDB.LoadOrStore(s.db, &lookalikesCache{})
	c := v.(*lookalikesCache)
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.val != nil && c.key == key {
		return c.val, nil
	}
	l, err := s.readLookalikes()
	if err != nil {
		return nil, err
	}
	c.key, c.val = key, l
	return l, nil
}

func (s *Store) readLookalikes() (*lookalikes, error) {
	people, err := s.People()
	if err != nil {
		return nil, err
	}
	l := &lookalikes{people: people, prints: make([][][]float32, len(people)), centres: make([][]float32, len(people)), at: map[int64]int{}}
	for i, p := range people {
		l.at[p.ID] = i
	}
	rows, err := s.db.Query(`SELECT person_id, embedding FROM faces
		WHERE person_id IS NOT NULL AND confirmed = 1 AND manual = 0`)
	if err != nil {
		return nil, err
	}
	err = scanRows(rows, func() error {
		var person int64
		var blob []byte
		if err := rows.Scan(&person, &blob); err != nil {
			return err
		}
		if i, ok := l.at[person]; ok && len(blob) > 0 {
			l.prints[i] = append(l.prints[i], decodeEmbedding(blob))
		}
		return nil
	})
	for i, p := range l.prints {
		l.centres[i] = faces.Mean(p)
	}
	return l, err
}

// --- Saying No ---

// NotThem records that the faces are not the person: a suggestion of them
// for these faces does not come back. It stays in this index, as hiding
// does: the sidecar says only who is in a photo.
func (s *Store) NotThem(ids []int64, person int64) error {
	if len(ids) == 0 {
		return nil
	}
	var name string
	if err := s.db.QueryRow(`SELECT name FROM people WHERE id = ?`, person).Scan(&name); err != nil {
		return err
	}
	if err := s.decideNot(ids, sameName(ids, name), true); err != nil {
		return err
	}
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback() //nolint:errcheck
	for _, id := range ids {
		if _, err := tx.Exec(`INSERT OR IGNORE INTO face_not (face, person)
			SELECT ?, id FROM people WHERE id = ?`, id, person); err != nil {
			return err
		}
	}
	return tx.Commit()
}

// rememberNot records, for faces about to lose their person, that they are
// not that person: "This is not Ben" and "Is instead" are a No too.
func (s *Store) rememberNot(ids []int64) error {
	if len(ids) == 0 {
		return nil
	}
	names, err := s.personNamesOf(ids)
	if err != nil {
		return err
	}
	if err := s.decideNot(ids, names, true); err != nil {
		return err
	}
	args := make([]any, len(ids))
	for i, id := range ids {
		args[i] = id
	}
	_, err = s.db.Exec(`INSERT OR IGNORE INTO face_not (face, person)
		SELECT id, person_id FROM faces WHERE id IN (`+placeholders(len(ids))+`)
		AND person_id IS NOT NULL AND confirmed = 1 AND manual = 0`, args...)
	return err
}

// forgetNot takes back a No for faces that were named the person after all.
func (s *Store) forgetNot(ids []int64, name string) error {
	if len(ids) == 0 {
		return nil
	}
	if err := s.decideNot(ids, sameName(ids, name), false); err != nil {
		return err
	}
	args := []any{name}
	for _, id := range ids {
		args = append(args, id)
	}
	_, err := s.db.Exec(`DELETE FROM face_not WHERE person = (SELECT id FROM people WHERE name = ?)
		AND face IN (`+placeholders(len(ids))+`)`, args...)
	return err
}

// notFor lists, for each of the faces the query selects (its first column),
// the people they are not.
func (s *Store) notFor(query string, args ...any) (map[int64]bool, error) {
	rows, err := s.db.Query(`SELECT DISTINCT n.person FROM face_not n WHERE n.face IN (`+query+`)`, args...)
	if err != nil {
		return nil, err
	}
	not := map[int64]bool{}
	err = scanRows(rows, func() error {
		var p int64
		if err := rows.Scan(&p); err != nil {
			return err
		}
		not[p] = true
		return nil
	})
	return not, err
}

// --- Suggestions ---

// suggestGroup is who a group of unnamed faces looks like, if anyone.
func (s *Store) suggestGroup(l *lookalikes, grp int64) (*Person, error) {
	_, embs, err := s.embeddings(`SELECT id, embedding FROM faces WHERE grp = ? AND `+unnamed, grp)
	if err != nil || len(embs) == 0 {
		return nil, err
	}
	not, err := s.notFor(`SELECT id FROM faces WHERE grp = ? AND `+unnamed, grp)
	if err != nil {
		return nil, err
	}
	p, _ := l.best(faces.Mean(embs), not)
	return p, nil
}

// GroupSuggestion is who a group of unnamed faces looks like, if anyone:
// for a group's own page, which may be far down the list.
func (s *Store) GroupSuggestion(grp int64) (*Person, error) {
	l, err := s.lookalikes()
	if err != nil {
		return nil, err
	}
	return s.suggestGroup(l, grp)
}

// suggestFace is who one unnamed face looks like, if anyone.
func (s *Store) suggestFace(l *lookalikes, id int64) (*Person, error) {
	_, embs, err := s.embeddings(`SELECT id, embedding FROM faces WHERE id = ?`, id)
	if err != nil || len(embs) == 0 {
		return nil, err
	}
	not, err := s.notFor(`SELECT ?`, id)
	if err != nil {
		return nil, err
	}
	p, _ := l.best(embs[0], not)
	return p, nil
}

// maybeShown is how many faces "Maybe Anna" shows at most, the likeliest first.
const maybeShown = 60

// MaybeFaces lists the unnamed faces that look most like the person of
// all the people named here, and enough to be suggested, the likeliest
// first — less the ones said not to be them.
func (s *Store) MaybeFaces(person int64) ([]FaceRef, error) {
	l, err := s.lookalikes()
	if err != nil {
		return nil, err
	}
	at, ok := l.at[person]
	if !ok || len(l.prints[at]) == 0 {
		return []FaceRef{}, nil
	}
	ids, embs, err := s.embeddings(`SELECT id, embedding FROM faces WHERE ` + unnamed)
	if err != nil {
		return nil, err
	}
	nos, err := s.allNot()
	if err != nil {
		return nil, err
	}
	return s.faceRefsInOrder(maybeHits(l, at, ids, embs, nos))
}

// maybeHits are the faces, of ids and their prints embs, that look most
// like the person at at of everyone, and enough, the likeliest first, at
// most maybeShown; a face's No's (nos) leave those people out.
func maybeHits(l *lookalikes, at int, ids []int64, embs [][]float32, nos map[int64]map[int64]bool) []int64 {
	type hit struct {
		id  int64
		sim float32
	}
	var hits []hit
	for i, e := range embs {
		// Them first, which is cheap; the others only for faces near them.
		if faces.Similarity(e, l.centres[at]) < worthComparing || faces.Resemblance(e, l.prints[at]) < faces.SuggestThreshold {
			continue
		}
		if p, sim := l.best(e, nos[ids[i]]); p != nil && p.ID == l.people[at].ID {
			hits = append(hits, hit{ids[i], sim})
		}
	}
	sort.SliceStable(hits, func(a, b int) bool { return hits[a].sim > hits[b].sim })
	out := make([]int64, 0, min(len(hits), maybeShown))
	for k := 0; k < len(hits) && k < maybeShown; k++ {
		out = append(out, hits[k].id)
	}
	return out
}

// allNot is every No of the library: face → the people it is not.
func (s *Store) allNot() (map[int64]map[int64]bool, error) {
	rows, err := s.db.Query(`SELECT face, person FROM face_not`)
	if err != nil {
		return nil, err
	}
	out := map[int64]map[int64]bool{}
	err = scanRows(rows, func() error {
		var f, p int64
		if err := rows.Scan(&f, &p); err != nil {
			return err
		}
		if out[f] == nil {
			out[f] = map[int64]bool{}
		}
		out[f][p] = true
		return nil
	})
	return out, err
}

// faceRefsInOrder reads the faces ids, in that order.
func (s *Store) faceRefsInOrder(ids []int64) ([]FaceRef, error) {
	if len(ids) == 0 {
		return []FaceRef{}, nil
	}
	args := make([]any, len(ids))
	for i, id := range ids {
		args[i] = id
	}
	refs, err := s.faceRefs(faceRefSelect+` WHERE f.id IN (`+placeholders(len(ids))+`)`, args...)
	if err != nil {
		return nil, err
	}
	pos := make(map[int64]int, len(ids))
	for i, id := range ids {
		pos[id] = i
	}
	sort.Slice(refs, func(a, b int) bool { return pos[refs[a].ID] < pos[refs[b].ID] })
	return refs, nil
}
