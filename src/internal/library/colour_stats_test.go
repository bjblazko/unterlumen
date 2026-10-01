package library

import (
	"math"
	"testing"
)

func near(a, b, tol float64) bool { return math.Abs(a-b) <= tol }

// Two greens and a red: the hue stays on the green side and the chroma is
// the most colourful one's, not the greyed-out mean of opposite hues.
func TestVividMean(t *testing.T) {
	got := vividMean([]LCh{{0.6, 0.15, 140}, {0.7, 0.12, 145}, {0.5, 0.10, 30}}, nil)
	if !near(got.L, 0.6, 1e-9) {
		t.Errorf("L = %v, want 0.6", got.L)
	}
	if got.C != 0.15 {
		t.Errorf("C = %v, want the 90th percentile 0.15", got.C)
	}
	if got.H < 110 || got.H > 145 {
		t.Errorf("H = %v, want on the green side", got.H)
	}
}

// A dull swatch covering most of the area keeps the chroma down.
func TestVividMeanWeighted(t *testing.T) {
	got := vividMean([]LCh{{0.5, 0.05, 60}, {0.5, 0.20, 60}}, []float64{0.95, 0.05})
	if got.C != 0.05 || !near(got.H, 60, 1e-9) {
		t.Errorf("got %+v, want chroma 0.05 at 60°", got)
	}
}

func colourFixture() *ColourSource {
	return &ColourSource{
		Photos: []ColourPhoto{
			{ID: "bw", Date: "2024-01-05T10:00:00", Mono: "mono"},
			{ID: "a", Date: "2024-01-06T10:00:00", Mono: "colour", Warmth: 0.5},
			{ID: "b", Date: "2024-01-07T10:00:00", Mono: "colour", Warmth: -0.15},   // at the edge: neutral
			{ID: "toned", Date: "2024-03-01T10:00:00", Mono: "tinted", Warmth: 0.9}, // not counted as warm
			{ID: "c", Date: "2024-03-02T10:00:00", Mono: "colour", Warmth: -0.4},    // cool, but no colour with a hue
			{ID: "undated", Date: "", Mono: "colour", Warmth: 0.8},                  // in no period and no month
		},
		Swatches: []ColourSwatch{
			{PhotoID: "a", HueBin: 2, Share: 0.5, Colour: LCh{0.7, 0.1, 70}},
			{PhotoID: "a", HueBin: 2, Share: 0.3, Colour: LCh{0.6, 0.1, 80}},
			{PhotoID: "b", HueBin: 8, Share: 0.1, Colour: LCh{0.5, 0.1, 255}},
			{PhotoID: "b", HueBin: -1, Share: 0.1, Colour: LCh{0.9, 0.01, 0}},
			{PhotoID: "c", HueBin: -1, Share: 0.9, Colour: LCh{0.5, 0.01, 0}},
		},
		Unanalysed: 4,
	}
}

func TestBuildColourPeriodsAndClasses(t *testing.T) {
	c := BuildColour(colourFixture(), "")
	if c.Granularity != "month" || len(c.Periods) != 2 || c.Periods[0] != "2024-01" || c.Periods[1] != "2024-03" {
		t.Fatalf("periods %v (%s)", c.Periods, c.Granularity)
	}
	want := map[string][]int{"mono": {1, 0}, "tinted": {0, 1}, "colour": {2, 1}}
	for _, cs := range c.Classes {
		if w := want[cs.Class]; w[0] != cs.Counts[0] || w[1] != cs.Counts[1] {
			t.Errorf("%s: %v, want %v", cs.Class, cs.Counts, w)
		}
	}
	// March's one colour photo has only greys, so March is a gap.
	if len(c.PeriodColours) != 1 || c.PeriodColours[0].Period != "2024-01" || c.PeriodColours[0].Photos != 2 {
		t.Errorf("period colours %+v", c.PeriodColours)
	}
	if c.AnalysedPhotos != 6 || c.UnanalysedPhotos != 4 {
		t.Errorf("analysed %d, unanalysed %d", c.AnalysedPhotos, c.UnanalysedPhotos)
	}
}

func TestBuildColourYears(t *testing.T) {
	src := &ColourSource{Photos: []ColourPhoto{
		{Date: "2015-06-01", Mono: "colour"}, {Date: "2024-06-01", Mono: "colour"},
	}}
	if c := BuildColour(src, ""); c.Granularity != "year" || c.Periods[0] != "2015" {
		t.Errorf("got %s %v, want years", c.Granularity, c.Periods)
	}
	if c := BuildColour(src, "month"); c.Periods[0] != "2015-06" {
		t.Errorf("asked for months, got %v", c.Periods)
	}
}

