package library

import (
	"database/sql"

	fd "huepattl.de/unterlumen/internal/facedecisions"
)

// What the owner decides about faces beyond their names — hiding, No's,
// covers — is kept in the folder installations share (facedecisions,
// ADR-0051 decision 15) and mirrored into each index. As with sidecars
// (ADR-0052), the shared file is written first; an index follows it.

// decisionsShared holds the file a library's own decisions went into;
// from then on the index follows that file. Another file — the shared
// folder changed — gets them too, before the index follows it.
const decisionsShared = "face_decisions_shared"

// facesOf returns each face's photo and box, as the shared file names it.
func (s *Store) decisionFaces(ids []int64) (map[int64]fd.Face, error) {
	rows, err := s.faceRows(ids)
	if err != nil {
		return nil, err
	}
	out := make(map[int64]fd.Face, len(rows))
	for _, r := range rows {
		out[r.faceID] = fd.Face{Photo: r.photoID, Box: [4]float64{r.box.X, r.box.Y, r.box.W, r.box.H}}
	}
	return out, nil
}

// decide changes the shared file for the faces ids; nothing without one.
func (s *Store) decide(ids []int64, change func(d *fd.Decisions, id int64, f fd.Face)) error {
	if s.decisions == nil || len(ids) == 0 {
		return nil
	}
	faces, err := s.decisionFaces(ids)
	if err != nil {
		return err
	}
	return s.decisions.Change(func(d *fd.Decisions) {
		for _, id := range ids {
			if f, ok := faces[id]; ok {
				change(d, id, f)
			}
		}
	})
}

func (s *Store) decideHidden(ids []int64, hidden bool) error {
	return s.decide(ids, func(d *fd.Decisions, _ int64, f fd.Face) {
		if hidden {
			d.AddHidden(f)
		} else {
			d.DropHidden(f)
		}
	})
}

// decideNot records, or with no false takes back, that the faces are not
// the people names gives for them.
func (s *Store) decideNot(ids []int64, names map[int64]string, no bool) error {
	return s.decide(ids, func(d *fd.Decisions, id int64, f fd.Face) {
		name, ok := names[id]
		switch {
		case !ok:
		case no:
			d.AddNot(f, name)
		default:
			d.DropNot(f, name)
		}
	})
}

// personNamesOf gives, for the faces that have a person, the person's name.
func (s *Store) personNamesOf(ids []int64) (map[int64]string, error) {
	out := map[int64]string{}
	if len(ids) == 0 {
		return out, nil
	}
	args := make([]any, len(ids))
	for i, id := range ids {
		args[i] = id
	}
	rows, err := s.db.Query(`SELECT f.id, p.name FROM faces f JOIN people p ON p.id = f.person_id
		WHERE f.confirmed = 1 AND f.manual = 0 AND f.id IN (`+placeholders(len(ids))+`)`, args...)
	if err != nil {
		return nil, err
	}
	err = scanRows(rows, func() error {
		var id int64
		var name string
		if err := rows.Scan(&id, &name); err != nil {
			return err
		}
		out[id] = name
		return nil
	})
	return out, err
}

func sameName(ids []int64, name string) map[int64]string {
	out := make(map[int64]string, len(ids))
	for _, id := range ids {
		out[id] = name
	}
	return out
}

func (s *Store) decideCover(person, face int64, chosen string) error {
	if s.decisions == nil {
		return nil
	}
	var name string
	if err := s.db.QueryRow(`SELECT name FROM people WHERE id = ?`, person).Scan(&name); err != nil {
		return err
	}
	faces, err := s.decisionFaces([]int64{face})
	if err != nil {
		return err
	}
	return s.decisions.Change(func(d *fd.Decisions) { d.SetCover(name, faces[face], chosen) })
}

// --- Following the shared file ---

// indexFace is a face of the index as the mirror compares it.
type indexFace struct {
	id      int64
	face    fd.Face
	person  int64 // 0 for none
	hidden  bool
	manual  bool
	matched bool
}

// ApplyDecisions makes the index's hidden faces, No's and covers what the
// shared file says. The first time, the library's own decisions go into
// the file, so nothing decided before is lost. It says whether faces came
// out of hiding, which calls for grouping anew.
func (s *Store) ApplyDecisions() (regroup bool, err error) {
	if s.decisions == nil {
		return false, nil
	}
	if err := s.shareOnce(); err != nil {
		return false, err
	}
	d, err := s.decisions.Read()
	if err != nil {
		return false, err
	}
	faces, err := s.indexFaces()
	if err != nil {
		return false, err
	}
	byName, err := s.peopleByName()
	if err != nil {
		return false, err
	}
	tx, err := s.db.Begin()
	if err != nil {
		return false, err
	}
	defer tx.Rollback() //nolint:errcheck
	if regroup, err = applyHidden(tx, faces, d.Hidden); err != nil {
		return false, err
	}
	if err := applyNots(tx, faces, d.Not, byName); err != nil {
		return false, err
	}
	if err := applyCovers(tx, faces, d.Covers, byName); err != nil {
		return false, err
	}
	return regroup, tx.Commit()
}

// shareOnce puts the library's own decisions into the file the first time
// the library meets it.
func (s *Store) shareOnce() error {
	into, _, err := s.GetProp(decisionsShared)
	if err != nil || into == s.decisions.Path() {
		return err
	}
	return s.exportDecisions()
}

func (s *Store) peopleByName() (map[string]Person, error) {
	people, err := s.People()
	byName := make(map[string]Person, len(people))
	for _, p := range people {
		byName[fd.Key(p.Name)] = p
	}
	return byName, err
}

