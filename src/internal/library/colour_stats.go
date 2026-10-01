package library

import (
	"math"
	"sort"
	"strconv"
)

// What the Colour statistics count as warm, cool or a main colour. The
// search uses the same values, so a click shows exactly the photos counted.
const (
	WarmthThreshold = 0.15 // warm above it, cool below its negative
	HueShareMin     = 0.2  // a swatch this large makes its hue a main colour of the photo
)

// LCh is a colour in OKLCh: lightness 0…1, chroma, hue in degrees.
type LCh struct {
	L float64 `json:"l"`
	C float64 `json:"c"`
	H float64 `json:"h"`
}

// LibraryColour is the Colour topic of the statistics.
type LibraryColour struct {
	Granularity      string             `json:"granularity"`
	Periods          []string           `json:"periods"`
	Classes          []ColourClassSlice `json:"classes"`
	PeriodColours    []PeriodColour     `json:"periodColours"`
	HueWheel         []HueSector        `json:"hueWheel"`
	NeutralShare     float64            `json:"neutralShare"`
	Seasons          []SeasonBar        `json:"seasons"`       // all years together
	SeasonsByYear    []YearSeasons      `json:"seasonsByYear"` // each year with colour photos, oldest first
	AnalysedPhotos   int                `json:"analysedPhotos"`
	UnanalysedPhotos int                `json:"unanalysedPhotos"`
}

// ColourClassSlice counts the photos of one class (mono, tinted, colour) per
// period, aligned to LibraryColour.Periods.
type ColourClassSlice struct {
	Class  string `json:"class"`
	Counts []int  `json:"counts"`
}

// PeriodColour is the colour of a period's colour photos.
type PeriodColour struct {
	Period string `json:"period"`
	Colour LCh    `json:"colour"`
	Photos int    `json:"photos"`
}

// HueSector is one 30° sector of the hue wheel: the part of all colour
// photos' area its swatches cover, their colour, and the photos it is a main
// colour of (a swatch of at least HueShareMin) — those a click shows.
type HueSector struct {
	Bin    int     `json:"bin"`
	Share  float64 `json:"share"`
	Colour LCh     `json:"colour"`
	Photos int     `json:"photos"`
}

// YearSeasons is the twelve months of one year.
type YearSeasons struct {
	Year    string      `json:"year"`
	Seasons []SeasonBar `json:"seasons"`
}

// SeasonBar is one month of the year, all years together: how many colour
// photos were warm and how many cool, and the colour of each group.
type SeasonBar struct {
	Month      int  `json:"month"`
	Photos     int  `json:"photos"`
	Warm       int  `json:"warm"`
	Cool       int  `json:"cool"`
	WarmColour *LCh `json:"warmColour,omitempty"`
	CoolColour *LCh `json:"coolColour,omitempty"`
}

var colourClasses = []string{"mono", "tinted", "colour"}

// BuildColour aggregates the measured photos of one or more libraries.
// granularity is "month", "year", or "" to choose by the span of dates.
func BuildColour(src *ColourSource, granularity string) *LibraryColour {
	dated := datedPhotos(src.Photos)
	if granularity != "month" && granularity != "year" {
		granularity = colourGranularity(dated)
	}
	n := 7
	if granularity == "year" {
		n = 4
	}
	periods, byPeriod := groupByPeriod(dated, n)
	hue, neutral := hueWheel(src.Swatches)
	main := hueSwatchesByPhoto(src.Swatches)
	return &LibraryColour{
		Granularity:      granularity,
		Periods:          periods,
		Classes:          classCounts(periods, byPeriod),
		PeriodColours:    periodColours(periods, byPeriod, main),
		HueWheel:         hue,
		NeutralShare:     neutral,
		Seasons:          seasons(dated, main),
		SeasonsByYear:    seasonsByYear(dated, main),
		AnalysedPhotos:   len(src.Photos),
		UnanalysedPhotos: src.Unanalysed,
	}
}

// datedPhotos keeps the photos whose date reads as at least a month.
func datedPhotos(photos []ColourPhoto) []ColourPhoto {
	out := make([]ColourPhoto, 0, len(photos))
	for _, p := range photos {
		if len(p.Date) >= 7 {
			out = append(out, p)
		}
	}
	return out
}