// The colour of a period comes from its photos' main colours, weighted by
// share, not from their average colours, which are greyed out already.
func TestBuildColourPeriodFromMainColours(t *testing.T) {
	got := BuildColour(colourFixture(), "").PeriodColours[0].Colour
	if got.C != 0.1 {
		t.Errorf("chroma %v, want the swatches' 0.1", got.C)
	}
	if got.H < 70 || got.H > 80 {
		t.Errorf("hue %v, want leaning to the large orange swatches", got.H)
	}
}

// A warm bar takes only its photos' warm colours: a warm photo's blue sky
// does not turn the bar grey.
func TestSeasonBarsTakeWarmAndCoolColours(t *testing.T) {
	src := &ColourSource{
		Photos: []ColourPhoto{{ID: "w", Date: "2024-06-01", Mono: "colour", Warmth: 0.5}},
		Swatches: []ColourSwatch{
			{PhotoID: "w", HueBin: 1, Share: 0.4, Colour: LCh{0.7, 0.15, 55}},
			{PhotoID: "w", HueBin: 8, Share: 0.6, Colour: LCh{0.6, 0.12, 250}},
		},
	}
	jun := BuildColour(src, "").Seasons[5]
	if jun.WarmColour == nil || !near(jun.WarmColour.H, 55, 1e-9) || jun.WarmColour.C != 0.15 {
		t.Errorf("warm colour %+v, want the orange swatch alone", jun.WarmColour)
	}
}

func TestBuildColourSeasonsByYear(t *testing.T) {
	src := &ColourSource{Photos: []ColourPhoto{
		{ID: "a", Date: "2023-06-01", Mono: "colour", Warmth: 0.5},
		{ID: "b", Date: "2024-06-01", Mono: "colour", Warmth: -0.5},
		{ID: "c", Date: "2024-07-01", Mono: "mono"},
	}}
	by := BuildColour(src, "").SeasonsByYear
	if len(by) != 2 || by[0].Year != "2023" || by[1].Year != "2024" {
		t.Fatalf("years %+v", by)
	}
	if by[0].Seasons[5].Warm != 1 || by[1].Seasons[5].Cool != 1 || by[1].Seasons[6].Photos != 0 {
		t.Errorf("June 2023 %+v, June 2024 %+v, July 2024 %+v", by[0].Seasons[5], by[1].Seasons[5], by[1].Seasons[6])
	}
}

func TestLeansWarmAndCool(t *testing.T) {
	for _, h := range []float64{5, 29, 55, 84, 110} {
		if !leansWarm(LCh{H: h}) || leansCool(LCh{H: h}) {
			t.Errorf("%v° should lean warm", h)
		}
	}
	for _, h := range []float64{195, 226, 264, 294} {
		if !leansCool(LCh{H: h}) || leansWarm(LCh{H: h}) {
			t.Errorf("%v° should lean cool", h)
		}
	}
	for _, h := range []float64{142, 330} { // green, magenta: neither
		if leansWarm(LCh{H: h}) || leansCool(LCh{H: h}) {
			t.Errorf("%v° should lean neither way", h)
		}
	}
}

func TestBuildColourHueWheel(t *testing.T) {
	c := BuildColour(colourFixture(), "")
	// Swatch area in all: 0.8 orange, 0.1 blue, 1.0 neutral.
	if !near(c.NeutralShare, 1.0/1.9, 1e-9) {
		t.Errorf("neutral = %v, want %v", c.NeutralShare, 1.0/1.9)
	}
	if len(c.HueWheel) != 2 || c.HueWheel[0].Bin != 2 || !near(c.HueWheel[0].Share, 0.8/1.9, 1e-9) || !near(c.HueWheel[1].Share, 0.1/1.9, 1e-9) {
		t.Errorf("wheel %+v", c.HueWheel)
	}
	// Two large orange swatches of one photo count it once; a small blue
	// one makes blue no main colour of any photo.
	if c.HueWheel[0].Photos != 1 || c.HueWheel[1].Photos != 0 {
		t.Errorf("photos per sector %d, %d; want 1, 0", c.HueWheel[0].Photos, c.HueWheel[1].Photos)
	}
	if c.HueWheel[0].Colour.H < 72 || c.HueWheel[0].Colour.H > 76 {
		t.Errorf("orange sector hue %v, want near 70 (weighted to the larger swatch)", c.HueWheel[0].Colour.H)
	}
}

func TestBuildColourSeasons(t *testing.T) {
	s := BuildColour(colourFixture(), "").Seasons
	if len(s) != 12 {
		t.Fatalf("%d months", len(s))
	}
	jan, mar := s[0], s[2]
	if jan.Photos != 2 || jan.Warm != 1 || jan.Cool != 0 || jan.WarmColour == nil || jan.CoolColour != nil {
		t.Errorf("January %+v", jan)
	}
	if mar.Photos != 1 || mar.Warm != 0 || mar.Cool != 1 || mar.CoolColour != nil {
		t.Errorf("March %+v", mar)
	}
}
