package library

import (
	"slices"
	"sort"
)

// Timeline returns time-series statistics across the requested libraries (or all if ids is nil).
func (m *Manager) Timeline(ids []string, pathPrefix, granularity string) (*LibraryTimeline, error) {
	libs, err := m.filterLibraries(ids)
	if err != nil {
		return nil, err
	}

	tlCacheKey := timelineCacheKey(libraryIDs(libs), pathPrefix, granularity)
	if v, ok := m.timelineCache.Load(tlCacheKey); ok {
		return v.(*LibraryTimeline), nil
	}

	var results []*LibraryTimeline
	for _, l := range libs {
		store, err := m.OpenStore(l.ID)
		if err != nil {
			continue
		}
		tl, err := store.Timeline(pathPrefix, granularity)
		store.Close()
		if err != nil {
			continue
		}
		results = append(results, tl)
	}
	if len(results) == 0 {
		return &LibraryTimeline{
			Granularity:    coalesceGranularity(granularity),
			Periods:        []string{},
			CameraUsage:    []CameraTimeSlice{},
			FocalStats:     []PeriodStats{},
			ISOStats:       []PeriodStats{},
			ApertureHeat:   []ApertureRow{},
			AspectRatios:   []AspectSlice{},
			MegapixelStats: []MegapixelStat{},
		}, nil
	}
	var tl *LibraryTimeline
	if len(results) == 1 {
		tl = results[0]
	} else {
		tl = mergeTLs(results)
	}
	tl.CameraUsage = topCameras(tl.CameraUsage, len(tl.Periods))
	m.timelineCache.Store(tlCacheKey, tl)
	return tl, nil
}

func coalesceGranularity(g string) string {
	if g == "year" {
		return "year"
	}
	return "month"
}

func mergeTLs(results []*LibraryTimeline) *LibraryTimeline {
	periods := unionPeriods(results)
	return &LibraryTimeline{
		Granularity:    mergedGranularity(results),
		Periods:        periods,
		CameraUsage:    mergeCameraUsage(results, periods),
		FocalStats:     mergePercentileStats(results, periods, func(r *LibraryTimeline) []PeriodStats { return r.FocalStats }),
		ISOStats:       mergePercentileStats(results, periods, func(r *LibraryTimeline) []PeriodStats { return r.ISOStats }),
		ApertureHeat:   mergeApertureHeat(results, periods),
		AspectRatios:   mergeAspectRatios(results, periods),
		MegapixelStats: mergeMegapixels(results, periods),
	}
}

// mergedGranularity prefers "year" if any library returned it.
func mergedGranularity(results []*LibraryTimeline) string {
	for _, r := range results {
		if r.Granularity == "year" {
			return "year"
		}
	}
	return "month"
}

// unionPeriods returns every period of any result, sorted.
func unionPeriods(results []*LibraryTimeline) []string {
	periodSet := make(map[string]bool)
	for _, r := range results {
		for _, p := range r.Periods {
			periodSet[p] = true
		}
	}
	periods := make([]string, 0, len(periodSet))
	for p := range periodSet {
		periods = append(periods, p)
	}
	sortStrings(periods)
	return periods
}

// periodGrid sums per-key count rows, each aligned to its own library's
// periods, onto the merged periods.
type periodGrid struct {
	idx  map[string]int
	rows map[string][]int
}

func newPeriodGrid(periods []string) *periodGrid {
	idx := make(map[string]int, len(periods))
	for i, p := range periods {
		idx[p] = i
	}
	return &periodGrid{idx: idx, rows: make(map[string][]int)}
}

// add sums counts, aligned to srcPeriods, into key's row. It returns their
// total and whether any count lined up with a period.
func (g *periodGrid) add(key string, srcPeriods []string, counts []int) (total int, overlapped bool) {
	if g.rows[key] == nil {
		g.rows[key] = make([]int, len(g.idx))
	}
	srcIdx := make(map[string]int, len(srcPeriods))
	for i, p := range srcPeriods {
		srcIdx[p] = i
	}
	for p, si := range srcIdx {
		if si < len(counts) {
			g.rows[key][g.idx[p]] += counts[si]
			total += counts[si]
			overlapped = true
		}
	}
	return total, overlapped
}

// mergeCameraUsage sums usage per (camera, period), every camera.
func mergeCameraUsage(results []*LibraryTimeline, periods []string) []CameraTimeSlice {
	grid := newPeriodGrid(periods)
	var ranked []string
	for _, r := range results {
		for _, cs := range r.CameraUsage {
			// A camera is listed only once one of its counts lines up with a period.
			if _, overlapped := grid.add(cs.Camera, r.Periods, cs.Counts); overlapped && !slices.Contains(ranked, cs.Camera) {
				ranked = append(ranked, cs.Camera)
			}
		}
	}
	cameras := make([]CameraTimeSlice, 0, len(ranked))
	for _, camera := range ranked {
		cameras = append(cameras, CameraTimeSlice{Camera: camera, Counts: grid.rows[camera]})
	}
	return cameras
}

