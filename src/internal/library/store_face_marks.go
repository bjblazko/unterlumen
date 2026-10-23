package library

import (
	"database/sql"
	"errors"
	"strings"

	"huepattl.de/unterlumen/internal/media"
)

// A face the detector missed — in profile, half hidden — is marked by hand:
// a box and a name. It is a face like any other, except that it has no
// embedding, so it never suggests a name or joins a group, and it is never
// unnamed: taking its name away takes the face away. Its region in the
// sidecar is how the other installation learns of it, whose detector misses
// it too.

// ErrFaceAlreadyThere says a face is marked where one is already known.
var ErrFaceAlreadyThere = errors.New("there is a face there already; name it in the list")

// ErrBadFaceBox says a box to mark is not inside the photo or too small.
var ErrBadFaceBox = errors.New("the frame must lie inside the photo")

const minMarkedFace = 0.005 // of the photo's width or height

// MarkFace marks a face by hand in the photo: the box in fractions of the
// upright photo, and who it is. Over a face that was hidden, it names that
// face instead. The sidecar is written first (ADR-0052);
// skipped is 1 when another program keeps the sidecar's regions, and the
// face is then in this index only.
func (s *Store) MarkFace(photoID string, box media.FaceRegion) (skipped int, err error) {
	box.Name = strings.TrimSpace(box.Name)
	if !insidePhoto(box) {
		return 0, ErrBadFaceBox
	}
	photoID = s.CurrentPhotoID(photoID)
	under, hidden, err := s.faceUnder(photoID, box)
	switch {
	case err != nil:
		return 0, err
	case hidden:
		// A face hidden there was found after all: the frame names it.
		return s.NameFaces([]int64{under}, box.Name)
	case under != 0:
		return 0, ErrFaceAlreadyThere
	}
	if werr := s.writeFaceChanges(photoID, []media.FaceRegion{box}); errors.Is(werr, media.ErrForeignRegions) {
		skipped = 1
	} else if werr != nil {
		return 0, werr
	}
	return skipped, s.insertMarkedFace(photoID, box)
}

func insidePhoto(box media.FaceRegion) bool {
	return box.X >= 0 && box.Y >= 0 && box.W >= minMarkedFace && box.H >= minMarkedFace &&
		box.X+box.W <= 1.0001 && box.Y+box.H <= 1.0001
}

// faceUnder is the face of the photo that box overlaps enough, if any,
// and whether it is hidden; 0 when the place is free.
func (s *Store) faceUnder(photoID string, box media.FaceRegion) (id int64, hidden bool, err error) {
	known, err := s.faceNamesOf(photoID)
	if err != nil {
		return 0, false, err
	}
	for _, k := range known {
		if media.Overlap(regionOf(k), box) >= media.RegionMatch {
			hidden, err := s.isHidden(k.id)
			return k.id, hidden, err
		}
	}
	return 0, false, nil
}

func (s *Store) insertMarkedFace(photoID string, box media.FaceRegion) error {
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback() //nolint:errcheck
	if err := insertMarkedFaceTx(tx, photoID, box); err != nil {
		return err
	}
	return tx.Commit()
}

func (s *Store) isHidden(face int64) (bool, error) {
	var hidden bool
	err := s.db.QueryRow(`SELECT person_id IS NULL AND confirmed = 1 FROM faces WHERE id = ?`, face).Scan(&hidden)
	return hidden, err
}

func insertMarkedFaceTx(tx *sql.Tx, photoID string, r media.FaceRegion) error {
	person, err := personID(tx, r.Name)
	if err != nil {
		return err
	}
	_, err = tx.Exec(`INSERT INTO faces (photo_id, x, y, w, h, score, embedding, person_id, confirmed, manual)
		VALUES (?, ?, ?, ?, ?, 0, x'', ?, 1, 1)`, photoID, r.X, r.Y, r.W, r.H, person)
	return err
}

func regionOf(f faceName) media.FaceRegion {
	return media.FaceRegion{Name: f.name, X: f.box.X, Y: f.box.Y, W: f.box.W, H: f.box.H}
}

// applyMarkedFaces makes the photo's marked faces what the sidecar's regions
// say, as a scan reads them: wanted are the regions no found face covers.
// A marked face takes the name of its region, one without a region goes —
// removed on the other installation — unless another program keeps regions
// in the sidecar (foreign), and a region without a face is marked.
func (s *Store) applyMarkedFaces(photoID string, marked []faceName, wanted []media.FaceRegion, foreign bool) error {
	if len(marked)+len(wanted) == 0 {
		return nil
	}
	used := make([]bool, len(wanted))
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback() //nolint:errcheck
	for _, m := range marked {
		if err := followRegion(tx, m, wanted, used, foreign); err != nil {
			return err
		}
	}
	for i, r := range wanted {
		if !used[i] {
			if err := insertMarkedFaceTx(tx, photoID, r); err != nil {
				return err
			}
		}
	}
	if err := dropPeopleWithoutFaces(tx); err != nil {
		return err
	}
	return tx.Commit()
}