func (s *Store) indexFaces() (map[string][]*indexFace, error) {
	rows, err := s.db.Query(`SELECT id, photo_id, x, y, w, h, COALESCE(person_id, 0), person_id IS NULL AND confirmed = 1, manual FROM faces`)
	if err != nil {
		return nil, err
	}
	out := map[string][]*indexFace{}
	err = scanRows(rows, func() error {
		f := &indexFace{}
		b := &f.face.Box
		if err := rows.Scan(&f.id, &f.face.Photo, &b[0], &b[1], &b[2], &b[3], &f.person, &f.hidden, &f.manual); err != nil {
			return err
		}
		out[f.face.Photo] = append(out[f.face.Photo], f)
		return nil
	})
	return out, err
}

// match is the face of the index the file's face means, if this library
// has its photo.
func match(faces map[string][]*indexFace, f fd.Face) *indexFace {
	for _, x := range faces[f.Photo] {
		if fd.Same(x.face, f) {
			return x
		}
	}
	return nil
}

// applyHidden hides the faces the file hides — not a face the sidecar
// names here, which says who is in the photo — and shows again the faces
// hidden here that the file no longer hides.
func applyHidden(tx *sql.Tx, faces map[string][]*indexFace, hidden []fd.Face) (shown bool, err error) {
	for _, h := range hidden {
		x := match(faces, h)
		if x == nil {
			continue
		}
		x.matched = true
		if !x.hidden && x.person == 0 && !x.manual {
			if _, err := tx.Exec(`UPDATE faces SET confirmed = 1, grp = NULL WHERE id = ?`, x.id); err != nil {
				return false, err
			}
		}
	}
	for _, list := range faces {
		for _, x := range list {
			if x.hidden && !x.matched {
				if _, err := tx.Exec(`UPDATE faces SET confirmed = 0 WHERE id = ?`, x.id); err != nil {
					return false, err
				}
				shown = true
			}
		}
	}
	return shown, nil
}

// applyNots makes the No's of the index those of the file, for the people
// named here.
func applyNots(tx *sql.Tx, faces map[string][]*indexFace, nots []fd.Not, byName map[string]Person) error {
	if _, err := tx.Exec(`DELETE FROM face_not`); err != nil {
		return err
	}
	for _, n := range nots {
		x, p := match(faces, n.Face), byName[fd.Key(n.Name)]
		if x == nil || p.ID == 0 {
			continue
		}
		if _, err := tx.Exec(`INSERT OR IGNORE INTO face_not (face, person) VALUES (?, ?)`, x.id, p.ID); err != nil {
			return err
		}
	}
	return nil
}

// applyCovers gives each person here the cover the file has for their
// name, when that face is one of theirs in this library.
func applyCovers(tx *sql.Tx, faces map[string][]*indexFace, covers map[string]fd.Cover, byName map[string]Person) error {
	for key, c := range covers {
		p, ok := byName[key]
		x := match(faces, c.Face)
		if !ok || x == nil || x.person != p.ID {
			continue
		}
		if p.Cover == x.id && p.CoverChosen == c.Chosen {
			continue
		}
		if _, err := tx.Exec(`UPDATE people SET cover = ?, cover_chosen = ? WHERE id = ?`, x.id, c.Chosen, p.ID); err != nil {
			return err
		}
	}
	return nil
}

// exportDecisions puts what this library decided before the file was
// shared into it: its hidden faces, its No's and its chosen covers.
func (s *Store) exportDecisions() error {
	own, err := s.ownDecisions()
	if err != nil {
		return err
	}
	err = s.decisions.Change(func(d *fd.Decisions) {
		for _, f := range own.Hidden {
			d.AddHidden(f)
		}
		for _, n := range own.Not {
			d.AddNot(n.Face, n.Name)
		}
		for name, c := range own.Covers {
			d.SetCover(name, c.Face, c.Chosen)
		}
	})
	if err != nil {
		return err
	}
	return s.SetProp(decisionsShared, s.decisions.Path())
}

// ownDecisions reads the index's decisions as the file keeps them.
func (s *Store) ownDecisions() (fd.Decisions, error) {
	own := fd.Decisions{Covers: map[string]fd.Cover{}}
	faces, err := s.indexFaces()
	if err != nil {
		return own, err
	}
	byID := map[int64]*indexFace{}
	for _, list := range faces {
		for _, x := range list {
			byID[x.id] = x
			if x.hidden {
				own.Hidden = append(own.Hidden, x.face)
			}
		}
	}
	people, err := s.People()
	if err != nil {
		return own, err
	}
	names := map[int64]string{}
	for _, p := range people {
		names[p.ID] = p.Name
		if x := byID[p.Cover]; x != nil && p.CoverChosen != "" {
			own.Covers[fd.Key(p.Name)] = fd.Cover{Face: x.face, Chosen: p.CoverChosen}
		}
	}
	nots, err := s.notRows()
	for _, n := range nots {
		if x, name := byID[n[0]], names[n[1]]; x != nil && name != "" {
			own.Not = append(own.Not, fd.Not{Face: x.face, Name: name})
		}
	}
	return own, err
}

func (s *Store) notRows() ([][2]int64, error) {
	rows, err := s.db.Query(`SELECT face, person FROM face_not`)
	if err != nil {
		return nil, err
	}
	var out [][2]int64
	err = scanRows(rows, func() error {
		var r [2]int64
		if err := rows.Scan(&r[0], &r[1]); err != nil {
			return err
		}
		out = append(out, r)
		return nil
	})
	return out, err
}
