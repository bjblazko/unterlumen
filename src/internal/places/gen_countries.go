//go:build ignore

// gen_countries builds countries.json.gz from Natural Earth's Admin 0
// countries at 1:10m (public domain), as Germany sees disputed areas — the
// _deu point of view: the Crimea in Ukraine, Northern Cyprus in Cyprus. Each country's ISO 3166-1 alpha-2
// code, its names, and its borders simplified to about half a kilometre and
// kept to a ten-thousandth of a degree, each ring as differences from its
// previous point. Run it when Natural Earth has a new version:
//
//	curl -LO https://raw.githubusercontent.com/nvkelso/natural-earth-vector/master/geojson/ne_10m_admin_0_countries_deu.geojson
//	go run gen_countries.go ne_10m_admin_0_countries_deu.geojson
package main

import (
	"compress/gzip"
	"encoding/json"
	"fmt"
	"math"
	"os"
)

// tolerance is how far, in degrees, a simplified border may leave the
// drawn one.
const tolerance = 0.005

const scale = 1e4

// languages are the names kept, as the app speaks them.
var languages = []string{"en", "de"}

type feature struct {
	Properties map[string]any `json:"properties"`
	Geometry   struct {
		Type        string          `json:"type"`
		Coordinates json.RawMessage `json:"coordinates"`
	} `json:"geometry"`
}

type country struct {
	Code  string            `json:"c,omitempty"`
	Names map[string]string `json:"n"`
	// Polygons are rings of delta-coded [lon, lat] ints; the first ring of
	// each polygon is its outline, the others its holes.
	Polygons [][][]int32 `json:"p"`
}

func main() {
	data, err := os.ReadFile(os.Args[1])
	if err != nil {
		panic(err)
	}
	var fc struct {
		Features []feature `json:"features"`
	}
	if err := json.Unmarshal(data, &fc); err != nil {
		panic(err)
	}
	var out []country
	for _, f := range fc.Features {
		c := country{Names: map[string]string{}}
		if code, _ := f.Properties["ISO_A2_EH"].(string); len(code) == 2 {
			c.Code = code
		}
		for _, l := range languages {
			if n, _ := f.Properties["NAME_"+map[string]string{"en": "EN", "de": "DE"}[l]].(string); n != "" {
				c.Names[l] = n
			}
		}
		for _, poly := range polygons(f) {
			var rings [][]int32
			for _, ring := range poly {
				if r := encode(simplify(ring)); r != nil {
					rings = append(rings, r)
				} else if len(rings) == 0 {
					break
				}
			}
			if len(rings) > 0 {
				c.Polygons = append(c.Polygons, rings)
			}
		}
		if len(c.Polygons) > 0 {
			out = append(out, c)
		}
	}
	file, err := os.Create("countries.json.gz")
	if err != nil {
		panic(err)
	}
	defer file.Close()
	zw, _ := gzip.NewWriterLevel(file, gzip.BestCompression)
	if err := json.NewEncoder(zw).Encode(out); err != nil {
		panic(err)
	}
	if err := zw.Close(); err != nil {
		panic(err)
	}
	fmt.Println(len(out), "countries")
}

func polygons(f feature) [][][][2]float64 {
	switch f.Geometry.Type {
	case "Polygon":
		var p [][][2]float64
		json.Unmarshal(f.Geometry.Coordinates, &p) //nolint:errcheck
		return [][][][2]float64{p}
	case "MultiPolygon":
		var mp [][][][2]float64
		json.Unmarshal(f.Geometry.Coordinates, &mp) //nolint:errcheck
		return mp
	}
	return nil
}

// simplify is Douglas–Peucker on a closed ring.
func simplify(ring [][2]float64) [][2]float64 {
	if len(ring) < 4 {
		return ring
	}
	keep := make([]bool, len(ring))
	keep[0], keep[len(ring)-1] = true, true
	var walk func(a, b int)
	walk = func(a, b int) {
		far, at := 0.0, -1
		for i := a + 1; i < b; i++ {
			if d := segDist(ring[i], ring[a], ring[b]); d > far {
				far, at = d, i
			}
		}
		if at >= 0 && far > tolerance {
			keep[at] = true
			walk(a, at)
			walk(at, b)
		}
	}
	// A closed ring starts and ends at one point: split it at its farthest.
	mid, far := 0, 0.0
	for i := range ring {
		if d := math.Hypot(ring[i][0]-ring[0][0], ring[i][1]-ring[0][1]); d > far {
			mid, far = i, d
		}
	}
	keep[mid] = true
	walk(0, mid)
	walk(mid, len(ring)-1)
	var out [][2]float64
	for i, k := range keep {
		if k {
			out = append(out, ring[i])
		}
	}
	return out
}

func segDist(p, a, b [2]float64) float64 {
	dx, dy := b[0]-a[0], b[1]-a[1]
	if dx == 0 && dy == 0 {
		return math.Hypot(p[0]-a[0], p[1]-a[1])
	}
	t := math.Max(0, math.Min(1, ((p[0]-a[0])*dx+(p[1]-a[1])*dy)/(dx*dx+dy*dy)))
	return math.Hypot(p[0]-a[0]-t*dx, p[1]-a[1]-t*dy)
}

// encode keeps a ring of at least a triangle, delta-coded; nil for less.
func encode(ring [][2]float64) []int32 {
	if len(ring) < 4 {
		return nil
	}
	out := make([]int32, 0, len(ring)*2)
	var px, py int32
	for _, p := range ring {
		x, y := int32(math.Round(p[0]*scale)), int32(math.Round(p[1]*scale))
		out = append(out, x-px, y-py)
		px, py = x, y
	}
	return out
}
