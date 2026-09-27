package timeline

import (
	"encoding/json"
	"testing"
)

func TestSkeletonRoundsRatiosAndKeepsDays(t *testing.T) {
	s := &Stream{Version: "v", Start: "2020-01-01", Undated: 4, Photos: []Photo{{Day: 0, Ratio: 2.0 / 3}, {Day: 9, Ratio: 1.5}}}
	sk := s.Skeleton()
	if sk.Days[1] != 9 || sk.Ratios[0] != 0.667 || sk.Undated != 4 || sk.Start != "2020-01-01" || sk.Version != "v" {
		t.Fatalf("skeleton = %+v", sk)
	}
}

func TestSkeletonOfEmptyStreamHasEmptyArrays(t *testing.T) {
	b, _ := json.Marshal((&Stream{Version: "v"}).Skeleton())
	if string(b) != `{"version":"v","start":"","days":[],"ratios":[],"undated":0}` {
		t.Fatalf("json = %s", b)
	}
}

func TestPageClampsToTheStream(t *testing.T) {
	s := &Stream{Photos: make([]Photo, 7)}
	for _, c := range []struct{ from, count, want int }{{0, 5, 5}, {5, 5, 2}, {7, 5, 0}, {-1, 5, 0}, {0, 0, 0}} {
		if got := len(s.Page(c.from, c.count)); got != c.want {
			t.Errorf("Page(%d, %d) has %d photos, want %d", c.from, c.count, got, c.want)
		}
	}
}
