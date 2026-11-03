package memories

import (
	"fmt"
	"math"
	"testing"

	"huepattl.de/unterlumen/internal/library"
)

func located(id string, lat, lon float64) library.MemoryPhoto {
	p := photoAt(id, "2020-01-01T10:00:00")
	p.Lat, p.Lon, p.Located = lat, lon, true
	return p
}

func TestFindHomeIsTheBusiestCell(t *testing.T) {
	var all []library.MemoryPhoto
	for i := 0; i < 5; i++ {
		all = append(all, located(fmt.Sprintf("h%d", i), 52.52, 13.40))
	}
	all = append(all, located("away", 38.72, -9.14), photoAt("nowhere", "2020-01-01T10:00:00"))
	home := FindHome(all)
	if !home.Known || math.Abs(home.Lat-52.5) > 1e-9 || math.Abs(home.Lon-13.4) > 1e-9 {
		t.Errorf("home = %+v, want the cell 52.5, 13.4", home)
	}
	if FindHome([]library.MemoryPhoto{photoAt("x", "2020-01-01T10:00:00")}).Known {
		t.Error("a home without any located photo")
	}
}

func TestKmBetween(t *testing.T) {
	// Berlin to Lisbon is about 2,310 km.
	if km := KmBetween(52.52, 13.40, 38.72, -9.14); km < 2250 || km > 2370 {
		t.Errorf("Berlin–Lisbon = %.0f km", km)
	}
}

func TestRarityAway(t *testing.T) {
	var all []library.MemoryPhoto
	for i := 0; i < 9; i++ {
		all = append(all, located(fmt.Sprintf("h%d", i), 52.52, 13.40))
	}
	far := located("far", 48.14, 11.58) // Munich, about 500 km
	all = append(all, far)
	r := NewRarity(all, FindHome(all))
	if got := r.Of(far); math.Abs(got-math.Log(10)) > 1e-9 {
		t.Errorf("rarity of the one photo away = %v, want ln 10", got)
	}
	if got := r.Of(all[0]); got != 0 {
		t.Errorf("rarity of a plain photo at home = %v, want 0", got)
	}
}

func TestRarityIsTheRarestFeature(t *testing.T) {
	var all []library.MemoryPhoto
	for i := 0; i < 99; i++ {
		p := photoAt(fmt.Sprintf("p%d", i), "2020-01-01T10:00:00")
		p.Things, p.Tone = []string{"tree"}, "normal"
		all = append(all, p)
	}
	owl := photoAt("owl", "2020-01-01T10:00:00")
	owl.Things, owl.Tone = []string{"tree", "owl"}, "low"
	all = append(all, owl)
	r := NewRarity(all, Home{})
	if got := r.Of(owl); math.Abs(got-math.Log(100)) > 1e-9 {
		t.Errorf("rarity = %v, want ln 100 (owl and low key are each 1 in 100; the maximum, not the sum)", got)
	}
}
