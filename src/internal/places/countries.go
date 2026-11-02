// Package places says which country a GPS position is in, offline, from
// Natural Earth's country borders built into the app (public domain;
// ADR-0061), disputed areas as Germany sees them. Nothing is asked of any
// server.
package places

import (
	"bytes"
	"compress/gzip"
	_ "embed"
	"encoding/json"
	"math"
	"sync"
)

//go:embed countries.json.gz
var countriesData []byte

// Country is a country by its ISO 3166-1 alpha-2 code ("" for an area
// without one, such as Somaliland) and its names by language.
type Country struct {
	Code  string            `json:"c,omitempty"`
	Names map[string]string `json:"n"`
	// Polygons as gen_countries.go writes them: delta-coded rings.
	Polygons [][][]int32 `json:"p"`

	rings []polygon
}

// Name is the country's name in lang, else in English.
func (c *Country) Name(lang string) string {
	if n := c.Names[lang]; n != "" {
		return n
	}
	return c.Names["en"]
}

type point struct{ x, y float64 }

type polygon struct {
	rings      [][]point // the outline, then its holes
	minX, minY float64
	maxX, maxY float64
}

const scale = 1e4

// coastReach is how far off a country's border a position still counts as
// in it, in degrees (about 20 km): photos taken on a beach, a pier or a
// boat near the shore, which the simplified borders leave in the sea.
const coastReach = 0.2

var (
	loadOnce  sync.Once
	countries []*Country
)

func load() []*Country {
	loadOnce.Do(func() {
		zr, err := gzip.NewReader(bytes.NewReader(countriesData))
		if err != nil {
			return
		}
		var list []*Country
		if json.NewDecoder(zr).Decode(&list) != nil {
			return
		}
		for _, c := range list {
			c.rings = decodePolygons(c.Polygons)
			c.Polygons = nil
		}
		countries = list
	})
	return countries
}

func decodePolygons(polys [][][]int32) []polygon {
	out := make([]polygon, 0, len(polys))
	for _, poly := range polys {
		p := polygon{minX: math.Inf(1), minY: math.Inf(1), maxX: math.Inf(-1), maxY: math.Inf(-1)}
		for _, ring := range poly {
			var pts []point
			var x, y int32
			for i := 0; i+1 < len(ring); i += 2 {
				x, y = x+ring[i], y+ring[i+1]
				pt := point{float64(x) / scale, float64(y) / scale}
				pts = append(pts, pt)
				p.minX, p.maxX = math.Min(p.minX, pt.x), math.Max(p.maxX, pt.x)
				p.minY, p.maxY = math.Min(p.minY, pt.y), math.Max(p.maxY, pt.y)
			}
			p.rings = append(p.rings, pts)
		}
		out = append(out, p)
	}
	return out
}

// CountryAt is the country at lat, lon: the one whose borders hold it, or
// the nearest within coastReach. false when there is none — the open sea.
func CountryAt(lat, lon float64) (*Country, bool) {
	pt := point{lon, lat}
	var near *Country
	nearest := coastReach
	for _, c := range load() {
		for i := range c.rings {
			p := &c.rings[i]
			if pt.x < p.minX-nearest || pt.x > p.maxX+nearest || pt.y < p.minY-nearest || pt.y > p.maxY+nearest {
				continue
			}
			if p.contains(pt) {
				return c, true
			}
			if d := p.distance(pt); d < nearest {
				near, nearest = c, d
			}
		}
	}
	return near, near != nil
}

func (p *polygon) contains(pt point) bool {
	if pt.x < p.minX || pt.x > p.maxX || pt.y < p.minY || pt.y > p.maxY {
		return false
	}
	in := false
	for _, ring := range p.rings {
		if inRing(ring, pt) {
			in = !in
		}
	}
	return in
}

// inRing is the even–odd rule.
func inRing(ring []point, pt point) bool {
	in := false
	for i, j := 0, len(ring)-1; i < len(ring); j, i = i, i+1 {
		a, b := ring[i], ring[j]
		if (a.y > pt.y) != (b.y > pt.y) && pt.x < (b.x-a.x)*(pt.y-a.y)/(b.y-a.y)+a.x {
			in = !in
		}
	}
	return in
}

// distance is how far pt is from the outline, in degrees, with longitude
// shrunk by the latitude so a degree east is as far as it is.
func (p *polygon) distance(pt point) float64 {
	k := math.Cos(pt.y * math.Pi / 180)
	best := math.Inf(1)
	ring := p.rings[0]
	for i, j := 0, len(ring)-1; i < len(ring); j, i = i, i+1 {
		best = math.Min(best, segDist(pt, ring[j], ring[i], k))
	}
	return best
}

func segDist(p, a, b point, k float64) float64 {
	ax, bx, px := a.x*k, b.x*k, p.x*k
	dx, dy := bx-ax, b.y-a.y
	t := 0.0
	if dx != 0 || dy != 0 {
		t = math.Max(0, math.Min(1, ((px-ax)*dx+(p.y-a.y)*dy)/(dx*dx+dy*dy)))
	}
	return math.Hypot(px-ax-t*dx, p.y-a.y-t*dy)
}