func colourGranularity(dated []ColourPhoto) string {
	if len(dated) == 0 {
		return "month"
	}
	lo, hi := dated[0].Date[:7], dated[0].Date[:7]
	for _, p := range dated[1:] {
		lo, hi = min(lo, p.Date[:7]), max(hi, p.Date[:7])
	}
	return granularityForSpan(lo, hi)
}

func groupByPeriod(dated []ColourPhoto, n int) ([]string, map[string][]ColourPhoto) {
	by := make(map[string][]ColourPhoto)
	for _, p := range dated {
		by[p.Date[:n]] = append(by[p.Date[:n]], p)
	}
	periods := make([]string, 0, len(by))
	for p := range by {
		periods = append(periods, p)
	}
	sort.Strings(periods)
	return periods, by
}

func classCounts(periods []string, by map[string][]ColourPhoto) []ColourClassSlice {
	out := make([]ColourClassSlice, len(colourClasses))
	for ci, class := range colourClasses {
		counts := make([]int, len(periods))
		for pi, period := range periods {
			for _, p := range by[period] {
				if p.Mono == class {
					counts[pi]++
				}
			}
		}
		out[ci] = ColourClassSlice{Class: class, Counts: counts}
	}
	return out
}

// hueSwatchesByPhoto keeps the swatches that have a hue, by photo.
func hueSwatchesByPhoto(swatches []ColourSwatch) map[string][]ColourSwatch {
	by := make(map[string][]ColourSwatch)
	for _, sw := range swatches {
		if sw.HueBin >= 0 {
			by[sw.PhotoID] = append(by[sw.PhotoID], sw)
		}
	}
	return by
}

// mainColour is the vivid mean of the photos' main colours that keep
// passes (nil keeps all), each weighted by how much of its photo it covers.
// A photo's average colour would not do: red cherries on green leaves
// average to a brownish grey before any mean of photos is taken. ok is
// false when none of the photos has such a colour.
func mainColour(photos []ColourPhoto, main map[string][]ColourSwatch, keep func(LCh) bool) (c LCh, ok bool) {
	var cols []LCh
	var weights []float64
	for _, p := range photos {
		for _, sw := range main[p.ID] {
			if keep == nil || keep(sw.Colour) {
				cols = append(cols, sw.Colour)
				weights = append(weights, sw.Share)
			}
		}
	}
	if len(cols) == 0 {
		return LCh{}, false
	}
	return vividMean(cols, weights), true
}

// A hue leans warm within 60° of orange-yellow (60°, the axis the warmth of
// a photo is measured along): red, orange, amber, yellow. It leans cool
// within 60° of the opposite: teal, sky blue, blue, violet.
func leansWarm(c LCh) bool { return math.Cos((c.H-60)*math.Pi/180) > 0.5 }
func leansCool(c LCh) bool { return math.Cos((c.H-60)*math.Pi/180) < -0.5 }

func colourOnly(photos []ColourPhoto) []ColourPhoto {
	var out []ColourPhoto
	for _, p := range photos {
		if p.Mono == "colour" {
			out = append(out, p)
		}
	}
	return out
}

// periodColours gives each period the colour of its colour photos. A period
// whose colour photos have no colour with a hue is a gap.
func periodColours(periods []string, by map[string][]ColourPhoto, main map[string][]ColourSwatch) []PeriodColour {
	out := []PeriodColour{}
	for _, period := range periods {
		photos := colourOnly(by[period])
		if c, ok := mainColour(photos, main, nil); ok {
			out = append(out, PeriodColour{Period: period, Colour: c, Photos: len(photos)})
		}
	}
	return out
}

