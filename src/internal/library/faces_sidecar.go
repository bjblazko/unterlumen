package library

import (
	"errors"
	"image"
	_ "image/jpeg" // the formats storedSize reads
	_ "image/png"
	"os"

	"huepattl.de/unterlumen/internal/media"
)

// WriteFaceNames writes the confirmed names of the faces in each photo into
// its sidecar, where the other installation and other programs read them
// (ADR-0051). A sidecar whose regions another program keeps is left alone
// and counted in skipped; the names then live in this index only.
func (s *Store) WriteFaceNames(photoIDs []string) (skipped int, err error) {
	for _, id := range photoIDs {
		werr := s.writeFaceNames(id)
		switch {
		case errors.Is(werr, media.ErrForeignRegions):
			skipped++
		case werr != nil && err == nil:
			err = werr
		}
	}
	return skipped, err
}

func (s *Store) writeFaceNames(photoID string) error {
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
	regions, err := s.namedFaces(photoID)
	if err != nil {
		return err
	}
	return media.WriteFaceRegions(path, regions, orientation, width, height)
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

func (s *Store) namedFaces(photoID string) ([]media.FaceRegion, error) {
	rows, err := s.db.Query(`SELECT p.name, f.x, f.y, f.w, f.h FROM faces f JOIN people p ON p.id = f.person_id
		WHERE f.photo_id = ? AND f.confirmed = 1 ORDER BY f.x`, photoID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var regions []media.FaceRegion
	for rows.Next() {
		var r media.FaceRegion
		if err := rows.Scan(&r.Name, &r.X, &r.Y, &r.W, &r.H); err != nil {
			return nil, err
		}
		regions = append(regions, r)
	}
	return regions, rows.Err()
}

// ApplyFaceNames names the faces of a photo after the regions in its
// sidecar, as indexing reads them when the sidecar changes. A face no
// region covers keeps what it had.
func (s *Store) ApplyFaceNames(photoID string, regions []media.FaceRegion) error {
	if len(regions) == 0 {
		return nil
	}
	refs, boxes, err := s.facesOf(photoID)
	if err != nil || len(refs) == 0 {
		return err
	}
	names := namesFromRegions(boxes, regions)
	for i, name := range names {
		if name == "" {
			continue
		}
		if _, err := s.NameFaces([]int64{refs[i]}, name); err != nil {
			return err
		}
	}
	return nil
}
