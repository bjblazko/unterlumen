package library

import (
	"context"
	"fmt"
)

// The 3D views of 2026-11-02 beside the Colour space's points: the palette
// space (each photo's three main colours in OKLab) and the meaning space
// (photos placed by what they show, from the vision model's vectors reduced
// to three axes, principalAxes). Both are one PointCloud.

// PointCloud is photos as points, stored by column. Colours are OKLab l, a,
// b and the colour's share of the frame, PerPhoto colours per photo; XYZ,
// three per photo, is where the meaning space places it. Total is how many
// photos there are, of which these may be a sample.
type PointCloud struct {
	Libraries []string  `json:"libraries"`
	ID        []string  `json:"id"`
	Lib       []int     `json:"lib"`
	Date      []string  `json:"date"`
	XYZ       []float32 `json:"xyz,omitempty"`
	Colours   []float32 `json:"colours"`
	PerPhoto  int       `json:"perPhoto"`
	Total     int       `json:"total"`
}

// paletteSample is how many photos the palette space draws at most: three
// lights and three lines each.
const paletteSample = 6000

// Space builds the named view ("palette" or "meaning") over the libraries
// asked for, within pathPrefix and the shared filter.
func (m *Manager) Space(ctx context.Context, ids []string, pathPrefix, view string, f Filter) (*PointCloud, error) {
	if view != "palette" && view != "meaning" {
		return nil, fmt.Errorf("no view %q", view)
	}
	libs, err := m.filterLibraries(ids)
	if err != nil {
		return nil, err
	}
	key := statsCacheKey(libraryIDs(libs), pathPrefix+f.Key()) + "|" + view
	if v, ok := m.spacesCache.Load(key); ok {
		return v.(*PointCloud), nil
	}
	pc := &PointCloud{PerPhoto: 1}
	var vecs [][]float32
	for _, l := range libs {
		store, err := m.OpenFiltered(l.ID, f)
		if err != nil {
			continue
		}
		sc := store.statsScope(pathPrefix)
		if view == "palette" {
			err = store.addPalettes(pc, sc, len(pc.Libraries), paletteSample/len(libs))
		} else {
			var v [][]float32
			v, err = m.addMeaningPoints(ctx, store, l.ID, pc, sc, len(pc.Libraries))
			vecs = append(vecs, v...)
		}
		store.Close()
		if err != nil {
			return nil, err
		}
		pc.Libraries = append(pc.Libraries, l.ID)
	}
	if view == "meaning" {
		for _, p := range principalAxes(vecs) {
			pc.XYZ = append(pc.XYZ, p[0], p[1], p[2])
		}
	}
	m.spacesCache.Store(key, pc)
	return pc, nil
}

// addPalettes adds up to limit photos of the scope with their three largest
// swatches; a photo with fewer repeats its last.
func (s *Store) addPalettes(pc *PointCloud, sc statsScope, lib, limit int) error {
	pc.PerPhoto = 3
	var total int
	if err := s.db.QueryRow(`SELECT COUNT(DISTINCT photo_id) FROM photo_palette WHERE photo_id IN `+inScope(sc), sc.args...).Scan(&total); err != nil {
		return err
	}
	pc.Total += total
	// Photo IDs are hashes, so the first by ID are a fair sample.
	rows, err := s.db.Query(`SELECT pp.photo_id, COALESCE(p.date_taken, ''), pp.l, pp.c, pp.h, pp.share
		FROM photo_palette pp JOIN photos p ON p.id = pp.photo_id
		WHERE pp.rank < 3 AND pp.photo_id IN (SELECT DISTINCT photo_id FROM photo_palette
			WHERE photo_id IN `+inScope(sc)+` ORDER BY photo_id LIMIT ?)
		ORDER BY pp.photo_id, pp.rank`, append(append([]any{}, sc.args...), limit)...)
	if err != nil {
		return err
	}
	var cur string
	var colours []float32
	flush := func() {
		if cur == "" {
			return
		}
		for len(colours) < 12 {
			colours = append(colours, colours[len(colours)-4:]...)
		}
		pc.Colours = append(pc.Colours, colours[:12]...)
	}
	err = scanRows(rows, func() error {
		var id, date string
		var c LCh
		var share float64
		if err := rows.Scan(&id, &date, &c.L, &c.C, &c.H, &share); err != nil {
			return err
		}
		if id != cur {
			flush()
			cur, colours = id, nil
			pc.ID, pc.Lib, pc.Date = append(pc.ID, id), append(pc.Lib, lib), append(pc.Date, date)
		}
		a, b := labAB(c)
		colours = append(colours, float32(c.L), float32(a), float32(b), float32(share))
		return nil
	})
	flush()
	return err
}

// addMeaningPoints adds the scope's photos that have a vector, with their
// main colour, and returns their vectors in the same order.
func (m *Manager) addMeaningPoints(ctx context.Context, s *Store, libID string, pc *PointCloud, sc statsScope, lib int) ([][]float32, error) {
	vc, err := m.photoVectors(ctx, libID)
	if err != nil {
		return nil, err
	}
	byID := make(map[string][]float32, len(vc.ids))
	for i, id := range vc.ids {
		byID[id] = vc.vecs[i]
	}
	rows, err := s.db.Query(`SELECT p.id, COALESCE(p.date_taken, ''), COALESCE(m.l, 0.6), COALESCE(m.c, 0), COALESCE(m.h, 0)
		FROM photos p LEFT JOIN (SELECT photo_id, l, c, h, MIN(rank) FROM photo_palette WHERE hue_bin IS NOT NULL GROUP BY photo_id) m ON m.photo_id = p.id
		WHERE `+okIn(sc), sc.args...)
	if err != nil {
		return nil, err
	}
	var vecs [][]float32
	err = scanRows(rows, func() error {
		var id, date string
		var c LCh
		if err := rows.Scan(&id, &date, &c.L, &c.C, &c.H); err != nil {
			return err
		}
		v, ok := byID[id]
		if !ok {
			return nil
		}
		a, b := labAB(c)
		pc.ID, pc.Lib, pc.Date = append(pc.ID, id), append(pc.Lib, lib), append(pc.Date, date)
		pc.Colours = append(pc.Colours, float32(c.L), float32(a), float32(b), 1)
		vecs = append(vecs, v)
		return nil
	})
	pc.Total += len(vecs)
	return vecs, err
}
