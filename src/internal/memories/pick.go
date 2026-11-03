package memories

import (
	"hash/fnv"
	"sort"
	"time"

	"huepattl.de/unterlumen/internal/appearance"
	"huepattl.de/unterlumen/internal/library"
)

const (
	momentGap = 30 * time.Minute // a longer pause starts a new moment
	sameLook  = 10               // dHash bits at most for two photos that look the same (ADR-0059)
	perPerson = 100              // a named person outweighs any rarity
	jitter    = 0.05             // the day's share in the choice among equals
)

// notMemories are the things that make a photo no memory: what was
// photographed to keep it, and what failed (things.Quality).
var notMemories = map[string]bool{"screenshot": true, "document": true, "blurred": true, "underexposed": true, "overexposed": true}

// Eligible says the photo may be a memory.
func Eligible(p library.MemoryPhoto) bool {
	for _, t := range p.Things {
		if notMemories[t] {
			return false
		}
	}
	return true
}

// eligible keeps the photos that may be memories.
func eligible(photos []library.MemoryPhoto) []library.MemoryPhoto {
	out := []library.MemoryPhoto{}
	for _, p := range photos {
		if Eligible(p) {
			out = append(out, p)
		}
	}
	return out
}

// Score ranks a photo: its named people first, then its rarity, then the
// seed's choice among equals.
func Score(p library.MemoryPhoto, r Rarity, seed string) float64 {
	return perPerson*float64(len(p.People)) + r.Of(p) + jitter*unit(seed+"|"+p.ID)
}

// unit maps a string to [0, 1), the same every time.
func unit(s string) float64 {
	h := fnv.New64a()
	h.Write([]byte(s))
	return float64(h.Sum64()>>11) / (1 << 53)
}

// Pick chooses up to n photos among cands, best first: one per moment, none
// that looks like one already chosen.
func Pick(cands []library.MemoryPhoto, r Rarity, seed string, n int) []library.MemoryPhoto {
	scored := []scoredPhoto{}
	for _, m := range moments(eligible(cands)) {
		scored = append(scored, bestOf(m, r, seed))
	}
	sort.SliceStable(scored, func(i, j int) bool { return scored[i].score > scored[j].score })
	picks := []library.MemoryPhoto{}
	for _, s := range scored {
		if len(picks) == n {
			break
		}
		if !looksLikeAny(s.photo, picks) {
			picks = append(picks, s.photo)
		}
	}
	return picks
}

type scoredPhoto struct {
	photo library.MemoryPhoto
	score float64
}

func bestOf(moment []library.MemoryPhoto, r Rarity, seed string) scoredPhoto {
	best := scoredPhoto{score: -1}
	for _, p := range moment {
		if s := Score(p, r, seed); s > best.score {
			best = scoredPhoto{p, s}
		}
	}
	return best
}

// moments sorts the photos by time and cuts them where more than momentGap
// passes. A photo whose date cannot be read is a moment of its own.
func moments(photos []library.MemoryPhoto) [][]library.MemoryPhoto {
	sorted := append([]library.MemoryPhoto{}, photos...)
	sort.SliceStable(sorted, func(i, j int) bool { return sorted[i].Taken < sorted[j].Taken })
	var out [][]library.MemoryPhoto
	var last time.Time
	for _, p := range sorted {
		at, ok := takenAt(p)
		if len(out) == 0 || !ok || last.IsZero() || at.Sub(last) > momentGap {
			out = append(out, nil)
		}
		out[len(out)-1] = append(out[len(out)-1], p)
		if ok {
			last = at
		} else {
			last = time.Time{}
		}
	}
	return out
}

// takenAt reads the time a photo was taken, local and without a zone.
func takenAt(p library.MemoryPhoto) (time.Time, bool) {
	if len(p.Taken) < 19 {
		return time.Time{}, false
	}
	t, err := time.Parse("2006-01-02T15:04:05", p.Taken[:19])
	return t, err == nil
}

func looksLikeAny(p library.MemoryPhoto, chosen []library.MemoryPhoto) bool {
	for _, c := range chosen {
		if p.Hashed && c.Hashed && appearanceDistance(p, c) <= sameLook {
			return true
		}
	}
	return false
}

func appearanceDistance(a, b library.MemoryPhoto) int {
	return appearance.HashDistance(a.DHash, b.DHash)
}
