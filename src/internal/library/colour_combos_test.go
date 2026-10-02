package library

import (
	"slices"
	"sort"
	"strconv"
	"strings"
	"testing"
)

const (
	orange = 1
	teal   = 6
	blue   = 8
)

func comboStore(t *testing.T) *Store {
	t.Helper()
	s := newTestStore(t)
	seedMeasured(t, s, []measuredPhoto{
		// Orange and teal, teal in two swatches that only together reach 10 %.
		{id: "film", path: "/lib/a.jpg", mono: "colour",
			swatches: []ColourSwatch{{HueBin: orange, Share: 0.4}, {HueBin: teal, Share: 0.06}, {HueBin: teal, Share: 0.05}, {HueBin: -1, Share: 0.49}}},
		// Orange, teal and blue.
		{id: "dusk", path: "/lib/b.jpg", mono: "colour",
			swatches: []ColourSwatch{{HueBin: orange, Share: 0.3}, {HueBin: teal, Share: 0.3}, {HueBin: blue, Share: 0.3}}},
		// Orange, with only a trace of teal.
		{id: "sign", path: "/lib/c.jpg", mono: "colour",
			swatches: []ColourSwatch{{HueBin: orange, Share: 0.8}, {HueBin: teal, Share: 0.05}}},
		{id: "grey", path: "/lib/d.jpg", mono: "colour",
			swatches: []ColourSwatch{{HueBin: -1, Share: 1}}},
	})
	return s
}

func TestHueSetsGroupPhotosByTheirHues(t *testing.T) {
	src, err := comboStore(t).ColourSource("")
	if err != nil {
		t.Fatal(err)
	}
	// The same photo read from a second library counts once.
	sets := hueSets(append(src.Swatches, src.Swatches...))
	got := map[string]int{}
	for _, s := range sets {
		got[fmtBins(s.Hues)] = s.Photos
	}
	want := map[string]int{"1,6": 1, "1,6,8": 1, "1": 1}
	if len(got) != len(want) {
		t.Fatalf("sets = %v, want %v", got, want)
	}
	for k, n := range want {
		if got[k] != n {
			t.Errorf("set %s = %d, want %d (all: %v)", k, got[k], n, got)
		}
	}
}

func TestListPhotosByColourCombination(t *testing.T) {
	s := comboStore(t)
	cases := []struct {
		name string
		hues []int
		want []string
	}{
		{"orange and teal", []int{orange, teal}, []string{"dusk", "film"}},
		{"orange, teal and blue", []int{orange, teal, blue}, []string{"dusk"}},
		{"orange alone", []int{orange}, []string{"dusk", "film", "sign"}},
	}
	for _, c := range cases {
		got, _ := listedIDs(t, s, ListPhotosOpts{Hues: c.hues})
		sort.Strings(got)
		if !slices.Equal(got, c.want) {
			t.Errorf("%s: got %v, want %v", c.name, got, c.want)
		}
	}
}

func fmtBins(bins []int) string {
	parts := make([]string, len(bins))
	for i, b := range bins {
		parts[i] = strconv.Itoa(b)
	}
	return strings.Join(parts, ",")
}
