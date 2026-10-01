package library

import (
	"math"
	"sort"
	"strings"
)

// ExposureSpace is the Exposure space topic of the statistics: every photo
// with focal length, aperture and ISO as a point, coloured by its camera,
// and the path of each period's median settings.
type ExposureSpace struct {
	Granularity string              `json:"granularity"`
	Libraries   []string            `json:"libraries"` // the ids Points.Lib indexes
	Cameras     []string            `json:"cameras"`   // most used first; "Other" last when there are more
	Points      ExposureSpacePoints `json:"points"`
	Path        []ExposureSpaceStep `json:"path"`
	Photos      int                 `json:"photos"` // photos with all three settings
}

// ExposureSpacePoints holds one entry per photo in every column. Camera
// indexes Cameras.
type ExposureSpacePoints struct {
	ID     []string  `json:"id"`
	Lib    []int     `json:"lib"`
	Date   []string  `json:"date"`
	Camera []int     `json:"camera"`
	Focal  []float64 `json:"focal"`
	FNum   []float64 `json:"fnum"`
	ISO    []float64 `json:"iso"`
}

// ExposureSpaceStep is one period on the path: the median of each setting.
type ExposureSpaceStep struct {
	Period string  `json:"period"`
	Focal  float64 `json:"focal"`
	FNum   float64 `json:"fnum"`
	ISO    float64 `json:"iso"`
	Photos int     `json:"photos"`
}

// LibraryExposure is the exposure points of one library.
type LibraryExposure struct {
	LibraryID string
	Points    []ExposurePoint
}

// BuildExposureSpace lays out the points of one or more libraries.
// granularity is "month", "year", or "" to choose by the span of dates.
func BuildExposureSpace(libs []LibraryExposure, granularity string) *ExposureSpace {
	es := &ExposureSpace{Libraries: []string{}, Path: []ExposureSpaceStep{}, Points: ExposureSpacePoints{
		ID: []string{}, Lib: []int{}, Date: []string{}, Camera: []int{},
		Focal: []float64{}, FNum: []float64{}, ISO: []float64{},
	}}
	var all []ExposurePoint
	for _, l := range libs {
		es.Libraries = append(es.Libraries, l.LibraryID)
		all = append(all, l.Points...)
	}
	es.Cameras = rankCameras(all)
	index := make(map[string]int, len(es.Cameras))
	for i, c := range es.Cameras {
		index[c] = i
	}
	for li, l := range libs {
		for _, p := range l.Points {
			ci, ok := index[cameraLabel(p.Camera)]
			if !ok {
				ci = len(es.Cameras) - 1 // "Other"
			}
			es.Points.add(p, li, ci)
		}
	}
	es.Photos = len(all)
	if granularity != "month" && granularity != "year" {
		granularity = datesGranularity(all, func(p ExposurePoint) string { return p.Date })
	}
	es.Granularity = granularity
	es.Path = exposurePath(all, granularity)
	return es
}

func (ps *ExposureSpacePoints) add(p ExposurePoint, lib, camera int) {
	ps.ID = append(ps.ID, p.ID)
	ps.Lib = append(ps.Lib, lib)
	ps.Date = append(ps.Date, p.Date)
	ps.Camera = append(ps.Camera, camera)
	ps.Focal = append(ps.Focal, round4(p.Focal))
	ps.FNum = append(ps.FNum, round4(p.FNum))
	ps.ISO = append(ps.ISO, round4(p.ISO))
}

// cameraLabel is the Model tag as people read it: the index keeps it as
// written, quotes and padding included.
func cameraLabel(model string) string {
	if s := strings.TrimSpace(strings.Trim(model, `"`)); s != "" {
		return strings.TrimSpace(s)
	}
	return "Unknown camera"
}

// rankCameras names the most used cameras, as many as the Camera usage
// chart does, and "Other" for the rest.
func rankCameras(points []ExposurePoint) []string {
	counts := map[string]int{}
	for _, p := range points {
		counts[cameraLabel(p.Camera)]++
	}
	names := make([]string, 0, len(counts))
	for n := range counts {
		names = append(names, n)
	}
	sort.Slice(names, func(i, j int) bool {
		if counts[names[i]] != counts[names[j]] {
			return counts[names[i]] > counts[names[j]]
		}
		return names[i] < names[j]
	})
	if len(names) <= topCameraCount {
		return names
	}
	return append(names[:topCameraCount:topCameraCount], "Other")
}

// datesGranularity chooses months or years by the span of the dates that
// read as at least a month.
func datesGranularity[T any](items []T, date func(T) string) string {
	lo, hi := "", ""
	for _, it := range items {
		d := date(it)
		if len(d) < 7 {
			continue
		}
		if lo == "" || d[:7] < lo {
			lo = d[:7]
		}
		hi = max(hi, d[:7])
	}
	if lo == "" {
		return "month"
	}
	return granularityForSpan(lo, hi)
}

// exposurePath takes the median of each setting per period. The settings
// are steps on a log scale, so a median, not a mean, is a setting people use.
func exposurePath(points []ExposurePoint, granularity string) []ExposureSpaceStep {
	n := 7
	if granularity == "year" {
		n = 4
	}
	by := map[string][]ExposurePoint{}
	for _, p := range points {
		if len(p.Date) >= 7 {
			by[p.Date[:n]] = append(by[p.Date[:n]], p)
		}
	}
	out := make([]ExposureSpaceStep, 0, len(by))
	for period, ps := range by {
		out = append(out, ExposureSpaceStep{
			Period: period, Photos: len(ps),
			Focal: median(ps, func(p ExposurePoint) float64 { return p.Focal }),
			FNum:  median(ps, func(p ExposurePoint) float64 { return p.FNum }),
			ISO:   median(ps, func(p ExposurePoint) float64 { return p.ISO }),
		})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Period < out[j].Period })
	return out
}

func median[T any](items []T, v func(T) float64) float64 {
	vals := make([]float64, len(items))
	for i, it := range items {
		vals[i] = v(it)
	}
	sort.Float64s(vals)
	m := len(vals) / 2
	if len(vals)%2 == 1 {
		return round4(vals[m])
	}
	return round4(math.Sqrt(vals[m-1] * vals[m])) // between two stops, on their scale
}
