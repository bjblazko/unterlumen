package library

// What the People topic of the statistics reads from one library: who is in
// which photo, how many faces each photo holds, and what every face looks
// like. Every query takes its photos through aliasCond, so the folder and
// the shared filter apply (ADR-0050).

// Appearance is a named person in a photo.
type Appearance struct {
	Name  string
	Photo string
	Date  string // date taken as stored, "" when unknown
}

// Appearances lists every named person in every photo in scope, once per
// photo.
func (s *Store) Appearances(pathGlob string) ([]Appearance, error) {
	where, args := s.aliasCond(pathGlob)
	rows, err := s.db.Query(`
		SELECT pe.name, p.id, COALESCE(p.date_taken, '')
		FROM faces f
		JOIN people pe ON pe.id = f.person_id
		JOIN photos p ON p.id = f.photo_id
		WHERE f.confirmed = 1 AND p.status = 'ok'`+where+`
		GROUP BY pe.id, p.id`, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Appearance{}
	err = scanRows(rows, func() error {
		var a Appearance
		if err := rows.Scan(&a.Name, &a.Photo, &a.Date); err != nil {
			return err
		}
		out = append(out, a)
		return nil
	})
	return out, err
}

// FaceCount is how many faces were found in a photo that was looked at.
type FaceCount struct {
	Date  string
	Faces int
}

// FaceCounts lists the faces of every photo in scope whose faces were
// looked for; a photo not looked at yet is left out, not counted as empty.
func (s *Store) FaceCounts(pathGlob string) ([]FaceCount, error) {
	where, args := s.aliasCond(pathGlob)
	rows, err := s.db.Query(`
		SELECT COALESCE(p.date_taken, ''), (SELECT COUNT(*) FROM faces f WHERE f.photo_id = p.id)
		FROM photos p JOIN photo_faces_found ff ON ff.photo_id = p.id
		WHERE p.status = 'ok'`+where, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []FaceCount{}
	err = scanRows(rows, func() error {
		var c FaceCount
		if err := rows.Scan(&c.Date, &c.Faces); err != nil {
			return err
		}
		out = append(out, c)
		return nil
	})
	return out, err
}

// FacePrint is one face with what it looks like, and its person's name ("" when unnamed).
type FacePrint struct {
	Face      int64
	Photo     string
	Date      string
	Name      string
	Embedding []float32
}

// FacePrints lists every face in scope that is not hidden.
func (s *Store) FacePrints(pathGlob string) ([]FacePrint, error) {
	where, args := s.aliasCond(pathGlob)
	rows, err := s.db.Query(`
		SELECT f.id, p.id, COALESCE(p.date_taken, ''), CASE WHEN f.confirmed = 1 THEN COALESCE(pe.name, '') ELSE '' END, f.embedding
		FROM faces f
		JOIN photos p ON p.id = f.photo_id
		LEFT JOIN people pe ON pe.id = f.person_id
		WHERE p.status = 'ok' AND NOT (f.person_id IS NULL AND f.confirmed = 1)`+where, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []FacePrint{}
	err = scanRows(rows, func() error {
		var fp FacePrint
		var blob []byte
		if err := rows.Scan(&fp.Face, &fp.Photo, &fp.Date, &fp.Name, &blob); err != nil {
			return err
		}
		fp.Embedding = decodeEmbedding(blob)
		out = append(out, fp)
		return nil
	})
	return out, err
}
