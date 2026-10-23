package library

import "database/sql"

// Hidden faces: those the owner said nobody is to name — strangers,
// statues, posters (ADR-0051). Hiding is shared with the other
// installation through the shared folder (store_face_decisions.go).

// HiddenFaces lists the faces the owner said nobody is to name, from the
// newest photo, so one hidden by mistake can be shown again; HiddenCount
// says how many.
func (s *Store) HiddenFaces() ([]FaceRef, error) {
	return s.faceRefs(faceRefSelect + `
		WHERE f.person_id IS NULL AND f.confirmed = 1
		ORDER BY CASE WHEN p.date_taken IS NULL OR p.date_taken = '' THEN 1 ELSE 0 END, p.date_taken DESC, f.w*f.h DESC`)
}

// ShowAgain takes faces out of hiding: they wait for a name again, each
// in the group it looks like.
func (s *Store) ShowAgain(ids []int64) error {
	if err := s.decideHidden(ids, false); err != nil {
		return err
	}
	if _, err := s.changeFaces(ids, func(*sql.Tx) (string, []any, error) {
		return `confirmed = 0`, nil, nil
	}); err != nil {
		return err
	}
	return s.Regroup()
}

func (s *Store) HiddenCount() (int, error) {
	var n int
	err := s.db.QueryRow(`SELECT COUNT(*) FROM faces WHERE person_id IS NULL AND confirmed = 1`).Scan(&n)
	return n, err
}