// topCameraCount is how many cameras a timeline names; the rest are "Other".
const topCameraCount = 5

// topCameras keeps the most used cameras, most photos first, and adds up the
// rest as "Other".
func topCameras(cameras []CameraTimeSlice, periods int) []CameraTimeSlice {
	total := func(cs CameraTimeSlice) (n int) {
		for _, c := range cs.Counts {
			n += c
		}
		return n
	}
	ranked := slices.Clone(cameras)
	sort.SliceStable(ranked, func(i, j int) bool {
		ti, tj := total(ranked[i]), total(ranked[j])
		if ti != tj {
			return ti > tj
		}
		return ranked[i].Camera < ranked[j].Camera
	})
	if len(ranked) <= topCameraCount {
		return ranked
	}
	other := make([]int, periods)
	for _, cs := range ranked[topCameraCount:] {
		for i, c := range cs.Counts {
			other[i] += c
		}
	}
	return append(ranked[:topCameraCount:topCameraCount], CameraTimeSlice{Camera: "Other", Counts: other})
}

// mergeApertureHeat sums the aperture bucket counts per period.
func mergeApertureHeat(results []*LibraryTimeline, periods []string) []ApertureRow {
	aperMap := make(map[string]map[string]int)
	for _, r := range results {
		for _, row := range r.ApertureHeat {
			if aperMap[row.Period] == nil {
				aperMap[row.Period] = make(map[string]int)
			}
			for k, v := range row.Buckets {
				aperMap[row.Period][k] += v
			}
		}
	}
	rows := make([]ApertureRow, 0, len(periods))
	for _, p := range periods {
		if buckets := aperMap[p]; len(buckets) > 0 {
			rows = append(rows, ApertureRow{Period: p, Buckets: buckets})
		}
	}
	return rows
}

// mergeAspectRatios sums counts per (ratio, period), in the fixed ratio order,
// leaving out ratios no photo has.
func mergeAspectRatios(results []*LibraryTimeline, periods []string) []AspectSlice {
	grid := newPeriodGrid(periods)
	for _, r := range results {
		for _, as := range r.AspectRatios {
			grid.add(as.Ratio, r.Periods, as.Counts)
		}
	}
	slices := make([]AspectSlice, 0, len(tlAspectOrder))
	for _, ratio := range tlAspectOrder {
		counts := grid.rows[ratio]
		if anyPositive(counts) {
			slices = append(slices, AspectSlice{Ratio: ratio, Counts: counts})
		}
	}
	return slices
}

func anyPositive(counts []int) bool {
	for _, c := range counts {
		if c > 0 {
			return true
		}
	}
	return false
}

// mergeMegapixels takes the max of the maxes and the count-weighted average
// per period.
func mergeMegapixels(results []*LibraryTimeline, periods []string) []MegapixelStat {
	mpMap := make(map[string]MegapixelStat)
	for _, r := range results {
		for _, ms := range r.MegapixelStats {
			cur := mpMap[ms.Period]
			cur.Max = max(cur.Max, ms.Max)
			// Weighted average: (cur.Avg*cur.Count + ms.Avg*ms.Count) / (cur.Count + ms.Count)
			total := cur.Count + ms.Count
			if total > 0 {
				cur.Avg = (cur.Avg*float64(cur.Count) + ms.Avg*float64(ms.Count)) / float64(total)
			}
			cur.Count = total
			cur.Period = ms.Period
			mpMap[ms.Period] = cur
		}
	}
	stats := make([]MegapixelStat, 0, len(periods))
	for _, p := range periods {
		if ms, ok := mpMap[p]; ok {
			stats = append(stats, ms)
		}
	}
	return stats
}

func mergePercentileStats(results []*LibraryTimeline, periods []string, getter func(*LibraryTimeline) []PeriodStats) []PeriodStats {
	type acc struct {
		sumMedian, sumP25, sumP75 float64
		count                     int
	}
	byPeriod := make(map[string]acc)
	for _, r := range results {
		for _, ps := range getter(r) {
			a := byPeriod[ps.Period]
			a.sumMedian += ps.Median * float64(ps.Count)
			a.sumP25 += ps.P25 * float64(ps.Count)
			a.sumP75 += ps.P75 * float64(ps.Count)
			a.count += ps.Count
			byPeriod[ps.Period] = a
		}
	}
	out := make([]PeriodStats, 0, len(periods))
	for _, p := range periods {
		a, ok := byPeriod[p]
		if !ok || a.count == 0 {
			continue
		}
		out = append(out, PeriodStats{
			Period: p,
			Median: a.sumMedian / float64(a.count),
			P25:    a.sumP25 / float64(a.count),
			P75:    a.sumP75 / float64(a.count),
			Count:  a.count,
		})
	}
	return out
}

// mapToValueCounts converts a value→count map to a []ValueCount sorted by value ascending.
func mapToValueCounts(m map[float64]int) []ValueCount {
	out := make([]ValueCount, 0, len(m))
	for v, n := range m {
		out = append(out, ValueCount{Value: v, Count: n})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Value < out[j].Value })
	return out
}
