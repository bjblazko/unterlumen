package memories

import (
	"math"
	"strconv"

	"huepattl.de/unterlumen/internal/library"
)

// Rarity knows how often each feature a photo can have occurs among the
// photos to choose from: each thing, a main hue, a low or high key, being
// away from home.
type Rarity struct {
	share map[string]float64
	home  Home
}

// NewRarity counts the features of the eligible photos in all.
func NewRarity(all []library.MemoryPhoto, home Home) Rarity {
	counts := map[string]int{}
	n := 0
	for _, p := range all {
		if !Eligible(p) {
			continue
		}
		n++
		for _, f := range features(p, home) {
			counts[f]++
		}
	}
	share := make(map[string]float64, len(counts))
	for f, c := range counts {
		share[f] = float64(c) / float64(n)
	}
	return Rarity{share: share, home: home}
}

// Of is the rarity of the photo's rarest feature, −ln of its share; 0 for
// a photo with none. The maximum rather than the sum, so a photo is not
// rare for showing many things.
func (r Rarity) Of(p library.MemoryPhoto) float64 {
	best := 0.0
	for _, f := range features(p, r.home) {
		if s, ok := r.share[f]; ok && s > 0 {
			best = math.Max(best, -math.Log(s))
		}
	}
	return best
}

func features(p library.MemoryPhoto, home Home) []string {
	out := append([]string{}, p.Things...)
	if p.HueBin >= 0 {
		out = append(out, "hue:"+strconv.Itoa(p.HueBin))
	}
	if p.Tone == "low" || p.Tone == "high" {
		out = append(out, "tone:"+p.Tone)
	}
	if home.away(p) {
		out = append(out, "away")
	}
	return out
}
