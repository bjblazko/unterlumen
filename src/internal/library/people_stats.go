package library

import (
	"sort"
	"strings"

	"huepattl.de/unterlumen/internal/faces"
)

// PeopleStats is the People topic of the statistics (ADR-0051): who is in
// the photos and with whom, how many faces a photo holds, and every face
// placed by what it looks like. People are put together across libraries by
// name, as everywhere else; only named people count.
type PeopleStats struct {
	Granularity string              `json:"granularity"`
	Libraries   []string            `json:"libraries"` // the ids every Lib column indexes
	People      []PeopleStatsPerson `json:"people"`    // most photos first; every Person column indexes it
	Appearances PeopleAppearances   `json:"appearances"`
	Crowd       PeopleCrowd         `json:"crowd"`
	FaceSpace   PeopleFaceSpace     `json:"faceSpace"`
}

// PeopleStatsPerson is a named person and the face that shows them.
type PeopleStatsPerson struct {
	Name   string `json:"name"`
	Photos int    `json:"photos"`
	Lib    int    `json:"lib"`
	Cover  int64  `json:"cover"`
}

// PeopleAppearances has one entry per named person in a photo, in columns.
type PeopleAppearances struct {
	Person []int    `json:"person"`
	Lib    []int    `json:"lib"`
	Photo  []string `json:"photo"`
	Date   []string `json:"date"`
}

// CrowdBucket is a range of faces per photo; Max is -1 for no upper end.
type CrowdBucket struct {
	Label string `json:"label"`
	Min   int    `json:"min"`
	Max   int    `json:"max"`
}

var crowdBuckets = []CrowdBucket{
	{"No face", 0, 0}, {"1 face", 1, 1}, {"2 faces", 2, 2}, {"3 to 5 faces", 3, 5}, {"6 or more", 6, -1},
}

// PeopleCrowd counts the photos looked at by how many faces they hold, in
// all and per period (Counts[period][bucket]).
type PeopleCrowd struct {
	Buckets []CrowdBucket `json:"buckets"`
	Total   []int         `json:"total"`
	Periods []string      `json:"periods"`
	Counts  [][]int       `json:"counts"`
	Photos  int           `json:"photos"`
}

// PeopleFaceSpace has one entry per face shown, in columns; Person is -1
// for a face nobody named. Faces is how many there are in scope, which is
// more than are shown when there are too many to draw.
type PeopleFaceSpace struct {
	Face   []int64   `json:"face"`
	Lib    []int     `json:"lib"`
	Photo  []string  `json:"photo"`
	Date   []string  `json:"date"`
	Person []int     `json:"person"`
	X      []float64 `json:"x"`
	Y      []float64 `json:"y"`
	Z      []float64 `json:"z"`
	Faces  int       `json:"faces"`
	Axes   string    `json:"axes"` // "people" or "faces": what the axes were taken from
}

// maxFacesShown is how many faces the Face space draws; beyond it, every
// named face is kept and the unnamed ones are thinned out evenly.
const maxFacesShown = 40000

// LibraryPeople is what one library contributes.
type LibraryPeople struct {
	LibraryID   string
	People      []Person
	Appearances []Appearance
	FaceCounts  []FaceCount
	Prints      []FacePrint
}

// PeopleStats reads the People topic across the requested libraries (all
// when ids is nil). It is not cached: names change outside scans.
func (m *Manager) PeopleStats(ids []string, pathPrefix, granularity string, f Filter) (*PeopleStats, error) {
	libs, err := m.filterLibraries(ids)
	if err != nil {
		return nil, err
	}
	pathGlob := ""
	if pathPrefix != "" {
		pathGlob = escapeLikePattern(pathPrefix) + "/%"
	}
	var all []LibraryPeople
	for _, l := range libs {
		lp, err := m.libraryPeople(l.ID, pathGlob, f)
		if err != nil {
			continue
		}
		all = append(all, lp)
	}
	return BuildPeopleStats(all, granularity), nil
}

func (m *Manager) libraryPeople(id, pathGlob string, f Filter) (LibraryPeople, error) {
	lp := LibraryPeople{LibraryID: id}
	store, err := m.OpenFiltered(id, f)
	if err != nil {
		return lp, err
	}
	defer store.Close()
	if lp.People, err = store.People(); err != nil {
		return lp, err
	}
	if lp.Appearances, err = store.Appearances(pathGlob); err != nil {
		return lp, err
	}
	if lp.FaceCounts, err = store.FaceCounts(pathGlob); err != nil {
		return lp, err
	}
	lp.Prints, err = store.FacePrints(pathGlob)
	return lp, err
}

// BuildPeopleStats puts the libraries together. granularity is "month",
// "year", or "" to choose by the span of dates.
func BuildPeopleStats(libs []LibraryPeople, granularity string) *PeopleStats {
	ps := &PeopleStats{Libraries: []string{}, People: []PeopleStatsPerson{}}
	for _, l := range libs {
		ps.Libraries = append(ps.Libraries, l.LibraryID)
	}
	index := peopleIndex(ps, libs)
	ps.Appearances = appearancesOf(libs, index)
	if granularity != "month" && granularity != "year" {
		granularity = datesGranularity(ps.Appearances.Date, func(d string) string { return d })
	}
	ps.Granularity = granularity
	ps.Crowd = crowdOf(libs, granularity)
	ps.FaceSpace = faceSpaceOf(libs, index)
	return ps
}

