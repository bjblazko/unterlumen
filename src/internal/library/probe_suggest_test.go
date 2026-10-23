//go:build probe

package library

import (
	"os"
	"sort"
	"testing"
	"time"

	"huepattl.de/unterlumen/internal/faces"
)

// Calibrates suggestions on a copy of a real library: every named face,
// left out, against its own person and the best other, by the person's
// centre and by the mean of the k nearest of their faces.
//
//	PROBE_DB=/path/to/copy/library.db go test -tags probe -run ProbeSuggest -v ./internal/library/
func TestProbeSuggest(t *testing.T) {
	path := os.Getenv("PROBE_DB")
	if path == "" {
		t.Skip("set PROBE_DB")
	}
	db, err := openDB(path)
	if err != nil {
		t.Fatal(err)
	}
	s := newStore(db, "")
	rows, err := s.db.Query(`SELECT person_id, embedding FROM faces WHERE person_id IS NOT NULL AND confirmed = 1 AND length(embedding) > 0`)
	if err != nil {
		t.Fatal(err)
	}
	byPerson := map[int64][][]float32{}
	type face struct {
		p int64
		e []float32
		i int
	}
	var all []face
	for rows.Next() {
		var p int64
		var b []byte
		rows.Scan(&p, &b)
		e := decodeEmbedding(b)
		all = append(all, face{p, e, len(byPerson[p])})
		byPerson[p] = append(byPerson[p], e)
	}
	rows.Close()
	topk := func(e []float32, ex [][]float32, skip, k int) float32 {
		var sims []float32
		for j, x := range ex {
			if j != skip {
				sims = append(sims, faces.Similarity(e, x))
			}
		}
		if len(sims) == 0 {
			return -1
		}
		sort.Slice(sims, func(a, b int) bool { return sims[a] > sims[b] })
		if len(sims) < k {
			k = len(sims)
		}
		var sum float32
		for _, v := range sims[:k] {
			sum += v
		}
		return sum / float32(k)
	}
	centre := func(ex [][]float32, skip int) []float32 {
		var rest [][]float32
		for j, x := range ex {
			if j != skip {
				rest = append(rest, x)
			}
		}
		return faces.Mean(rest)
	}
	// sample at most 3000 faces for speed
	step := len(all)/3000 + 1
	for _, method := range []string{"centre", "k1", "k3", "k5"} {
		var own, other []float32
		for n := 0; n < len(all); n += step {
			f := all[n]
			if len(byPerson[f.p]) < 3 {
				continue
			}
			var o, best float32 = -1, -1
			for p, ex := range byPerson {
				var sc float32
				skip := -1
				if p == f.p {
					skip = f.i
				}
				switch method {
				case "centre":
					c := centre(ex, skip)
					if c == nil {
						continue
					}
					sc = faces.Similarity(c, f.e)
				case "k1":
					sc = topk(f.e, ex, skip, 1)
				case "k3":
					sc = topk(f.e, ex, skip, 3)
				case "k5":
					sc = topk(f.e, ex, skip, 5)
				}
				if p == f.p {
					o = sc
				} else if sc > best {
					best = sc
				}
			}
			own = append(own, o)
			other = append(other, best)
		}
		for _, th := range []float32{0.40, 0.45, 0.50, 0.55, 0.60, 0.65} {
			right, wrong, none := 0, 0, 0
			for i := range own {
				switch {
				case own[i] >= th && own[i] >= other[i]:
					right++
				case other[i] >= th && other[i] > own[i]:
					wrong++
				default:
					none++
				}
			}
			n := float64(len(own))
			t.Logf("%-6s t=%.2f  right %5.1f%%  wrong %4.1f%%  none %5.1f%%", method, th, 100*float64(right)/n, 100*float64(wrong)/n, 100*float64(none)/n)
		}
	}
}

func TestProbeSuggestTimings(t *testing.T) {
	path := os.Getenv("PROBE_DB")
	if path == "" {
		t.Skip("set PROBE_DB")
	}
	db, err := openDB(path)
	if err != nil {
		t.Fatal(err)
	}
	s := newStore(db, "")
	people, _ := s.People()
	timed := func(name string, f func() error) {
		start := time.Now()
		if err := f(); err != nil {
			t.Errorf("%s: %v", name, err)
		}
		t.Logf("%-28s %6d ms", name, time.Since(start).Milliseconds())
	}
	timed("lookalikes (cold)", func() error { _, err := s.lookalikes(); return err })
	timed("lookalikes (warm)", func() error { _, err := s.lookalikes(); return err })
	timed("FaceGroups 60", func() error { _, err := s.FaceGroups(1, 60); return err })
	for _, p := range people[:3] {
		var n int
		timed("MaybeFaces "+p.Name[:1]+"…", func() error { r, err := s.MaybeFaces(p.ID); n = len(r); return err })
		t.Logf("  %d faces, %d maybe", p.Faces, n)
	}
	var photo string
	s.db.QueryRow(`SELECT photo_id FROM faces WHERE person_id IS NULL AND confirmed = 0 LIMIT 1`).Scan(&photo)
	timed("PhotoFaces (unnamed)", func() error { _, err := s.PhotoFaces(photo); return err })
}
