package library

import (
	"database/sql"
	"errors"
	"strings"
	"time"

	"huepattl.de/unterlumen/internal/faces"
)

// Person is someone named in a library, with the faces confirmed as theirs
// and the face that shows them best.
type Person struct {
	ID     int64  `json:"id"`
	Name   string `json:"name"`
	Faces  int    `json:"faces"`
	Photos int    `json:"photos"`
	Cover  int64  `json:"cover"`
	// CoverChosen is when the owner chose Cover, empty when it is their
	// largest face because nobody chose one or the chosen face left them.
	CoverChosen string `json:"coverChosen,omitempty"`
}

// FaceGroup is a set of unnamed faces that look alike. Faces holds a few,
// the largest first; Suggestion is the named person it looks most like,
// when it looks enough like them.
type FaceGroup struct {
	ID         int64   `json:"id"`
	Size       int     `json:"size"`
	Faces      []int64 `json:"faces"`
	Suggestion *Person `json:"suggestion,omitempty"`
}

// FaceRef is one face, the photo it is in, and where in it: fractions of
// the photo's upright thumbnail, top left.
type FaceRef struct {
	ID      int64   `json:"id"`
	PhotoID string  `json:"photoId"`
	X       float64 `json:"x"`
	Y       float64 `json:"y"`
	W       float64 `json:"w"`
	H       float64 `json:"h"`
	File    string  `json:"file,omitempty"` // the photo's file name, where a page opens the photo
	Date    string  `json:"date,omitempty"` // when the photo was taken, empty when nobody knows
}

// unnamed selects the faces still waiting for a name.
const unnamed = `person_id IS NULL AND confirmed = 0`

// Regroup sorts the unnamed faces into groups of faces that look alike,
// the largest faces first since they are the clearest.
func (s *Store) Regroup() error {
	ids, embs, err := s.embeddings(`SELECT id, embedding FROM faces WHERE ` + unnamed + ` ORDER BY w*h DESC`)
	if err != nil {
		return err
	}
	groups := faces.Group(embs)
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback() //nolint:errcheck
	for i, id := range ids {
		if _, err := tx.Exec(`UPDATE faces SET grp = ? WHERE id = ?`, groups[i], id); err != nil {
			return err
		}
	}
	return tx.Commit()
}

func (s *Store) embeddings(query string, args ...any) ([]int64, [][]float32, error) {
	rows, err := s.db.Query(query, args...)
	if err != nil {
		return nil, nil, err
	}
	defer rows.Close()
	var ids []int64
	var embs [][]float32
	for rows.Next() {
		var id int64
		var blob []byte
		if err := rows.Scan(&id, &blob); err != nil {
			return nil, nil, err
		}
		if len(blob) == 0 {
			continue // marked by hand: nothing to compare
		}
		ids = append(ids, id)
		embs = append(embs, decodeEmbedding(blob))
	}
	return ids, embs, rows.Err()
}

