package library

import (
	"strings"

	"huepattl.de/unterlumen/internal/faces"
)

// PersonNamed finds the person of that name in the library, without regard
// to case, as PersonNames puts names together; false when nobody here has it.
func (s *Store) PersonNamed(name string) (Person, bool, error) {
	people, err := s.People()
	if err != nil {
		return Person{}, false, err
	}
	for _, p := range people {
		if strings.EqualFold(p.Name, name) {
			return p, true, nil
		}
	}
	return Person{}, false, nil
}

// personEmbeddings returns what the person's confirmed faces look like.
func (s *Store) personEmbeddings(id int64) ([][]float32, error) {
	_, embs, err := s.embeddings(`SELECT id, embedding FROM faces WHERE person_id = ? AND confirmed = 1`, id)
	return embs, err
}

// nearestPerson is the named person who looks most like centre, if anyone
// looks enough like it to be suggested.
func (s *Store) nearestPerson(centre []float32) (*Person, error) {
	looks, err := s.lookalikes()
	if err != nil {
		return nil, err
	}
	p, _ := looks.best(centre, nil)
	return p, nil
}

// nearestGroup is the group of unnamed faces that looks most like centre,
// if one looks enough like it to be suggested, with its sample faces.
func (s *Store) nearestGroup(centre []float32) (*FaceGroup, error) {
	rows, err := s.db.Query(`SELECT grp, embedding FROM faces WHERE ` + unnamed + ` AND grp IS NOT NULL`)
	if err != nil {
		return nil, err
	}
	members := map[int64][][]float32{}
	var order []int64
	for rows.Next() {
		var grp int64
		var blob []byte
		if err := rows.Scan(&grp, &blob); err != nil {
			rows.Close()
			return nil, err
		}
		if _, ok := members[grp]; !ok {
			order = append(order, grp)
		}
		members[grp] = append(members[grp], decodeEmbedding(blob))
	}
	rows.Close()
	if err := rows.Err(); err != nil || len(order) == 0 {
		return nil, err
	}
	centres := make([][]float32, len(order))
	for i, grp := range order {
		centres[i] = faces.Mean(members[grp])
	}
	best, sim := faces.Nearest(centres, centre)
	if best < 0 || sim < faces.SuggestThreshold {
		return nil, nil
	}
	g := FaceGroup{ID: order[best], Size: len(members[order[best]])}
	if g.Faces, err = s.sampleFaces(g.ID); err != nil {
		return nil, err
	}
	return &g, nil
}
