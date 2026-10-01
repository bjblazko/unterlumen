package appearance

import (
	"math"
	"sort"
)

// Key is a photo's overall brightness as photographers name it.
type Key string

const (
	LowKey    Key = "low"
	NormalKey Key = "normal"
	HighKey   Key = "high"
)

// Tone is how light and dark are spread over a photo. Lightness is OKLab
// L, 0 for black and 1 for white; middle grey is about 0.6.
type Tone struct {
	Mean, Median float64
	P05, P95     float64 // the darkest and the lightest 5 % start here
	Contrast     float64 // standard deviation of lightness
	Key          Key
	ClipHigh     float64 // share of pure white pixels
	ClipLow      float64 // share of pure black pixels
	Histogram    [16]uint8
}

const (
	highKeyMedian = 0.75
	lowKeyMedian  = 0.35
	darkL         = 0.35 // a high-key photo has few pixels below this
	highKeyDark   = 0.10
	clipHigh      = 250 // every channel at or above: pure white
	clipLow       = 5   // every channel at or below: pure black
)

func measureTone(px pixels) Tone {
	ls := make([]float64, len(px.lab))
	var sum, sum2 float64
	var t Tone
	for i, c := range px.lab {
		ls[i] = c.L
		sum += c.L
		sum2 += c.L * c.L
	}
	n := float64(max(len(ls), 1))
	sort.Float64s(ls)
	t.Mean = sum / n
	t.Contrast = math.Sqrt(math.Max(sum2/n-t.Mean*t.Mean, 0))
	t.Median = percentile(ls, 0.5)
	t.P05 = percentile(ls, 0.05)
	t.P95 = percentile(ls, 0.95)
	t.Key = toneKey(t.Median, shareBelow(ls, darkL))
	t.ClipHigh, t.ClipLow = clipping(px.rgb)
	t.Histogram = histogram(px.lab)
	return t
}

func toneKey(median, darkShare float64) Key {
	switch {
	case median >= highKeyMedian && darkShare < highKeyDark:
		return HighKey
	case median <= lowKeyMedian:
		return LowKey
	}
	return NormalKey
}

// shareBelow is the share of sorted values below v.
func shareBelow(sorted []float64, v float64) float64 {
	if len(sorted) == 0 {
		return 0
	}
	return float64(sort.SearchFloat64s(sorted, v)) / float64(len(sorted))
}

func clipping(rgb [][3]uint8) (high, low float64) {
	var h, l int
	for _, p := range rgb {
		if p[0] >= clipHigh && p[1] >= clipHigh && p[2] >= clipHigh {
			h++
		}
		if p[0] <= clipLow && p[1] <= clipLow && p[2] <= clipLow {
			l++
		}
	}
	n := float64(max(len(rgb), 1))
	return float64(h) / n, float64(l) / n
}

// histogram counts lightness in 16 bins and scales the counts so the
// fullest bin is 255: the shape, in 16 bytes.
func histogram(lab []Lab) [16]uint8 {
	var counts [16]int
	for _, c := range lab {
		counts[min(max(int(c.L*16), 0), 15)]++
	}
	top := 0
	for _, c := range counts {
		top = max(top, c)
	}
	var h [16]uint8
	if top == 0 {
		return h
	}
	for i, c := range counts {
		h[i] = uint8((c*255 + top/2) / top)
	}
	return h
}
