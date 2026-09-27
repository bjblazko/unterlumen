package library

import (
	"math"
	"testing"
)

func TestParseGPSCoord(t *testing.T) {
	cases := []struct {
		coord, ref string
		want       float64
		ok         bool
	}{
		{"48.879850", "N", 48.879850, true},
		{"2.5", "W", -2.5, true},
		{"[48/1, 52/1, 4746/100]", "N", 48 + 52.0/60 + 47.46/3600, true},
		{"[33/1, 51/1, 0/1]", "S", -(33 + 51.0/60), true},
		{"[1/0, 2/1, 3/1]", "N", 0, false},
		{"", "N", 0, false},
		{"north", "N", 0, false},
	}
	for _, c := range cases {
		got, ok := ParseGPSCoord(c.coord, c.ref)
		if ok != c.ok || math.Abs(got-c.want) > 1e-9 {
			t.Errorf("ParseGPSCoord(%q, %q) = %v, %v; want %v, %v", c.coord, c.ref, got, ok, c.want, c.ok)
		}
	}
}
