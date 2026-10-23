package library

import (
	"context"
	"encoding/json"
	"errors"
	"sort"

	"huepattl.de/unterlumen/internal/things"
)

// Searching photos by words (ADR-0054): the words become CLIP's vector of
// them, compared with every photo's vector. Similarities of a phrase to
// photos lie close together, so the photos taken are those near the best
// match of all libraries, never far below a floor, at most wordsMost of them,
// the best first.
const (
	wordsNearBest = 0.035
	wordsFloor    = 0.22
	wordsMost     = 500
)

// ErrNoWords means searching by words needs things to be turned on.
var ErrNoWords = errors.New("searching by words needs Things: turn it on in Statistics, Things")

// vectorCache holds a library's photo vectors, read again when they
// changed: their number, or their last row, which every write renews.
type vectorCache struct {
	n    [2]int64
	ids  []string
	vecs [][]float32
}

// photoVectors is every photo vector of a library, from memory when no
// photo was looked at since.
func (m *Manager) photoVectors(ctx context.Context, id string) (*vectorCache, error) {
	store, err := m.OpenStore(id)
	if err != nil {
		return nil, err
	}
	var n [2]int64
	if err := store.db.QueryRowContext(ctx, `SELECT COUNT(*), COALESCE(MAX(rowid), 0) FROM photo_vectors`).Scan(&n[0], &n[1]); err != nil {
		return nil, err
	}
	if v, ok := m.vectors.Load(id); ok && v.(*vectorCache).n == n {
		return v.(*vectorCache), nil
	}
	rows, err := store.db.QueryContext(ctx, `SELECT photo_id, vector FROM photo_vectors`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	c := &vectorCache{n: n}
	err = scanRows(rows, func() error {
		var pid string
		var b []byte
		if err := rows.Scan(&pid, &b); err != nil {
			return err
		}
		c.ids, c.vecs = append(c.ids, pid), append(c.vecs, decodeEmbedding(b))
		return nil
	})
	if err != nil {
		return nil, err
	}
	m.vectors.Store(id, c)
	return c, nil
}

// wordMatches finds, per library, the photos that match words, with how
// well they match.
func (m *Manager) wordMatches(ctx context.Context, libs []*Library, words string) (map[string]map[string]float32, error) {
	if !m.ThingsInstalled() {
		return nil, ErrNoWords
	}
	look, err := m.thingsLook()
	if err != nil {
		return nil, err
	}
	q, err := look.engine.Describe(words)
	if err != nil {
		return nil, err
	}
	type hit struct {
		lib, photo string
		sim        float32
	}
	var hits []hit
	for _, l := range libs {
		c, err := m.photoVectors(ctx, l.ID)
		if err != nil {
			continue
		}
		for i, v := range c.vecs {
			hits = append(hits, hit{l.ID, c.ids[i], things.Similarity(q, v)})
		}
	}
	sort.Slice(hits, func(a, b int) bool { return hits[a].sim > hits[b].sim })
	out := map[string]map[string]float32{}
	for i, h := range hits {
		if i == wordsMost || h.sim < wordsFloor || h.sim < hits[0].sim-wordsNearBest {
			break
		}
		if out[h.lib] == nil {
			out[h.lib] = map[string]float32{}
		}
		out[h.lib][h.photo] = h.sim
	}
	return out, nil
}

func matchesJSON(matches map[string]float32) string {
	ids := make([]string, 0, len(matches))
	for id := range matches {
		ids = append(ids, id)
	}
	b, _ := json.Marshal(ids)
	return string(b)
}

// listMatches lists the photos that match the words and the other
// conditions, the best match first. There are at most wordsMost, so they are
// read whole and sorted here.
func (s *Store) listMatches(ctx context.Context, opts ListPhotosOpts) (ListPhotosResult, error) {
	fromSQL, whereSQL, args := newPhotoFilter(opts).sql()
	rows, err := s.db.QueryContext(ctx,
		`SELECT p.id, p.path_hint, p.filename, p.file_size, p.indexed_at, p.status, p.date_taken,
		        (SELECT value FROM exif_index WHERE photo_id=p.id AND field='GPSLatitude' LIMIT 1),
		        (SELECT value FROM exif_index WHERE photo_id=p.id AND field='FilmSimulation' LIMIT 1)
		 FROM photos p WHERE p.id IN (SELECT p.id `+fromSQL+` WHERE `+whereSQL+`)`, args...)
	if err != nil {
		return ListPhotosResult{}, err
	}
	photos, err := scanListedPhotos(rows)
	if err != nil {
		return ListPhotosResult{}, err
	}
	for i := range photos {
		photos[i].Match = opts.matches[photos[i].ID]
	}
	sort.SliceStable(photos, func(i, j int) bool { return photos[i].Match > photos[j].Match })
	total := len(photos)
	from, to := min(opts.Offset, total), min(opts.Offset+opts.Limit, total)
	return ListPhotosResult{Photos: photos[from:to], Total: total}, nil
}