// hueWheel sums the swatch shares per hue sector, as parts of all swatches'
// area, and gives the neutrals' part separately.
func hueWheel(swatches []ColourSwatch) ([]HueSector, float64) {
	var total, neutral float64
	cols := make([][]LCh, 12)
	weights := make([][]float64, 12)
	main := make([]map[string]bool, 12)
	for _, sw := range swatches {
		total += sw.Share
		if sw.HueBin < 0 || sw.HueBin > 11 {
			neutral += sw.Share
			continue
		}
		if sw.Share >= HueShareMin {
			if main[sw.HueBin] == nil {
				main[sw.HueBin] = map[string]bool{}
			}
			main[sw.HueBin][sw.PhotoID] = true
		}
		cols[sw.HueBin] = append(cols[sw.HueBin], sw.Colour)
		weights[sw.HueBin] = append(weights[sw.HueBin], sw.Share)
	}
	out := []HueSector{}
	if total == 0 {
		return out, 0
	}
	for bin := range cols {
		if len(cols[bin]) == 0 {
			continue
		}
		var sum float64
		for _, w := range weights[bin] {
			sum += w
		}
		out = append(out, HueSector{Bin: bin, Share: sum / total, Colour: vividMean(cols[bin], weights[bin]), Photos: len(main[bin])})
	}
	return out, neutral / total
}

// seasons counts warm and cool colour photos per month of the year.
func seasons(dated []ColourPhoto, main map[string][]ColourSwatch) []SeasonBar {
	out := make([]SeasonBar, 12)
	warm := make([][]ColourPhoto, 12)
	cool := make([][]ColourPhoto, 12)
	for i := range out {
		out[i].Month = i + 1
	}
	for _, p := range dated {
		m, err := strconv.Atoi(p.Date[5:7])
		if err != nil || m < 1 || m > 12 || p.Mono != "colour" {
			continue
		}
		out[m-1].Photos++
		switch {
		case p.Warmth > WarmthThreshold:
			warm[m-1] = append(warm[m-1], p)
		case p.Warmth < -WarmthThreshold:
			cool[m-1] = append(cool[m-1], p)
		}
	}
	for i := range out {
		out[i].Warm, out[i].Cool = len(warm[i]), len(cool[i])
		// A bar shows what makes its photos warm or cool: their warm or cool
		// colours, not the blue sky of a warm photo.
		out[i].WarmColour = mainColourOrNil(warm[i], main, leansWarm)
		out[i].CoolColour = mainColourOrNil(cool[i], main, leansCool)
	}
	return out
}

// seasonsByYear gives the months of each year that has colour photos.
func seasonsByYear(dated []ColourPhoto, main map[string][]ColourSwatch) []YearSeasons {
	years, by := groupByPeriod(colourOnly(dated), 4)
	out := make([]YearSeasons, 0, len(years))
	for _, y := range years {
		out = append(out, YearSeasons{Year: y, Seasons: seasons(by[y], main)})
	}
	return out
}

func mainColourOrNil(photos []ColourPhoto, main map[string][]ColourSwatch, keep func(LCh) bool) *LCh {
	c, ok := mainColour(photos, main, keep)
	if !ok {
		return nil
	}
	return &c
}

// vividChroma is the weighted percentile of chroma a mark is drawn with:
// the colour as it appears at its most colourful in the photos, not the
// typical swatch, which is dull in any photo library (owner's decision,
// 2026-10-01).
const vividChroma = 0.9

// vividMean averages colours in OKLab for lightness and hue, but takes the
// chroma at vividChroma: a plain mean of many hues greys out towards brown.
// Weights may be nil for equal weights.
func vividMean(cols []LCh, weights []float64) LCh {
	w := func(i int) float64 {
		if weights == nil {
			return 1
		}
		return weights[i]
	}
	var l, a, b, sum float64
	for i, c := range cols {
		rad := c.H * math.Pi / 180
		l += w(i) * c.L
		a += w(i) * c.C * math.Cos(rad)
		b += w(i) * c.C * math.Sin(rad)
		sum += w(i)
	}
	if sum == 0 {
		return LCh{}
	}
	h := math.Atan2(b, a) * 180 / math.Pi
	if h < 0 {
		h += 360
	}
	return LCh{L: l / sum, C: weightedChromaAt(cols, w, vividChroma), H: h}
}

// weightedChromaAt is the chroma below which the fraction q of the weight lies.
func weightedChromaAt(cols []LCh, w func(int) float64, q float64) float64 {
	idx := make([]int, len(cols))
	var total float64
	for i := range idx {
		idx[i] = i
		total += w(i)
	}
	sort.Slice(idx, func(x, y int) bool { return cols[idx[x]].C < cols[idx[y]].C })
	var acc float64
	for _, i := range idx {
		acc += w(i)
		if acc >= total*q {
			return cols[i].C
		}
	}
	return 0
}
