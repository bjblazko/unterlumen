package library

import (
	"errors"
	"image"
	_ "image/jpeg" // the formats storedSize reads
	_ "image/png"
	"os"
	"strings"

	"huepattl.de/unterlumen/internal/faces"
	"huepattl.de/unterlumen/internal/media"
)

// faceRow is a face about to be changed: where it is and whether it carries
// a name the sidecar should have.
type faceRow struct {
	faceID  int64
	photoID string
	box     media.FaceRegion
	named   bool
}

func (s *Store) faceRows(ids []int64) ([]faceRow, error) {
	if len(ids) == 0 {
		return nil, nil
	}
	args := make([]any, len(ids))
	for i, id := range ids {
		args[i] = id
	}
	rows, err := s.db.Query(`SELECT id, photo_id, x, y, w, h, person_id IS NOT NULL AND confirmed = 1
		FROM faces WHERE id IN (`+placeholders(len(ids))+`)`, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []faceRow
	for rows.Next() {
		var r faceRow
		if err := rows.Scan(&r.faceID, &r.photoID, &r.box.X, &r.box.Y, &r.box.W, &r.box.H, &r.named); err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}

// writeNamesFirst writes the faces' new name — "" takes it away — into their
// photos' sidecars before the index changes (ADR-0052): each photo's sidecar
// as it is now, changing only these faces. onlyNamed leaves the sidecar
// alone for faces that had no name, which it cannot hold. It returns the
// faces whose index may change: those whose sidecar was written, and those
// whose regions another program keeps (counted in skipped; their names
// stay in this index only). A photo whose sidecar cannot be written keeps
// its faces as they were, and the first such error is returned.
func (s *Store) writeNamesFirst(rows []faceRow, name string, onlyNamed bool) (ok []int64, skipped int, err error) {
	byPhoto := map[string][]faceRow{}
	var order []string
	for _, r := range rows {
		if _, seen := byPhoto[r.photoID]; !seen {
			order = append(order, r.photoID)
		}
		byPhoto[r.photoID] = append(byPhoto[r.photoID], r)
	}
	for _, photo := range order {
		var changes []media.FaceRegion
		for _, r := range byPhoto[photo] {
			if onlyNamed && !r.named {
				continue
			}
			c := r.box
			c.Name = name
			changes = append(changes, c)
		}
		werr := s.writeFaceChanges(photo, changes)
		switch {
		case errors.Is(werr, media.ErrForeignRegions):
			skipped++
		case werr != nil:
			if err == nil {
				err = werr
			}
			continue
		}
		for _, r := range byPhoto[photo] {
			ok = append(ok, r.faceID)
		}
	}
	return ok, skipped, err
}

func (s *Store) writeFaceChanges(photoID string, changes []media.FaceRegion) error {
	if len(changes) == 0 {
		return nil
	}
	var path string
	var orientation, width, height int
	err := s.db.QueryRow(`
		SELECT p.path_hint,
		       COALESCE((SELECT CAST(TRIM(value, '"') AS INTEGER) FROM exif_index WHERE photo_id = p.id AND field = 'Orientation'), 1),
		       COALESCE((SELECT CAST(TRIM(value, '"') AS INTEGER) FROM exif_index WHERE photo_id = p.id AND field = 'PixelXDimension'), 0),
		       COALESCE((SELECT CAST(TRIM(value, '"') AS INTEGER) FROM exif_index WHERE photo_id = p.id AND field = 'PixelYDimension'), 0)
		FROM photos p WHERE p.id = ?`, photoID).Scan(&path, &orientation, &width, &height)
	if err != nil {
		return err
	}
	if width == 0 || height == 0 {
		width, height = storedSize(path)
	}
	return media.ChangeFaceRegions(path, changes, orientation, width, height)
}

// storedSize reads the pixel size from the photo's header, for photos
// whose EXIF does not say; 0, 0 for a format Go cannot read.
func storedSize(path string) (int, int) {
	f, err := os.Open(path)
	if err != nil {
		return 0, 0
	}
	defer f.Close()
	cfg, _, err := image.DecodeConfig(f)
	if err != nil {
		return 0, 0
	}
	return cfg.Width, cfg.Height
}

// ApplyFaceNames makes the names of a photo's faces in the index what the
// regions in its sidecar say, as a scan reads them (ADR-0052): a face a
// region covers takes its name, and a named face no region covers loses its
// name — taken away on the other installation — unless another program
// keeps regions in the sidecar (foreign), where a name may only have been
// impossible to write. Hidden faces stay hidden: the sidecar says only who
// is in a photo. The regions no found face covers are the faces marked by
// hand (applyMarkedFaces). Only the index changes; a scan writes no sidecar.
func (s *Store) ApplyFaceNames(photoID string, regions []media.FaceRegion, foreign bool) error {
	current, err := s.faceNamesOf(photoID)
	if err != nil || len(current)+len(regions) == 0 {
		return err
	}
	var found, marked []faceName
	for _, c := range current {
		if c.manual {
			marked = append(marked, c)
		} else {
			found = append(found, c)
		}
	}
	if err := s.applyFoundNames(found, regions, foreign); err != nil {
		return err
	}
	return s.applyMarkedFaces(photoID, marked, regionsNoFaceCovers(regions, found), foreign)
}

func (s *Store) applyFoundNames(found []faceName, regions []media.FaceRegion, foreign bool) error {
	boxes := make([]faces.Face, len(found))
	for i, c := range found {
		boxes[i] = c.box
	}
	names := namesFromRegions(boxes, regions)
	var err error
	for i, c := range found {
		switch {
		case names[i] != "" && !strings.EqualFold(names[i], c.name):
			err = s.nameFacesInIndex([]int64{c.id}, names[i])
		case names[i] == "" && c.name != "" && !foreign:
			err = s.unnameFacesInIndex([]int64{c.id})
		}
		if err != nil {
			return err
		}
	}
	return nil
}

type faceName struct {
	id     int64
	box    faces.Face
	name   string // "" for a face nobody named, or one hidden
	manual bool   // marked by hand
}

func (s *Store) faceNamesOf(photoID string) ([]faceName, error) {
	rows, err := s.db.Query(`SELECT f.id, f.x, f.y, f.w, f.h, COALESCE(p.name, ''), f.manual
		FROM faces f LEFT JOIN people p ON p.id = f.person_id AND f.confirmed = 1
		WHERE f.photo_id = ?`, photoID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []faceName
	for rows.Next() {
		var f faceName
		if err := rows.Scan(&f.id, &f.box.X, &f.box.Y, &f.box.W, &f.box.H, &f.name, &f.manual); err != nil {
			return nil, err
		}
		out = append(out, f)
	}
	return out, rows.Err()
}
