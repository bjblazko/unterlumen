package library

import (
	"time"

	"huepattl.de/unterlumen/internal/media"
)

// FolderBrowseResult holds the immediate subfolders and direct photos at a given folder level.
type FolderBrowseResult struct {
	Subfolders []string `json:"subfolders"`
	Photos     []Photo  `json:"photos"`
	Total      int      `json:"total"`
}

// BrowseFolder returns the immediate subdirectory names and photos directly inside folderAbs.
// Photos nested in subdirectories are excluded from Photos but their parent directory appears
// in Subfolders. No filesystem reads are performed; all data comes from the DB.
func (s *Store) BrowseFolder(folderAbs string) (FolderBrowseResult, error) {
	prefix := folderAbs + "/"
	photos, err := s.directPhotos(prefix)
	if err != nil {
		return FolderBrowseResult{}, err
	}
	subfolders, err := s.subfolderNames(prefix)
	if err != nil {
		return FolderBrowseResult{}, err
	}
	return FolderBrowseResult{Subfolders: subfolders, Photos: photos, Total: len(photos)}, nil
}

// directPhotos returns the photos directly inside prefix, never nil — GLOB
// rules out any nested path (extra slash). DateTaken is joined from
// exif_index for client-side sorting support. GPS, film simulation, and image
// dimensions are fetched for overlay badges.
func (s *Store) directPhotos(prefix string) ([]Photo, error) {
	rows, err := s.db.Query(
		`SELECT p.id, p.path_hint, p.filename, p.file_size, p.indexed_at,
		        COALESCE(e.value, '') AS date_taken,
		        (SELECT value FROM exif_index WHERE photo_id=p.id AND field='GPSLatitude' LIMIT 1),
		        (SELECT value FROM exif_index WHERE photo_id=p.id AND field='FilmSimulation' LIMIT 1),
		        CAST(json_extract(p.exif_json,'$.width')  AS INTEGER),
		        CAST(json_extract(p.exif_json,'$.height') AS INTEGER)
		 FROM photos p
		 LEFT JOIN exif_index e ON e.photo_id = p.id AND e.field = 'DateTaken'
		 WHERE p.status='ok' AND p.path_hint GLOB ? AND p.path_hint NOT GLOB ?`,
		prefix+"*", prefix+"*/*",
	)
	if err != nil {
		return nil, err
	}
	photos := []Photo{}
	err = scanRows(rows, func() error {
		var p Photo
		var indexedAt string
		var gpsLat, filmSim *string
		var imgWidth, imgHeight *int
		if err := rows.Scan(&p.ID, &p.PathHint, &p.Filename, &p.FileSize, &indexedAt, &p.DateTaken, &gpsLat, &filmSim, &imgWidth, &imgHeight); err != nil {
			return err
		}
		p.IndexedAt, _ = time.Parse(time.RFC3339, indexedAt)
		p.Exif = overlayExif(gpsLat, filmSim, imgWidth, imgHeight)
		photos = append(photos, p)
		return nil
	})
	if err != nil {
		return nil, err
	}
	return photos, nil
}

// overlayExif is the EXIF a thumbnail's overlay badges need, or nil when the
// photo has none of it.
func overlayExif(gpsLat, filmSim *string, width, height *int) map[string]string {
	if gpsLat == nil && filmSim == nil && (width == nil || height == nil) {
		return nil
	}
	exif := make(map[string]string)
	if gpsLat != nil {
		exif["GPSLatitude"] = *gpsLat
	}
	if filmSim != nil {
		exif["FilmSimulation"] = *filmSim
	}
	if width != nil && height != nil && *width > 0 && *height > 0 {
		if ar := media.AspectRatioLabel(*width, *height); ar != "" {
			exif["AspectRatio"] = ar
		}
	}
	return exif
}

// BrowseFolderRecursive returns all photos nested anywhere under folderAbs (including subdirectories).
// No filesystem reads are performed; all data comes from the DB.
func (s *Store) BrowseFolderRecursive(folderAbs string) ([]Photo, error) {
	prefix := folderAbs + "/"
	rows, err := s.db.Query(
		`SELECT p.id, p.path_hint, p.filename, p.file_size, p.indexed_at,
		        COALESCE(e.value, '') AS date_taken
		 FROM photos p
		 LEFT JOIN exif_index e ON e.photo_id = p.id AND e.field = 'DateTaken'
		 WHERE p.status='ok' AND p.path_hint GLOB ?`,
		prefix+"*",
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var photos []Photo
	for rows.Next() {
		var p Photo
		var indexedAt string
		if err := rows.Scan(&p.ID, &p.PathHint, &p.Filename, &p.FileSize, &indexedAt, &p.DateTaken); err != nil {
			return nil, err
		}
		p.IndexedAt, _ = time.Parse(time.RFC3339, indexedAt)
		photos = append(photos, p)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	if photos == nil {
		photos = []Photo{}
	}
	return photos, nil
}
