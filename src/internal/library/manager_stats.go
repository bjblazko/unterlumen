package library

import (
	"fmt"
	"sort"
)

// Statistics returns aggregated statistics across the requested libraries (or all if ids is nil).
// pathPrefix, when non-empty, restricts each library's results to photos whose path starts with that prefix.
func (m *Manager) Statistics(ids []string, pathPrefix string, f Filter) (*LibraryStatistics, error) {
	libs, err := m.filterLibraries(ids)
	if err != nil {
		return nil, err
	}
	cacheKey := statsCacheKey(libraryIDs(libs), pathPrefix+f.Key())
	if v, ok := m.statsCache.Load(cacheKey); ok {
		return v.(*LibraryStatistics), nil
	}

	sm := newStatsMerger()
	for _, l := range libs {
		st, warning := m.libraryStatistics(l, pathPrefix, f)
		if warning != "" {
			sm.merged.Warnings = append(sm.merged.Warnings, warning)
			continue
		}
		sm.add(st)
	}
	merged := sm.result()
	m.statsCache.Store(cacheKey, merged)
	return merged, nil
}

func libraryIDs(libs []*Library) []string {
	ids := make([]string, len(libs))
	for i, l := range libs {
		ids[i] = l.ID
	}
	return ids
}

// libraryStatistics returns one library's statistics, or the warning to show
// when they cannot be read.
func (m *Manager) libraryStatistics(l *Library, pathPrefix string, f Filter) (*LibraryStatistics, string) {
	store, err := m.OpenFiltered(l.ID, f)
	if err != nil {
		return nil, fmt.Sprintf("library %q could not be read", l.Name)
	}
	st, err := store.Statistics(pathPrefix)
	store.Close()
	if err != nil {
		return nil, fmt.Sprintf("library %q statistics unavailable", l.Name)
	}
	if st.GonePhotos > 0 {
		st.GoneLibraries = []string{l.ID}
	}
	return st, ""
}

// statsMerger sums the statistics of several libraries.
type statsMerger struct {
	merged     *LibraryStatistics
	formats    map[string]int
	filmSims   map[string]int
	cameraLens map[[2]string]int
	focal      map[float64]int
	focal35    map[float64]int
	apertures  map[float64]int
	isos       map[float64]int
}

func newStatsMerger() *statsMerger {
	return &statsMerger{
		merged:     &LibraryStatistics{ShootingDays: make(map[string]int)},
		formats:    make(map[string]int),
		filmSims:   make(map[string]int),
		cameraLens: make(map[[2]string]int),
		focal:      make(map[float64]int),
		focal35:    make(map[float64]int),
		apertures:  make(map[float64]int),
		isos:       make(map[float64]int),
	}
}

func (sm *statsMerger) add(st *LibraryStatistics) {
	sm.merged.TotalPhotos += st.TotalPhotos
	sm.merged.GonePhotos += st.GonePhotos
	sm.merged.GoneLibraries = append(sm.merged.GoneLibraries, st.GoneLibraries...)
	addNameCounts(sm.formats, st.Formats)
	addNameCounts(sm.filmSims, st.FilmSims)
	addValueCounts(sm.focal, st.FocalLengths)
	addValueCounts(sm.focal35, st.FocalLengths35)
	addValueCounts(sm.apertures, st.Apertures)
	addValueCounts(sm.isos, st.ISOs)
	for _, clc := range st.CameraLens {
		sm.cameraLens[[2]string{clc.Camera, clc.Lens}] += clc.Count
	}
	for h, n := range st.ShootingHours {
		sm.merged.ShootingHours[h] += n
	}
	for day, n := range st.ShootingDays {
		sm.merged.ShootingDays[day] += n
	}
}

// result sorts the sums: names by count, values by value, and camera × lens
// by count, capped at 100.
func (sm *statsMerger) result() *LibraryStatistics {
	merged := sm.merged
	merged.Formats = mapToNameCounts(sm.formats)
	merged.FilmSims = mapToNameCounts(sm.filmSims)
	merged.FocalLengths = mapToValueCounts(sm.focal)
	merged.FocalLengths35 = mapToValueCounts(sm.focal35)
	merged.Apertures = mapToValueCounts(sm.apertures)
	merged.ISOs = mapToValueCounts(sm.isos)
	merged.CameraLens = make([]CameraLensCount, 0, len(sm.cameraLens))
	for key, count := range sm.cameraLens {
		merged.CameraLens = append(merged.CameraLens, CameraLensCount{Camera: key[0], Lens: key[1], Count: count})
	}
	sort.SliceStable(merged.CameraLens, func(i, j int) bool { return merged.CameraLens[i].Count > merged.CameraLens[j].Count })
	if len(merged.CameraLens) > 100 {
		merged.CameraLens = merged.CameraLens[:100]
	}
	return merged
}

func addNameCounts(sums map[string]int, ncs []NameCount) {
	for _, nc := range ncs {
		sums[nc.Name] += nc.Count
	}
}

func addValueCounts(sums map[float64]int, vcs []ValueCount) {
	for _, vc := range vcs {
		sums[vc.Value] += vc.Count
	}
}

// mapToNameCounts converts a name→count map to a []NameCount sorted by count descending.
func mapToNameCounts(m map[string]int) []NameCount {
	out := make([]NameCount, 0, len(m))
	for name, count := range m {
		out = append(out, NameCount{Name: name, Count: count})
	}
	sortNameCounts(out)
	return out
}
