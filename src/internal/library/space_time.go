package library

// SpaceTime is the Space and time topic of the statistics: every located
// photo with its place, date and main colour. The view projects the places
// and groups the days itself.
type SpaceTime struct {
	Libraries []string        `json:"libraries"` // the ids Points.Lib indexes
	Points    SpaceTimePoints `json:"points"`
	Photos    int             `json:"photos"`
}

// SpaceTimePoints holds one entry per photo in every column. L, A and B
// are the photo's main colour in OKLab, or L -1 when it is not analysed
// yet.
type SpaceTimePoints struct {
	ID   []string  `json:"id"`
	Lib  []int     `json:"lib"`
	Date []string  `json:"date"`
	Lat  []float64 `json:"lat"`
	Lon  []float64 `json:"lon"`
	L    []float64 `json:"l"`
	A    []float64 `json:"a"`
	B    []float64 `json:"b"`
}

// LibrarySpaceTime is the located photos of one library and the colours
// of its analysed photos, by id.
type LibrarySpaceTime struct {
	LibraryID string
	Places    []GeoPoint
	Colours   map[string]LCh
}

// BuildSpaceTime keeps the located photos that have a date.
func BuildSpaceTime(libs []LibrarySpaceTime) *SpaceTime {
	st := &SpaceTime{Libraries: []string{}, Points: SpaceTimePoints{
		ID: []string{}, Lib: []int{}, Date: []string{}, Lat: []float64{}, Lon: []float64{},
		L: []float64{}, A: []float64{}, B: []float64{},
	}}
	for li, lib := range libs {
		st.Libraries = append(st.Libraries, lib.LibraryID)
		for _, p := range lib.Places {
			if len(p.Taken) < 10 {
				continue
			}
			ps := &st.Points
			ps.ID = append(ps.ID, p.ID)
			ps.Lib = append(ps.Lib, li)
			ps.Date = append(ps.Date, p.Taken)
			ps.Lat = append(ps.Lat, round4(p.Lat))
			ps.Lon = append(ps.Lon, round4(p.Lon))
			l, a, b := -1.0, 0.0, 0.0
			if c, ok := lib.Colours[p.ID]; ok {
				l = round4(c.L)
				a, b = labAB(c)
				a, b = round4(a), round4(b)
			}
			ps.L = append(ps.L, l)
			ps.A = append(ps.A, a)
			ps.B = append(ps.B, b)
		}
	}
	st.Photos = len(st.Points.ID)
	return st
}
