package memories

import (
	"hash/fnv"
	"sort"
	"time"

	"huepattl.de/unterlumen/internal/library"
)

// joinDays is how many calendar days may lie between two away days of one
// trip: up to two days at home between them, as JOURNEY_GAP in the
// Statistics.
const joinDays = 3

// anniversaryDays is how near today a trip of an earlier year must have
// begun to come back on its anniversary.
const anniversaryDays = 7

// Trip is days away from home, at most two days apart.
type Trip struct {
	Start, End, Country string
	Photos              int // on the days away
}

type awayDay struct {
	day string
	n   int
}

// FindTrips finds the trips as the Statistics' journeys do
// (stats-places-charts.js, findJourneys): each day's centre, weighted by its
// photos' tenth-of-a-degree cells; away when more than 100 km from home.
func FindTrips(all []library.MemoryPhoto, home Home) []Trip {
	if !home.Known {
		return nil
	}
	var trips []Trip
	for _, d := range awayDays(all, home) {
		if n := len(trips); n > 0 && daysBetween(trips[n-1].End, d.day) <= joinDays {
			trips[n-1].End = d.day
			trips[n-1].Photos += d.n
			continue
		}
		trips = append(trips, Trip{Start: d.day, End: d.day, Photos: d.n})
	}
	for i := range trips {
		trips[i].Country = mainCountry(all, trips[i].Start, trips[i].End)
	}
	return trips
}

func awayDays(all []library.MemoryPhoto, home Home) []awayDay {
	type centre struct{ n, lat, lon float64 }
	days := map[string]*centre{}
	for _, p := range all {
		if !p.Located || len(p.Taken) < 10 {
			continue
		}
		c := cellOf(p.Lat, p.Lon)
		d := days[p.Taken[:10]]
		if d == nil {
			d = &centre{}
			days[p.Taken[:10]] = d
		}
		d.n++
		d.lat += float64(c[0]) / 10
		d.lon += float64(c[1]) / 10
	}
	var out []awayDay
	for day, d := range days {
		if KmBetween(home.Lat, home.Lon, d.lat/d.n, d.lon/d.n) > awayKm {
			out = append(out, awayDay{day, int(d.n)})
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].day < out[j].day })
	return out
}

// mainCountry is the country most photos of those days name.
func mainCountry(all []library.MemoryPhoto, start, end string) string {
	counts := map[string]int{}
	best := ""
	for _, p := range all {
		if p.Country == "" || len(p.Taken) < 10 || p.Taken[:10] < start || p.Taken[:10] > end {
			continue
		}
		counts[p.Country]++
		if c := counts[p.Country]; c > counts[best] || (c == counts[best] && p.Country < best) {
			best = p.Country
		}
	}
	return best
}

func daysBetween(a, b string) int {
	ta, _ := time.Parse(dayLayout, a)
	tb, _ := time.Parse(dayLayout, b)
	return int(tb.Sub(ta).Hours() / 24)
}

// tripCard is a trip that began near today in an earlier year, the most
// recent of them; otherwise one of the day's choosing.
func (w *world) tripCard() (memory, bool) {
	var past []Trip
	for _, t := range FindTrips(w.all, w.home) {
		if t.End < w.today() {
			past = append(past, t)
		}
	}
	if len(past) == 0 {
		return memory{}, false
	}
	for i := len(past) - 1; i >= 0; i-- {
		if w.nearAnniversary(past[i].Start) {
			return w.tripMemory(past[i].Start)
		}
	}
	return w.tripMemory(past[choose(w.today()+"|trip", len(past))].Start)
}

// nearAnniversary says start's month and day lie within anniversaryDays of
// today's, in an earlier year.
func (w *world) nearAnniversary(start string) bool {
	s, err := time.Parse(dayLayout, start)
	if err != nil || s.Year() >= w.day.Year() {
		return false
	}
	for _, y := range []int{w.day.Year() - 1, w.day.Year(), w.day.Year() + 1} {
		again := time.Date(y, s.Month(), s.Day(), 0, 0, 0, 0, time.UTC)
		if d := again.Sub(w.day).Hours() / 24; d >= -anniversaryDays && d <= anniversaryDays {
			return true
		}
	}
	return false
}

// tripMemory is the trip that began on key: every photo of its days.
func (w *world) tripMemory(key string) (memory, bool) {
	for _, t := range FindTrips(w.all, w.home) {
		if t.Start != key {
			continue
		}
		photos := w.where(func(p library.MemoryPhoto) bool {
			return len(p.Taken) >= 10 && p.Taken[:10] >= t.Start && p.Taken[:10] <= t.End
		})
		card := Card{Kind: "trip", Key: key, Start: t.Start, End: t.End, Country: t.Country, Days: daysBetween(t.Start, t.End) + 1}
		return memory{card: card, photos: photos, picks: w.pick("trip", key, photos, picksPer)}, true
	}
	return memory{}, false
}

// choose is the seed's choice among n, the same every time.
func choose(seed string, n int) int {
	h := fnv.New64a()
	h.Write([]byte(seed))
	return int(h.Sum64() % uint64(n))
}
