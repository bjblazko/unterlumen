// Package memories chooses the photos the Memories place shows (ADR-0064):
// what happened on this day in earlier years, a trip, a person, this month
// back then and photos rediscovered — by what the photos show, never by
// stars. It is pure: the photos come from library.Manager.MemoryPhotos.
package memories

import (
	"math"

	"huepattl.de/unterlumen/internal/library"
)

// awayKm is how far from home a place must be to be away, as in the
// Statistics' journeys (stats-places-charts.js, JOURNEY_KM).
const awayKm = 100

// Home is the centre of the tenth-of-a-degree cell with the most photos.
type Home struct {
	Lat, Lon float64
	Known    bool
}

// cellOf is a position's tenth-of-a-degree cell, as the places tallies
// round it.
func cellOf(lat, lon float64) [2]int {
	return [2]int{int(math.Round(lat * 10)), int(math.Round(lon * 10))}
}

// FindHome finds home among the located photos; unknown without any.
func FindHome(all []library.MemoryPhoto) Home {
	counts := map[[2]int]int{}
	var best [2]int
	for _, p := range all {
		if !p.Located {
			continue
		}
		c := cellOf(p.Lat, p.Lon)
		counts[c]++
		if counts[c] > counts[best] || (counts[c] == counts[best] && less(c, best)) {
			best = c
		}
	}
	if len(counts) == 0 {
		return Home{}
	}
	return Home{Lat: float64(best[0]) / 10, Lon: float64(best[1]) / 10, Known: true}
}

func less(a, b [2]int) bool { return a[0] < b[0] || (a[0] == b[0] && a[1] < b[1]) }

// away says the photo was taken more than awayKm from home.
func (h Home) away(p library.MemoryPhoto) bool {
	return h.Known && p.Located && KmBetween(h.Lat, h.Lon, p.Lat, p.Lon) > awayKm
}

// KmBetween is the great-circle distance, as the Statistics measure it.
func KmBetween(aLat, aLon, bLat, bLon float64) float64 {
	r := math.Pi / 180
	a := math.Pow(math.Sin((bLat-aLat)*r/2), 2) + math.Cos(aLat*r)*math.Cos(bLat*r)*math.Pow(math.Sin((bLon-aLon)*r/2), 2)
	return 12742 * math.Asin(math.Sqrt(a))
}
