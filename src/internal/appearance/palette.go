package appearance

import (
	"math"
	"math/rand"
	"sort"
)

// Swatch is one of a photo's main colours and how much of the photo it covers.
type Swatch struct {
	Colour Lab
	Share  float64 // 0…1
}

// HueBin is the 30° sector the swatch's hue lies in, 0 to 11 starting at 0°,
// or -1 for a neutral that has no hue worth naming.
func (s Swatch) HueBin() int {
	if s.Colour.Chroma() < NeutralChroma {
		return -1
	}
	return int(s.Colour.Hue()/30) % 12
}

// NeutralChroma is the chroma below which a colour is a grey, a black or a
// white rather than a hue.
const NeutralChroma = 0.03

const (
	paletteK          = 5
	paletteSamples    = 4096
	paletteIterations = 12
	paletteMergeDist  = 0.06 // closer swatches, in the weighted space, are one colour
	paletteMinShare   = 0.03 // smaller swatches are left out
	// hueWeight stretches a and b against L while clustering. Lightness spans
	// 1, chroma rarely 0.3, so plain OKLab splits a photo into five
	// brightnesses of one hue instead of its colours.
	hueWeight = 3.0
)

// palette finds a photo's main colours by k-means in OKLab, with the colour
// axes weighted against lightness. It is deterministic: the same photo
// always gives the same swatches.
func palette(lab []Lab) []Swatch {
	samples := weighted(sample(lab, paletteSamples), hueWeight)
	if len(samples) == 0 {
		return nil
	}
	centres := seedCentres(samples, paletteK)
	assign := make([]int, len(samples))
	for range paletteIterations {
		if !assignNearest(samples, centres, assign) {
			break
		}
		centres = recentre(samples, assign, len(centres))
	}
	swatches := keepLarge(mergeClose(toSwatches(samples, centres, assign)))
	for i := range swatches {
		swatches[i].Colour = weighted([]Lab{swatches[i].Colour}, 1/hueWeight)[0]
	}
	return swatches
}

// weighted returns the colours with a and b multiplied by w.
func weighted(lab []Lab, w float64) []Lab {
	out := make([]Lab, len(lab))
	for i, c := range lab {
		out[i] = Lab{c.L, c.A * w, c.B * w}
	}
	return out
}

// sample takes at most n evenly spaced pixels.
func sample(lab []Lab, n int) []Lab {
	if len(lab) <= n {
		return lab
	}
	out := make([]Lab, n)
	step := float64(len(lab)) / float64(n)
	for i := range out {
		out[i] = lab[int(float64(i)*step)]
	}
	return out
}

// seedCentres picks k starting centres the k-means++ way, with a fixed seed.
func seedCentres(samples []Lab, k int) []Lab {
	rng := rand.New(rand.NewSource(1))
	centres := []Lab{samples[rng.Intn(len(samples))]}
	d2 := make([]float64, len(samples))
	for len(centres) < k {
		var sum float64
		for i, s := range samples {
			d2[i] = math.Inf(1)
			for _, c := range centres {
				d2[i] = math.Min(d2[i], s.dist2(c))
			}
			sum += d2[i]
		}
		if sum == 0 {
			break // fewer distinct colours than k
		}
		target := rng.Float64() * sum
		i := 0
		for ; i < len(d2)-1 && target > d2[i]; i++ {
			target -= d2[i]
		}
		centres = append(centres, samples[i])
	}
	return centres
}

// assignNearest gives each sample its nearest centre and says whether any
// assignment changed.
func assignNearest(samples, centres []Lab, assign []int) bool {
	changed := false
	for i, s := range samples {
		best, bestD := 0, math.Inf(1)
		for j, c := range centres {
			if d := s.dist2(c); d < bestD {
				best, bestD = j, d
			}
		}
		if assign[i] != best {
			assign[i] = best
			changed = true
		}
	}
	return changed
}

func recentre(samples []Lab, assign []int, k int) []Lab {
	sums := make([]Lab, k)
	counts := make([]int, k)
	for i, s := range samples {
		j := assign[i]
		sums[j].L += s.L
		sums[j].A += s.A
		sums[j].B += s.B
		counts[j]++
	}
	for j := range sums {
		if n := float64(counts[j]); n > 0 {
			sums[j] = Lab{sums[j].L / n, sums[j].A / n, sums[j].B / n}
		}
	}
	return sums
}

func toSwatches(samples, centres []Lab, assign []int) []Swatch {
	counts := make([]int, len(centres))
	for _, j := range assign {
		counts[j]++
	}
	var out []Swatch
	for j, c := range centres {
		if counts[j] > 0 {
			out = append(out, Swatch{Colour: c, Share: float64(counts[j]) / float64(len(samples))})
		}
	}
	return out
}

// mergeClose joins swatches that look like one colour, weighting each by
// its share.
func mergeClose(sw []Swatch) []Swatch {
	maxD2 := paletteMergeDist * paletteMergeDist
	for merged := true; merged; {
		merged = false
		for i := 0; i < len(sw) && !merged; i++ {
			for j := i + 1; j < len(sw); j++ {
				if sw[i].Colour.dist2(sw[j].Colour) < maxD2 {
					sw[i] = joinSwatches(sw[i], sw[j])
					sw = append(sw[:j], sw[j+1:]...)
					merged = true
					break
				}
			}
		}
	}
	return sw
}

func joinSwatches(a, b Swatch) Swatch {
	t := a.Share + b.Share
	wa, wb := a.Share/t, b.Share/t
	return Swatch{
		Colour: Lab{
			a.Colour.L*wa + b.Colour.L*wb,
			a.Colour.A*wa + b.Colour.A*wb,
			a.Colour.B*wa + b.Colour.B*wb,
		},
		Share: t,
	}
}

// keepLarge drops swatches too small to matter and sorts the rest, largest
// first.
func keepLarge(sw []Swatch) []Swatch {
	out := sw[:0]
	for _, s := range sw {
		if s.Share >= paletteMinShare {
			out = append(out, s)
		}
	}
	sort.SliceStable(out, func(i, j int) bool { return out[i].Share > out[j].Share })
	return out
}
