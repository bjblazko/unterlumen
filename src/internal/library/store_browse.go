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

	// Direct photos only — GLOB rules out any nested path (extra slash).
	// DateTaken is joined from exif_index for client-side sorting support.
	// GPS, film simulation, and image dimensions are fetched for overlay badges.
	photoRows, err := s.db.Query(
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
		return FolderBrowseResult{}, err
	}
	defer photoRows.Close()

	var directPhotos []Photo
	for photoRows.Next() {
		var p Photo
		var indexedAt string
		var gpsLat, filmSim *string
		var imgWidth, imgHeight *int
		if err := photoRows.Scan(&p.ID, &p.PathHint, &p.Filename, &p.FileSize, &indexedAt, &p.DateTaken, &gpsLat, &filmSim, &imgWidth, &imgHeight); err != nil {
			return FolderBrowseResult{}, err
		}
		p.IndexedAt, _ = time.Parse(time.RFC3339, indexedAt)
		if gpsLat != nil || filmSim != nil || (imgWidth != nil && imgHeight != nil) {
			p.Exif = make(map[string]string)
			if gpsLat != nil {
				p.Exif["GPSLatitude"] = *gpsLat
			}
			if filmSim != nil {
				p.Exif["FilmSimulation"] = *filmSim
			}
			if imgWidth != nil && imgHeight != nil && *imgWidth > 0 && *imgHeight > 0 {
				if ar := media.AspectRatioLabel(*imgWidth, *imgHeight); ar != "" {
					p.Exif["AspectRatio"] = ar
				}
			}
		}
		directPhotos = append(directPhotos, p)
	}
	if err := photoRows.Err(); err != nil {
		return FolderBrowseResult{}, err
	}

	// Subfolders: extract the first path segment below prefix for all nested photos.
	// SUBSTR/INSTR in SQL avoids returning full rows; DISTINCT collapses duplicates.
	sfRows, err := s.db.Query(
		`SELECT DISTINCT SUBSTR(path_hint, length(?)+1, INSTR(SUBSTR(path_hint, length(?)+1), '/')-1)
		 FROM photos
		 WHERE status='ok' AND path_hint GLOB ?`,
		prefix, prefix, prefix+"*/*",
	)
	if err != nil {
		return FolderBrowseResult{}, err
	}
	defer sfRows.Close()

	var subfolders []string
	for sfRows.Next() {
		var name string
		if err := sfRows.Scan(&name); err != nil {
			return FolderBrowseResult{}, err
		}
		if name != "" {
			subfolders = append(subfolders, name)
		}
	}
	if err := sfRows.Err(); err != nil {
		return FolderBrowseResult{}, err
	}
	sortStrings(subfolders)

	if directPhotos == nil {
		directPhotos = []Photo{}
	}
	if subfolders == nil {
		subfolders = []string{}
	}
	return FolderBrowseResult{
		Subfolders: subfolders,
		Photos:     directPhotos,
		Total:      len(directPhotos),
	}, nil
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
