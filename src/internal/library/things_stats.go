package library

import (
	"sort"

	"huepattl.de/unterlumen/internal/things"
)

// ThingsStats is the Things topic of the statistics (ADR-0054): which things
// the photos show, how that changed, and which come together. Things are put
// together across libraries by name.
type ThingsStats struct {
	Installed   bool               `json:"installed"`
	Granularity string             `json:"granularity"`
	Libraries   []string           `json:"libraries"` // the ids every Lib column indexes
	Looked      int                `json:"looked"`    // photos in scope looked at for things
	Things      []ThingsStatsThing `json:"things"`    // most photos first; every Thing column indexes it
	Appearances ThingAppearances   `json:"appearances"`
}

// ThingsStatsThing is a thing and its photos in scope.
type ThingsStatsThing struct {
	Name     string `json:"name"`
	Category string `json:"category"`
	Photos   int    `json:"photos"`
}

// ThingAppearances has one entry per thing in a photo, in columns.
type ThingAppearances struct {
	Thing []int    `json:"thing"`
	Lib   []int    `json:"lib"`
	Photo []string `json:"photo"`
	Date  []string `json:"date"`
}

// ThingsStats reads the Things topic across the requested libraries (all
// when ids is nil), through the shared filter.
func (m *Manager) ThingsStats(ids []string, pathPrefix, granularity string, f Filter) (*ThingsStats, error) {
	libs, err := m.filterLibraries(ids)
	if err != nil {
		return nil, err
	}
	pathGlob := ""
	if pathPrefix != "" {
		pathGlob = escapeLikePattern(pathPrefix) + "/%"
	}
	ts := &ThingsStats{Installed: m.ThingsInstalled(), Libraries: []string{}, Things: []ThingsStatsThing{},
		Appearances: ThingAppearances{Thing: []int{}, Lib: []int{}, Photo: []string{}, Date: []string{}}}
	var all [][]Appearance
	for _, l := range libs {
		apps, looked, err := m.libraryThings(l.ID, pathGlob, f)
		if err != nil {
			continue
		}
		ts.Libraries = append(ts.Libraries, l.ID)
		ts.Looked += looked
		all = append(all, apps)
	}
	index := thingsIndex(ts, all)
	for li, apps := range all {
		for _, a := range apps {
			ts.Appearances.Thing = append(ts.Appearances.Thing, index[a.Name])
			ts.Appearances.Lib = append(ts.Appearances.Lib, li)
			ts.Appearances.Photo = append(ts.Appearances.Photo, a.Photo)
			ts.Appearances.Date = append(ts.Appearances.Date, a.Date)
		}
	}
	ts.Granularity = granularity
	if granularity != "month" && granularity != "year" {
		ts.Granularity = datesGranularity(ts.Appearances.Date, func(d string) string { return d })
	}
	return ts, nil
}

func (m *Manager) libraryThings(id, pathGlob string, f Filter) ([]Appearance, int, error) {
	store, err := m.OpenFiltered(id, f)
	if err != nil {
		return nil, 0, err
	}
	defer store.Close()
	apps, err := store.ThingAppearances(pathGlob)
	if err != nil {
		return nil, 0, err
	}
	looked, err := store.ThingsLooked(pathGlob)
	return apps, looked, err
}

// thingsIndex lists the things with photos in scope, most photos first, and
// returns each one's place in the list.
func thingsIndex(ts *ThingsStats, all [][]Appearance) map[string]int {
	photos := map[string]int{}
	for _, apps := range all {
		for _, a := range apps {
			photos[a.Name]++
		}
	}
	for name, n := range photos {
		ts.Things = append(ts.Things, ThingsStatsThing{Name: name, Category: string(things.CategoryOf(name)), Photos: n})
	}
	sort.Slice(ts.Things, func(i, j int) bool {
		if ts.Things[i].Photos != ts.Things[j].Photos {
			return ts.Things[i].Photos > ts.Things[j].Photos
		}
		return ts.Things[i].Name < ts.Things[j].Name
	})
	index := make(map[string]int, len(ts.Things))
	for i, t := range ts.Things {
		index[t.Name] = i
	}
	return index
}
