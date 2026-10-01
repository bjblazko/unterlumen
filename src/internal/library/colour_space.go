package library

import (
	"math"
	"sort"
)

// ColourSpace is the Colour space topic of the statistics: every analysed
// photo as a point in OKLab, and the path of the periods' mean colours.
// Points are stored by column, so tens of thousands stay a small answer.
type ColourSpace struct {
	Granularity      string            `json:"granularity"`
	Libraries        []string          `json:"libraries"` // the ids Points.Lib indexes
	Points           ColourSpacePoints `json:"points"`
	Path             []ColourSpaceStep `json:"path"`
	AnalysedPhotos   int               `json:"analysedPhotos"`
	UnanalysedPhotos int               `json:"unanalysedPhotos"`
}

// ColourSpacePoints holds one entry per photo in every column. L, A and B
// are OKLab.
type ColourSpacePoints struct {
	ID   []string  `json:"id"`
	Lib  []int     `json:"lib"`
	Date []string  `json:"date"`
	Mono []string  `json:"mono"`
	L    []float64 `json:"l"`
	A    []float64 `json:"a"`
	B    []float64 `json:"b"`
}

// ColourSpaceStep is one period on the path: the mean OKLab colour of its
// colour photos.
type ColourSpaceStep struct {
	Period string  `json:"period"`
	L      float64 `json:"l"`
	A      float64 `json:"a"`
	B      float64 `json:"b"`
	Photos int     `json:"photos"`
}

// LibraryPoints is the points of one library.
type LibraryPoints struct {
	LibraryID  string
	Points     []ColourPoint
	Unanalysed int
}

// BuildColourSpace lays out the points of one or more libraries.
// granularity is "month", "year", or "" to choose by the span of dates.
func BuildColourSpace(libs []LibraryPoints, granularity string) *ColourSpace {
	cs := &ColourSpace{Libraries: []string{}, Path: []ColourSpaceStep{}, Points: ColourSpacePoints{
		ID: []string{}, Lib: []int{}, Date: []string{}, Mono: []string{},
		L: []float64{}, A: []float64{}, B: []float64{},
	}}
	var all []ColourPoint
	for i, l := range libs {
		cs.Libraries = append(cs.Libraries, l.LibraryID)
		cs.UnanalysedPhotos += l.Unanalysed
		for _, p := range l.Points {
			cs.Points.add(p, i)
		}
		all = append(all, l.Points...)
	}
	cs.AnalysedPhotos = len(all)
	if granularity != "month" && granularity != "year" {
		granularity = pointGranularity(all)
	}
	cs.Granularity = granularity
	cs.Path = colourPath(all, granularity)
	return cs
}

func (ps *ColourSpacePoints) add(p ColourPoint, lib int) {
	a, b := labAB(p.Colour)
	ps.ID = append(ps.ID, p.ID)
	ps.Lib = append(ps.Lib, lib)
	ps.Date = append(ps.Date, p.Date)
	ps.Mono = append(ps.Mono, p.Mono)
	ps.L = append(ps.L, round4(p.Colour.L))
	ps.A = append(ps.A, round4(a))
	ps.B = append(ps.B, round4(b))
}

func labAB(c LCh) (a, b float64) {
	rad := c.H * math.Pi / 180
	return c.C * math.Cos(rad), c.C * math.Sin(rad)
}

func round4(v float64) float64 { return math.Round(v*1e4) / 1e4 }

func pointGranularity(points []ColourPoint) string {
	lo, hi := "", ""
	for _, p := range points {
		if len(p.Date) < 7 {
			continue
		}
		if lo == "" || p.Date[:7] < lo {
			lo = p.Date[:7]
		}
		hi = max(hi, p.Date[:7])
	}
	if lo == "" {
		return "month"
	}
	return granularityForSpan(lo, hi)
}

// colourPath averages the colour photos of each period in OKLab: a and b
// separately, so opposite hues cancel out as they do for the eye.
func colourPath(points []ColourPoint, granularity string) []ColourSpaceStep {
	n := 7
	if granularity == "year" {
		n = 4
	}
	by := map[string]*ColourSpaceStep{}
	for _, p := range points {
		if len(p.Date) < 7 || p.Mono != "colour" {
			continue
		}
		period := p.Date[:n]
		st := by[period]
		if st == nil {
			st = &ColourSpaceStep{Period: period}
			by[period] = st
		}
		a, b := labAB(p.Colour)
		st.L += p.Colour.L
		st.A += a
		st.B += b
		st.Photos++
	}
	out := make([]ColourSpaceStep, 0, len(by))
	for _, st := range by {
		k := float64(st.Photos)
		out = append(out, ColourSpaceStep{Period: st.Period, Photos: st.Photos,
			L: round4(st.L / k), A: round4(st.A / k), B: round4(st.B / k)})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Period < out[j].Period })
	return out
}
