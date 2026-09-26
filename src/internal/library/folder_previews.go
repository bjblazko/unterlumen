package library

// FolderPreview is what a subfolder tile shows before you open it: how many
// photos its whole branch holds, the years they span, and its newest few.
type FolderPreview struct {
	Name       string   `json:"name"`
	PhotoCount int      `json:"photoCount"`
	FirstTaken string   `json:"firstTaken,omitempty"` // date_taken as stored; empty when no photo has one
	LastTaken  string   `json:"lastTaken,omitempty"`
	PhotoIDs   []string `json:"photoIds"` // newest first by date taken, undated last by indexed time
}

// FolderPreviewPhotos is how many photos a tile shows.
const FolderPreviewPhotos = 4

// FolderPreviews returns one preview per direct subfolder of folderAbs,
// counting every photo nested below it at any depth — a folder that holds
// only more folders still shows what lies further down.
func (s *Store) FolderPreviews(folderAbs string) ([]FolderPreview, error) {
	prefix := folderAbs + "/"
	nested := `SELECT SUBSTR(path_hint, length(?1)+1, INSTR(SUBSTR(path_hint, length(?1)+1), '/')-1) AS sub,
	                  id, NULLIF(date_taken, '') AS date_taken, indexed_at
	           FROM photos WHERE status='ok' AND path_hint GLOB ?2`
	glob := prefix + "*/*"

	byName := map[string]*FolderPreview{}
	var order []string

	rows, err := s.db.Query(
		`SELECT sub, COUNT(*), COALESCE(MIN(date_taken),''), COALESCE(MAX(date_taken),'')
		 FROM (`+nested+`) WHERE sub != '' GROUP BY sub ORDER BY sub`,
		prefix, glob,
	)
	if err != nil {
		return nil, err
	}
	for rows.Next() {
		p := &FolderPreview{PhotoIDs: []string{}}
		if err := rows.Scan(&p.Name, &p.PhotoCount, &p.FirstTaken, &p.LastTaken); err != nil {
			rows.Close()
			return nil, err
		}
		byName[p.Name] = p
		order = append(order, p.Name)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return nil, err
	}

	rows, err = s.db.Query(
		`SELECT sub, id FROM (
		     SELECT sub, id, ROW_NUMBER() OVER (
		         PARTITION BY sub
		         ORDER BY date_taken IS NULL, date_taken DESC, indexed_at DESC) AS rn
		     FROM (`+nested+`) WHERE sub != '')
		 WHERE rn <= ? ORDER BY sub, rn`,
		prefix, glob, FolderPreviewPhotos,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var sub, id string
		if err := rows.Scan(&sub, &id); err != nil {
			return nil, err
		}
		if p := byName[sub]; p != nil {
			p.PhotoIDs = append(p.PhotoIDs, id)
		}
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	out := make([]FolderPreview, 0, len(order))
	for _, name := range order {
		out = append(out, *byName[name])
	}
	return out, nil
}