// peopleIndex lists the named people with photos in scope, most photos
// first, and returns each lower-cased name's place in the list.
func peopleIndex(ps *PeopleStats, libs []LibraryPeople) map[string]int {
	photos := map[string]map[string]bool{}
	display := map[string]string{}
	cover := map[string]PeopleStatsPerson{}
	best := map[string]Person{}
	for li, l := range libs {
		for _, a := range l.Appearances {
			key := strings.ToLower(a.Name)
			if photos[key] == nil {
				photos[key] = map[string]bool{}
				display[key] = a.Name
			}
			photos[key][l.LibraryID+"/"+a.Photo] = true
		}
		for _, p := range l.People {
			key := strings.ToLower(p.Name)
			if b, ok := best[key]; !ok || showsBetter(p.CoverChosen, p.Faces, b.CoverChosen, b.Faces) {
				best[key] = p
				cover[key] = PeopleStatsPerson{Lib: li, Cover: p.Cover}
			}
		}
	}
	for key, set := range photos {
		c := cover[key]
		ps.People = append(ps.People, PeopleStatsPerson{Name: display[key], Photos: len(set), Lib: c.Lib, Cover: c.Cover})
	}
	sort.Slice(ps.People, func(i, j int) bool {
		if ps.People[i].Photos != ps.People[j].Photos {
			return ps.People[i].Photos > ps.People[j].Photos
		}
		return strings.ToLower(ps.People[i].Name) < strings.ToLower(ps.People[j].Name)
	})
	index := make(map[string]int, len(ps.People))
	for i, p := range ps.People {
		index[strings.ToLower(p.Name)] = i
	}
	return index
}

func appearancesOf(libs []LibraryPeople, index map[string]int) PeopleAppearances {
	out := PeopleAppearances{Person: []int{}, Lib: []int{}, Photo: []string{}, Date: []string{}}
	for li, l := range libs {
		for _, a := range l.Appearances {
			out.Person = append(out.Person, index[strings.ToLower(a.Name)])
			out.Lib = append(out.Lib, li)
			out.Photo = append(out.Photo, a.Photo)
			out.Date = append(out.Date, a.Date)
		}
	}
	return out
}

func crowdBucket(faces int) int {
	for i, b := range crowdBuckets {
		if faces >= b.Min && (b.Max < 0 || faces <= b.Max) {
			return i
		}
	}
	return len(crowdBuckets) - 1
}

func crowdOf(libs []LibraryPeople, granularity string) PeopleCrowd {
	c := PeopleCrowd{Buckets: crowdBuckets, Total: make([]int, len(crowdBuckets)), Periods: []string{}, Counts: [][]int{}}
	n := 7
	if granularity == "year" {
		n = 4
	}
	byPeriod := map[string][]int{}
	for _, l := range libs {
		for _, fc := range l.FaceCounts {
			b := crowdBucket(fc.Faces)
			c.Total[b]++
			c.Photos++
			if len(fc.Date) < n {
				continue
			}
			p := fc.Date[:n]
			if byPeriod[p] == nil {
				byPeriod[p] = make([]int, len(crowdBuckets))
			}
			byPeriod[p][b]++
		}
	}
	for p := range byPeriod {
		c.Periods = append(c.Periods, p)
	}
	sort.Strings(c.Periods)
	for _, p := range c.Periods {
		c.Counts = append(c.Counts, byPeriod[p])
	}
	return c
}

// faceSpaceOf places the faces, every named one and as many unnamed ones as
// fit within maxFacesShown.
func faceSpaceOf(libs []LibraryPeople, index map[string]int) PeopleFaceSpace {
	fs := PeopleFaceSpace{Face: []int64{}, Lib: []int{}, Photo: []string{}, Date: []string{}, Person: []int{},
		X: []float64{}, Y: []float64{}, Z: []float64{}}
	named, unnamed := 0, 0
	for _, l := range libs {
		for _, p := range l.Prints {
			if p.Name != "" {
				named++
			} else {
				unnamed++
			}
		}
	}
	fs.Faces = named + unnamed
	if fs.Faces == 0 {
		return fs
	}
	every := 1
	if fs.Faces > maxFacesShown && unnamed > 0 {
		room := max(1, maxFacesShown-named)
		every = (unnamed + room - 1) / room
	}
	var prints [][]float32
	seen := 0
	for li, l := range libs {
		for _, p := range l.Prints {
			person := -1
			if p.Name != "" {
				person = index[strings.ToLower(p.Name)]
			} else {
				seen++
				if seen%every != 0 {
					continue
				}
			}
			fs.Face = append(fs.Face, p.Face)
			fs.Lib = append(fs.Lib, li)
			fs.Photo = append(fs.Photo, p.Photo)
			fs.Date = append(fs.Date, p.Date)
			fs.Person = append(fs.Person, person)
			prints = append(prints, p.Embedding)
		}
	}
	basis, axesFrom := axesBasis(prints, fs.Person)
	fs.Axes = axesFrom
	mean, axes := faceAxes(basis)
	for _, pos := range projectFaces(prints, mean, axes) {
		fs.X = append(fs.X, round4(pos[0]))
		fs.Y = append(fs.Y, round4(pos[1]))
		fs.Z = append(fs.Z, round4(pos[2]))
	}
	return fs
}

// axesBasis is what the axes are taken from: the average face of each named
// person when there are enough of them, else the faces themselves.
func axesBasis(prints [][]float32, person []int) ([][]float32, string) {
	byPerson := map[int][][]float32{}
	for i, p := range person {
		if p >= 0 {
			byPerson[p] = append(byPerson[p], prints[i])
		}
	}
	if len(byPerson) < minPeopleForAxes {
		return prints, "faces"
	}
	keys := make([]int, 0, len(byPerson))
	for k := range byPerson {
		keys = append(keys, k)
	}
	sort.Ints(keys)
	centres := make([][]float32, len(keys))
	for i, k := range keys {
		centres[i] = faces.Mean(byPerson[k])
	}
	return centres, "people"
}