// People lists the people named in the library, those with the most faces first.
func (s *Store) People() ([]Person, error) {
	rows, err := s.db.Query(`
		SELECT p.id, p.name, COUNT(f.id), COUNT(DISTINCT f.photo_id),
		       COALESCE((SELECT c.id FROM faces c WHERE c.id = p.cover AND c.person_id = p.id AND c.confirmed = 1),
		                (SELECT c.id FROM faces c WHERE c.person_id = p.id AND c.confirmed = 1 ORDER BY c.w*c.h DESC LIMIT 1)),
		       CASE WHEN EXISTS (SELECT 1 FROM faces c WHERE c.id = p.cover AND c.person_id = p.id AND c.confirmed = 1)
		            THEN COALESCE(p.cover_chosen, '') ELSE '' END
		FROM people p JOIN faces f ON f.person_id = p.id AND f.confirmed = 1
		GROUP BY p.id ORDER BY COUNT(f.id) DESC, p.name`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	people := []Person{}
	for rows.Next() {
		var p Person
		if err := rows.Scan(&p.ID, &p.Name, &p.Faces, &p.Photos, &p.Cover, &p.CoverChosen); err != nil {
			return nil, err
		}
		people = append(people, p)
	}
	return people, rows.Err()
}

// FaceGroups lists the groups of at least minSize unnamed faces, the
// largest first, at most limit of them.
func (s *Store) FaceGroups(minSize, limit int) ([]FaceGroup, error) {
	groups, err := s.groupSizes(minSize, limit)
	if err != nil {
		return nil, err
	}
	looks, err := s.lookalikes()
	if err != nil {
		return nil, err
	}
	for i := range groups {
		g := &groups[i]
		if g.Faces, err = s.sampleFaces(g.ID); err != nil {
			return nil, err
		}
		if g.Suggestion, err = s.suggestGroup(looks, g.ID); err != nil {
			return nil, err
		}
	}
	return groups, nil
}

func (s *Store) groupSizes(minSize, limit int) ([]FaceGroup, error) {
	rows, err := s.db.Query(`SELECT grp, COUNT(*) FROM faces WHERE `+unnamed+` AND grp IS NOT NULL
		GROUP BY grp HAVING COUNT(*) >= ? ORDER BY COUNT(*) DESC, grp LIMIT ?`, minSize, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	groups := []FaceGroup{}
	for rows.Next() {
		var g FaceGroup
		if err := rows.Scan(&g.ID, &g.Size); err != nil {
			return nil, err
		}
		groups = append(groups, g)
	}
	return groups, rows.Err()
}

// GroupCount is how many groups of unnamed faces there are.
func (s *Store) GroupCount() (int, error) {
	var n int
	err := s.db.QueryRow(`SELECT COUNT(DISTINCT grp) FROM faces WHERE ` + unnamed + ` AND grp IS NOT NULL`).Scan(&n)
	return n, err
}

const groupSample = 6

func (s *Store) sampleFaces(grp int64) ([]int64, error) {
	rows, err := s.db.Query(`SELECT id FROM faces WHERE grp = ? AND `+unnamed+` ORDER BY w*h DESC LIMIT ?`, grp, groupSample)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var ids []int64
	for rows.Next() {
		var id int64
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		ids = append(ids, id)
	}
	return ids, rows.Err()
}

// GroupFaces lists the unnamed faces of a group, the largest first;
// PersonFaces the confirmed faces of a person, from the newest photo to the
// oldest, those without a date last.
func (s *Store) GroupFaces(grp int64) ([]FaceRef, error) {
	return s.faceRefs(faceRefSelect+`
		WHERE f.grp = ? AND f.person_id IS NULL AND f.confirmed = 0 ORDER BY f.w*f.h DESC`, grp)
}

func (s *Store) PersonFaces(person int64) ([]FaceRef, error) {
	return s.faceRefs(faceRefSelect+`
		WHERE f.person_id = ? AND f.confirmed = 1
		ORDER BY CASE WHEN p.date_taken IS NULL OR p.date_taken = '' THEN 1 ELSE 0 END, p.date_taken DESC, f.w*f.h DESC`, person)
}

// faceRefSelect reads the columns of a FaceRef, the photo's file name and
// date with it; faceRefs scans them.
const faceRefSelect = `SELECT f.id, f.photo_id, f.x, f.y, f.w, f.h, COALESCE(p.filename, ''), COALESCE(p.date_taken, '')
	FROM faces f LEFT JOIN photos p ON p.id = f.photo_id`

func (s *Store) faceRefs(query string, args ...any) ([]FaceRef, error) {
	rows, err := s.db.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	refs := []FaceRef{}
	for rows.Next() {
		var r FaceRef
		if err := rows.Scan(&r.ID, &r.PhotoID, &r.X, &r.Y, &r.W, &r.H, &r.File, &r.Date); err != nil {
			return nil, err
		}
		refs = append(refs, r)
	}
	return refs, rows.Err()
}

// FaceBox is where a face is: its photo's thumbnail and its box in fractions of it.
type FaceBox struct {
	ThumbPath  string
	X, Y, W, H float64
}

func (s *Store) FaceBox(id int64) (FaceBox, error) {
	var b FaceBox
	err := s.db.QueryRow(`SELECT p.thumb_path, f.x, f.y, f.w, f.h FROM faces f JOIN photos p ON p.id = f.photo_id WHERE f.id = ?`, id).
		Scan(&b.ThumbPath, &b.X, &b.Y, &b.W, &b.H)
	return b, err
}

// NameFaces confirms the faces as the person called name, adding them when
// there is nobody of that name: the sidecars first, then the index. skipped
// counts the photos whose regions another program keeps, where the name
// stays in this index only; a photo whose sidecar cannot be written keeps
// its faces as they were, and the error says so.
func (s *Store) NameFaces(ids []int64, name string) (skipped int, err error) {
	name = strings.TrimSpace(name)
	return s.changeNames(ids, name, false, func(ok []int64) error {
		// Faces that were someone else are not them: a No for the old name.
		if err := s.rememberNot(ok); err != nil {
			return err
		}
		// A face hidden there is named after all.
		if err := s.decideHidden(ok, false); err != nil {
			return err
		}
		if err := s.nameFacesInIndex(ok, name); err != nil {
			return err
		}
		return s.forgetNot(ok, name)
	})
}

// UnnameFaces takes the faces back from whoever they were confirmed as, or
// from being hidden, and puts them together in a group of their own; their
// names leave the sidecars, and they are not suggested as that person again.
func (s *Store) UnnameFaces(ids []int64) (skipped int, err error) {
	return s.changeNames(ids, "", true, func(ok []int64) error {
		if err := s.rememberNot(ok); err != nil {
			return err
		}
		return s.unnameFacesInIndex(ok)
	})
}

// HideFaces marks the faces as ones nobody is to name: strangers, statues.
// Hiding stays in this index — the sidecar says only who is in a photo — but
// a name the faces had leaves the sidecar.
func (s *Store) HideFaces(ids []int64) (skipped int, err error) {
	return s.changeNames(ids, "", true, func(ok []int64) error {
		if err := s.decideHidden(ok, true); err != nil {
			return err
		}
		_, err := s.changeFaces(ok, func(*sql.Tx) (string, []any, error) {
			return `person_id = NULL, confirmed = 1`, nil, nil
		})
		return err
	})
}

// changeNames writes name for the faces into the sidecars first
// (writeNamesFirst) and then changes the index of the faces that may change.
func (s *Store) changeNames(ids []int64, name string, onlyNamed bool, inIndex func([]int64) error) (int, error) {
	rows, err := s.faceRows(ids)
	if err != nil {
		return 0, err
	}
	ok, skipped, werr := s.writeNamesFirst(rows, name, onlyNamed)
	if err := inIndex(ok); err != nil {
		return skipped, err
	}
	return skipped, werr
}

// nameFacesInIndex confirms the faces as name in the index only, as a scan
// does when it reads the name from a sidecar.
func (s *Store) nameFacesInIndex(ids []int64, name string) error {
	_, err := s.changeFaces(ids, func(tx *sql.Tx) (string, []any, error) {
		id, err := personID(tx, name)
		return `person_id = ?, confirmed = 1`, []any{id}, err
	})
	return err
}

// unnameFacesInIndex makes the faces unnamed again, in a group of their own.
func (s *Store) unnameFacesInIndex(ids []int64) error {
	_, err := s.changeFaces(ids, func(tx *sql.Tx) (string, []any, error) {
		var next int64
		err := tx.QueryRow(`SELECT COALESCE(MAX(grp), -1) + 1 FROM faces`).Scan(&next)
		return `person_id = NULL, confirmed = 0, grp = ?`, []any{next}, err
	})
	return err
}

// changeFaces sets the columns set returns on the faces ids, removes the
// people left without a face, and returns the photos the faces are in.
func (s *Store) changeFaces(ids []int64, set func(*sql.Tx) (string, []any, error)) ([]string, error) {
	if len(ids) == 0 {
		return nil, nil
	}
	tx, err := s.db.Begin()
	if err != nil {
		return nil, err
	}
	defer tx.Rollback() //nolint:errcheck
	cols, args, err := set(tx)
	if err != nil {
		return nil, err
	}
	in := placeholders(len(ids))
	idArgs := make([]any, len(ids))
	for i, id := range ids {
		idArgs[i] = id
	}
	if _, err := tx.Exec(`UPDATE faces SET `+cols+` WHERE id IN (`+in+`)`, append(args, idArgs...)...); err != nil {
		return nil, err
	}
	photos, err := photosOfFaces(tx, in, idArgs)
	if err != nil {
		return nil, err
	}
	// A face marked by hand is there only as someone: without a name, it goes.
	if _, err := tx.Exec(`DELETE FROM faces WHERE manual = 1 AND person_id IS NULL AND id IN (`+in+`)`, idArgs...); err != nil {
		return nil, err
	}
	if err := dropPeopleWithoutFaces(tx); err != nil {
		return nil, err
	}
	return photos, tx.Commit()
}

func dropPeopleWithoutFaces(tx *sql.Tx) error {
	_, err := tx.Exec(`DELETE FROM people WHERE id NOT IN (SELECT person_id FROM faces WHERE person_id IS NOT NULL)`)
	return err
}

func photosOfFaces(tx *sql.Tx, in string, ids []any) ([]string, error) {
	rows, err := tx.Query(`SELECT DISTINCT photo_id FROM faces WHERE id IN (`+in+`)`, ids...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var photos []string
	for rows.Next() {
		var p string
		if err := rows.Scan(&p); err != nil {
			return nil, err
		}
		photos = append(photos, p)
	}
	return photos, rows.Err()
}

// coverChosenLayout has a fixed width, so later choices sort after earlier ones.
const coverChosenLayout = "2006-01-02T15:04:05.000000000Z"

// ErrNotTheirFace says a face chosen as a person's cover is not one of
// their confirmed faces.
var ErrNotTheirFace = errors.New("the face is not one of this person's")

// ChooseCover makes face the one that shows the person. The choice is this
// installation's, like hiding a face: the sidecar says who is in a photo,
// not how one installation likes to show them. Should the face leave the
// person, they are shown by their largest face again. When it was chosen
// is kept: across libraries, the cover chosen last shows the person.
func (s *Store) ChooseCover(person, face int64) error {
	now := time.Now().UTC().Format(coverChosenLayout)
	var theirs bool
	if err := s.db.QueryRow(`SELECT EXISTS (SELECT 1 FROM faces WHERE id = ? AND person_id = ? AND confirmed = 1)`, face, person).Scan(&theirs); err != nil {
		return err
	}
	if !theirs {
		return ErrNotTheirFace
	}
	if err := s.decideCover(person, face, now); err != nil {
		return err
	}
	res, err := s.db.Exec(`UPDATE people SET cover = ?, cover_chosen = ? WHERE id = ?
		AND EXISTS (SELECT 1 FROM faces WHERE id = ? AND person_id = ? AND confirmed = 1)`, face, now, person, face, person)
	if err != nil {
		return err
	}
	if n, _ := res.RowsAffected(); n == 0 {
		return ErrNotTheirFace
	}
	return nil
}

// RenamePerson gives a person another name — in the sidecars of their
// photos first, then in the index. When someone else already has it, the
// two become one.
func (s *Store) RenamePerson(id int64, name string) (skipped int, err error) {
	refs, err := s.PersonFaces(id)
	if err != nil {
		return 0, err
	}
	ids := make([]int64, len(refs))
	for i, r := range refs {
		ids[i] = r.ID
	}
	name = strings.TrimSpace(name)
	return s.changeNames(ids, name, false, func(ok []int64) error {
		if len(ok) == len(ids) {
			// Everyone renamed: the person keeps its ID, unless the name
			// is someone else's.
			res, err := s.db.Exec(`UPDATE people SET name = ? WHERE id = ?
				AND NOT EXISTS (SELECT 1 FROM people WHERE name = ? AND id != ?)`, name, id, name, id)
			if err != nil {
				return err
			}
			if n, _ := res.RowsAffected(); n > 0 {
				return nil
			}
		}
		return s.nameFacesInIndex(ok, name)
	})
}

// ForgetPerson takes the name away from all the person's faces — in the
// sidecars first, then in the index — and the faces go back to the groups
// waiting for a name. skipped is as for NameFaces.
func (s *Store) ForgetPerson(id int64) (skipped int, err error) {
	refs, err := s.PersonFaces(id)
	if err != nil {
		return 0, err
	}
	ids := make([]int64, len(refs))
	for i, r := range refs {
		ids[i] = r.ID
	}
	return s.UnnameFaces(ids)
}

// PhotoFace is a face of one photo, as its info panel shows it: where it is
// and who it is; Hidden for a face the owner said not to name.
type PhotoFace struct {
	FaceRef
	Name   string `json:"name,omitempty"`
	Hidden bool   `json:"hidden,omitempty"`
	Manual bool   `json:"manual,omitempty"` // marked by hand
	// Suggestion is who a face nobody named looks like, if anyone.
	Suggestion *Person `json:"suggestion,omitempty"`
	Age        *int    `json:"age,omitempty"` // in whole years on the day the photo was taken, when both are known
}

// PhotoFaces lists the faces found in a photo, left to right, with who
// each unnamed one looks like.
func (s *Store) PhotoFaces(photoID string) ([]PhotoFace, error) {
	rows, err := s.db.Query(`SELECT f.id, f.photo_id, f.x, f.y, f.w, f.h, COALESCE(p.name, ''), f.person_id IS NULL AND f.confirmed = 1, f.manual
		FROM faces f LEFT JOIN people p ON p.id = f.person_id AND f.confirmed = 1
		WHERE f.photo_id = ? ORDER BY f.x`, s.CurrentPhotoID(photoID))
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []PhotoFace{}
	for rows.Next() {
		var f PhotoFace
		if err := rows.Scan(&f.ID, &f.PhotoID, &f.X, &f.Y, &f.W, &f.H, &f.Name, &f.Hidden, &f.Manual); err != nil {
			return nil, err
		}
		out = append(out, f)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	rows.Close()
	return out, s.suggestNames(out)
}

// suggestNames says who each face nobody named looks like.
func (s *Store) suggestNames(found []PhotoFace) error {
	var looks *lookalikes
	for i := range found {
		f := &found[i]
		if f.Name != "" || f.Hidden || f.Manual {
			continue
		}
		var err error
		if looks == nil {
			if looks, err = s.lookalikes(); err != nil {
				return err
			}
		}
		if f.Suggestion, err = s.suggestFace(looks, f.ID); err != nil {
			return err
		}
	}
	return nil
}
