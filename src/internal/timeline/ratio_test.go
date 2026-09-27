package timeline

import "testing"

func TestDisplayRatioSwapsForQuarterTurns(t *testing.T) {
	cases := []struct {
		w, h int
		o    string
		want float64
	}{
		{6000, 4000, "", 1.5},
		{6000, 4000, "1", 1.5},
		{6000, 4000, "6", 4000.0 / 6000},
		{6000, 4000, "8", 4000.0 / 6000},
		{6000, 4000, "3", 1.5},
		{0, 4000, "6", fallbackRatio},
		{6000, 4000, "rotate", 1.5},
	}
	for _, c := range cases {
		if got := displayRatio(c.w, c.h, c.o); got != c.want {
			t.Errorf("displayRatio(%d, %d, %q) = %v, want %v", c.w, c.h, c.o, got, c.want)
		}
	}
}
