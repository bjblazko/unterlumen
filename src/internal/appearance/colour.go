package appearance

import (
	"math"
	"sort"
)

// MonoClass says whether a photo is in colour.
type MonoClass string

const (
	Mono   MonoClass = "mono"   // black and white
	Tinted MonoClass = "tinted" // one hue throughout: sepia, cyanotype, a toned black and white
	Colour MonoClass = "colour"
)

// Calibrated on the e2e fixtures: a photo converted to black and white has
// a 99th-percentile chroma of 0.000, the dullest colour photo 0.021. A warm
// sunset has its hues within 8–14°; toning puts them all on one.
const (
	monoChroma     = 0.012 // below this at the 99th percentile, a photo has no colour
	tintMaxChroma  = 0.08  // a toned photo's 99th-percentile chroma stays below this
	tintMaxSpread  = 6.0   // and its hues lie within this many degrees
	chromaticFloor = 0.01  // pixels below this carry no hue worth counting
)

// monoClass sorts a photo into black and white, toned, or colour, and gives
// the hue of a toned one.
func monoClass(lab []Lab) (MonoClass, float64) {
	chromas := make([]float64, len(lab))
	for i, c := range lab {
		chromas[i] = c.Chroma()
	}
	sort.Float64s(chromas)
	p99 := percentile(chromas, 0.99)
	if p99 < monoChroma {
		return Mono, 0
	}
	hue, spread := hueSpread(lab)
	if spread < tintMaxSpread && p99 < tintMaxChroma {
		return Tinted, hue
	}
	return Colour, 0
}

// hueSpread gives the chroma-weighted mean hue of the coloured pixels and
// the circular standard deviation of their hues, both in degrees.
func hueSpread(lab []Lab) (hue, spread float64) {
	var sumA, sumB, weight float64
	for _, c := range lab {
		ch := c.Chroma()
		if ch < chromaticFloor {
			continue
		}
		// a/ch and b/ch are the hue's unit vector; weighting by ch leaves a, b.
		sumA += c.A
		sumB += c.B
		weight += ch
	}
	if weight == 0 {
		return 0, 360
	}
	r := math.Hypot(sumA, sumB) / weight
	spread = math.Sqrt(-2*math.Log(math.Max(r, 1e-9))) * 180 / math.Pi
	return Lab{A: sumA, B: sumB}.Hue(), spread
}

// averageColour is the mean of all pixels in OKLab.
func averageColour(lab []Lab) Lab {
	var sum Lab
	for _, c := range lab {
		sum.L += c.L
		sum.A += c.A
		sum.B += c.B
	}
	n := float64(max(len(lab), 1))
	return Lab{sum.L / n, sum.A / n, sum.B / n}
}

// colourfulness is the measure of Hasler and Süsstrunk (2003), which
// matches how colourful people judge a photo to be.
func colourfulness(rgb [][3]uint8) float64 {
	var sRG, sYB, sRG2, sYB2 float64
	for _, p := range rgb {
		r, g, b := float64(p[0]), float64(p[1]), float64(p[2])
		rg := r - g
		yb := 0.5*(r+g) - b
		sRG += rg
		sYB += yb
		sRG2 += rg * rg
		sYB2 += yb * yb
	}
	n := float64(max(len(rgb), 1))
	mRG, mYB := sRG/n, sYB/n
	vRG := math.Max(sRG2/n-mRG*mRG, 0)
	vYB := math.Max(sYB2/n-mYB*mYB, 0)
	return math.Sqrt(vRG+vYB) + 0.3*math.Hypot(mRG, mYB)
}

// warmAxis points from blue to orange in the a–b plane of OKLab.
var warmAxis = [2]float64{math.Cos(60 * math.Pi / 180), math.Sin(60 * math.Pi / 180)}

// warmth says how far a photo's colours lean to orange (+1) or blue (-1).
// Grey pixels count for nothing, so a photo with a little colour can still
// be clearly warm.
func warmth(lab []Lab) float64 {
	var proj, chroma float64
	for _, c := range lab {
		proj += c.A*warmAxis[0] + c.B*warmAxis[1]
		chroma += c.Chroma()
	}
	if chroma < 1e-9 {
		return 0
	}
	return proj / chroma
}

// percentile reads the p-th quantile (0…1) from sorted values.
func percentile(sorted []float64, p float64) float64 {
	if len(sorted) == 0 {
		return 0
	}
	return sorted[int(p*float64(len(sorted)-1)+0.5)]
}