// followRegion gives a marked face the name of the region over it and marks
// that region used, or removes the face when no region is over it — unless
// another program keeps regions in the sidecar.
func followRegion(tx *sql.Tx, m faceName, wanted []media.FaceRegion, used []bool, foreign bool) error {
	i := bestRegion(regionOf(m), wanted, used)
	switch {
	case i >= 0:
		used[i] = true
		if strings.EqualFold(wanted[i].Name, m.name) {
			return nil
		}
		return setFacePerson(tx, m.id, wanted[i].Name)
	case !foreign:
		_, err := tx.Exec(`DELETE FROM faces WHERE id = ?`, m.id)
		return err
	}
	return nil
}

// bestRegion is the index of the unused region box overlaps most, if
// enough; -1 when none does.
func bestRegion(box media.FaceRegion, regions []media.FaceRegion, used []bool) int {
	best, at := media.RegionMatch, -1
	for i, r := range regions {
		if o := media.Overlap(box, r); !used[i] && o >= best {
			best, at = o, i
		}
	}
	return at
}

func setFacePerson(tx *sql.Tx, faceID int64, name string) error {
	person, err := personID(tx, name)
	if err != nil {
		return err
	}
	_, err = tx.Exec(`UPDATE faces SET person_id = ?, confirmed = 1 WHERE id = ?`, person, faceID)
	return err
}

// regionsNoFaceCovers are the regions that no found face overlaps enough:
// the faces marked by hand.
func regionsNoFaceCovers(regions []media.FaceRegion, found []faceName) []media.FaceRegion {
	var out []media.FaceRegion
	for _, r := range regions {
		covered := false
		for _, f := range found {
			if media.Overlap(regionOf(f), r) >= media.RegionMatch {
				covered = true
				break
			}
		}
		if !covered {
			out = append(out, r)
		}
	}
	return out
}

// markedFoundAgain pairs each face marked by hand in a photo with a found
// face that overlaps it enough (media.Overlap, in SQL), saying whether the
// found face is unnamed.
const markedFoundAgain = `SELECT m.id, m.person_id, f.id, f.person_id IS NULL AND f.confirmed = 0
	FROM faces m JOIN faces f ON f.photo_id = m.photo_id AND f.manual = 0
	WHERE m.photo_id = ? AND m.manual = 1
	  AND MAX(0, MIN(m.x+m.w, f.x+f.w) - MAX(m.x, f.x)) * MAX(0, MIN(m.y+m.h, f.y+f.h) - MAX(m.y, f.y))
	      >= ? * (m.w*m.h + f.w*f.h - MAX(0, MIN(m.x+m.w, f.x+f.w) - MAX(m.x, f.x)) * MAX(0, MIN(m.y+m.h, f.y+f.h) - MAX(m.y, f.y)))`

type foundAgain struct {
	marked, person, face int64
	unnamed              bool
}

// settleMarkedFaces runs after a pass stored the faces it found in a photo:
// a marked face the detector now finds as well gives its name to the found
// face, if that has none, and goes.
func settleMarkedFaces(tx *sql.Tx, photoID string) error {
	hits, err := marksFoundAgain(tx, photoID)
	if err != nil {
		return err
	}
	gone, named := map[int64]bool{}, map[int64]bool{}
	for _, h := range hits {
		if gone[h.marked] {
			continue
		}
		gone[h.marked] = true
		if h.unnamed && !named[h.face] {
			named[h.face] = true
			if _, err := tx.Exec(`UPDATE faces SET person_id = ?, confirmed = 1 WHERE id = ?`, h.person, h.face); err != nil {
				return err
			}
		}
		if _, err := tx.Exec(`DELETE FROM faces WHERE id = ?`, h.marked); err != nil {
			return err
		}
	}
	return nil
}

func marksFoundAgain(tx *sql.Tx, photoID string) ([]foundAgain, error) {
	rows, err := tx.Query(markedFoundAgain, photoID, media.RegionMatch)
	if err != nil {
		return nil, err
	}
	var hits []foundAgain
	err = scanRows(rows, func() error {
		var h foundAgain
		if err := rows.Scan(&h.marked, &h.person, &h.face, &h.unnamed); err != nil {
			return err
		}
		hits = append(hits, h)
		return nil
	})
	return hits, err
}
